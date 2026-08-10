package distributionauth

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"hash/crc32"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlplanetest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/distribution"
	"github.com/deliri/primitive/v2026/release"
	"github.com/deliri/primitive/v2026/temporal"
)

type distributionAuthFixtureRequest struct {
	offering      core.Offering
	authorityByte byte
	deviceByte    byte
	nonceByte     byte
}

type distributionAuthFixture struct {
	update  UpdateRequestDocument
	upgrade UpgradeRequestDocument
	trusted attest.TrustedKeys
}

func TestCredentialedDistributionRequestVerificationLayerTriadAuthenticatesEveryOffering(t *testing.T) {
	t.Parallel()

	admitted := 0
	for value := 0; value <= 255; value++ {
		offering := core.Offering(value)
		if !offering.IsValid() {
			continue
		}
		admitted++
		t.Run(offering.String(), func(t *testing.T) {
			t.Parallel()

			fixture := newDistributionAuthFixture(t, distributionAuthFixtureRequest{
				offering: offering, authorityByte: byte(value) + 0x20,
				deviceByte: byte(value) + 0x40, nonceByte: byte(value) + 1,
			})
			update, err := VerifyUpdate(UpdateVerification{
				Document: fixture.update, TrustedKeys: fixture.trusted,
			})
			if err != nil {
				t.Fatalf("VerifyUpdate(%v) error = %v, want nil", offering, err)
			}
			updatePayload, err := update.Payload()
			if err != nil || updatePayload != fixture.update.Request.Payload {
				t.Fatalf("VerifiedUpdate.Payload(%v) = (%+v, %v), want exact payload and nil",
					offering, updatePayload, err)
			}
			upgrade, err := VerifyUpgrade(UpgradeVerification{
				Document: fixture.upgrade, TrustedKeys: fixture.trusted,
			})
			if err != nil {
				t.Fatalf("VerifyUpgrade(%v) error = %v, want nil", offering, err)
			}
			upgradePayload, err := upgrade.Payload()
			if err != nil || upgradePayload != fixture.upgrade.Request.Payload {
				t.Fatalf("VerifiedUpgrade.Payload(%v) = (%+v, %v), want exact payload and nil",
					offering, upgradePayload, err)
			}
		})
	}
	if admitted < 3 {
		t.Fatalf("admitted offerings = %d, want at least the shipped set", admitted)
	}
	boundaries := []struct {
		name    string
		request distributionAuthFixtureRequest
	}{
		{name: "minimum authority maximum device and minimum nonce", request: distributionAuthFixtureRequest{
			authorityByte: 1, deviceByte: 255, nonceByte: 1,
		}},
		{name: "maximum authority minimum device and maximum nonce", request: distributionAuthFixtureRequest{
			authorityByte: 255, deviceByte: 1, nonceByte: 255,
		}},
		{name: "authority one below midpoint device at midpoint", request: distributionAuthFixtureRequest{
			authorityByte: 127, deviceByte: 128, nonceByte: 127,
		}},
		{name: "authority at midpoint device one below midpoint", request: distributionAuthFixtureRequest{
			authorityByte: 128, deviceByte: 127, nonceByte: 128,
		}},
		{name: "low distinct key material", request: distributionAuthFixtureRequest{
			authorityByte: 2, deviceByte: 3, nonceByte: 2,
		}},
		{name: "high distinct key material", request: distributionAuthFixtureRequest{
			authorityByte: 254, deviceByte: 253, nonceByte: 254,
		}},
		{name: "alternating key material", request: distributionAuthFixtureRequest{
			authorityByte: 0x55, deviceByte: 0xaa, nonceByte: 0x55,
		}},
	}
	for _, tc := range boundaries {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fixture := newDistributionAuthFixture(t, tc.request)
			update, updateErr := VerifyUpdate(UpdateVerification{
				Document: fixture.update, TrustedKeys: fixture.trusted,
			})
			if updateErr != nil || update.document != fixture.update {
				t.Fatalf("VerifyUpdate(%s) = (%v, %v), want exact proof and nil", tc.name, update, updateErr)
			}
			upgrade, upgradeErr := VerifyUpgrade(UpgradeVerification{
				Document: fixture.upgrade, TrustedKeys: fixture.trusted,
			})
			if upgradeErr != nil || upgrade.document != fixture.upgrade {
				t.Fatalf("VerifyUpgrade(%s) = (%v, %v), want exact proof and nil", tc.name, upgrade, upgradeErr)
			}
		})
	}
}

