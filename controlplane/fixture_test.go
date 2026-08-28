package controlplane_test

import (
	"crypto/ed25519"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
	"github.com/deliri/primitive/v2026/temporal"
)

// otherRequestNonceHex is a valid nonce that is not the golden's, used to stand
// in for a response to some other request.
const otherRequestNonceHex = "5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c"

// issueTestRegistration builds one genuinely signed registration.
//
// The payload comes from the authority's own fixture rather than from
// hand-assembled parts, so the facts under test are the facts a real authority
// emits, with every field populated the way production populates it. Only the
// signatures are replaced: each nested document is re-signed with a real
// Ed25519 key this test holds, because verification is the thing being proved
// and the authority's private key is not in the repository.
func issueTestRegistration(t testing.TB) issuedRegistration {
	t.Helper()

	signerPublic, signer := testSigningKey(t, 1)

	var golden controlplane.RegistrationDocument
	if err := golden.UnmarshalJSON(readGolden(t, "registration_response.json")); err != nil {
		t.Fatalf("decoding the golden response error = %v, want nil", err)
	}
	payload := golden.Payload
	payload.Lease = resignLease(t, payload.Lease, signer)
	trusted, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
		Keys: []core.Ed25519PublicKey{signerPublic},
	})
	if err != nil {
		t.Fatalf("NewTrustedKeys() error = %v, want nil", err)
	}
	server := testControlplaneServer(t, trusted)
	payload.Certificate = resignCertificate(t, server, payload.Certificate, signer)

	document, err := server.IssueRegistration(payload, signer)
	if err != nil {
		t.Fatalf("Server.IssueRegistration() error = %v, want nil", err)
	}
	return issuedRegistration{
		document:    document,
		expectation: expectationFor(payload.Header),
		trusted:     trusted,
		build:       payload.Certificate.Body.Build,
		deviceKey:   payload.Certificate.Body.DeviceKey,
	}
}

// expectationFor is what a client that made this exact request would hold. The
// prior instant is unset, which is the first-contact case.
func expectationFor(header controlplane.ResponseHeader) controlplane.ResponseExpectation {
	return controlplane.ResponseExpectation{
		RequestNonce: header.RequestNonce,
		Account:      header.Account,
		Installation: header.Installation,
		Revision:     header.Revision,
		Family:       header.Family,
		Offering:     header.Offering,
	}
}

func acceptedProtocolAssessment(t testing.TB, header controlplane.ResponseHeader) controlwire.ProtocolAssessment {
	t.Helper()

	support, err := controlwire.PublishedProtocolSupport()
	if err != nil {
		t.Fatalf("PublishedProtocolSupport() error = %v, want nil", err)
	}
	assessment, err := controlwire.AssessProtocol(controlwire.ProtocolAssessmentRequest{
		Support: support,
		Capability: controlwire.ProtocolCapability{
			Revision: header.Revision,
			Family:   header.Family,
		},
	})
	if err != nil || assessment.Outcome != controlwire.ProtocolSupportOutcomeAccepted {
		t.Fatalf("AssessProtocol(published pair) = (%+v, %v), want accepted and nil", assessment, err)
	}
	return assessment
}

func upgradeRequiredProtocolAssessment(t testing.TB, header controlplane.ResponseHeader) controlwire.ProtocolAssessment {
	t.Helper()

	otherFamily := controlwire.RouteFamilyCheckIns
	if header.Family == otherFamily {
		otherFamily = controlwire.RouteFamilyRegistrations
	}
	support, err := controlwire.NewProtocolSupport(controlwire.ProtocolSupportRequest{
		Capabilities: []controlwire.ProtocolCapability{{Revision: header.Revision, Family: otherFamily}},
	})
	if err != nil {
		t.Fatalf("NewProtocolSupport(foreign route only) error = %v, want nil", err)
	}
	assessment, err := controlwire.AssessProtocol(controlwire.ProtocolAssessmentRequest{
		Support: support,
		Capability: controlwire.ProtocolCapability{
			Revision: header.Revision,
			Family:   header.Family,
		},
	})
	if err != nil || assessment.Outcome != controlwire.ProtocolSupportOutcomeUpgradeRequired {
		t.Fatalf("AssessProtocol(retired pair) = (%+v, %v), want upgrade-required and nil", assessment, err)
	}
	return assessment
}

