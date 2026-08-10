package release

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type (
	protocolFact[T any]      struct{}
	internalFlow[T any]      struct{}
	capabilityWrapper[T any] struct{}
	failureDetail[T any]     struct{}
)

type releaseContractInventory struct {
	embeddedBuildIdentityText         internalFlow[embeddedBuildIdentityText]
	MainPackage                       protocolFact[MainPackage]
	LinkerAssignment                  protocolFact[LinkerAssignment]
	LinkerAssignments                 protocolFact[LinkerAssignments]
	BuildTag                          protocolFact[BuildTag]
	BuildTags                         protocolFact[BuildTags]
	BuildPlanRequest                  protocolFact[BuildPlanRequest]
	BuildCommand                      capabilityWrapper[BuildCommand]
	BuildPlan                         capabilityWrapper[BuildPlan]
	BuildProcessRequest               protocolFact[BuildProcessRequest]
	BuildToolVerificationRequest      protocolFact[BuildToolVerificationRequest]
	VerifiedBuildTools                capabilityWrapper[VerifiedBuildTools]
	RepositoryVerificationRequest     protocolFact[RepositoryVerificationRequest]
	VerifiedRepository                capabilityWrapper[VerifiedRepository]
	RepositoryCommitMismatchError     failureDetail[RepositoryCommitMismatchError]
	RepositoryDirtyError              failureDetail[RepositoryDirtyError]
	repositoryGitRequest              internalFlow[repositoryGitRequest]
	repositoryStatusWriter            internalFlow[repositoryStatusWriter]
	repositoryIndexWriter             internalFlow[repositoryIndexWriter]
	GoModulePath                      protocolFact[GoModulePath]
	GoModuleVersion                   protocolFact[GoModuleVersion]
	GoModuleSum                       protocolFact[GoModuleSum]
	BuildDependency                   protocolFact[BuildDependency]
	BuildDependencies                 protocolFact[BuildDependencies]
	buildDependencyStorage            internalFlow[buildDependencyStorage]
	buildDependencyWire               internalFlow[buildDependencyWire]
	buildDependenciesWire             internalFlow[buildDependenciesWire]
	BuildDependencyObservationRequest protocolFact[BuildDependencyObservationRequest]
	goListModuleWire                  internalFlow[goListModuleWire]
	goListErrorWire                   failureDetail[goListErrorWire]
	goListPackageWire                 internalFlow[goListPackageWire]
	dependencyObservation             internalFlow[dependencyObservation]
	dependencyProcessOutcome          internalFlow[dependencyProcessOutcome]
	ArtifactInspectionRequest         protocolFact[ArtifactInspectionRequest]
	artifactByteInspection            internalFlow[artifactByteInspection]
	artifactPatternFinder             internalFlow[artifactPatternFinder]
	AdvanceLatestRequest              protocolFact[AdvanceLatestRequest]
	LatestAdvance                     capabilityWrapper[LatestAdvance]
	ArtifactIdentity                  protocolFact[ArtifactIdentity]
	BinaryFilename                    protocolFact[BinaryFilename]
	ArtifactIntegrity                 protocolFact[ArtifactIntegrity]
	artifactIntegrityWire             internalFlow[artifactIntegrityWire]
	ArtifactRequest                   protocolFact[ArtifactRequest]
	Artifact                          protocolFact[Artifact]
	artifactWire                      internalFlow[artifactWire]
	TargetSet                         protocolFact[TargetSet]
	ArtifactSetRequest                protocolFact[ArtifactSetRequest]
	ArtifactSet                       protocolFact[ArtifactSet]
	MetadataAssetRequest              protocolFact[MetadataAssetRequest]
	MetadataInspectionRequest         protocolFact[MetadataInspectionRequest]
	MetadataAsset                     protocolFact[MetadataAsset]
	metadataAssetWire                 internalFlow[metadataAssetWire]
	MetadataSetRequest                protocolFact[MetadataSetRequest]
	MetadataSet                       protocolFact[MetadataSet]
	BuildProvenanceRequest            protocolFact[BuildProvenanceRequest]
	BuildProvenance                   protocolFact[BuildProvenance]
	linkerAssignmentWire              internalFlow[linkerAssignmentWire]
	buildProvenanceWire               internalFlow[buildProvenanceWire]
	AssessLatestRequest               protocolFact[AssessLatestRequest]
	LatestAssessment                  capabilityWrapper[LatestAssessment]
	Generation                        protocolFact[Generation]
	LatestIdentity                    protocolFact[LatestIdentity]
	LatestFact                        protocolFact[LatestFact]
	latestFactWire                    internalFlow[latestFactWire]
	LatestDocument                    protocolFact[LatestDocument]
	IssueLatestRequest                protocolFact[IssueLatestRequest]
	VerifyLatestRequest               protocolFact[VerifyLatestRequest]
	VerifiedLatest                    capabilityWrapper[VerifiedLatest]
	ManifestIdentity                  protocolFact[ManifestIdentity]
	ManifestDocumentDigest            protocolFact[ManifestDocumentDigest]
	ManifestFactRequest               protocolFact[ManifestFactRequest]
	ManifestFact                      protocolFact[ManifestFact]
	manifestFactWire                  internalFlow[manifestFactWire]
	manifestIdentityWire              internalFlow[manifestIdentityWire]
	ManifestDocument                  protocolFact[ManifestDocument]
	IssueManifestRequest              protocolFact[IssueManifestRequest]
	VerifyManifestRequest             protocolFact[VerifyManifestRequest]
	VerifiedManifest                  capabilityWrapper[VerifiedManifest]
	CachedLatest                      capabilityWrapper[CachedLatest]
	EvaluateRequest                   protocolFact[EvaluateRequest]
	CurrentRelease                    capabilityWrapper[CurrentRelease]
	CurrentSummary                    protocolFact[CurrentSummary]
	AvailableRelease                  capabilityWrapper[AvailableRelease]
	AvailableSummary                  protocolFact[AvailableSummary]
	RefreshDirective                  protocolFact[RefreshDirective]
	ReassessDirective                 protocolFact[ReassessDirective]
	Selection                         capabilityWrapper[Selection]
	PreparedRelease                   capabilityWrapper[PreparedRelease]
	Preparation                       capabilityWrapper[Preparation]
	OfferingMismatchError             failureDetail[OfferingMismatchError]
}

