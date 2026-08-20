package distributionauth

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"hash/crc32"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplanetest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/deploy"
	"github.com/deliri/primitive/v2026/distribution"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/release"
)

type publicationAuthFixtureRequest struct {
	offering      core.Offering
	authorityByte byte
	deviceByte    byte
	releaseByte   byte
	nonceByte     byte
}

type publicationAuthRelease struct {
	payloads [release.PublicationObjectCount][]byte
	document release.ManifestDocument
	verified release.VerifiedManifest
	keys     attest.TrustedKeys
}

type publicationAuthUpload struct {
	target     objectstore.UploadTarget
	projection objectstore.UploadCapabilityProjection
}

type publicationAuthFixture struct {
	installation    controlplanetest.Installation
	grant           distribution.PublicationGrantDocument
	grantProjection distribution.PublicationGrantProjection
	completion      PublicationCompletionDocument
	grantProof      distribution.VerifiedPublicationGrant
	document        PublicationRequestDocument
	release         publicationAuthRelease
	verified        VerifiedPublication
	authority       attest.TrustedKeys
}

type publicationAuthTransport struct {
	generation int
}

func (t *publicationAuthTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if _, err := io.Copy(io.Discard, request.Body); err != nil {
		return nil, err
	}
	if err := request.Body.Close(); err != nil {
		return nil, err
	}
	t.generation++
	header := make(http.Header)
	header.Set("x-goog-generation", strconv.Itoa(t.generation))
	return &http.Response{
		StatusCode: http.StatusOK, Header: header, Body: http.NoBody,
		ContentLength: 0, Request: request,
	}, nil
}

func newPublicationAuthFixture(
	t testing.TB,
	request publicationAuthFixtureRequest,
) publicationAuthFixture {
	t.Helper()
	request = publicationAuthFixtureDefaults(request)
	installation, err := controlplanetest.IssueInstallation(controlplanetest.InstallationRequest{
		AuthoritySeed: distributionAuthSeed(request.authorityByte),
		DeviceSeed:    distributionAuthSeed(request.deviceByte),
		Offering:      request.offering,
	})
	if err != nil {
		t.Fatalf("controlplanetest.IssueInstallation() error = %v, want nil", err)
	}
	releaseFixture := newPublicationAuthRelease(t, installation, request.releaseByte)
	requestPayload := distribution.PublicationRequestPayload{
		Manifest: releaseFixture.document, Build: installation.Build,
		Nonce: distributionAuthNonce(t, request.nonceByte), Revision: installation.Certificate.Body.Revision,
	}
	requestDocument, err := distribution.IssuePublicationRequest(distribution.PublicationRequestIssuance{
		Signer: installation.DevicePrivate, Payload: requestPayload,
	})
	if err != nil {
		t.Fatalf("distribution.IssuePublicationRequest() error = %v, want nil", err)
	}
	document, err := AssemblePublication(PublicationRequestAssembly{
		Request: requestDocument, Certificate: installation.Certificate,
	})
	if err != nil {
		t.Fatalf("AssemblePublication() error = %v, want nil", err)
	}
	authority := publicationAuthTrustedKeys(t, installation.AuthorityPrivate)
	verified, err := VerifyPublication(PublicationVerification{
		Document: document, TrustedKeys: authority, ManifestKeys: releaseFixture.keys,
	})
	if err != nil {
		t.Fatalf("VerifyPublication() error = %v, want nil", err)
	}
	grantProjection, grant, grantProof, uploadTarget := newPublicationAuthGrant(
		t, verified, installation.AuthorityPrivate, authority,
	)
	completion := newPublicationAuthCompletion(
		t, installation, releaseFixture, verified, grantProof, uploadTarget,
	)
	return publicationAuthFixture{
		installation: installation, release: releaseFixture, document: document,
		verified: verified, authority: authority, grant: grant, grantProjection: grantProjection,
		grantProof: grantProof, completion: completion,
	}
}

func publicationAuthFixtureDefaults(request publicationAuthFixtureRequest) publicationAuthFixtureRequest {
	if request.offering == core.OfferingUnknown {
		request.offering = core.OfferingWitness
	}
	if request.authorityByte == 0 {
		request.authorityByte = 0x21
	}
	if request.deviceByte == 0 {
		request.deviceByte = 0x31
	}
	if request.releaseByte == 0 {
		request.releaseByte = 0x51
	}
	if request.nonceByte == 0 {
		request.nonceByte = 0x41
	}
	return request
}

