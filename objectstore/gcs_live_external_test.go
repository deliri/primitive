package objectstore_test

import (
	"bytes"
	"context"
	"errors"
	"hash/crc32"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	gcsLiveBucketEnvironment             = "PRIMITIVE_OBJECTSTORE_GCS_LIVE_BUCKET"
	gcsLivePrefixEnvironment             = "PRIMITIVE_OBJECTSTORE_GCS_LIVE_PREFIX"
	gcsSoftDeleteBucketEnvironment       = "PRIMITIVE_OBJECTSTORE_GCS_SOFT_DELETE_BUCKET"
	gcsLivePrimaryObjectLeaf             = "primary.bin"
	gcsLiveEmptyObjectLeaf               = "empty.bin"
	gcsLiveShortSourceObjectLeaf         = "short-source.bin"
	gcsLiveWrongDigestObjectLeaf         = "wrong-digest.bin"
	gcsLiveContentType                   = "application/octet-stream"
	gcsLiveCacheControl                  = "private, no-store"
	gcsLiveExpectedObjects               = 2
	gcsLiveDestructiveObjectBound        = 16
	gcsLiveCustomTimeNanoseconds   int64 = 1_786_183_200_000_000_000
)

var gcsLivePayload = []byte("primitive authenticated object lifecycle\n")

