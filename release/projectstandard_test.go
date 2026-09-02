// Package release owns its product policy.
package release_test

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
		Path:        authorPath("release"),
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
	capability := func(value string) primitivecore.PackageIdentity {
		result, err := primitivecore.ParsePackageIdentity(value)
		generatedErr = errors.Join(generatedErr, err)
		return result
	}
	catalog := projectstandards.PackageFileCatalog{
		Package: sourcePath("release"),
		Files: []projectstandards.SourceFile{
			{
				Path: sourcePath("release/advance.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy, UnresolvedSites: 0},
			},
			{
				Path: sourcePath("release/advance_selection_hostile_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 3, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy, UnresolvedSites: 0},
			},
			{
				Path: sourcePath("release/artifact.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 5},
			},
			{
				Path: sourcePath("release/artifact_file_ingress_unix_fuzz_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 1, Benchmarks: 0, FuzzTargets: 1},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 1, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("filestore")}}},
			},
			{
				Path: sourcePath("release/artifact_file_lifecycle_unix_external_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 3, Benchmarks: 2, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 2, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("filestore")}}},
			},
			{
				Path: sourcePath("release/artifact_inspection.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 7, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("filestore")}, {Package: capability("process")}}},
			},
			{
				Path: sourcePath("release/artifact_inspection_external_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 3, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 4, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("filestore")}, {Package: capability("hostfacts")}, {Package: capability("process")}}},
			},
			{
				Path: sourcePath("release/artifact_inspection_internal_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 7, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectDirectObserved, UnresolvedSites: 0, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("filestore")}}},
			},
			{
				Path: sourcePath("release/assessment.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy, UnresolvedSites: 0},
			},
			{
				Path: sourcePath("release/benchmark_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 3, Benchmarks: 3, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 7},
			},
			{
				Path: sourcePath("release/boundary_hostile_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 16, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 25, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("temporal")}}},
			},
			{
				Path: sourcePath("release/build_command_assignments_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 3, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy, UnresolvedSites: 0},
			},
			{
				Path: sourcePath("release/build_controlled_environment_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 5, Benchmarks: 0, FuzzTargets: 1},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy, UnresolvedSites: 0},
			},
			{
				Path: sourcePath("release/build_dependencies.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 1},
			},
			{
				Path: sourcePath("release/build_dependencies_internal_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 8, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 1},
			},
			{
				Path: sourcePath("release/build_identity.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy, UnresolvedSites: 0},
			},
			{
				Path: sourcePath("release/build_identity_internal_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 4, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectDirectObserved, UnresolvedSites: 0, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("filestore")}}},
			},
			{
				Path: sourcePath("release/build_lifecycle_hostile_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 1, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 1, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("temporal")}, {Package: capability("process")}}},
			},
			{
				Path: sourcePath("release/build_plan.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy, UnresolvedSites: 0},
			},
			{
				Path: sourcePath("release/build_plan_hostile_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 6, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy, UnresolvedSites: 0},
			},
			{
				Path: sourcePath("release/build_process.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 1, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("process")}}},
			},
			{
				Path: sourcePath("release/build_process_hostile_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 2, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 2, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("temporal")}, {Package: capability("process")}}},
			},
			{
				Path: sourcePath("release/build_process_internal_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 1, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 1},
			},
			{
				Path: sourcePath("release/build_tags_hostile_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 6, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy, UnresolvedSites: 0},
			},
			{
				Path: sourcePath("release/build_tools.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 3, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("filestore")}, {Package: capability("process")}}},
			},
			{
				Path: sourcePath("release/build_tools_hostile_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 4, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectDirectObserved, UnresolvedSites: 0, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("filestore")}}},
			},
			{
				Path: sourcePath("release/build_tools_internal_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 1, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy, UnresolvedSites: 0},
			},
			{
				Path: sourcePath("release/build_tools_live_external_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 1, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 2, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("hostfacts")}, {Package: capability("temporal")}, {Package: capability("process")}}},
			},
			{
				Path: sourcePath("release/closure_hostile_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 9, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 2, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("temporal")}}},
			},
			{
				Path: sourcePath("release/dependency_observation.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 3, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("process")}}},
			},
			{
				Path: sourcePath("release/dependency_observation_external_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 4, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectDirectObserved, UnresolvedSites: 0, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("hostfacts")}, {Package: capability("process")}}},
			},
			{
				Path: sourcePath("release/dependency_observation_internal_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 2, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 1},
			},
			{
				Path: sourcePath("release/doc.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy, UnresolvedSites: 0},
			},
			{
				Path: sourcePath("release/doctrine_contract.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy, UnresolvedSites: 0},
			},
			{
				Path: sourcePath("release/domain.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy, UnresolvedSites: 0},
			},
			{
				Path: sourcePath("release/enums.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 3},
			},
			{
				Path: sourcePath("release/errors.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 1},
			},
			{
				Path: sourcePath("release/external_door_fuzz_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 2, Benchmarks: 0, FuzzTargets: 2},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectMediated, UnresolvedSites: 0, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("keygen")}}},
			},
			{
				Path: sourcePath("release/fuzz_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 1, Benchmarks: 0, FuzzTargets: 1},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 1},
			},
			{
				Path: sourcePath("release/internal_contract_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 5, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectDirectObserved, UnresolvedSites: 0, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("filestore")}}},
			},
			{
				Path: sourcePath("release/json_contract.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy, UnresolvedSites: 0},
			},
			{
				Path: sourcePath("release/latest.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 6},
			},
			{
				Path: sourcePath("release/manifest.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 5},
			},
			{
				Path: sourcePath("release/manifest_document_digest_hostile_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 2, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 4},
			},
			{
				Path: sourcePath("release/manifest_signer_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 1, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy, UnresolvedSites: 0},
			},
			{
				Path: sourcePath("release/material.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 6, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("keygen")}}},
			},
			{
				Path: sourcePath("release/material_external_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 6, Benchmarks: 0, FuzzTargets: 1},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 4, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("keygen")}}},
			},
			{
				Path: sourcePath("release/offering_external_fixture_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 1},
			},
			{
				Path: sourcePath("release/offering_fixture_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 1},
			},
			{
				Path: sourcePath("release/project_version.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy, UnresolvedSites: 0},
			},
			{
				Path: sourcePath("release/project_version_hostile_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 2, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy, UnresolvedSites: 0},
			},
			{
				Path: sourcePath("release/projectstandard_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy, UnresolvedSites: 0},
			},
			{
				Path: sourcePath("release/publication_metadata.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 9},
			},
			{
				Path: sourcePath("release/publication_metadata_hostile_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 5, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 7},
			},
			{
				Path: sourcePath("release/publication_metadata_internal_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 5, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 7, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("filestore")}}},
			},
			{
				Path: sourcePath("release/publication_role.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 1},
			},
			{
				Path: sourcePath("release/publication_role_hostile_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 1, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy, UnresolvedSites: 0},
			},
			{
				Path: sourcePath("release/release_hostile_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 5, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 3, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("temporal")}}},
			},
			{
				Path: sourcePath("release/repository.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 1, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("filestore")}, {Package: capability("process")}}},
			},
			{
				Path: sourcePath("release/repository_external_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 3, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 2, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("filestore")}, {Package: capability("hostfacts")}, {Package: capability("temporal")}, {Package: capability("process")}}},
			},
			{
				Path: sourcePath("release/repository_internal_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 5, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 1, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("process")}}},
			},
			{
				Path: sourcePath("release/selection.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy, UnresolvedSites: 0},
			},
			{
				Path: sourcePath("release/test_support_test.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 1, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("temporal")}}},
			},
			{
				Path: sourcePath("release/toolchain.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy, UnresolvedSites: 0},
			},
			{
				Path: sourcePath("release/validation_witnesses.go"), Package: sourcePath("release"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy, UnresolvedSites: 0},
			},
		},
	}
	if generatedErr != nil {
		return projectstandards.PackageFileCatalog{}, generatedErr
	}
	if err := catalog.Validate(); err != nil {
		return projectstandards.PackageFileCatalog{}, err
	}
	return catalog, nil
}
