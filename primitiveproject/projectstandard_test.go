// Package primitiveproject_test owns compiler-checked knowledge about package primitiveproject.
package primitiveproject_test

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
		Path:        authorPath("primitiveproject"),
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
		Package: sourcePath("primitiveproject"),
		Files: []projectstandards.SourceFile{
			{
				Path: sourcePath("primitiveproject/policy_test.go"), Package: sourcePath("primitiveproject"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{{Path: sourcePath("errors"), Kind: projectstandards.SourceImportKindStandardLibrary}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/core"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/projectstandards"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/temporal"), Kind: projectstandards.SourceImportKindProject}, {Path: sourcePath("testing"), Kind: projectstandards.SourceImportKindStandardLibrary}}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 1, Benchmarks: 0, FuzzTargets: 0, Symbols: []projectstandards.SourceDeclaration{{Name: identifier("projectStandardSourcePath"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 13, Column: 7, Exported: false, AttestBound: false}, {Name: identifier("ProjectStandardKnowledge"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 18, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("projectStandardBuilder"), Kind: projectstandards.SourceDeclarationKindType, Line: 87, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("identifier"), Receiver: sourceIdentifier("projectStandardBuilder"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 91, Column: 34, Exported: false, AttestBound: false}, {Name: identifier("name"), Receiver: sourceIdentifier("projectStandardBuilder"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 97, Column: 34, Exported: false, AttestBound: false}, {Name: identifier("text"), Receiver: sourceIdentifier("projectStandardBuilder"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 103, Column: 34, Exported: false, AttestBound: false}, {Name: identifier("path"), Receiver: sourceIdentifier("projectStandardBuilder"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 109, Column: 34, Exported: false, AttestBound: false}, {Name: identifier("reason"), Receiver: sourceIdentifier("projectStandardBuilder"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 115, Column: 34, Exported: false, AttestBound: false}, {Name: identifier("boundary"), Receiver: sourceIdentifier("projectStandardBuilder"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 119, Column: 34, Exported: false, AttestBound: false}, {Name: identifier("TestProjectStandardKnowledgeClosesAuthoredMeaning"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 123, Column: 6, Exported: true, AttestBound: false}}},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("temporal")}}, Mediated: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/temporal"), Capability: &projectstandards.PrimitiveCapabilityUse{Package: capability("temporal")}, Selector: sourceIdentifier("InstantFromNanoseconds"), Line: 132, Column: 11}}, Unresolved: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/primitiveproject"), Selector: sourceIdentifier("name"), Line: 23, Column: 19}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/primitiveproject"), Selector: sourceIdentifier("text"), Line: 24, Column: 19}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/primitiveproject"), Selector: sourceIdentifier("path"), Line: 28, Column: 19}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/primitiveproject"), Selector: sourceIdentifier("reason"), Line: 30, Column: 4}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/primitiveproject"), Selector: sourceIdentifier("boundary"), Line: 36, Column: 4}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/primitiveproject"), Selector: sourceIdentifier("identifier"), Line: 49, Column: 9}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/primitiveproject"), Selector: sourceIdentifier("Validate"), Line: 81, Column: 12}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/primitiveproject"), Selector: sourceIdentifier("Parallel"), Line: 124, Column: 2}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/primitiveproject"), Selector: sourceIdentifier("Fatalf"), Line: 128, Column: 3}}},
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
