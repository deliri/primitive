package runnercontrol_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/temporal"
)

func FuzzMachineObservationSubmissionSemanticClosure(f *testing.F) {
	seed := machineObservationSubmissionFixture(f)
	canonical, err := seed.MarshalJSON()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		proveStructureJSONClosure(t, "MachineObservationSubmission", seed, data, (*runnercontrol.MachineObservationSubmission).UnmarshalJSON)
	})
}

func FuzzSchedulingClaimSemanticAuthentication(f *testing.F) {
	seed, trusted := schedulingClaimDocumentFixture(f)
	canonical, err := seed.MarshalJSON()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		proveStructureJSONClosure(t, "SchedulingClaim", seed, data, (*runnercontrol.SchedulingClaim).UnmarshalJSON)
		var got runnercontrol.SchedulingClaim
		if err := got.UnmarshalJSON(data); err != nil {
			return
		}
		err := runnercontrol.VerifySchedulingClaim(got, trusted)
		if err != nil {
			if !errors.Is(err, core.ErrAttestVerification) {
				t.Fatalf("VerifySchedulingClaim() error = %v, want typed authentication refusal", err)
			}
			return
		}
		// Collections may omit optional direct work. Every admitted signed document
		// must still be one of the independently issued seeds, never a forged value.
		sameSignedNominal(t, got.Capability, seed.Capability)
		for _, member := range got.Members {
			sameSignedNominal(t, member, seed.Members[0])
		}
		for _, direct := range got.Direct {
			sameSignedNominal(t, direct, seed.Direct[0])
		}
	})
}

func sameSignedNominal[T structureJSONValue](t *testing.T, got, want T) {
	t.Helper()
	encoded, err := got.MarshalJSON()
	canonical, wantErr := want.MarshalJSON()
	if err != nil || wantErr != nil || !bytes.Equal(encoded, canonical) {
		t.Fatalf("authenticated nominal document = %d bytes/%v, want exact signed seed %d bytes/%v", len(encoded), err, len(canonical), wantErr)
	}
}

func FuzzSourceAcquisitionSemanticClosure(f *testing.F) {
	_, projection := sourceAcquisitionSocketFixture(f)
	_, trusted := completionSignerFixture(f)
	canonical, err := core.MarshalCanonicalJSONDocument(projection)
	if err != nil {
		f.Fatal(err)
	}
	var seed runnercontrol.SourceAcquisition
	if err := seed.UnmarshalJSON(canonical); err != nil {
		f.Fatal(err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		got := seed
		err := got.UnmarshalJSON(data)
		if err != nil {
			if !errors.Is(err, core.ErrJSONContract) || !bytes.Equal(sourceAcquisitionCanonical(t, got), canonical) {
				t.Fatalf("SourceAcquisition refusal = %v, want typed refusal and preserved receiver", err)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("accepted SourceAcquisition.Validate() = %v, want nil", err)
		}
		encoded := sourceAcquisitionCanonical(t, got)
		var second runnercontrol.SourceAcquisition
		if err := second.UnmarshalJSON(encoded); err != nil || !bytes.Equal(sourceAcquisitionCanonical(t, second), encoded) {
			t.Fatalf("SourceAcquisition canonical closure error = %v, want exact typed round trip", err)
		}
		verification := runnercontrol.SourceArchiveVerification{Document: got.Document, TrustedKeys: trusted, ObservedAt: temporal.InstantFromNanoseconds(2)}
		verifyErr := runnercontrol.VerifySourceArchive(verification)
		if verifyErr == nil {
			sameSignedNominal(t, got.Document, seed.Document)
		} else if !errors.Is(verifyErr, core.ErrAttestVerification) && !errors.Is(verifyErr, core.ErrPrimitiveContract) {
			t.Fatalf("source verification = %v, want typed signature or interval refusal", verifyErr)
		}
	})
}

// The receiver intentionally cannot disclose its bearer through MarshalJSON.
// The test explicitly constructs the real issue-side type for round-trip proof.
func sourceAcquisitionCanonical(t *testing.T, value runnercontrol.SourceAcquisition) []byte {
	t.Helper()
	target, targetErr := value.Capability.Target()
	provider, providerErr := value.Capability.Provider()
	capability, capabilityErr := objectstore.NewDownloadCapabilityProjection(provider, target)
	if err := errors.Join(targetErr, providerErr, capabilityErr); err != nil {
		t.Fatalf("source issue projection setup = %v, want nil", err)
	}
	projection := runnercontrol.SourceAcquisitionProjection{SchemaVersion: value.SchemaVersion, ContentType: value.ContentType, Members: value.Members, Grant: value.Grant, Capability: capability, Document: value.Document, Fence: value.Fence, Integrity: value.Integrity, Policy: value.Policy}
	if err := projection.Validate(); err != nil {
		t.Fatalf("source issue projection Validate() = %v, want nil", err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(projection)
	if err != nil {
		t.Fatalf("source canonical projection = %v, want nil", err)
	}
	return encoded
}