func newPublicationAuthRelease(
	t testing.TB,
	installation controlplanetest.Installation,
	signerByte byte,
) publicationAuthRelease {
	t.Helper()
	installed := installation.Build
	var result publicationAuthRelease
	var artifacts [release.TargetCount]release.Artifact
	targets := release.Targets()
	for index := range release.TargetCount {
		platform, ok := targets.At(index)
		if !ok {
			t.Fatalf("release.Targets().At(%d) ok = false, want true", index)
		}
		build, err := core.NewBuildIdentity(core.BuildIdentityRequest{
			Offering: installed.Offering(), Version: installed.Version(),
			Commit: installed.Commit(), Platform: platform,
		})
		if err != nil {
			t.Fatalf("core.NewBuildIdentity(%d) error = %v, want nil", index, err)
		}
		payload := []byte("publication-auth-artifact-" + strconv.Itoa(index))
		integrity := publicationAuthIntegrity(t, payload)
		extent, err := core.NewByteCount(integrity.Length.Uint64())
		if err != nil {
			t.Fatalf("core.NewByteCount(artifact %d) error = %v, want nil", index, err)
		}
		artifacts[index], err = release.NewArtifact(release.ArtifactRequest{
			Build: build, Extent: extent, SHA256: integrity.SHA256, CRC32C: integrity.CRC32C,
		})
		if err != nil {
			t.Fatalf("release.NewArtifact(%d) error = %v, want nil", index, err)
		}
		result.payloads[index] = payload
	}
	artifactSet, err := release.NewArtifactSet(release.ArtifactSetRequest{Artifacts: artifacts})
	if err != nil {
		t.Fatalf("release.NewArtifactSet() error = %v, want nil", err)
	}
	metadata, payloads := publicationAuthMetadata(t)
	for index, payload := range payloads {
		result.payloads[release.TargetCount+1+index] = payload
	}
	fact, err := release.NewManifestFact(release.ManifestFactRequest{
		Revision: release.Revision2026V1, Offering: installed.Offering(),
		Version: installed.Version(), Commit: installed.Commit(),
		CreatedAt: installation.Certificate.Body.IssuedAt, Artifacts: artifactSet,
		Provenance: publicationAuthProvenance(t), Metadata: metadata,
	})
	if err != nil {
		t.Fatalf("release.NewManifestFact() error = %v, want nil", err)
	}
	signer := publicationAuthPrivateKey(signerByte)
	result.document, err = release.IssueManifest(release.IssueManifestRequest{Signer: signer, Fact: fact})
	if err != nil {
		t.Fatalf("release.IssueManifest() error = %v, want nil", err)
	}
	result.payloads[release.TargetCount], err = result.document.MarshalJSON()
	if err != nil {
		t.Fatalf("release.ManifestDocument.MarshalJSON() error = %v, want nil", err)
	}
	result.keys = publicationAuthTrustedKeys(t, signer)
	result.verified, err = release.VerifyManifest(release.VerifyManifestRequest{
		Document: result.document, TrustedKeys: result.keys,
		ExpectedOffering: installed.Offering(),
	})
	if err != nil {
		t.Fatalf("release.VerifyManifest() error = %v, want nil", err)
	}
	return result
}

func publicationAuthMetadata(
	t testing.TB,
) (release.MetadataSet, [release.MetadataAssetCount][]byte) {
	t.Helper()
	var assets [release.MetadataAssetCount]release.MetadataAsset
	var payloads [release.MetadataAssetCount][]byte
	for index := range release.MetadataAssetCount {
		payload := []byte("publication-auth-metadata-" + strconv.Itoa(index))
		integrity := publicationAuthIntegrity(t, payload)
		extent, err := core.NewByteCount(integrity.Length.Uint64())
		if err != nil {
			t.Fatalf("core.NewByteCount(metadata %d) error = %v, want nil", index, err)
		}
		asset, err := release.NewMetadataAsset(release.MetadataAssetRequest{
			Kind: release.MetadataKind(index + 1), Extent: extent,
			SHA256: integrity.SHA256, CRC32C: integrity.CRC32C,
		})
		if err != nil {
			t.Fatalf("release.NewMetadataAsset(%d) error = %v, want nil", index, err)
		}
		assets[index], payloads[index] = asset, payload
	}
	set, err := release.NewMetadataSet(release.MetadataSetRequest{Assets: assets})
	if err != nil {
		t.Fatalf("release.NewMetadataSet() error = %v, want nil", err)
	}
	return set, payloads
}

