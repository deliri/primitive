package gcsobjects

import (
	"bytes"
	jsonlegacy "encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	storageapi "google.golang.org/api/storage/v1"
)

func FuzzPublicGCSMediaResponseSemanticClosure(f *testing.F) {
	body := []byte("canonical public PDF fixture\n")
	integrity := gcsProviderIntegrity(f, body, body)
	recorder := httptest.NewRecorder()
	writeGCSObjectResponse(gcsObjectResponse{t: f, writer: recorder, integrity: integrity, metadata: storageapi.Object{
		ContentType: "application/pdf", Acl: []*storageapi.ObjectAccessControl{{Entity: "allUsers", Role: "READER"}},
	}})
	f.Add(recorder.Body.Bytes())
	f.Add([]byte("null"))
	f.Add([]byte("{}"))
	f.Add([]byte("<html>provider error</html>"))
	f.Fuzz(func(t *testing.T, data []byte) {
		client := bucketTestClient(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, received, err := receiveGCSMultipart(request)
			if err != nil || received != integrity {
				t.Errorf("fuzz request integrity = (%v,%v), want %v and nil", received, err, integrity)
				writer.WriteHeader(400)
				return
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write(data)
		}))
		contentType, err := core.ParseHTTPMediaType("application/pdf")
		if err != nil {
			t.Fatal(err)
		}
		request := GCSMediaUpload{Source: bytes.NewReader(body), Bucket: parsedGCSBucket(t, gcsProviderBucketText), Name: parsedGCSObjectName(t, gcsProviderObjectText), ContentType: contentType, Integrity: integrity, CustomTime: temporal.InstantFromNanoseconds(gcsProviderInstantNanos)}
		if err := request.Validate(); err != nil {
			t.Fatal(err)
		}
		got, gotErr := UploadMedia(t.Context(), client, request)
		if gotErr != nil {
			if got != (GCSObjectMetadata{}) || (!errors.Is(gotErr, core.ErrObjectStoreContract) && !errors.Is(gotErr, core.ErrObjectStoreIntegrity)) {
				t.Fatalf("response refusal = (%v,%v), want zero and typed object-store error", got, gotErr)
			}
			return
		}
		if got.Validate() != nil || got.Bucket() != request.Bucket || got.Name() != request.Name || got.Length() != integrity.Length || got.CRC32C() != integrity.CRC32C || got.ContentType() != contentType || got.CacheControl() != request.CacheControl || got.CustomTime() != request.CustomTime {
			t.Fatalf("accepted metadata = %v, want exact typed request agreement", got)
		}
		var provider storageapi.Object
		if err := jsonlegacy.Unmarshal(data, &provider); err != nil {
			t.Fatalf("accepted provider JSON error = %v, want nil", err)
		}
		generation, err := got.Generation().Int64()
		if err != nil || generation != provider.Generation {
			t.Fatalf("generation = (%d,%v), want provider generation %d", generation, err, provider.Generation)
		}
		publicRead := false
		for _, entry := range provider.Acl {
			if entry == nil {
				t.Fatalf("accepted ACL = %v, want no null entry", provider.Acl)
			}
			if entry.Entity == "allUsers" && entry.Role == "READER" {
				publicRead = true
			}
		}
		if !publicRead {
			t.Fatalf("accepted ACL = %v, want explicit public object read", provider.Acl)
		}
	})
}

func FuzzPublicGCSCopyExactBodySemanticClosure(f *testing.F) {
	canonical := []byte("canonical public PDF fixture\n")
	f.Add(canonical)
	f.Add([]byte{})
	f.Add(append(bytes.Clone(canonical), 'x'))
	f.Add(canonical[:len(canonical)-1])
	f.Fuzz(func(t *testing.T, data []byte) {
		client := publicCopyFixtureClient(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.Header.Get("Authorization") != "" || request.Header.Get("Cookie") != "" {
				t.Errorf("public request has credentials, want none")
				writer.WriteHeader(400)
				return
			}
			writer.Header().Set("Content-Type", "application/pdf")
			_, _ = writer.Write(data)
		}))
		request := publicCopyFixtureRequest(t, canonical)
		if err := request.Validate(); err != nil {
			t.Fatal(err)
		}
		got, gotErr := VerifyPublicGCSObject(t.Context(), client, request)
		if bytes.Equal(data, canonical) {
			want, err := ObjectAddress(request.Bucket, request.Name)
			if err != nil || gotErr != nil || got != want {
				t.Fatalf("canonical copy = (%v,%v), want %v and nil", got, gotErr, want)
			}
			return
		}
		if !errors.Is(gotErr, core.ErrObjectStoreIntegrity) || got != (core.HTTPEndpoint{}) {
			t.Fatalf("mutated body = (%v,%v), want zero and typed integrity refusal", got, gotErr)
		}
	})
}
