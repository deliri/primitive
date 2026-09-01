package runnercontrol_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
)

type capabilityDocumentJSONValue interface {
	Validate() error
	MarshalJSON() ([]byte, error)
}

func FuzzCapabilityDocumentsSemanticClosure(f *testing.F) {
	claim, trusted := schedulingClaimDocumentFixture(f)
	addCapabilityDocumentSeed(f, 0, claim.Capability)
	addCapabilityDocumentSeed(f, 1, claim.Members[0])
	addCapabilityDocumentSeed(f, 2, claim.Direct[0])
	addCapabilityDocumentSeed(f, 3, claim)
	for selector := range uint8(4) {
		f.Add(selector, []byte{})
		f.Add(selector, []byte(`{}`))
		f.Add(selector, []byte(`{"payload":`))
	}

	f.Fuzz(func(t *testing.T, selector uint8, data []byte) {
		switch selector % 4 {
		case 0:
			proveCapabilityDocumentClosure(t, "SchedulingCapabilityDocument", claim.Capability, data, (*runnercontrol.SchedulingCapabilityDocument).UnmarshalJSON, runnercontrol.VerifySchedulingCapability, trusted)
		case 1:
			proveCapabilityDocumentClosure(t, "MemberCapabilityDocument", claim.Members[0], data, (*runnercontrol.MemberCapabilityDocument).UnmarshalJSON, runnercontrol.VerifyMemberCapability, trusted)
		case 2:
			proveCapabilityDocumentClosure(t, "ExperimentCapabilityDocument", claim.Direct[0], data, (*runnercontrol.ExperimentCapabilityDocument).UnmarshalJSON, runnercontrol.VerifyExperimentCapability, trusted)
		case 3:
			proveCapabilityDocumentClosure(t, "SchedulingClaim", claim, data, (*runnercontrol.SchedulingClaim).UnmarshalJSON, runnercontrol.VerifySchedulingClaim, trusted)
		}
	})
}

func addCapabilityDocumentSeed[T capabilityDocumentJSONValue](f *testing.F, selector uint8, value T) {
	f.Helper()
	encoded, err := value.MarshalJSON()
	if err != nil {
		f.Fatalf("capability document selector %d MarshalJSON(seed) error = %v, want nil", selector, err)
	}
	f.Add(selector, encoded)
}

func proveCapabilityDocumentClosure[T capabilityDocumentJSONValue](
	t *testing.T,
	name string,
	seed T,
	data []byte,
	decode func(*T, []byte) error,
	verify func(T, attest.TrustedKeys) error,
	trusted attest.TrustedKeys,
) {
	t.Helper()
	canonical, err := seed.MarshalJSON()
	if err != nil {
		t.Fatalf("%s.MarshalJSON(seed) error = %v, want nil", name, err)
	}
	got := seed
	gotErr := decode(&got, data)
	if gotErr != nil {
		after, marshalErr := got.MarshalJSON()
		if !errors.Is(gotErr, core.ErrJSONContract) || marshalErr != nil || !bytes.Equal(after, canonical) {
			t.Fatalf("%s.UnmarshalJSON(rejected) = (receiver %q, marshal error %v, error %v), want preserved %q and errors.Is(..., %v)", name, after, marshalErr, gotErr, canonical, core.ErrJSONContract)
		}
		return
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("%s.UnmarshalJSON(accepted).Validate() error = %v, want nil", name, err)
	}
	encoded, err := got.MarshalJSON()
	if err != nil || len(encoded) > core.JSONDocumentMaximumBytes {
		t.Fatalf("%s.MarshalJSON(accepted) = (%d bytes, %v), want <= %d bytes and nil", name, len(encoded), err, core.JSONDocumentMaximumBytes)
	}
	var roundTrip T
	if err := decode(&roundTrip, encoded); err != nil {
		t.Fatalf("%s canonical round trip error = %v, want nil", name, err)
	}
	second, err := roundTrip.MarshalJSON()
	if err != nil || !bytes.Equal(second, encoded) {
		t.Fatalf("%s second canonical projection = (%q, %v), want (%q, nil)", name, second, err, encoded)
	}
	verifyErr := verify(got, trusted)
	if verifyErr == nil && !bytes.Equal(encoded, canonical) {
		t.Fatalf("%s verification accepted mutated bytes %q, want only genuinely signed seed %q", name, encoded, canonical)
	}
	if verifyErr != nil && !errors.Is(verifyErr, core.ErrAttestVerification) {
		t.Fatalf("%s verification error = %v, want nil or %v", name, verifyErr, core.ErrAttestVerification)
	}
}
