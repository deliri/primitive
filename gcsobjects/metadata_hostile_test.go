package gcsobjects

import (
	"context"
	"encoding/binary"
	"errors"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"google.golang.org/api/googleapi"
)

const metadataFixtureInstantNanos = 1_786_183_200_000_000_000

// TestMetadataFromGCSAttrsProjectsProviderEvidence exercises the read-back
// projection against real provider attributes without a live bucket. The
// served-versus-stored split means a stored file carries no cache directive, so
// an absent Cache-Control must project to sealed evidence rather than a
// malformed field. This is the offline proof of the UploadFile read path that
// the skipped live test cannot supply in ordinary CI.
func TestMetadataFromGCSAttrsProjectsProviderEvidence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		build           func(t *testing.T) *storage.ObjectAttrs
		wantErr         error
		name            string
		wantContentType string
		wantCache       string
	}{
		{
			name:            "served media attrs carry content type and a cache directive",
			build:           validMediaGCSAttrs,
			wantContentType: "image/webp",
			wantCache:       "private, no-store",
		},
		{
			name: "stored file attrs carry octet-stream and an absent cache directive",
			build: func(t *testing.T) *storage.ObjectAttrs {
				attrs := validMediaGCSAttrs(t)
				attrs.ContentType = "application/octet-stream"
				attrs.CacheControl = ""
				return attrs
			},
			wantContentType: "application/octet-stream",
			wantCache:       "",
		},
		{
			name: "signed upload attrs may carry no optional custom time",
			build: func(t *testing.T) *storage.ObjectAttrs {
				attrs := validMediaGCSAttrs(t)
				attrs.CustomTime = time.Time{}
				return attrs
			},
			wantContentType: "image/webp",
			wantCache:       "private, no-store",
		},
		{
			name:    "nil attrs are refused",
			build:   func(t *testing.T) *storage.ObjectAttrs { return nil },
			wantErr: core.ErrObjectStoreContract,
		},
		{
			name: "unset generation is refused",
			build: func(t *testing.T) *storage.ObjectAttrs {
				attrs := validMediaGCSAttrs(t)
				attrs.Generation = 0
				return attrs
			},
			wantErr: core.ErrObjectStoreContract,
		},
		{
			name: "negative size is refused as a size violation",
			build: func(t *testing.T) *storage.ObjectAttrs {
				attrs := validMediaGCSAttrs(t)
				attrs.Size = -1
				return attrs
			},
			wantErr: core.ErrObjectStoreSize,
		},
		{
			name: "invalid bucket name is refused",
			build: func(t *testing.T) *storage.ObjectAttrs {
				attrs := validMediaGCSAttrs(t)
				attrs.Bucket = "Invalid_UPPER"
				return attrs
			},
			wantErr: core.ErrObjectStoreContract,
		},
		{
			name: "reserved object name is refused",
			build: func(t *testing.T) *storage.ObjectAttrs {
				attrs := validMediaGCSAttrs(t)
				attrs.Name = ".."
				return attrs
			},
			wantErr: core.ErrObjectStoreContract,
		},
		{
			name: "malformed content type is refused",
			build: func(t *testing.T) *storage.ObjectAttrs {
				attrs := validMediaGCSAttrs(t)
				attrs.ContentType = "not a media type"
				return attrs
			},
			wantErr: core.ErrObjectStoreContract,
		},
		{
			name: "present but malformed cache directive is refused",
			build: func(t *testing.T) *storage.ObjectAttrs {
				attrs := validMediaGCSAttrs(t)
				attrs.CacheControl = "\x01bad"
				return attrs
			},
			wantErr: core.ErrObjectStoreContract,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := metadataFromGCSAttrs(tc.build(t))
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("metadataFromGCSAttrs() error = %v, want errors.Is(..., %v)", gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("metadataFromGCSAttrs() error = %v, want nil", gotErr)
			}
			if gotValidateErr := got.Validate(); gotValidateErr != nil {
				t.Fatalf("metadataFromGCSAttrs().Validate() error = %v, want nil", gotValidateErr)
			}
			if got.ContentType().String() != tc.wantContentType {
				t.Fatalf("metadata content type = %q, want %q", got.ContentType().String(), tc.wantContentType)
			}
			if got.CacheControl().String() != tc.wantCache {
				t.Fatalf("metadata cache directive = %q, want %q", got.CacheControl().String(), tc.wantCache)
			}
		})
	}
}

