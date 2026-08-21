package release

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestInspectMetadataAssetProvesExactStreamingBytes(t *testing.T) {
	t.Parallel()

	payload := bytes.Repeat([]byte("documentation-block-"), 8192)
	extent := mustByteCount(t, uint64(len(payload)))
	asset, err := InspectMetadataAsset(MetadataInspectionRequest{
		Source: bytes.NewReader(payload), Extent: extent, Kind: MetadataKindDocumentation,
	})
	if err != nil {
		t.Fatalf("InspectMetadataAsset() error = %v", err)
	}
	digest := sha256.Sum256(payload)
	if asset.Integrity().Extent() != extent ||
		asset.Integrity().SHA256() != core.NewSHA256Digest(digest) {
		t.Fatalf("InspectMetadataAsset() integrity differs from exact source bytes")
	}
	for _, declared := range []uint64{uint64(len(payload) - 1), uint64(len(payload) + 1)} {
		_, gotErr := InspectMetadataAsset(MetadataInspectionRequest{
			Source: bytes.NewReader(payload), Extent: mustByteCount(t, declared),
			Kind: MetadataKindDocumentation,
		})
		if !errors.Is(gotErr, core.ErrReleaseManifest) {
			t.Fatalf("InspectMetadataAsset(extent %d) error = %v, want %v", declared, gotErr, core.ErrReleaseManifest)
		}
	}
}

func TestMetadataSetOwnsExactCustomerDocumentRoles(t *testing.T) {
	t.Parallel()

	set := fixtureMetadataSet(t)
	wantSuffixes := [...]string{
		metadataDependenciesSuffix, metadataDocumentationSuffix, metadataReleaseNotesSuffix,
	}
	wantMedia := [...]string{
		metadataJSONMediaType, metadataZIPMediaType, "text/markdown; charset=utf-8",
	}
	for index := range MetadataAssetCount {
		asset, ok := set.At(index)
		if !ok {
			t.Fatalf("MetadataSet.At(%d) ok = false", index)
		}
		if asset.Kind() != MetadataKind(index+1) {
			t.Fatalf("MetadataSet.At(%d).Kind() = %v, want %v", index, asset.Kind(), MetadataKind(index+1))
		}
		mediaType, err := asset.ContentType()
		if err != nil || mediaType.String() != wantMedia[index] {
			t.Fatalf("MetadataSet.At(%d).ContentType() = (%q, %v), want (%q, nil)", index, mediaType, err, wantMedia[index])
		}
		filename, err := asset.Filename(core.OfferingPeachfuzz, core.NewReleaseVersion(2026, 0, 11))
		if err != nil || filename.String() != "peachfuzz-2026.0.11-"+wantSuffixes[index] {
			t.Fatalf("MetadataSet.At(%d).Filename() = (%q, %v)", index, filename, err)
		}
	}
	encoded, err := json.Marshal(set)
	if err != nil {
		t.Fatalf("json.Marshal(MetadataSet) error = %v", err)
	}
	var decoded MetadataSet
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(MetadataSet) error = %v", err)
	}
	if decoded != set {
		t.Fatalf("MetadataSet JSON round trip differs")
	}
}

func TestMetadataSetRejectsRoleAndMediaTypeSubstitution(t *testing.T) {
	t.Parallel()

	valid := fixtureMetadataSet(t)
	assets := valid.assets
	assets[0], assets[1] = assets[1], assets[0]
	if _, err := NewMetadataSet(MetadataSetRequest{Assets: assets}); !errors.Is(err, core.ErrReleaseManifest) {
		t.Fatalf("NewMetadataSet(swapped roles) error = %v, want %v", err, core.ErrReleaseManifest)
	}

	asset := valid.assets[0]
	encoded, err := json.Marshal(asset)
	if err != nil {
		t.Fatalf("json.Marshal(MetadataAsset) error = %v", err)
	}
	tampered := strings.Replace(string(encoded), metadataJSONMediaType, metadataZIPMediaType, 1)
	var decoded MetadataAsset
	if err := json.Unmarshal([]byte(tampered), &decoded); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("json.Unmarshal(media substitution) error = %v, want %v", err, core.ErrJSONContract)
	}
}

func TestBuildProvenanceCanonicalJSONRoundTripPreservesExactGoReproductionFacts(t *testing.T) {
	t.Parallel()

	provenance := fixtureBuildProvenance(t)
	encoded, err := json.Marshal(provenance)
	if err != nil {
		t.Fatalf("json.Marshal(BuildProvenance) error = %v, want nil", err)
	}
	var decoded BuildProvenance
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(BuildProvenance) error = %v, want nil", err)
	}
	if decoded != provenance {
		t.Fatalf("BuildProvenance JSON round trip = %v, want %v", decoded, provenance)
	}
	canonical, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("json.Marshal(round-trip BuildProvenance) error = %v, want nil", err)
	}
	if !bytes.Equal(canonical, encoded) {
		t.Fatalf("BuildProvenance second canonical JSON = %q, want %q", canonical, encoded)
	}
}

func TestBuildProvenanceRejectsUnverifiedTools(t *testing.T) {
	t.Parallel()

	plan := provenanceBuildPlan(t)
	tools := provenanceVerifiedTools(t)
	request := BuildProvenanceRequest{Plan: plan, Tools: tools}
	if _, err := NewBuildProvenance(request); err != nil {
		t.Fatalf("NewBuildProvenance(valid) error = %v", err)
	}
	request.Tools = VerifiedBuildTools{}
	if _, err := NewBuildProvenance(request); !errors.Is(err, core.ErrReleaseManifest) {
		t.Fatalf("NewBuildProvenance(unverified tools) error = %v, want %v", err, core.ErrReleaseManifest)
	}
}

func provenanceBuildPlan(t *testing.T) BuildPlan {
	t.Helper()
	mainPackage, _ := ParseMainPackage("github.com/offGridSoft/bug/cmd/bug")
	output, _ := core.ParseRelativePath("dist")
	commit, _ := core.ParseBuildCommit("b5c32d95d212b0a1a8cef4126e4d11ff288079ef")
	plan, err := PrepareBuildPlan(BuildPlanRequest{
		Offering: core.OfferingBug, Version: core.NewReleaseVersion(2026, 0, 11), Commit: commit,
		MainPackage: mainPackage, OutputDirectory: output,
		GoToolchain: CurrentGoToolchain(), ModuleMode: BuildModuleVendor,
		LinkerAssignments: emptyLinkerAssignmentsForTest(),
	})
	if err != nil {
		t.Fatalf("PrepareBuildPlan() error = %v", err)
	}
	return plan
}

func provenanceVerifiedTools(t *testing.T) VerifiedBuildTools {
	t.Helper()
	goPath, _ := core.ParseAbsolutePath("/usr/local/go/bin/go")
	goSum := sha256.Sum256([]byte("go"))
	value := VerifiedBuildTools{
		goExecutable:       goPath,
		goExecutableDigest: core.NewSHA256Digest(goSum),
		hostPlatform: core.Platform{
			OperatingSystem: core.OperatingSystemDarwin, Architecture: core.CPUArchitectureARM64,
		},
		goToolchain: CurrentGoToolchain(), valid: true,
	}
	if err := value.Validate(); err != nil {
		t.Fatalf("VerifiedBuildTools.Validate() error = %v", err)
	}
	return value
}
