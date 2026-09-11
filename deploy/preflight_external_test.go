package deploy_test

import (
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/deploy"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/release"
)

type unreadSource struct{ reads int }

func (s *unreadSource) Read([]byte) (int, error) { s.reads++; return 0, io.EOF }

func TestUploadAdmissionProviderExtentLayerTriad(t *testing.T) {
	t.Parallel()
	spec, err := objectstore.Spec(objectstore.ProviderGoogleCloudStorage)
	if err != nil {
		t.Fatalf("Objectstore.Spec error = %v, want nil", err)
	}
	maximum := spec.UploadMaximum.Uint64()
	for _, tc := range []struct {
		name    string
		extent  uint64
		wantErr error
	}{
		{name: "smallest nonempty nominal object", extent: 1},
		{name: "one byte below GCS limit", extent: maximum - 1},
		{name: "exact GCS limit", extent: maximum},
		{name: "one byte above GCS limit", extent: maximum + 1, wantErr: core.ErrObjectStoreSize},
		{name: "maximum representable extent cannot cross provider wall", extent: ^uint64(0), wantErr: core.ErrObjectStoreSize},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			extent, err := core.NewByteCount(tc.extent)
			if err != nil {
				t.Fatalf("NewByteCount error = %v, want nil", err)
			}
			asset, err := release.NewMetadataAsset(release.MetadataAssetRequest{Kind: release.MetadataKindDependencies, Extent: extent, SHA256: core.SHA256Of([]byte("nominal extent facts")), CRC32C: core.NewCRC32C(1)})
			if err != nil {
				t.Fatalf("NewMetadataAsset error = %v, want nil", err)
			}
			source := &unreadSource{}
			capability := fixtureCapability(t, 0)
			request := deploy.UploadItemRequest{Source: source, Capability: capability, Commitment: fixtureCommitment(t, capability), Integrity: asset.Integrity(), Role: release.PublicationRoleDependencies}
			got, err := deploy.NewUploadItem(request)
			if !errors.Is(err, tc.wantErr) || !errors.Is(request.Validate(), tc.wantErr) {
				t.Fatalf("NewUploadItem/Validate errors = (%v,%v), want %v", err, request.Validate(), tc.wantErr)
			}
			if source.reads != 0 {
				t.Fatalf("admission read count = %d, want zero", source.reads)
			}
			if tc.wantErr != nil {
				if !errors.Is(err, core.ErrDeployContract) || !errors.Is(got.Validate(), core.ErrDeployContract) {
					t.Fatalf("oversized item = (%v,%v), want sealed refusal", got, err)
				}
			} else if err := got.Validate(); err != nil {
				t.Fatalf("nominal item.Validate error = %v, want nil", err)
			}
		})
	}
}
