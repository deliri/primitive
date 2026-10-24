package objectstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/jsonio"
	"github.com/deliri/primitive/v2026/temporal"
)

const s3FixtureAccess = "AKIAIOSFODNN7EXAMPLE"
const s3FixtureSecret = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"

// This is a foreign-format fixture writer, not a production credential export.
// The production owner deliberately refuses marshaling decoded secret values.
type s3CredentialFixture struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
}

func s3CredentialFixtureJSON(t testing.TB) []byte {
	t.Helper()
	var data bytes.Buffer
	err := jsonio.WriteProjection(t.Context(), &data, s3CredentialFixture{AccessKeyID: s3FixtureAccess, SecretAccessKey: s3FixtureSecret}, jsonio.OwnedCodecPolicy{}, jsonio.LayoutCompact)
	if err != nil {
		t.Fatalf("credential fixture encode error = %v, want nil", err)
	}
	return data.Bytes()
}

func s3FileFixture(t testing.TB, dir string, payload []byte) S3FileUpload {
	t.Helper()
	path := filepath.Join(dir, "static-credentials.json")
	if err := os.WriteFile(path, s3CredentialFixtureJSON(t), 0600); err != nil {
		t.Fatalf("credential fixture write error = %v, want nil", err)
	}
	file, err := core.ParseAbsolutePath(path)
	if err != nil {
		t.Fatal(err)
	}
	endpoint, err := core.ParseHTTPEndpoint("https://s3.us-east-1.amazonaws.com")
	if err != nil {
		t.Fatal(err)
	}
	region, err := ParseS3Region("us-east-1")
	if err != nil {
		t.Fatal(err)
	}
	bucket, err := ParseS3Bucket("retention.example")
	if err != nil {
		t.Fatal(err)
	}
	key, err := ParseS3Key("scratch/archive.tar")
	if err != nil {
		t.Fatal(err)
	}
	expires, err := temporal.DurationFromSeconds(60)
	if err != nil {
		t.Fatal(err)
	}
	maximum, err := core.NewByteCount(64 * 1024) // Explicit fixture policy, not an AWS limit.
	if err != nil {
		t.Fatal(err)
	}
	return S3FileUpload{CredentialFile: file, CredentialMaximumBytes: maximum, Endpoint: endpoint, Region: region, Bucket: bucket, Key: key, Source: bytes.NewReader(payload), Integrity: s3FileIntegrity(payload), ContentType: core.HTTPMediaTypeOctetStream(), SignedAt: temporal.InstantFromNanoseconds(4_000_000_000_000_000_000), Expires: expires}
}

func s3FileIntegrity(payload []byte) Integrity {
	length, _ := core.NewByteLength(uint64(len(payload)))
	return Integrity{Length: length, SHA256: core.SHA256Of(payload), CRC32C: s3FileCRC(payload)}
}

func s3FileCRC(payload []byte) core.CRC32C {
	return core.NewCRC32C(crc32.Checksum(payload, crc32.MakeTable(crc32.Castagnoli)))
}

func s3LocalProvider(t testing.TB, handler http.Handler) Client {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	transport := server.Client().Transport.(*http.Transport).Clone()
	transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	transport.TLSClientConfig.ServerName = "example.com"
	address := strings.TrimPrefix(server.URL, "https://")
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, address)
	}
	t.Cleanup(transport.CloseIdleConnections)
	network, err := exchange.NewClient(&http.Client{Transport: transport})
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(network)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

