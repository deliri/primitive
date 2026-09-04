package controlplane_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
)

// issuedRegistration is one real signed registration and everything a client
// needs to verify it. Nothing here is a stand-in: the keys are real Ed25519
// keys, the signatures are produced by Attest over the real canonical bytes,
// and verification runs the same path a customer binary runs.
type issuedRegistration struct {
	expectation controlplane.ResponseExpectation
	build       core.BuildIdentity
	document    controlplane.RegistrationDocument
	trusted     attest.TrustedKeys
	deviceKey   core.Ed25519PublicKey
}

// verification returns the request a well-behaved client would submit.
func (i issuedRegistration) verification() controlplane.RegistrationVerification {
	return controlplane.RegistrationVerification{
		Document: i.document, Expected: i.expectation,
		Build: i.build, DeviceKey: i.deviceKey,
	}
}

func (i issuedRegistration) client(t testing.TB) controlplane.Client {
	t.Helper()
	return testControlplaneClient(t, i.trusted)
}

func (i issuedRegistration) server(t testing.TB) controlplane.Authority {
	t.Helper()
	return testControlplaneServer(t, i.trusted)
}

// TestVerifyRegistrationAcceptsOnlyAResponseToThisExactRequest is the end-to-end
// proof. It signs a real registration and then attacks the verification with
// every substitution a hostile or confused authority could make.
//
// Each rejection case changes exactly one fact and keeps the signature valid
// where it can, because the failures that matter are not malformed bytes. They
// are authentic documents that belong to a different request, a different
// machine, or a different binary.
func TestVerifyRegistrationAcceptsOnlyAResponseToThisExactRequest(t *testing.T) {
	t.Parallel()

	issued := issueTestRegistration(t)
	client := issued.client(t)

	t.Run("the genuine response verifies", func(t *testing.T) {
		t.Parallel()

		verified, err := client.VerifyRegistration(issued.verification())
		if err != nil {
			t.Fatalf("VerifyRegistration() error = %v, want nil", err)
		}
		payload, err := verified.Payload()
		if err != nil {
			t.Fatalf("Payload() error = %v, want nil", err)
		}
		if got := payload.Header.Status.AdmitsGrant(); !got {
			t.Errorf("verified grant status %v AdmitsGrant() = %t, want true",
				payload.Header.Status, got)
		}
		if _, err := verified.Lease(); err != nil {
			t.Errorf("Lease() error = %v, want nil", err)
		}
		if _, err := verified.Certificate(); err != nil {
			t.Errorf("Certificate() error = %v, want nil", err)
		}
	})

	t.Run("authenticated certificate ownership survives input and accessor mutation", func(t *testing.T) {
		t.Parallel()

		isolated := issueTestRegistration(t)
		request := isolated.verification()
		want := *request.Document.Payload.Certificate
		verified, err := isolated.client(t).VerifyRegistration(request)
		if err != nil {
			t.Fatalf("VerifyRegistration() error = %v, want nil", err)
		}
		request.Document.Payload.Certificate.Body = controlplane.InstallationCertificateBody{}
		payload, err := verified.Payload()
		if err != nil {
			t.Fatalf("VerifiedRegistration.Payload() error = %v, want nil", err)
		}
		payload.Certificate.Body = controlplane.InstallationCertificateBody{}
		got, err := verified.Certificate()
		if err != nil || got != want {
			t.Fatalf("VerifiedRegistration.Certificate(after mutation) = (%v, %v), want original authenticated certificate", got, err)
		}
	})

	t.Run("a response to another request is refused by nonce", func(t *testing.T) {
		t.Parallel()

		request := issued.verification()
		request.Expected.RequestNonce = otherRequestNonce(t)

		requireBindingRefusal(t, client, request, controlplane.ResponseHeaderFieldRequestNonce)
	})

	t.Run("a response for another installation is refused", func(t *testing.T) {
		t.Parallel()

		request := issued.verification()
		_, otherDevice := testDeviceKey(t, 9)
		request.Expected.Installation = otherDevice

		requireBindingRefusal(t, client, request, controlplane.ResponseHeaderFieldInstallation)
	})

	t.Run("a response for another offering is refused", func(t *testing.T) {
		t.Parallel()

		request := issued.verification()
		request.Expected.Offering = controlplaneOffering(t, 1)

		requireBindingRefusal(t, client, request, controlplane.ResponseHeaderFieldOffering)
	})

	t.Run("a credential for another device key is refused", func(t *testing.T) {
		t.Parallel()

		// The document is authentic and binds to the request. What differs is
		// the key this machine actually holds, so the certificate it carries
		// belongs to somebody else.
		request := issued.verification()
		otherPublic, _ := testDeviceKey(t, 9)
		request.DeviceKey = otherPublic

		_, err := client.VerifyRegistration(request)
		if !errors.Is(err, core.ErrControlPlaneInstallationBinding) {
			t.Fatalf("VerifyRegistration() error = %v, want %v",
				err, core.ErrControlPlaneInstallationBinding)
		}
	})

	t.Run("a credential for another build is refused", func(t *testing.T) {
		t.Parallel()

		request := issued.verification()
		request.Build = testBuildIdentity(t, 2026, 0, 2)

		_, err := client.VerifyRegistration(request)
		if !errors.Is(err, core.ErrControlPlaneInstallationBinding) {
			t.Fatalf("VerifyRegistration() error = %v, want %v",
				err, core.ErrControlPlaneInstallationBinding)
		}
	})

	t.Run("a response signed by an untrusted key is refused", func(t *testing.T) {
		t.Parallel()

		request := issued.verification()
		otherPublic, _ := testSigningKey(t, 77)
		trusted, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
			Keys: []core.Ed25519PublicKey{otherPublic},
		})
		if err != nil {
			t.Fatalf("NewTrustedKeys() error = %v, want nil", err)
		}
		untrustedClient := testControlplaneClient(t, trusted)

		verified, err := untrustedClient.VerifyRegistration(request)
		if !errors.Is(err, core.ErrControlPlaneRegistration) ||
			!errors.Is(err, core.ErrAttestVerification) {
			t.Fatalf("VerifyRegistration() error = %v, want %v and %v",
				err, core.ErrControlPlaneRegistration, core.ErrAttestVerification)
		}
		if verified != (controlplane.VerifiedRegistration{}) {
			t.Fatalf("VerifyRegistration() = %v, want zero proof on refusal", verified)
		}
	})

	t.Run("an authority clock that moved backward is refused", func(t *testing.T) {
		t.Parallel()

		request := issued.verification()
		request.Expected.PriorProviderTime = testInstant(t, 500)

		_, err := client.VerifyRegistration(request)
		if !errors.Is(err, core.ErrControlPlaneProviderTimeRollback) {
			t.Fatalf("VerifyRegistration() error = %v, want %v",
				err, core.ErrControlPlaneProviderTimeRollback)
		}
	})

	t.Run("an authority clock that advanced is accepted", func(t *testing.T) {
		t.Parallel()

		// The mirror of the rollback case. A clock that moved forward is normal
		// operation, so the rollback rule must not reject every prior instant.
		request := issued.verification()
		request.Expected.PriorProviderTime = testInstant(t, 50)

		if _, err := client.VerifyRegistration(request); err != nil {
			t.Fatalf("VerifyRegistration() error = %v, want nil", err)
		}
	})

	t.Run("a tampered payload byte is refused", func(t *testing.T) {
		t.Parallel()

		// Changing a signed fact must break the signature rather than be
		// caught only by a consistency rule that a forger could satisfy.
		request := issued.verification()
		tampered := request.Document
		tampered.Payload.Header.Status = controlplane.ProductStatusPaymentRetry
		request.Document = tampered

		verified, err := client.VerifyRegistration(request)
		if !errors.Is(err, core.ErrControlPlaneRegistration) ||
			!errors.Is(err, core.ErrAttestVerification) {
			t.Fatalf("VerifyRegistration() error = %v, want %v and %v",
				err, core.ErrControlPlaneRegistration, core.ErrAttestVerification)
		}
		if verified != (controlplane.VerifiedRegistration{}) {
			t.Fatalf("VerifyRegistration() = %v, want zero proof on refusal", verified)
		}
	})

	t.Run("a stripped certificate is refused", func(t *testing.T) {
		t.Parallel()

		// Removing the credential from a grant leaves a document that is
		// internally contradictory before any signature is checked.
		request := issued.verification()
		stripped := request.Document
		stripped.Payload.Certificate = nil
		request.Document = stripped

		verified, err := client.VerifyRegistration(request)
		if !errors.Is(err, core.ErrControlPlaneRegistration) ||
			!errors.Is(err, core.ErrControlPlaneDecisionConsistency) {
			t.Fatalf("VerifyRegistration() error = %v, want %v and %v",
				err, core.ErrControlPlaneRegistration, core.ErrControlPlaneDecisionConsistency)
		}
		if verified != (controlplane.VerifiedRegistration{}) {
			t.Fatalf("VerifyRegistration() = %v, want zero proof on refusal", verified)
		}
	})
}

