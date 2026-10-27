package gcsobjects

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"net/http"
	"runtime"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	storageapi "google.golang.org/api/storage/v1"
)

func TestGCSMediaUploadObjectScopeLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		body    []byte
		status  int
		omitACL bool
		nullACL bool
		wantErr error
	}{
		{name: "public bytes commit with only an object grant", body: []byte("public legal document\n")},
		{name: "empty served object still has an explicit object grant"},
		{name: "provider ACL refusal leaves no publication proof", body: []byte("refused"), status: http.StatusForbidden, wantErr: core.ErrObjectStoreDestination},
		{name: "uniform bucket access refuses rather than granting bucket IAM", body: []byte("uniform"), status: http.StatusBadRequest, wantErr: core.ErrObjectStoreDestination},
		{name: "existing object is a conflict without an access mutation", body: []byte("existing"), status: http.StatusPreconditionFailed, wantErr: core.ErrObjectStoreConflict},
		{name: "missing public ACL acknowledgement cannot become proof", body: []byte("unconfirmed"), omitACL: true, wantErr: core.ErrObjectStoreDestination},
		{name: "null upload ACL entry is refused without a process panic", body: []byte("malformed"), nullACL: true, wantErr: core.ErrObjectStoreContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int64
			integrity := gcsProviderIntegrity(t, tc.body, tc.body)
			client := bucketTestClient(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				calls.Add(1)
				if request.Method != http.MethodPost || request.URL.Query().Get("predefinedAcl") != "publicRead" || request.URL.Query().Get("ifGenerationMatch") != "0" {
					t.Errorf("publication request = %s %s, want one publicRead object create with generation zero and no bucket IAM", request.Method, request.URL.String())
					writer.WriteHeader(http.StatusForbidden)
					return
				}
				metadata, gotIntegrity, err := receiveGCSMultipart(request)
				if err != nil || gotIntegrity != integrity || metadata.Name != gcsProviderObjectText {
					t.Errorf("upload declaration = (%v, %v, %v), want exact object and integrity %v", metadata, gotIntegrity, err, integrity)
					writer.WriteHeader(http.StatusBadRequest)
					return
				}
				if tc.status != 0 {
					writeGoogleAPIError(writer, tc.status)
					return
				}
				if !tc.omitACL {
					metadata.Acl = []*storageapi.ObjectAccessControl{{Entity: "allUsers", Role: "READER"}}
				}
				if tc.nullACL {
					metadata.Acl = append(metadata.Acl, nil)
				}
				writeGCSObjectResponse(gcsObjectResponse{t: t, writer: writer, metadata: metadata, integrity: integrity})
			}))
			mediaType, err := core.ParseHTTPMediaType("application/pdf")
			if err != nil {
				t.Fatal(err)
			}
			got, gotErr := UploadMedia(t.Context(), client, GCSMediaUpload{Source: bytes.NewReader(tc.body), Bucket: parsedGCSBucket(t, gcsProviderBucketText), Name: parsedGCSObjectName(t, gcsProviderObjectText), ContentType: mediaType, Integrity: integrity, CustomTime: temporal.InstantFromNanoseconds(gcsProviderInstantNanos)})
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got != (GCSObjectMetadata{}) {
					t.Fatalf("UploadMedia = (%v, %v), want zero and %v", got, gotErr, tc.wantErr)
				}
			} else if gotErr != nil || got.Validate() != nil || got.Length() != integrity.Length || got.Name().String() != gcsProviderObjectText {
				t.Fatalf("UploadMedia = (%v, %v), want exact validated object", got, gotErr)
			}
			if gotCalls := calls.Load(); gotCalls != 1 {
				t.Fatalf("provider calls = %d, want one object create and zero bucket requests", gotCalls)
			}
		})
	}
}

