package gcsobjects

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/temporal"
	"google.golang.org/api/googleapi"
)

// The official SDK must not replay an unknown-length source after the provider
// consumed it. A failed acknowledgement leaves zero retention proof even when
// every source byte reached the provider; absence of a remote object is not a
// cancellation or transport-error guarantee.
func TestGCSFileStreamProviderFailureLayerTriad(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name           string
		status         int
		closeTransport bool
		payload        []byte
		wantErr        error
	}{
		{name: "an acknowledged consumed source earns exact retention proof", payload: []byte("acknowledged source")},
		{name: "a retryable provider refusal cannot replay the consumed source", status: http.StatusServiceUnavailable, payload: []byte("consumed before refusal"), wantErr: core.ErrObjectStoreDestination},
		{name: "a lost response after source consumption leaves no proof", closeTransport: true, payload: []byte("consumed before connection close"), wantErr: core.ErrObjectStoreDestination},
		{name: "an empty source is not verified by a failed acknowledgement", status: http.StatusServiceUnavailable, wantErr: core.ErrObjectStoreDestination},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			consumed := make(chan objectstore.Integrity, 2)
			client := bucketTestClient(t, http.HandlerFunc(func(out http.ResponseWriter, request *http.Request) {
				calls.Add(1)
				metadata, integrity, err := receiveGCSMultipart(request)
				if err != nil {
					t.Errorf("provider source decode=%v, want the complete declared stream", err)
					out.WriteHeader(http.StatusBadRequest)
					return
				}
				select {
				case consumed <- integrity:
				default:
					t.Errorf("provider completions=%d, want one consumed stream", calls.Load())
				}
				if row.closeTransport {
					hijacker, ok := out.(http.Hijacker)
					if !ok {
						t.Errorf("provider writer=%T, want native HTTP connection custody", out)
						out.WriteHeader(http.StatusInternalServerError)
						return
					}
					connection, _, err := hijacker.Hijack()
					if err != nil {
						t.Errorf("provider connection custody=%v, want nil", err)
						return
					}
					if err := connection.Close(); err != nil {
						t.Errorf("provider connection close=%v, want nil", err)
					}
					return
				}
				if row.status != 0 {
					out.WriteHeader(row.status)
					return
				}
				writeGCSObjectResponse(gcsObjectResponse{t: t, writer: out, metadata: metadata, integrity: integrity})
			}))
			duration, err := temporal.DurationFromSeconds(30)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: t.Context(), Duration: duration})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(cancel)
			got, err := UploadFileStream(ctx, client, GCSFileStreamUpload{
				Source: bytes.NewReader(row.payload), Bucket: parsedGCSBucket(t, gcsProviderBucketText),
				Name: parsedGCSObjectName(t, gcsProviderObjectText), CustomTime: temporal.InstantFromNanoseconds(gcsProviderInstantNanos),
			})
			if row.wantErr != nil {
				if !errors.Is(err, row.wantErr) || got != (GCSFileStreamResult{}) {
					t.Fatalf("failed acknowledgement=%+v/%v, want zero proof and %v", got, err, row.wantErr)
				}
				if row.closeTransport && !errors.Is(err, io.EOF) {
					t.Fatalf("lost-response cause=%v, want native %v retained", err, io.EOF)
				}
				if row.status != 0 {
					provider, ok := errors.AsType[*googleapi.Error](err)
					if !ok || provider.Code != row.status {
						t.Fatalf("provider refusal=%v, want native HTTP status %d", err, row.status)
					}
				}
			} else {
				integrity, integrityErr := got.Integrity()
				want := gcsProviderIntegrity(t, row.payload, row.payload)
				if err != nil || integrityErr != nil || integrity != want {
					t.Fatalf("acknowledged source=%v/%v/%v, want %v and nil errors", integrity, integrityErr, err, want)
				}
			}
			if count := calls.Load(); count != 1 {
				t.Fatalf("provider attempts=%d, want exactly one attempt without replay", count)
			}
			select {
			case observed := <-consumed:
				want := gcsProviderIntegrity(t, row.payload, row.payload)
				if observed != want {
					t.Fatalf("provider consumed=%v, want complete source %v", observed, want)
				}
			default:
				t.Fatal("provider consumed no source, want the complete source before acknowledgement")
			}
		})
	}
}
