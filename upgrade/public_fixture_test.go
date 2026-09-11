package upgrade

import (
	"crypto/ed25519"
	"crypto/sha256"
	json "encoding/json/v2"
	"github.com/deliri/primitive/v2026/release"
	"hash/crc32"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

type releaseFixture struct {
	verified       release.VerifiedManifest
	verifiedLatest release.VerifiedLatest
}

func latestTimeEvidenceAt(t testing.TB, nanoseconds int64) release.LatestTimeEvidence {
	t.Helper()

	observation, err := temporal.NewObservation(time.Unix(0, nanoseconds).UTC())
	if err != nil {
		t.Fatalf("temporal.NewObservation(%d) error = %v, want nil", nanoseconds, err)
	}
	return release.LatestTimeEvidence{
		StartedAt: observation, ObservedAt: observation,
		DurableHighWater: temporal.InstantFromNanoseconds(nanoseconds),
	}
}

func newReleaseFixtureForOffering(
	t testing.TB,
	offering core.Offering,
	version core.ReleaseVersion,
	payload []byte,
	generation uint64,
) releaseFixture {
	t.Helper()
	manifestKey := deterministicKey(11)
	latestKey := deterministicKey(29)
	manifestTrust := trustedKey(t, manifestKey)
	latestTrust := trustedKey(t, latestKey)
	commit, err := core.ParseBuildCommit("0123456789abcdef0123456789abcdef01234567")
	if err != nil {
		t.Fatalf("core.ParseBuildCommit() error = %v", err)
	}
	targets := release.Targets()
	var builds [release.TargetCount]core.BuildIdentity
	var artifacts [release.TargetCount]release.Artifact
	for index := range release.TargetCount {
		target, ok := targets.At(index)
		if !ok {
			t.Fatalf("release.Targets().At(%d) ok = false, want true", index)
		}
		builds[index], err = core.NewBuildIdentity(core.BuildIdentityRequest{
			Offering: offering,
			Version:  version,
			Commit:   commit,
			Platform: target,
		})
		if err != nil {
			t.Fatalf("core.NewBuildIdentity(%d) error = %v", index, err)
		}
		sum := sha256.Sum256(payload)
		artifacts[index], err = release.NewArtifact(release.ArtifactRequest{
			Build:  builds[index],
			Extent: mustByteCount(t, uint64(len(payload))),
			SHA256: core.NewSHA256Digest(sum),
			CRC32C: core.NewCRC32C(crc32.Checksum(payload, crc32.MakeTable(crc32.Castagnoli))),
		})
		if err != nil {
			t.Fatalf("release.NewArtifact(%d) error = %v", index, err)
		}
	}
	artifactSet, err := release.NewArtifactSet(release.ArtifactSetRequest{Artifacts: artifacts})
	if err != nil {
		t.Fatalf("release.NewArtifactSet() error = %v", err)
	}
	metadata := fixtureMetadataSet(t)
	provenance := fixtureBuildProvenance(t)
	fact, err := release.NewManifestFact(release.ManifestFactRequest{
		Revision:   release.Revision2026V1,
		Offering:   offering,
		Version:    version,
		Commit:     commit,
		CreatedAt:  temporal.InstantFromNanoseconds(1_000),
		Artifacts:  artifactSet,
		Provenance: provenance,
		Metadata:   metadata,
	})
	if err != nil {
		t.Fatalf("release.NewManifestFact() error = %v", err)
	}
	manifest, err := release.IssueManifest(release.IssueManifestRequest{Fact: fact, Signer: manifestKey})
	if err != nil {
		t.Fatalf("release.IssueManifest() error = %v", err)
	}
	verified, err := release.VerifyManifest(release.VerifyManifestRequest{
		Document: manifest, TrustedKeys: manifestTrust, ExpectedOffering: offering,
	})
	if err != nil {
		t.Fatalf("release.VerifyManifest() error = %v", err)
	}
	latest, err := release.IssueLatest(release.IssueLatestRequest{
		Manifest:   verified,
		Generation: mustGeneration(t, generation),
		IssuedAt:   temporal.InstantFromNanoseconds(2_000),
		ValidFrom:  temporal.InstantFromNanoseconds(2_000),
		ValidUntil: temporal.InstantFromNanoseconds(2_000 + int64(release.ReleaseLatestMaximumLifetimeNanoseconds)),
		Key:        latestKey,
	})
	if err != nil {
		t.Fatalf("release.IssueLatest() error = %v", err)
	}
	verifiedLatest, err := release.VerifyLatest(release.VerifyLatestRequest{
		Document: latest, LatestKeys: latestTrust, ManifestKeys: manifestTrust,
		ExpectedOffering: offering,
	})
	if err != nil {
		t.Fatalf("release.VerifyLatest() error = %v", err)
	}
	return releaseFixture{verified: verified, verifiedLatest: verifiedLatest}
}

func fixtureMetadataSet(t testing.TB) release.MetadataSet {
	t.Helper()
	var assets [release.MetadataAssetCount]release.MetadataAsset
	for index := range release.MetadataAssetCount {
		digest := sha256.Sum256([]byte{byte(index + 41)})
		asset, err := release.NewMetadataAsset(release.MetadataAssetRequest{
			Kind: release.MetadataKind(index + 1), Extent: mustByteCount(t, uint64(index+11)),
			SHA256: core.NewSHA256Digest(digest), CRC32C: core.NewCRC32C(uint32(index + 31)),
		})
		if err != nil {
			t.Fatalf("release.NewMetadataAsset(%d) error = %v", index, err)
		}
		assets[index] = asset
	}
	set, err := release.NewMetadataSet(release.MetadataSetRequest{Assets: assets})
	if err != nil {
		t.Fatalf("release.NewMetadataSet() error = %v", err)
	}
	return set
}

func fixtureBuildProvenance(t testing.TB) release.BuildProvenance {
	t.Helper()
	goToolchain, err := release.CurrentGoToolchain().Version()
	if err != nil {
		t.Fatalf("release.CurrentGoToolchain().Version() error = %v, want nil", err)
	}
	wire := struct {
		GoToolchain        string            `json:"go_toolchain"`
		MainPackage        string            `json:"main_package"`
		ModuleMode         string            `json:"module_mode"`
		BuildTags          []string          `json:"build_tags"`
		LinkerAssignments  []struct{}        `json:"linker_assignments"`
		GoExecutableSHA256 core.SHA256Digest `json:"go_executable_sha256"`
	}{
		GoToolchain: goToolchain, MainPackage: "github.com/example/product/cmd/product",
		ModuleMode: "vendor", BuildTags: []string{}, LinkerAssignments: []struct{}{},
		GoExecutableSHA256: core.SHA256Of([]byte("publication-auth-go")),
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

func deterministicKey(seedByte byte) ed25519.PrivateKey {
	seed := make([]byte, ed25519.SeedSize)
	for index := range seed {
		seed[index] = seedByte + byte(index)
	}
	return ed25519.NewKeyFromSeed(seed)
}

func trustedKey(t testing.TB, private ed25519.PrivateKey) attest.TrustedKeys {
	t.Helper()
	return trustedKeys(t, private)
}

func trustedKeys(t testing.TB, privateKeys ...ed25519.PrivateKey) attest.TrustedKeys {
	t.Helper()
	keys := make([]core.Ed25519PublicKey, len(privateKeys))
	for index, private := range privateKeys {
		public, err := core.NewEd25519PublicKey(ed25519.PublicKey(private[ed25519.SeedSize:]))
		if err != nil {
			t.Fatalf("NewEd25519PublicKey(%d) error = %v", index, err)
		}
		keys[index] = public
	}
	trusted, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{Keys: keys})
	if err != nil {
		t.Fatalf("NewTrustedKeys() error = %v", err)
	}
	return trusted
}

func mustByteCount(t testing.TB, value uint64) core.ByteCount {
	t.Helper()
	count, err := core.NewByteCount(value)
	if err != nil {
		t.Fatalf("NewByteCount(%d) error = %v", value, err)
	}
	return count
}

func mustGeneration(t testing.TB, value uint64) release.Generation {
	t.Helper()
	generation, err := release.NewGeneration(value)
	if err != nil {
		t.Fatalf("release.NewGeneration(%d) error = %v", value, err)
	}
	return generation
}
