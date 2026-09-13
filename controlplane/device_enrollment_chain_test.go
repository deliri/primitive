package controlplane_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
)

// This is a direct cryptographic chain ratchet, not database or quota proof.
// The product must atomically persist Replay under the enrollment-token record.
func TestDeviceEnrollmentCryptographicChainRejectsRebindingAndImpersonation(t *testing.T) {
	t.Parallel()
	issued := issueTestCheckIn(t, controlplaneOffering(t, 3), testCheckInWindow())
	defer clear(issued.device)
	defer clear(issued.authority)
	server := issued.server(t)
	token, err := controlwire.NewRegistrationToken([controlwire.RegistrationTokenBytes]byte{83})
	if err != nil {
		t.Fatalf("NewRegistrationToken() error = %v, want nil", err)
	}
	defer token.Destroy()
	request := controlplane.RegistrationRequest{
		Token: token, Build: issued.certificate.Body.Build,
		DeviceKey: issued.certificate.Body.DeviceKey, Installation: issued.subject.DeviceID,
		RequestNonce: issued.request.Payload.RequestNonce, Revision: issued.certificate.Body.Revision,
	}
	wantIdentity, err := request.Identity()
	if err != nil {
		t.Fatalf("Identity() error = %v, want nil", err)
	}
	verifier, err := token.Verifier()
	if err != nil {
		t.Fatalf("Verifier() error = %v, want nil", err)
	}
	canonical, err := request.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v, want nil", err)
	}
	defer clear(canonical)
	proof, err := server.VerifyRegistrationAuthority(controlplane.RegistrationAuthorityVerification{Request: request, ExpectedVerifier: verifier})
	if err != nil {
		t.Fatalf("VerifyRegistrationAuthority(first use) error = %v, want nil", err)
	}
	identity, err := proof.Identity()
	if err != nil || identity != wantIdentity {
		t.Fatalf("Identity(first use) = (%+v, %v), want (%+v, nil)", identity, err, wantIdentity)
	}
	replay, disposition, err := proof.Replay()
	if err != nil || disposition != controlwire.ReplayDispositionFresh {
		t.Fatalf("Replay(first use) = (%v, %v), want (fresh, nil)", disposition, err)
	}
	certificate, err := server.IssueRegisteredInstallation(controlplane.RegistrationCertificateIssuance{
		Registration: proof, IssuedAt: issued.certificate.Body.IssuedAt,
		Account: issued.certificate.Body.Account, Entitlement: issued.subject.EntitlementID,
	}, issued.authority)
	if err != nil || certificate.Body != issued.certificate.Body {
		t.Fatalf("IssueRegisteredInstallation() = (%+v, %v), want (%+v, nil)", certificate.Body, err, issued.certificate.Body)
	}
	checkIn, err := issued.client(t).IssueCheckIn(issued.request.Payload, issued.device, certificate)
	if err != nil {
		t.Fatalf("IssueCheckIn(bound private key) error = %v, want nil", err)
	}
	verified, err := server.VerifyCheckIn(controlplane.CheckInVerification{Request: checkIn})
	if err != nil {
		t.Fatalf("VerifyCheckIn(bound private key) error = %v, want nil", err)
	}
	got, err := verified.Request()
	if err != nil || got.Certificate.Body != certificate.Body || got.Payload.Installation != identity.Installation {
		t.Fatalf("Request(verified) = (%+v, %v), want exact issued certificate and installation %v", got.Certificate.Body, err, identity.Installation)
	}

	// An exact delivery retry is neutral: it retains the original commitment.
	var retry controlplane.RegistrationRequest
	if err := retry.UnmarshalJSON(canonical); err != nil {
		t.Fatalf("UnmarshalJSON(retry) error = %v, want nil", err)
	}
	exact, err := server.VerifyRegistrationAuthority(controlplane.RegistrationAuthorityVerification{Request: retry, ExpectedVerifier: verifier, PriorReplay: &replay})
	if err != nil {
		t.Fatalf("VerifyRegistrationAuthority(retry) error = %v, want nil", err)
	}
	gotReplay, disposition, err := exact.Replay()
	if err != nil || disposition != controlwire.ReplayDispositionExact || !gotReplay.Equal(replay) {
		t.Fatalf("Replay(retry) = (%v, %v, %v), want unchanged commitment and exact", gotReplay, disposition, err)
	}

	// One semantic mutation: another installation's key (and its derived ID).
	foreignPublic, foreignPrivate := testSigningKey(t, 84)
	defer clear(foreignPrivate)
	var foreign controlplane.RegistrationRequest
	if err := foreign.UnmarshalJSON(canonical); err != nil {
		t.Fatalf("UnmarshalJSON(foreign installation) error = %v, want nil", err)
	}
	foreign.DeviceKey = foreignPublic
	foreign.Installation, err = lease.DeviceIDForPublicKey(foreignPublic)
	if err != nil || foreign.Installation == identity.Installation {
		t.Fatalf("foreign installation = (%v, %v), want distinct valid identity", foreign.Installation, err)
	}
	refused, err := server.VerifyRegistrationAuthority(controlplane.RegistrationAuthorityVerification{Request: foreign, ExpectedVerifier: verifier, PriorReplay: &replay})
	if !errors.Is(err, core.ErrControlWireReplayConflict) || !errors.Is(err, core.ErrControlPlaneRegistration) || refused != (controlplane.VerifiedRegistrationAuthority{}) {
		t.Fatalf("VerifyRegistrationAuthority(second installation) = (%v, %v), want zero proof and %v", refused, err, core.ErrControlWireReplayConflict)
	}

	// Knowing the public identity and copying the certificate is insufficient.
	forged := checkIn
	forged.Attestation, err = attest.Sign(attest.SignRequest[controlplane.SigningDomain]{Body: checkIn.Payload, Signer: foreignPrivate})
	if err != nil {
		t.Fatalf("Sign(foreign private key) error = %v, want nil", err)
	}
	// Claim the enrolled public key while retaining the foreign signature.
	// This reaches cryptographic verification, not just signer-field validation.
	forged.Attestation.Signer = certificate.Body.DeviceKey
	if forged.Attestation == checkIn.Attestation {
		t.Fatalf("forged attestation = %v, want changed signature under unchanged claimed signer", forged.Attestation)
	}
	if err := forged.Validate(); err != nil {
		t.Fatalf("forged.Validate() error = %v, want structurally valid input", err)
	}
	denied, err := server.VerifyCheckIn(controlplane.CheckInVerification{Request: forged})
	if !errors.Is(err, core.ErrAttestVerification) || !errors.Is(err, core.ErrControlPlaneCheckIn) {
		t.Fatalf("VerifyCheckIn(foreign private key) error = %v, want %v", err, core.ErrAttestVerification)
	}
	if got, err := denied.Request(); !errors.Is(err, core.ErrControlPlaneCheckIn) || got.Certificate != (controlplane.InstallationCertificateDocument{}) {
		t.Fatalf("Request(refused) = (%v, %v), want no certificate and typed refusal", got.Certificate, err)
	}
	requireZeroVerifiedCheckIn(t, denied)
}