var (
	_ releaseContractInventory
	_ = releaseContractInventory{}.embeddedBuildIdentityText
	_ = releaseContractInventory{}.artifactIntegrityWire
	_ = releaseContractInventory{}.artifactWire
	_ = releaseContractInventory{}.latestFactWire
	_ = releaseContractInventory{}.manifestFactWire
	_ = releaseContractInventory{}.manifestIdentityWire
	_ = releaseContractInventory{}.artifactByteInspection
	_ = releaseContractInventory{}.artifactPatternFinder
	_ = releaseContractInventory{}.metadataAssetWire
	_ = releaseContractInventory{}.linkerAssignmentWire
	_ = releaseContractInventory{}.buildProvenanceWire
	_ = releaseContractInventory{}.goListModuleWire
	_ = releaseContractInventory{}.goListErrorWire
	_ = releaseContractInventory{}.goListPackageWire
	_ = releaseContractInventory{}.dependencyObservation
	_ = releaseContractInventory{}.dependencyProcessOutcome
	_ = releaseContractInventory{}.buildDependencyStorage
	_ = releaseContractInventory{}.buildDependencyWire
	_ = releaseContractInventory{}.buildDependenciesWire
	_ = releaseContractInventory{}.repositoryGitRequest
	_ = releaseContractInventory{}.repositoryStatusWriter
	_ = releaseContractInventory{}.repositoryIndexWriter
)

func TestProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	got, err := productionStructNames()
	if err != nil {
		t.Fatalf("productionStructNames() error = %v, want nil", err)
	}
	want, err := inventoryStructNames()
	if err != nil {
		t.Fatalf("inventoryStructNames() error = %v, want nil", err)
	}
	for _, name := range got {
		if !slices.Contains(want, name) {
			t.Errorf("production struct %q has no compiler-visible data-flow role", name)
		}
	}
	for _, name := range want {
		if !slices.Contains(got, name) {
			t.Errorf("classified struct %q does not exist in production", name)
		}
	}
}