func TestCredentialedDistributionRequestVerificationLayerTriadRefusesDeviceAuthorityAndInstalledBuildSubstitution(t *testing.T) {
	t.Parallel()

	base := newDistributionAuthFixture(t, distributionAuthFixtureRequest{})
	otherDevice := newDistributionAuthFixture(t, distributionAuthFixtureRequest{
		authorityByte: 0x51, deviceByte: 0x52, nonceByte: 0x53,
	})
	otherOffering := newDistributionAuthFixture(t, distributionAuthFixtureRequest{
		offering: core.OfferingBug, authorityByte: 0x61, deviceByte: 0x62, nonceByte: 0x63,
	})
	wrongUpdateDevice, err := AssembleUpdate(UpdateRequestAssembly{
		Request: otherDevice.update.Request, Certificate: base.update.Certificate,
	})
	if err != nil {
		t.Fatalf("AssembleUpdate(same-build other device) error = %v, want nil", err)
	}
	wrongUpgradeDevice, err := AssembleUpgrade(UpgradeRequestAssembly{
		Request: otherDevice.upgrade.Request, Certificate: base.upgrade.Certificate,
	})
	if err != nil {
		t.Fatalf("AssembleUpgrade(same-build other device) error = %v, want nil", err)
	}
	if document, err := AssembleUpdate(UpdateRequestAssembly{
		Request: otherOffering.update.Request, Certificate: base.update.Certificate,
	}); !errors.Is(err, core.ErrControlPlaneResponseBinding) || document != (UpdateRequestDocument{}) {
		t.Fatalf("AssembleUpdate(other build) = (%+v, %v), want zero and errors.Is %v",
			document, err, core.ErrControlPlaneResponseBinding)
	}
	if document, err := AssembleUpgrade(UpgradeRequestAssembly{
		Request: otherOffering.upgrade.Request, Certificate: base.upgrade.Certificate,
	}); !errors.Is(err, core.ErrControlPlaneResponseBinding) || document != (UpgradeRequestDocument{}) {
		t.Fatalf("AssembleUpgrade(other installed build) = (%+v, %v), want zero and errors.Is %v",
			document, err, core.ErrControlPlaneResponseBinding)
	}
	tamperedUpdateNonce := base.update
	tamperedUpdateNonce.Request.Payload.Nonce = distributionAuthNonce(t, 0x72)
	tamperedUpgradeNonce := base.upgrade
	tamperedUpgradeNonce.Request.Payload.Nonce = distributionAuthNonce(t, 0x72)
	tamperedUpdateSigner := base.update
	tamperedUpdateSigner.Request.Attestation.Signer = otherDevice.update.Certificate.Body.DeviceKey
	tamperedUpgradeSigner := base.upgrade
	tamperedUpgradeSigner.Request.Attestation.Signer = otherDevice.upgrade.Certificate.Body.DeviceKey
	tamperedUpdateLength := base.update
	updateLength, err := tamperedUpdateLength.Request.Attestation.BodyLength.Uint64()
	if err != nil {
		t.Fatalf("update request BodyLength.Uint64() error = %v, want nil", err)
	}
	tamperedUpdateLength.Request.Attestation.BodyLength, err = core.NewByteCount(updateLength + 1)
	if err != nil {
		t.Fatalf("core.NewByteCount(tampered update length) error = %v, want nil", err)
	}
	tamperedUpgradeLength := base.upgrade
	upgradeLength, err := tamperedUpgradeLength.Request.Attestation.BodyLength.Uint64()
	if err != nil {
		t.Fatalf("upgrade request BodyLength.Uint64() error = %v, want nil", err)
	}
	tamperedUpgradeLength.Request.Attestation.BodyLength, err = core.NewByteCount(upgradeLength + 1)
	if err != nil {
		t.Fatalf("core.NewByteCount(tampered upgrade length) error = %v, want nil", err)
	}
	tamperedUpdateDigest := base.update
	tamperedUpdateDigest.Request.Attestation.BodySHA256 = core.SHA256Of([]byte("other update request"))
	tamperedUpgradeDigest := base.upgrade
	tamperedUpgradeDigest.Request.Attestation.BodySHA256 = core.SHA256Of([]byte("other upgrade request"))
	tamperedUpdateCertificateSigner := base.update
	tamperedUpdateCertificateSigner.Certificate.Attestation.Signer = otherDevice.update.Certificate.Attestation.Signer
	tamperedUpgradeCertificateSigner := base.upgrade
	tamperedUpgradeCertificateSigner.Certificate.Attestation.Signer = otherDevice.upgrade.Certificate.Attestation.Signer
	tamperedUpdateIssuedAt := base.update
	tamperedUpdateIssuedAt.Certificate.Body.IssuedAt = temporal.InstantFromNanoseconds(1_800_000_000_000_000_000)
	tamperedUpgradeIssuedAt := base.upgrade
	tamperedUpgradeIssuedAt.Certificate.Body.IssuedAt = temporal.InstantFromNanoseconds(1_800_000_000_000_000_000)
	updateCases := []struct {
		wantErr  error
		name     string
		document UpdateRequestDocument
		trusted  attest.TrustedKeys
	}{
		{name: "zero update document", trusted: base.trusted, wantErr: core.ErrControlPlaneContract},
		{name: "zero update authority trust", document: base.update, wantErr: core.ErrControlPlaneContract},
		{name: "same-build other update device", document: wrongUpdateDevice, trusted: base.trusted, wantErr: core.ErrAttestVerification},
		{name: "other update authority", document: base.update, trusted: otherDevice.trusted, wantErr: core.ErrAttestVerification},
		{name: "update nonce changed after issue", document: tamperedUpdateNonce, trusted: base.trusted, wantErr: core.ErrAttestVerification},
		{name: "update signer changed after issue", document: tamperedUpdateSigner, trusted: base.trusted, wantErr: core.ErrAttestVerification},
		{name: "update body length changed after issue", document: tamperedUpdateLength, trusted: base.trusted, wantErr: core.ErrAttestVerification},
		{name: "update body digest changed after issue", document: tamperedUpdateDigest, trusted: base.trusted, wantErr: core.ErrAttestVerification},
		{name: "update certificate signer changed after issue", document: tamperedUpdateCertificateSigner, trusted: base.trusted, wantErr: core.ErrAttestVerification},
		{name: "update certificate issued-at changed after issue", document: tamperedUpdateIssuedAt, trusted: base.trusted, wantErr: core.ErrAttestVerification},
	}
	for _, tc := range updateCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := VerifyUpdate(UpdateVerification{Document: tc.document, TrustedKeys: tc.trusted})
			if !errors.Is(gotErr, tc.wantErr) || got != (VerifiedUpdate{}) {
				t.Fatalf("VerifyUpdate(%s) = (%v, %v), want zero and errors.Is %v",
					tc.name, got, gotErr, tc.wantErr)
			}
		})
	}
	upgradeCases := []struct {
		wantErr  error
		name     string
		document UpgradeRequestDocument
		trusted  attest.TrustedKeys
	}{
		{name: "zero upgrade document", trusted: base.trusted, wantErr: core.ErrControlPlaneContract},
		{name: "zero upgrade authority trust", document: base.upgrade, wantErr: core.ErrControlPlaneContract},
		{name: "same-build other upgrade device", document: wrongUpgradeDevice, trusted: base.trusted, wantErr: core.ErrAttestVerification},
		{name: "other upgrade authority", document: base.upgrade, trusted: otherDevice.trusted, wantErr: core.ErrAttestVerification},
		{name: "upgrade nonce changed after issue", document: tamperedUpgradeNonce, trusted: base.trusted, wantErr: core.ErrAttestVerification},
		{name: "upgrade signer changed after issue", document: tamperedUpgradeSigner, trusted: base.trusted, wantErr: core.ErrAttestVerification},
		{name: "upgrade body length changed after issue", document: tamperedUpgradeLength, trusted: base.trusted, wantErr: core.ErrAttestVerification},
		{name: "upgrade body digest changed after issue", document: tamperedUpgradeDigest, trusted: base.trusted, wantErr: core.ErrAttestVerification},
		{name: "upgrade certificate signer changed after issue", document: tamperedUpgradeCertificateSigner, trusted: base.trusted, wantErr: core.ErrAttestVerification},
		{name: "upgrade certificate issued-at changed after issue", document: tamperedUpgradeIssuedAt, trusted: base.trusted, wantErr: core.ErrAttestVerification},
	}
	for _, tc := range upgradeCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := VerifyUpgrade(UpgradeVerification{Document: tc.document, TrustedKeys: tc.trusted})
			if !errors.Is(gotErr, tc.wantErr) || got != (VerifiedUpgrade{}) {
				t.Fatalf("VerifyUpgrade(%s) = (%v, %v), want zero and errors.Is %v",
					tc.name, got, gotErr, tc.wantErr)
			}
		})
	}
}

