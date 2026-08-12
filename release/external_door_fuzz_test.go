package release

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// releaseJSONDoor is the explicit fuzz inventory for every public Release JSON
// ingress. One target is intentional: every corpus seed carries its door, and
// the callback dispatches to that concrete public UnmarshalJSON method rather
// than merely exercising whichever nested decoder a top-level document happens
// to reach.
type releaseJSONDoor uint8

const (
	releaseJSONDoorUnknown releaseJSONDoor = iota
	releaseJSONDoorAvailableSummary
	releaseJSONDoorPublicationRole
	releaseJSONDoorGeneration
	releaseJSONDoorLatestIdentity
	releaseJSONDoorLatestFact
	releaseJSONDoorLatestDocument
	releaseJSONDoorMetadataKind
	releaseJSONDoorMetadataAsset
	releaseJSONDoorMetadataSet
	releaseJSONDoorBuildProvenance
	releaseJSONDoorRevision
	releaseJSONDoorBuildDependencies
	releaseJSONDoorBinaryFilename
	releaseJSONDoorArtifactIdentity
	releaseJSONDoorArtifactIntegrity
	releaseJSONDoorArtifact
	releaseJSONDoorArtifactSet
	releaseJSONDoorManifestIdentity
	releaseJSONDoorManifestDocumentDigest
	releaseJSONDoorManifestFact
	releaseJSONDoorManifestDocument
	releaseJSONDoorLimit
)

func (d releaseJSONDoor) receiverName() string {
	switch d {
	case releaseJSONDoorAvailableSummary:
		return "AvailableSummary"
	case releaseJSONDoorPublicationRole:
		return "PublicationRole"
	case releaseJSONDoorGeneration:
		return "Generation"
	case releaseJSONDoorLatestIdentity:
		return "LatestIdentity"
	case releaseJSONDoorLatestFact:
		return "LatestFact"
	case releaseJSONDoorLatestDocument:
		return "LatestDocument"
	case releaseJSONDoorMetadataKind:
		return "MetadataKind"
	case releaseJSONDoorMetadataAsset:
		return "MetadataAsset"
	case releaseJSONDoorMetadataSet:
		return "MetadataSet"
	case releaseJSONDoorBuildProvenance:
		return "BuildProvenance"
	case releaseJSONDoorRevision:
		return "Revision"
	case releaseJSONDoorBuildDependencies:
		return "BuildDependencies"
	case releaseJSONDoorBinaryFilename:
		return "BinaryFilename"
	case releaseJSONDoorArtifactIdentity:
		return "ArtifactIdentity"
	case releaseJSONDoorArtifactIntegrity:
		return "ArtifactIntegrity"
	case releaseJSONDoorArtifact:
		return "Artifact"
	case releaseJSONDoorArtifactSet:
		return "ArtifactSet"
	case releaseJSONDoorManifestIdentity:
		return "ManifestIdentity"
	case releaseJSONDoorManifestDocumentDigest:
		return "ManifestDocumentDigest"
	case releaseJSONDoorManifestFact:
		return "ManifestFact"
	case releaseJSONDoorManifestDocument:
		return "ManifestDocument"
	case releaseJSONDoorUnknown, releaseJSONDoorLimit:
		return ""
	default:
		return ""
	}
}

type releaseJSONDoorSeed struct {
	document []byte
	door     releaseJSONDoor
}

type releaseJSONDoorFixtures struct {
	available              AvailableSummary
	publicationRole        PublicationRole
	generation             Generation
	latestIdentity         LatestIdentity
	latestFact             LatestFact
	latestDocument         LatestDocument
	metadataKind           MetadataKind
	metadataAsset          MetadataAsset
	metadataSet            MetadataSet
	buildProvenance        BuildProvenance
	revision               Revision
	buildDependencies      BuildDependencies
	binaryFilename         BinaryFilename
	artifactIdentity       ArtifactIdentity
	artifactIntegrity      ArtifactIntegrity
	artifact               Artifact
	artifactSet            ArtifactSet
	manifestIdentity       ManifestIdentity
	manifestDocumentDigest ManifestDocumentDigest
	manifestFact           ManifestFact
	manifestDocument       ManifestDocument
	release                releaseFixture
}