func TestPublicOperationsAreExactReleaseIntent(t *testing.T) {
	t.Parallel()

	got, err := exportedFunctionNames()
	if err != nil {
		t.Fatalf("exportedFunctionNames() error = %v, want nil", err)
	}
	want := []string{
		"AdvanceLatest",
		"AssessLatest",
		"CurrentGoToolchain",
		"EmbeddedBuildIdentity",
		"Evaluate",
		"InspectBuiltArtifact",
		"InspectMetadataAsset",
		"IssueLatest",
		"IssueManifest",
		"MissingCachedLatest",
		"NewArtifact",
		"NewArtifactSet",
		"NewBuildProvenance",
		"NewBuildTags",
		"NewCachedLatest",
		"NewGeneration",
		"NewLinkerAssignment",
		"NewLinkerAssignments",
		"NewManifestFact",
		"NewMetadataAsset",
		"NewMetadataSet",
		"ObserveBuildDependencies",
		"ParseBuildTag",
		"ParseMainPackage",
		"ParseProjectVersion",
		"PrepareBuildPlan",
		"PrepareBuildProcess",
		"PublicationRoleAt",
		"Targets",
		"VerifyBuildTools",
		"VerifyLatest",
		"VerifyManifest",
		"VerifyRepository",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("exported top-level functions = %v, want %v", got, want)
	}
}

func TestEvaluateObtainsInstalledIdentityOnlyFromReleaseEmbedding(t *testing.T) {
	t.Parallel()

	set := token.NewFileSet()
	file, err := parser.ParseFile(set, "selection.go", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parser.ParseFile(selection.go) error = %v", err)
	}
	var requestFields []string
	embeddedCall := false
	for _, declaration := range file.Decls {
		switch node := declaration.(type) {
		case *ast.GenDecl:
			for _, raw := range node.Specs {
				spec, ok := raw.(*ast.TypeSpec)
				if !ok || spec.Name.Name != "EvaluateRequest" {
					continue
				}
				structure := spec.Type.(*ast.StructType)
				for _, field := range structure.Fields.List {
					for _, name := range field.Names {
						requestFields = append(requestFields, name.Name)
					}
				}
			}
		case *ast.FuncDecl:
			if node.Name.Name != "Evaluate" {
				continue
			}
			ast.Inspect(node.Body, func(raw ast.Node) bool {
				call, ok := raw.(*ast.CallExpr)
				if !ok {
					return true
				}
				function, ok := call.Fun.(*ast.Ident)
				if ok && function.Name == "EmbeddedBuildIdentity" {
					embeddedCall = true
				}
				return true
			})
		}
	}
	wantFields := []string{"InstalledManifest", "Latest", "Observation"}
	if !slices.Equal(requestFields, wantFields) {
		t.Fatalf("EvaluateRequest fields = %v, want %v", requestFields, wantFields)
	}
	if !embeddedCall {
		t.Fatalf("Evaluate does not call EmbeddedBuildIdentity")
	}
}

func productionStructNames() ([]string, error) {
	files, err := productionFiles()
	if err != nil {
		return nil, err
	}
	var names []string
	set := token.NewFileSet()
	for _, name := range files {
		file, err := parser.ParseFile(set, name, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		for _, declaration := range file.Decls {
			generic, ok := declaration.(*ast.GenDecl)
			if !ok || generic.Tok != token.TYPE {
				continue
			}
			for _, raw := range generic.Specs {
				spec := raw.(*ast.TypeSpec)
				if _, ok := spec.Type.(*ast.StructType); ok {
					names = append(names, spec.Name.Name)
				}
			}
		}
	}
	slices.Sort(names)
	return names, nil
}

func inventoryStructNames() ([]string, error) {
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, "internal_contract_test.go", nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	for _, declaration := range file.Decls {
		generic, ok := declaration.(*ast.GenDecl)
		if !ok || generic.Tok != token.TYPE {
			continue
		}
		for _, raw := range generic.Specs {
			spec := raw.(*ast.TypeSpec)
			if spec.Name.Name != "releaseContractInventory" {
				continue
			}
			structure := spec.Type.(*ast.StructType)
			var names []string
			for _, field := range structure.Fields.List {
				for _, name := range field.Names {
					names = append(names, name.Name)
				}
			}
			slices.Sort(names)
			return names, nil
		}
	}
	return nil, core.ErrReleaseContract
}

func exportedFunctionNames() ([]string, error) {
	files, err := productionFiles()
	if err != nil {
		return nil, err
	}
	set := token.NewFileSet()
	var names []string
	for _, name := range files {
		file, err := parser.ParseFile(set, name, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok && function.Recv == nil && function.Name.IsExported() {
				names = append(names, function.Name.Name)
			}
		}
	}
	slices.Sort(names)
	return names, nil
}

func productionFiles() ([]string, error) {
	entries, err := os.ReadDir(".")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") &&
			!strings.HasSuffix(entry.Name(), "_test.go") {
			names = append(names, entry.Name())
		}
	}
	slices.Sort(names)
	return names, nil
}
