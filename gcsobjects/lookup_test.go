package gcsobjects

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	storageapi "google.golang.org/api/storage/v1"
)

// This drives the official SDK against a local provider, not a live bucket.
// Metadata is an observation only; callers must verify bytes before certifying them.
func TestLookupGCSObjectLayerTriadExactKeyAndNoEnumeration(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		mutate  func(*storageapi.Object)
		status  int
		wantErr error
	}{
		{name: "exact key discovers current generation"},
		{name: "absent key yields zero metadata", status: http.StatusNotFound, wantErr: core.ErrObjectStoreAbsent},
		{name: "denied metadata is not absence", status: http.StatusForbidden, wantErr: core.ErrObjectStoreSource},
		{name: "foreign bucket cannot satisfy known key", mutate: func(v *storageapi.Object) { v.Bucket = "foreign-bucket" }, wantErr: core.ErrObjectStoreIntegrity},
		{name: "foreign name cannot satisfy known key", mutate: func(v *storageapi.Object) { v.Name = "foreign/object" }, wantErr: core.ErrObjectStoreIntegrity},
		{name: "zero generation yields no snapshot", mutate: func(v *storageapi.Object) { v.Generation = 0 }, wantErr: core.ErrObjectStoreSource},
		{name: "negative generation yields no snapshot", mutate: func(v *storageapi.Object) { v.Generation = -1 }, wantErr: core.ErrObjectStoreSource},
		{name: "missing creation time yields no provenance", mutate: func(v *storageapi.Object) { v.TimeCreated = "" }, wantErr: core.ErrObjectStoreSource},
		{name: "missing update time yields no provenance", mutate: func(v *storageapi.Object) { v.Updated = "" }, wantErr: core.ErrObjectStoreSource},
		{name: "missing media type yields no nominal object", mutate: func(v *storageapi.Object) { v.ContentType = "" }, wantErr: core.ErrObjectStoreSource},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			integrity := observationIntegrity(t, []byte("exact object recovery"))
			want := observationProviderObject(t, integrity, 42)
			response := want
			if tc.mutate != nil {
				tc.mutate(&response)
			}
			var calls atomic.Int64
			client := bucketTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != http.MethodGet || r.URL.Path != "/storage/v1/b/"+gcsProviderBucketText+"/o/"+gcsProviderObjectText || r.URL.Query().Get("generation") != "" || r.URL.Query().Get("prefix") != "" {
					t.Errorf("lookup request = %s %s, want exact object GET without enumeration or selected generation", r.Method, r.URL)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if tc.status != 0 {
					writeGoogleAPIError(w, tc.status)
					return
				}
				writeProviderJSON(t, w, response)
			}))
			got, gotErr := LookupGCSObject(context.Background(), client, GCSObjectLookupRequest{
				Bucket: parsedGCSBucket(t, gcsProviderBucketText), Name: parsedGCSObjectName(t, gcsProviderObjectText),
			})
			if calls.Load() != 1 {
				t.Fatalf("provider calls = %d, want 1", calls.Load())
			}
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got != (GCSObjectMetadata{}) {
					t.Fatalf("LookupGCSObject() = (%v, %v), want zero and %v", got, gotErr, tc.wantErr)
				}
				return
			}
			generation, err := got.Generation().Int64()
			if gotErr != nil || err != nil || got.Validate() != nil || got.Bucket().String() != want.Bucket || got.Name().String() != want.Name || generation != want.Generation || got.Length() != integrity.Length || got.CRC32C() != integrity.CRC32C {
				t.Fatalf("LookupGCSObject() = (%v, %v, %v), want exact identity, generation 42 and integrity %v", got, gotErr, err, integrity)
			}
		})
	}
}

