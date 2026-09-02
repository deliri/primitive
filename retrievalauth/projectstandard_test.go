// Package retrievalauth_test owns compiler-checked knowledge about package retrievalauth.
package retrievalauth_test

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
		Path:        authorPath("retrievalauth"),
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
		Package: sourcePath("retrievalauth"),
		Files: []projectstandards.SourceFile{
			{
				Path: sourcePath("retrievalauth/architecture_test.go"), Package: sourcePath("retrievalauth"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{{Path: sourcePath("embed"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("go/ast"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("go/parser"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("go/token"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("slices"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("sort"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("strings"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("testing"), Kind: projectstandards.SourceImportKindStandardLibrary}}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 1, Benchmarks: 0, FuzzTargets: 0, Symbols: []projectstandards.SourceDeclaration{{Name: identifier("retrievalAuthContractSources"), Kind: projectstandards.SourceDeclarationKindVariable, Line: 15, Column: 5, Exported: false, AttestBound: false}, {Name: identifier("protocolFact"), Kind: projectstandards.SourceDeclarationKindType, Line: 18, Column: 2, Exported: false, AttestBound: false}, {Name: identifier("capabilityWrapper"), Kind: projectstandards.SourceDeclarationKindType, Line: 19, Column: 2, Exported: false, AttestBound: false}, {Name: identifier("retrievalAuthContractInventory"), Kind: projectstandards.SourceDeclarationKindType, Line: 22, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("TestRetrievalAuthDataFlowStructInventoryRatchet"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 31, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("retrievalAuthProductionStructNames"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 41, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("retrievalAuthClassifiedStructNames"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 77, Column: 6, Exported: false, AttestBound: false}}},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, Unresolved: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("Parallel"), Line: 32, Column: 2}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("Fatalf"), Line: 37, Column: 3}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("Helper"), Line: 42, Column: 2}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("ReadDir"), Line: 44, Column: 18}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("IsDir"), Line: 51, Column: 6}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("Name"), Line: 51, Column: 42}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("ReadFile"), Line: 54, Column: 22}, {ImportPath: sourcePath("go/parser"), Selector: sourceIdentifier("ParseFile"), Line: 58, Column: 21}, {ImportPath: sourcePath("go/parser"), Selector: sourceIdentifier("ParseFile"), Line: 84, Column: 15}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("Fatal"), Line: 109, Column: 2}}},
			},
			{
				Path: sourcePath("retrievalauth/doc.go"), Package: sourcePath("retrievalauth"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy},
			},
			{
				Path: sourcePath("retrievalauth/errors.go"), Package: sourcePath("retrievalauth"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{{Path: sourcePath("errors"), Kind: projectstandards.SourceImportKindStandardLibrary}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/core"), Kind: projectstandards.SourceImportKindProject}}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0, Symbols: []projectstandards.SourceDeclaration{{Name: identifier("contractError"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 9, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("jsonError"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 13, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("bindingError"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 17, Column: 6, Exported: false, AttestBound: false}}},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy},
			},
			{
				Path: sourcePath("retrievalauth/external_document_fuzz_test.go"), Package: sourcePath("retrievalauth"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{{Path: sourcePath("bytes"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("errors"), Kind: projectstandards.SourceImportKindStandardLibrary}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/core"), Kind: projectstandards.SourceImportKindProject}, {Path: sourcePath("testing"), Kind: projectstandards.SourceImportKindStandardLibrary}}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 1, Benchmarks: 0, FuzzTargets: 1, Symbols: []projectstandards.SourceDeclaration{{Name: identifier("FuzzRequestDocumentExternalDecoderAndVerifier"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 11, Column: 6, Exported: true, AttestBound: false}}},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, Unresolved: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("newRetrievalAuthFixture"), Line: 12, Column: 13}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("MarshalJSON"), Line: 14, Column: 20}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("Fatalf"), Line: 16, Column: 3}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("Add"), Line: 28, Column: 3}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("Fuzz"), Line: 30, Column: 2}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("UnmarshalJSON"), Line: 32, Column: 16}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("Validate"), Line: 41, Column: 13}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("Verify"), Line: 52, Column: 26}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("retrievalAuthServer"), Line: 52, Column: 75}}},
			},
			{
				Path: sourcePath("retrievalauth/request.go"), Package: sourcePath("retrievalauth"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{{Path: sourcePath("encoding/json/v2"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("errors"), Kind: projectstandards.SourceImportKindStandardLibrary}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/attest"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/controlplane"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/controlwire"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/core"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/retrieval"), Kind: projectstandards.SourceImportKindProject}}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0, Symbols: []projectstandards.SourceDeclaration{{Name: identifier("RequestDocumentJSONMaximumBytes"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 15, Column: 2, Exported: true, AttestBound: false}, {Name: identifier("RequestDocument"), Kind: projectstandards.SourceDeclarationKindType, Line: 20, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("Validate"), Receiver: sourceIdentifier("RequestDocument"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 25, Column: 26, Exported: true, AttestBound: false}, {Name: identifier("ControlRoute"), Receiver: sourceIdentifier("RequestDocument"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 40, Column: 26, Exported: true, AttestBound: false}, {Name: identifier("ControlRevision"), Receiver: sourceIdentifier("RequestDocument"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 47, Column: 26, Exported: true, AttestBound: false}, {Name: identifier("ControlNonce"), Receiver: sourceIdentifier("RequestDocument"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 52, Column: 26, Exported: true, AttestBound: false}, {Name: identifier("ControlRequestBodyLimit"), Receiver: sourceIdentifier("RequestDocument"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 56, Column: 24, Exported: true, AttestBound: false}, {Name: identifier("RequestAssembly"), Kind: projectstandards.SourceDeclarationKindType, Line: 60, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("Validate"), Receiver: sourceIdentifier("RequestAssembly"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 65, Column: 26, Exported: true, AttestBound: false}, {Name: identifier("Assemble"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 69, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("MarshalJSON"), Receiver: sourceIdentifier("RequestDocument"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 76, Column: 26, Exported: true, AttestBound: false}, {Name: identifier("UnmarshalJSON"), Receiver: sourceIdentifier("RequestDocument"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 88, Column: 27, Exported: true, AttestBound: false}, {Name: identifier("Verification"), Kind: projectstandards.SourceDeclarationKindType, Line: 111, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("Validate"), Receiver: sourceIdentifier("Verification"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 116, Column: 23, Exported: true, AttestBound: false}, {Name: identifier("Verified"), Kind: projectstandards.SourceDeclarationKindType, Line: 123, Column: 6, Exported: true, AttestBound: true}, {Name: identifier("Verify"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 129, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("Validate"), Receiver: sourceIdentifier("Verified"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 155, Column: 19, Exported: true, AttestBound: false}, {Name: identifier("Document"), Receiver: sourceIdentifier("Verified"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 162, Column: 19, Exported: true, AttestBound: false}}},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, Unresolved: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("Validate"), Line: 26, Column: 24}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("contractError"), Line: 27, Column: 10}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("bindingError"), Line: 30, Column: 10}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("Scope"), Line: 32, Column: 16}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("Offering"), Line: 42, Column: 3}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("jsonError"), Line: 78, Column: 15}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("VerifyInstallationCertificate"), Line: 133, Column: 22}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("DeviceKeys"), Line: 139, Column: 21}}},
			},
			{
				Path: sourcePath("retrievalauth/request_hostile_test.go"), Package: sourcePath("retrievalauth"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{{Path: sourcePath("bytes"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("crypto/ed25519"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("errors"), Kind: projectstandards.SourceImportKindStandardLibrary}, {Path: sourcePath("fmt"), Kind: projectstandards.SourceImportKindStandardLibrary}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/attest"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/controlplane"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/controlplanetest"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/controlwire"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/core"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/retrieval"), Kind: projectstandards.SourceImportKindProject}, {Path: sourcePath("testing"), Kind: projectstandards.SourceImportKindStandardLibrary}}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 3, Benchmarks: 0, FuzzTargets: 0, Symbols: []projectstandards.SourceDeclaration{{Name: identifier("retrievalAuthFixtureChit"), Kind: projectstandards.SourceDeclarationKindConstant, Line: 18, Column: 7, Exported: false, AttestBound: false}, {Name: identifier("retrievalAuthFixtureRequest"), Kind: projectstandards.SourceDeclarationKindType, Line: 20, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("retrievalAuthFixture"), Kind: projectstandards.SourceDeclarationKindType, Line: 27, Column: 6, Exported: false, AttestBound: true}, {Name: identifier("retrievalAuthJSONCase"), Kind: projectstandards.SourceDeclarationKindType, Line: 35, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("TestRetrievalAuthAssemblyLayerTriad"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 40, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("TestRetrievalAuthVerificationLayerTriad"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 131, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("TestRetrievalAuthDocumentJSONLayerTriad"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 220, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("marshalReorderedRetrievalAuthDocument"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 285, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("retrievalAuthPadJSON"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 298, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("retrievalAuthHostileJSONCases"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 305, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("newRetrievalAuthFixture"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 331, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("retrievalAuthOffering"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 381, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("retrievalAuthPayload"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 390, Column: 6, Exported: false, AttestBound: false}, {Name: identifier("retrievalAuthSeed"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 421, Column: 6, Exported: false, AttestBound: false}}},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, Unresolved: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("Parallel"), Line: 41, Column: 2}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("Run"), Line: 43, Column: 2}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("ControlRoute"), Line: 60, Column: 23}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("Offering"), Line: 61, Column: 26}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("Family"), Line: 62, Column: 5}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("ControlNonce"), Line: 63, Column: 5}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("Fatalf"), Line: 64, Column: 5}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("Assemble"), Line: 67, Column: 19}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("UnmarshalJSON"), Line: 82, Column: 13}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("mutate"), Line: 112, Column: 5}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("Verify"), Line: 150, Column: 24}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("retrievalAuthServer"), Line: 150, Column: 80}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("Document"), Line: 151, Column: 24}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("MarshalJSON"), Line: 224, Column: 23}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("Helper"), Line: 286, Column: 2}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("IsValid"), Line: 337, Column: 6}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("Validate"), Line: 384, Column: 12}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("Scope"), Line: 396, Column: 16}}},
			},
			{
				Path: sourcePath("retrievalauth/response.go"), Package: sourcePath("retrievalauth"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{{Path: sourcePath("crypto"), Kind: projectstandards.SourceImportKindStandardLibrary}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/controlplane"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/controlwire"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/core"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/retrieval"), Kind: projectstandards.SourceImportKindProject}}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0, Symbols: []projectstandards.SourceDeclaration{{Name: identifier("ResponseIssuance"), Kind: projectstandards.SourceDeclarationKindType, Line: 12, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("ResponseVerification"), Kind: projectstandards.SourceDeclarationKindType, Line: 20, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("Validate"), Receiver: sourceIdentifier("ResponseIssuance"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 26, Column: 27, Exported: true, AttestBound: false}, {Name: identifier("IssueResponse"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 30, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("responseIssuance"), Receiver: sourceIdentifier("ResponseIssuance"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 36, Column: 27, Exported: false, AttestBound: false}, {Name: identifier("Validate"), Receiver: sourceIdentifier("ResponseVerification"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 42, Column: 31, Exported: true, AttestBound: false}, {Name: identifier("VerifyResponse"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 46, Column: 6, Exported: true, AttestBound: false}, {Name: identifier("responseVerification"), Receiver: sourceIdentifier("ResponseVerification"), Kind: projectstandards.SourceDeclarationKindMethod, Line: 52, Column: 31, Exported: false, AttestBound: false}}},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, Unresolved: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("ValidateForFamily"), Line: 27, Column: 9}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("responseIssuance"), Line: 27, Column: 9}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("responseVerification"), Line: 43, Column: 9}}},
			},
			{
				Path: sourcePath("retrievalauth/response_hostile_test.go"), Package: sourcePath("retrievalauth"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{{Path: sourcePath("errors"), Kind: projectstandards.SourceImportKindStandardLibrary}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/core"), Kind: projectstandards.SourceImportKindProject}, {Path: sourcePath("testing"), Kind: projectstandards.SourceImportKindStandardLibrary}}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 1, Benchmarks: 0, FuzzTargets: 0, Symbols: []projectstandards.SourceDeclaration{{Name: identifier("TestRetrievalResponseBoundaryRefusesEveryNeutralInput"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 10, Column: 6, Exported: true, AttestBound: false}}},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, Unresolved: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("Parallel"), Line: 11, Column: 2}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("Validate"), Line: 14, Column: 12}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("Fatalf"), Line: 15, Column: 3}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("IssueResponse"), Line: 17, Column: 21}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("VerifyResponse"), Line: 25, Column: 19}}},
			},
			{
				Path: sourcePath("retrievalauth/side_capability_test.go"), Package: sourcePath("retrievalauth"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Imports:      &projectstandards.SourceFileImports{Values: []projectstandards.SourceImport{{ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/attest"), Kind: projectstandards.SourceImportKindProject}, {ProjectModule: projectModule("github.com/deliri/primitive/v2026"), Path: sourcePath("github.com/deliri/primitive/v2026/controlplane"), Kind: projectstandards.SourceImportKindProject}, {Path: sourcePath("testing"), Kind: projectstandards.SourceImportKindStandardLibrary}}},
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0, Symbols: []projectstandards.SourceDeclaration{{Name: identifier("retrievalAuthServer"), Kind: projectstandards.SourceDeclarationKindFunction, Line: 10, Column: 6, Exported: false, AttestBound: false}}},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, Unresolved: []projectstandards.SourceEffectSite{{ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("Helper"), Line: 11, Column: 2}, {ImportPath: sourcePath("github.com/deliri/primitive/v2026/retrievalauth"), Selector: sourceIdentifier("Fatalf"), Line: 14, Column: 3}}},
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
