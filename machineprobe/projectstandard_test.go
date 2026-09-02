// Package machineprobe_test owns compiler-checked knowledge about package machineprobe.
package machineprobe_test

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
		Path:        authorPath("machineprobe"),
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
		Package: sourcePath("machineprobe"),
		Files: []projectstandards.SourceFile{
			{
				Path: sourcePath("machineprobe/architecture_test.go"), Package: sourcePath("machineprobe"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{{Path: sourcePath("embed"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("go/ast"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("go/parser"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("go/token"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("slices"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("strings"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("testing"), Kind: projectstandards.SourceImportKindStandardLibrary}}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 1, Benchmarks: 0, FuzzTargets: 0, Symbols: []projectstandards.SourceDeclaration{{Name: identifier("machineProbeSources"), Kind: projectstandards.SourceDeclarationKindVariable, Line: 14, Column: 5, Exported: false, AttestBound: false}, {Name: identifier("TestMachineProbeProductionStructsHaveCompilerVisibleDataFlowRoles"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 16, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("machineProbeDataFlowInventory"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 30, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("machineProbeReceiverName"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 70, Column: 6, Exported: false, AttestBound: false}}},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, Unresolved: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Selector: sourceIdentifier("Parallel"), Line: 17, Column: 2}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Selector: sourceIdentifier("Fatalf"), Line: 21, Column: 3}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Selector: sourceIdentifier("ReadDir"), Line: 31, Column: 18}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Selector: sourceIdentifier("Name"), Line: 38, Column: 24}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Selector: sourceIdentifier("ReadFile"), Line: 41, Column: 22}, {ImportPath: sourcePath("go/parser"), Selector: sourceIdentifier("ParseFile"), Line: 45, Column: 21}}},
			},
			{
				Path: sourcePath("machineprobe/collect.go"), Package: sourcePath("machineprobe"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{{Path: sourcePath("bytes"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("context"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("encoding/json/v2"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("errors"), Kind: projectstandards.SourceImportKindStandardLibrary}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/core"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/filestore"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/process"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/projectstandards"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/temporal"), Kind: projectstandards.SourceImportKindProject}}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0, Symbols: []projectstandards.SourceDeclaration{{Name: identifier("OutputMaximumBytes"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 17, Column: 2, Exported: true, AttestBound: false}, {Name: identifier("ScriptMaximumBytes"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 18, Column: 2, Exported: true, AttestBound: false}, {Name: identifier("Request"), Kind: projectstandards.SourceDeclarationKindType, Line: 21, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("Validate"), Receiver: sourceIdentifier("Request"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 33, Column: 18, Exported: true, AttestBound: false}, {Name: identifier("FailureKind"), Kind: projectstandards.SourceDeclarationKindType, Line: 43, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("FailureUnknown"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 46, Column: 2, Exported: true, AttestBound: false}, {Name: identifier("FailureExit"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 47, Column: 2, Exported: true, AttestBound: false}, {Name: identifier("FailureOutput"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 48, Column: 2, Exported: true, AttestBound: false}, {Name: identifier("failureLimit"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 49, Column: 2, Exported: false, AttestBound: false}, {Name: identifier("Validate"), Receiver: sourceIdentifier("FailureKind"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 52, Column: 22, Exported: true, AttestBound: false}, {Name: identifier("IsValid"), Receiver: sourceIdentifier("FailureKind"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 59, Column: 22, Exported: true, AttestBound: false}, {Name: identifier("String"), Receiver: sourceIdentifier("FailureKind"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 61, Column: 22, Exported: true, AttestBound: false}, {Name: identifier("MarshalJSON"), Receiver: sourceIdentifier("FailureKind"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 68, Column: 22, Exported: true, AttestBound: false}, {Name: identifier("UnmarshalJSON"), Receiver: sourceIdentifier("FailureKind"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 75, Column: 23, Exported: true, AttestBound: false}, {Name: identifier("Failure"), Kind: projectstandards.SourceDeclarationKindType, Line: 92, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("failureInput"), Kind: projectstandards.SourceDeclarationKindType, Line: 100, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("executionFactInput"), Kind: projectstandards.SourceDeclarationKindType, Line: 107, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("Error"), Receiver: sourceIdentifier("Failure"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 117, Column: 18, Exported: true, AttestBound: false}, {Name: identifier("Unwrap"), Receiver: sourceIdentifier("Failure"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 118, Column: 18, Exported: true, AttestBound: false}, {Name: identifier("Kind"), Receiver: sourceIdentifier("Failure"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 119, Column: 18, Exported: true, AttestBound: false}, {Name: identifier("ExitCode"), Receiver: sourceIdentifier("Failure"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 120, Column: 18, Exported: true, AttestBound: false}, {Name: identifier("StderrDigest"), Receiver: sourceIdentifier("Failure"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 121, Column: 18, Exported: true, AttestBound: false}, {Name: identifier("StderrBytes"), Receiver: sourceIdentifier("Failure"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 122, Column: 18, Exported: true, AttestBound: false}, {Name: identifier("Validate"), Receiver: sourceIdentifier("Failure"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 124, Column: 18, Exported: true, AttestBound: false}, {Name: identifier("Collect"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 134, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("run"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 162, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("readScript"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 198, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("executionFact"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 220, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("newFailure"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 260, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("byteLength"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 272, Column: 6, Exported: false, AttestBound: false}}},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("filestore")}, {Package: capability("process")}}, Mediated: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/process"), Capability: &projectstandards.PrimitiveCapabilityUse{Package: capability("process")}, Selector: sourceIdentifier("NewArgument"), Line: 170, Column: 19}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/process"), Capability: &projectstandards.PrimitiveCapabilityUse{Package: capability("process")}, Selector: sourceIdentifier("Run"), Line: 180, Column: 17}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/filestore"), Capability: &projectstandards.PrimitiveCapabilityUse{Package: capability("filestore")}, Selector: sourceIdentifier("OpenParent"), Line: 199, Column: 19}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/filestore"), Capability: &projectstandards.PrimitiveCapabilityUse{Package: capability("filestore")}, Selector: sourceIdentifier("Read"), Line: 209, Column: 20}}, Unresolved: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Selector: sourceIdentifier("Validate"), Line: 34, Column: 24}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Selector: sourceIdentifier("IsZero"), Line: 37, Column: 5}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Selector: sourceIdentifier("IsValid"), Line: 62, Column: 6}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Selector: sourceIdentifier("String"), Line: 72, Column: 41}, {ImportPath: sourcePath("encoding/json/v2"), Selector: sourceIdentifier("Unmarshal"), Line: 143, Column: 12}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Selector: sourceIdentifier("Fingerprint"), Line: 146, Column: 22}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Selector: sourceIdentifier("Uint64"), Line: 167, Column: 5}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Selector: sourceIdentifier("Bytes"), Line: 190, Column: 41}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Selector: sourceIdentifier("Close"), Line: 205, Column: 15}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Selector: sourceIdentifier("ExitCode"), Line: 221, Column: 15}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Selector: sourceIdentifier("Int"), Line: 225, Column: 19}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Selector: sourceIdentifier("CPUTime"), Line: 236, Column: 14}}},
			},
			{
				Path: sourcePath("machineprobe/collect_layer_triad_test.go"), Package: sourcePath("machineprobe"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{{Path: sourcePath("bytes"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("errors"), Kind: projectstandards.SourceImportKindStandardLibrary}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/core"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/filestore"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/id"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/process"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/projectstandards"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/temporal"), Kind: projectstandards.SourceImportKindProject}, {Path: sourcePath("io/fs"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("os"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("path/filepath"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("testing"), Kind: projectstandards.SourceImportKindStandardLibrary}}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 2, Benchmarks: 0, FuzzTargets: 0, Symbols: []projectstandards.SourceDeclaration{{Name: identifier("TestMachineProbeProcessBoundaryLayerTriad"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 20, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("TestMachineProbeScriptExtentBoundary"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 71, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("exactProbeScript"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 104, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("isZeroObservation"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 117, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("writeProbeFixture"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 123, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("writeFile"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 170, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("validProbeScript"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 182, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("machineReport"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 186, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("mustUUID"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 208, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("mustIdentifier"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 217, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("mustName"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 225, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("mustByteCount"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 233, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("mustByteLength"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 241, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("mustDuration"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 249, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("mustAbsolutePath"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 257, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("mustRelativePath"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 265, Column: 6, Exported: false, AttestBound: false}}},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("filestore")}, {Package: capability("temporal")}, {Package: capability("process")}}, Mediated: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/filestore"), Capability: &projectstandards.PrimitiveCapabilityUse{Package: capability("filestore")}, Selector: sourceIdentifier("OpenRoot"), Line: 126, Column: 15}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/process"), Capability: &projectstandards.PrimitiveCapabilityUse{Package: capability("process")}, Selector: sourceIdentifier("Resolve"), Line: 148, Column: 15}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/temporal"), Capability: &projectstandards.PrimitiveCapabilityUse{Package: capability("temporal")}, Selector: sourceIdentifier("DurationFromSeconds"), Line: 158, Column: 20}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/temporal"), Capability: &projectstandards.PrimitiveCapabilityUse{Package: capability("temporal")}, Selector: sourceIdentifier("InstantFromNanoseconds"), Line: 163, Column: 73}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/filestore"), Capability: &projectstandards.PrimitiveCapabilityUse{Package: capability("filestore")}, Selector: sourceIdentifier("Write"), Line: 173, Column: 12}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/temporal"), Capability: &projectstandards.PrimitiveCapabilityUse{Package: capability("temporal")}, Selector: sourceIdentifier("DurationFromSeconds"), Line: 251, Column: 14}}, Unresolved: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Selector: sourceIdentifier("Parallel"), Line: 21, Column: 2}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Selector: sourceIdentifier("Run"), Line: 35, Column: 3}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Selector: sourceIdentifier("TempDir"), Line: 37, Column: 17}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Selector: sourceIdentifier("script"), Line: 38, Column: 14}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Selector: sourceIdentifier("Context"), Line: 40, Column: 40}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Selector: sourceIdentifier("Fatalf"), Line: 43, Column: 6}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Selector: sourceIdentifier("Validate"), Line: 45, Column: 15}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Selector: sourceIdentifier("Uint64"), Line: 48, Column: 102}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Selector: sourceIdentifier("Kind"), Line: 57, Column: 39}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Selector: sourceIdentifier("StderrBytes"), Line: 60, Column: 22}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Selector: sourceIdentifier("Helper"), Line: 105, Column: 2}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Selector: sourceIdentifier("Cleanup"), Line: 130, Column: 2}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Selector: sourceIdentifier("Close"), Line: 131, Column: 18}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Selector: sourceIdentifier("Errorf"), Line: 132, Column: 4}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Selector: sourceIdentifier("MarshalJSON"), Line: 138, Column: 25}}},
			},
			{
				Path: sourcePath("machineprobe/data_flow_inventory.go"), Package: sourcePath("machineprobe"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0, Symbols: []projectstandards.SourceDeclaration{{Name: identifier("protocolFact"), Kind: projectstandards.SourceDeclarationKindType, Line: 5, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("sealedProjection"), Kind: projectstandards.SourceDeclarationKindType, Line: 6, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("internalFlow"), Kind: projectstandards.SourceDeclarationKindType, Line: 7, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("machineProbeProtocolFact"), Receiver: sourceIdentifier("Request"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 9, Column: 16, Exported: false, AttestBound: false}, {Name: identifier("machineProbeSealedProjection"), Receiver: sourceIdentifier("Failure"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 10, Column: 16, Exported: false, AttestBound: false}, {Name: identifier("machineProbeInternalFlow"), Receiver: sourceIdentifier("failureInput"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 11, Column: 21, Exported: false, AttestBound: false}, {Name: identifier("machineProbeInternalFlow"), Receiver: sourceIdentifier("executionFactInput"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 12, Column: 27, Exported: false, AttestBound: false}}},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy},
			},
			{
				Path: sourcePath("machineprobe/doc.go"), Package: sourcePath("machineprobe"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy},
			},
			{
				Path: sourcePath("machineprobe/doctrine_contract.go"), Package: sourcePath("machineprobe"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{{ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/core"), Kind: projectstandards.SourceImportKindProject}}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0, Symbols: []projectstandards.SourceDeclaration{{Name: identifier("doctrinePackageCapability"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 5, Column: 7, Exported: false, AttestBound: false}}},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy},
			},
			{
				Path: sourcePath("machineprobe/validation_witnesses.go"), Package: sourcePath("machineprobe"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{{ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/core"), Kind: projectstandards.SourceImportKindProject}}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, Unresolved: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/machineprobe"), Selector: sourceIdentifier("FailureKind"), Line: 5, Column: 37}}},
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
