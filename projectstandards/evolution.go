package projectstandards

import (
	"errors"
	"math"

	"github.com/deliri/primitive/v2026/core"
)

// CountChange is a signed change in one bounded Project standards count. Positive values
// mean the fact increased in the later snapshot; negative values mean it
// decreased. Its unit is a count, not bytes or time.
type CountChange int64

// NewCountChange derives the signed movement between two exact uint32 source
// counts. The subtraction is widened before execution, so neither direction
// can wrap.
func NewCountChange(before, after uint32) CountChange {
	return CountChange(int64(after) - int64(before))
}

func (c CountChange) Validate() error {
	if c < -math.MaxUint32 || c > math.MaxUint32 {
		return contractError(errors.New("project standards count change exceeds the uint32 source domain"))
	}
	return nil
}

type CoverageChange struct {
	Before *uint16
	After  *uint16
}

func (c CoverageChange) Validate() error {
	if c.Before != nil && *c.Before > 10_000 || c.After != nil && *c.After > 10_000 {
		return contractError(errors.New("project standards coverage change exceeds basis-point bounds"))
	}
	return nil
}

// InventoryChange makes the exact compiler-visible code-shape movement
// between two compatible package Project standards snapshots explicit.
type InventoryChange struct {
	GoPackages          CountChange
	JavaScriptUnits     CountChange
	Files               CountChange
	TestFiles           CountChange
	Documents           CountChange
	TestDeclarations    CountChange
	Benchmarks          CountChange
	FuzzTargets         CountChange
	CoverageBasisPoints CoverageChange
}

func (c InventoryChange) Validate() error {
	return contractJoin(
		c.GoPackages.Validate(), c.JavaScriptUnits.Validate(), c.Files.Validate(), c.TestFiles.Validate(),
		c.Documents.Validate(), c.TestDeclarations.Validate(), c.Benchmarks.Validate(), c.FuzzTargets.Validate(),
		c.CoverageBasisPoints.Validate(),
	)
}

// EvidenceChange exposes how much more or less independently accepted proof
// exists after the package change. It does not reinterpret non-accepting work
// as success.
type EvidenceChange struct {
	Surfaces           CountChange
	Requests           CountChange
	Admissions         CountChange
	Refusals           CountChange
	Observations       CountChange
	Current            CountChange
	Stale              CountChange
	Selections         CountChange
	Passed             CountChange
	Failed             CountChange
	NonAccepting       CountChange
	Benchmarks         CountChange
	Artifacts          CountChange
	ComplexityCaptures CountChange
}

func (c EvidenceChange) Validate() error {
	return contractJoin(
		c.Surfaces.Validate(), c.Requests.Validate(), c.Admissions.Validate(), c.Refusals.Validate(),
		c.Observations.Validate(), c.Current.Validate(), c.Stale.Validate(), c.Selections.Validate(),
		c.Passed.Validate(), c.Failed.Validate(), c.NonAccepting.Validate(), c.Benchmarks.Validate(),
		c.Artifacts.Validate(), c.ComplexityCaptures.Validate(),
	)
}

// SourceUsageChange preserves absence as a first-class fact. A missing first
// analysis followed by a present second analysis is a visibility gain, not an
// invented code improvement.
type SourceUsageChange struct {
	BeforeAvailable          bool
	AfterAvailable           bool
	Declarations             CountChange
	ProductionReferenced     CountChange
	RuntimeEntryPoints       CountChange
	UnresolvedDeclarations   CountChange
	TestReferencedOnly       CountChange
	NoReferenceObserved      CountChange
	ObservedConsumerPackages CountChange
}

func (c SourceUsageChange) Validate() error {
	return contractJoin(
		c.Declarations.Validate(), c.ProductionReferenced.Validate(), c.RuntimeEntryPoints.Validate(),
		c.UnresolvedDeclarations.Validate(), c.TestReferencedOnly.Validate(), c.NoReferenceObserved.Validate(),
		c.ObservedConsumerPackages.Validate(),
	)
}

