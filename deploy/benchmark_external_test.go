package deploy_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/deploy"
	"github.com/deliri/primitive/v2026/release"
)

func BenchmarkNewUploadItem(b *testing.B) {
	capability := fixtureCapability(b, 0)
	request := deploy.UploadItemRequest{
		Source:     sourceForTest(0),
		Capability: capability,
		Commitment: fixtureCommitment(b, capability),
		Integrity:  fixtureReleaseIntegrity(b, fixturePayload(0)),
		Role:       release.PublicationRoleWindowsAMD64,
	}
	var wantErr error
	b.ReportAllocs()
	var last deploy.UploadItem
	for b.Loop() {
		got, err := deploy.NewUploadItem(request)
		if !errors.Is(err, wantErr) {
			b.Fatalf("deploy.NewUploadItem() error = %v, want %v", err, wantErr)
		}
		last = got
	}
	if err := last.Validate(); err != nil {
		b.Fatalf("deploy.NewUploadItem().Validate() error = %v, want nil", err)
	}
}
