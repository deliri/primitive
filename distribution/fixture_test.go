package distribution_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"hash/crc32"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/release"
	"github.com/deliri/primitive/v2026/temporal"
)

type releaseFixture struct {
	payloads  [release.PublicationObjectCount][]byte
	document  release.ManifestDocument
	manifest  release.VerifiedManifest
	latest    release.VerifiedLatest
	artifacts [release.TargetCount]release.Artifact
	builds    [release.TargetCount]core.BuildIdentity
}

func newReleaseFixture(
	t testing.TB,
	version core.ReleaseVersion,
	generation uint64,
) releaseFixture {
	t.Helper()
	commit, err := core.ParseBuildCommit("b5c32d95d212b0a1a8cef4126e4d11ff288079ef")
	if err != nil {
		t.Fatalf("core.ParseBuildCommit() error = %v, want nil", err)
	}
	targets := release.Targets()
	var fixture releaseFixture
	for index := range release.TargetCount {
		platform, ok := targets.At(index)
		if !ok {
			t.Fatalf("release.Targets().At(%d) ok = false, want true", index)
		}
		build, buildErr := core.NewBuildIdentity(core.BuildIdentityRequest{
			Offering: core.OfferingBug, Version: version, Commit: commit, Platform: platform,
		})
		if buildErr != nil {
			t.Fatalf("core.NewBuildIdentity(%d) error = %v, want nil", index, buildErr)
		}
		payload := releasePayload(index, version)
		integrity := objectIntegrity(t, payload)
		extent, extentErr := core.NewByteCount(integrity.Length.Uint64())
		if extentErr != nil {
			t.Fatalf("core.NewByteCount(artifact %d) error = %v, want nil", index, extentErr)
		}
		artifact, artifactErr := release.NewArtifact(release.ArtifactRequest{
			Build: build, Extent: extent,
			SHA256: integrity.SHA256, CRC32C: integrity.CRC32C,
		})
		if artifactErr != nil {
			t.Fatalf("release.NewArtifact(%d) error = %v, want nil", index, artifactErr)
		}
		fixture.builds[index] = build
		fixture.artifacts[index] = artifact
		fixture.payloads[index] = payload
	}
	artifacts, err := release.NewArtifactSet(release.ArtifactSetRequest{Artifacts: fixture.artifacts})
	if err != nil {
		t.Fatalf("release.NewArtifactSet() error = %v, want nil", err)
	}
	metadata, metadataPayloads := releaseMetadataFixture(t, version)
	for index, payload := range metadataPayloads {
		fixture.payloads[release.TargetCount+1+index] = payload
	}
	fact, err := release.NewManifestFact(release.ManifestFactRequest{
		Revision: release.Revision2026V1, Offering: core.OfferingBug,
		Version: version, Commit: commit, CreatedAt: temporal.InstantFromNanoseconds(1_000),
		Artifacts: artifacts, Provenance: releaseProvenance(t), Metadata: metadata,
	})
	if err != nil {
		t.Fatalf("release.NewManifestFact() error = %v, want nil", err)
	}
	key := signingKey(19)
	document, err := release.IssueManifest(release.IssueManifestRequest{Signer: key, Fact: fact})
	if err != nil {
		t.Fatalf("release.IssueManifest() error = %v, want nil", err)
	}
	fixture.document = document
	fixture.payloads[release.TargetCount], err = json.Marshal(document)
	if err != nil {
		t.Fatalf("json.Marshal(release.ManifestDocument) error = %v, want nil", err)
	}
	trusted := trustedKeys(t, key)
	fixture.manifest, err = release.VerifyManifest(release.VerifyManifestRequest{
		Document: document, TrustedKeys: trusted, ExpectedOffering: core.OfferingBug,
	})
	if err != nil {
		t.Fatalf("release.VerifyManifest() error = %v, want nil", err)
	}
	generationValue, err := release.NewGeneration(generation)
	if err != nil {
		t.Fatalf("release.NewGeneration() error = %v, want nil", err)
	}
	latestDocument, err := release.IssueLatest(release.IssueLatestRequest{
		Key: key, Manifest: fixture.manifest, Generation: generationValue,
		IssuedAt:   temporal.InstantFromNanoseconds(1_000),
		ValidFrom:  temporal.InstantFromNanoseconds(2_000),
		ValidUntil: temporal.InstantFromNanoseconds(10_000),
	})
	if err != nil {
		t.Fatalf("release.IssueLatest() error = %v, want nil", err)
	}
	fixture.latest, err = release.VerifyLatest(release.VerifyLatestRequest{
		Document: latestDocument, LatestKeys: trusted, ManifestKeys: trusted,
		ExpectedOffering: core.OfferingBug,
	})
	if err != nil {
		t.Fatalf("release.VerifyLatest() error = %v, want nil", err)
	}
	return fixture
}

