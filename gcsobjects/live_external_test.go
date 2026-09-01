package gcsobjects_test

import (
	"bytes"
	"context"
	"errors"
	"hash/crc32"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/gcsobjects"
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
	gcsLiveMediaContentType              = "image/webp"
	gcsLiveFileContentType               = "application/octet-stream"
	gcsLiveMediaCacheControl             = "private, no-store"
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
	bucket, gotBucketErr := gcsobjects.ParseGCSBucket(bucketText)
	if gotBucketErr != nil {
		t.Fatalf("ParseGCSBucket(%q) error = %v, want nil", bucketText, gotBucketErr)
	}
	prefix, gotPrefixErr := gcsobjects.ParseGCSObjectPrefix(prefixText)
	if gotPrefixErr != nil {
		t.Fatalf("ParseGCSObjectPrefix(%q) error = %v, want nil", prefixText, gotPrefixErr)
	}
	maximum, gotMaximumErr := core.NewByteCount(gcsLiveDestructiveObjectBound)
	if gotMaximumErr != nil {
		t.Fatalf("NewByteCount(%d) error = %v, want nil", gcsLiveDestructiveObjectBound, gotMaximumErr)
	}
	client, gotClientErr := gcsobjects.NewGCSClient(context.Background(), gcsobjects.GCSClientConfig{
		Authentication: gcsobjects.GCSAuthenticationApplicationDefault,
	})
	if gotClientErr != nil {
		t.Fatalf("NewGCSClient() error = %v, want nil", gotClientErr)
	}
	t.Cleanup(func() {
		_, gotDeleteErr := gcsobjects.DeleteGCSObjects(context.Background(), client, gcsobjects.GCSDeleteRequest{
			Bucket: bucket, Prefix: prefix, MaxObjects: maximum,
		})
		if gotDeleteErr != nil {
			t.Errorf("cleanup DeleteGCSObjects(%q) error = %v, want nil", prefix.String(), gotDeleteErr)
		}
		if gotCloseErr := client.Close(); gotCloseErr != nil {
			t.Errorf("GCSClient.Close() error = %v, want nil", gotCloseErr)
		}
	})

	initial, gotInitialDeleteErr := gcsobjects.DeleteGCSObjects(context.Background(), client, gcsobjects.GCSDeleteRequest{
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
	created, gotCreateErr := gcsobjects.UploadMedia(context.Background(), client, liveGCSMediaUpload(t, bucket, primaryName, gcsLivePayload, gcsLivePayload))
	if gotCreateErr != nil {
		t.Fatalf("UploadMedia(%q) error = %v, want nil", primaryName.String(), gotCreateErr)
	}
	verifyLiveGCSMetadata(t, created, bucket, primaryName, gcsLivePayload, gcsLiveMediaContentType, gcsLiveMediaCacheControl)

	_, gotConflictErr := gcsobjects.UploadMedia(context.Background(), client, liveGCSMediaUpload(t, bucket, primaryName, gcsLivePayload, gcsLivePayload))
	if !errors.Is(gotConflictErr, core.ErrObjectStoreConflict) {
		t.Fatalf("duplicate UploadMedia(%q) error = %v, want errors.Is(..., %v)", primaryName.String(), gotConflictErr, core.ErrObjectStoreConflict)
	}

	emptyName := liveGCSObjectName(t, prefix, gcsLiveEmptyObjectLeaf)
	emptyMetadata, gotEmptyCreateErr := gcsobjects.UploadFile(context.Background(), client, liveGCSFileUpload(t, bucket, emptyName, nil, nil))
	if gotEmptyCreateErr != nil {
		t.Fatalf("UploadFile(%q empty stream) error = %v, want nil", emptyName.String(), gotEmptyCreateErr)
	}
	verifyLiveGCSMetadata(t, emptyMetadata, bucket, emptyName, nil, gcsLiveFileContentType, "")
	exactDeleted, gotExactDeleteErr := gcsobjects.DeleteGCSObject(context.Background(), client, gcsobjects.GCSDeleteObjectRequest{
		Bucket: bucket, Name: emptyName,
	})
	if gotExactDeleteErr != nil {
		t.Fatalf("DeleteGCSObject(%q) error = %v, want nil", emptyName.String(), gotExactDeleteErr)
	}
	if gotExactValidateErr := exactDeleted.Validate(); gotExactValidateErr != nil || exactDeleted.Name() != emptyName {
		t.Fatalf("exact delete evidence = (%q, %v), want (%q, nil)", exactDeleted.Name().String(), gotExactValidateErr, emptyName.String())
	}
	if exactDeleted.Generation() != emptyMetadata.Generation() {
		gotGen, _ := exactDeleted.Generation().Int64()
		wantGen, _ := emptyMetadata.Generation().Int64()
		t.Fatalf("exact delete generation = %d, want the created generation %d", gotGen, wantGen)
	}
	verifyLiveGCSObjectAbsent(t, client, bucket, emptyName, core.SHA256Of(nil))
	emptyMetadata, gotEmptyCreateErr = gcsobjects.UploadFile(context.Background(), client, liveGCSFileUpload(t, bucket, emptyName, nil, nil))
	if gotEmptyCreateErr != nil {
		t.Fatalf("replacement UploadFile(%q empty stream) error = %v, want nil", emptyName.String(), gotEmptyCreateErr)
	}
	verifyLiveGCSMetadata(t, emptyMetadata, bucket, emptyName, nil, gcsLiveFileContentType, "")

	shortName := liveGCSObjectName(t, prefix, gcsLiveShortSourceObjectLeaf)
	_, gotShortErr := gcsobjects.UploadFile(context.Background(), client, liveGCSFileUpload(t, bucket, shortName, gcsLivePayload[:len(gcsLivePayload)-1], gcsLivePayload))
	if !errors.Is(gotShortErr, core.ErrObjectStoreSource) || !errors.Is(gotShortErr, core.ErrObjectStoreIntegrity) {
		t.Fatalf("short source UploadFile(%q) error = %v, want source and integrity identities", shortName.String(), gotShortErr)
	}
	verifyLiveGCSObjectAbsent(t, client, bucket, shortName, core.SHA256Of(gcsLivePayload))

	wrongDigestName := liveGCSObjectName(t, prefix, gcsLiveWrongDigestObjectLeaf)
	wrongDigestRequest := liveGCSFileUpload(t, bucket, wrongDigestName, gcsLivePayload, gcsLivePayload)
	wrongDigestRequest.Integrity.SHA256 = core.SHA256Of([]byte("different bytes"))
	_, gotWrongDigestErr := gcsobjects.UploadFile(context.Background(), client, wrongDigestRequest)
	if !errors.Is(gotWrongDigestErr, core.ErrObjectStoreSource) || !errors.Is(gotWrongDigestErr, core.ErrObjectStoreIntegrity) {
		t.Fatalf("wrong digest UploadFile(%q) error = %v, want source and integrity identities", wrongDigestName.String(), gotWrongDigestErr)
	}
	verifyLiveGCSObjectAbsent(t, client, bucket, wrongDigestName, core.SHA256Of(gcsLivePayload))

	readRequest := liveGCSReadRequest(t, bucket, primaryName, liveGCSIntegrity(t, gcsLivePayload))
	readResult, gotReadErr := gcsobjects.ReadGCSObject(context.Background(), client, readRequest)
	if gotReadErr != nil {
		t.Fatalf("ReadGCSObject(%q) error = %v, want nil", primaryName.String(), gotReadErr)
	}
	readMetadata, metadataErr := readResult.Metadata()
	staged, stagedErr := readResult.Staged()
	var downloaded bytes.Buffer
	_, readErr := filestore.Read(t.Context(), filestore.ReadRequest{
		Destination: &downloaded, Location: readRequest.Destination.Temporary,
		MaximumBytes: liveGCSMaximum(t, len(gcsLivePayload)+1),
	})
	if metadataErr != nil || stagedErr != nil || readErr != nil || !bytes.Equal(downloaded.Bytes(), gcsLivePayload) {
		t.Fatalf("ReadGCSObject(%q) bytes = %q, want %q", primaryName.String(), downloaded.Bytes(), gcsLivePayload)
	}
	verifyLiveGCSMetadata(t, readMetadata, bucket, primaryName, gcsLivePayload, gcsLiveMediaContentType, gcsLiveMediaCacheControl)
	if err := filestore.Discard(t.Context(), staged); err != nil {
		t.Fatalf("filestore.Discard(verified GCS stage) error = %v, want nil", err)
	}

	wrongIntegrity := liveGCSIntegrity(t, gcsLivePayload)
	wrongIntegrity.SHA256 = core.SHA256Of([]byte("wrong read digest"))
	wrongRequest := liveGCSReadRequest(t, bucket, primaryName, wrongIntegrity)
	_, gotWrongReadErr := gcsobjects.ReadGCSObject(context.Background(), client, wrongRequest)
	if !errors.Is(gotWrongReadErr, core.ErrObjectStoreIntegrity) {
		t.Fatalf("ReadGCSObject(%q wrong checksum) error = %v, want errors.Is(..., %v)", primaryName.String(), gotWrongReadErr, core.ErrObjectStoreIntegrity)
	}

	deleted, gotDeleteErr := gcsobjects.DeleteGCSObjects(context.Background(), client, gcsobjects.GCSDeleteRequest{
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
	verifyLiveGCSObjectAbsent(t, client, bucket, primaryName, core.SHA256Of(gcsLivePayload))
	verifyLiveGCSObjectAbsent(t, client, bucket, emptyName, core.SHA256Of(nil))
}

func TestAuthenticatedGCSDeletionRefusesSoftDeleteRetention(t *testing.T) {
	t.Parallel()

	bucketText, bucketSet := os.LookupEnv(gcsSoftDeleteBucketEnvironment)
	prefixText, prefixSet := os.LookupEnv(gcsLivePrefixEnvironment)
	if !bucketSet || !prefixSet {
		t.Skipf("soft-delete provider proof requires %s and %s", gcsSoftDeleteBucketEnvironment, gcsLivePrefixEnvironment)
	}
	bucket, gotBucketErr := gcsobjects.ParseGCSBucket(bucketText)
	if gotBucketErr != nil {
		t.Fatalf("ParseGCSBucket(%q) error = %v, want nil", bucketText, gotBucketErr)
	}
	prefix, gotPrefixErr := gcsobjects.ParseGCSObjectPrefix(prefixText)
	if gotPrefixErr != nil {
		t.Fatalf("ParseGCSObjectPrefix(%q) error = %v, want nil", prefixText, gotPrefixErr)
	}
	maximum, gotMaximumErr := core.NewByteCount(1)
	if gotMaximumErr != nil {
		t.Fatalf("NewByteCount(1) error = %v, want nil", gotMaximumErr)
	}
	client, gotClientErr := gcsobjects.NewGCSClient(context.Background(), gcsobjects.GCSClientConfig{
		Authentication: gcsobjects.GCSAuthenticationApplicationDefault,
	})
	if gotClientErr != nil {
		t.Fatalf("NewGCSClient() error = %v, want nil", gotClientErr)
	}
	t.Cleanup(func() {
		if gotCloseErr := client.Close(); gotCloseErr != nil {
			t.Errorf("GCSClient.Close() error = %v, want nil", gotCloseErr)
		}
	})

	_, gotDeleteErr := gcsobjects.DeleteGCSObjects(context.Background(), client, gcsobjects.GCSDeleteRequest{
		Bucket: bucket, Prefix: prefix, MaxObjects: maximum,
	})
	if !errors.Is(gotDeleteErr, core.ErrObjectStoreConflict) {
		t.Fatalf("DeleteGCSObjects(%q soft-delete bucket) error = %v, want errors.Is(..., %v)", prefix.String(), gotDeleteErr, core.ErrObjectStoreConflict)
	}
}

func liveGCSMediaUpload(t *testing.T, bucket gcsobjects.GCSBucket, name gcsobjects.GCSObjectName, source, expected []byte) gcsobjects.GCSMediaUpload {
	t.Helper()
	mediaType, gotMediaTypeErr := core.ParseHTTPMediaType(gcsLiveMediaContentType)
	if gotMediaTypeErr != nil {
		t.Fatalf("ParseHTTPMediaType(%q) error = %v, want nil", gcsLiveMediaContentType, gotMediaTypeErr)
	}
	cacheControl, gotCacheErr := gcsobjects.ParseGCSCacheControl(gcsLiveMediaCacheControl)
	if gotCacheErr != nil {
		t.Fatalf("ParseGCSCacheControl(%q) error = %v, want nil", gcsLiveMediaCacheControl, gotCacheErr)
	}
	return gcsobjects.GCSMediaUpload{
		Bucket: bucket, Name: name, Source: bytes.NewReader(source),
		Integrity: liveGCSIntegrity(t, expected), ContentType: mediaType,
		CacheControl: cacheControl,
		CustomTime:   temporal.InstantFromNanoseconds(gcsLiveCustomTimeNanoseconds),
	}
}

func liveGCSFileUpload(t *testing.T, bucket gcsobjects.GCSBucket, name gcsobjects.GCSObjectName, source, expected []byte) gcsobjects.GCSFileUpload {
	t.Helper()
	return gcsobjects.GCSFileUpload{
		Bucket: bucket, Name: name, Source: bytes.NewReader(source),
		Integrity:  liveGCSIntegrity(t, expected),
		CustomTime: temporal.InstantFromNanoseconds(gcsLiveCustomTimeNanoseconds),
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

func liveGCSObjectName(t *testing.T, prefix gcsobjects.GCSObjectPrefix, leaf string) gcsobjects.GCSObjectName {
	t.Helper()
	name, gotNameErr := gcsobjects.ParseGCSObjectName(prefix.String() + leaf)
	if gotNameErr != nil {
		t.Fatalf("ParseGCSObjectName(%q) error = %v, want nil", prefix.String()+leaf, gotNameErr)
	}
	return name
}

func verifyLiveGCSMetadata(
	t *testing.T,
	got gcsobjects.GCSObjectMetadata,
	wantBucket gcsobjects.GCSBucket,
	wantName gcsobjects.GCSObjectName,
	wantData []byte,
	wantContentType string,
	wantCacheControl string,
) {
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
	if got.ContentType().String() != wantContentType || got.CacheControl().String() != wantCacheControl {
		t.Fatalf("GCSObjectMetadata content facts = (%q, %q), want (%q, %q)", got.ContentType().String(), got.CacheControl().String(), wantContentType, wantCacheControl)
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

func verifyLiveGCSObjectAbsent(t *testing.T, client *gcsobjects.GCSClient, bucket gcsobjects.GCSBucket, name gcsobjects.GCSObjectName, sha256 core.SHA256Digest) {
	t.Helper()
	integrity := liveGCSIntegrity(t, gcsLivePayload)
	integrity.SHA256 = sha256
	request := liveGCSReadRequest(t, bucket, name, integrity)
	_, gotReadErr := gcsobjects.ReadGCSObject(context.Background(), client, request)
	if !errors.Is(gotReadErr, core.ErrObjectStoreAbsent) {
		t.Fatalf("ReadGCSObject(%q after refused write or purge) error = %v, want errors.Is(..., %v)", name.String(), gotReadErr, core.ErrObjectStoreAbsent)
	}
}

func liveGCSReadRequest(
	t *testing.T,
	bucket gcsobjects.GCSBucket,
	name gcsobjects.GCSObjectName,
	integrity objectstore.Integrity,
) gcsobjects.GCSReadRequest {
	t.Helper()
	absolute, err := core.ParseAbsolutePath(t.TempDir())
	if err != nil {
		t.Fatalf("core.ParseAbsolutePath(GCS read root) error = %v, want nil", err)
	}
	root, err := filestore.OpenRoot(t.Context(), absolute)
	if err != nil {
		t.Fatalf("filestore.OpenRoot(GCS read root) error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = root.Close() })
	path, err := core.ParseRelativePath("download.stage")
	if err != nil {
		t.Fatalf("core.ParseRelativePath(download.stage) error = %v, want nil", err)
	}
	return gcsobjects.GCSReadRequest{
		Bucket: bucket, Name: name, Integrity: integrity,
		Destination: filestore.StageDestinationRequest{
			Temporary:     filestore.Location{Root: root, Path: path},
			ExpectedBytes: integrity.Length, Mode: 0o600,
		},
	}
}

func liveGCSMaximum(t *testing.T, value int) core.ByteCount {
	t.Helper()
	if value == 0 {
		value = 1
	}
	maximum, gotMaximumErr := core.NewByteCount(uint64(value))
	if gotMaximumErr != nil {
		t.Fatalf("NewByteCount(%d) error = %v, want nil", value, gotMaximumErr)
	}
	return maximum
}
