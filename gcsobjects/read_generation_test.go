package gcsobjects

import (
	"bytes"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// Local HTTP provider, real official SDK and private-stage lifecycle. This is
// generation-binding proof, not deployed GCS/IAM or execution acceptance.
func TestGCSReadGenerationRefusalLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		generation int64
		wantErr    error
		wantCalls  bool
	}{
		{name: "pinned generation creates exact verified private stage", generation: gcsProviderGeneration, wantCalls: true},
		{name: "absent generation never contacts provider", wantErr: core.ErrObjectStoreContract},
		{name: "identical bytes from foreign generation issue no stage", generation: gcsProviderGeneration + 1, wantErr: core.ErrObjectStoreIntegrity, wantCalls: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			var calls atomic.Int32
			client := bucketTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.URL.Query().Get("alt") != "json" && r.URL.Query().Get("generation") != strconv.FormatInt(tc.generation, 10) {
					t.Errorf("SDK generation = %q, want %d", r.URL.Query().Get("generation"), tc.generation)
				}
				if r.URL.Query().Get("alt") == "json" {
					writeGCSAttrsResponse(t, w, gcsProviderPayload)
					return
				}
				writeGCSMediaResponseFraming(t, w, gcsProviderPayload, false)
			}))
			destination, _ := gcsReadStageDestination(t, dir, uint64(len(gcsProviderPayload)))
			var generation GCSGeneration
			if tc.generation != 0 {
				var err error
				generation, err = NewGCSGeneration(tc.generation)
				if err != nil {
					t.Fatalf("NewGCSGeneration = %v, want nil", err)
				}
			}
			got, err := ReadGCSObject(t.Context(), client, GCSReadRequest{Bucket: parsedGCSBucket(t, gcsProviderBucketText), Name: parsedGCSObjectName(t, gcsProviderObjectText), Generation: generation, Destination: destination, Integrity: gcsProviderIntegrity(t, gcsProviderPayload, gcsProviderPayload)})
			if tc.wantErr == nil {
				content, readErr := os.ReadFile(filepath.Join(dir, destination.Temporary.Path.String()))
				if err != nil || got.Validate() != nil || readErr != nil || !bytes.Equal(content, gcsProviderPayload) || calls.Load() == 0 {
					t.Fatalf("pinned read = (%v, %v), bytes match=%t, read error=%v, calls=%d, want verified exact bytes", got, err, bytes.Equal(content, gcsProviderPayload), readErr, calls.Load())
				}
				metadata, metaErr := got.Metadata()
				if metaErr != nil || metadata.Generation() != generation {
					t.Fatalf("metadata generation = %v/%v, want %v/nil", metadata.Generation(), metaErr, generation)
				}
				stage, stageErr := got.Staged()
				if stageErr != nil {
					t.Fatalf("Staged = %v, want nil", stageErr)
				}
				if err := filestore.Discard(t.Context(), stage); err != nil {
					t.Fatalf("Discard = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) || got != (GCSReadResult{}) || (calls.Load() > 0) != tc.wantCalls {
				t.Fatalf("generation read = (%v, %v), calls = %d, want zero/%v and provider contacted=%t", got, err, calls.Load(), tc.wantErr, tc.wantCalls)
			}
			if _, err := os.Stat(filepath.Join(dir, destination.Temporary.Path.String())); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("rejected stage stat = %v, want %v", err, os.ErrNotExist)
			}
		})
	}
}