// A pinned official AWS signer independently verifies the complete target and
// signed create-only/checksum/media fields observed by the real local TLS
// provider. It does not claim live AWS account or authorization proof.
func s3ObservedSignature(request *http.Request, r S3FileUpload) (bool, error) {
	u := *request.URL
	u.Scheme = core.SchemeHTTPS
	u.Host = request.Host
	query := u.Query()
	got := query.Get(core.SignatureV4SignatureQuery)
	for _, key := range []string{core.SignatureV4SignatureQuery, core.SignatureV4AlgorithmQuery, core.SignatureV4CredentialQuery, core.SignatureV4DateQuery, core.SignatureV4SignedHeadersQuery} {
		query.Del(key)
	}
	u.RawQuery = query.Encode()
	native, err := http.NewRequestWithContext(request.Context(), http.MethodPut, u.String(), nil)
	if err != nil {
		return false, err
	}
	native.Header.Set(core.HTTPHeaderContentType().String(), request.Header.Get(core.HTTPHeaderContentType().String()))
	native.Header.Set(headerIfNoneMatch, request.Header.Get(headerIfNoneMatch))
	native.Header.Set(headerS3ChecksumCRC32C, request.Header.Get(headerS3ChecksumCRC32C))
	stamp, err := r.SignedAt.Time()
	if err != nil {
		return false, err
	}
	signer := v4.NewSigner(func(options *v4.SignerOptions) {
		options.DisableHeaderHoisting = true
		options.DisableURIPathEscaping = true
	})
	want, _, err := signer.PresignHTTP(request.Context(), aws.Credentials{AccessKeyID: s3FixtureAccess, SecretAccessKey: s3FixtureSecret}, native, core.S3UnsignedPayload, core.S3SigningService, r.Region.String(), stamp)
	if err != nil {
		return false, err
	}
	endpoint, err := core.ParseHTTPEndpoint(want)
	if err != nil {
		return false, err
	}
	parsed := endpoint.HTTPURL()
	return got != "" && got == parsed.Query().Get(core.SignatureV4SignatureQuery), nil
}

func TestS3FileUploadLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, key string
		size      int
	}{
		{name: "positive ordinary sealed file", key: "scratch/archive.tar", size: 39},
		{name: "neutral empty sealed object", key: "empty", size: 0},
		{name: "one byte object", key: "one", size: 1},
		{name: "below copy window", key: "below", size: 32767},
		{name: "exact copy window", key: "window", size: 32768},
		{name: "above copy window", key: "above", size: 32769},
		{name: "multiple windows and final partial", key: "many", size: 65539},
		{name: "key preserves repeated and leading slashes", key: "//source///archive", size: 13},
		{name: "key preserves balanced dot segments", key: "scratch/a/.././archive", size: 13},
		{name: "key preserves Unicode reserved bytes and NUL", key: "東京/+ %?#\x00", size: 13},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			payload := bytes.Repeat([]byte{0, 0x81, 0xff, 0x42}, tc.size/4+1)[:tc.size]
			r := s3FileFixture(t, dir, payload)
			if tc.name == "positive ordinary sealed file" {
				path := filepath.Join(dir, "sealed.tar")
				if err := os.WriteFile(path, payload, 0600); err != nil {
					t.Fatal(err)
				}
				file, err := os.Open(path)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := file.Close(); err != nil {
						t.Errorf("borrowed source close error=%v, want nil", err)
					}
				})
				r.Source = file
			}
			var err error
			r.Key, err = ParseS3Key(tc.key)
			if err != nil {
				t.Fatal(err)
			}
			var calls atomic.Int32
			client := s3LocalProvider(t, http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				calls.Add(1)
				body, err := io.ReadAll(request.Body)
				checksum, _ := r.Integrity.CRC32C.Base64()
				signed, signErr := s3ObservedSignature(request, r)
				if err != nil || signErr != nil || !signed || request.Method != http.MethodPut || request.ContentLength != int64(len(payload)) || !bytes.Equal(body, payload) || request.URL.Path != "/"+r.Bucket.String()+"/"+tc.key || request.Header.Get(headerIfNoneMatch) != "*" || request.Header.Get(headerS3ChecksumCRC32C) != checksum || request.Header.Get(core.HTTPHeaderContentType().String()) != r.ContentType.String() {
					t.Errorf("provider request method=%s bytes=%d declared=%d path=%q create=%q crc=%q signed=%v errors=(%v,%v), want exact sealed bytes=%d key=%q and official signature", request.Method, len(body), request.ContentLength, request.URL.Path, request.Header.Get(headerIfNoneMatch), request.Header.Get(headerS3ChecksumCRC32C), signed, err, signErr, len(payload), tc.key)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.Header().Set(headerS3Version, "version-exact")
				w.WriteHeader(http.StatusOK)
			}))
			got, gotErr := UploadS3File(t.Context(), client, r)
			version, present := got.Version()
			if gotErr != nil || got.Validate() != nil || got.Commitment() != CommitmentConfirmed || got.Bytes() != r.Integrity.Length || got.SHA256() != r.Integrity.SHA256 || got.CRC32C() != r.Integrity.CRC32C || !present || version.String() != "version-exact" || calls.Load() != 1 {
				t.Fatalf("UploadS3File = (%v,%v) calls=%d version=%q/%v, want one exact confirmed transfer", got, gotErr, calls.Load(), version.String(), present)
			}
			if file, ok := r.Source.(*os.File); ok {
				position, err := file.Seek(0, io.SeekCurrent)
				if err != nil || position != int64(len(payload)) {
					t.Fatalf("borrowed source cursor=(%d,%v), want still-open exhausted extent %d", position, err, len(payload))
				}
			}
		})
	}
}

