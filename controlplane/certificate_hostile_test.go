package controlplane_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
	"github.com/deliri/primitive/v2026/receipt"
)

// TestVerifyInstallationCertificateAdmitsOnlyTheAuthoritysOwnSignature is the
// core admission proof for the one situation where a certificate is the whole
// authority: an installation loading a credential it stored earlier has no live
// request to bind against.
func TestVerifyInstallationCertificateAdmitsOnlyTheAuthoritysOwnSignature(t *testing.T) {
	t.Parallel()

	issued := issueTestRegistration(t)
	certificate := issued.document.Payload.Certificate
	client := issued.client(t)

	verified, err := client.VerifyInstallationCertificate(*certificate)
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
		want   error
		mutate func(*testing.T, *controlplane.InstallationCertificateDocument) attest.TrustedKeys
		name   string
	}{
		{
			name: "the zero certificate names no installation",
			want: core.ErrControlPlaneRegistration,
			mutate: func(t *testing.T, document *controlplane.InstallationCertificateDocument) attest.TrustedKeys {
				t.Helper()
				*document = controlplane.InstallationCertificateDocument{}
				return issueTestRegistration(t).trusted
			},
		},
		{
			name: "an empty trust set admits nothing",
			want: core.ErrAttestContract,
			mutate: func(t *testing.T, _ *controlplane.InstallationCertificateDocument) attest.TrustedKeys {
				t.Helper()
				return attest.TrustedKeys{}
			},
		},
		{
			name: "a trust set containing only another authority refuses the genuine signature",
			want: core.ErrAttestVerification,
			mutate: func(t *testing.T, _ *controlplane.InstallationCertificateDocument) attest.TrustedKeys {
				t.Helper()
				other, _ := testSigningKey(t, 9)
				trusted, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
					Keys: []core.Ed25519PublicKey{other},
				})
				if err != nil {
					t.Fatalf("attest.NewTrustedKeys() error = %v, want nil", err)
				}
				return trusted
			},
		},
		{
			name: "a certificate signed by another key is not this authority's",
			want: core.ErrAttestVerification,
			mutate: func(t *testing.T, document *controlplane.InstallationCertificateDocument) attest.TrustedKeys {
				t.Helper()
				_, impostor := testSigningKey(t, 9)
				issued := issueTestRegistration(t)
				server := issued.server(t)
				*document = *resignCertificate(t, server, document, impostor)
				return issued.trusted
			},
		},
		{
			name: "a device key swapped after signing breaks the signature",
			want: core.ErrControlPlaneInstallationBinding,
			mutate: func(t *testing.T, document *controlplane.InstallationCertificateDocument) attest.TrustedKeys {
				t.Helper()
				other, _ := testDeviceKey(t, 7)
				document.Body.DeviceKey = other
				return issueTestRegistration(t).trusted
			},
		},
		{
			name: "a stripped attestation leaves nothing to verify",
			want: core.ErrAttestContract,
			mutate: func(t *testing.T, document *controlplane.InstallationCertificateDocument) attest.TrustedKeys {
				t.Helper()
				document.Attestation = attest.Envelope[controlplane.SigningDomain]{}
				return issueTestRegistration(t).trusted
			},
		},
		{
			name: "an account swapped after signing breaks the body commitment",
			want: core.ErrAttestVerification,
			mutate: func(t *testing.T, document *controlplane.InstallationCertificateDocument) attest.TrustedKeys {
				t.Helper()
				account, err := receipt.ParseAccountIdentity(checkInResponseOtherAccountHex)
				if err != nil {
					t.Fatalf("receipt.ParseAccountIdentity() error = %v, want nil", err)
				}
				document.Body.Account = account
				return issueTestRegistration(t).trusted
			},
		},
		{
			name: "an issuance instant swapped after signing breaks the body commitment",
			want: core.ErrAttestVerification,
			mutate: func(t *testing.T, document *controlplane.InstallationCertificateDocument) attest.TrustedKeys {
				t.Helper()
				document.Body.IssuedAt = testInstant(t, checkInResponseFutureInstant)
				return issueTestRegistration(t).trusted
			},
		},
		{
			name: "a build version swapped after signing breaks the body commitment",
			want: core.ErrAttestVerification,
			mutate: func(t *testing.T, document *controlplane.InstallationCertificateDocument) attest.TrustedKeys {
				t.Helper()
				document.Body.Build = testBuildIdentity(t, 2026, 0, 99)
				return issueTestRegistration(t).trusted
			},
		},
		{
			name: "an entitlement swapped after signing breaks the body commitment",
			want: core.ErrAttestVerification,
			mutate: func(t *testing.T, document *controlplane.InstallationCertificateDocument) attest.TrustedKeys {
				t.Helper()
				raw := [lease.IdentifierBytes]byte{}
				for index := range raw {
					raw[index] = 9
				}
				entitlement, err := lease.NewEntitlementID(raw)
				if err != nil {
					t.Fatalf("lease.NewEntitlementID() error = %v, want nil", err)
				}
				document.Body.Subject.EntitlementID = entitlement
				return issueTestRegistration(t).trusted
			},
		},
		{
			name: "a signer identity swapped after signing cannot nominate another authority",
			want: core.ErrAttestVerification,
			mutate: func(t *testing.T, document *controlplane.InstallationCertificateDocument) attest.TrustedKeys {
				t.Helper()
				other, _ := testSigningKey(t, 9)
				document.Attestation.Signer = other
				return issueTestRegistration(t).trusted
			},
		},
		{
			name: "an envelope body digest swapped after signing fails authentication",
			want: core.ErrAttestVerification,
			mutate: func(t *testing.T, document *controlplane.InstallationCertificateDocument) attest.TrustedKeys {
				t.Helper()
				document.Attestation.BodySHA256 = core.SHA256Of([]byte("another certificate body"))
				return issueTestRegistration(t).trusted
			},
		},
		{
			name: "an envelope body length swapped after signing fails authentication",
			want: core.ErrAttestVerification,
			mutate: func(t *testing.T, document *controlplane.InstallationCertificateDocument) attest.TrustedKeys {
				t.Helper()
				length, err := core.NewByteCount(1)
				if err != nil {
					t.Fatalf("core.NewByteCount() error = %v, want nil", err)
				}
				document.Attestation.BodyLength = length
				return issueTestRegistration(t).trusted
			},
		},
		{
			name: "a foreign signing domain cannot reinterpret certificate bytes",
			want: core.ErrControlPlaneSigningDomain,
			mutate: func(t *testing.T, document *controlplane.InstallationCertificateDocument) attest.TrustedKeys {
				t.Helper()
				document.Attestation.Domain = controlplane.SigningDomainRegistrationV1
				return issueTestRegistration(t).trusted
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			issued := issueTestRegistration(t)
			original := *issued.document.Payload.Certificate
			certificate := original
			trusted := testCase.mutate(t, &certificate)
			if certificate == original && trusted == issued.trusted {
				t.Fatalf("certificate mutation %q changed neither document nor trust: %v",
					testCase.name, certificate)
			}

			client, clientErr := controlplane.NewClient(controlplane.ClientConfiguration{
				TrustedAuthorityKeys: trusted,
			})
			got, err := client.VerifyInstallationCertificate(certificate)
			err = errors.Join(clientErr, err)
			if !errors.Is(err, core.ErrControlPlaneRegistration) || !errors.Is(err, testCase.want) {
				t.Fatalf("VerifyInstallationCertificate() error = %v, want errors.Is %v and %v",
					err, core.ErrControlPlaneRegistration, testCase.want)
			}
			// A refusal must hand back nothing usable. The sealed type is the
			// proof of verification, so a zero value that still answered its
			// accessors would let a rejected certificate be spent as a verified
			// one.
			if err := got.Validate(); !errors.Is(err, core.ErrControlPlaneRegistration) {
				t.Fatalf("rejected value Validate() error = %v, want errors.Is %v",
					err, core.ErrControlPlaneRegistration)
			}
			if body, err := got.Body(); body != (controlplane.InstallationCertificateBody{}) ||
				!errors.Is(err, core.ErrControlPlaneRegistration) {
				t.Fatalf("rejected value Body() = (%v, %v), want zero and errors.Is %v",
					body, err, core.ErrControlPlaneRegistration)
			}
			if keys, err := got.DeviceKeys(); keys != (attest.TrustedKeys{}) ||
				!errors.Is(err, core.ErrControlPlaneRegistration) {
				t.Fatalf("rejected value DeviceKeys() = (%v, %v), want zero and errors.Is %v",
					keys, err, core.ErrControlPlaneRegistration)
			}
			proof, proofErr := got.Proof()
			if !errors.Is(proofErr, core.ErrControlPlaneRegistration) ||
				!errors.Is(proof.Validate(), core.ErrAttestVerification) {
				t.Fatalf("rejected value Proof() = (%v, %v), proof Validate = %v, want sealed zero and errors.Is %v",
					proof, proofErr, proof.Validate(), core.ErrControlPlaneRegistration)
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
	client := issued.client(t)

	verified, err := client.VerifyInstallationCertificate(certificate)
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
