// Package temporal owns its product policy.
package temporal_test

import (
	"errors"

	primitivecore "github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/projectstandards"
)

const packageStandardAuthorshipPending = false

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
		Path:            authorPath("temporal"),
		AuthorTitle:     authorName("Temporal mechanics"),
		AuthorProblem:   authorText("Go exposes wall time, monotonic elapsed time, durations, deadlines, timers, and persistence projections through related but meaningfully different representations whose unchecked use can overflow, lose precision, hide rollback, or leak scheduler timing into policy."),
		AuthorPurpose:   authorText("Provide exact validated temporal values, checked arithmetic, canonical projections, wall-plus-monotonic observations, intervals, and bounded context-aware deadline, wait, and ticker effects."),
		AuthorAudience:  authorText("Go packages that need deterministic temporal policy inputs or a small owned boundary over the standard library time and context mechanisms."),
		AuthorValue:     authorText("Callers keep duration and expiry policy while sharing one compiler-visible implementation of units, bounds, ordering, overflow behavior, persistence shape, cancellation, and timer ownership."),
		AuthorSteward:   authorName("Primitive temporal"),
		AuthorSubstrate: authorName("Go time and context"),
		AuthorRuntime:   authorName("Typed values and bounded effects"),
		AuthorRemoval:   authorText("Removing this package requires replacing every temporal value, wire projection, checked arithmetic rule, observation boundary, deadline constructor, wait, and ticker with another single validated owner; raw time values and duplicated clock helpers are not an admissible replacement."),
		Created:         created, Changed: changed,
		AuthorReasons: []projectstandards.Reason{
			authorReason("Separate wall and elapsed truth", "A serialized wall instant and an in-process monotonic observation answer different questions; preserving the distinction prevents rollback and clock adjustment from corrupting elapsed work."),
			authorReason("Make units and overflow visible", "Named values and checked operations stop seconds, milliseconds, and nanoseconds from becoming interchangeable integers and reject arithmetic that cannot be represented exactly."),
			authorReason("Own timer lifetimes", "Waits, deadlines, and tickers must bind cancellation, stop behavior, and resource ownership instead of leaving callers to reproduce subtle standard-library cleanup rules."),
		},
		AuthorOwns: []projectstandards.Boundary{
			authorBoundary("Temporal value mechanics", "Validated instants, non-negative durations, aggregate durations, intervals, comparisons, checked arithmetic, and canonical text, numeric, and persistence projections."),
			authorBoundary("Clock observations", "Acquisition and validation of one wall-plus-monotonic observation and computation of elapsed duration while the monotonic component remains available."),
			authorBoundary("Bounded temporal effects", "Context-derived timeouts and deadlines, cancellation-aware waits, and explicitly stopped tickers implemented through Go's time and context packages."),
		},
		AuthorDoesNotOwn: []projectstandards.Boundary{
			authorBoundary("Product timing policy", "Consumers choose TTLs, retry eligibility, grace periods, cadence, expiry semantics, and the action taken when a deadline or interval is reached."),
			authorBoundary("Durable authority", "A wall observation is not a lease, replay high-water mark, trusted timestamp, or acceptance receipt; the package only supplies temporal mechanics used by those owners."),
			authorBoundary("Presentation policy", "Human time zones, locale formatting, relative-time prose, and product display precision remain with the presenting product."),
		},
		AuthorUsage: []projectstandards.Usage{{
			ID: authorIdentifier("observe-and-apply-time"), Title: authorName("Observe and apply time"), Audience: authorText("Package authors implementing deterministic policy around current time or elapsed work."),
			Goal:    authorText("Acquire temporal facts once at the effect boundary and pass validated typed values into policy without ambient clock reads."),
			Outcome: authorText("Equal policy inputs produce equal decisions, elapsed work uses monotonic evidence, and every wait or deadline has an explicit cancellation and cleanup path."),
			Steps: []projectstandards.UsageStep{
				{Title: authorName("Observe the boundary"), Detail: authorText("Call Observe once where the operation enters the real world and retain its validated wall and monotonic facts."), Reference: &projectstandards.CodeReference{Path: authorPath("temporal/observation.go")}},
				{Title: authorName("Compute with typed values"), Detail: authorText("Construct and validate instants, durations, aggregates, or intervals, then use checked comparison and arithmetic instead of raw integer or time operations."), Reference: &projectstandards.CodeReference{Path: authorPath("temporal/instant.go")}},
				{Title: authorName("Execute bounded waiting"), Detail: authorText("When the operation must block, use the validated timeout, deadline, wait, or ticker request and preserve the returned cancellation identity."), Reference: &projectstandards.CodeReference{Path: authorPath("temporal/effects.go")}},
			},
		}},
		AuthorFeatures: []projectstandards.Feature{
			{
				ID: authorIdentifier("exact-temporal-values"), Title: authorName("Exact temporal values"),
				Technical:        authorText("Private validated representations expose instants, durations, aggregates, and intervals through checked constructors, arithmetic, comparisons, and canonical projections."),
				Benefit:          authorText("Callers cannot silently confuse units, overflow arithmetic, admit invalid zero states, or invent a second persistence encoding."),
				ProofRequirement: authorText("Tests must exhaust zero and signed boundaries, arithmetic overflow, precision projections, canonical round trips, malformed external encodings, and interval ordering."),
				Delivery:         projectstandards.DeliveryDelivered,
			},
			{
				ID: authorIdentifier("owned-temporal-effects"), Title: authorName("Owned temporal effects"),
				Technical:        authorText("Validated observations, timeouts, deadlines, waits, and tickers directly contain Go's standard time and context effects behind explicit request and cleanup contracts."),
				Benefit:          authorText("Products receive real machine timing behavior without duplicating clocks, timer draining, cancellation propagation, or ticker lifetime rules."),
				ProofRequirement: authorText("Tests must prove cancellation identity, parent deadline precedence, immediate terminal contexts, timeout equality, ticker stop ownership, monotonic elapsed measurement, and absence of sleep-based synchronization."),
				Delivery:         projectstandards.DeliveryDelivered,
			},
		},
		AuthorAssurance: projectstandards.Assurance{
			Policy: projectstandards.AssuranceControl{
				Stage: projectstandards.AssuranceStagePolicy, Authority: projectstandards.AssuranceAuthorityProduct,
				Statement:  authorText("Consumers own timing thresholds and outcomes; temporal owns only reusable value, arithmetic, observation, and blocking mechanics."),
				References: []projectstandards.CodeReference{{Path: authorPath("temporal/doc.go")}, {Path: authorPath("temporal/contracts.go")}}, SurfaceIDs: []projectstandards.Identifier{authorIdentifier("temporal-policy-boundary")},
			},
			Validation: projectstandards.AssuranceControl{
				Stage: projectstandards.AssuranceStageValidation, Authority: projectstandards.AssuranceAuthorityCore,
				Statement:  authorText("Each temporal type owns its admitted domain and every public constructor, arithmetic operation, projection, and effect request validates before crossing its boundary."),
				References: []projectstandards.CodeReference{{Path: authorPath("temporal/instant.go")}, {Path: authorPath("temporal/duration.go")}, {Path: authorPath("temporal/interval.go")}}, SurfaceIDs: []projectstandards.Identifier{authorIdentifier("temporal-owned-validation")},
			},
			Effects: projectstandards.AssuranceControl{
				Stage: projectstandards.AssuranceStageEffects, Authority: projectstandards.AssuranceAuthorityPrimitive,
				Statement:  authorText("Only observation and effect constructors touch the ambient clock, contexts, timers, and tickers; typed value operations remain deterministic policy mechanics."),
				References: []projectstandards.CodeReference{{Path: authorPath("temporal/observation.go")}, {Path: authorPath("temporal/effects.go")}}, SurfaceIDs: []projectstandards.Identifier{authorIdentifier("temporal-effect-ownership")},
			},
			Proof: projectstandards.AssuranceControl{
				Stage: projectstandards.AssuranceStageProof, Authority: projectstandards.AssuranceAuthorityIndependent,
				Statement:  authorText("Hostile external tests, fuzz targets, benchmarks, and architecture ratchets prove numeric closure, canonical ingress, checked bounds, cancellation, monotonic elapsed behavior, and stable allocation surfaces."),
				References: []projectstandards.CodeReference{{Path: authorPath("temporal/numeric_external_test.go")}, {Path: authorPath("temporal/effects_external_test.go")}, {Path: authorPath("temporal/domain_closure_internal_test.go")}, {Path: authorPath("temporal/benchmark_external_test.go")}}, SurfaceIDs: []projectstandards.Identifier{authorIdentifier("temporal-hostile-proof")},
			},
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
		Package: sourcePath("temporal"),
		Files: []projectstandards.SourceFile{
			{
				Path: sourcePath("temporal/aggregate.go"), Package: sourcePath("temporal"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 10},
			},
			{
				Path: sourcePath("temporal/aggregate_persistence_external_test.go"), Package: sourcePath("temporal"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 10, Benchmarks: 0, FuzzTargets: 3},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 17, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("temporal")}}},
			},
			{
				Path: sourcePath("temporal/architecture_test.go"), Package: sourcePath("temporal"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 4, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 2, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("filestore")}}},
			},
			{
				Path: sourcePath("temporal/benchmark_external_test.go"), Package: sourcePath("temporal"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 3, Benchmarks: 3, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectMediated, UnresolvedSites: 0, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("temporal")}}},
			},
			{
				Path: sourcePath("temporal/contracts.go"), Package: sourcePath("temporal"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy, UnresolvedSites: 0},
			},
			{
				Path: sourcePath("temporal/doc.go"), Package: sourcePath("temporal"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy, UnresolvedSites: 0},
			},
			{
				Path: sourcePath("temporal/domain_closure_internal_test.go"), Package: sourcePath("temporal"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 4, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy, UnresolvedSites: 0},
			},
			{
				Path: sourcePath("temporal/duration.go"), Package: sourcePath("temporal"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 2},
			},
			{
				Path: sourcePath("temporal/effects.go"), Package: sourcePath("temporal"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectImplementation, UnresolvedSites: 0, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("temporal")}}},
			},
			{
				Path: sourcePath("temporal/effects_external_test.go"), Package: sourcePath("temporal"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 6, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 8, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("temporal")}}},
			},
			{
				Path: sourcePath("temporal/errors.go"), Package: sourcePath("temporal"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy, UnresolvedSites: 0},
			},
			{
				Path: sourcePath("temporal/external_ingress_inventory_test.go"), Package: sourcePath("temporal"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy, UnresolvedSites: 0},
			},
			{
				Path: sourcePath("temporal/instant.go"), Package: sourcePath("temporal"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 1},
			},
			{
				Path: sourcePath("temporal/instant_text_external_test.go"), Package: sourcePath("temporal"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 3, Benchmarks: 0, FuzzTargets: 1},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectMediated, UnresolvedSites: 0, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("temporal")}}},
			},
			{
				Path: sourcePath("temporal/interval.go"), Package: sourcePath("temporal"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy, UnresolvedSites: 0},
			},
			{
				Path: sourcePath("temporal/numeric.go"), Package: sourcePath("temporal"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy, UnresolvedSites: 0},
			},
			{
				Path: sourcePath("temporal/numeric_external_test.go"), Package: sourcePath("temporal"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 11, Benchmarks: 0, FuzzTargets: 2},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 7, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("temporal")}}},
			},
			{
				Path: sourcePath("temporal/observation.go"), Package: sourcePath("temporal"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectImplementation, UnresolvedSites: 0, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("temporal")}}},
			},
			{
				Path: sourcePath("temporal/observation_interval_test.go"), Package: sourcePath("temporal"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 7, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 1, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("temporal")}}},
			},
			{
				Path: sourcePath("temporal/persistence.go"), Package: sourcePath("temporal"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindProduction, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 2},
			},
			{
				Path: sourcePath("temporal/projectstandard_test.go"), Package: sourcePath("temporal"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 0, Benchmarks: 0, FuzzTargets: 0},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectPurePolicy, UnresolvedSites: 0},
			},
			{
				Path: sourcePath("temporal/value_external_test.go"), Package: sourcePath("temporal"),
				Language: projectstandards.SourceLanguageGo, Kind: projectstandards.SourceFileKindTest, Generated: false,
				Declarations: projectstandards.SourceFileDeclarations{TestDeclarations: 9, Benchmarks: 0, FuzzTargets: 1},
				Effects:      projectstandards.SourceFileEffects{Posture: projectstandards.PrimitiveEffectUnresolved, UnresolvedSites: 1, Capabilities: []projectstandards.PrimitiveCapabilityUse{{Package: capability("temporal")}}},
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
