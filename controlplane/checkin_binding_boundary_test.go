package controlplane_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
)

func TestReceivedCheckInSignerBindingLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		mutate        func(*testing.T, *controlplane.CheckInRequest)
		wantErr       error
		wantVerifyErr error
	}{
		{name: "exact certified signer survives decode projection and authentication"},
		{name: "authentic foreign signature cannot enter this certificate boundary", mutate: func(t *testing.T, request *controlplane.CheckInRequest) {
			_, key := testSigningKey(t, checkInOtherDeviceSeed)
			envelope, err := attest.Sign(attest.SignRequest[controlplane.SigningDomain]{Body: request.Payload, Signer: key})
			if err != nil {
				t.Fatalf("Sign(foreign) error = %v, want nil", err)
			}
			request.Attestation = envelope
		}, wantErr: core.ErrControlPlaneInstallationBinding, wantVerifyErr: core.ErrControlPlaneInstallationBinding},
		{name: "changing only signer cannot hide behind later signature verification", mutate: func(t *testing.T, request *controlplane.CheckInRequest) {
			request.Attestation.Signer, _ = testSigningKey(t, checkInOtherDeviceSeed)
		}, wantErr: core.ErrControlPlaneInstallationBinding, wantVerifyErr: core.ErrControlPlaneInstallationBinding},
		{name: "matching signer does not turn corrupted signature into proof", mutate: func(_ *testing.T, request *controlplane.CheckInRequest) {
			request.Attestation.Signature = request.Certificate.Attestation.Signature
		}, wantVerifyErr: core.ErrAttestVerification},
		{name: "foreign signing domain retains the check-in refusal identity", mutate: func(_ *testing.T, request *controlplane.CheckInRequest) {
			request.Attestation.Domain = controlplane.SigningDomainRegistrationV1
		}, wantErr: core.ErrControlPlaneDecisionConsistency, wantVerifyErr: core.ErrControlPlaneDecisionConsistency},
		{name: "absent attestation creates no admitted request", mutate: func(_ *testing.T, request *controlplane.CheckInRequest) {
			request.Attestation = attest.Envelope[controlplane.SigningDomain]{}
		}, wantErr: core.ErrControlPlaneCheckIn, wantVerifyErr: core.ErrControlPlaneCheckIn},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			issued := issueTestCheckIn(t, controlplaneOffering(t, 3), testCheckInWindow())
			request := issued.request
			if tc.mutate != nil {
				tc.mutate(t, &request)
			}
			if tc.mutate != nil && request.Attestation == issued.request.Attestation {
				t.Fatalf("attestation mutation = %v, want changed signed fact", request.Attestation)
			}
			if got := request.Validate(); !errors.Is(got, tc.wantErr) {
				t.Errorf("Validate() error = %v, want %v", got, tc.wantErr)
			}
			encoded, marshalErr := request.MarshalJSON()
			if !errors.Is(marshalErr, tc.wantErr) || (tc.wantErr != nil && len(encoded) != 0) {
				t.Errorf("MarshalJSON() = (%d bytes, %v), want refusal %v with zero bytes or exact admitted projection", len(encoded), marshalErr, tc.wantErr)
			}
			var wire []byte
			if request.Attestation == (attest.Envelope[controlplane.SigningDomain]{}) {
				wire = []byte("null") // Deliberately absent hostile input, not a valid fixture.
			} else {
				wire = checkInAttestationMutationJSON(t, issued.request, request.Attestation)
			}
			for _, initial := range []struct {
				name    string
				request controlplane.CheckInRequest
			}{
				{"populated receiver keeps original facts on refusal", issued.request},
				{"empty receiver cannot acquire partial facts", controlplane.CheckInRequest{}},
			} {
				t.Run(initial.name, func(t *testing.T) {
					t.Parallel()
					got := initial.request
					decodeErr := got.UnmarshalJSON(wire)
					if !errors.Is(decodeErr, tc.wantErr) {
						t.Fatalf("UnmarshalJSON() error = %v, want %v", decodeErr, tc.wantErr)
					}
					if tc.wantErr != nil {
						if !errors.Is(decodeErr, core.ErrJSONContract) || !errors.Is(decodeErr, core.ErrControlPlaneCheckIn) {
							t.Fatalf("decode refusal = %v, want JSON and check-in identities", decodeErr)
						}
						if isZeroCheckInRequest(initial.request) {
							if !isZeroCheckInRequest(got) {
								t.Fatalf("refused receiver = %v, want zero", got)
							}
						} else {
							retained, err := got.MarshalJSON()
							original := mustCheckInJSON(t, initial.request)
							if err != nil || !bytes.Equal(retained, original) {
								t.Fatalf("retained receiver = (%d bytes, %v), want exact original %d bytes", len(retained), err, len(original))
							}
						}
						return
					}
					projected, err := got.MarshalJSON()
					if err != nil || !bytes.Equal(projected, encoded) {
						t.Fatalf("admitted projection = (%d bytes, %v), want exact %d bytes", len(projected), err, len(encoded))
					}
				})
			}
			proof, verifyErr := issued.server(t).VerifyCheckIn(controlplane.CheckInVerification{Request: request})
			if !errors.Is(verifyErr, tc.wantVerifyErr) {
				t.Fatalf("VerifyCheckIn() error = %v, want %v", verifyErr, tc.wantVerifyErr)
			}
			observed, observationErr := proof.Request()
			if tc.wantVerifyErr != nil {
				if !isZeroCheckInRequest(observed) || !errors.Is(observationErr, core.ErrControlPlaneCheckIn) {
					t.Fatalf("refused proof Request() = (%v, %v), want zero and check-in refusal", observed, observationErr)
				}
			} else if observationErr != nil || observed.Attestation != request.Attestation {
				t.Fatalf("authenticated observation = (%v, %v), want exact attestation", observed.Attestation, observationErr)
			}
		})
	}
}

// Mutate the real typed envelope inside a real emitted request. The enclosing
// document is deliberately hostile; its production marshaler must refuse it.
func checkInAttestationMutationJSON(t testing.TB, original controlplane.CheckInRequest, envelope attest.Envelope[controlplane.SigningDomain]) []byte {
	t.Helper()
	encoded := mustCheckInJSON(t, original)
	before, beforeErr := original.Attestation.MarshalJSON()
	after, afterErr := envelope.MarshalJSON()
	if beforeErr != nil || afterErr != nil || bytes.Count(encoded, before) != 1 {
		t.Fatalf("envelope mutation = (%v, %v, %d occurrences), want two typed encodings and one original envelope", beforeErr, afterErr, bytes.Count(encoded, before))
	}
	return bytes.Replace(encoded, before, after, 1)
}