func TestAuthenticatedGCSLifecycleUsesTheRealProviderAndProvesDeletion(t *testing.T) {
	t.Parallel()

	bucketText, bucketSet := os.LookupEnv(gcsLiveBucketEnvironment)
	prefixText, prefixSet := os.LookupEnv(gcsLivePrefixEnvironment)
	if !bucketSet || !prefixSet {
		t.Skipf("real provider proof requires %s and %s", gcsLiveBucketEnvironment, gcsLivePrefixEnvironment)
	}
	bucket, gotBucketErr := objectstore.ParseGCSBucket(bucketText)
	if gotBucketErr != nil {
		t.Fatalf("ParseGCSBucket(%q) error = %v, want nil", bucketText, gotBucketErr)
	}
	prefix, gotPrefixErr := objectstore.ParseGCSObjectPrefix(prefixText)
	if gotPrefixErr != nil {
		t.Fatalf("ParseGCSObjectPrefix(%q) error = %v, want nil", prefixText, gotPrefixErr)
	}
	maximum, gotMaximumErr := core.NewByteCount(gcsLiveDestructiveObjectBound)
	if gotMaximumErr != nil {
		t.Fatalf("NewByteCount(%d) error = %v, want nil", gcsLiveDestructiveObjectBound, gotMaximumErr)
	}
	client, gotClientErr := objectstore.NewGCSClient(context.Background(), objectstore.GCSClientConfig{
		Authentication: objectstore.GCSAuthenticationApplicationDefault,
	})
	if gotClientErr != nil {
		t.Fatalf("NewGCSClient() error = %v, want nil", gotClientErr)
	}
	t.Cleanup(func() {
		_, gotDeleteErr := objectstore.DeleteGCSObjects(context.Background(), client, objectstore.GCSDeleteRequest{
			Bucket: bucket, Prefix: prefix, MaxObjects: maximum,
		})
		if gotDeleteErr != nil {
			t.Errorf("cleanup DeleteGCSObjects(%q) error = %v, want nil", prefix.String(), gotDeleteErr)
		}
		if gotCloseErr := client.Close(); gotCloseErr != nil {
			t.Errorf("GCSClient.Close() error = %v, want nil", gotCloseErr)
		}
	})

	initial, gotInitialDeleteErr := objectstore.DeleteGCSObjects(context.Background(), client, objectstore.GCSDeleteRequest{
		Bucket: bucket, Prefix: prefix, MaxObjects: maximum,
	})
	if gotInitialDeleteErr != nil {
		t.Fatalf("initial DeleteGCSObjects(%q) error = %v, want nil", prefix.String(), gotInitialDeleteErr)
	}
	if gotInitialValidateErr := initial.Validate(); gotInitialValidateErr != nil || initial.Prefix() != prefix {
		t.Fatalf("initial delete evidence = (%q, %v), want (%q, nil)", initial.Prefix().String(), gotInitialValidateErr, prefix.String())
	}
	if gotDeleted := initial.Deleted().Uint64(); gotDeleted != 0 {
		t.Fatalf("initial DeleteGCSObjects(%q) deleted = %d, want 0 neutral objects", prefix.String(), gotDeleted)
	}

	primaryName := liveGCSObjectName(t, prefix, gcsLivePrimaryObjectLeaf)
	primaryRequest := liveGCSWriteRequest(t, bucket, primaryName, gcsLivePayload, gcsLivePayload)
	created, gotCreateErr := objectstore.CreateGCSObject(context.Background(), client, primaryRequest)
	if gotCreateErr != nil {
		t.Fatalf("CreateGCSObject(%q) error = %v, want nil", primaryName.String(), gotCreateErr)
	}
	verifyLiveGCSMetadata(t, created, bucket, primaryName, gcsLivePayload)

	duplicateRequest := liveGCSWriteRequest(t, bucket, primaryName, gcsLivePayload, gcsLivePayload)
	_, gotConflictErr := objectstore.CreateGCSObject(context.Background(), client, duplicateRequest)
	if !errors.Is(gotConflictErr, core.ErrObjectStoreConflict) {
		t.Fatalf("duplicate CreateGCSObject(%q) error = %v, want errors.Is(..., %v)", primaryName.String(), gotConflictErr, core.ErrObjectStoreConflict)
	}

	emptyName := liveGCSObjectName(t, prefix, gcsLiveEmptyObjectLeaf)
	emptyRequest := liveGCSWriteRequest(t, bucket, emptyName, nil, nil)
	emptyMetadata, gotEmptyCreateErr := objectstore.CreateGCSObject(context.Background(), client, emptyRequest)
	if gotEmptyCreateErr != nil {
		t.Fatalf("CreateGCSObject(%q empty stream) error = %v, want nil", emptyName.String(), gotEmptyCreateErr)
	}
	verifyLiveGCSMetadata(t, emptyMetadata, bucket, emptyName, nil)

	shortName := liveGCSObjectName(t, prefix, gcsLiveShortSourceObjectLeaf)
	shortRequest := liveGCSWriteRequest(t, bucket, shortName, gcsLivePayload[:len(gcsLivePayload)-1], gcsLivePayload)
	_, gotShortErr := objectstore.CreateGCSObject(context.Background(), client, shortRequest)
	if !errors.Is(gotShortErr, core.ErrObjectStoreSource) || !errors.Is(gotShortErr, core.ErrObjectStoreIntegrity) {
		t.Fatalf("short source CreateGCSObject(%q) error = %v, want source and integrity identities", shortName.String(), gotShortErr)
	}
	verifyLiveGCSObjectAbsent(t, client, bucket, shortName, liveGCSIntegrity(t, gcsLivePayload))

	wrongDigestName := liveGCSObjectName(t, prefix, gcsLiveWrongDigestObjectLeaf)
	wrongDigestRequest := liveGCSWriteRequest(t, bucket, wrongDigestName, gcsLivePayload, gcsLivePayload)
	wrongDigestRequest.Integrity.SHA256 = core.SHA256Of([]byte("different bytes"))
	_, gotWrongDigestErr := objectstore.CreateGCSObject(context.Background(), client, wrongDigestRequest)
	if !errors.Is(gotWrongDigestErr, core.ErrObjectStoreSource) || !errors.Is(gotWrongDigestErr, core.ErrObjectStoreIntegrity) {
		t.Fatalf("wrong digest CreateGCSObject(%q) error = %v, want source and integrity identities", wrongDigestName.String(), gotWrongDigestErr)
	}
	verifyLiveGCSObjectAbsent(t, client, bucket, wrongDigestName, liveGCSIntegrity(t, gcsLivePayload))

	var downloaded bytes.Buffer
	readMetadata, gotReadErr := objectstore.ReadGCSObject(context.Background(), client, objectstore.GCSReadRequest{
		Bucket: bucket, Name: primaryName, Destination: &downloaded,
		Integrity: liveGCSIntegrity(t, gcsLivePayload),
	})
	if gotReadErr != nil {
		t.Fatalf("ReadGCSObject(%q) error = %v, want nil", primaryName.String(), gotReadErr)
	}
	if !bytes.Equal(downloaded.Bytes(), gcsLivePayload) {
		t.Fatalf("ReadGCSObject(%q) bytes = %q, want %q", primaryName.String(), downloaded.Bytes(), gcsLivePayload)
	}
	verifyLiveGCSMetadata(t, readMetadata, bucket, primaryName, gcsLivePayload)

	wrongReadIntegrity := liveGCSIntegrity(t, gcsLivePayload)
	wrongChecksum, gotWrongChecksumErr := wrongReadIntegrity.CRC32C.Uint32()
	if gotWrongChecksumErr != nil {
		t.Fatalf("CRC32C.Uint32() error = %v, want nil", gotWrongChecksumErr)
	}
	wrongReadIntegrity.CRC32C = core.NewCRC32C(wrongChecksum + 1)
	var rejected bytes.Buffer
	_, gotWrongReadErr := objectstore.ReadGCSObject(context.Background(), client, objectstore.GCSReadRequest{
		Bucket: bucket, Name: primaryName, Destination: &rejected, Integrity: wrongReadIntegrity,
	})
	if !errors.Is(gotWrongReadErr, core.ErrObjectStoreIntegrity) {
		t.Fatalf("ReadGCSObject(%q wrong checksum) error = %v, want errors.Is(..., %v)", primaryName.String(), gotWrongReadErr, core.ErrObjectStoreIntegrity)
	}
	if !bytes.Equal(rejected.Bytes(), gcsLivePayload) {
		t.Fatalf("wrong-checksum read streamed bytes = %q, want %q before independent refusal", rejected.Bytes(), gcsLivePayload)
	}

	deleted, gotDeleteErr := objectstore.DeleteGCSObjects(context.Background(), client, objectstore.GCSDeleteRequest{
		Bucket: bucket, Prefix: prefix, MaxObjects: maximum,
	})
	if gotDeleteErr != nil {
		t.Fatalf("DeleteGCSObjects(%q) error = %v, want nil", prefix.String(), gotDeleteErr)
	}
	if gotDeleteValidateErr := deleted.Validate(); gotDeleteValidateErr != nil || deleted.Prefix() != prefix {
		t.Fatalf("delete evidence = (%q, %v), want (%q, nil)", deleted.Prefix().String(), gotDeleteValidateErr, prefix.String())
	}
	if gotDeleted := deleted.Deleted().Uint64(); gotDeleted != gcsLiveExpectedObjects {
		t.Fatalf("DeleteGCSObjects(%q) deleted = %d, want %d exact objects", prefix.String(), gotDeleted, gcsLiveExpectedObjects)
	}
	verifyLiveGCSObjectAbsent(t, client, bucket, primaryName, liveGCSIntegrity(t, gcsLivePayload))
	verifyLiveGCSObjectAbsent(t, client, bucket, emptyName, liveGCSIntegrity(t, nil))
}

