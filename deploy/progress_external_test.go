package deploy_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/deploy"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/release"
)

func TestProgressObservationLayerTriad(t *testing.T) {
	t.Parallel()
	refusal := errors.New("test observer refused observation")
	for _, mode := range []string{"exact progress is delivered", "observer refusal preserves confirmed prefix", "absent observer does not suppress publication"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			request := newDeployFixtureRequest(t)
			index := release.PublicationObjectCount - 1
			payload := fixturePayload(index)
			var observed uint64
			var observations int
			var observer objectstore.ProgressObserver
			if mode != "absent observer does not suppress publication" {
				observer = func(progress objectstore.TransferProgress) error {
					if err := progress.Validate(); err != nil || progress.Direction() != objectstore.DirectionUpload || progress.Total().Uint64() != uint64(len(payload)) || progress.Completed().Uint64() < observed {
						return errors.Join(core.ErrObjectStoreContract, err)
					}
					observed = progress.Completed().Uint64()
					observations++
					if mode == "observer refusal preserves confirmed prefix" {
						return refusal
					}
					return nil
				}
			}
			c := fixtureCapability(t, index)
			item, err := deploy.NewUploadItem(deploy.UploadItemRequest{Source: bytes.NewReader(payload), Observer: observer, Capability: c, Commitment: fixtureCommitment(t, c), Integrity: fixtureReleaseIntegrity(t, payload), Role: release.PublicationRoleReleaseNotes})
			if err != nil {
				t.Fatalf("NewUploadItem observer error = %v, want nil", err)
			}
			request.Items[index] = item
			plan, err := deploy.PrepareRelease(request)
			if err != nil {
				t.Fatalf("PrepareRelease observer error = %v, want nil", err)
			}
			transport := &recordingTransport{failAt: -1}
			got, err := deploy.ReleaseGCS(t.Context(), deployObjectstoreClient(t, transport), plan)
			if mode == "observer refusal preserves confirmed prefix" {
				failure, present := errors.AsType[*deploy.UploadError](err)
				if !errors.Is(err, refusal) || !present || failure.Role != release.PublicationRoleReleaseNotes || failure.Transfer.Commitment() == objectstore.CommitmentConfirmed || got.Count() != index || observations != 1 {
					t.Fatalf("observer refusal = (%d receipts,%d observations,%v), want prefix and exact refusal", got.Count(), observations, err)
				}
			} else if err != nil || got.Count() != release.PublicationObjectCount {
				t.Fatalf("progress publication = (%d,%v), want complete", got.Count(), err)
			}
			if observer != nil && (observations == 0 || observed != uint64(len(payload))) {
				t.Fatalf("progress = (%d observations,%d bytes), want complete nonvacuous source observation", observations, observed)
			}
			if observer == nil && observations != 0 {
				t.Fatalf("absent observer count = %d, want zero", observations)
			}
			if transport.requests != release.PublicationObjectCount {
				t.Fatalf("progress request count = %d, want one request per role", transport.requests)
			}
		})
	}
}
