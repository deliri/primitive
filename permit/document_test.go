package permit

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestPermitVerificationLayerTriad(t *testing.T) {
	t.Parallel()
	request, signer := permitFixture(t)
	got, err := Verify(request)
	if err != nil || got.Allows(permitAction(t, "operation-a"), request.EffectiveAt) != nil || !errors.Is(got.Allows(permitAction(t, "operation-b"), request.EffectiveAt), core.ErrPermitAction) {
		t.Fatalf("Verify()/start/green = %v/%v/%v, want nil/nil/core.ErrPermitAction", err, got.Allows(permitAction(t, "operation-a"), request.EffectiveAt), got.Allows(permitAction(t, "operation-b"), request.EffectiveAt))
	}
	changed := request
	changed.Document.Terms.Actions = Actions{}
	if changed.Document.Terms == request.Document.Terms {
		t.Fatal("mutation changed terms = false, want true")
	}
	refused, err := Verify(changed)
	if !errors.Is(err, core.ErrPermitAuthentication) || refused != (Verified{}) {
		t.Fatalf("unsigned mutation = %v/%v, want zero/core.ErrPermitAuthentication", refused, err)
	}
	changed.Document, err = Sign(changed.Document.Terms, signer)
	if err != nil {
		t.Fatalf("Sign(empty skills) = %v, want nil", err)
	}
	neutral, err := Verify(changed)
	if err != nil || !errors.Is(neutral.Allows(permitAction(t, "operation-a"), request.EffectiveAt), core.ErrPermitAction) {
		t.Fatalf("signed empty set = %v/%v, want nil/core.ErrPermitAction", err, neutral.Allows(permitAction(t, "operation-a"), request.EffectiveAt))
	}
}

func TestPermitSignedValidityBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		at      int64
		wantErr error
	}{
		{"before activation", 99, core.ErrPermitValidity}, {"exact activation", 100, nil},
		{"after activation", 101, nil}, {"before contact", 149, nil},
		{"exact contact does not revoke permission", 150, nil}, {"after contact", 151, nil},
		{"before expiry", 199, nil}, {"exact expiry", 200, core.ErrPermitValidity},
		{"after expiry", 201, core.ErrPermitValidity}, {"minimum instant", -1 << 63, core.ErrPermitValidity}, {"maximum instant", 1<<63 - 1, core.ErrPermitValidity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request, _ := permitFixture(t)
			request.EffectiveAt = temporal.InstantFromNanoseconds(tc.at)
			got, err := Verify(request)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Verify(%d) = %v, want %v", tc.at, err, tc.wantErr)
			}
			if err != nil && got != (Verified{}) {
				t.Fatalf("refused proof = %v, want zero", got)
			}
		})
	}
}

func TestPermitPreviouslyVerifiedProofExpiresAtInvocation(t *testing.T) {
	t.Parallel()
	request, _ := permitFixture(t)
	proof, err := Verify(request)
	if err != nil {
		t.Fatalf("Verify before expiry = %v, want nil", err)
	}
	for _, tc := range []struct {
		name    string
		at      int64
		wantErr error
	}{
		{"before signed start", 99, core.ErrPermitValidity},
		{"exact signed start", 100, nil},
		{"last valid nanosecond", 199, nil},
		{"exact expiry invalidates retained proof", 200, core.ErrPermitValidity},
		{"after expiry cannot reuse retained proof", 201, core.ErrPermitValidity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := proof.Allows(permitAction(t, "operation-a"), temporal.InstantFromNanoseconds(tc.at))
			if !errors.Is(got, tc.wantErr) {
				t.Fatalf("Allows at %d = %v, want %v", tc.at, got, tc.wantErr)
			}
		})
	}
}

func TestPermitForeignInstallationAndTrustRefuse(t *testing.T) {
	t.Parallel()
	request, _ := permitFixture(t)
	foreign, err := lease.NewDeviceID([16]byte{99})
	if err != nil {
		t.Fatalf("NewDeviceID() = %v, want nil", err)
	}
	request.Subject.DeviceID = foreign
	got, err := Verify(request)
	if !errors.Is(err, core.ErrPermitBinding) || got != (Verified{}) {
		t.Fatalf("foreign installation = %v/%v, want zero/core.ErrPermitBinding", got, err)
	}
	request.Subject = request.Document.Terms.Subject
	request.TrustedKeys = attest.TrustedKeys{}
	got, err = Verify(request)
	if !errors.Is(err, core.ErrPermitAuthentication) || got != (Verified{}) {
		t.Fatalf("empty trust = %v/%v, want zero/core.ErrPermitAuthentication", got, err)
	}
}