func TestAuthenticatedGCSDeletionRefusesSoftDeleteRetention(t *testing.T) {
	t.Parallel()

	bucketText, bucketSet := os.LookupEnv(gcsSoftDeleteBucketEnvironment)
	prefixText, prefixSet := os.LookupEnv(gcsLivePrefixEnvironment)
	if !bucketSet || !prefixSet {
		t.Skipf("soft-delete provider proof requires %s and %s", gcsSoftDeleteBucketEnvironment, gcsLivePrefixEnvironment)
	}
	bucket, gotBucketErr := objectstore.ParseGCSBucket(bucketText)
	if gotBucketErr != nil {
		t.Fatalf("ParseGCSBucket(%q) error = %v, want nil", bucketText, gotBucketErr)
	}
	prefix, gotPrefixErr := objectstore.ParseGCSObjectPrefix(prefixText)
	if gotPrefixErr != nil {
		t.Fatalf("ParseGCSObjectPrefix(%q) error = %v, want nil", prefixText, gotPrefixErr)
	}
	maximum, gotMaximumErr := core.NewByteCount(1)
	if gotMaximumErr != nil {
		t.Fatalf("NewByteCount(1) error = %v, want nil", gotMaximumErr)
	}
	client, gotClientErr := objectstore.NewGCSClient(context.Background(), objectstore.GCSClientConfig{
		Authentication: objectstore.GCSAuthenticationApplicationDefault,
	})
	if gotClientErr != nil {
		t.Fatalf("NewGCSClient() error = %v, want nil", gotClientErr)
	}
	t.Cleanup(func() {
		if gotCloseErr := client.Close(); gotCloseErr != nil {
			t.Errorf("GCSClient.Close() error = %v, want nil", gotCloseErr)
		}
	})

	_, gotDeleteErr := objectstore.DeleteGCSObjects(context.Background(), client, objectstore.GCSDeleteRequest{
		Bucket: bucket, Prefix: prefix, MaxObjects: maximum,
	})
	if !errors.Is(gotDeleteErr, core.ErrObjectStoreConflict) {
		t.Fatalf("DeleteGCSObjects(%q soft-delete bucket) error = %v, want errors.Is(..., %v)", prefix.String(), gotDeleteErr, core.ErrObjectStoreConflict)
	}
}

func liveGCSWriteRequest(t *testing.T, bucket objectstore.GCSBucket, name objectstore.GCSObjectName, source, expected []byte) objectstore.GCSWriteRequest {
	t.Helper()
	mediaType, gotMediaTypeErr := core.ParseHTTPMediaType(gcsLiveContentType)
	if gotMediaTypeErr != nil {
		t.Fatalf("ParseHTTPMediaType(%q) error = %v, want nil", gcsLiveContentType, gotMediaTypeErr)
	}
	cacheControl, gotCacheErr := objectstore.ParseGCSCacheControl(gcsLiveCacheControl)
	if gotCacheErr != nil {
		t.Fatalf("ParseGCSCacheControl(%q) error = %v, want nil", gcsLiveCacheControl, gotCacheErr)
	}
	return objectstore.GCSWriteRequest{
		Bucket: bucket, Name: name, Source: bytes.NewReader(source),
		Integrity: liveGCSIntegrity(t, expected), ContentType: mediaType,
		CacheControl: cacheControl,
		CustomTime:   temporal.InstantFromNanoseconds(gcsLiveCustomTimeNanoseconds),
	}
}

