package release

import (
	"crypto/ed25519"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

type releaseFixture struct {
	builds         [TargetCount]core.BuildIdentity
	manifestKey    ed25519.PrivateKey
	latestKey      ed25519.PrivateKey
	artifacts      [TargetCount]Artifact
	artifactSet    ArtifactSet
	latest         LatestDocument
	manifest       ManifestDocument
	verified       VerifiedManifest
	verifiedLatest VerifiedLatest
	manifestTrust  attest.TrustedKeys
	latestTrust    attest.TrustedKeys
}

func latestTimeEvidenceAt(t testing.TB, nanoseconds int64) LatestTimeEvidence {
	t.Helper()

	observation, err := temporal.NewObservation(time.Unix(0, nanoseconds).UTC())
	if err != nil {
		t.Fatalf("temporal.NewObservation(%d) error = %v, want nil", nanoseconds, err)
	}
	return LatestTimeEvidence{
		StartedAt: observation, ObservedAt: observation,
		DurableHighWater: temporal.InstantFromNanoseconds(nanoseconds),
	}
}

func newReleaseFixture(t testing.TB, version core.ReleaseVersion, generation uint64) releaseFixture {
	t.Helper()
	return newReleaseFixtureForOffering(t, releaseOffering(t, 2), version, generation)
}

func newReleaseFixtureForOffering(
	t testing.TB,
	offering core.Offering,
	version core.ReleaseVersion,
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
	targets := Targets()
	var builds [TargetCount]core.BuildIdentity
	var artifacts [TargetCount]Artifact
	for index := range TargetCount {
		target, ok := targets.At(index)
		if !ok {
			t.Fatalf("Targets().At(%d) ok = false, want true", index)
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
		sum := sha256.Sum256([]byte{byte(index + 1)})
		artifacts[index], err = NewArtifact(ArtifactRequest{
			Build:  builds[index],
			Extent: mustByteCount(t, uint64(index+1)),
			SHA256: core.NewSHA256Digest(sum),
			CRC32C: core.NewCRC32C(uint32(index)),
		})
		if err != nil {
			t.Fatalf("NewArtifact(%d) error = %v", index, err)
		}
	}
	artifactSet, err := NewArtifactSet(ArtifactSetRequest{Artifacts: artifacts})
	if err != nil {
		t.Fatalf("NewArtifactSet() error = %v", err)
	}
	metadata := fixtureMetadataSet(t)
	provenance := fixtureBuildProvenance(t)
	fact, err := NewManifestFact(ManifestFactRequest{
		Revision:   Revision2026V1,
		Offering:   offering,
		Version:    version,
		Commit:     commit,
		CreatedAt:  temporal.InstantFromNanoseconds(1_000),
		Artifacts:  artifactSet,
		Provenance: provenance,
		Metadata:   metadata,
	})
	if err != nil {
		t.Fatalf("NewManifestFact() error = %v", err)
	}
	manifest, err := IssueManifest(IssueManifestRequest{Fact: fact, Signer: manifestKey})
	if err != nil {
		t.Fatalf("IssueManifest() error = %v", err)
	}
	verified, err := VerifyManifest(VerifyManifestRequest{
		Document: manifest, TrustedKeys: manifestTrust, ExpectedOffering: offering,
	})
	if err != nil {
		t.Fatalf("VerifyManifest() error = %v", err)
	}
	latest, err := IssueLatest(IssueLatestRequest{
		Manifest:   verified,
		Generation: mustGeneration(t, generation),
		IssuedAt:   temporal.InstantFromNanoseconds(2_000),
		ValidFrom:  temporal.InstantFromNanoseconds(2_000),
		ValidUntil: temporal.InstantFromNanoseconds(2_000 + int64(ReleaseLatestMaximumLifetimeNanoseconds)),
		Key:        latestKey,
	})
	if err != nil {
		t.Fatalf("IssueLatest() error = %v", err)
	}
	verifiedLatest, err := VerifyLatest(VerifyLatestRequest{
		Document: latest, LatestKeys: latestTrust, ManifestKeys: manifestTrust,
		ExpectedOffering: offering,
	})
	if err != nil {
		t.Fatalf("VerifyLatest() error = %v", err)
	}
	return releaseFixture{
		manifestKey: manifestKey, latestKey: latestKey,
		manifestTrust: manifestTrust, latestTrust: latestTrust,
		builds: builds, artifacts: artifacts, artifactSet: artifactSet,
		manifest: manifest, verified: verified, latest: latest, verifiedLatest: verifiedLatest,
	}
}

func fixtureMetadataSet(t testing.TB) MetadataSet {
	t.Helper()
	var assets [MetadataAssetCount]MetadataAsset
	for index := range MetadataAssetCount {
		digest := sha256.Sum256([]byte{byte(index + 41)})
		asset, err := NewMetadataAsset(MetadataAssetRequest{
			Kind: MetadataKind(index + 1), Extent: mustByteCount(t, uint64(index+11)),
			SHA256: core.NewSHA256Digest(digest), CRC32C: core.NewCRC32C(uint32(index + 31)),
		})
		if err != nil {
			t.Fatalf("NewMetadataAsset(%d) error = %v", index, err)
		}
		assets[index] = asset
	}
	set, err := NewMetadataSet(MetadataSetRequest{Assets: assets})
	if err != nil {
		t.Fatalf("NewMetadataSet() error = %v", err)
	}
	return set
}

func fixtureBuildProvenance(t testing.TB) BuildProvenance {
	t.Helper()
	goDigest := sha256.Sum256([]byte("go tool"))
	mainPackage, err := ParseMainPackage("github.com/example/product/cmd/product")
	if err != nil {
		t.Fatalf("ParseMainPackage() error = %v", err)
	}
	value := BuildProvenance{
		linkerAssignments:  emptyLinkerAssignmentsForTest(),
		mainPackage:        mainPackage,
		goExecutableDigest: core.NewSHA256Digest(goDigest),
		goToolchain:        CurrentGoToolchain(),
		moduleMode:         BuildModuleReadonly,
		valid:              true,
	}
	if err := value.Validate(); err != nil {
		t.Fatalf("BuildProvenance.Validate() error = %v", err)
	}
	return value
}

func emptyLinkerAssignmentsForTest() LinkerAssignments {
	value, _ := NewLinkerAssignments(nil)
	return value
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
		public, err := core.NewEd25519PublicKey(private.Public().(ed25519.PublicKey))
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

func mustGeneration(t testing.TB, value uint64) Generation {
	t.Helper()
	generation, err := NewGeneration(value)
	if err != nil {
		t.Fatalf("NewGeneration(%d) error = %v", value, err)
	}
	return generation
}

func issueVerifiedLatest(
	t testing.TB,
	fixture releaseFixture,
	manifest VerifiedManifest,
	generation uint64,
	issuedAt int64,
	validFrom int64,
	validUntil int64,
) VerifiedLatest {
	t.Helper()
	document, err := IssueLatest(IssueLatestRequest{
		Manifest: manifest, Generation: mustGeneration(t, generation),
		IssuedAt:   temporal.InstantFromNanoseconds(issuedAt),
		ValidFrom:  temporal.InstantFromNanoseconds(validFrom),
		ValidUntil: temporal.InstantFromNanoseconds(validUntil),
		Key:        fixture.latestKey,
	})
	if err != nil {
		t.Fatalf("IssueLatest(generation %d) error = %v", generation, err)
	}
	verified, err := VerifyLatest(VerifyLatestRequest{
		Document: document, LatestKeys: fixture.latestTrust,
		ManifestKeys:     fixture.manifestTrust,
		ExpectedOffering: releaseOffering(t, 2),
	})
	if err != nil {
		t.Fatalf("VerifyLatest(generation %d) error = %v", generation, err)
	}
	return verified
}
