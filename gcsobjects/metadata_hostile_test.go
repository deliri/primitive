package gcsobjects

import (
	"context"
	"errors"
	"testing"

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