// TestProjectGCSErrorMapsProviderFailuresToStableIdentities proves the effect
// leaf translates real provider failures into the stable identities callers
// decide on, and preserves cancellation, without inventing an absence or
// conflict the provider did not report.
func TestProjectGCSErrorMapsProviderFailuresToStableIdentities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cause        error
		name         string
		operation    core.ErrorIdentity
		wantAbsent   bool
		wantConflict bool
		wantCancel   bool
	}{
		{name: "object-not-exist sentinel is absence", cause: storage.ErrObjectNotExist, operation: core.ErrObjectStoreSource, wantAbsent: true},
		{name: "provider 404 is absence", cause: &googleapi.Error{Code: 404}, operation: core.ErrObjectStoreDestination, wantAbsent: true},
		{name: "provider 409 is conflict", cause: &googleapi.Error{Code: 409}, operation: core.ErrObjectStoreDestination, wantConflict: true},
		{name: "provider 412 precondition is conflict", cause: &googleapi.Error{Code: 412}, operation: core.ErrObjectStoreDestination, wantConflict: true},
		{name: "provider 500 is neither absence nor conflict", cause: &googleapi.Error{Code: 500}, operation: core.ErrObjectStoreDestination},
		{name: "context cancellation is preserved", cause: context.Canceled, operation: core.ErrObjectStoreSource, wantCancel: true},
		{name: "deadline exceeded is preserved", cause: context.DeadlineExceeded, operation: core.ErrObjectStoreSource, wantCancel: true},
		{name: "opaque provider error carries only contract and operation", cause: errors.New("provider boom"), operation: core.ErrObjectStoreSource},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := projectGCSError(tc.cause, tc.operation)
			if !errors.Is(got, core.ErrObjectStoreContract) {
				t.Fatalf("projectGCSError() = %v, want the contract identity", got)
			}
			if !errors.Is(got, tc.operation) {
				t.Fatalf("projectGCSError() = %v, want the operation identity %v", got, tc.operation)
			}
			if gotAbsent := errors.Is(got, core.ErrObjectStoreAbsent); gotAbsent != tc.wantAbsent {
				t.Fatalf("projectGCSError() absence = %t, want %t", gotAbsent, tc.wantAbsent)
			}
			if gotConflict := errors.Is(got, core.ErrObjectStoreConflict); gotConflict != tc.wantConflict {
				t.Fatalf("projectGCSError() conflict = %t, want %t", gotConflict, tc.wantConflict)
			}
			if tc.wantCancel && !errors.Is(got, tc.cause) {
				t.Fatalf("projectGCSError() = %v, want preserved cancellation %v", got, tc.cause)
			}
		})
	}
}

func validMediaGCSAttrs(t *testing.T) *storage.ObjectAttrs {
	t.Helper()
	instant, gotErr := temporal.InstantFromNanoseconds(metadataFixtureInstantNanos).Time()
	if gotErr != nil {
		t.Fatalf("InstantFromNanoseconds().Time() error = %v, want nil", gotErr)
	}
	return &storage.ObjectAttrs{
		Bucket:       "primitive-object-tests",
		Name:         "users/01/profile/photo.webp",
		Generation:   1_786_000_000_000_001,
		Size:         7,
		CRC32C:       0x01020304,
		ContentType:  "image/webp",
		CacheControl: "private, no-store",
		Created:      instant,
		Updated:      instant,
		CustomTime:   instant,
	}
}