// PackageEvolution is the typed result that falls out of taking a package
// Project standards snapshot before and after a change. It retains source coordinates and
// reports only independently derivable movement.
type PackageEvolution struct {
	Package                Identifier
	Path                   SourcePath
	BeforeRevision         core.BuildCommit
	AfterRevision          core.BuildCommit
	Inventory              InventoryChange
	Evidence               EvidenceChange
	SourceUsage            SourceUsageChange
	NewReviewCandidates    []FunctionUsage
	FormerReviewCandidates []FunctionUsage
}

func (e PackageEvolution) Validate() error {
	if err := contractJoin(e.Package.Validate(), e.Path.Validate(), e.BeforeRevision.Validate(), e.AfterRevision.Validate(), e.Inventory.Validate(), e.Evidence.Validate(), e.SourceUsage.Validate()); err != nil {
		return err
	}
	if len(e.NewReviewCandidates) > FunctionUsageMaximum || len(e.FormerReviewCandidates) > FunctionUsageMaximum {
		return contractError(errors.New("project standards package evolution review candidates exceed their bound"))
	}
	if err := validateFunctionUsages(e.Path, e.NewReviewCandidates); err != nil {
		return err
	}
	return validateFunctionUsages(e.Path, e.FormerReviewCandidates)
}

// ComparePackageSnapshots derives a compatible before/after result. It
// refuses package identity drift and analyzer-generation drift so a tooling
// change cannot masquerade as a source-code insight.
func ComparePackageSnapshots(before, after PackageSnapshot) (PackageEvolution, error) {
	if err := contractJoin(before.Validate(), after.Validate()); err != nil {
		return PackageEvolution{}, err
	}
	if !packageSnapshotsComparable(before, after) {
		return PackageEvolution{}, conflictError(errors.New("project standards package snapshots do not identify the same package contract"))
	}
	if !sourceAnalysisComparable(before.Code.SourceUsage, after.Code.SourceUsage) {
		return PackageEvolution{}, conflictError(errors.New("project standards package snapshots use different source-analysis generations"))
	}
	beforeEvidence, err := before.EvidenceSummary()
	if err != nil {
		return PackageEvolution{}, err
	}
	afterEvidence, err := after.EvidenceSummary()
	if err != nil {
		return PackageEvolution{}, err
	}
	evolution := PackageEvolution{
		Package: before.Package.Key, Path: before.Code.Package,
		BeforeRevision: before.Package.Revision, AfterRevision: after.Package.Revision,
		Inventory:   inventoryChange(before.Code.Inventory, after.Code.Inventory),
		Evidence:    evidenceChange(beforeEvidence, afterEvidence),
		SourceUsage: sourceUsageChange(before.Code.SourceUsage, after.Code.SourceUsage),
	}
	evolution.NewReviewCandidates, evolution.FormerReviewCandidates = reviewCandidateChanges(before.Code.SourceUsage, after.Code.SourceUsage)
	if err := evolution.Validate(); err != nil {
		return PackageEvolution{}, err
	}
	return evolution, nil
}

func packageSnapshotsComparable(before, after PackageSnapshot) bool {
	return before.Package.Key == after.Package.Key && sameSubject(before.Package.Subject, after.Package.Subject) &&
		before.Package.GroupID == after.Package.GroupID && before.Package.Language == after.Package.Language &&
		before.Code.Package == after.Code.Package
}

func sourceAnalysisComparable(before, after *PackageSourceUsage) bool {
	return before == nil || after == nil || before.Generation == after.Generation
}

func inventoryChange(before, after Inventory) InventoryChange {
	return InventoryChange{
		GoPackages: change(before.GoPackages, after.GoPackages), JavaScriptUnits: change(before.JavaScriptUnits, after.JavaScriptUnits),
		Files: change(before.Files, after.Files), TestFiles: change(before.TestFiles, after.TestFiles),
		Documents: change(before.Documents, after.Documents), TestDeclarations: change(before.TestDeclarations, after.TestDeclarations),
		Benchmarks: change(before.Benchmarks, after.Benchmarks), FuzzTargets: change(before.FuzzTargets, after.FuzzTargets),
		CoverageBasisPoints: coverageChange(before.CoverageBasisPoints, after.CoverageBasisPoints),
	}
}

