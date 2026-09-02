// Package lineio_test owns compiler-checked knowledge about package lineio.
package lineio_test

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
		Path:        authorPath("lineio"),
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
		Package: sourcePath("lineio"),
		Files: []projectstandards.SourceFile{
			{
				Path: sourcePath("lineio/architecture_test.go"), Package: sourcePath("lineio"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{{Path: sourcePath("context"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("errors"), Kind: projectstandards.SourceImportKindStandardLibrary}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/core"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/filestore"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/lineio"), Kind: projectstandards.SourceImportKindProject}, {Path: sourcePath("go/ast"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("go/parser"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("go/token"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("reflect"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("runtime"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("slices"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("strings"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("testing"), Kind: projectstandards.SourceImportKindStandardLibrary}}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 1, Benchmarks: 0, FuzzTargets: 0, Symbols: []projectstandards.SourceDeclaration{{Name: identifier("lineioProductionDirectoryEntryMaximum"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 20, Column: 7, Exported: false, AttestBound: false}, {Name: identifier("lineioRequestContract"), Kind: projectstandards.SourceDeclarationKindType, Line: 23, Column: 2, Exported: false, AttestBound: false}, {Name: identifier("lineioCapabilityContract"), Kind: projectstandards.SourceDeclarationKindType, Line: 24, Column: 2, Exported: false, AttestBound: false}, {Name: identifier("lineioContractInventory"), Kind: projectstandards.SourceDeclarationKindType, Line: 29, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("TestLineIOProductionStructsHaveCompilerVisibleDataFlowRoles"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 41, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("lineioProductionStructNames"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 54, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("productionStructNames"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 117, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("lineioClassifiedStructNames"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 134, Column: 6, Exported: false, AttestBound: false}}},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("filestore")}}, Mediated: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/filestore"), Capability: &projectstandards.PrimitiveCapabilityUse{Package: capability("filestore")}, Selector: sourceIdentifier("OpenRoot"), Line: 67, Column: 15}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/filestore"), Capability: &projectstandards.PrimitiveCapabilityUse{Package: capability("filestore")}, Selector: sourceIdentifier("NewDirectoryEntryMaximum"), Line: 78, Column: 23}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/filestore"), Capability: &projectstandards.PrimitiveCapabilityUse{Package: capability("filestore")}, Selector: sourceIdentifier("Walk"), Line: 83, Column: 13}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/filestore"), Capability: &projectstandards.PrimitiveCapabilityUse{Package: capability("filestore")}, Selector: sourceIdentifier("OpenRead"), Line: 95, Column: 21}}, Unresolved: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Line: 38, Column: 23}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Parallel"), Line: 42, Column: 2}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Context"), Line: 44, Column: 42}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Fatalf"), Line: 46, Column: 3}, {ImportPath: sourcePath("runtime"), Selector: sourceIdentifier("Caller"), Line: 55, Column: 30}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Parent"), Line: 63, Column: 24}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Close"), Line: 72, Column: 38}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("IsDir"), Line: 88, Column: 7}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Name"), Line: 91, Column: 12}, {ImportPath: sourcePath("go/parser"), Selector: sourceIdentifier("ParseFile"), Line: 101, Column: 24}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("String"), Line: 101, Column: 48}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("NumField"), Line: 136, Column: 26}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Field"), Line: 138, Column: 18}}},
			},
			{
				Path: sourcePath("lineio/doc.go"), Package: sourcePath("lineio"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy},
			},
			{
				Path: sourcePath("lineio/scanner.go"), Package: sourcePath("lineio"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{{Path: sourcePath("bufio"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("errors"), Kind: projectstandards.SourceImportKindStandardLibrary}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/core"), Kind: projectstandards.SourceImportKindProject}, {Path: sourcePath("io"), Kind: projectstandards.SourceImportKindStandardLibrary}}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0, Symbols: []projectstandards.SourceDeclaration{{Name: identifier("MaximumLineBytes"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 14, Column: 2, Exported: true, AttestBound: false}, {Name: identifier("scanLinesMaximumDelimiterBytes"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 15, Column: 2, Exported: false, AttestBound: false}, {Name: identifier("BufferPolicy"), Kind: projectstandards.SourceDeclarationKindType, Line: 19, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("Validate"), Receiver: sourceIdentifier("BufferPolicy"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 25, Column: 23, Exported: true, AttestBound: false}, {Name: identifier("Request"), Kind: projectstandards.SourceDeclarationKindType, Line: 50, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("Validate"), Receiver: sourceIdentifier("Request"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 56, Column: 18, Exported: true, AttestBound: false}, {Name: identifier("Scanner"), Kind: projectstandards.SourceDeclarationKindType, Line: 65, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("MaximumLineByteCount"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 72, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("New"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 78, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("Validate"), Receiver: sourceIdentifier("Scanner"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 93, Column: 19, Exported: true, AttestBound: false}, {Name: identifier("Scan"), Receiver: sourceIdentifier("Scanner"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 101, Column: 19, Exported: true, AttestBound: false}, {Name: identifier("Bytes"), Receiver: sourceIdentifier("Scanner"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 114, Column: 19, Exported: true, AttestBound: false}, {Name: identifier("Err"), Receiver: sourceIdentifier("Scanner"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 123, Column: 19, Exported: true, AttestBound: false}, {Name: identifier("nativeBounds"), Receiver: sourceIdentifier("BufferPolicy"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 130, Column: 23, Exported: false, AttestBound: false}, {Name: identifier("boundedScanLines"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 147, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("linePrefixExceedsMaximum"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 166, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("lineTooLongError"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 173, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("classifyScanError"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 177, Column: 6, Exported: false, AttestBound: false}}},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, Unresolved: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Validate"), Line: 26, Column: 12}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Uint64"), Line: 32, Column: 18}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("nativeBounds"), Line: 82, Column: 27}, {ImportPath: sourcePath("bufio"), Selector: sourceIdentifier("NewScanner"), Line: 86, Column: 13}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Buffer"), Line: 87, Column: 2}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Split"), Line: 88, Column: 2}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Scan"), Line: 105, Column: 5}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Err"), Line: 108, Column: 28}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Bytes"), Line: 118, Column: 9}, {ImportPath: sourcePath("bufio"), Selector: sourceIdentifier("ScanLines"), Line: 149, Column: 26}}},
			},
			{
				Path: sourcePath("lineio/scanner_filestore_layer_triad_external_test.go"), Package: sourcePath("lineio"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{{Path: sourcePath("bufio"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("bytes"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("errors"), Kind: projectstandards.SourceImportKindStandardLibrary}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/core"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/filestore"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/lineio"), Kind: projectstandards.SourceImportKindProject}, {Path: sourcePath("io/fs"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("slices"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("testing"), Kind: projectstandards.SourceImportKindStandardLibrary}}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 1, Benchmarks: 0, FuzzTargets: 0, Symbols: []projectstandards.SourceDeclaration{{Name: identifier("lineioProofTargetName"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 17, Column: 2, Exported: false, AttestBound: false}, {Name: identifier("lineioProofTemporaryName"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 18, Column: 2, Exported: false, AttestBound: false}, {Name: identifier("lineioProofFileMode"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 19, Column: 2, Exported: false, AttestBound: false}, {Name: identifier("TestScannerFilestoreLayerTriad"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 22, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("mustRelativePath"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 106, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("stringsOfLength"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 115, Column: 6, Exported: false, AttestBound: false}}},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("filestore")}}, Mediated: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/filestore"), Capability: &projectstandards.PrimitiveCapabilityUse{Package: capability("filestore")}, Selector: sourceIdentifier("OpenRoot"), Line: 44, Column: 17}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/filestore"), Capability: &projectstandards.PrimitiveCapabilityUse{Package: capability("filestore")}, Selector: sourceIdentifier("Write"), Line: 55, Column: 13}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/filestore"), Capability: &projectstandards.PrimitiveCapabilityUse{Package: capability("filestore")}, Selector: sourceIdentifier("OpenRead"), Line: 66, Column: 17}}, Unresolved: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Parallel"), Line: 23, Column: 2}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Run"), Line: 37, Column: 3}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("TempDir"), Line: 39, Column: 21}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Fatalf"), Line: 42, Column: 5}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Context"), Line: 44, Column: 36}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Cleanup"), Line: 48, Column: 4}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Close"), Line: 49, Column: 20}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Errorf"), Line: 50, Column: 6}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("mustByteCount"), Line: 61, Column: 19}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("scanStrings"), Line: 88, Column: 11}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Err"), Line: 92, Column: 14}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Helper"), Line: 107, Column: 2}}},
			},
			{
				Path: sourcePath("lineio/scanner_hostile_external_test.go"), Package: sourcePath("lineio"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{{Path: sourcePath("bufio"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("bytes"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("errors"), Kind: projectstandards.SourceImportKindStandardLibrary}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/core"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/lineio"), Kind: projectstandards.SourceImportKindProject}, {Path: sourcePath("io"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("slices"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("strings"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("testing"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("testing/iotest"), Kind: projectstandards.SourceImportKindStandardLibrary}}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 3, Benchmarks: 0, FuzzTargets: 1, Symbols: []projectstandards.SourceDeclaration{{Name: identifier("hostileInitialBytes"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 18, Column: 2, Exported: false, AttestBound: false}, {Name: identifier("hostileMaximumLineBytes"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 19, Column: 2, Exported: false, AttestBound: false}, {Name: identifier("hostileReaderError"), Kind: projectstandards.SourceDeclarationKindType, Line: 22, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("Error"), Receiver: sourceIdentifier("hostileReaderError"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 24, Column: 27, Exported: true, AttestBound: false}, {Name: identifier("hostileCaseClass"), Kind: projectstandards.SourceDeclarationKindType, Line: 26, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("hostileCaseUnknown"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 29, Column: 2, Exported: false, AttestBound: false}, {Name: identifier("hostileCasePositive"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 30, Column: 2, Exported: false, AttestBound: false}, {Name: identifier("hostileCaseNegative"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 31, Column: 2, Exported: false, AttestBound: false}, {Name: identifier("hostileCaseBoundary"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 32, Column: 2, Exported: false, AttestBound: false}, {Name: identifier("hostileCaseLimit"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 33, Column: 2, Exported: false, AttestBound: false}, {Name: identifier("Validate"), Receiver: sourceIdentifier("hostileCaseClass"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 36, Column: 27, Exported: true, AttestBound: false}, {Name: identifier("scannerHostileCase"), Kind: projectstandards.SourceDeclarationKindType, Line: 43, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("TestScannerIngressLayerTriadHostileTable"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 53, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("TestScannerStateExhaustive"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 159, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("FuzzScannerSemanticClosure"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 192, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("scannerRequest"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 237, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("scannerRequestFrom"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 241, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("scannerRequestWithPolicy"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 254, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("scannerRequestWithCounts"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 260, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("nilScannerRequest"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 269, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("typedNilScannerRequest"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 278, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("mustByteCount"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 289, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("scanStrings"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 298, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("independentScanLines"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 306, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("dropFinalCarriageReturn"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 328, Column: 6, Exported: false, AttestBound: false}}},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, Unresolved: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Parallel"), Line: 54, Column: 2}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Fatalf"), Line: 58, Column: 3}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Uint64"), Line: 60, Column: 32}, {ImportPath: sourcePath("testing/iotest"), Selector: sourceIdentifier("OneByteReader"), Line: 79, Column: 143}, {ImportPath: sourcePath("testing/iotest"), Selector: sourceIdentifier("HalfReader"), Line: 80, Column: 145}, {ImportPath: sourcePath("testing/iotest"), Selector: sourceIdentifier("ErrReader"), Line: 90, Column: 139}, {ImportPath: sourcePath("testing/iotest"), Selector: sourceIdentifier("ErrReader"), Line: 92, Column: 55}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Validate"), Line: 119, Column: 13}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Run"), Line: 123, Column: 3}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("request"), Line: 125, Column: 37}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Err"), Line: 139, Column: 18}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Line: 162, Column: 13}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Scan"), Line: 184, Column: 8}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Bytes"), Line: 184, Column: 29}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Add"), Line: 205, Column: 3}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Fuzz"), Line: 208, Column: 2}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("Helper"), Line: 243, Column: 3}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/lineio"), Selector: sourceIdentifier("source"), Line: 245, Column: 12}}},
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
