package cloudflare

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// This wire fixture is produced by encoding/xml from typed fields. The TLS
// peer represents provider protocol behavior; a separate live run proves R2.
type multipartCreatedFixture struct {
	XMLName  xml.Name `xml:"InitiateMultipartUploadResult"`
	Bucket   string
	Key      string
	UploadID string `xml:"UploadId"`
}

func multipartCreatedBytes(t testing.TB) []byte {
	t.Helper()
	data, err := xml.Marshal(multipartCreatedFixture{Bucket: "media-bucket", Key: "video/owner/large file.mp4", UploadID: "opaque+/upload=="})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestR2MultipartClientRealTLSLifecycle(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	payload := strings.Repeat("media-bytes", 1024)
	client := testExchange(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if !verifyR2TestSignature(r, []byte("r2-secret-key")) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		q := r.URL.Query()
		switch {
		case r.Method == "POST" && q.Has("uploads"):
			if _, err := w.Write(multipartCreatedBytes(t)); err != nil {
				t.Error(err)
			}
		case r.Method == "PUT" && q.Get("partNumber") == "1":
			body, err := io.ReadAll(r.Body)
			if err != nil || string(body) != payload || r.ContentLength != int64(len(payload)) {
				t.Errorf("uploaded=%d/%d/%v, want exact %d", len(body), r.ContentLength, err, len(payload))
			}
			w.Header().Set("ETag", `"opaque-part"`)
		case r.Method == "POST" && q.Get("uploadId") == "opaque+/upload==":
			var body struct {
				XMLName xml.Name          `xml:"CompleteMultipartUpload"`
				Parts   []R2CompletedPart `xml:"Part"`
			}
			if err := xml.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if len(body.Parts) != 1 || body.Parts[0] != (R2CompletedPart{PartNumber: 1, ETag: `"opaque-part"`}) {
				t.Errorf("completed parts=%v, want exact provider receipt", body.Parts)
			}
			result := struct {
				XMLName                              xml.Name `xml:"CompleteMultipartUploadResult"`
				Bucket, Key, ETag, ChecksumCRC64NVME string
			}{Bucket: "media-bucket", Key: "video/owner/large file.mp4", ETag: `"opaque-complete-1"`, ChecksumCRC64NVME: "AAAAAAAAAAA="}
			if err := xml.NewEncoder(w).Encode(result); err != nil {
				t.Error(err)
			}
		case r.Method == "DELETE":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	})
	r2, err := NewR2Client(client)
	if err != nil {
		t.Fatal(err)
	}
	server := testR2Server(t, R2JurisdictionDefault)
	create, err := server.PresignMultipart(t.Context(), multipartIntent(t, R2MultipartCreate))
	if err != nil {
		t.Fatal(err)
	}
	session, err := r2.CreateMultipart(t.Context(), create, testPolicy())
	if err != nil || session.UploadID.String() != "opaque+/upload==" {
		t.Fatalf("create=%v/%v, want bound provider session", session, err)
	}
	partGrant, err := server.PresignMultipart(t.Context(), multipartIntent(t, R2MultipartPart))
	if err != nil {
		t.Fatal(err)
	}
	length, err := core.NewByteLength(uint64(len(payload)))
	if err != nil {
		t.Fatal(err)
	}
	part, err := r2.UploadPart(t.Context(), partGrant, strings.NewReader(payload), length, testPolicy())
	if err != nil {
		t.Fatal(err)
	}
	complete, err := server.PresignMultipart(t.Context(), multipartIntent(t, R2MultipartComplete))
	if err != nil {
		t.Fatal(err)
	}
	result, err := r2.CompleteMultipart(t.Context(), complete, multipartTestParts(part), testPolicy())
	if err != nil || result.ETag != `"opaque-complete-1"` || result.Key != session.Key {
		t.Fatalf("complete=%v/%v, want exact object acceptance", result, err)
	}
	abort, err := server.PresignMultipart(t.Context(), multipartIntent(t, R2MultipartAbort))
	if err != nil {
		t.Fatal(err)
	}
	if err := r2.AbortMultipart(t.Context(), abort, testPolicy()); err != nil {
		t.Fatal(err)
	}
	if got := requests.Load(); got != 4 {
		t.Fatalf("requests=%d, want 4 single attempts", got)
	}
}

func TestR2MultipartCreateRejectsHostileProviderDocuments(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"empty response", func([]byte) []byte { return nil }},
		{"truncated XML", func(b []byte) []byte { return b[:len(b)-1] }},
		{"HTTP success carrying Error", func([]byte) []byte { return []byte(`<Error><Code>NoSuchUpload</Code></Error>`) }},
		{"crossed bucket", func(b []byte) []byte { return bytes.ReplaceAll(b, []byte("media-bucket"), []byte("other-bucket")) }},
		{"crossed key", func(b []byte) []byte { return bytes.ReplaceAll(b, []byte("large file.mp4"), []byte("other.mp4")) }},
		{"missing upload ID", func(b []byte) []byte { return bytes.ReplaceAll(b, []byte("opaque+/upload=="), nil) }},
		{"duplicate identity", func(b []byte) []byte {
			return bytes.ReplaceAll(b, []byte("</UploadId>"), []byte("</UploadId><UploadId>other</UploadId>"))
		}},
		{"unknown field", func(b []byte) []byte {
			return bytes.ReplaceAll(b, []byte("</UploadId>"), []byte("</UploadId><Future>1</Future>"))
		}},
		{"nested scalar", func(b []byte) []byte { return bytes.ReplaceAll(b, []byte("opaque+/upload=="), []byte("<nested/>")) }},
		{"trailing second root", func(b []byte) []byte { return append(b, b...) }},
		{"directives refused", func(b []byte) []byte { return append([]byte(`<!DOCTYPE x>`), b...) }},
		{"unexpected namespace", func(b []byte) []byte {
			return bytes.Replace(b, []byte("<InitiateMultipartUploadResult>"), []byte(`<InitiateMultipartUploadResult xmlns="https://other">`), 1)
		}},
		{"attribute on scalar", func(b []byte) []byte { return bytes.Replace(b, []byte("<Key>"), []byte(`<Key hidden="yes">`), 1) }},
		{"unstructured multiwindow response", func([]byte) []byte {
			return bytes.Repeat([]byte("x"), (2<<20)+1)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			baseline := multipartCreatedBytes(t)
			if got, err := decodeR2MultipartXML(bytes.NewReader(baseline), "InitiateMultipartUploadResult"); err != nil || got.UploadID == "" {
				t.Fatalf("baseline=%v/%v, want populated valid", got, err)
			}
			body := tc.mutate(bytes.Clone(baseline))
			if bytes.Equal(body, baseline) {
				t.Fatal("mutation unchanged, want changed fact")
			}
			peer := testExchange(t, func(w http.ResponseWriter, r *http.Request) {
				if _, err := w.Write(body); err != nil {
					t.Error(err)
				}
			})
			client, err := NewR2Client(peer)
			if err != nil {
				t.Fatal(err)
			}
			server := testR2Server(t, R2JurisdictionDefault)
			grant, err := server.PresignMultipart(t.Context(), multipartIntent(t, R2MultipartCreate))
			if err != nil {
				t.Fatal(err)
			}
			got, err := client.CreateMultipart(t.Context(), grant, testPolicy())
			if !errors.Is(err, core.ErrCloudflareResponse) || got != (R2MultipartUpload{}) {
				t.Fatalf("create=%v/%v, want zero and typed response refusal", got, err)
			}
		})
	}
}

