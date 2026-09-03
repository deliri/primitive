package runnercontrol_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/runprotocol"
)

func TestObservationDeliveryProtocolLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive exact configured origin closes staged signed evidence", func(t *testing.T) {
		t.Parallel()
		stage, pages, verifier := deliveryProtocolFixture(t)
		identity, identityErr := stage.Identity()
		gotErr := verifier.Verify(stage, pages)
		if identityErr != nil || gotErr != nil {
			t.Fatalf("delivery closure = (identity error %v, verify error %v), want (nil, nil)", identityErr, gotErr)
		}
		if identity.Manifest != stage.Envelope.Payload.ExperimentDeliveryManifest {
			t.Fatalf("delivery manifest digest = %v, want envelope digest %v", identity.Manifest, stage.Envelope.Payload.ExperimentDeliveryManifest)
		}
	})

	t.Run("negative destination substitution is rejected before publication", func(t *testing.T) {
		t.Parallel()
		stage, pages, verifier := deliveryProtocolFixture(t)
		foreign, foreignErr := runprotocol.NewIdentifier("foreign-origin-api")
		if foreignErr != nil {
			t.Fatalf("runprotocol.NewIdentifier(foreign destination) error = %v, want nil", foreignErr)
		}
		verifier.Destination = foreign
		gotErr := verifier.Verify(stage, pages)
		if !errors.Is(gotErr, core.ErrPrimitiveContract) {
			t.Fatalf("ObservationDeliveryVerifier.Verify(foreign destination) error = %v, want errors.Is(..., %v)", gotErr, core.ErrPrimitiveContract)
		}
	})

	t.Run("neutral pre-runner delivery closes with zero pages and no invented experiment", func(t *testing.T) {
		t.Parallel()
		envelope, manifest, controlKeys, runnerKeys := preRunnerObservationDeliveryFixture(t)
		stage := runnercontrol.ObservationDeliveryStage{SchemaVersion: runnercontrol.SchemaVersion, Envelope: envelope, Manifest: manifest}
		verifier := runnercontrol.ObservationDeliveryVerifier{
			Origin: envelope.Payload.Origin, Destination: envelope.Payload.Destination,
			Audience: envelope.Payload.Audience, Grant: envelope.Payload.DeliveryGrant,
			ControlKeys: controlKeys, RunnerKeys: runnerKeys,
		}
		gotErr := verifier.Verify(stage, nil)
		if gotErr != nil || stage.Manifest.PageCount != 0 || len(stage.Manifest.Entries) != 0 {
			t.Fatalf("ObservationDeliveryVerifier.Verify(pre-runner) = (error %v, pages %d, entries %d), want (nil, 0, 0)", gotErr, stage.Manifest.PageCount, len(stage.Manifest.Entries))
		}
	})
}

func deliveryProtocolFixture(t testing.TB) (runnercontrol.ObservationDeliveryStage, []runnercontrol.ExperimentDeliveryPage, runnercontrol.ObservationDeliveryVerifier) {
	t.Helper()
	envelope, manifest, pages, controlKeys, runnerKeys := completedObservationDeliveryFixture(t)
	stage := runnercontrol.ObservationDeliveryStage{SchemaVersion: runnercontrol.SchemaVersion, Envelope: envelope, Manifest: manifest}
	verifier := runnercontrol.ObservationDeliveryVerifier{
		Origin: envelope.Payload.Origin, Destination: envelope.Payload.Destination,
		Audience: envelope.Payload.Audience, Grant: envelope.Payload.DeliveryGrant,
		ControlKeys: controlKeys, RunnerKeys: runnerKeys,
	}
	return stage, pages, verifier
}
