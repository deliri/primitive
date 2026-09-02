// Package gomodule_test owns compiler-checked knowledge about package gomodule.
package gomodule_test

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
		Path:        authorPath("gomodule"),
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
		Package: sourcePath("gomodule"),
		Files: []projectstandards.SourceFile{
			{
				Path: sourcePath("gomodule/doc.go"), Package: sourcePath("gomodule"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy},
			},
			{
				Path: sourcePath("gomodule/import_path.go"), Package: sourcePath("gomodule"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{{Path: sourcePath("errors"), Kind: projectstandards.SourceImportKindStandardLibrary}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/core"), Kind: projectstandards.SourceImportKindProject}, {Path: sourcePath("golang.org/x/mod/module"), Kind: projectstandards.SourceImportKindExternal}}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0, Symbols: []projectstandards.SourceDeclaration{{Name: identifier("ImportPathMaximumBytes"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 11, Column: 7, Exported: true, AttestBound: false}, {Name: identifier("ImportPath"), Kind: projectstandards.SourceDeclarationKindType, Line: 15, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("ParseImportPath"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 20, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("Validate"), Receiver: sourceIdentifier("ImportPath"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 29, Column: 21, Exported: true, AttestBound: false}, {Name: identifier("String"), Receiver: sourceIdentifier("ImportPath"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 40, Column: 21, Exported: true, AttestBound: false}, {Name: identifier("MarshalJSON"), Receiver: sourceIdentifier("ImportPath"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 48, Column: 21, Exported: true, AttestBound: false}, {Name: identifier("UnmarshalJSON"), Receiver: sourceIdentifier("ImportPath"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 61, Column: 22, Exported: true, AttestBound: false}}},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, Unresolved: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/gomodule"), Selector: sourceIdentifier("Validate"), Line: 22, Column: 12}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gomodule"), Selector: sourceIdentifier("contractError"), Line: 31, Column: 10}, {ImportPath: sourcePath("golang.org/x/mod/module"), Selector: sourceIdentifier("CheckImportPath"), Line: 33, Column: 12}}},
			},
			{
				Path: sourcePath("gomodule/path.go"), Package: sourcePath("gomodule"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{{Path: sourcePath("errors"), Kind: projectstandards.SourceImportKindStandardLibrary}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/core"), Kind: projectstandards.SourceImportKindProject}, {Path: sourcePath("golang.org/x/mod/module"), Kind: projectstandards.SourceImportKindExternal}}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0, Symbols: []projectstandards.SourceDeclaration{{Name: identifier("PathMaximumBytes"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 11, Column: 7, Exported: true, AttestBound: false}, {Name: identifier("Path"), Kind: projectstandards.SourceDeclarationKindType, Line: 14, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("ParsePath"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 19, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("Validate"), Receiver: sourceIdentifier("Path"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 28, Column: 15, Exported: true, AttestBound: false}, {Name: identifier("String"), Receiver: sourceIdentifier("Path"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 42, Column: 15, Exported: true, AttestBound: false}, {Name: identifier("MarshalJSON"), Receiver: sourceIdentifier("Path"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 50, Column: 15, Exported: true, AttestBound: false}, {Name: identifier("UnmarshalJSON"), Receiver: sourceIdentifier("Path"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 63, Column: 16, Exported: true, AttestBound: false}, {Name: identifier("contractError"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 79, Column: 6, Exported: false, AttestBound: false}}},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, Unresolved: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/gomodule"), Selector: sourceIdentifier("Validate"), Line: 21, Column: 12}, {ImportPath: sourcePath("golang.org/x/mod/module"), Selector: sourceIdentifier("CheckPath"), Line: 35, Column: 12}}},
			},
			{
				Path: sourcePath("gomodule/path_hostile_test.go"), Package: sourcePath("gomodule"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{{Path: sourcePath("errors"), Kind: projectstandards.SourceImportKindStandardLibrary}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/core"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/gomodule"), Kind: projectstandards.SourceImportKindProject}, {Path: sourcePath("strings"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("testing"), Kind: projectstandards.SourceImportKindStandardLibrary}}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 6, Benchmarks: 0, FuzzTargets: 2, Symbols: []projectstandards.SourceDeclaration{{Name: identifier("modulePathCaseClass"), Kind: projectstandards.SourceDeclarationKindType, Line: 12, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("modulePathCaseValid"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 15, Column: 2, Exported: false, AttestBound: false}, {Name: identifier("modulePathCaseReject"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 16, Column: 2, Exported: false, AttestBound: false}, {Name: identifier("modulePathCaseBoundary"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 17, Column: 2, Exported: false, AttestBound: false}, {Name: identifier("TestParsePathHostileDomain"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 20, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("TestPathJSONLayerTriad"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 107, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("TestParseImportPathHostileDomain"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 155, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("TestImportPathJSONLayerTriad"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 236, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("FuzzPathJSONSemanticClosure"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 284, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("FuzzImportPathJSONSemanticClosure"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 324, Column: 6, Exported: true, AttestBound: false}}},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, Unresolved: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/gomodule"), Selector: sourceIdentifier("Parallel"), Line: 21, Column: 2}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gomodule"), Selector: sourceIdentifier("Run"), Line: 78, Column: 3}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gomodule"), Selector: sourceIdentifier("Fatalf"), Line: 83, Column: 5}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gomodule"), Selector: sourceIdentifier("Validate"), Line: 94, Column: 14}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gomodule"), Selector: sourceIdentifier("String"), Line: 97, Column: 7}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gomodule"), Selector: sourceIdentifier("MarshalJSON"), Line: 117, Column: 19}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gomodule"), Selector: sourceIdentifier("UnmarshalJSON"), Line: 122, Column: 13}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gomodule"), Selector: sourceIdentifier("Add"), Line: 293, Column: 2}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/gomodule"), Selector: sourceIdentifier("Fuzz"), Line: 297, Column: 2}}},
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