func availableSummaryFixture(
	t testing.TB,
	installed releaseFixture,
	candidate releaseFixture,
) release.AvailableSummary {
	t.Helper()

	const platformIndex = 2
	filename, err := candidate.artifacts[platformIndex].Filename()
	if err != nil {
		t.Fatalf("release.Artifact.Filename() error = %v, want nil", err)
	}
	summary := release.AvailableSummary{
		Installed: installed.builds[platformIndex], Candidate: candidate.builds[platformIndex],
		Manifest: candidate.manifest.Identity(), ManifestDocument: candidate.manifest.DocumentDigest(),
		Artifact: candidate.artifacts[platformIndex].Identity(), Filename: filename,
		Integrity:  candidate.artifacts[platformIndex].Integrity(),
		ValidUntil: candidate.latest.Fact().ValidUntil(),
	}
	if err := summary.Validate(); err != nil {
		t.Fatalf("release.AvailableSummary.Validate() error = %v, want nil", err)
	}
	return summary
}

func releaseMetadataFixture(
	t testing.TB,
	version core.ReleaseVersion,
) (release.MetadataSet, [release.MetadataAssetCount][]byte) {
	t.Helper()
	var assets [release.MetadataAssetCount]release.MetadataAsset
	var payloads [release.MetadataAssetCount][]byte
	for index := range release.MetadataAssetCount {
		payload := []byte("metadata-" + strconv.Itoa(index) + "-" + version.String())
		integrity := objectIntegrity(t, payload)
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

func releaseProvenance(t testing.TB) release.BuildProvenance {
	t.Helper()
	goToolchain, err := release.CurrentGoToolchain().Version()
	if err != nil {
		t.Fatalf("release.CurrentGoToolchain().Version() error = %v, want nil", err)
	}
	goDigest := sha256.Sum256([]byte("go-tool"))
	garbleDigest := sha256.Sum256([]byte("garble-tool"))
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
		LinkerAssignments      []struct{}        `json:"linker_assignments"`
		GoExecutableSHA256     core.SHA256Digest `json:"go_executable_sha256"`
		GarbleExecutableSHA256 core.SHA256Digest `json:"garble_executable_sha256"`
	}{
		GarbleDerivation: "one", GarbleModule: "mvdan.cc/garble",
		GarbleVersion:   "v0.17.0",
		GarbleRevision:  "39c484d3007e9a608ac8692dab0b9bb5f71dfc2a",
		GarbleModuleSum: "h1:XJ6jJhlT8HTEU9Dd02nLDUciuyPDXGRopwy/Cuoo/0M=",
		GarbleLiterals:  "obfuscate", GarbleDiagnostics: "preserve",
		GoToolchain: goToolchain, MainPackage: "github.com/offGridSoft/bug/cmd/bug",
		ModuleMode: "vendor", LinkerAssignments: []struct{}{},
		GoExecutableSHA256:     core.NewSHA256Digest(goDigest),
		GarbleExecutableSHA256: core.NewSHA256Digest(garbleDigest),
	}
	encoded, err := json.Marshal(wire)
	if err != nil {
		t.Fatalf("json.Marshal(provenance) error = %v, want nil", err)
	}
	var provenance release.BuildProvenance
	if err := json.Unmarshal(encoded, &provenance); err != nil {
		t.Fatalf("json.Unmarshal(release.BuildProvenance) error = %v, want nil", err)
	}
	return provenance
}

func releasePayload(index int, version core.ReleaseVersion) []byte {
	return []byte("release-object-" + strconv.Itoa(index) + "-" + version.String())
}

func objectIntegrity(t testing.TB, payload []byte) objectstore.Integrity {
	t.Helper()
	digest := sha256.Sum256(payload)
	length, err := core.NewByteLength(uint64(len(payload)))
	if err != nil {
		t.Fatalf("core.NewByteLength() error = %v, want nil", err)
	}
	return objectstore.Integrity{
		Length: length, SHA256: core.NewSHA256Digest(digest),
		CRC32C: core.NewCRC32C(crc32.Checksum(payload, crc32.MakeTable(crc32.Castagnoli))),
	}
}

func signingKey(seedByte byte) ed25519.PrivateKey {
	seed := make([]byte, ed25519.SeedSize)
	for index := range seed {
		seed[index] = seedByte + byte(index)
	}
	return ed25519.NewKeyFromSeed(seed)
}

func trustedKeys(t testing.TB, key ed25519.PrivateKey) attest.TrustedKeys {
	t.Helper()
	public, err := core.NewEd25519PublicKey(key.Public().(ed25519.PublicKey))
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

func requestNonce(t testing.TB, marker byte) controlwire.RequestNonce {
	t.Helper()
	var raw [controlwire.NonceBytes]byte
	for index := range raw {
		raw[index] = marker + byte(index)
	}
	nonce, err := controlwire.NewRequestNonce(raw)
	if err != nil {
		t.Fatalf("controlwire.NewRequestNonce() error = %v, want nil", err)
	}
	return nonce
}

func authorityNonce(t testing.TB, marker byte) controlwire.AuthorityNonce {
	t.Helper()
	var raw [controlwire.NonceBytes]byte
	for index := range raw {
		raw[index] = marker + byte(index)
	}
	nonce, err := controlwire.NewAuthorityNonce(raw)
	if err != nil {
		t.Fatalf("controlwire.NewAuthorityNonce() error = %v, want nil", err)
	}
	return nonce
}

func uploadCapabilityProjection(
	t testing.TB,
	index int,
) (objectstore.UploadCapabilityProjection, objectstore.UploadCapability) {
	t.Helper()
	rawURL := "https://storage.googleapis.com/bucket/object-" + strconv.Itoa(index) +
		"?X-Goog-Signature=signature&X-Goog-SignedHeaders=" +
		url.QueryEscape("host;x-goog-hash;x-goog-if-generation-match")
	document := struct {
		Provider  string `json:"provider"`
		Method    string `json:"method"`
		URL       string `json:"url"`
		ExpiresAt int64  `json:"expires_at"`
	}{
		Provider: objectstore.ProviderGoogleCloudStorage.String(),
		Method:   objectstore.UploadMethodTokenSignedPut, URL: rawURL,
		ExpiresAt: time.Date(2035, time.January, 1, 0, 0, 0, 0, time.UTC).UnixNano(),
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("json.Marshal(upload capability) error = %v, want nil", err)
	}
	var capability objectstore.UploadCapability
	if err := json.Unmarshal(encoded, &capability); err != nil {
		t.Fatalf("json.Unmarshal(objectstore.UploadCapability) error = %v, want nil", err)
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
	return projection, capability
}

func downloadCapabilityProjection(
	t testing.TB,
	index int,
) (objectstore.DownloadCapabilityProjection, objectstore.DownloadCapability) {
	t.Helper()
	rawURL := "https://storage.googleapis.com/bucket/object-" + strconv.Itoa(index) +
		"?X-Goog-Signature=signature&X-Goog-SignedHeaders=" + url.QueryEscape("host")
	document := struct {
		Provider  string `json:"provider"`
		Method    string `json:"method"`
		URL       string `json:"url"`
		ExpiresAt int64  `json:"expires_at"`
	}{
		Provider: objectstore.ProviderGoogleCloudStorage.String(),
		Method:   objectstore.DownloadMethodTokenSignedGet, URL: rawURL,
		ExpiresAt: time.Date(2035, time.January, 1, 0, 0, 0, 0, time.UTC).UnixNano(),
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("json.Marshal(download capability) error = %v, want nil", err)
	}
	var capability objectstore.DownloadCapability
	if err := json.Unmarshal(encoded, &capability); err != nil {
		t.Fatalf("json.Unmarshal(objectstore.DownloadCapability) error = %v, want nil", err)
	}
	target, err := capability.Target()
	if err != nil {
		t.Fatalf("objectstore.DownloadCapability.Target() error = %v, want nil", err)
	}
	projection, err := objectstore.NewDownloadCapabilityProjection(
		objectstore.ProviderGoogleCloudStorage, target,
	)
	if err != nil {
		t.Fatalf("objectstore.NewDownloadCapabilityProjection() error = %v, want nil", err)
	}
	return projection, capability
}

func objectstoreClient(t testing.TB, transport http.RoundTripper) objectstore.Client {
	t.Helper()
	exchangeClient, err := exchange.NewClient(&http.Client{Transport: transport})
	if err != nil {
		t.Fatalf("exchange.NewClient() error = %v, want nil", err)
	}
	objectClient, err := objectstore.NewClient(exchangeClient)
	if err != nil {
		t.Fatalf("objectstore.NewClient() error = %v, want nil", err)
	}
	return objectClient
}

func objectstorePolicy(t testing.TB) objectstore.Policy {
	t.Helper()
	operation, err := temporal.DurationFromSeconds(10)
	if err != nil {
		t.Fatalf("temporal.DurationFromSeconds(operation) error = %v, want nil", err)
	}
	attempt, err := temporal.DurationFromSeconds(5)
	if err != nil {
		t.Fatalf("temporal.DurationFromSeconds(attempt) error = %v, want nil", err)
	}
	limit, err := core.NewByteCount(4 << 10)
	if err != nil {
		t.Fatalf("core.NewByteCount(error body limit) error = %v, want nil", err)
	}
	return objectstore.Policy{
		OperationTimeout: operation, AttemptTimeout: attempt, ErrorBodyLimit: limit,
	}
}
