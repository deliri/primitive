package controlplane_test

import (
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
)

// TestVerifyInstallationCertificateAdmitsOnlyTheAuthoritysOwnSignature is the
// core admission proof for the one situation where a certificate is the whole
// authority: an installation loading a credential it stored earlier has no live
// request to bind against.
func TestVerifyInstallationCertificateAdmitsOnlyTheAuthoritysOwnSignature(t *testing.T) {
	t.Parallel()

	issued := issueTestRegistration(t)
	certificate := issued.document.Payload.Certificate

	verified, err := controlplane.VerifyInstallationCertificate(*certificate, issued.trusted)
	if err != nil {
		t.Fatalf("VerifyInstallationCertificate() error = %v, want nil", err)
	}
	body, err := verified.Body()
	if err != nil {
		t.Fatalf("Body() error = %v, want nil", err)
	}
	if body.DeviceKey != issued.deviceKey {
		t.Fatalf("Body().DeviceKey = %v, want %v", body.DeviceKey, issued.deviceKey)
	}

	// The returned trust set must hold the installation's key and nothing else.
	// This is the whole point of returning a set rather than a bare key: a
	// caller must not be able to check an installation's signature against the
	// authority's key, or against a set the caller assembled.
	deviceKeys, err := verified.DeviceKeys()
	if err != nil {
		t.Fatalf("DeviceKeys() error = %v, want nil", err)
	}
	wantKeys, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
		Keys: []core.Ed25519PublicKey{issued.deviceKey},
	})
	if err != nil {
		t.Fatalf("NewTrustedKeys() error = %v, want nil", err)
	}
	if deviceKeys != wantKeys {
		t.Fatalf("DeviceKeys() = %v, want exactly the installation key set %v", deviceKeys, wantKeys)
	}
	if deviceKeys == issued.trusted {
		t.Fatalf("DeviceKeys() = the authority trust set, want the installation's own key set")
	}
}

// TestVerifyInstallationCertificateRefusesEveryUnauthenticInput is the
// rejection table. Each case is a way a stored credential file could be wrong
// on disk or a way a forged one could arrive.
func TestVerifyInstallationCertificateRefusesEveryUnauthenticInput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		mutate func(*testing.T, *controlplane.InstallationCertificateDocument) attest.TrustedKeys
		name   string
	}{
		{
			name: "the zero certificate names no installation",
			mutate: func(t *testing.T, document *controlplane.InstallationCertificateDocument) attest.TrustedKeys {
				t.Helper()
				*document = controlplane.InstallationCertificateDocument{}
				return issueTestRegistration(t).trusted
			},
		},
		{
			name: "an empty trust set admits nothing",
			mutate: func(t *testing.T, _ *controlplane.InstallationCertificateDocument) attest.TrustedKeys {
				t.Helper()
				return attest.TrustedKeys{}
			},
		},
		{
			name: "a certificate signed by another key is not this authority's",
			mutate: func(t *testing.T, document *controlplane.InstallationCertificateDocument) attest.TrustedKeys {
				t.Helper()
				_, impostor := testSigningKey(t, 9)
				*document = *resignCertificate(t, document, impostor)
				return issueTestRegistration(t).trusted
			},
		},
		{
			name: "a device key swapped after signing breaks the signature",
			mutate: func(t *testing.T, document *controlplane.InstallationCertificateDocument) attest.TrustedKeys {
				t.Helper()
				other, _ := testDeviceKey(t, 7)
				document.Body.DeviceKey = other
				return issueTestRegistration(t).trusted
			},
		},
		{
			name: "a stripped attestation leaves nothing to verify",
			mutate: func(t *testing.T, document *controlplane.InstallationCertificateDocument) attest.TrustedKeys {
				t.Helper()
				document.Attestation = attest.Envelope[controlplane.SigningDomain]{}
				return issueTestRegistration(t).trusted
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			issued := issueTestRegistration(t)
			certificate := *issued.document.Payload.Certificate
			trusted := testCase.mutate(t, &certificate)

			got, err := controlplane.VerifyInstallationCertificate(certificate, trusted)
			if err == nil {
				t.Fatalf("VerifyInstallationCertificate() = %v, error = nil, want a refusal", got)
			}
			// A refusal must hand back nothing usable. The sealed type is the
			// proof of verification, so a zero value that still answered its
			// accessors would let a rejected certificate be spent as a verified
			// one.
			if err := got.Validate(); err == nil {
				t.Fatalf("rejected value Validate() = nil, want a refusal")
			}
			if _, err := got.Body(); err == nil {
				t.Fatalf("rejected value Body() error = nil, want a refusal")
			}
			if _, err := got.DeviceKeys(); err == nil {
				t.Fatalf("rejected value DeviceKeys() error = nil, want a refusal")
			}
			if _, err := got.Proof(); err == nil {
				t.Fatalf("rejected value Proof() error = nil, want a refusal")
			}
		})
	}
}

// TestVerifyInstallationCertificateIsTheRuleCheckInApplies proves the two
// callers share one implementation rather than two that agree today.
//
// The check-in path reaches the same rule through an unexported helper, so the
// only observable way to state the obligation is that a certificate admitted
// for a stored credential is exactly the certificate a check-in would accept,
// and the device key each derives is the same. If the helper ever stopped
// delegating, one of these two would drift and this comparison would catch it.
func TestVerifyInstallationCertificateIsTheRuleCheckInApplies(t *testing.T) {
	t.Parallel()

	issued := issueTestRegistration(t)
	certificate := *issued.document.Payload.Certificate

	verified, err := controlplane.VerifyInstallationCertificate(certificate, issued.trusted)
	if err != nil {
		t.Fatalf("VerifyInstallationCertificate() error = %v, want nil", err)
	}
	deviceKeys, err := verified.DeviceKeys()
	if err != nil {
		t.Fatalf("DeviceKeys() error = %v, want nil", err)
	}

	// A check-in signed by the installation must verify against exactly the key
	// set the certificate path derived, with no separate assembly step.
	wantKeys, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
		Keys: []core.Ed25519PublicKey{certificate.Body.DeviceKey},
	})
	if err != nil {
		t.Fatalf("NewTrustedKeys() error = %v, want nil", err)
	}
	if deviceKeys != wantKeys {
		t.Fatalf("DeviceKeys() = %v, want %v", deviceKeys, wantKeys)
	}

	// And the proof must survive revalidation, which is what makes possessing
	// the sealed value meaningful rather than decorative.
	proof, err := verified.Proof()
	if err != nil {
		t.Fatalf("Proof() error = %v, want nil", err)
	}
	if err := proof.Validate(); err != nil {
		t.Fatalf("Proof().Validate() error = %v, want nil", err)
	}
}