type s3EarlyResponseTransport struct{}

func (s3EarlyResponseTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return &http.Response{Request: request, StatusCode: http.StatusOK, Header: make(http.Header), Body: http.NoBody}, nil
}

func TestS3FileEarlySuccessIsNotSourceProof(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := s3FileFixture(t, dir, []byte("sealed"))
	network, err := exchange.NewClient(&http.Client{Transport: s3EarlyResponseTransport{}})
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(network)
	if err != nil {
		t.Fatal(err)
	}
	got, gotErr := UploadS3File(t.Context(), client, r)
	if got != (Transfer{}) || !errors.Is(gotErr, core.ErrObjectStoreIntegrity) {
		t.Fatalf("unconsumed synthetic early HTTP200 = (%v,%v), want zero proof and source-integrity refusal", got, gotErr)
	}
}

type s3CanceledUploadOutcome struct {
	transfer Transfer
	err      error
}
type s3CancelableSource struct {
	ctx     context.Context
	written int
}

func (s *s3CancelableSource) Read(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	if s.written < 32768 {
		count := min(len(data), 32768-s.written)
		for i := range count {
			data[i] = 'x'
		}
		s.written += count
		return count, nil
	}
	<-s.ctx.Done()
	return 0, s.ctx.Err()
}

func TestS3FileCancellationAfterProgressJoins(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := s3FileFixture(t, dir, bytes.Repeat([]byte("x"), 65536))
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	r.Source = &s3CancelableSource{ctx: ctx}
	progress := make(chan struct{}, 1)
	client := s3LocalProvider(t, http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		var first [1]byte
		_, err := io.ReadFull(request.Body, first[:])
		if err == nil {
			progress <- struct{}{}
		}
		_, _ = io.Copy(io.Discard, request.Body)
	}))
	joined := make(chan s3CanceledUploadOutcome, 1)
	go func() {
		got, err := UploadS3File(ctx, client, r)
		joined <- s3CanceledUploadOutcome{transfer: got, err: err}
	}()
	backstop, stop := context.WithTimeout(context.Background(), 30*time.Second)
	defer stop()
	select {
	case <-progress:
	case <-backstop.Done():
		cancel()
		select {
		case <-joined:
		case <-backstop.Done():
		}
		t.Fatalf("provider progress error=%v, want first byte before cancellation", backstop.Err())
	}
	cancel()
	select {
	case got := <-joined:
		if got.transfer != (Transfer{}) || !errors.Is(got.err, context.Canceled) {
			t.Fatalf("post-progress cancellation = (%v,%v), want joined zero and cancellation", got.transfer, got.err)
		}
	case <-backstop.Done():
		t.Fatalf("upload join error=%v, want completed cancellation", backstop.Err())
	}
}

type s3FileFailingSource struct{ cause error }

func (s s3FileFailingSource) Read([]byte) (int, error) { return 0, s.cause }

