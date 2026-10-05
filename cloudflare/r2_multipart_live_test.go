//go:build integration

package cloudflare

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

// Native SDK client proof against real R2. This does not claim browser CORS,
// application auth, workout persistence, or a 100 GiB network transfer.
// witness:waiver test/determinism -- live R2 validates signatures against its clock; fresh timestamps isolate probe objects. Results assert bytes and identities, never wall time.
func TestR2MultipartLiveDirectClientLifecycle(t *testing.T) {
	t.Parallel()
	path := os.Getenv("KERNEL_MEDIA_LIVE_CONFIG")
	if path == "" {
		t.Fatal("live credential configuration absent")
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal("live credential file unavailable")
	}
	var input struct {
		Access string `json:"r2_access_key"`
		Secret string `json:"r2_secret_key"`
	}
	err = json.NewDecoder(io.LimitReader(file, 65536)).Decode(&input)
	if closeErr := file.Close(); err != nil || closeErr != nil {
		t.Fatal("live credential configuration invalid")
	}
	credentials, err := ParseR2Credentials([]byte(input.Access), []byte(input.Secret))
	if err != nil {
		t.Fatal("credential admission failed")
	}
	account, err := ParseAccountID("489102215ff69da592e147bee1e03fd0")
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewR2Server(credentials, account, R2JurisdictionDefault)
	if err != nil {
		t.Fatal("server construction failed")
	}
	if err := credentials.Close(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Error("server close failed")
		}
	})
	transport := http.DefaultTransport.(*http.Transport).Clone()
	t.Cleanup(transport.CloseIdleConnections)
	connection, err := exchange.NewClient(&http.Client{Transport: multipartLiveTransport{base: transport, t: t}})
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewR2Client(connection)
	if err != nil {
		t.Fatal(err)
	}
	bucket, err := ParseR2Bucket("cleanlift")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	key, err := ParseR2Key(fmt.Sprintf("kernel-proof/multipart/%d.bin", now.UnixNano()))
	if err != nil {
		t.Fatal(err)
	}
	signedAt, err := temporal.InstantFromUnixSeconds(now.Unix())
	if err != nil {
		t.Fatal(err)
	}
	expires, err := temporal.DurationFromSeconds(900)
	if err != nil {
		t.Fatal(err)
	}
	timeout, err := temporal.DurationFromSeconds(120)
	if err != nil {
		t.Fatal(err)
	}
	policy := exchange.StreamPolicy{OperationTimeout: timeout, Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject}}
	ctx, cancel := context.WithTimeout(t.Context(), 4*time.Minute)
	defer cancel()
	grant := func(action R2MultipartAction, id R2UploadID, number uint16) R2MultipartGrant {
		intent := R2MultipartPresignRequest{Bucket: bucket, Key: key, SignedAt: signedAt, Expires: expires, Action: action, UploadID: id, PartNumber: number}
		if action == R2MultipartCreate {
			intent.ContentType = core.HTTPMediaTypeOctetStream()
		}
		result, err := server.PresignMultipart(ctx, intent)
		if err != nil {
			t.Fatal("multipart capability failed")
		}
		return result
	}
	upload, err := client.CreateMultipart(ctx, grant(R2MultipartCreate, R2UploadID{}, 0), policy)
	if err != nil {
		t.Fatalf("create live multipart failed: %T", err)
	}
	t.Log("provider accepted multipart creation")
	completed := false
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 120*time.Second)
		defer stop()
		if !completed {
			abort, err := server.PresignMultipart(cleanup, R2MultipartPresignRequest{Bucket: bucket, Key: key, SignedAt: signedAt, Expires: expires, Action: R2MultipartAbort, UploadID: upload.UploadID})
			if err != nil || client.AbortMultipart(cleanup, abort, policy) != nil {
				t.Error("multipart cleanup abort failed")
			}
		}
		removal, err := server.Presign(cleanup, R2PresignRequest{Bucket: bucket, Key: key, SignedAt: signedAt, Expires: expires, Method: exchange.MethodDelete})
		if err != nil {
			t.Error("object cleanup capability failed")
			return
		}
		if _, err := client.Delete(cleanup, removal, policy); err != nil {
			t.Error("object cleanup failed")
		}
	})
	var parts []R2CompletedPart
	want := sha256.New()
	sizes := []uint64{core.CloudflareR2MultipartMinimumPartBytes, core.CloudflareR2MultipartMinimumPartBytes, 17}
	var total uint64
	for index, size := range sizes {
		length, err := core.NewByteLength(size)
		if err != nil {
			t.Fatal(err)
		}
		source := io.LimitReader(multipartPatternReader{value: byte(index + 1)}, int64(size))
		receipt, err := client.UploadPart(ctx, grant(R2MultipartPart, upload.UploadID, uint16(index+1)), io.TeeReader(source, want), length, policy)
		if err != nil {
			t.Fatalf("live part %d failed: %T", index+1, err)
		}
		parts = append(parts, receipt)
		total += size
	}
	t.Logf("provider accepted %d direct streamed parts / %d bytes", len(parts), total)
	result, err := client.CompleteMultipart(ctx, grant(R2MultipartComplete, upload.UploadID, 0), parts, policy)
	if err != nil {
		t.Fatalf("live completion failed: %T", err)
	}
	completed = true
	if result.Bucket != bucket || result.Key != key || !r2ETag(result.ETag) {
		t.Fatal("completed object binding invalid")
	}
	read, err := server.Presign(ctx, R2PresignRequest{Bucket: bucket, Key: key, SignedAt: signedAt, Expires: expires, Method: exchange.MethodGet})
	if err != nil {
		t.Fatal(err)
	}
	got := sha256.New()
	if _, err := client.Read(ctx, read, got, policy); err != nil {
		t.Fatalf("live readback failed: %T", err)
	}
	if string(got.Sum(nil)) != string(want.Sum(nil)) {
		t.Fatal("live multipart readback digest differs from streamed source")
	}
	t.Log("completed object readback matches exact streamed source hash")
	// A separate session demonstrates abort prevents subsequent part spending.
	abandoned, err := client.CreateMultipart(ctx, grant(R2MultipartCreate, R2UploadID{}, 0), policy)
	if err != nil {
		t.Fatalf("abort fixture create failed: %T", err)
	}
	aborted := false
	t.Cleanup(func() {
		if aborted {
			return
		}
		cleanup, stop := context.WithTimeout(context.Background(), 120*time.Second)
		defer stop()
		capability, err := server.PresignMultipart(cleanup, R2MultipartPresignRequest{Bucket: bucket, Key: key, SignedAt: signedAt, Expires: expires, Action: R2MultipartAbort, UploadID: abandoned.UploadID})
		if err != nil || client.AbortMultipart(cleanup, capability, policy) != nil {
			t.Error("abandoned multipart cleanup failed")
		}
	})
	if err := client.AbortMultipart(ctx, grant(R2MultipartAbort, abandoned.UploadID, 0), policy); err != nil {
		t.Fatalf("abort failed: %T", err)
	}
	aborted = true
	one, err := core.NewByteLength(1)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.UploadPart(ctx, grant(R2MultipartPart, abandoned.UploadID, 1), io.LimitReader(multipartPatternReader{value: 1}, 1), one, policy)
	refusal, ok := errors.AsType[exchange.StatusError](err)
	if !ok {
		t.Fatalf("aborted session part error=%T, want typed provider 404", err)
	}
	status, err := refusal.Status().Int()
	if err != nil || status != http.StatusNotFound {
		t.Fatalf("aborted session status=%d/%v, want 404", status, err)
	}
	t.Log("aborted multipart session refuses a subsequent part upload")
}

type multipartPatternReader struct{ value byte }

func (r multipartPatternReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = r.value
	}
	return len(p), nil
}

// Retains only XML names and lengths for diagnosing live protocol differences;
// credentials and bearer URLs never enter test output. Exact bytes are returned
// unchanged to the actual SDK parser after this bounded observation.
type multipartLiveTransport struct {
	base http.RoundTripper
	t    *testing.T
}

func (o multipartLiveTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	response, err := o.base.RoundTrip(r)
	if err != nil || r.Method != "POST" || response.StatusCode != http.StatusOK {
		return response, err
	}
	data, readErr := io.ReadAll(io.LimitReader(response.Body, core.CloudflareR2MultipartResponseMaximumBytes+1))
	closeErr := response.Body.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return nil, err
	}
	response.Body = io.NopCloser(bytes.NewReader(data))
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		if start, ok := token.(xml.StartElement); ok {
			o.t.Logf("provider control XML element=%s namespace=%s attributes=%d", start.Name.Local, start.Name.Space, len(start.Attr))
		}
	}
	return response, nil
}