// Fixture source for tests whose subject is the HTTP boundary. Production callers
// supply their own receipt stream; this helper does not encode or validate it.
func multipartTestParts(parts ...R2CompletedPart) R2CompletedParts {
	return func(ctx context.Context, yield func(R2CompletedPart) error) error {
		for _, part := range parts {
			if err := yield(part); err != nil {
				return err
			}
		}
		return ctx.Err()
	}
}

func TestR2MultipartCompletionWriterLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		parts   []R2CompletedPart
		wantErr error
	}{
		{name: "one actual part receipt", parts: []R2CompletedPart{{PartNumber: 1, ETag: `"one"`}}},
		{name: "XML metacharacters remain the exact ETag", parts: []R2CompletedPart{{PartNumber: 1, ETag: `"<&>"`}}},
		{name: "ordered native part numbers may have gaps", parts: []R2CompletedPart{{PartNumber: 1, ETag: `"one"`}, {PartNumber: core.CloudflareR2MultipartMaximumParts, ETag: `"last"`}}},
		{name: "empty source cannot complete", wantErr: core.ErrCloudflareBinding},
		{name: "duplicate number cannot complete", parts: []R2CompletedPart{{PartNumber: 1, ETag: `"one"`}, {PartNumber: 1, ETag: `"other"`}}, wantErr: core.ErrCloudflareBinding},
		{name: "reversed provider order cannot complete", parts: []R2CompletedPart{{PartNumber: 2, ETag: `"two"`}, {PartNumber: 1, ETag: `"one"`}}, wantErr: core.ErrCloudflareBinding},
		{name: "zero part is not a provider receipt", parts: []R2CompletedPart{{ETag: `"one"`}}, wantErr: core.ErrCloudflareBinding},
		{name: "missing ETag cannot complete", parts: []R2CompletedPart{{PartNumber: 1}}, wantErr: core.ErrCloudflareBinding},
		{name: "header injection cannot complete", parts: []R2CompletedPart{{PartNumber: 1, ETag: "\"x\n\""}}, wantErr: core.ErrCloudflareBinding},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var wire bytes.Buffer
			err := writeMultipartCompletion(t.Context(), &wire, multipartTestParts(tc.parts...))
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("write completion = %v, want %v", err, tc.wantErr)
			}
			var decoded struct {
				XMLName xml.Name          `xml:"CompleteMultipartUpload"`
				Parts   []R2CompletedPart `xml:"Part"`
			}
			decodeErr := xml.Unmarshal(wire.Bytes(), &decoded)
			if tc.wantErr != nil {
				if decodeErr == nil {
					t.Fatal("refused source emitted a complete provider command")
				}
				return
			}
			if decodeErr != nil || !slices.Equal(decoded.Parts, tc.parts) {
				t.Fatalf("decoded receipt sequence = %v/%v, want %v", decoded.Parts, decodeErr, tc.parts)
			}
		})
	}
}