func FuzzReleaseExternalJSONDoorInventory(f *testing.F) {
	fixtures := releaseJSONFixturesForFuzz(f)
	for _, seed := range releaseJSONSeedsForFuzz(f, fixtures) {
		f.Add(uint8(seed.door), seed.document)
	}
	for _, hostile := range [][]byte{
		nil,
		{},
		[]byte(`null`),
		[]byte(`{}`),
		[]byte(`[]`),
		[]byte(`""`),
		[]byte(`0`),
		[]byte(`true`),
		[]byte(`{`),
		bytes.Repeat([]byte(`[`), core.JSONNestingDepthMaximum+1),
	} {
		f.Add(uint8(releaseJSONDoorManifestDocument), hostile)
	}

	f.Fuzz(func(t *testing.T, rawDoor uint8, data []byte) {
		door := releaseJSONDoor(rawDoor)
		switch door {
		case releaseJSONDoorAvailableSummary:
			fuzzReleaseJSONValue(t, data, fixtures.available)
		case releaseJSONDoorPublicationRole:
			fuzzReleaseJSONValue(t, data, fixtures.publicationRole)
		case releaseJSONDoorGeneration:
			fuzzReleaseJSONValue(t, data, fixtures.generation)
		case releaseJSONDoorLatestIdentity:
			fuzzReleaseJSONValue(t, data, fixtures.latestIdentity)
		case releaseJSONDoorLatestFact:
			fuzzReleaseJSONValue(t, data, fixtures.latestFact)
		case releaseJSONDoorLatestDocument:
			fuzzReleaseLatestDocument(t, data, fixtures)
		case releaseJSONDoorMetadataKind:
			fuzzReleaseJSONValue(t, data, fixtures.metadataKind)
		case releaseJSONDoorMetadataAsset:
			fuzzReleaseJSONValue(t, data, fixtures.metadataAsset)
		case releaseJSONDoorMetadataSet:
			fuzzReleaseJSONValue(t, data, fixtures.metadataSet)
		case releaseJSONDoorBuildProvenance:
			fuzzReleaseJSONValue(t, data, fixtures.buildProvenance)
		case releaseJSONDoorRevision:
			fuzzReleaseJSONValue(t, data, fixtures.revision)
		case releaseJSONDoorBuildDependencies:
			fuzzReleaseJSONValue(t, data, fixtures.buildDependencies)
		case releaseJSONDoorBinaryFilename:
			fuzzReleaseJSONValue(t, data, fixtures.binaryFilename)
		case releaseJSONDoorArtifactIdentity:
			fuzzReleaseJSONValue(t, data, fixtures.artifactIdentity)
		case releaseJSONDoorArtifactIntegrity:
			fuzzReleaseJSONValue(t, data, fixtures.artifactIntegrity)
		case releaseJSONDoorArtifact:
			fuzzReleaseJSONValue(t, data, fixtures.artifact)
		case releaseJSONDoorArtifactSet:
			fuzzReleaseJSONValue(t, data, fixtures.artifactSet)
		case releaseJSONDoorManifestIdentity:
			fuzzReleaseJSONValue(t, data, fixtures.manifestIdentity)
		case releaseJSONDoorManifestDocumentDigest:
			fuzzReleaseJSONValue(t, data, fixtures.manifestDocumentDigest)
		case releaseJSONDoorManifestFact:
			fuzzReleaseJSONValue(t, data, fixtures.manifestFact)
		case releaseJSONDoorManifestDocument:
			fuzzReleaseManifestDocument(t, data, fixtures)
		case releaseJSONDoorUnknown, releaseJSONDoorLimit:
			return
		default:
			return
		}
	})
}

type releaseTextDoor uint8

const (
	releaseTextDoorUnknown releaseTextDoor = iota
	releaseTextDoorProjectVersion
	releaseTextDoorMainPackage
	releaseTextDoorBuildTag
	releaseTextDoorLimit
)

func (d releaseTextDoor) functionName() string {
	switch d {
	case releaseTextDoorProjectVersion:
		return "ParseProjectVersion"
	case releaseTextDoorMainPackage:
		return "ParseMainPackage"
	case releaseTextDoorBuildTag:
		return "ParseBuildTag"
	case releaseTextDoorUnknown, releaseTextDoorLimit:
		return ""
	default:
		return ""
	}
}