func TestGCSObjectPublicReadGrantLayerTriad(t *testing.T) {
	t.Parallel()
	owner := storageapi.ObjectAccessControl{Entity: "user-owner@example.test", Role: "OWNER"}
	reader := storageapi.ObjectAccessControl{Entity: "group-private@example.test", Role: "READER"}
	public := storageapi.ObjectAccessControl{Entity: "allUsers", Role: "READER"}
	cases := []struct {
		name                    string
		before                  []storageapi.ObjectAccessControl
		readStatus              int
		patchStatus             int
		foreignRead             bool
		missingMetageneration   bool
		nullACLEntry            bool
		foreignAfter            bool
		droppedOwner            bool
		unchangedMetageneration bool
		wantPatches             int64
		wantChanged             bool
		wantErr                 error
	}{
		{name: "grant preserves owner and unrelated private group", before: []storageapi.ObjectAccessControl{owner, reader}, wantPatches: 1, wantChanged: true},
		{name: "already public observes without rewriting access", before: []storageapi.ObjectAccessControl{owner, reader, public}},
		{name: "provider absent identity remains absent", readStatus: http.StatusNotFound, wantErr: core.ErrObjectStoreAbsent},
		{name: "provider permission refusal does not attempt update", readStatus: http.StatusForbidden, wantErr: core.ErrObjectStoreSource},
		{name: "foreign generation cannot authorize an update", before: []storageapi.ObjectAccessControl{owner}, foreignRead: true, wantErr: core.ErrObjectStoreIntegrity},
		{name: "missing metadata generation cannot authorize a CAS", before: []storageapi.ObjectAccessControl{owner}, missingMetageneration: true, wantErr: core.ErrObjectStoreContract},
		{name: "unobserved empty ACL cannot erase existing permissions", wantErr: core.ErrObjectStoreContract},
		{name: "null ACL entry from provider is a typed refusal", before: []storageapi.ObjectAccessControl{owner}, nullACLEntry: true, wantErr: core.ErrObjectStoreContract},
		{name: "duplicate ACL entity refuses before update", before: []storageapi.ObjectAccessControl{owner, owner}, wantErr: core.ErrObjectStoreContract},
		{name: "unknown ACL role is not silently normalized", before: []storageapi.ObjectAccessControl{{Entity: owner.Entity, Role: "FUTURE"}}, wantErr: core.ErrObjectStoreContract},
		{name: "public write permission is not silently downgraded", before: []storageapi.ObjectAccessControl{owner, {Entity: public.Entity, Role: "OWNER"}}, wantErr: core.ErrObjectStoreContract},
		{name: "empty entity is not submitted to the provider", before: []storageapi.ObjectAccessControl{{Role: "READER"}}, wantErr: core.ErrObjectStoreContract},
		{name: "concurrent ACL change refuses the metadata CAS", before: []storageapi.ObjectAccessControl{owner, reader}, patchStatus: http.StatusPreconditionFailed, wantPatches: 1, wantErr: core.ErrObjectStoreConflict},
		{name: "object policy refusal preserves destination cause", before: []storageapi.ObjectAccessControl{owner}, patchStatus: http.StatusForbidden, wantPatches: 1, wantErr: core.ErrObjectStoreDestination},
		{name: "foreign updated generation releases no grant", before: []storageapi.ObjectAccessControl{owner}, foreignAfter: true, wantPatches: 1, wantErr: core.ErrObjectStoreIntegrity},
		{name: "provider dropping unrelated owner releases no grant", before: []storageapi.ObjectAccessControl{owner, reader}, droppedOwner: true, wantPatches: 1, wantErr: core.ErrObjectStoreConflict},
		{name: "unchanged metadata generation cannot confirm a mutation", before: []storageapi.ObjectAccessControl{owner}, unchangedMetageneration: true, wantPatches: 1, wantErr: core.ErrObjectStoreConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var reads, patches atomic.Int64
			integrity := gcsProviderIntegrity(t, gcsProviderPayload, gcsProviderPayload)
			client := bucketTestClient(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/storage/v1/b/"+gcsProviderBucketText+"/o/"+gcsProviderObjectText || request.URL.Query().Get("generation") != strconv.FormatInt(gcsProviderGeneration, 10) {
					t.Errorf("ACL scope = %s %s, want exact object generation without bucket IAM", request.Method, request.URL.String())
					writer.WriteHeader(http.StatusBadRequest)
					return
				}
				metadata := storageapi.Object{Metageneration: 17}
				for _, entry := range tc.before {
					entry := entry
					metadata.Acl = append(metadata.Acl, &entry)
				}
				if tc.nullACLEntry {
					metadata.Acl = append(metadata.Acl, nil)
				}
				if request.Method == http.MethodGet {
					reads.Add(1)
					if tc.readStatus != 0 {
						writeGoogleAPIError(writer, tc.readStatus)
						return
					}
					if tc.missingMetageneration {
						metadata.Metageneration = 0
					}
				} else if request.Method == http.MethodPatch {
					patches.Add(1)
					if got := request.URL.Query().Get("ifMetagenerationMatch"); got != "17" {
						t.Errorf("CAS = %q, want 17", got)
					}
					var update storageapi.Object
					if err := json.UnmarshalRead(request.Body, &update); err != nil {
						t.Errorf("ACL update decode = %v, want nil", err)
						writer.WriteHeader(400)
						return
					}
					if len(update.Acl) != len(tc.before)+1 {
						t.Errorf("updated entries = %d, want %d", len(update.Acl), len(tc.before)+1)
					}
					for index, prior := range tc.before {
						if index >= len(update.Acl) || update.Acl[index].Entity != prior.Entity || update.Acl[index].Role != prior.Role {
							t.Errorf("preserved ACL entry %d = %v, want %v", index, update.Acl, prior)
						}
					}
					if tc.patchStatus != 0 {
						writeGoogleAPIError(writer, tc.patchStatus)
						return
					}
					metadata = update
					metadata.Metageneration = 18
					if tc.unchangedMetageneration {
						metadata.Metageneration = 17
					}
					if tc.droppedOwner {
						metadata.Acl = metadata.Acl[1:]
					}
				} else {
					t.Errorf("method = %s, want GET or PATCH", request.Method)
					writer.WriteHeader(405)
					return
				}
				if (tc.foreignRead && request.Method == http.MethodGet) || (tc.foreignAfter && request.Method == http.MethodPatch) {
					metadata.Generation = gcsProviderGeneration + 1
					// The shared metadata producer pins generation; emit the changed
					// typed provider fact after that producer's complete projection.
					metadata.Bucket, metadata.Name = gcsProviderBucketText, gcsProviderObjectText
					metadata.Size = integrity.Length.Uint64()
					metadata.Crc32c, _ = integrity.CRC32C.Base64()
					metadata.ContentType = gcsProviderMediaTypeText
					instant, _ := temporal.InstantFromNanoseconds(gcsProviderInstantNanos).Time()
					metadata.TimeCreated, metadata.Updated, metadata.CustomTime = instant.Format("2006-01-02T15:04:05Z07:00"), instant.Format("2006-01-02T15:04:05Z07:00"), instant.Format("2006-01-02T15:04:05Z07:00")
					writeProviderJSON(t, writer, metadata)
					return
				}
				writeGCSObjectResponse(gcsObjectResponse{t: t, writer: writer, metadata: metadata, integrity: integrity})
			}))
			generation, err := NewGCSGeneration(gcsProviderGeneration)
			if err != nil {
				t.Fatal(err)
			}
			got, gotErr := GrantGCSObjectPublicRead(t.Context(), client, GCSObjectPublicReadRequest{Bucket: parsedGCSBucket(t, gcsProviderBucketText), Name: parsedGCSObjectName(t, gcsProviderObjectText), Generation: generation})
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got != (GCSObjectPublicReadGrant{}) {
					t.Fatalf("GrantGCSObjectPublicRead = (%v, %v), want zero and %v", got, gotErr, tc.wantErr)
				}
				if tc.nullACLEntry {
					if _, present := errors.AsType[runtime.Error](gotErr); !present {
						t.Fatalf("malformed ACL cause = %v, want preserved runtime.Error", gotErr)
					}
				}
			} else {
				changed, changeErr := got.Changed()
				metadata, metadataErr := got.Metadata()
				if gotErr != nil || changeErr != nil || metadataErr != nil || changed != tc.wantChanged || metadata.Generation() != generation {
					t.Fatalf("grant = (%v, %v, %v, %v, %v), want exact generation and changed=%t", got, gotErr, changed, changeErr, metadataErr, tc.wantChanged)
				}
			}
			if gotReads, gotPatches := reads.Load(), patches.Load(); gotReads != 1 || gotPatches != tc.wantPatches {
				t.Fatalf("provider calls = (%d,%d), want one metadata read and %d object CAS updates", gotReads, gotPatches, tc.wantPatches)
			}
		})
	}
}