func TestCredentialedDistributionRequestJSONLayerTriadIsStrictBoundedAndPreserving(t *testing.T) {
	t.Parallel()

	fixture := newDistributionAuthFixture(t, distributionAuthFixtureRequest{})
	t.Run("update", func(t *testing.T) {
		t.Parallel()
		testUpdateJSONBoundary(t, fixture.update)
	})
	t.Run("upgrade", func(t *testing.T) {
		t.Parallel()
		testUpgradeJSONBoundary(t, fixture.upgrade)
	})
}

func TestCredentialedDistributionRequestVerificationLayerTriadZeroValuesNeverAcquireProof(t *testing.T) {
	t.Parallel()

	update, updateErr := VerifyUpdate(UpdateVerification{})
	if !errors.Is(updateErr, core.ErrControlPlaneContract) || update != (VerifiedUpdate{}) {
		t.Fatalf("VerifyUpdate(zero) = (%v, %v), want zero and errors.Is %v",
			update, updateErr, core.ErrControlPlaneContract)
	}
	updatePayload, updateErr := (VerifiedUpdate{}).Payload()
	if !errors.Is(updateErr, core.ErrControlPlaneContract) || updatePayload != (distribution.UpdateRequestPayload{}) {
		t.Fatalf("VerifiedUpdate{}.Payload() = (%v, %v), want zero and errors.Is %v",
			updatePayload, updateErr, core.ErrControlPlaneContract)
	}
	upgrade, upgradeErr := VerifyUpgrade(UpgradeVerification{})
	if !errors.Is(upgradeErr, core.ErrControlPlaneContract) || upgrade != (VerifiedUpgrade{}) {
		t.Fatalf("VerifyUpgrade(zero) = (%v, %v), want zero and errors.Is %v",
			upgrade, upgradeErr, core.ErrControlPlaneContract)
	}
	upgradePayload, upgradeErr := (VerifiedUpgrade{}).Payload()
	if !errors.Is(upgradeErr, core.ErrControlPlaneContract) || upgradePayload != (distribution.UpgradeRequestPayload{}) {
		t.Fatalf("VerifiedUpgrade{}.Payload() = (%v, %v), want zero and errors.Is %v",
			upgradePayload, upgradeErr, core.ErrControlPlaneContract)
	}
}