func FuzzReleaseExternalTextDoorInventory(f *testing.F) {
	mainPackage, err := ParseMainPackage("github.com/offGridSoft/witness/cmd/witness")
	if err != nil {
		f.Fatalf("ParseMainPackage(seed) error = %v, want nil", err)
	}
	buildTag, err := ParseBuildTag("release")
	if err != nil {
		f.Fatalf("ParseBuildTag(seed) error = %v, want nil", err)
	}
	f.Add(uint8(releaseTextDoorProjectVersion), PrimitiveVersion.String())
	f.Add(uint8(releaseTextDoorMainPackage), mainPackage.String())
	f.Add(uint8(releaseTextDoorBuildTag), buildTag.String())
	for _, hostile := range []string{"", " ", "\x00", "\xff", "-flag", "a,b"} {
		f.Add(uint8(releaseTextDoorBuildTag), hostile)
	}

	f.Fuzz(func(t *testing.T, rawDoor uint8, value string) {
		switch releaseTextDoor(rawDoor) {
		case releaseTextDoorProjectVersion:
			got, gotErr := ParseProjectVersion(value)
			if gotErr != nil {
				requireReleaseTextRefusal(t, gotErr, got.String())
				return
			}
			if got.Validate() != nil || got.String() != value || got != PrimitiveVersion {
				t.Fatalf("ParseProjectVersion(%q) = (%v, %v), want exact current compiler version",
					value, got, gotErr)
			}
		case releaseTextDoorMainPackage:
			got, gotErr := ParseMainPackage(value)
			if gotErr != nil {
				requireReleaseTextRefusal(t, gotErr, got.String())
				return
			}
			if got.Validate() != nil || got.String() != value {
				t.Fatalf("ParseMainPackage(%q) = (%v, %v), want exact validated text",
					value, got, gotErr)
			}
			roundTrip, roundTripErr := ParseMainPackage(got.String())
			if roundTripErr != nil || roundTrip != got {
				t.Fatalf("ParseMainPackage(String()) = (%v, %v), want exact fixed point",
					roundTrip, roundTripErr)
			}
		case releaseTextDoorBuildTag:
			got, gotErr := ParseBuildTag(value)
			if gotErr != nil {
				requireReleaseTextRefusal(t, gotErr, got.String())
				return
			}
			if got.Validate() != nil || got.String() != value {
				t.Fatalf("ParseBuildTag(%q) = (%v, %v), want exact validated text",
					value, got, gotErr)
			}
			roundTrip, roundTripErr := ParseBuildTag(got.String())
			if roundTripErr != nil || roundTrip != got {
				t.Fatalf("ParseBuildTag(String()) = (%v, %v), want exact fixed point",
					roundTrip, roundTripErr)
			}
		case releaseTextDoorUnknown, releaseTextDoorLimit:
			return
		default:
			return
		}
	})
}

func fuzzReleaseJSONValue[T core.ValidatedJSONMarshaler](
	t *testing.T,
	data []byte,
	seed T,
) {
	t.Helper()

	before, err := seed.MarshalJSON()
	if err != nil {
		t.Fatalf("seed MarshalJSON() error = %v, want nil", err)
	}
	candidate := seed
	decoder, ok := any(&candidate).(json.Unmarshaler)
	if !ok {
		t.Fatalf("release JSON door receiver %T lacks json.Unmarshaler", &candidate)
	}
	decodeErr := decoder.UnmarshalJSON(data)
	if decodeErr != nil {
		if !errors.Is(decodeErr, core.ErrReleaseContract) ||
			!errors.Is(decodeErr, core.ErrJSONContract) {
			t.Fatalf("release JSON door error = %v, want %v and %v",
				decodeErr, core.ErrReleaseContract, core.ErrJSONContract)
		}
		after, marshalErr := candidate.MarshalJSON()
		if marshalErr != nil || !bytes.Equal(after, before) {
			t.Fatalf("rejected release JSON door changed its receiver: marshal error %v",
				marshalErr)
		}
		return
	}
	if err := candidate.Validate(); err != nil {
		t.Fatalf("accepted release JSON door validation error = %v, want nil", err)
	}
	canonical, err := candidate.MarshalJSON()
	if err != nil {
		t.Fatalf("accepted release JSON door MarshalJSON() error = %v, want nil", err)
	}
	if len(canonical) > core.JSONDocumentMaximumBytes {
		t.Fatalf("accepted release JSON canonical extent = %d, want <= %d",
			len(canonical), core.JSONDocumentMaximumBytes)
	}
	var roundTrip T
	roundTripDecoder, ok := any(&roundTrip).(json.Unmarshaler)
	if !ok {
		t.Fatalf("release round-trip receiver %T lacks json.Unmarshaler", &roundTrip)
	}
	if err := roundTripDecoder.UnmarshalJSON(canonical); err != nil {
		t.Fatalf("canonical release JSON decode error = %v, want nil", err)
	}
	if err := roundTrip.Validate(); err != nil {
		t.Fatalf("round-trip release JSON validation error = %v, want nil", err)
	}
	second, err := roundTrip.MarshalJSON()
	if err != nil || !bytes.Equal(second, canonical) {
		t.Fatalf("release JSON door lacks a canonical fixed point: marshal error %v", err)
	}
}