func TestGCSObjectPublicationRejectsInvalidIngressBeforeEffect(t *testing.T) {
	t.Parallel()
	var calls atomic.Int64
	client := bucketTestClient(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	generation, err := NewGCSGeneration(gcsProviderGeneration)
	if err != nil {
		t.Fatal(err)
	}
	valid := GCSObjectPublicReadRequest{Bucket: parsedGCSBucket(t, gcsProviderBucketText), Name: parsedGCSObjectName(t, gcsProviderObjectText), Generation: generation}
	for _, missing := range []string{"bucket", "name", "generation"} {
		request := valid
		switch missing {
		case "bucket":
			request.Bucket = GCSBucket{}
		case "name":
			request.Name = GCSObjectName{}
		case "generation":
			request.Generation = GCSGeneration{}
		}
		got, gotErr := GrantGCSObjectPublicRead(t.Context(), client, request)
		if !errors.Is(gotErr, core.ErrObjectStoreContract) || got != (GCSObjectPublicReadGrant{}) {
			t.Fatalf("unset %s = (%v,%v), want typed refusal without a grant", missing, got, gotErr)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got, gotErr := GrantGCSObjectPublicRead(ctx, client, valid)
	if !errors.Is(gotErr, context.Canceled) || got != (GCSObjectPublicReadGrant{}) {
		t.Fatalf("cancelled = (%v,%v), want preserved cancellation and zero", got, gotErr)
	}
	if gotCalls := calls.Load(); gotCalls != 0 {
		t.Fatalf("invalid ingress calls = %d, want zero", gotCalls)
	}
}
