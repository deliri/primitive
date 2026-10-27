package gcsobjects

import (
	"bytes"
	"context"
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

type streamRefusalAfterProgress struct {
	ctx      context.Context
	progress <-chan struct{}
	panics   bool
}

func (r streamRefusalAfterProgress) Read([]byte) (int, error) {
	select {
	case <-r.progress:
		if r.panics {
			panic(io.ErrUnexpectedEOF)
		}
		return 0, io.ErrUnexpectedEOF
	case <-r.ctx.Done():
		return 0, context.Cause(r.ctx)
	}
}

func TestGCSFileStreamProducerRefusalCannotSealPrefixLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		panics  bool
		wantErr error
	}{
		{name: "native source failure after provider progress", wantErr: io.ErrUnexpectedEOF},
		{name: "panicking source after provider progress", panics: true, wantErr: core.ErrFilestoreCallbackPanic},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			duration, err := temporal.DurationFromSeconds(30)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: t.Context(), Duration: duration})
			if err != nil {
				t.Fatal(err)
			}
			defer cancel()
			progress := make(chan struct{})
			provider := make(chan error, 1)
			client := bucketTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				request.Body = &streamProgressBody{body: request.Body, progress: progress}
				metadata, integrity, err := receiveGCSMultipart(request)
				provider <- err
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				writeGCSObjectResponse(gcsObjectResponse{t: t, writer: w, metadata: metadata, integrity: integrity})
			}))
			prefix := bytes.Repeat([]byte{'x'}, 2*gcsFileStreamBufferBytes)
			source := io.MultiReader(bytes.NewReader(prefix), streamRefusalAfterProgress{ctx: ctx, progress: progress, panics: tc.panics})
			got, gotErr := UploadFileStream(ctx, client, GCSFileStreamUpload{
				Source: source, Bucket: parsedGCSBucket(t, gcsProviderBucketText), Name: parsedGCSObjectName(t, gcsProviderObjectText),
				CustomTime: temporal.InstantFromNanoseconds(gcsProviderInstantNanos),
			})
			if got != (GCSFileStreamResult{}) || !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("progressed source refusal = (%v,%v), want zero and cause %v", got, gotErr, tc.wantErr)
			}
			select {
			case <-progress:
			default:
				t.Fatal("provider progress absent, want real streamed prefix before refusal")
			}
			select {
			case err := <-provider:
				if err == nil {
					t.Fatal("provider admitted complete multipart prefix, want interrupted request")
				}
			case <-ctx.Done():
				t.Fatalf("provider joined = %v, want interrupted request joined", context.Cause(ctx))
			}
		})
	}
}
