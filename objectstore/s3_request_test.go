package objectstore

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestS3FileRequestRefusesBeforeSourceEffects(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		change func(*S3FileUpload)
	}{
		{name: "absent credential file", change: func(r *S3FileUpload) { r.CredentialFile = core.AbsolutePath{} }},
		{name: "absent source", change: func(r *S3FileUpload) { r.Source = nil }},
		{name: "absent integrity", change: func(r *S3FileUpload) { r.Integrity = Integrity{} }},
		{name: "absent region", change: func(r *S3FileUpload) { r.Region = S3Region{} }},
		{name: "absent bucket", change: func(r *S3FileUpload) { r.Bucket = S3Bucket{} }},
		{name: "absent key", change: func(r *S3FileUpload) { r.Key = S3Key{} }},
		{name: "absent endpoint", change: func(r *S3FileUpload) { r.Endpoint = core.HTTPEndpoint{} }},
		{name: "absent content type", change: func(r *S3FileUpload) { r.ContentType = core.HTTPMediaType{} }},
		{name: "above native single PutObject limit", change: func(r *S3FileUpload) { r.Integrity.Length, _ = core.NewByteLength(AmazonS3PutObjectMaximumBytes + 1) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			r := s3FileFixture(t, dir, []byte("sealed"))
			tc.change(&r)
			client, err := NewStandardClient()
			if err != nil {
				t.Fatal(err)
			}
			got, gotErr := UploadS3File(t.Context(), client, r)
			if got != (Transfer{}) || !errors.Is(gotErr, core.ErrObjectStoreContract) {
				t.Fatalf("request refusal = (%v,%v), want zero and typed contract", got, gotErr)
			}
		})
	}
	for _, endpoint := range []string{"http://s3.us-east-1.amazonaws.com", "https://example.com", "https://retention.s3.us-east-1.amazonaws.com", "https://s3.us-east-1.amazonaws.com/bucket", "https://s3.us-east-1.amazonaws.com?token=unowned", "https://s3.us-east-1.amazonaws.com?", "https://s3.us-east-1.amazonaws.com:8443", "https://s3-accelerate.amazonaws.com"} {
		t.Run(endpoint, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			r := s3FileFixture(t, dir, []byte("sealed"))
			var err error
			r.Endpoint, err = core.ParseHTTPEndpoint(endpoint)
			if err != nil {
				t.Fatal(err)
			}
			if err := r.Validate(); !errors.Is(err, core.ErrObjectStoreContract) {
				t.Fatalf("service origin %q validation=%v, want refusal before signing", endpoint, err)
			}
		})
	}
}

type s3CountedSource struct {
	reader *bytes.Reader
	reads  atomic.Int32
}

func (s *s3CountedSource) Read(data []byte) (int, error) { s.reads.Add(1); return s.reader.Read(data) }
func (s *s3CountedSource) Len() int                      { return s.reader.Len() }

func TestS3FileCredentialRefusalHasNoTransferEffects(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		data   []byte
		absent bool
		want   error
	}{
		{name: "unsupported token file", data: []byte(`{"access_key_id":"` + s3FixtureAccess + `","secret_access_key":"` + s3FixtureSecret + `","session_token":"do-not-render"}`), want: core.ErrObjectStoreContract},
		{name: "unsupported profile file", data: []byte(`{"profile":"operator"}`), want: core.ErrObjectStoreContract},
		{name: "malformed secret-bearing JSON", data: []byte(`{"access_key_id":"` + s3FixtureAccess + `","secret_access_key":"do-not-render"`), want: core.ErrJSONContract},
		{name: "native missing credential file", absent: true, want: os.ErrNotExist},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			r := s3FileFixture(t, dir, []byte("sealed"))
			if tc.absent {
				if err := os.Remove(r.CredentialFile.String()); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(r.CredentialFile.String(), tc.data, 0600); err != nil {
				t.Fatal(err)
			}
			var calls atomic.Int32
			client := s3LocalProvider(t, http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				calls.Add(1)
				_, _ = io.Copy(io.Discard, request.Body)
				w.WriteHeader(http.StatusOK)
			}))
			source := &s3CountedSource{reader: bytes.NewReader([]byte("sealed"))}
			r.Source = source
			got, gotErr := UploadS3File(t.Context(), client, r)
			if !errors.Is(gotErr, core.ErrObjectStoreContract) {
				t.Fatalf("credential refusal error=%v, want stable object-store identity before diagnostic checks", gotErr)
			}
			if got != (Transfer{}) || !errors.Is(gotErr, tc.want) || calls.Load() != 0 || source.reads.Load() != 0 {
				t.Fatalf("credential refusal = (%v,%v), requests=%d source reads=%d, want zero/%v/no effects", got, gotErr, calls.Load(), source.reads.Load(), tc.want)
			}
			if text := gotErr.Error(); strings.Contains(text, "do-not-render") || strings.Contains(text, s3FixtureAccess) || strings.Contains(text, s3FixtureSecret) {
				t.Fatalf("credential failure diagnostic=%q, want no credential text", text)
			}
		})
	}
}
