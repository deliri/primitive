package release

import (
	"embed"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

//go:embed *.go
var releaseContractSources embed.FS

type (
	protocolFact[T any]      struct{}
	wireProtocol[T any]      struct{}
	internalFlow[T any]      struct{}
	capabilityWrapper[T any] struct{}
	failureDetail[T any]     struct{}
)

type releaseContractInventory struct {
	latestIdentityWire       wireProtocol[latestIdentityWire]
	artifactIdentityWire     wireProtocol[artifactIdentityWire]
	latestDocumentEncoding   wireProtocol[latestDocumentEncoding]
	latestDocumentWire       wireProtocol[latestDocumentWire]
	manifestDocumentEncoding wireProtocol[manifestDocumentEncoding]
	manifestDocumentWire     wireProtocol[manifestDocumentWire]
	availableSummaryWire     wireProtocol[availableSummaryWire]

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
	buildDependencyWire               wireProtocol[buildDependencyWire]
	buildDependenciesWire             wireProtocol[buildDependenciesWire]
	BuildDependencyObservationRequest protocolFact[BuildDependencyObservationRequest]
	goListModuleWire                  wireProtocol[goListModuleWire]
	goListErrorWire                   wireProtocol[goListErrorWire]
	goListPackageWire                 wireProtocol[goListPackageWire]
	dependencyObservation             internalFlow[dependencyObservation]
	dependencyProcessOutcome          internalFlow[dependencyProcessOutcome]
	ArtifactInspectionRequest         protocolFact[ArtifactInspectionRequest]
	openedArtifactInspection          internalFlow[openedArtifactInspection]
	artifactByteInspection            internalFlow[artifactByteInspection]
	artifactPatternFinder             internalFlow[artifactPatternFinder]
	AdvanceLatestRequest              protocolFact[AdvanceLatestRequest]
	LatestAdvance                     capabilityWrapper[LatestAdvance]
	ArtifactIdentity                  protocolFact[ArtifactIdentity]
	BinaryFilename                    protocolFact[BinaryFilename]
	ArtifactIntegrity                 protocolFact[ArtifactIntegrity]
	artifactIntegrityWire             wireProtocol[artifactIntegrityWire]
	ArtifactRequest                   protocolFact[ArtifactRequest]
	Artifact                          protocolFact[Artifact]
	artifactWire                      wireProtocol[artifactWire]
	TargetSet                         protocolFact[TargetSet]
	ArtifactSetRequest                protocolFact[ArtifactSetRequest]
	ArtifactSet                       protocolFact[ArtifactSet]
	MetadataAssetRequest              protocolFact[MetadataAssetRequest]
	MetadataInspectionRequest         protocolFact[MetadataInspectionRequest]
	MetadataAsset                     protocolFact[MetadataAsset]
	metadataAssetWire                 wireProtocol[metadataAssetWire]
	MetadataSetRequest                protocolFact[MetadataSetRequest]
	MetadataSet                       protocolFact[MetadataSet]
	BuildProvenanceRequest            protocolFact[BuildProvenanceRequest]
	BuildProvenance                   protocolFact[BuildProvenance]
	linkerAssignmentWire              wireProtocol[linkerAssignmentWire]
	buildProvenanceWire               wireProtocol[buildProvenanceWire]
	AssessLatestRequest               protocolFact[AssessLatestRequest]
	LatestTimeEvidence                protocolFact[LatestTimeEvidence]
	LatestAssessment                  capabilityWrapper[LatestAssessment]
	Generation                        protocolFact[Generation]
	LatestIdentity                    protocolFact[LatestIdentity]
	LatestFact                        protocolFact[LatestFact]
	latestFactWire                    wireProtocol[latestFactWire]
	LatestDocument                    protocolFact[LatestDocument]
	IssueLatestRequest                protocolFact[IssueLatestRequest]
	VerifyLatestRequest               protocolFact[VerifyLatestRequest]
	VerifiedLatest                    capabilityWrapper[VerifiedLatest]
	ManifestIdentity                  protocolFact[ManifestIdentity]
	ManifestDocumentDigest            protocolFact[ManifestDocumentDigest]
	ManifestFactRequest               protocolFact[ManifestFactRequest]
	ManifestFact                      protocolFact[ManifestFact]
	manifestFactWire                  wireProtocol[manifestFactWire]
	manifestIdentityWire              wireProtocol[manifestIdentityWire]
	ManifestDocument                  protocolFact[ManifestDocument]
	IssueManifestRequest              protocolFact[IssueManifestRequest]
	VerifyManifestRequest             protocolFact[VerifyManifestRequest]
	VerifiedManifest                  capabilityWrapper[VerifiedManifest]
	CachedLatest                      capabilityWrapper[CachedLatest]
	EvaluateRequest                   protocolFact[EvaluateRequest]
	EvaluateInstalledRequest          protocolFact[EvaluateInstalledRequest]
	CurrentRelease                    capabilityWrapper[CurrentRelease]
	CurrentSummary                    protocolFact[CurrentSummary]
	AvailableRelease                  capabilityWrapper[AvailableRelease]
	AvailableSummary                  protocolFact[AvailableSummary]
	RefreshDirective                  protocolFact[RefreshDirective]
	ReassessDirective                 protocolFact[ReassessDirective]
	Selection                         capabilityWrapper[Selection]
	selectionComparison               internalFlow[selectionComparison]
	currentSelection                  internalFlow[currentSelection]
	PreparedRelease                   capabilityWrapper[PreparedRelease]
	Preparation                       capabilityWrapper[Preparation]
	OfferingMismatchError             failureDetail[OfferingMismatchError]
	MaterialRequest                   protocolFact[MaterialRequest]
	materialRequestWire               wireProtocol[materialRequestWire]
	MaterialRequestInput              protocolFact[MaterialRequestInput]
	ReleaseSigningSeed                protocolFact[ReleaseSigningSeed]
	MaterialResponse                  protocolFact[MaterialResponse]
	materialResponseWire              wireProtocol[materialResponseWire]
	Material                          capabilityWrapper[Material]
}

var (
	_ releaseContractInventory
	_ = releaseContractInventory{}.latestIdentityWire
	_ = releaseContractInventory{}.artifactIdentityWire
	_ = releaseContractInventory{}.latestDocumentEncoding
	_ = releaseContractInventory{}.latestDocumentWire
	_ = releaseContractInventory{}.manifestDocumentEncoding
	_ = releaseContractInventory{}.manifestDocumentWire
	_ = releaseContractInventory{}.availableSummaryWire

	_ = releaseContractInventory{}.embeddedBuildIdentityText
	_ = releaseContractInventory{}.artifactIntegrityWire
	_ = releaseContractInventory{}.artifactWire
	_ = releaseContractInventory{}.latestFactWire
	_ = releaseContractInventory{}.manifestFactWire
	_ = releaseContractInventory{}.manifestIdentityWire
	_ = releaseContractInventory{}.artifactByteInspection
	_ = releaseContractInventory{}.openedArtifactInspection
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
	_ = releaseContractInventory{}.materialRequestWire
	_ = releaseContractInventory{}.materialResponseWire
	_ = releaseContractInventory{}.selectionComparison
	_ = releaseContractInventory{}.currentSelection
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
		"EvaluateInstalled",
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
		"NewMaterialRequest",
		"NewMetadataAsset",
		"NewMetadataSet",
		"NewReleaseSigningSeed",
		"ObserveBuildDependencies",
		"ParseBuildTag",
		"ParseMainPackage",
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

func TestExternalIngressFuzzInventoryMatchesEveryPublicDecoder(t *testing.T) {
	t.Parallel()

	gotJSON, err := exportedJSONReceiverNames()
	if err != nil {
		t.Fatalf("exportedJSONReceiverNames() error = %v, want nil", err)
	}
	var wantJSON []string
	for door := range releaseJSONDoorLimit {
		if door < releaseJSONDoorUnknown+1 {
			continue
		}
		name := door.receiverName()
		if name == "" {
			t.Fatalf("release JSON fuzz door %d has no compiler-visible receiver", door)
		}
		wantJSON = append(wantJSON, name)
	}
	slices.Sort(wantJSON)
	if !slices.Equal(gotJSON, wantJSON) {
		t.Fatalf("public JSON receivers = %v, fuzz inventory = %v", gotJSON, wantJSON)
	}

	functions, err := exportedFunctionNames()
	if err != nil {
		t.Fatalf("exportedFunctionNames error = %v, want nil", err)
	}
	var gotText []string
	for _, name := range functions {
		if strings.HasPrefix(name, "Parse") {
			gotText = append(gotText, name)
		}
	}
	var wantText []string
	for door := range releaseTextDoorLimit {
		if door < releaseTextDoorUnknown+1 {
			continue
		}
		name := door.functionName()
		if name == "" {
			t.Fatalf("release text fuzz door %d has no compiler-visible function", door)
		}
		wantText = append(wantText, name)
	}
	slices.Sort(wantText)
	if !slices.Equal(gotText, wantText) {
		t.Fatalf("public text parsers = %v, fuzz inventory = %v", gotText, wantText)
	}
}

// This is a syntax contract: these two public request shapes and their direct
// embedded-identity calls stay explicit. Behavioral selection tests separately
// prove the resulting identities and refusals.
func TestSelectionIdentitySourceContracts(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, request, function string
		fields                  []string
		embedded                bool
	}{
		{name: "running identity is acquired by Evaluate", request: "EvaluateRequest", function: "Evaluate", fields: []string{"Time", "InstalledManifest", "Latest"}, embedded: true},
		{name: "supplied identity stays supplied to EvaluateInstalled", request: "EvaluateInstalledRequest", function: "EvaluateInstalled", fields: []string{"Installed", "Evaluate"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, err := parseReleaseContractFile(token.NewFileSet(), "selection.go")
			if err != nil {
				t.Fatalf("parse selection source error = %v, want nil", err)
			}
			fields, embedded, found := releaseSelectionShape(file, tc.request, tc.function)
			if !found || !slices.Equal(fields, tc.fields) || embedded != tc.embedded {
				t.Fatalf("%s shape = (%v, embedded %t, found %t), want (%v, embedded %t, found true)", tc.function, fields, embedded, found, tc.fields, tc.embedded)
			}
		})
	}
}

func releaseSelectionShape(file *ast.File, request, function string) ([]string, bool, bool) {
	var fields []string
	var foundRequest, foundFunction, embedded bool
	for _, declaration := range file.Decls {
		switch node := declaration.(type) {
		case *ast.GenDecl:
			for _, raw := range node.Specs {
				spec, ok := raw.(*ast.TypeSpec)
				if !ok || spec.Name.Name != request {
					continue
				}
				structure, ok := ast.Unparen(spec.Type).(*ast.StructType)
				if !ok {
					continue
				}
				foundRequest = true
				for _, field := range structure.Fields.List {
					if len(field.Names) == 0 {
						fields = append(fields, "")
					}
					for _, name := range field.Names {
						fields = append(fields, name.Name)
					}
				}
			}
		case *ast.FuncDecl:
			if node.Recv != nil || node.Name.Name != function || node.Body == nil {
				continue
			}
			foundFunction = true
			ast.Inspect(node.Body, func(raw ast.Node) bool {
				call, ok := raw.(*ast.CallExpr)
				if !ok {
					return true
				}
				name, ok := ast.Unparen(call.Fun).(*ast.Ident)
				if ok && name.Name == "EmbeddedBuildIdentity" {
					embedded = true
				}
				return true
			})
		}
	}
	return fields, embedded, foundRequest && foundFunction
}

func parseReleaseContractFile(set *token.FileSet, name string) (*ast.File, error) {
	data, err := releaseContractSources.ReadFile(name)
	if err != nil {
		return nil, err
	}
	return parser.ParseFile(set, name, data, parser.SkipObjectResolution)
}

func productionStructNames() ([]string, error) {
	names, err := productionFiles()
	if err != nil {
		return nil, err
	}
	set := token.NewFileSet()
	var files []*ast.File
	for _, name := range names {
		file, err := parseReleaseContractFile(set, name)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return releaseStructNames(set, files), nil
}

// Inventory every named struct, including defined projections. Local and
// anonymous carriers retain source coordinates and therefore cannot match a
// package-level compiler inventory entry. The resolver only follows this
// package's finite declarations; it does not model imported packages.
func releaseStructNames(set *token.FileSet, files []*ast.File) []string {
	declarations := make(map[string]ast.Expr)
	topLevel := make(map[*ast.TypeSpec]bool)
	namedLiterals := make(map[*ast.StructType]bool)
	for _, file := range files {
		for _, declaration := range file.Decls {
			generic, ok := declaration.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, raw := range generic.Specs {
				spec, ok := raw.(*ast.TypeSpec)
				if !ok {
					continue
				}
				declarations[spec.Name.Name] = spec.Type
				topLevel[spec] = true
			}
		}
	}
	var names []string
	for name, expression := range declarations {
		if releaseUnderlyingStruct(expression, declarations, len(declarations)) != nil {
			names = append(names, name)
		}
	}
	for _, file := range files {
		ast.Inspect(file, func(node ast.Node) bool {
			if spec, ok := node.(*ast.TypeSpec); ok {
				if literal, ok := ast.Unparen(spec.Type).(*ast.StructType); ok {
					namedLiterals[literal] = true
				}
				if !topLevel[spec] && releaseUnderlyingStruct(spec.Type, declarations, len(declarations)) != nil {
					names = append(names, set.Position(spec.Pos()).String()+" local "+spec.Name.Name)
				}
			}
			if structure, ok := node.(*ast.StructType); ok && !namedLiterals[structure] {
				names = append(names, set.Position(structure.Pos()).String()+" anonymous struct")
			}
			return true
		})
	}
	slices.Sort(names)
	return names
}

func releaseUnderlyingStruct(expression ast.Expr, declarations map[string]ast.Expr, remaining int) *ast.StructType {
	if remaining < 0 {
		return nil
	}
	switch value := ast.Unparen(expression).(type) {
	case *ast.StructType:
		return value
	case *ast.Ident:
		return releaseUnderlyingStruct(declarations[value.Name], declarations, remaining-1)
	case *ast.IndexExpr:
		return releaseUnderlyingStruct(value.X, declarations, remaining-1)
	case *ast.IndexListExpr:
		return releaseUnderlyingStruct(value.X, declarations, remaining-1)
	default:
		return nil
	}
}

func exportedJSONReceiverNames() ([]string, error) {
	files, err := productionFiles()
	if err != nil {
		return nil, err
	}
	set := token.NewFileSet()
	var names []string
	for _, name := range files {
		file, err := parseReleaseContractFile(set, name)
		if err != nil {
			return nil, err
		}
		names = append(names, releaseJSONReceiverNames(file)...)
	}
	slices.Sort(names)
	return names, nil
}

func releaseJSONReceiverNames(file *ast.File) []string {
	var names []string
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "UnmarshalJSON" || function.Recv == nil || len(function.Recv.List) != 1 {
			continue
		}
		if name := releaseReceiverName(function.Recv.List[0].Type); ast.IsExported(name) {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

func releaseReceiverName(expression ast.Expr) string {
	switch value := ast.Unparen(expression).(type) {
	case *ast.Ident:
		return value.Name
	case *ast.StarExpr:
		return releaseReceiverName(value.X)
	case *ast.IndexExpr:
		return releaseReceiverName(value.X)
	case *ast.IndexListExpr:
		return releaseReceiverName(value.X)
	default:
		return ""
	}
}

func inventoryStructNames() ([]string, error) {
	set := token.NewFileSet()
	file, err := parseReleaseContractFile(set, "internal_contract_test.go")
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
		file, err := parseReleaseContractFile(set, name)
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
	entries, err := releaseContractSources.ReadDir(".")
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