// FuzzMetadataFromGCSAttrsSemanticClosure pressures the official SDK handoff.
// Accepted provider facts must project exactly into a validated sealed value;
// every refusal keeps the result zero and retains a stable typed identity.
func FuzzMetadataFromGCSAttrsSemanticClosure(f *testing.F) {
	f.Add(metadataFuzzBytes(metadataFuzzSeed{
		bucket: "primitive-object-tests", name: "users/01/profile/photo.webp",
		generation: 1, contentType: "application/octet-stream",
	}))
	f.Add(metadataFuzzBytes(metadataFuzzSeed{
		bucket: "primitive-object-tests", name: "users/01/profile/photo.webp",
		generation: 42, size: 7, checksum: 0x01020304,
		contentType: "image/webp", cacheControl: "private, no-store", hasCustomTime: true,
	}))
	f.Add(metadataFuzzBytes(metadataFuzzSeed{
		bucket: "Invalid_UPPER", name: "..", size: -1,
		contentType: "not a media type", cacheControl: "\x01bad",
	}))

	f.Fuzz(func(t *testing.T, input []byte) {
		attrs, hasCustomTime := metadataAttrsFromFuzzBytes(input)
		instant := time.Unix(0, metadataFixtureInstantNanos).UTC()
		attrs.Created = instant
		attrs.Updated = instant
		if hasCustomTime {
			attrs.CustomTime = instant
		}
		got, gotErr := metadataFromGCSAttrs(attrs)
		if gotErr != nil {
			if (!errors.Is(gotErr, core.ErrObjectStoreContract) && !errors.Is(gotErr, core.ErrObjectStoreSize)) ||
				got != (GCSObjectMetadata{}) {
				t.Fatalf("metadataFromGCSAttrs(rejected) = (%v, %v), want zero and stable object-store identity", got, gotErr)
			}
			return
		}
		if got.Validate() != nil {
			t.Fatalf("metadataFromGCSAttrs(accepted).Validate() error = %v, want nil", got.Validate())
		}
		projectedGeneration, generationErr := got.Generation().Int64()
		if generationErr != nil || projectedGeneration != attrs.Generation || got.Bucket().String() != attrs.Bucket ||
			got.Name().String() != attrs.Name || got.Length().Uint64() != uint64(attrs.Size) || got.CRC32C() != core.NewCRC32C(attrs.CRC32C) {
			t.Fatalf("metadataFromGCSAttrs(accepted) identity/integrity = (%q, %q, %d, %d, %v), want exact provider facts",
				got.Bucket().String(), got.Name().String(), projectedGeneration, got.Length().Uint64(), got.CRC32C())
		}
		wantContentType, contentTypeErr := core.ParseHTTPMediaType(attrs.ContentType)
		if contentTypeErr != nil || got.ContentType() != wantContentType || got.CacheControl().String() != attrs.CacheControl {
			t.Fatalf("metadataFromGCSAttrs(accepted) HTTP facts = (%q, %q), want typed (%q, %q), parse error %v",
				got.ContentType().String(), got.CacheControl().String(), wantContentType.String(), attrs.CacheControl, contentTypeErr)
		}
		if (got.CustomTime() != (temporal.Instant{})) != hasCustomTime {
			t.Fatalf("metadataFromGCSAttrs(accepted) custom-time presence = %t, want %t",
				got.CustomTime() != (temporal.Instant{}), hasCustomTime)
		}
	})
}

type metadataFuzzSeed struct {
	bucket        string
	name          string
	contentType   string
	cacheControl  string
	generation    int64
	size          int64
	checksum      uint32
	hasCustomTime bool
}

func metadataFuzzBytes(seed metadataFuzzSeed) []byte {
	encoded := make([]byte, 21, 21+len(seed.bucket)+len(seed.name)+len(seed.contentType)+len(seed.cacheControl)+3)
	binary.BigEndian.PutUint64(encoded[0:8], uint64(seed.generation))
	binary.BigEndian.PutUint64(encoded[8:16], uint64(seed.size))
	binary.BigEndian.PutUint32(encoded[16:20], seed.checksum)
	if seed.hasCustomTime {
		encoded[20] = 1
	}
	encoded = append(encoded, seed.bucket...)
	encoded = append(encoded, 0)
	encoded = append(encoded, seed.name...)
	encoded = append(encoded, 0)
	encoded = append(encoded, seed.contentType...)
	encoded = append(encoded, 0)
	return append(encoded, seed.cacheControl...)
}

func metadataAttrsFromFuzzBytes(input []byte) (*storage.ObjectAttrs, bool) {
	encoded := make([]byte, 21)
	copy(encoded, input)
	fields := strings.SplitN(string(input[min(len(input), 21):]), "\x00", 4)
	for len(fields) < 4 {
		fields = append(fields, "")
	}
	return &storage.ObjectAttrs{
		Bucket: fields[0], Name: fields[1], ContentType: fields[2], CacheControl: fields[3],
		Generation: int64(binary.BigEndian.Uint64(encoded[0:8])),
		Size:       int64(binary.BigEndian.Uint64(encoded[8:16])),
		CRC32C:     binary.BigEndian.Uint32(encoded[16:20]),
	}, encoded[20]&1 == 1
}