func publicationAuthProvenance(t testing.TB) release.BuildProvenance {
	t.Helper()
	goToolchain, err := release.CurrentGoToolchain().Version()
	if err != nil {
		t.Fatalf("release.CurrentGoToolchain().Version() error = %v, want nil", err)
	}
	wire := struct {
		GarbleDerivation       string            `json:"garble_derivation"`
		GarbleModule           string            `json:"garble_module"`
		GarbleVersion          string            `json:"garble_version"`
		GarbleRevision         string            `json:"garble_revision"`
		GarbleModuleSum        string            `json:"garble_module_sum"`
		GarbleLiterals         string            `json:"garble_literals"`
		GarbleDiagnostics      string            `json:"garble_diagnostics"`
		GoToolchain            string            `json:"go_toolchain"`
		MainPackage            string            `json:"main_package"`
		ModuleMode             string            `json:"module_mode"`
		BuildTags              []string          `json:"build_tags"`
		LinkerAssignments      []struct{}        `json:"linker_assignments"`
		GoExecutableSHA256     core.SHA256Digest `json:"go_executable_sha256"`
		GarbleExecutableSHA256 core.SHA256Digest `json:"garble_executable_sha256"`
	}{
		GarbleDerivation: "one", GarbleModule: "mvdan.cc/garble",
		GarbleVersion:   "v0.17.0",
		GarbleRevision:  "39c484d3007e9a608ac8692dab0b9bb5f71dfc2a",
		GarbleModuleSum: "h1:XJ6jJhlT8HTEU9Dd02nLDUciuyPDXGRopwy/Cuoo/0M=",
		GarbleLiterals:  "obfuscate", GarbleDiagnostics: "preserve",
		GoToolchain: goToolchain, MainPackage: "github.com/offGridSoft/witness/cmd/witness",
		ModuleMode: "vendor", BuildTags: []string{}, LinkerAssignments: []struct{}{},
		GoExecutableSHA256:     core.SHA256Of([]byte("publication-auth-go")),
		GarbleExecutableSHA256: core.SHA256Of([]byte("publication-auth-garble")),
	}
	encoded, err := json.Marshal(wire)
	if err != nil {
		t.Fatalf("json.Marshal(BuildProvenance fixture) error = %v, want nil", err)
	}
	var provenance release.BuildProvenance
	if err := provenance.UnmarshalJSON(encoded); err != nil {
		t.Fatalf("release.BuildProvenance.UnmarshalJSON() error = %v, want nil", err)
	}
	return provenance
}

func newPublicationAuthGrant(
	t testing.TB,
	request VerifiedPublication,
	signer ed25519.PrivateKey,
	trusted attest.TrustedKeys,
) (
	distribution.PublicationGrantProjection,
	distribution.PublicationGrantDocument,
	distribution.VerifiedPublicationGrant,
	objectstore.UploadTarget,
) {
	t.Helper()
	payload, err := request.Payload()
	if err != nil {
		t.Fatalf("VerifiedPublication.Payload() error = %v, want nil", err)
	}
	commitment, err := distribution.CommitRequest(payload)
	if err != nil {
		t.Fatalf("distribution.CommitRequest() error = %v, want nil", err)
	}
	var projections [release.PublicationObjectCount]objectstore.UploadCapabilityProjection
	var commitments [release.PublicationObjectCount]objectstore.UploadCapabilityCommitment
	var firstTarget objectstore.UploadTarget
	for index := range projections {
		upload := publicationAuthUploadFixture(t, index)
		projections[index] = upload.projection
		if index == 0 {
			firstTarget = upload.target
		}
		commitments[index], err = projections[index].Commitment()
		if err != nil {
			t.Fatalf("UploadCapabilityProjection(%d).Commitment() error = %v, want nil", index, err)
		}
	}
	grantPayload := distribution.PublicationGrantPayload{
		Request: commitment, Authorization: publicationAuthAuthorityNonce(t, 0x61),
		Commitments: commitments,
		IssuedAt:    request.document.Certificate.Body.IssuedAt,
		ExpiresAt:   firstTarget.ExpiresAt,
	}
	projection, err := distribution.IssuePublicationGrant(distribution.PublicationGrantIssuance{
		Signer: signer, Capabilities: projections, Payload: grantPayload,
	})
	if err != nil {
		t.Fatalf("distribution.IssuePublicationGrant() error = %v, want nil", err)
	}
	encoded, err := projection.MarshalJSON()
	if err != nil {
		t.Fatalf("PublicationGrantProjection.MarshalJSON() error = %v, want nil", err)
	}
	var document distribution.PublicationGrantDocument
	if err := document.UnmarshalJSON(encoded); err != nil {
		t.Fatalf("PublicationGrantDocument.UnmarshalJSON() error = %v, want nil", err)
	}
	verified, err := distribution.VerifyPublicationGrant(distribution.PublicationGrantExpectation{
		Request: payload, Document: document, TrustedKeys: trusted,
		ObservedAt: request.document.Certificate.Body.IssuedAt,
	})
	if err != nil {
		t.Fatalf("distribution.VerifyPublicationGrant() error = %v, want nil", err)
	}
	return projection, document, verified, firstTarget
}

