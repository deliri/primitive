// Package testserial_test owns compiler-checked knowledge about package testserial.
package testserial_test

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
		Path:        authorPath("testserial"),
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
		Package: sourcePath("testserial"),
		Files: []projectstandards.SourceFile{
			{
				Path: sourcePath("testserial/architecture_test.go"), Package: sourcePath("testserial"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{{ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/core"), Kind: projectstandards.SourceImportKindProject}, {Path: sourcePath("go/ast"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("go/parser"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("go/token"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("os"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("path/filepath"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("slices"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("strconv"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("strings"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("testing"), Kind: projectstandards.SourceImportKindStandardLibrary}}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 2, Benchmarks: 0, FuzzTargets: 0, Symbols: []projectstandards.SourceDeclaration{{Name: identifier("TestTestserialExactPublicSurfaceAndStandardTestingBoundary"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 17, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("TestTestserialArchitectureMatcherClassifiesSyntheticBoundaries"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 36, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("testserialArchitectureScan"), Kind: projectstandards.SourceDeclarationKindType, Line: 161, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("scanTestserialArchitecture"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 170, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("scanTestserialFile"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 195, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("scanTestserialFunction"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 231, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("scanTestserialDeclaration"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 251, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("testserialReceiverName"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 280, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("sortTestserialArchitectureScan"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 291, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("testserialArchitectureScansEqual"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 300, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("testserialProductionGoFiles"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 312, Column: 6, Exported: false, AttestBound: false}}},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectDirectObserved, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("filestore")}}, Direct: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("os"), Capability: &projectstandards.PrimitiveCapabilityUse{Package: capability("filestore")}, Selector: sourceIdentifier("WriteFile"), Line: 147, Column: 17}, {ImportPath: sourcePath("go/parser"), Capability: &projectstandards.PrimitiveCapabilityUse{Package: capability("filestore")}, Selector: sourceIdentifier("ParseFile"), Line: 180, Column: 21}, {ImportPath: sourcePath("os"), Capability: &projectstandards.PrimitiveCapabilityUse{Package: capability("filestore")}, Selector: sourceIdentifier("ReadDir"), Line: 313, Column: 18}}, Unresolved: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/testserial"), Selector: sourceIdentifier("Parallel"), Line: 18, Column: 2}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/testserial"), Selector: sourceIdentifier("Fatalf"), Line: 22, Column: 3}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/testserial"), Selector: sourceIdentifier("Run"), Line: 142, Column: 3}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/testserial"), Selector: sourceIdentifier("TempDir"), Line: 145, Column: 12}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/testserial"), Selector: sourceIdentifier("IsExported"), Line: 235, Column: 6}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/testserial"), Selector: sourceIdentifier("IsValid"), Line: 258, Column: 7}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/testserial"), Selector: sourceIdentifier("String"), Line: 272, Column: 23}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/testserial"), Selector: sourceIdentifier("IsDir"), Line: 319, Column: 6}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/testserial"), Selector: sourceIdentifier("Name"), Line: 320, Column: 23}}},
			},
			{
				Path: sourcePath("testserial/declare.go"), Package: sourcePath("testserial"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{{ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/core"), Kind: projectstandards.SourceImportKindProject}, {Path: sourcePath("testing"), Kind: projectstandards.SourceImportKindStandardLibrary}}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0, Symbols: []projectstandards.SourceDeclaration{{Name: identifier("Declare"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 14, Column: 6, Exported: true, AttestBound: false}}},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, Unresolved: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/testserial"), Selector: sourceIdentifier("Helper"), Line: 15, Column: 2}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/testserial"), Selector: sourceIdentifier("Validate"), Line: 16, Column: 12}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/testserial"), Selector: sourceIdentifier("Fatal"), Line: 17, Column: 3}}},
			},
			{
				Path: sourcePath("testserial/declare_external_test.go"), Package: sourcePath("testserial"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{{ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/core"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/testserial"), Kind: projectstandards.SourceImportKindProject}, {Path: sourcePath("math"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("testing"), Kind: projectstandards.SourceImportKindStandardLibrary}}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 2, Benchmarks: 0, FuzzTargets: 0, Symbols: []projectstandards.SourceDeclaration{{Name: identifier("TestDeclareAcceptsEveryAdmittedDeclaration"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 24, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("TestDeclareRejectsInvalidDeclarationsBeforeFollowingBehavior"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 80, Column: 6, Exported: true, AttestBound: false}}},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, Unresolved: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/testserial"), Selector: sourceIdentifier("IsValid"), Line: 33, Column: 7}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/testserial"), Selector: sourceIdentifier("Validate"), Line: 42, Column: 17}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/testserial"), Selector: sourceIdentifier("GoIdentifier"), Line: 46, Column: 12}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/testserial"), Selector: sourceIdentifier("Fatalf"), Line: 61, Column: 5}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/testserial"), Selector: sourceIdentifier("Fatal"), Line: 72, Column: 3}}},
			},
			{
				Path: sourcePath("testserial/doc.go"), Package: sourcePath("testserial"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy},
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
