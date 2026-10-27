package gcsobjects

import (
	"bytes"
	"context"
	jsonlegacy "encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	storageapi "google.golang.org/api/storage/v1"
)

func TestGCSStoredFileObjectScopeLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		empty         bool
		nullACL       bool
		ownerACL      bool
		defaultPublic bool
		omitTime      bool
		status        int
		cancel        bool
		wantErr       error
	}{
		{name: "positive private file sends no access grant"},
		{name: "neutral empty file still preserves private access", empty: true},
		{name: "positive provider owner ACL remains provider owned", ownerACL: true},
		{name: "positive provider default ACL remains untouched", defaultPublic: true},
		{name: "neutral absent optional private custom time remains absent", omitTime: true},
		{name: "negative null stored ACL cannot kill SDK process", nullACL: true, wantErr: core.ErrObjectStoreContract},
		{name: "negative provider unavailable is one attempt", status: http.StatusServiceUnavailable, wantErr: core.ErrObjectStoreDestination},
		{name: "neutral cancellation causes zero provider effects", cancel: true, wantErr: context.Canceled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := []byte("private exact file\n")
			if tc.empty {
				body = nil
			}
			integrity := gcsProviderIntegrity(t, body, body)
			var calls atomic.Int64
			client := bucketTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != http.MethodPost || r.URL.Query().Get("predefinedAcl") != "" || r.URL.Query().Get("ifGenerationMatch") != "0" {
					t.Errorf("stored-file request = %s %s, want private create-only with no ACL or IAM change", r.Method, r.URL.String())
					w.WriteHeader(400)
					return
				}
				metadata, got, err := receiveGCSMultipart(r)
				if err != nil || got != integrity {
					t.Errorf("stored-file declaration = %v/%v, want exact integrity%v", got, err, integrity)
					w.WriteHeader(400)
					return
				}
				if tc.status != 0 {
					w.WriteHeader(tc.status)
					return
				}
				if tc.ownerACL {
					metadata.Acl = []*storageapi.ObjectAccessControl{{Entity: "user-owner@example.test", Role: "OWNER"}}
				}
				if tc.defaultPublic {
					metadata.Acl = []*storageapi.ObjectAccessControl{{Entity: "allUsers", Role: "READER"}}
				}
				if tc.nullACL {
					metadata.Acl = []*storageapi.ObjectAccessControl{nil}
				}
				recorder := httptest.NewRecorder()
				writeGCSObjectResponse(gcsObjectResponse{t: t, writer: recorder, metadata: metadata, integrity: integrity})
				if tc.omitTime {
					var projected storageapi.Object
					if err := jsonlegacy.Unmarshal(recorder.Body.Bytes(), &projected); err != nil {
						t.Error(err)
						w.WriteHeader(500)
						return
					}
					projected.CustomTime = ""
					encoded, err := jsonlegacy.Marshal(projected)
					if err != nil {
						t.Error(err)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(encoded)
					return
				}
				for k, values := range recorder.Header() {
					for _, v := range values {
						w.Header().Add(k, v)
					}
				}
				_, _ = w.Write(recorder.Body.Bytes())
			}))
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancel {
				cancel()
			}
			got, gotErr := UploadFile(ctx, client, GCSFileUpload{Source: bytes.NewReader(body), Bucket: parsedGCSBucket(t, gcsProviderBucketText), Name: parsedGCSObjectName(t, gcsProviderObjectText), Integrity: integrity, CustomTime: temporal.InstantFromNanoseconds(gcsProviderInstantNanos)})
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got != (GCSObjectMetadata{}) {
					t.Fatalf("UploadFile = (%v,%v), want zero and %v", got, gotErr, tc.wantErr)
				}
			} else if gotErr != nil || got.Validate() != nil || got.Length() != integrity.Length || got.CRC32C() != integrity.CRC32C {
				t.Fatalf("UploadFile = (%v,%v), want exact metadata", got, gotErr)
			}
			if tc.omitTime && got.CustomTime() != (temporal.Instant{}) {
				t.Fatalf("optional CustomTime = %v, want unset provider fact", got.CustomTime())
			}
			wantCalls := int64(1)
			if tc.cancel {
				wantCalls = 0
			}
			if calls.Load() != wantCalls {
				t.Fatalf("provider calls = %d, want %d", calls.Load(), wantCalls)
			}
		})
	}
}

func FuzzStoredGCSFileResponseSemanticClosure(f *testing.F) {
	body := []byte("canonical private file fixture\n")
	integrity := gcsProviderIntegrity(f, body, body)
	recorder := httptest.NewRecorder()
	writeGCSObjectResponse(gcsObjectResponse{t: f, writer: recorder, integrity: integrity, metadata: storageapi.Object{ContentType: core.HTTPMediaTypeOctetStream().String()}})
	f.Add(recorder.Body.Bytes())
	f.Add([]byte("null"))
	f.Add([]byte("{}"))
	f.Add([]byte("<html>provider refusal</html>"))
	f.Fuzz(func(t *testing.T, data []byte) {
		client := bucketTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, observed, err := receiveGCSMultipart(r)
			if err != nil || observed != integrity || r.URL.Query().Get("predefinedAcl") != "" {
				t.Errorf("private fuzz source/ACL = %v/%v/%q, want exact bytes and no access grant", observed, err, r.URL.Query().Get("predefinedAcl"))
				w.WriteHeader(400)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(data)
		}))
		request := GCSFileUpload{Source: bytes.NewReader(body), Bucket: parsedGCSBucket(t, gcsProviderBucketText), Name: parsedGCSObjectName(t, gcsProviderObjectText), Integrity: integrity, CustomTime: temporal.InstantFromNanoseconds(gcsProviderInstantNanos)}
		if err := request.Validate(); err != nil {
			t.Fatal(err)
		}
		got, gotErr := UploadFile(t.Context(), client, request)
		if gotErr != nil {
			if got != (GCSObjectMetadata{}) || (!errors.Is(gotErr, core.ErrObjectStoreContract) && !errors.Is(gotErr, core.ErrObjectStoreIntegrity)) {
				t.Fatalf("private response refusal = (%v,%v), want zero and stable object-store cause", got, gotErr)
			}
			return
		}
		var provider storageapi.Object
		if err := jsonlegacy.Unmarshal(data, &provider); err != nil {
			t.Fatalf("accepted provider = %v, want decodable typed object", err)
		}
		generation, err := got.Generation().Int64()
		if err != nil || got.Validate() != nil || got.Bucket() != request.Bucket || got.Name() != request.Name || got.ContentType() != core.HTTPMediaTypeOctetStream() || got.Length() != integrity.Length || got.CRC32C() != integrity.CRC32C || got.CacheControl() != (GCSCacheControl{}) || generation != provider.Generation {
			t.Fatalf("accepted private metadata = %v/%v, want exact source/identity/generation", got, err)
		}
		for _, acl := range provider.Acl {
			if acl == nil {
				t.Fatalf("accepted private ACL = %v, want no null entry", provider.Acl)
			}
		}
		if provider.CustomTime == "" {
			if got.CustomTime() != (temporal.Instant{}) {
				t.Fatalf("private optional time = %v, want absent", got.CustomTime())
			}
		} else {
			want, err := temporal.ParseRFC3339(provider.CustomTime)
			if err != nil || got.CustomTime() != want {
				t.Fatalf("private provider time = %v/%v, want exact %v", got.CustomTime(), err, want)
			}
		}
	})
}
