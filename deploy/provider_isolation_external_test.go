package deploy_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/deploy"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/release"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestDeployRejectsValidForeignProviderSocket(t *testing.T) {
	t.Parallel()
	signed, err := objectstore.ParseSignedURL("https://bucket.s3.us-east-1.amazonaws.com/object?X-Amz-Signature=signature&X-Amz-SignedHeaders=host%3Bif-none-match%3Bx-amz-checksum-crc32c")
	if err != nil {
		t.Fatalf("ParseSignedURL error = %v, want nil", err)
	}
	headers, err := objectstore.NewSignedHeaders(nil)
	if err != nil {
		t.Fatalf("NewSignedHeaders error = %v, want nil", err)
	}
	projection, err := objectstore.NewUploadCapabilityProjection(objectstore.ProviderAmazonS3, objectstore.UploadTarget{URL: signed, Headers: headers, ExpiresAt: temporal.InstantFromNanoseconds(2051222400000000000)})
	if err != nil {
		t.Fatalf("valid S3 projection error = %v, want nil", err)
	}
	wire, err := projection.MarshalJSON()
	if err != nil {
		t.Fatalf("S3 projection MarshalJSON error = %v, want nil", err)
	}
	var capability objectstore.UploadCapability
	if err := capability.UnmarshalJSON(wire); err != nil {
		t.Fatalf("valid S3 capability error = %v, want nil", err)
	}
	source := &unreadSource{}
	got, err := deploy.NewUploadItem(deploy.UploadItemRequest{Source: source, Capability: capability, Commitment: fixtureCommitment(t, capability), Integrity: fixtureReleaseIntegrity(t, fixturePayload(0)), Role: release.PublicationRoleWindowsAMD64})
	if !errors.Is(err, core.ErrDeployContract) || !errors.Is(got.Validate(), core.ErrDeployContract) || source.reads != 0 {
		t.Fatalf("foreign socket admission = (%v,%v,%d reads), want sealed refusal before reading", got, err, source.reads)
	}
}