func fuzzReleaseManifestDocument(
	t *testing.T,
	data []byte,
	fixtures releaseJSONDoorFixtures,
) {
	t.Helper()
	fuzzReleaseJSONValue(t, data, fixtures.manifestDocument)

	candidate := fixtures.manifestDocument
	if err := candidate.UnmarshalJSON(data); err != nil {
		return
	}
	proof, err := VerifyManifest(VerifyManifestRequest{
		Document: candidate, TrustedKeys: fixtures.release.manifestTrust,
		ExpectedOffering: core.OfferingWitness,
	})
	if err != nil {
		if !errors.Is(err, core.ErrReleaseVerification) || proof != (VerifiedManifest{}) {
			t.Fatalf("VerifyManifest(fuzz document) = (%v, %v), want typed refusal and zero proof",
				proof, err)
		}
		return
	}
	if proof.Validate() != nil || candidate != fixtures.manifestDocument {
		t.Fatalf("VerifyManifest(fuzz document) authenticated facts outside the signed seed")
	}
}

func fuzzReleaseLatestDocument(
	t *testing.T,
	data []byte,
	fixtures releaseJSONDoorFixtures,
) {
	t.Helper()
	fuzzReleaseJSONValue(t, data, fixtures.latestDocument)

	candidate := fixtures.latestDocument
	if err := candidate.UnmarshalJSON(data); err != nil {
		return
	}
	proof, err := VerifyLatest(VerifyLatestRequest{
		Document: candidate, LatestKeys: fixtures.release.latestTrust,
		ManifestKeys:     fixtures.release.manifestTrust,
		ExpectedOffering: core.OfferingWitness,
	})
	if err != nil {
		if !errors.Is(err, core.ErrReleaseVerification) || proof != (VerifiedLatest{}) {
			t.Fatalf("VerifyLatest(fuzz document) = (%v, %v), want typed refusal and zero proof",
				proof, err)
		}
		return
	}
	if proof.Validate() != nil || candidate != fixtures.latestDocument {
		t.Fatalf("VerifyLatest(fuzz document) authenticated facts outside the signed seed")
	}
}

func releaseJSONFixturesForFuzz(t testing.TB) releaseJSONDoorFixtures {
	t.Helper()

	installed := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 1)
	candidate := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 31), 2)
	cache, err := NewCachedLatest(candidate.verifiedLatest)
	if err != nil {
		t.Fatalf("NewCachedLatest() error = %v, want nil", err)
	}
	selection, err := evaluateWithInstalled(EvaluateRequest{
		InstalledManifest: installed.verified,
		Latest:            cache,
		Observation:       temporal.InstantFromNanoseconds(2_000),
	}, installed.builds[2])
	if err != nil {
		t.Fatalf("evaluateWithInstalled() error = %v, want nil", err)
	}
	available, ok := selection.Available()
	if !ok {
		t.Fatalf("Selection.Available() ok = false, want true")
	}
	summary, err := available.Summary()
	if err != nil {
		t.Fatalf("AvailableRelease.Summary() error = %v, want nil", err)
	}
	metadata := candidate.manifest.Fact.Metadata()
	metadataAsset, ok := metadata.At(0)
	if !ok {
		t.Fatalf("MetadataSet.At(0) ok = false, want true")
	}
	artifact := candidate.artifacts[0]
	filename, err := artifact.Filename()
	if err != nil {
		t.Fatalf("Artifact.Filename() error = %v, want nil", err)
	}
	dependencies, err := newBuildDependencies(
		mustModulePath(t, testMainModule), CurrentGoToolchain(),
		moduleFixtures(t, "example.com/a", "example.com/b"),
	)
	if err != nil {
		t.Fatalf("newBuildDependencies() error = %v, want nil", err)
	}
	return releaseJSONDoorFixtures{
		available: summary, publicationRole: PublicationRoleManifest,
		generation:     candidate.latest.Fact.Generation(),
		latestIdentity: candidate.latest.Fact.Identity(),
		latestFact:     candidate.latest.Fact, latestDocument: candidate.latest,
		metadataKind: MetadataKindDependencies, metadataAsset: metadataAsset,
		metadataSet: metadata, buildProvenance: candidate.manifest.Fact.Provenance(),
		revision: Revision2026V1, buildDependencies: dependencies,
		binaryFilename: filename, artifactIdentity: artifact.Identity(),
		artifactIntegrity: artifact.Integrity(), artifact: artifact,
		artifactSet:            candidate.artifactSet,
		manifestIdentity:       candidate.manifest.Fact.Identity(),
		manifestDocumentDigest: candidate.verified.DocumentDigest(),
		manifestFact:           candidate.manifest.Fact, manifestDocument: candidate.manifest,
		release: candidate,
	}
}