func testUpdateJSONBoundary(t *testing.T, document UpdateRequestDocument) {
	t.Helper()

	encoded, err := document.MarshalJSON()
	if err != nil {
		t.Fatalf("UpdateRequestDocument.MarshalJSON() error = %v, want nil", err)
	}
	reordered, err := json.Marshal(struct {
		Request     distribution.UpdateRequestDocument           `json:"request"`
		Certificate controlplane.InstallationCertificateDocument `json:"certificate"`
	}{Request: document.Request, Certificate: document.Certificate})
	if err != nil {
		t.Fatalf("json.Marshal(reordered update) error = %v, want nil", err)
	}
	valid := distributionAuthValidJSONCases(t, encoded, reordered)
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var receiver UpdateRequestDocument
			if err := receiver.UnmarshalJSON(tc.data); err != nil || receiver != document {
				t.Fatalf("UpdateRequestDocument.UnmarshalJSON(%s) = (%+v, %v), want exact and nil",
					tc.name, receiver, err)
			}
		})
	}
	for _, tc := range distributionAuthInvalidJSONCases(encoded) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			receiver := document
			if err := receiver.UnmarshalJSON(tc.data); !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("UpdateRequestDocument.UnmarshalJSON(%s) error = %v, want errors.Is %v",
					tc.name, err, core.ErrJSONContract)
			}
			if receiver != document {
				t.Fatalf("UpdateRequestDocument.UnmarshalJSON(%s) mutated receiver", tc.name)
			}
		})
	}
	var receiver *UpdateRequestDocument
	if err := receiver.UnmarshalJSON(encoded); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil UpdateRequestDocument.UnmarshalJSON() error = %v, want errors.Is %v", err, core.ErrJSONContract)
	}
}