func newPublicationAuthCompletion(
	t testing.TB,
	installation controlplanetest.Installation,
	releaseFixture publicationAuthRelease,
	request VerifiedPublication,
	grant distribution.VerifiedPublicationGrant,
	uploadTarget objectstore.UploadTarget,
) PublicationCompletionDocument {
	t.Helper()
	var sources [release.PublicationObjectCount]distribution.PublicationSource
	for index, payload := range releaseFixture.payloads {
		sources[index] = distribution.PublicationSource{Reader: bytes.NewReader(payload)}
	}
	plan, err := distribution.PreparePublicationPlan(distribution.PublicationPlanRequest{
		Grant: grant, Manifest: releaseFixture.verified, Sources: sources,
		Policy: publicationAuthObjectstorePolicy(t, installation, uploadTarget),
	})
	if err != nil {
		t.Fatalf("distribution.PreparePublicationPlan() error = %v, want nil", err)
	}
	receipts, err := deploy.ReleaseGCS(
		context.Background(), publicationAuthObjectstoreClient(t), plan,
	)
	if err != nil {
		t.Fatalf("deploy.ReleaseGCS() error = %v, want nil", err)
	}
	innerRequest, err := distribution.VerifyPublicationRequest(distribution.PublicationRequestVerification{
		Document:     request.document.Request,
		RequestKeys:  publicationAuthTrustedKeys(t, installation.DevicePrivate),
		ManifestKeys: releaseFixture.keys, ExpectedOffering: installation.Build.Offering(),
	})
	if err != nil {
		t.Fatalf("distribution.VerifyPublicationRequest() error = %v, want nil", err)
	}
	projection, err := distribution.IssuePublicationCompletion(distribution.PublicationCompletionIssuance{
		Signer: installation.DevicePrivate, Receipts: receipts, Grant: grant, Request: innerRequest,
	})
	if err != nil {
		t.Fatalf("distribution.IssuePublicationCompletion() error = %v, want nil", err)
	}
	credentialed, err := AssemblePublicationCompletionProjection(PublicationCompletionProjectionAssembly{
		Completion: projection, Certificate: installation.Certificate,
	})
	if err != nil {
		t.Fatalf("AssemblePublicationCompletionProjection() error = %v, want nil", err)
	}
	encoded, err := credentialed.MarshalJSON()
	if err != nil {
		t.Fatalf("PublicationCompletionProjection.MarshalJSON() error = %v, want nil", err)
	}
	strict, err := core.EncodeValidatedJSON(credentialed, core.DefaultStrictJSONLimits())
	if err != nil || !bytes.Equal(strict, encoded) {
		t.Fatalf("EncodeValidatedJSON(PublicationCompletionProjection) = (%d bytes, %v), want exact %d-byte receive-only projection",
			len(strict), err, len(encoded))
	}
	var document PublicationCompletionDocument
	if err := document.UnmarshalJSON(encoded); err != nil {
		t.Fatalf("PublicationCompletionDocument.UnmarshalJSON() error = %v, want nil", err)
	}
	return document
}

