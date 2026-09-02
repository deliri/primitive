// Package gotoolchain_test owns compiler-checked knowledge about package gotoolchain.
package gotoolchain_test

import (
	"errors"

	primitivecore "github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/projectstandards"
)

const packageStandardAuthorshipPending = true

// PackageStandardKnowledge returns authored package meaning. It remains
// fail-closed until its owner supplies every required knowledge field.
func PackageStandardKnowledge(created projectstandards.OptionalGitOrigin, changed projectstandards.GitOrigin) (projectstandards.PackageKnowledge, error) {
	if packageStandardAuthorshipPending {
		return projectstandards.PackageKnowledge{}, primitivecore.ErrProjectStandardsContract
	}
	var authoredErr error
	authorPath := func(value string) projectstandards.SourcePath {
		result, err := projectstandards.ParseSourcePath(value)
		authoredErr = errors.Join(authoredErr, err)
		return result
	}
	authorIdentifier := func(value string) projectstandards.Identifier {
		result, err := projectstandards.NewIdentifier(value)
		authoredErr = errors.Join(authoredErr, err)
		return result
	}
	authorName := func(value string) projectstandards.Name {
		result, err := projectstandards.NewName(value)
		authoredErr = errors.Join(authoredErr, err)
		return result
	}
	authorText := func(value string) projectstandards.Text {
		result, err := projectstandards.NewText(value)
		authoredErr = errors.Join(authoredErr, err)
		return result
	}
	authorReason := func(title, detail string) projectstandards.Reason {
		return projectstandards.Reason{Title: authorName(title), Detail: authorText(detail)}
	}
	authorBoundary := func(title, detail string) projectstandards.Boundary {
		return projectstandards.Boundary{Title: authorName(title), Detail: authorText(detail)}
	}
	knowledge := projectstandards.PackageKnowledge{
		Path:        authorPath("gotoolchain"),
		AuthorTitle: authorName(""), AuthorProblem: authorText(""), AuthorPurpose: authorText(""),
		AuthorAudience: authorText(""), AuthorValue: authorText(""), AuthorSteward: authorName(""),
		AuthorSubstrate: authorName(""), AuthorRuntime: authorName(""), AuthorRemoval: authorText(""),
		Created: created, Changed: changed,
		AuthorReasons:    []projectstandards.Reason{authorReason("", "")},
		AuthorOwns:       []projectstandards.Boundary{authorBoundary("", "")},
		AuthorDoesNotOwn: []projectstandards.Boundary{authorBoundary("", "")},
		AuthorUsage: []projectstandards.Usage{{
			ID: authorIdentifier(""), Title: authorName(""), Audience: authorText(""),
			Goal: authorText(""), Outcome: authorText(""),
			Steps: []projectstandards.UsageStep{{Title: authorName(""), Detail: authorText("")}},
		}},
		AuthorFeatures: []projectstandards.Feature{{
			ID: authorIdentifier(""), Title: authorName(""), Technical: authorText(""),
			Benefit: authorText(""), ProofRequirement: authorText(""), Delivery: projectstandards.DeliveryUnknown,
		}},
		AuthorAssurance: projectstandards.Assurance{
			Policy:     projectstandards.AssuranceControl{Stage: projectstandards.AssuranceStagePolicy, Authority: projectstandards.AssuranceAuthorityProduct, Statement: authorText(""), References: []projectstandards.CodeReference{{Path: authorPath("")}}, SurfaceIDs: []projectstandards.Identifier{authorIdentifier("")}},
			Validation: projectstandards.AssuranceControl{Stage: projectstandards.AssuranceStageValidation, Authority: projectstandards.AssuranceAuthorityCore, Statement: authorText(""), References: []projectstandards.CodeReference{{Path: authorPath("")}}, SurfaceIDs: []projectstandards.Identifier{authorIdentifier("")}},
			Effects:    projectstandards.AssuranceControl{Stage: projectstandards.AssuranceStageEffects, Authority: projectstandards.AssuranceAuthorityPrimitive, Statement: authorText(""), References: []projectstandards.CodeReference{{Path: authorPath("")}}, SurfaceIDs: []projectstandards.Identifier{authorIdentifier("")}},
			Proof:      projectstandards.AssuranceControl{Stage: projectstandards.AssuranceStageProof, Authority: projectstandards.AssuranceAuthorityIndependent, Statement: authorText(""), References: []projectstandards.CodeReference{{Path: authorPath("")}}, SurfaceIDs: []projectstandards.Identifier{authorIdentifier("")}},
		},
	}
	if authoredErr != nil {
		return projectstandards.PackageKnowledge{}, authoredErr
	}
	if err := knowledge.Validate(); err != nil {
		return projectstandards.PackageKnowledge{}, err
	}
	return knowledge, nil
}