func TestS3FileUploadFailureLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		change func(*S3FileUpload)
		status int
		want   error
	}{
		{name: "negative local SHA disagreement withdraws provider success", change: func(r *S3FileUpload) { r.Integrity.SHA256 = core.SHA256Of([]byte("foreign")) }, status: 200, want: core.ErrObjectStoreIntegrity},
		{name: "local CRC disagreement withdraws provider success", change: func(r *S3FileUpload) { r.Integrity.CRC32C = s3FileCRC([]byte("foreign")) }, status: 200, want: core.ErrObjectStoreIntegrity},
		{name: "source shorter than sealed extent", change: func(r *S3FileUpload) { r.Source = bytes.NewReader([]byte("x")) }, status: 200, want: core.ErrObjectStoreSource},
		{name: "source longer than sealed extent", change: func(r *S3FileUpload) { r.Source = bytes.NewReader([]byte("sealed-long")) }, status: 200, want: core.ErrObjectStoreSource},
		{name: "wrapped EOF is a source refusal", change: func(r *S3FileUpload) {
			r.Source = s3FileFailingSource{cause: fmt.Errorf("late native refusal: %w", io.EOF)}
		}, status: 200, want: io.EOF},
		{name: "native source cause remains inspectable", change: func(r *S3FileUpload) { r.Source = s3FileFailingSource{cause: io.ErrClosedPipe} }, status: 200, want: io.ErrClosedPipe},
		{name: "create-only provider conflict", status: 412, want: core.ErrObjectStoreConflict},
		{name: "provider permission refusal", status: 403, want: core.ErrExchangeAuthentication},
		{name: "provider server error is not retried", status: 503, want: core.ErrExchangeUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			r := s3FileFixture(t, dir, []byte("sealed"))
			if tc.change != nil {
				tc.change(&r)
			}
			var calls atomic.Int32
			client := s3LocalProvider(t, http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				calls.Add(1)
				_, _ = io.Copy(io.Discard, request.Body)
				w.WriteHeader(tc.status)
			}))
			got, gotErr := UploadS3File(t.Context(), client, r)
			if got != (Transfer{}) || !errors.Is(gotErr, tc.want) || calls.Load() > 1 {
				t.Fatalf("UploadS3File = (%v,%v), calls=%d, want zero and %v without retry", got, gotErr, calls.Load(), tc.want)
			}
			if tc.status == http.StatusForbidden || tc.status == http.StatusServiceUnavailable {
				status, present := errors.AsType[exchange.StatusError](gotErr)
				code, statusErr := status.Status().Int()
				if !present || statusErr != nil || code != tc.status || status.Expected() != core.HTTPStatusOK() {
					t.Fatalf("provider status cause=(%d,%v,%v), want native status %d and expected200", code, present, statusErr, tc.status)
				}
			}
		})
	}

	t.Run("neutral canceled request reads neither credentials nor source", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		r := s3FileFixture(t, dir, []byte("sealed"))
		r.Source = s3FileFailingSource{cause: io.ErrClosedPipe}
		if err := os.Remove(r.CredentialFile.String()); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		client, err := NewStandardClient()
		if err != nil {
			t.Fatal(err)
		}
		got, gotErr := UploadS3File(ctx, client, r)
		if got != (Transfer{}) || !errors.Is(gotErr, context.Canceled) || errors.Is(gotErr, os.ErrNotExist) || errors.Is(gotErr, io.ErrClosedPipe) {
			t.Fatalf("canceled UploadS3File = (%v,%v), want zero and cancellation before file/source effects", got, gotErr)
		}
	})
}