func testUpgradeJSONBoundary(t *testing.T, document UpgradeRequestDocument) {
	t.Helper()

	encoded, err := document.MarshalJSON()
	if err != nil {
		t.Fatalf("UpgradeRequestDocument.MarshalJSON() error = %v, want nil", err)
	}
	reordered, err := json.Marshal(struct {
		Request     distribution.UpgradeRequestDocument          `json:"request"`
		Certificate controlplane.InstallationCertificateDocument `json:"certificate"`
	}{Request: document.Request, Certificate: document.Certificate})
	if err != nil {
		t.Fatalf("json.Marshal(reordered upgrade) error = %v, want nil", err)
	}
	for _, tc := range distributionAuthValidJSONCases(t, encoded, reordered) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var receiver UpgradeRequestDocument
			if err := receiver.UnmarshalJSON(tc.data); err != nil || receiver != document {
				t.Fatalf("UpgradeRequestDocument.UnmarshalJSON(%s) = (%+v, %v), want exact and nil",
					tc.name, receiver, err)
			}
		})
	}
	for _, tc := range distributionAuthInvalidJSONCases(encoded) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			receiver := document
			if err := receiver.UnmarshalJSON(tc.data); !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("UpgradeRequestDocument.UnmarshalJSON(%s) error = %v, want errors.Is %v",
					tc.name, err, core.ErrJSONContract)
			}
			if receiver != document {
				t.Fatalf("UpgradeRequestDocument.UnmarshalJSON(%s) mutated receiver", tc.name)
			}
		})
	}
	var receiver *UpgradeRequestDocument
	if err := receiver.UnmarshalJSON(encoded); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil UpgradeRequestDocument.UnmarshalJSON() error = %v, want errors.Is %v", err, core.ErrJSONContract)
	}
}

type distributionAuthJSONCase struct {
	name string
	data []byte
}

func distributionAuthValidJSONCases(
	t *testing.T,
	encoded []byte,
	reordered []byte,
) []distributionAuthJSONCase {
	t.Helper()

	var indented bytes.Buffer
	if err := json.Indent(&indented, encoded, "", "  "); err != nil {
		t.Fatalf("json.Indent(credentialed distribution request) error = %v, want nil", err)
	}
	return []distributionAuthJSONCase{
		{name: "canonical", data: encoded},
		{name: "reordered", data: reordered},
		{name: "indented", data: indented.Bytes()},
		{name: "leading space", data: append([]byte(" "), encoded...)},
		{name: "trailing newline", data: append(bytes.Clone(encoded), '\n')},
		{name: "carriage return framing", data: append(append([]byte("\r"), encoded...), '\r')},
		{name: "mixed whitespace", data: append(append([]byte("\t\r\n"), encoded...), ' ', '\t')},
		{name: "half ceiling", data: distributionAuthPadJSON(encoded, RequestDocumentJSONMaximumBytes/2)},
		{name: "one below ceiling", data: distributionAuthPadJSON(encoded, RequestDocumentJSONMaximumBytes-1)},
		{name: "exact ceiling", data: distributionAuthPadJSON(encoded, RequestDocumentJSONMaximumBytes)},
	}
}