func FuzzPermitDecodeSignedSemanticClosure(f *testing.F) {
	request, key := permitFixture(f)
	withReporting := request.Document.Terms
	withReporting.Reporting = reportSchedule(f)
	var signErr error
	request.Document, signErr = Sign(withReporting, key)
	if signErr != nil {
		f.Fatalf("Sign(reporting grant) error = %v, want nil", signErr)
	}
	// Absence is a genuine signed product state, not an authentication failure.
	// Keep both compiler-produced seeds so retained corpus from either state has
	// an independent authentication oracle.
	withoutReporting := request.Document.Terms
	withoutReporting.Reporting = ReportSchedule{}
	plain, err := Sign(withoutReporting, key)
	if err != nil {
		f.Fatalf("Sign(no reporting grant) error = %v, want nil", err)
	}
	plainBytes, err := plain.MarshalJSON()
	if err != nil {
		f.Fatalf("MarshalJSON(no reporting grant) error = %v, want nil", err)
	}
	f.Add(plainBytes, uint8(0))
	var seed bytes.Buffer
	if err := request.Document.Write(&seed); err != nil {
		f.Fatalf("Write(seed) = %v, want nil", err)
	}
	for selector := uint8(0); selector < 15; selector++ {
		f.Add(seed.Bytes(), selector)
	}
	f.Add([]byte{}, uint8(0))
	f.Add([]byte(`{"terms":null}`), uint8(0))
	f.Fuzz(func(t *testing.T, data []byte, selector uint8) {
		got, err := Decode(bytes.NewReader(data))
		if err != nil {
			if !errors.Is(err, core.ErrPermitContract) || got != (Document{}) {
				t.Fatalf("Decode rejection = %v/%v, want zero/core.ErrPermitContract", got, err)
			}
			return
		}
		var first, second bytes.Buffer
		if err := got.Write(&first); err != nil || first.Len() > DocumentMaximumBytes {
			t.Fatalf("Write = %d/%v, want bounded/nil", first.Len(), err)
		}
		roundTrip, err := Decode(bytes.NewReader(first.Bytes()))
		if err != nil || roundTrip != got {
			t.Fatalf("round trip = %v/%v, want %v/nil", roundTrip, err, got)
		}
		if err := roundTrip.Write(&second); err != nil || !bytes.Equal(first.Bytes(), second.Bytes()) {
			t.Fatalf("canonical second write equal/error = %v/%v, want true/nil", bytes.Equal(first.Bytes(), second.Bytes()), err)
		}
		candidate := request
		candidate.Document = got
		proof, verifyErr := Verify(candidate)
		if got == request.Document || got == plain {
			if verifyErr != nil || proof.Allows(permitAction(t, "operation-a"), request.EffectiveAt) != nil {
				t.Fatalf("authentic seed = %v, want valid start permission", verifyErr)
			}
			if got == plain {
				selector %= 9
			}
			candidate.Document.Terms = mutatePermitTerm(t, got.Terms, selector)
			mutated, mutationErr := Verify(candidate)
			if !errors.Is(mutationErr, core.ErrPermitAuthentication) || mutated != (Verified{}) {
				t.Fatalf("structurally valid signed mutation = %v/%v, want zero/core.ErrPermitAuthentication", mutated, mutationErr)
			}
		} else if !errors.Is(verifyErr, core.ErrPermitAuthentication) || proof != (Verified{}) {
			t.Fatalf("unsigned recombination = %v/%v, want zero/core.ErrPermitAuthentication", proof, verifyErr)
		}
	})
}

func FuzzPermitDomainText(f *testing.F) {
	seed, err := DomainV1.MarshalText()
	if err != nil {
		f.Fatalf("MarshalText seed = %v, want nil", err)
	}
	f.Add(seed)
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := (Domain(0)).ParseCanonicalText(data)
		if bytes.Equal(data, seed) {
			if err != nil || got != DomainV1 {
				t.Fatalf("ParseCanonicalText = %v/%v, want DomainV1/nil", got, err)
			}
		} else if !errors.Is(err, core.ErrPermitContract) || got != 0 {
			t.Fatalf("unknown domain = %v/%v, want zero/core.ErrPermitContract", got, err)
		}
	})
}

func FuzzPermitRevisionJSON(f *testing.F) {
	seed, err := RevisionV1.MarshalJSON()
	if err != nil {
		f.Fatalf("MarshalJSON seed = %v, want nil", err)
	}
	f.Add(seed)
	f.Add([]byte("null"))
	f.Add([]byte("256"))
	f.Fuzz(func(t *testing.T, data []byte) {
		got := RevisionV1
		err := got.UnmarshalJSON(data)
		if err != nil {
			if !errors.Is(err, core.ErrPermitRevision) || got != RevisionV1 {
				t.Fatalf("revision rejection = %v/%v, want unchanged/core.ErrPermitRevision", got, err)
			}
			return
		}
		encoded, err := got.MarshalJSON()
		if err != nil || got != RevisionV1 || !bytes.Equal(encoded, seed) {
			t.Fatalf("admitted revision = %v/%q/%v, want V1/canonical/nil", got, encoded, err)
		}
	})
}

func TestVerifiedTermsRemainBoundToAuthenticatedSource(t *testing.T) {
	t.Parallel()
	request, _ := permitFixture(t)
	verified, err := Verify(request)
	if err != nil {
		t.Fatalf("Verify = %v, want nil", err)
	}
	got, err := verified.Terms()
	if err != nil || got != request.Document.Terms {
		t.Fatalf("Terms = %v/%v, want exact signed terms/nil", got, err)
	}
	got.Actions = Actions{}
	after, err := verified.Terms()
	if err != nil || after != request.Document.Terms || verified.Allows(permitAction(t, "operation-a"), request.EffectiveAt) != nil {
		t.Fatalf("Terms after caller mutation = %v/%v, want original authenticated permission", after, err)
	}
	empty, err := (Verified{}).Terms()
	if empty != (Terms{}) || !errors.Is(err, core.ErrPermitAuthentication) {
		t.Fatalf("zero proof Terms = %v/%v, want zero/core.ErrPermitAuthentication", empty, err)
	}
}