func evidenceChange(before, after EvidenceSummary) EvidenceChange {
	return EvidenceChange{
		Surfaces: change(before.SurfaceCount, after.SurfaceCount), Requests: change(before.RequestedCount, after.RequestedCount),
		Admissions: change(before.AdmittedCount, after.AdmittedCount), Refusals: change(before.RefusedCount, after.RefusedCount),
		Observations: change(before.ObservedCount, after.ObservedCount), Current: change(before.CurrentCount, after.CurrentCount),
		Stale: change(before.StaleCount, after.StaleCount), Selections: change(before.SelectionCount, after.SelectionCount),
		Passed: change(before.PassedCount, after.PassedCount), Failed: change(before.FailedCount, after.FailedCount),
		NonAccepting: change(before.NonAcceptingCount, after.NonAcceptingCount), Benchmarks: change(before.BenchmarkCount, after.BenchmarkCount),
		Artifacts: change(before.ArtifactCount, after.ArtifactCount), ComplexityCaptures: change(before.ComplexityCaptureCount, after.ComplexityCaptureCount),
	}
}

func sourceUsageChange(before, after *PackageSourceUsage) SourceUsageChange {
	change := SourceUsageChange{BeforeAvailable: before != nil, AfterAvailable: after != nil}
	var left, right PackageSourceUsage
	if before != nil {
		left = *before
	}
	if after != nil {
		right = *after
	}
	change.Declarations = changeCount(left.DeclarationCount, right.DeclarationCount)
	change.ProductionReferenced = changeCount(left.ProductionReferenced, right.ProductionReferenced)
	change.RuntimeEntryPoints = changeCount(left.RuntimeEntryPoints, right.RuntimeEntryPoints)
	change.UnresolvedDeclarations = changeCount(left.UnresolvedDeclarations, right.UnresolvedDeclarations)
	change.TestReferencedOnly = changeCount(left.TestReferencedOnly, right.TestReferencedOnly)
	change.NoReferenceObserved = changeCount(left.NoReferenceObserved, right.NoReferenceObserved)
	change.ObservedConsumerPackages = changeCount(uint32(len(left.ObservedConsumerPackages)), uint32(len(right.ObservedConsumerPackages)))
	return change
}

func reviewCandidateChanges(before, after *PackageSourceUsage) ([]FunctionUsage, []FunctionUsage) {
	if after == nil {
		return nil, nil
	}
	if before == nil {
		return cloneFunctionUsages(after.ReviewCandidates), nil
	}
	return candidateDifference(after.ReviewCandidates, before.ReviewCandidates), candidateDifference(before.ReviewCandidates, after.ReviewCandidates)
}

func candidateDifference(values, excluded []FunctionUsage) []FunctionUsage {
	result := make([]FunctionUsage, 0, len(values))
	for index := range values {
		if !containsFunctionUsage(excluded, values[index].Function) {
			result = append(result, cloneFunctionUsage(values[index]))
		}
	}
	return result
}

func containsFunctionUsage(values []FunctionUsage, reference CodeReference) bool {
	for index := range values {
		if codeReferenceEqual(values[index].Function, reference) {
			return true
		}
	}
	return false
}

func cloneFunctionUsages(values []FunctionUsage) []FunctionUsage {
	result := make([]FunctionUsage, len(values))
	for index := range values {
		result[index] = cloneFunctionUsage(values[index])
	}
	return result
}

func cloneFunctionUsage(value FunctionUsage) FunctionUsage {
	result := value
	if value.Function.Symbol != nil {
		symbol := *value.Function.Symbol
		result.Function.Symbol = &symbol
	}
	result.ObservedConsumers = append([]SourcePath(nil), value.ObservedConsumers...)
	return result
}

func coverageChange(before, after *uint16) CoverageChange {
	return CoverageChange{Before: cloneUint16(before), After: cloneUint16(after)}
}

func cloneUint16(value *uint16) *uint16 {
	if value == nil {
		return nil
	}
	result := *value
	return &result
}

func change[T ~uint16 | ~uint32](before, after T) CountChange {
	return changeCount(uint32(before), uint32(after))
}

func changeCount(before, after uint32) CountChange {
	return NewCountChange(before, after)
}