func releaseJSONSeedsForFuzz(
	t testing.TB,
	fixtures releaseJSONDoorFixtures,
) []releaseJSONDoorSeed {
	t.Helper()

	return []releaseJSONDoorSeed{
		releaseJSONSeedForFuzz(t, releaseJSONDoorAvailableSummary, fixtures.available),
		releaseJSONSeedForFuzz(t, releaseJSONDoorPublicationRole, fixtures.publicationRole),
		releaseJSONSeedForFuzz(t, releaseJSONDoorGeneration, fixtures.generation),
		releaseJSONSeedForFuzz(t, releaseJSONDoorLatestIdentity, fixtures.latestIdentity),
		releaseJSONSeedForFuzz(t, releaseJSONDoorLatestFact, fixtures.latestFact),
		releaseJSONSeedForFuzz(t, releaseJSONDoorLatestDocument, fixtures.latestDocument),
		releaseJSONSeedForFuzz(t, releaseJSONDoorMetadataKind, fixtures.metadataKind),
		releaseJSONSeedForFuzz(t, releaseJSONDoorMetadataAsset, fixtures.metadataAsset),
		releaseJSONSeedForFuzz(t, releaseJSONDoorMetadataSet, fixtures.metadataSet),
		releaseJSONSeedForFuzz(t, releaseJSONDoorBuildProvenance, fixtures.buildProvenance),
		releaseJSONSeedForFuzz(t, releaseJSONDoorRevision, fixtures.revision),
		releaseJSONSeedForFuzz(t, releaseJSONDoorBuildDependencies, fixtures.buildDependencies),
		releaseJSONSeedForFuzz(t, releaseJSONDoorBinaryFilename, fixtures.binaryFilename),
		releaseJSONSeedForFuzz(t, releaseJSONDoorArtifactIdentity, fixtures.artifactIdentity),
		releaseJSONSeedForFuzz(t, releaseJSONDoorArtifactIntegrity, fixtures.artifactIntegrity),
		releaseJSONSeedForFuzz(t, releaseJSONDoorArtifact, fixtures.artifact),
		releaseJSONSeedForFuzz(t, releaseJSONDoorArtifactSet, fixtures.artifactSet),
		releaseJSONSeedForFuzz(t, releaseJSONDoorManifestIdentity, fixtures.manifestIdentity),
		releaseJSONSeedForFuzz(t, releaseJSONDoorManifestDocumentDigest, fixtures.manifestDocumentDigest),
		releaseJSONSeedForFuzz(t, releaseJSONDoorManifestFact, fixtures.manifestFact),
		releaseJSONSeedForFuzz(t, releaseJSONDoorManifestDocument, fixtures.manifestDocument),
	}
}

func releaseJSONSeedForFuzz(
	t testing.TB,
	door releaseJSONDoor,
	value core.ValidatedJSONMarshaler,
) releaseJSONDoorSeed {
	t.Helper()

	document, err := value.MarshalJSON()
	if err != nil {
		t.Fatalf("release fuzz seed MarshalJSON(%d) error = %v, want nil", door, err)
	}
	return releaseJSONDoorSeed{door: door, document: document}
}

func requireReleaseTextRefusal(t *testing.T, got error, projection string) {
	t.Helper()

	if !errors.Is(got, core.ErrReleaseContract) {
		t.Fatalf("release text door error = %v, want %v", got, core.ErrReleaseContract)
	}
	if projection != "" {
		t.Fatalf("rejected release text door projection = %q, want sealed empty text", projection)
	}
}