func resignLease(t testing.TB, document lease.Document, signer ed25519.PrivateKey) lease.Document {
	t.Helper()

	envelope, err := attest.Sign(attest.SignRequest[lease.Domain]{
		Body: document.Decision, Signer: signer,
	})
	if err != nil {
		t.Fatalf("signing the lease decision error = %v, want nil", err)
	}
	resigned := lease.Document{Decision: document.Decision, Attestation: envelope}
	if err := resigned.Validate(); err != nil {
		t.Fatalf("re-signed lease Validate() error = %v, want nil", err)
	}
	return resigned
}

func resignCertificate(
	t testing.TB,
	server controlplane.Server,
	document *controlplane.InstallationCertificateDocument,
	signer ed25519.PrivateKey,
) *controlplane.InstallationCertificateDocument {
	t.Helper()

	if document == nil {
		t.Fatalf("golden response certificate = %v, want a certificate to re-sign", document)
	}
	resigned, err := server.IssueInstallationCertificate(document.Body, signer)
	if err != nil {
		t.Fatalf("Server.IssueInstallationCertificate() error = %v, want nil", err)
	}
	return &resigned
}

func testControlplaneClient(t testing.TB, trusted attest.TrustedKeys) controlplane.Client {
	t.Helper()

	client, err := controlplane.NewClient(controlplane.ClientConfiguration{
		TrustedAuthorityKeys: trusted,
	})
	if err != nil {
		t.Fatalf("controlplane.NewClient() error = %v, want nil", err)
	}
	return client
}

func testControlplaneServer(t testing.TB, trusted attest.TrustedKeys) controlplane.Server {
	t.Helper()

	server, err := controlplane.NewServer(controlplane.ServerConfiguration{
		TrustedAuthorityKeys: trusted,
	})
	if err != nil {
		t.Fatalf("controlplane.NewServer() error = %v, want nil", err)
	}
	return server
}

// testSigningKey returns a deterministic real Ed25519 key pair. Deterministic
// so a failure reproduces exactly; real because Attest verifies real signatures
// and a stand-in would prove nothing about that path.
func testSigningKey(t testing.TB, seed byte) (core.Ed25519PublicKey, ed25519.PrivateKey) {
	t.Helper()

	material := make([]byte, ed25519.SeedSize)
	for index := range material {
		material[index] = seed
	}
	private := ed25519.NewKeyFromSeed(material)
	public, err := core.NewEd25519PublicKey(private.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatalf("NewEd25519PublicKey() error = %v, want nil", err)
	}
	return public, private
}

// testDeviceKey returns a key pair and the installation identity it derives, so
// a test can name a device that is genuinely a different device.
func testDeviceKey(t testing.TB, seed byte) (core.Ed25519PublicKey, lease.DeviceID) {
	t.Helper()

	public, _ := testSigningKey(t, seed)
	device, err := lease.DeviceIDForPublicKey(public)
	if err != nil {
		t.Fatalf("DeviceIDForPublicKey() error = %v, want nil", err)
	}
	return public, device
}

// testBuildIdentity returns a build that differs from the golden's only in
// version, so a certificate-to-build mismatch is the single changed fact.
func testBuildIdentity(t testing.TB, major, minor, patch uint32) core.BuildIdentity {
	t.Helper()

	var golden controlplane.RegistrationDocument
	if err := golden.UnmarshalJSON(readGolden(t, "registration_response.json")); err != nil {
		t.Fatalf("decoding the golden response error = %v, want nil", err)
	}
	original := golden.Payload.Certificate.Body.Build
	identity, err := core.NewBuildIdentity(core.BuildIdentityRequest{
		Offering: original.Offering(), Version: core.NewReleaseVersion(major, minor, patch),
		Commit: original.Commit(), Platform: original.Platform(),
	})
	if err != nil {
		t.Fatalf("NewBuildIdentity() error = %v, want nil", err)
	}
	return identity
}

func testInstant(t testing.TB, nanoseconds int64) temporal.Instant {
	t.Helper()

	instant := temporal.InstantFromNanoseconds(nanoseconds)
	if err := instant.Validate(); err != nil {
		t.Fatalf("InstantFromNanoseconds(%d).Validate() error = %v, want nil", nanoseconds, err)
	}
	return instant
}

func otherRequestNonce(t testing.TB) controlwire.RequestNonce {
	t.Helper()

	nonce, err := controlwire.ParseRequestNonce(otherRequestNonceHex)
	if err != nil {
		t.Fatalf("ParseRequestNonce() error = %v, want nil", err)
	}
	return nonce
}