// TestVerifiedRegistrationCannotBeForged proves the zero value is inert, so a
// caller cannot manufacture the proof type and skip verification.
func TestVerifiedRegistrationCannotBeForged(t *testing.T) {
	t.Parallel()

	var forged controlplane.VerifiedRegistration
	if err := forged.Validate(); !errors.Is(err, core.ErrControlPlaneRegistration) {
		t.Fatalf("zero VerifiedRegistration Validate() error = %v, want %v", err, core.ErrControlPlaneRegistration)
	}
	if payload, err := forged.Payload(); !errors.Is(err, core.ErrControlPlaneRegistration) || payload != (controlplane.RegistrationPayload{}) {
		t.Errorf("zero VerifiedRegistration Payload() = (%v, %v), want zero and %v", payload, err, core.ErrControlPlaneRegistration)
	}
	if granted, err := forged.Lease(); !errors.Is(err, core.ErrControlPlaneRegistration) ||
		!errors.Is(granted.Validate(), core.ErrLeaseContract) {
		t.Errorf("zero VerifiedRegistration Lease() = (%v, %v), want errors.Is %v and unusable with %v",
			granted, err, core.ErrControlPlaneRegistration, core.ErrLeaseContract)
	}
	if certificate, err := forged.Certificate(); !errors.Is(err, core.ErrControlPlaneRegistration) || certificate != (controlplane.InstallationCertificateDocument{}) {
		t.Errorf("zero VerifiedRegistration Certificate() = (%v, %v), want zero and %v", certificate, err, core.ErrControlPlaneRegistration)
	}
}

func requireBindingRefusal(
	t *testing.T,
	client controlplane.Client,
	request controlplane.RegistrationVerification,
	want controlplane.ResponseHeaderField,
) {
	t.Helper()

	_, err := client.VerifyRegistration(request)
	if !errors.Is(err, core.ErrControlPlaneResponseBinding) {
		t.Fatalf("VerifyRegistration() error = %v, want %v", err, core.ErrControlPlaneResponseBinding)
	}
	var binding controlplane.ResponseBindingError
	if !errors.As(err, &binding) {
		t.Fatalf("VerifyRegistration() error = %v, want a ResponseBindingError", err)
	}
	if got := binding.Field(); got != want {
		t.Fatalf("binding failure named %v, want %v", got, want)
	}
}