// PackageStandardCode returns Forge's regenerated, compiler-checked file facts.
func PackageStandardCode() (projectstandards.PackageFileCatalog, error) {
	var generatedErr error
	sourcePath := func(value string) projectstandards.SourcePath {
		result, err := projectstandards.ParseSourcePath(value)
		generatedErr = errors.Join(generatedErr, err)
		return result
	}
	projectModule := func(value string) *projectstandards.SourcePath {
		result := sourcePath(value)
		return &result
	}
	capability := func(value string) primitivecore.PackageIdentity {
		result, err := primitivecore.ParsePackageIdentity(value)
		generatedErr = errors.Join(generatedErr, err)
		return result
	}
	identifier := func(value string) projectstandards.Identifier {
		result, err := projectstandards.NewIdentifier(value)
		generatedErr = errors.Join(generatedErr, err)
		return result
	}
	sourceIdentifier := func(value string) *projectstandards.Identifier {
		result := identifier(value)
		return &result
	}
	catalog := projectstandards.PackageFileCatalog{
		Package: sourcePath("gotoolchain"),
		Files: []projectstandards.SourceFile{
			{
				Path: sourcePath("gotoolchain/contracts.go"), Package: sourcePath("gotoolchain"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{{Path: sourcePath("errors"), Kind: projectstandards.SourceImportKindStandardLibrary}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/core"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/gomodule"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/process"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/temporal"), Kind: projectstandards.SourceImportKindProject}, {Path: sourcePath("go/token"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("strings"), Kind: projectstandards.SourceImportKindStandardLibrary}}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0, Symbols: []projectstandards.SourceDeclaration{{Name: identifier("OutputMaximumBytes"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 16, Column: 2, Exported: true, AttestBound: false}, {Name: identifier("PackageMaximumCount"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 18, Column: 2, Exported: true, AttestBound: false}, {Name: identifier("toolchainVersionMaximumBytes"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 19, Column: 2, Exported: false, AttestBound: false}, {Name: identifier("WorkspaceMode"), Kind: projectstandards.SourceDeclarationKindType, Line: 23, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("WorkspaceModeUnknown"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 26, Column: 2, Exported: true, AttestBound: false}, {Name: identifier("WorkspaceModeAmbient"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 28, Column: 2, Exported: true, AttestBound: false}, {Name: identifier("WorkspaceModeDisabled"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 30, Column: 2, Exported: true, AttestBound: false}, {Name: identifier("Validate"), Receiver: sourceIdentifier("WorkspaceMode"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 33, Column: 24, Exported: true, AttestBound: false}, {Name: identifier("IsValid"), Receiver: sourceIdentifier("WorkspaceMode"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 41, Column: 24, Exported: true, AttestBound: false}, {Name: identifier("OffWireEnum"), Receiver: sourceIdentifier("WorkspaceMode"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 44, Column: 22, Exported: true, AttestBound: false}, {Name: identifier("String"), Receiver: sourceIdentifier("WorkspaceMode"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 47, Column: 24, Exported: true, AttestBound: false}, {Name: identifier("Limits"), Kind: projectstandards.SourceDeclarationKindType, Line: 60, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("DefaultLimits"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 67, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("Validate"), Receiver: sourceIdentifier("Limits"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 80, Column: 17, Exported: true, AttestBound: false}, {Name: identifier("Configuration"), Kind: projectstandards.SourceDeclarationKindType, Line: 92, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("Validate"), Receiver: sourceIdentifier("Configuration"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 97, Column: 24, Exported: true, AttestBound: false}, {Name: identifier("ToolchainVersion"), Kind: projectstandards.SourceDeclarationKindType, Line: 102, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("ParseToolchainVersion"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 105, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("Validate"), Receiver: sourceIdentifier("ToolchainVersion"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 113, Column: 27, Exported: true, AttestBound: false}, {Name: identifier("validateVersionComponent"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 125, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("String"), Receiver: sourceIdentifier("ToolchainVersion"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 137, Column: 27, Exported: true, AttestBound: false}, {Name: identifier("BuildContext"), Kind: projectstandards.SourceDeclarationKindType, Line: 145, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("Validate"), Receiver: sourceIdentifier("BuildContext"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 151, Column: 23, Exported: true, AttestBound: false}, {Name: identifier("PackageName"), Kind: projectstandards.SourceDeclarationKindType, Line: 156, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("ParsePackageName"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 159, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("Validate"), Receiver: sourceIdentifier("PackageName"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 167, Column: 22, Exported: true, AttestBound: false}, {Name: identifier("String"), Receiver: sourceIdentifier("PackageName"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 174, Column: 22, Exported: true, AttestBound: false}, {Name: identifier("Package"), Kind: projectstandards.SourceDeclarationKindType, Line: 182, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("Validate"), Receiver: sourceIdentifier("Package"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 189, Column: 18, Exported: true, AttestBound: false}, {Name: identifier("PackageCatalog"), Kind: projectstandards.SourceDeclarationKindType, Line: 206, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("Validate"), Receiver: sourceIdentifier("PackageCatalog"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 208, Column: 25, Exported: true, AttestBound: false}, {Name: identifier("ObservationRequest"), Kind: projectstandards.SourceDeclarationKindType, Line: 227, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("Validate"), Receiver: sourceIdentifier("ObservationRequest"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 229, Column: 29, Exported: true, AttestBound: false}, {Name: identifier("ListRequest"), Kind: projectstandards.SourceDeclarationKindType, Line: 237, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("Validate"), Receiver: sourceIdentifier("ListRequest"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 243, Column: 22, Exported: true, AttestBound: false}, {Name: identifier("CompileRequest"), Kind: projectstandards.SourceDeclarationKindType, Line: 254, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("Validate"), Receiver: sourceIdentifier("CompileRequest"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 259, Column: 25, Exported: true, AttestBound: false}, {Name: identifier("Compilation"), Kind: projectstandards.SourceDeclarationKindType, Line: 264, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("Validate"), Receiver: sourceIdentifier("Compilation"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 266, Column: 22, Exported: true, AttestBound: false}}},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("temporal")}}, Mediated: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/temporal"), Capability: &projectstandards.PrimitiveCapabilityUse{Package: capability("temporal")}, Selector: sourceIdentifier("DurationFromSeconds"), Line: 72, Column: 15}}, Unresolved: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/gotoolchain"), Selector: sourceIdentifier("contractError"), Line: 35, Column: 10}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gotoolchain"), Selector: sourceIdentifier("Validate"), Line: 41, Column: 48}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gotoolchain"), Selector: sourceIdentifier("IsValid"), Line: 48, Column: 6}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gotoolchain"), Selector: sourceIdentifier("Uint64"), Line: 84, Column: 16}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gotoolchain"), Selector: sourceIdentifier("IsKeyword"), Line: 168, Column: 37}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gotoolchain"), Selector: sourceIdentifier("String"), Line: 217, Column: 14}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gotoolchain"), Selector: sourceIdentifier("ExitCode"), Line: 270, Column: 15}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gotoolchain"), Selector: sourceIdentifier("Success"), Line: 274, Column: 18}}},
			},
			{
				Path: sourcePath("gotoolchain/doc.go"), Package: sourcePath("gotoolchain"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy},
			},
			{
				Path: sourcePath("gotoolchain/doctrine_contract.go"), Package: sourcePath("gotoolchain"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{{ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/core"), Kind: projectstandards.SourceImportKindProject}}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0, Symbols: []projectstandards.SourceDeclaration{{Name: identifier("doctrinePackageCapability"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 5, Column: 7, Exported: false, AttestBound: false}}},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy},
			},
			{
				Path: sourcePath("gotoolchain/toolchain.go"), Package: sourcePath("gotoolchain"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{{Path: sourcePath("bytes"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("context"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("encoding/json/jsontext"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("encoding/json/v2"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("errors"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("fmt"), Kind: projectstandards.SourceImportKindStandardLibrary}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/core"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/gomodule"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/process"), Kind: projectstandards.SourceImportKindProject}, {Path: sourcePath("io"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("slices"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("strings"), Kind: projectstandards.SourceImportKindStandardLibrary}}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0, Symbols: []projectstandards.SourceDeclaration{{Name: identifier("Capability"), Kind: projectstandards.SourceDeclarationKindType, Line: 20, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("Open"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 27, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("Validate"), Receiver: sourceIdentifier("Capability"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 56, Column: 21, Exported: true, AttestBound: false}, {Name: identifier("ObserveModule"), Receiver: sourceIdentifier("Capability"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 61, Column: 21, Exported: true, AttestBound: false}, {Name: identifier("ObserveBuildContext"), Receiver: sourceIdentifier("Capability"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 81, Column: 21, Exported: true, AttestBound: false}, {Name: identifier("ListPackages"), Receiver: sourceIdentifier("Capability"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 93, Column: 21, Exported: true, AttestBound: false}, {Name: identifier("CompilePackage"), Receiver: sourceIdentifier("Capability"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 110, Column: 21, Exported: true, AttestBound: false}, {Name: identifier("execute"), Receiver: sourceIdentifier("Capability"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 130, Column: 21, Exported: false, AttestBound: false}, {Name: identifier("runToolchainGroup"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 162, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("buildContextWire"), Kind: projectstandards.SourceDeclarationKindType, Line: 174, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("decodeBuildContext"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 181, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("packageWire"), Kind: projectstandards.SourceDeclarationKindType, Line: 201, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("moduleWire"), Kind: projectstandards.SourceDeclarationKindType, Line: 210, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("decodePackageCatalog"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 214, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("packageFromWire"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 249, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("disableWorkspace"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 272, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("contractError"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 300, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("outputError"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 304, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("executionError"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 308, Column: 6, Exported: false, AttestBound: false}}},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("process")}}, Mediated: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/process"), Capability: &projectstandards.PrimitiveCapabilityUse{Package: capability("process")}, Selector: sourceIdentifier("Resolve"), Line: 35, Column: 18}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/process"), Capability: &projectstandards.PrimitiveCapabilityUse{Package: capability("process")}, Selector: sourceIdentifier("AmbientEnvironment"), Line: 39, Column: 22}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/process"), Capability: &projectstandards.PrimitiveCapabilityUse{Package: capability("process")}, Selector: sourceIdentifier("DiscardDeviceArgument"), Line: 114, Column: 18}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/process"), Capability: &projectstandards.PrimitiveCapabilityUse{Package: capability("process")}, Selector: sourceIdentifier("ParseArguments"), Line: 131, Column: 20}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/process"), Capability: &projectstandards.PrimitiveCapabilityUse{Package: capability("process")}, Selector: sourceIdentifier("Begin"), Line: 163, Column: 20}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/process"), Capability: &projectstandards.PrimitiveCapabilityUse{Package: capability("process")}, Selector: sourceIdentifier("NewEnvironmentName"), Line: 273, Column: 15}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/process"), Capability: &projectstandards.PrimitiveCapabilityUse{Package: capability("process")}, Selector: sourceIdentifier("NewEnvironmentValue"), Line: 277, Column: 16}}, Unresolved: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/gotoolchain"), Selector: sourceIdentifier("Validate"), Line: 28, Column: 12}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gotoolchain"), Selector: sourceIdentifier("execute"), Line: 65, Column: 20}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gotoolchain"), Selector: sourceIdentifier("Value"), Line: 118, Column: 23}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gotoolchain"), Selector: sourceIdentifier("ExitCode"), Line: 147, Column: 15}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gotoolchain"), Selector: sourceIdentifier("Success"), Line: 151, Column: 18}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gotoolchain"), Selector: sourceIdentifier("String"), Line: 156, Column: 35}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gotoolchain"), Selector: sourceIdentifier("Bytes"), Line: 159, Column: 21}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gotoolchain"), Selector: sourceIdentifier("Wait"), Line: 167, Column: 21}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gotoolchain"), Selector: sourceIdentifier("Sweep"), Line: 171, Column: 38}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gotoolchain"), Selector: sourceIdentifier("UnmarshalText"), Line: 187, Column: 12}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gotoolchain"), Selector: sourceIdentifier("ParseToolchainVersion"), Line: 190, Column: 18}, {ImportPath: sourcePath("encoding/json/jsontext"), Selector: sourceIdentifier("NewDecoder"), Line: 219, Column: 13}, {ImportPath: sourcePath("encoding/json/v2"), Selector: sourceIdentifier("UnmarshalDecode"), Line: 223, Column: 10}, {ImportPath: sourcePath("encoding/json/v2"), Selector: sourceIdentifier("UnmarshalDecode"), Line: 238, Column: 13}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gotoolchain"), Selector: sourceIdentifier("ParsePackageName"), Line: 257, Column: 15}}},
			},
			{
				Path: sourcePath("gotoolchain/toolchain_external_test.go"), Package: sourcePath("gotoolchain"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{{Path: sourcePath("context"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("errors"), Kind: projectstandards.SourceImportKindStandardLibrary}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/core"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/gotoolchain"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/process"), Kind: projectstandards.SourceImportKindProject}, {Path: sourcePath("testing"), Kind: projectstandards.SourceImportKindStandardLibrary}}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 2, Benchmarks: 0, FuzzTargets: 0, Symbols: []projectstandards.SourceDeclaration{{Name: identifier("TestCapabilityProductionPathLayerTriad"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 13, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("TestCompilerScalarsRejectUnknownAndPreserveCanonicalValues"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 106, Column: 6, Exported: true, AttestBound: false}}},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("process")}}, Mediated: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/process"), Capability: &projectstandards.PrimitiveCapabilityUse{Package: capability("process")}, Selector: sourceIdentifier("WorkingDirectory"), Line: 27, Column: 20}}, Unresolved: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/gotoolchain"), Selector: sourceIdentifier("Parallel"), Line: 14, Column: 2}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gotoolchain"), Selector: sourceIdentifier("Fatalf"), Line: 18, Column: 3}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gotoolchain"), Selector: sourceIdentifier("Parent"), Line: 31, Column: 19}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gotoolchain"), Selector: sourceIdentifier("Run"), Line: 36, Column: 2}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gotoolchain"), Selector: sourceIdentifier("ObserveModule"), Line: 40, Column: 24}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gotoolchain"), Selector: sourceIdentifier("String"), Line: 41, Column: 26}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gotoolchain"), Selector: sourceIdentifier("ObserveBuildContext"), Line: 44, Column: 22}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gotoolchain"), Selector: sourceIdentifier("Validate"), Line: 48, Column: 13}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gotoolchain"), Selector: sourceIdentifier("ListPackages"), Line: 51, Column: 23}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gotoolchain"), Selector: sourceIdentifier("CompilePackage"), Line: 61, Column: 30}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gotoolchain"), Selector: sourceIdentifier("cancel"), Line: 98, Column: 3}}},
			},
		},
	}
	architecture, err := projectstandards.DerivePackageArchitecture(catalog.Files)
	generatedErr = errors.Join(generatedErr, err)
	catalog.Architecture = &architecture
	if generatedErr != nil {
		return projectstandards.PackageFileCatalog{}, generatedErr
	}
	if err := catalog.ValidateComplete(); err != nil {
		return projectstandards.PackageFileCatalog{}, err
	}
	return catalog, nil
}