func TestLookupGCSObjectRejectsInvalidCallBeforeProvider(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		cancel     bool
		nilContext bool
		nilClient  bool
		zeroBucket bool
		zeroName   bool
		wantErr    error
	}{
		{name: "cancelled context performs no request", cancel: true, wantErr: context.Canceled},
		{name: "nil context performs no request", nilContext: true, wantErr: core.ErrObjectStoreContract},
		{name: "nil client performs no request", nilClient: true, wantErr: core.ErrObjectStoreContract},
		{name: "zero bucket performs no request", zeroBucket: true, wantErr: core.ErrObjectStoreContract},
		{name: "zero name performs no request", zeroName: true, wantErr: core.ErrObjectStoreContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int64
			client := bucketTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls.Add(1); w.WriteHeader(http.StatusBadRequest) }))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancel {
				cancel()
			}
			var callContext context.Context = ctx
			if tc.nilContext {
				callContext = nil
			}
			if tc.nilClient {
				client = nil
			}
			request := GCSObjectLookupRequest{Bucket: parsedGCSBucket(t, gcsProviderBucketText), Name: parsedGCSObjectName(t, gcsProviderObjectText)}
			if tc.zeroBucket {
				request.Bucket = GCSBucket{}
			}
			if tc.zeroName {
				request.Name = GCSObjectName{}
			}
			got, gotErr := LookupGCSObject(callContext, client, request)
			if !errors.Is(gotErr, tc.wantErr) || got != (GCSObjectMetadata{}) || calls.Load() != 0 {
				t.Fatalf("LookupGCSObject() = (%v, %v), calls %d, want zero, %v and no provider call", got, gotErr, calls.Load(), tc.wantErr)
			}
		})
	}
}

// Mutations reach the official SDK's provider decoder. Accepted results must
// preserve the requested identity and a positive exact generation; foreign
// identities and nonpositive generations must never yield metadata.
func FuzzLookupGCSObjectProviderIdentityAndGeneration(f *testing.F) {
	integrity := observationIntegrity(f, []byte("exact object recovery"))
	seed := observationProviderObject(f, integrity, 42)
	f.Add(seed.Bucket, seed.Name, seed.Generation)
	f.Add("foreign-bucket", seed.Name, seed.Generation)
	f.Add(seed.Bucket, "foreign/object", seed.Generation)
	f.Add(seed.Bucket, seed.Name, int64(0))
	f.Add(seed.Bucket, seed.Name, int64(-1))
	f.Add(seed.Bucket, seed.Name, int64(1))
	f.Add(seed.Bucket, seed.Name, int64(^uint64(0)>>1))
	f.Fuzz(func(t *testing.T, bucket, name string, generation int64) {
		// Secondary oracle work stays within the provider's bounded metadata field
		// surface; oversized-body pressure is covered by the SDK transport tests.
		if len(bucket) > 1024 || len(name) > 2048 {
			return
		}
		response := seed
		response.Bucket, response.Name, response.Generation = bucket, name, generation
		client := bucketTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { writeProviderJSON(t, w, response) }))
		got, gotErr := LookupGCSObject(context.Background(), client, GCSObjectLookupRequest{Bucket: parsedGCSBucket(t, seed.Bucket), Name: parsedGCSObjectName(t, seed.Name)})
		wantAccepted := bucket == seed.Bucket && name == seed.Name && generation > 0
		if !wantAccepted {
			if gotErr == nil || !errors.Is(gotErr, core.ErrObjectStoreSource) || got != (GCSObjectMetadata{}) {
				t.Fatalf("LookupGCSObject(mutated provider) = (%v, %v), want zero and typed source refusal", got, gotErr)
			}
			return
		}
		gotGeneration, err := got.Generation().Int64()
		if gotErr != nil || err != nil || got.Validate() != nil || gotGeneration != generation || got.Bucket().String() != bucket || got.Name().String() != name || got.Length() != integrity.Length || got.CRC32C() != integrity.CRC32C {
			t.Fatalf("LookupGCSObject(accepted provider) = (%v, %v, %v), want exact identity, generation %d and integrity %v", got, gotErr, err, generation, integrity)
		}
	})
}