func publicationAuthUploadFixture(
	t testing.TB,
	index int,
) publicationAuthUpload {
	t.Helper()
	document := struct {
		Provider  string `json:"provider"`
		Method    string `json:"method"`
		URL       string `json:"url"`
		ExpiresAt int64  `json:"expires_at"`
	}{
		Provider: objectstore.ProviderGoogleCloudStorage.String(),
		Method:   objectstore.UploadMethodTokenSignedPut,
		URL: "https://storage.googleapis.com/publication-auth/object-" + strconv.Itoa(index) +
			"?X-Goog-Signature=signature&X-Goog-SignedHeaders=" +
			url.QueryEscape("host;x-goog-hash;x-goog-if-generation-match"),
		ExpiresAt: time.Date(2035, time.January, 1, 0, 0, 0, 0, time.UTC).UnixNano(),
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("json.Marshal(UploadCapability) error = %v, want nil", err)
	}
	var capability objectstore.UploadCapability
	if err := capability.UnmarshalJSON(encoded); err != nil {
		t.Fatalf("objectstore.UploadCapability.UnmarshalJSON() error = %v, want nil", err)
	}
	target, err := capability.Target()
	if err != nil {
		t.Fatalf("objectstore.UploadCapability.Target() error = %v, want nil", err)
	}
	projection, err := objectstore.NewUploadCapabilityProjection(
		objectstore.ProviderGoogleCloudStorage, target,
	)
	if err != nil {
		t.Fatalf("objectstore.NewUploadCapabilityProjection() error = %v, want nil", err)
	}
	return publicationAuthUpload{projection: projection, target: target}
}

func publicationAuthIntegrity(t testing.TB, payload []byte) objectstore.Integrity {
	t.Helper()
	length, err := core.NewByteLength(uint64(len(payload)))
	if err != nil {
		t.Fatalf("core.NewByteLength() error = %v, want nil", err)
	}
	digest := sha256.Sum256(payload)
	return objectstore.Integrity{
		Length: length, SHA256: core.NewSHA256Digest(digest),
		CRC32C: core.NewCRC32C(crc32.Checksum(payload, crc32.MakeTable(crc32.Castagnoli))),
	}
}

func publicationAuthPrivateKey(marker byte) ed25519.PrivateKey {
	seed := distributionAuthSeed(marker)
	return ed25519.NewKeyFromSeed(seed[:])
}

func publicationAuthTrustedKeys(t testing.TB, signer ed25519.PrivateKey) attest.TrustedKeys {
	t.Helper()
	public, err := core.NewEd25519PublicKey(signer.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatalf("core.NewEd25519PublicKey() error = %v, want nil", err)
	}
	trusted, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
		Keys: []core.Ed25519PublicKey{public},
	})
	if err != nil {
		t.Fatalf("attest.NewTrustedKeys() error = %v, want nil", err)
	}
	return trusted
}

func publicationAuthAuthorityNonce(t testing.TB, marker byte) controlwire.AuthorityNonce {
	t.Helper()
	raw := [controlwire.NonceBytes]byte{}
	for index := range raw {
		raw[index] = marker
	}
	nonce, err := controlwire.NewAuthorityNonce(raw)
	if err != nil {
		t.Fatalf("controlwire.NewAuthorityNonce() error = %v, want nil", err)
	}
	return nonce
}

func publicationAuthObjectstoreClient(t testing.TB) objectstore.Client {
	t.Helper()
	exchangeClient, err := exchange.NewClient(&http.Client{Transport: &publicationAuthTransport{}})
	if err != nil {
		t.Fatalf("exchange.NewClient() error = %v, want nil", err)
	}
	objectClient, err := objectstore.NewClient(exchangeClient)
	if err != nil {
		t.Fatalf("objectstore.NewClient() error = %v, want nil", err)
	}
	return objectClient
}

func publicationAuthObjectstorePolicy(
	t testing.TB,
	installation controlplanetest.Installation,
	target objectstore.UploadTarget,
) objectstore.Policy {
	t.Helper()
	operation, err := target.ExpiresAt.Since(installation.Certificate.Body.IssuedAt)
	if err != nil {
		t.Fatalf("upload expiry Since(certificate issue) error = %v, want nil", err)
	}
	limit, err := core.NewByteCount(4 << 10)
	if err != nil {
		t.Fatalf("core.NewByteCount(error body limit) error = %v, want nil", err)
	}
	return objectstore.Policy{
		OperationTimeout: operation, AttemptTimeout: operation, ErrorBodyLimit: limit,
	}
}