func TestS3FileCredentialsLayerTriad(t *testing.T) {
	t.Parallel()
	canonical := string(s3CredentialFixtureJSON(t))
	for _, tc := range []struct {
		name, data string
		want       bool
	}{
		{name: "positive declared static pair", data: canonical, want: true},
		{name: "neutral whitespace preserves static pair", data: " \n" + canonical + " \t", want: true},
		{name: "reordered declared fields", data: `{"secret_access_key":"` + s3FixtureSecret + `","access_key_id":"` + s3FixtureAccess + `"}`, want: true},
		{name: "negative temporary session token", data: strings.TrimSuffix(canonical, "}") + `,"session_token":"never-log-me"}`},
		{name: "profile discovery is unsupported", data: strings.TrimSuffix(canonical, "}") + `,"profile":"operator"}`},
		{name: "unknown member cannot silently disappear", data: strings.TrimSuffix(canonical, "}") + `,"future":"never-log-me"}`},
		{name: "duplicate access key", data: strings.TrimSuffix(canonical, "}") + `,"access_key_id":"never-log-me"}`},
		{name: "missing secret", data: `{"access_key_id":"` + s3FixtureAccess + `"}`},
		{name: "null secret", data: `{"access_key_id":"` + s3FixtureAccess + `","secret_access_key":null}`},
		{name: "wrong typed secret", data: `{"access_key_id":"` + s3FixtureAccess + `","secret_access_key":31}`},
		{name: "empty secret", data: `{"access_key_id":"` + s3FixtureAccess + `","secret_access_key":""}`},
		{name: "temporary access key without token cannot masquerade as static", data: strings.Replace(canonical, "AKIA", "ASIA", 1)},
		{name: "access key control byte refused", data: strings.Replace(canonical, s3FixtureAccess, `unsafe\naccess-key`, 1)},
		{name: "trailing second document", data: canonical + canonical},
		{name: "truncated object", data: strings.TrimSuffix(canonical, "}")},
		{name: "neutral absent document", data: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := decodeS3FileCredentials(t.Context(), strings.NewReader(tc.data))
			defer got.destroy()
			if tc.want {
				if gotErr != nil || got.Validate() != nil || string(got.AccessKeyID.value) != s3FixtureAccess || string(got.SecretAccessKey.value) != s3FixtureSecret {
					t.Fatalf("credential decode error = %v, sizes=(%d,%d), want exact declared static pair", gotErr, len(got.AccessKeyID.value), len(got.SecretAccessKey.value))
				}
				return
			}
			if !errors.Is(gotErr, core.ErrObjectStoreContract) || len(got.AccessKeyID.value) != 0 || len(got.SecretAccessKey.value) != 0 {
				t.Fatalf("credential decode error=%v sizes=(%d,%d), want typed refusal and zero credential", gotErr, len(got.AccessKeyID.value), len(got.SecretAccessKey.value))
			}
			for _, verb := range []string{"%v", "%+v", "%#v", "%s"} {
				if text := fmt.Sprintf(verb, gotErr); strings.Contains(text, s3FixtureAccess) || strings.Contains(text, s3FixtureSecret) || strings.Contains(text, "never-log-me") {
					t.Fatalf("credential diagnostic %s = %q, want redacted", verb, text)
				}
			}
		})
	}
	got, err := decodeS3FileCredentials(t.Context(), s3FileFailingSource{cause: io.ErrClosedPipe})
	if !errors.Is(err, io.ErrClosedPipe) || got.Validate() == nil {
		t.Fatalf("native credential read = (%v,%v), want zero and retained native cause", got, err)
	}
}

func TestS3FileSigningExpiryLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		nanos      int64
		stamp      int64
		want       bool
		wantExpiry int64
	}{
		{name: "one below native minimum", nanos: 999999999},
		{name: "positive exact native minimum", nanos: 1000000000, stamp: 1000000000, want: true, wantExpiry: 2000000000},
		{name: "one above minimum is fractional not rounded", nanos: 1000000001},
		{name: "second whole second", nanos: 2000000000, want: true, wantExpiry: 2000000000},
		{name: "one nanosecond below maximum is not rounded", nanos: 604800000000000 - 1},
		{name: "exact inclusive maximum", nanos: 604800000000000, want: true, wantExpiry: 604800000000000},
		{name: "one nanosecond above maximum", nanos: 604800000000001},
		{name: "one second above maximum", nanos: 604801000000000},
		{name: "neutral zero does not select implicit expiry"},
		{name: "instant overflow refused", nanos: 1000000000, stamp: math.MaxInt64},
		{name: "fractional signing time uses published whole second", nanos: 1000000000, stamp: 1999999999, want: true, wantExpiry: 2000000000},
		{name: "negative fraction uses prior UTC second", nanos: 1000000000, stamp: -1, want: true, wantExpiry: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			r := s3FileFixture(t, dir, []byte("sealed"))
			r.Expires, _ = temporal.DurationFromNanoseconds(tc.nanos)
			r.SignedAt = temporal.InstantFromNanoseconds(tc.stamp)
			gotErr := r.Validate()
			if !tc.want {
				if !errors.Is(gotErr, core.ErrObjectStoreContract) {
					t.Fatalf("request expiry validation = %v, want typed refusal", gotErr)
				}
				return
			}
			credentials, err := decodeS3FileCredentials(t.Context(), bytes.NewReader(s3CredentialFixtureJSON(t)))
			if err != nil {
				t.Fatal(err)
			}
			defer credentials.destroy()
			target, err := presignS3File(t.Context(), r, credentials)
			stamp, stampErr := target.ExpiresAt.Nanoseconds()
			if gotErr != nil || err != nil || stampErr != nil || stamp != tc.wantExpiry || target.URL.value.Query().Get(core.S3QueryExpires) != fmt.Sprint(tc.nanos/1000000000) {
				t.Fatalf("presign expiry = (%d,%v,%v), validate=%v, want %d with exact wire duration", stamp, err, stampErr, gotErr, tc.wantExpiry)
			}
		})
	}
}
