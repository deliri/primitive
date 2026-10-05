package cloudflare

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
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
	result, err := r2.CompleteMultipart(t.Context(), complete, []R2CompletedPart{part}, testPolicy())
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
		{"oversized control document", func([]byte) []byte {
			return bytes.Repeat([]byte("x"), core.CloudflareR2MultipartResponseMaximumBytes+1)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			baseline := multipartCreatedBytes(t)
			if got, err := decodeR2MultipartXML(baseline, "InitiateMultipartUploadResult"); err != nil || got.UploadID == "" {
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

func TestR2MultipartCompletionStreamsExactOrderedManifest(t *testing.T) {
	t.Parallel()
	parts := make([]R2CompletedPart, core.CloudflareR2MultipartMaximumParts)
	for i := range parts {
		parts[i] = R2CompletedPart{PartNumber: uint16(i + 1), ETag: fmt.Sprintf(`"part-%d"`, i+1)}
	}
	source, length, err := multipartCompletionBody(parts)
	if err != nil {
		t.Fatal(err)
	}
	var wire bytes.Buffer
	scratch := make([]byte, 31)
	n, err := io.CopyBuffer(&wire, source, scratch)
	if err != nil || uint64(n) != length {
		t.Fatalf("wire=%d/%v, want declared %d", n, err, length)
	}
	var decoded struct {
		XMLName xml.Name          `xml:"CompleteMultipartUpload"`
		Parts   []R2CompletedPart `xml:"Part"`
	}
	if err := xml.Unmarshal(wire.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Parts) != len(parts) {
		t.Fatalf("parts=%d, want %d", len(decoded.Parts), len(parts))
	}
	for i := range parts {
		if decoded.Parts[i] != parts[i] {
			t.Fatalf("part %d=%v, want %v", i, decoded.Parts[i], parts[i])
		}
	}
	for _, bad := range [][]R2CompletedPart{nil, parts[:0], append(parts, parts[0]), {parts[1], parts[0]}, {parts[0], parts[0]}, {{PartNumber: 0, ETag: `"x"`}}, {{PartNumber: 1, ETag: ""}}, {{PartNumber: 1, ETag: "\"x\n\""}}} {
		got, size, err := multipartCompletionBody(bad)
		if !errors.Is(err, core.ErrCloudflareBinding) || got != nil || size != 0 {
			t.Fatalf("bad manifest=%v/%d/%v, want zero and typed refusal", got, size, err)
		}
	}
}
