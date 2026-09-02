// Package controlplanetest_test owns compiler-checked knowledge about package controlplanetest.
package controlplanetest_test

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
		Path:        authorPath("controlplanetest"),
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
		Package: sourcePath("controlplanetest"),
		Files: []projectstandards.SourceFile{
			{
				Path: sourcePath("controlplanetest/doc.go"), Package: sourcePath("controlplanetest"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy},
			},
			{
				Path: sourcePath("controlplanetest/installation.go"), Package: sourcePath("controlplanetest"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{{Path: sourcePath("crypto/ed25519"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("errors"), Kind: projectstandards.SourceImportKindStandardLibrary}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/attest"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/controlplane"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/controlwire"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/core"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/lease"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/receipt"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/temporal"), Kind: projectstandards.SourceImportKindProject}}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0, Symbols: []projectstandards.SourceDeclaration{{Name: identifier("fixtureAccountIdentity"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 17, Column: 2, Exported: false, AttestBound: false}, {Name: identifier("fixtureEntitlementIdentity"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 18, Column: 2, Exported: false, AttestBound: false}, {Name: identifier("fixtureBuildCommit"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 19, Column: 2, Exported: false, AttestBound: false}, {Name: identifier("fixtureIssuedAtNanoseconds"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 20, Column: 2, Exported: false, AttestBound: false}, {Name: identifier("InstallationRequest"), Kind: projectstandards.SourceDeclarationKindType, Line: 25, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("Installation"), Kind: projectstandards.SourceDeclarationKindType, Line: 33, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("Validate"), Receiver: sourceIdentifier("InstallationRequest"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 43, Column: 30, Exported: true, AttestBound: false}, {Name: identifier("seedIsZero"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 53, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("IssueInstallation"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 64, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("fixtureBuild"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 112, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("fixtureCertificateBody"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 131, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("Validate"), Receiver: sourceIdentifier("Installation"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 158, Column: 23, Exported: true, AttestBound: false}}},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("temporal")}}, Mediated: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/temporal"), Capability: &projectstandards.PrimitiveCapabilityUse{Package: capability("temporal")}, Selector: sourceIdentifier("InstantFromNanoseconds"), Line: 148, Column: 13}}, Unresolved: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/controlplanetest"), Selector: sourceIdentifier("Validate"), Line: 44, Column: 12}, {ImportPath: sourcePath("crypto/ed25519"), Selector: sourceIdentifier("NewKeyFromSeed"), Line: 68, Column: 22}, {ImportPath: sourcePath("crypto/ed25519"), Selector: sourceIdentifier("NewKeyFromSeed"), Line: 69, Column: 19}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/controlplanetest"), Selector: sourceIdentifier("Public"), Line: 71, Column: 3}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/controlplanetest"), Selector: sourceIdentifier("IssueInstallationCertificate"), Line: 100, Column: 22}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/controlplanetest"), Selector: sourceIdentifier("Offering"), Line: 151, Column: 14}}},
			},
			{
				Path: sourcePath("controlplanetest/installation_test.go"), Package: sourcePath("controlplanetest"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{{Path: sourcePath("crypto/ed25519"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("errors"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("fmt"), Kind: projectstandards.SourceImportKindStandardLibrary}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/controlplane"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/core"), Kind: projectstandards.SourceImportKindProject}, {Path: sourcePath("testing"), Kind: projectstandards.SourceImportKindStandardLibrary}}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 2, Benchmarks: 0, FuzzTargets: 0, Symbols: []projectstandards.SourceDeclaration{{Name: identifier("fixtureSeed"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 13, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("TestIssueInstallationConstructsOpaqueOfferingsThroughTheRealIssuers"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 24, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("TestIssueInstallationReturnsNeutralForEveryInvalidSeedRelation"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 51, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("controlplaneTestOffering"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 104, Column: 6, Exported: false, AttestBound: false}}},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, Unresolved: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/controlplanetest"), Selector: sourceIdentifier("Parallel"), Line: 25, Column: 2}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/controlplanetest"), Selector: sourceIdentifier("IssueInstallation"), Line: 32, Column: 19}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/controlplanetest"), Selector: sourceIdentifier("Fatalf"), Line: 38, Column: 4}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/controlplanetest"), Selector: sourceIdentifier("Validate"), Line: 40, Column: 13}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/controlplanetest"), Selector: sourceIdentifier("Offering"), Line: 43, Column: 6}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/controlplanetest"), Selector: sourceIdentifier("Run"), Line: 87, Column: 3}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/controlplanetest"), Selector: sourceIdentifier("Helper"), Line: 105, Column: 2}}},
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