func distributionAuthInvalidJSONCases(encoded []byte) []distributionAuthJSONCase {
	unknown := append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"future":true}`)...)
	duplicateRequest := append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"request":null}`)...)
	duplicateCertificate := append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"certificate":null}`)...)
	return []distributionAuthJSONCase{
		{name: "empty"},
		{name: "whitespace only", data: []byte(" \t\r\n")},
		{name: "null", data: []byte(`null`)},
		{name: "boolean", data: []byte(`true`)},
		{name: "scalar", data: []byte(`1`)},
		{name: "string", data: []byte(`"distribution"`)},
		{name: "array", data: []byte(`[]`)},
		{name: "empty object", data: []byte(`{}`)},
		{name: "unknown member", data: unknown},
		{name: "duplicate request", data: duplicateRequest},
		{name: "duplicate certificate", data: duplicateCertificate},
		{name: "missing request", data: []byte(`{"certificate":null}`)},
		{name: "missing certificate", data: []byte(`{"request":null}`)},
		{name: "request wrong type", data: []byte(`{"request":true,"certificate":null}`)},
		{name: "certificate wrong type", data: []byte(`{"request":null,"certificate":true}`)},
		{name: "open object", data: []byte(`{`)},
		{name: "open array", data: []byte(`[`)},
		{name: "truncated", data: encoded[:len(encoded)-1]},
		{name: "half truncated", data: encoded[:len(encoded)/2]},
		{name: "two documents", data: append(bytes.Clone(encoded), encoded...)},
		{name: "trailing scalar", data: append(bytes.Clone(encoded), []byte(` 0`)...)},
		{name: "one above ceiling", data: distributionAuthPadJSON(encoded, RequestDocumentJSONMaximumBytes+1)},
	}
}

func newDistributionAuthFixture(
	t testing.TB,
	request distributionAuthFixtureRequest,
) distributionAuthFixture {
	t.Helper()

	if request.offering == core.OfferingUnknown {
		request.offering = core.OfferingWitness
	}
	if request.authorityByte == 0 {
		request.authorityByte = 0x21
	}
	if request.deviceByte == 0 {
		request.deviceByte = 0x31
	}
	if request.nonceByte == 0 {
		request.nonceByte = 0x41
	}
	installation, err := controlplanetest.IssueInstallation(controlplanetest.InstallationRequest{
		AuthoritySeed: distributionAuthSeed(request.authorityByte),
		DeviceSeed:    distributionAuthSeed(request.deviceByte), Offering: request.offering,
	})
	if err != nil {
		t.Fatalf("controlplanetest.IssueInstallation() error = %v, want nil", err)
	}
	nonce := distributionAuthNonce(t, request.nonceByte)
	updateRequest, err := distribution.IssueUpdateRequest(distribution.UpdateRequestIssuance{
		Signer: installation.DevicePrivate,
		Payload: distribution.UpdateRequestPayload{
			Build: installation.Build, Nonce: nonce, Revision: controlwire.Revision2026V1,
		},
	})
	if err != nil {
		t.Fatalf("distribution.IssueUpdateRequest() error = %v, want nil", err)
	}
	update, err := AssembleUpdate(UpdateRequestAssembly{
		Request: updateRequest, Certificate: installation.Certificate,
	})
	if err != nil {
		t.Fatalf("distributionauth.AssembleUpdate() error = %v, want nil", err)
	}
	upgradeRequest, err := distribution.IssueUpgradeRequest(distribution.UpgradeRequestIssuance{
		Signer: installation.DevicePrivate,
		Payload: distribution.UpgradeRequestPayload{
			Available: distributionAvailableSummary(t, installation.Build),
			Nonce:     nonce, Revision: controlwire.Revision2026V1,
		},
	})
	if err != nil {
		t.Fatalf("distribution.IssueUpgradeRequest() error = %v, want nil", err)
	}
	upgrade, err := AssembleUpgrade(UpgradeRequestAssembly{
		Request: upgradeRequest, Certificate: installation.Certificate,
	})
	if err != nil {
		t.Fatalf("distributionauth.AssembleUpgrade() error = %v, want nil", err)
	}
	trusted, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
		Keys: []core.Ed25519PublicKey{installation.AuthorityPublic},
	})
	if err != nil {
		t.Fatalf("attest.NewTrustedKeys() error = %v, want nil", err)
	}
	return distributionAuthFixture{
		update: update, upgrade: upgrade, trusted: trusted,
	}
}

func distributionAvailableSummary(t testing.TB, installed core.BuildIdentity) release.AvailableSummary {
	t.Helper()

	candidate, err := core.NewBuildIdentity(core.BuildIdentityRequest{
		Offering: installed.Offering(), Version: core.NewReleaseVersion(2026, 0, 54),
		Commit: installed.Commit(), Platform: installed.Platform(),
	})
	if err != nil {
		t.Fatalf("core.NewBuildIdentity(candidate) error = %v, want nil", err)
	}
	content := []byte("candidate executable")
	extent, err := core.NewByteCount(uint64(len(content)))
	if err != nil {
		t.Fatalf("core.NewByteCount(candidate) error = %v, want nil", err)
	}
	artifact, err := release.NewArtifact(release.ArtifactRequest{
		Build: candidate, Extent: extent, SHA256: core.SHA256Of(content),
		CRC32C: core.NewCRC32C(crc32.Checksum(content, crc32.MakeTable(crc32.Castagnoli))),
	})
	if err != nil {
		t.Fatalf("release.NewArtifact(candidate) error = %v, want nil", err)
	}
	filename, err := artifact.Filename()
	if err != nil {
		t.Fatalf("release.Artifact.Filename() error = %v, want nil", err)
	}
	manifest := distributionManifestIdentity(t, core.SHA256Of([]byte("manifest fact")))
	document := distributionManifestDocumentDigest(t, core.SHA256Of([]byte("manifest document")))
	summary := release.AvailableSummary{
		Installed: installed, Candidate: candidate, Manifest: manifest,
		ManifestDocument: document, Artifact: artifact.Identity(), Filename: filename,
		Integrity: artifact.Integrity(), ValidUntil: temporal.InstantFromNanoseconds(1_900_000_000_000_000_000),
	}
	if err := summary.Validate(); err != nil {
		t.Fatalf("release.AvailableSummary.Validate() error = %v, want nil", err)
	}
	return summary
}

func distributionManifestIdentity(t testing.TB, digest core.SHA256Digest) release.ManifestIdentity {
	t.Helper()

	encoded, err := digest.MarshalJSON()
	if err != nil {
		t.Fatalf("SHA256Digest.MarshalJSON(manifest) error = %v, want nil", err)
	}
	var identity release.ManifestIdentity
	if err := json.Unmarshal(encoded, &identity); err != nil {
		t.Fatalf("json.Unmarshal(ManifestIdentity) error = %v, want nil", err)
	}
	return identity
}

func distributionManifestDocumentDigest(
	t testing.TB,
	digest core.SHA256Digest,
) release.ManifestDocumentDigest {
	t.Helper()

	encoded, err := digest.MarshalJSON()
	if err != nil {
		t.Fatalf("SHA256Digest.MarshalJSON(document) error = %v, want nil", err)
	}
	var identity release.ManifestDocumentDigest
	if err := json.Unmarshal(encoded, &identity); err != nil {
		t.Fatalf("json.Unmarshal(ManifestDocumentDigest) error = %v, want nil", err)
	}
	return identity
}

func distributionAuthSeed(marker byte) [ed25519.SeedSize]byte {
	seed := [ed25519.SeedSize]byte{}
	for index := range seed {
		seed[index] = marker
	}
	return seed
}

func distributionAuthNonce(t testing.TB, marker byte) controlwire.RequestNonce {
	t.Helper()

	raw := [controlwire.NonceBytes]byte{}
	for index := range raw {
		raw[index] = marker
	}
	nonce, err := controlwire.NewRequestNonce(raw)
	if err != nil {
		t.Fatalf("controlwire.NewRequestNonce() error = %v, want nil", err)
	}
	return nonce
}

func distributionAuthPadJSON(encoded []byte, length int) []byte {
	if length < len(encoded) {
		return nil
	}
	padded := make([]byte, length)
	for index := 0; index < length-len(encoded); index++ {
		padded[index] = ' '
	}
	copy(padded[length-len(encoded):], encoded)
	return padded
}