func liveGCSIntegrity(t *testing.T, data []byte) objectstore.Integrity {
	t.Helper()
	length, gotLengthErr := core.NewByteLength(uint64(len(data)))
	if gotLengthErr != nil {
		t.Fatalf("NewByteLength(%d) error = %v, want nil", len(data), gotLengthErr)
	}
	return objectstore.Integrity{
		SHA256: core.SHA256Of(data), Length: length,
		CRC32C: core.NewCRC32C(crc32.Checksum(data, crc32.MakeTable(crc32.Castagnoli))),
	}
}

func liveGCSObjectName(t *testing.T, prefix objectstore.GCSObjectPrefix, leaf string) objectstore.GCSObjectName {
	t.Helper()
	name, gotNameErr := objectstore.ParseGCSObjectName(prefix.String() + leaf)
	if gotNameErr != nil {
		t.Fatalf("ParseGCSObjectName(%q) error = %v, want nil", prefix.String()+leaf, gotNameErr)
	}
	return name
}

func verifyLiveGCSMetadata(t *testing.T, got objectstore.GCSObjectMetadata, wantBucket objectstore.GCSBucket, wantName objectstore.GCSObjectName, wantData []byte) {
	t.Helper()
	if gotErr := got.Validate(); gotErr != nil {
		t.Fatalf("GCSObjectMetadata.Validate() error = %v, want nil", gotErr)
	}
	if got.Bucket() != wantBucket || got.Name() != wantName {
		t.Fatalf("GCSObjectMetadata identity = (%q, %q), want (%q, %q)", got.Bucket().String(), got.Name().String(), wantBucket.String(), wantName.String())
	}
	if got.Length().Uint64() != uint64(len(wantData)) {
		t.Fatalf("GCSObjectMetadata.Length() = %d, want %d", got.Length().Uint64(), len(wantData))
	}
	wantChecksum := crc32.Checksum(wantData, crc32.MakeTable(crc32.Castagnoli))
	gotChecksum, gotChecksumErr := got.CRC32C().Uint32()
	if gotChecksumErr != nil || gotChecksum != wantChecksum {
		t.Fatalf("GCSObjectMetadata.CRC32C() = (%d, %v), want (%d, nil)", gotChecksum, gotChecksumErr, wantChecksum)
	}
	if got.ContentType().String() != gcsLiveContentType || got.CacheControl().String() != gcsLiveCacheControl {
		t.Fatalf("GCSObjectMetadata content facts = (%q, %q), want (%q, %q)", got.ContentType().String(), got.CacheControl().String(), gcsLiveContentType, gcsLiveCacheControl)
	}
	wantCustomTime := temporal.InstantFromNanoseconds(gcsLiveCustomTimeNanoseconds)
	if comparison, gotCompareErr := got.CustomTime().Compare(wantCustomTime); gotCompareErr != nil || comparison != core.ComparisonEqual {
		t.Fatalf("GCSObjectMetadata.CustomTime().Compare() = (%v, %v), want (%v, nil)", comparison, gotCompareErr, core.ComparisonEqual)
	}
	if got.CreatedAt().Validate() != nil || got.UpdatedAt().Validate() != nil {
		t.Fatalf("GCSObjectMetadata provider times = (%v, %v), want both set", got.CreatedAt(), got.UpdatedAt())
	}
	if generation, gotGenerationErr := got.Generation().Int64(); gotGenerationErr != nil || generation <= 0 {
		t.Fatalf("GCSObjectMetadata.Generation() = (%d, %v), want positive provider generation", generation, gotGenerationErr)
	}
}

func verifyLiveGCSObjectAbsent(t *testing.T, client *objectstore.GCSClient, bucket objectstore.GCSBucket, name objectstore.GCSObjectName, integrity objectstore.Integrity) {
	t.Helper()
	var destination bytes.Buffer
	_, gotReadErr := objectstore.ReadGCSObject(context.Background(), client, objectstore.GCSReadRequest{
		Bucket: bucket, Name: name, Destination: &destination, Integrity: integrity,
	})
	if !errors.Is(gotReadErr, core.ErrObjectStoreAbsent) {
		t.Fatalf("ReadGCSObject(%q after refused write or purge) error = %v, want errors.Is(..., %v)", name.String(), gotReadErr, core.ErrObjectStoreAbsent)
	}
	if destination.Len() != 0 {
		t.Fatalf("ReadGCSObject(%q absent) destination bytes = %d, want 0", name.String(), destination.Len())
	}
}
