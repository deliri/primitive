package gcsobjects_test

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"hash/crc32"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gcsobjects"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	gcsLiveCapabilityPrefixEnvironment = "PRIMITIVE_OBJECTSTORE_GCS_CAPABILITY_PREFIX"
	gcsLiveServiceAccountEnvironment   = "PRIMITIVE_OBJECTSTORE_GCS_LIVE_SERVICE_ACCOUNT"
	gcsLiveCapabilityObjectLeaf        = "private-display-proof.webp"
	gcsLiveCapabilityPayloadText       = "primitive private display capability proof\n"
)

func TestGCSDownloadCapabilityRetrievesOneRealPrivateObjectThroughObjectstore(t *testing.T) {
	t.Parallel()

	bucketText, bucketSet := os.LookupEnv(gcsLiveBucketEnvironment)
	prefixText, prefixSet := os.LookupEnv(gcsLiveCapabilityPrefixEnvironment)
	accountText, accountSet := os.LookupEnv(gcsLiveServiceAccountEnvironment)
	if !bucketSet || !prefixSet || !accountSet {
		t.Skipf("real capability proof requires %s, %s, and %s", gcsLiveBucketEnvironment, gcsLiveCapabilityPrefixEnvironment, gcsLiveServiceAccountEnvironment)
	}
	bucket, gotBucketErr := gcsobjects.ParseGCSBucket(bucketText)
	if gotBucketErr != nil {
		t.Fatalf("ParseGCSBucket(%q) error = %v, want nil", bucketText, gotBucketErr)
	}
	prefix, gotPrefixErr := gcsobjects.ParseGCSObjectPrefix(prefixText)
	if gotPrefixErr != nil {
		t.Fatalf("ParseGCSObjectPrefix(%q) error = %v, want nil", prefixText, gotPrefixErr)
	}
	account, gotAccountErr := gcsobjects.ParseGCSServiceAccount(accountText)
	if gotAccountErr != nil {
		t.Fatalf("ParseGCSServiceAccount(%q) error = %v, want nil", accountText, gotAccountErr)
	}
	name := liveGCSObjectName(t, prefix, gcsLiveCapabilityObjectLeaf)
	maximum := liveGCSMaximum(t, 2)
	payload := []byte(gcsLiveCapabilityPayloadText)

	ctx := context.Background()
	provider, gotClientErr := gcsobjects.NewGCSClient(ctx, gcsobjects.GCSClientConfig{
		Authentication: gcsobjects.GCSAuthenticationApplicationDefault,
	})
	if gotClientErr != nil {
		t.Fatalf("NewGCSClient() error = %v, want nil", gotClientErr)
	}
	t.Cleanup(func() {
		_, gotDeleteErr := gcsobjects.DeleteGCSObjects(context.Background(), provider, gcsobjects.GCSDeleteRequest{
			Bucket: bucket, Prefix: prefix, MaxObjects: maximum,
		})
		if gotDeleteErr != nil {
			t.Errorf("cleanup DeleteGCSObjects(%q) error = %v, want nil", prefix.String(), gotDeleteErr)
		}
		if gotCloseErr := provider.Close(); gotCloseErr != nil {
			t.Errorf("GCSClient.Close() error = %v, want nil", gotCloseErr)
		}
	})
	initial, gotInitialErr := gcsobjects.DeleteGCSObjects(ctx, provider, gcsobjects.GCSDeleteRequest{
		Bucket: bucket, Prefix: prefix, MaxObjects: maximum,
	})
	if gotInitialErr != nil || initial.Validate() != nil || initial.Deleted().Uint64() != 0 {
		t.Fatalf("initial DeleteGCSObjects(%q) = (%v, %v), want validated neutral zero deletion and nil", prefix.String(), initial, gotInitialErr)
	}
	created, gotCreateErr := gcsobjects.UploadMedia(ctx, provider, liveGCSMediaUpload(
		t, bucket, name, payload, payload,
	))
	if gotCreateErr != nil || created.Validate() != nil {
		t.Fatalf("UploadMedia(%q) = (%v, %v), want validated provider result and nil", name.String(), created, gotCreateErr)
	}

	issuer, gotIssuerErr := gcsobjects.NewGCSCapabilityIssuer(ctx, gcsobjects.GCSClientConfig{
		Authentication: gcsobjects.GCSAuthenticationApplicationDefault,
	})
	if gotIssuerErr != nil {
		t.Fatalf("NewGCSCapabilityIssuer() error = %v, want nil", gotIssuerErr)
	}
	lifetime, gotLifetimeErr := temporal.DurationFromMinutes(5)
	if gotLifetimeErr != nil {
		t.Fatalf("temporal.DurationFromMinutes(5) error = %v, want nil", gotLifetimeErr)
	}
	projection, gotIssueErr := gcsobjects.IssueGCSDownloadCapability(ctx, issuer, gcsobjects.GCSDownloadCapabilityRequest{
		Bucket: bucket, Name: name, ServiceAccount: account, Lifetime: lifetime,
	})
	if gotIssueErr != nil || projection.IsZero() || projection.Validate() != nil {
		t.Fatalf("IssueGCSDownloadCapability(%q) = (%v, %v), want validated nonzero projection and nil", name.String(), projection, gotIssueErr)
	}
	encoded, gotMarshalErr := projection.MarshalJSON()
	if gotMarshalErr != nil || len(encoded) > objectstore.CapabilityJSONMaximumBytes {
		t.Fatalf("DownloadCapabilityProjection.MarshalJSON() = (%d bytes, %v), want bounded and nil", len(encoded), gotMarshalErr)
	}
	var capability objectstore.DownloadCapability
	if gotDecodeErr := json.Unmarshal(encoded, &capability); gotDecodeErr != nil || capability.Validate() != nil {
		t.Fatalf("DownloadCapability.UnmarshalJSON() = (%v, %v), want validated capability and nil", capability, gotDecodeErr)
	}

	contentType, gotContentTypeErr := core.ParseHTTPMediaType(gcsLiveMediaContentType)
	if gotContentTypeErr != nil {
		t.Fatalf("ParseHTTPMediaType(%q) error = %v, want nil", gcsLiveMediaContentType, gotContentTypeErr)
	}
	length, gotLengthErr := core.NewByteLength(uint64(len(payload)))
	if gotLengthErr != nil {
		t.Fatalf("NewByteLength(%d) error = %v, want nil", len(payload), gotLengthErr)
	}
	integrity := objectstore.Integrity{
		SHA256: core.SHA256Of(payload),
		Length: length,
		CRC32C: core.NewCRC32C(crc32.Checksum(payload, crc32.MakeTable(crc32.Castagnoli))),
	}
	client, gotObjectstoreClientErr := objectstore.NewStandardClient()
	if gotObjectstoreClientErr != nil {
		t.Fatalf("objectstore.NewStandardClient() error = %v, want nil", gotObjectstoreClientErr)
	}
	var downloaded bytes.Buffer
	transfer, gotDownloadErr := objectstore.Download(ctx, client, objectstore.DownloadCapabilityRequest{
		Destination: &downloaded, ContentType: contentType, Capability: capability,
		Integrity: integrity, Policy: liveGCSDownloadPolicy(t),
	})
	if gotDownloadErr != nil || transfer.Validate() != nil {
		t.Fatalf("objectstore.Download(private capability) = (%v, %v; contract=%t integrity=%t absent=%t exchange_response=%t content_type=%t body_limit=%t), want validated transfer and nil", transfer, gotDownloadErr, errors.Is(gotDownloadErr, core.ErrObjectStoreContract), errors.Is(gotDownloadErr, core.ErrObjectStoreIntegrity), errors.Is(gotDownloadErr, core.ErrObjectStoreAbsent), errors.Is(gotDownloadErr, core.ErrExchangeResponse), errors.Is(gotDownloadErr, core.ErrExchangeContentType), errors.Is(gotDownloadErr, core.ErrExchangeBodyLimit))
	}
	if !bytes.Equal(downloaded.Bytes(), payload) {
		t.Fatalf("objectstore.Download(private capability) bytes = %q, want %q", downloaded.Bytes(), payload)
	}
	if gotProvider, wantProvider := transfer.Provider(), objectstore.ProviderGoogleCloudStorage; gotProvider != wantProvider {
		t.Fatalf("objectstore.Download(private capability) provider = %v, want %v", gotProvider, wantProvider)
	}
	if gotDirection, wantDirection := transfer.Direction(), objectstore.DirectionDownload; gotDirection != wantDirection {
		t.Fatalf("objectstore.Download(private capability) direction = %v, want %v", gotDirection, wantDirection)
	}
}

func liveGCSDownloadPolicy(t testing.TB) objectstore.Policy {
	t.Helper()

	operation, gotOperationErr := temporal.DurationFromSeconds(30)
	attempt, gotAttemptErr := temporal.DurationFromSeconds(15)
	errorBody, gotErrorBodyErr := core.NewByteCount(4096)
	if gotOperationErr != nil || gotAttemptErr != nil || gotErrorBodyErr != nil {
		t.Fatalf("download policy construction errors = (%v, %v, %v), want nil", gotOperationErr, gotAttemptErr, gotErrorBodyErr)
	}
	return objectstore.Policy{OperationTimeout: operation, AttemptTimeout: attempt, ErrorBodyLimit: errorBody}
}
