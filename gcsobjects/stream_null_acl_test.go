package gcsobjects

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	storageapi "google.golang.org/api/storage/v1"
)

func TestGCSFileStreamNullACLNativeRefusal(t *testing.T) {
	t.Parallel()
	payload := []byte("unknown final extent stored stream\n")
	client := bucketTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Query().Get("ifGenerationMatch") != "0" || r.URL.Query().Get("predefinedAcl") != "" {
			t.Errorf("stream upload request = %s %s, want private create-only", r.Method, r.URL.String())
			w.WriteHeader(400)
			return
		}
		metadata, integrity, err := receiveGCSMultipart(r)
		if err != nil || integrity.SHA256 != core.SHA256Of(payload) {
			t.Errorf("stream source = %v/%v, want exact payload", integrity, err)
			w.WriteHeader(400)
			return
		}
		metadata.Acl = []*storageapi.ObjectAccessControl{nil}
		writeGCSObjectResponse(gcsObjectResponse{t: t, writer: w, metadata: metadata, integrity: integrity})
	}))
	// Hide bytes.Reader's extent methods: the API observes one live-style read.
	source := struct{ io.Reader }{Reader: bytes.NewReader(payload)}
	got, gotErr := UploadFileStream(t.Context(), client, GCSFileStreamUpload{Source: source, Bucket: parsedGCSBucket(t, gcsProviderBucketText), Name: parsedGCSObjectName(t, gcsProviderObjectText), CustomTime: temporal.InstantFromNanoseconds(gcsProviderInstantNanos)})
	if got != (GCSFileStreamResult{}) || !errors.Is(gotErr, core.ErrObjectStoreContract) {
		t.Fatalf("UploadFileStream malformed ACL = (%v,%v), want zero and typed refusal", got, gotErr)
	}
}
