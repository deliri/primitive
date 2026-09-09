package controlplane_test

import (
	"crypto/ed25519"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
)

func TestRegistrationAuthorityConsumesTokenOnEveryExit(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		zeroAuthority bool
		mutate        func(*controlplane.RegistrationAuthorityVerification)
		wantErr       error
	}{
		{name: "fresh authenticated request consumes secret"},
		{name: "zero authority still owns destruction", zeroAuthority: true, wantErr: core.ErrControlPlaneContract},
		{name: "missing verifier cannot retain presented secret", mutate: func(v *controlplane.RegistrationAuthorityVerification) {
			v.ExpectedVerifier = controlwire.RegistrationTokenVerifier{}
		}, wantErr: core.ErrControlWireToken},
		{name: "bad nonce cannot retain presented secret", mutate: func(v *controlplane.RegistrationAuthorityVerification) {
			v.Request.RequestNonce = controlwire.RequestNonce{}
		}, wantErr: core.ErrControlWireNonce},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := issueTestRegistration(t).server(t)
			if tc.zeroAuthority {
				server = controlplane.Authority{}
			}
			request := registrationRequestFixture(t)
			verifier, err := request.Token.Verifier()
			if err != nil {
				t.Fatalf("Verifier() error = %v, want nil", err)
			}
			input := controlplane.RegistrationAuthorityVerification{Request: request, ExpectedVerifier: verifier}
			if tc.mutate != nil {
				tc.mutate(&input)
			}
			got, gotErr := server.VerifyRegistrationAuthority(input)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Errorf("VerifyRegistrationAuthority() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil && got != (controlplane.VerifiedRegistrationAuthority{}) {
				t.Errorf("refused proof = %v, want zero", got)
			}
			if tc.wantErr == nil {
				if err := got.Validate(); err != nil {
					t.Errorf("accepted proof Validate() error = %v, want nil", err)
				}
			}
			if err := request.Token.Validate(); !errors.Is(err, core.ErrControlWireToken) {
				t.Fatalf("presented token after return = %v, want destroyed %v", err, core.ErrControlWireToken)
			}
		})
	}
}

func TestCheckInIssuanceRefusesForeignBindingWithoutPartialDocument(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		mutate  func(*testing.T, *controlplane.CheckInPayload, *controlplane.InstallationCertificateDocument, *ed25519.PrivateKey)
		wantErr error
	}{
		{name: "exact device key and certificate produce verifiable request"},
		{name: "foreign signer cannot issue under another device certificate", mutate: func(t *testing.T, _ *controlplane.CheckInPayload, _ *controlplane.InstallationCertificateDocument, key *ed25519.PrivateKey) {
			_, *key = testSigningKey(t, checkInOtherDeviceSeed)
		}, wantErr: core.ErrControlPlaneInstallationBinding},
		{name: "foreign certificate build cannot escape beside error", mutate: func(t *testing.T, _ *controlplane.CheckInPayload, c *controlplane.InstallationCertificateDocument, _ *ed25519.PrivateKey) {
			c.Body.Build = alternateBuild(t, c.Body.Build)
		}, wantErr: core.ErrControlPlaneDecisionConsistency},
		{name: "missing payload nonce signs nothing", mutate: func(_ *testing.T, p *controlplane.CheckInPayload, _ *controlplane.InstallationCertificateDocument, _ *ed25519.PrivateKey) {
			p.RequestNonce = controlwire.RequestNonce{}
		}, wantErr: core.ErrControlWireNonce},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			issued := issueTestCheckIn(t, controlplaneOffering(t, 3), testCheckInWindow())
			payload, certificate, key := issued.request.Payload, issued.certificate, issued.device
			if tc.mutate != nil {
				tc.mutate(t, &payload, &certificate, &key)
			}
			got, err := issued.client(t).IssueCheckIn(payload, key, certificate)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("IssueCheckIn() error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if !isZeroCheckInRequest(got) {
					t.Fatalf("refused IssueCheckIn() = %v, want zero document", got)
				}
				return
			}
			proof, err := issued.server(t).VerifyCheckIn(controlplane.CheckInVerification{Request: got})
			if err != nil {
				t.Fatalf("VerifyCheckIn(issued) error = %v, want nil", err)
			}
			request, err := proof.Request()
			if err != nil || request.Attestation != got.Attestation {
				t.Fatalf("verified request = (%v, %v), want exact issued attestation", request, err)
			}
		})
	}
}
