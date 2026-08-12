package deploy_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/deploy"
	"github.com/deliri/primitive/v2026/release"
)

func TestReleasePublicationLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive exact plan reaches every provider object", func(t *testing.T) {
		t.Parallel()
		fixture := newDeployFixture(t)
		provider := &loopbackProvider{}
		receipts, err := deploy.ReleaseGCS(t.Context(), deployLoopbackClient(t, provider), fixture.plan)
		if err != nil || receipts.Count() != release.PublicationObjectCount || len(provider.recorded()) != release.PublicationObjectCount {
			t.Fatalf("ReleaseGCS(complete) = (receipts %d, requests %d, %v), want (%d, %d, nil)", receipts.Count(), len(provider.recorded()), err, release.PublicationObjectCount, release.PublicationObjectCount)
		}
	})

	t.Run("negative first provider loss returns typed zero confirmed prefix", func(t *testing.T) {
		t.Parallel()
		fixture := newDeployFixture(t)
		transport := &recordingTransport{failAt: 0}
		receipts, err := deploy.ReleaseGCS(context.Background(), deployObjectstoreClient(t, transport), fixture.plan)
		var typed *deploy.UploadError
		if !errors.Is(err, core.ErrDeployContract) || !errors.As(err, &typed) ||
			typed.Role != release.PublicationRoleWindowsAMD64 || receipts.Count() != 0 || transport.requests != 1 {
			t.Fatalf("ReleaseGCS(first loss) = (receipts %d, requests %d, error %v), want zero, one, typed Windows AMD64 failure", receipts.Count(), transport.requests, err)
		}
	})

	t.Run("neutral absent plan performs no provider request", func(t *testing.T) {
		t.Parallel()
		transport := &recordingTransport{failAt: -1}
		receipts, err := deploy.ReleaseGCS(context.Background(), deployObjectstoreClient(t, transport), deploy.ReleasePlan{})
		if !errors.Is(err, core.ErrDeployContract) || receipts.Count() != 0 || transport.requests != 0 {
			t.Fatalf("ReleaseGCS(zero plan) = (receipts %d, requests %d, error %v), want zero, zero, %v", receipts.Count(), transport.requests, err, core.ErrDeployContract)
		}
	})
}
