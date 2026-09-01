package projectstandards

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const (
	SchemaVersion              = 2
	PackageComponentMaximum    = 64
	PackageContributionMaximum = 32
)

// Package is the authored package-level explanation. It owns package
// identity, purpose, ownership, usage, features, assurance, and grouping; it
// does not own source analysis or execution evidence.
type Package struct {
	Subject   SubjectIdentity  `json:"subject"`
	Key       Identifier       `json:"key"`
	GroupID   Identifier       `json:"group_id"`
	Language  Name             `json:"language"`
	Knowledge PackageKnowledge `json:"knowledge"`
	Revision  core.BuildCommit `json:"revision"`
}

func (p Package) Validate() error {
	return contractJoin(p.Key.Validate(), p.Subject.Validate(), p.Revision.Validate(), p.GroupID.Validate(), p.Language.Validate(), p.Knowledge.Validate())
}

// Code is the exact code-facing package projection. Inventory, components,
// source use, and complexity observations live here rather than in Package.
type Code struct {
	Files       *PackageFileCatalog `json:"files,omitempty"`
	SourceUsage *PackageSourceUsage `json:"source_usage,omitempty"`
	Package     SourcePath          `json:"package"`
	Components  []Component         `json:"components"`
	Inventory   Inventory           `json:"inventory"`
}

func (c Code) Validate() error {
	if len(c.Components) > PackageComponentMaximum {
		return contractError(errors.New("project standards code component count exceeds its bound"))
	}
	if err := contractJoin(c.Package.Validate(), c.Inventory.Validate()); err != nil {
		return err
	}
	if err := c.validateComponents(); err != nil {
		return err
	}
	if err := c.validateFiles(); err != nil {
		return err
	}
	return c.validateSourceUsage()
}

func (c Code) validateFiles() error {
	if c.Files == nil {
		return nil
	}
	if err := c.Files.Validate(); err != nil {
		return err
	}
	if c.Files.Package != c.Package {
		return conflictError(errors.New("project standards file catalog package differs from code package"))
	}
	if !summarizePackageFiles(c.Files.Files).matches(c.Inventory) {
		return conflictError(errors.New("project standards file catalog does not close against inventory"))
	}
	return nil
}

type fileInventoryTotals struct {
	files, testFiles, documents           uint64
	declarations, benchmarks, fuzzTargets uint64
}

func summarizePackageFiles(files []SourceFile) fileInventoryTotals {
	totals := fileInventoryTotals{files: uint64(len(files))}
	for _, file := range files {
		if file.Kind == SourceFileKindTest {
			totals.testFiles++
		}
		if file.Kind == SourceFileKindDocumentation {
			totals.documents++
		}
		totals.declarations += uint64(file.Declarations.TestDeclarations)
		totals.benchmarks += uint64(file.Declarations.Benchmarks)
		totals.fuzzTargets += uint64(file.Declarations.FuzzTargets)
	}
	return totals
}

func (t fileInventoryTotals) matches(inventory Inventory) bool {
	return t.files == uint64(inventory.Files) && t.testFiles == uint64(inventory.TestFiles) && t.documents == uint64(inventory.Documents) &&
		t.declarations == uint64(inventory.TestDeclarations) && t.benchmarks == uint64(inventory.Benchmarks) && t.fuzzTargets == uint64(inventory.FuzzTargets)
}

func (c Code) validateComponents() error {
	for index := range c.Components {
		if err := c.Components[index].Validate(); err != nil {
			return err
		}
		if c.Components[index].Package != c.Package {
			return conflictError(errors.New("project standards component package differs from code package"))
		}
		for previous := range index {
			if c.Components[previous].Path == c.Components[index].Path {
				return conflictError(errors.New("project standards component path is duplicated"))
			}
		}
	}
	return nil
}

func (c Code) validateSourceUsage() error {
	if c.SourceUsage == nil {
		return nil
	}
	if err := c.SourceUsage.Validate(); err != nil {
		return err
	}
	if c.SourceUsage.Package != c.Package {
		return conflictError(errors.New("project standards source usage package differs from code package"))
	}
	return nil
}

// Evidence is the exact proof-facing package projection. It references
// admitted requests and accepted observations without owning runner policy or
// persistence-provider details.
type Evidence struct {
	Package      SourcePath                             `json:"package"`
	Surfaces     []ProjectStandardsEvidenceSurface      `json:"surfaces"`
	Requests     []ProjectStandardsRequestReference     `json:"requests"`
	Observations []ProjectStandardsObservationReference `json:"observations"`
}

func (e Evidence) Validate() error {
	if err := e.validateShape(); err != nil {
		return err
	}
	if err := e.validateSurfaces(); err != nil {
		return err
	}
	if err := e.validateRequests(); err != nil {
		return err
	}
	return e.validateObservations()
}

func (e Evidence) validateShape() error {
	if err := e.Package.Validate(); err != nil {
		return err
	}
	if len(e.Surfaces) == 0 || len(e.Surfaces) > EvidenceSurfaceMaximum || len(e.Requests) > RequestReferenceMaximum || len(e.Observations) > ObservationReferenceMaximum {
		return contractError(errors.New("project standards evidence shape is invalid"))
	}
	return nil
}

// EvidenceSummary is a bounded rollup derived from exact request and
// observation references. It preserves accounting facts and does not assign a
// product-facing freshness or quality classification.
type EvidenceSummary struct {
	SurfaceCount           uint16 `json:"surface_count"`
	RequestedCount         uint16 `json:"requested_count"`
	AdmittedCount          uint16 `json:"admitted_count"`
	RefusedCount           uint16 `json:"refused_count"`
	ObservedCount          uint16 `json:"observed_count"`
	CurrentCount           uint16 `json:"current_count"`
	StaleCount             uint16 `json:"stale_count"`
	SelectionCount         uint16 `json:"selection_count"`
	PassedCount            uint16 `json:"passed_count"`
	FailedCount            uint16 `json:"failed_count"`
	NonAcceptingCount      uint16 `json:"non_accepting_count"`
	BenchmarkCount         uint16 `json:"benchmark_count"`
	ArtifactCount          uint16 `json:"artifact_count"`
	ComplexityCaptureCount uint16 `json:"complexity_capture_count"`
}

func (s EvidenceSummary) Validate() error {
	if s.SurfaceCount > EvidenceSurfaceMaximum || s.RequestedCount > RequestReferenceMaximum || s.ObservedCount > ObservationReferenceMaximum {
		return contractError(errors.New("project standards evidence summary exceeds its bounds"))
	}
	if s.AdmittedCount+s.RefusedCount != s.RequestedCount || s.CurrentCount+s.StaleCount != s.ObservedCount {
		return conflictError(errors.New("project standards evidence summary accounting does not close"))
	}
	if s.SelectionCount+s.PassedCount+s.FailedCount+s.NonAcceptingCount != s.ObservedCount {
		return conflictError(errors.New("project standards evidence outcome accounting does not close"))
	}
	return nil
}

// SourceUsageSummary is derived from one PackageSourceUsage observation.
type SourceUsageSummary struct {
	Generation               Identifier                 `json:"generation"`
	Revision                 core.BuildCommit           `json:"revision"`
	Completeness             SourceAnalysisCompleteness `json:"completeness"`
	DeclarationCount         uint32                     `json:"declaration_count"`
	ProductionReferenced     uint32                     `json:"production_referenced"`
	RuntimeEntryPoints       uint32                     `json:"runtime_entry_points"`
	UnresolvedDeclarations   uint32                     `json:"unresolved_declarations"`
	TestReferencedOnly       uint32                     `json:"test_referenced_only"`
	NoReferenceObserved      uint32                     `json:"no_reference_observed"`
	ObservedConsumerPackages uint16                     `json:"observed_consumer_packages"`
}

func (s SourceUsageSummary) Validate() error {
	if err := contractJoin(s.Generation.Validate(), s.Revision.Validate(), s.Completeness.Validate()); err != nil {
		return err
	}
	if s.DeclarationCount != s.ProductionReferenced+s.RuntimeEntryPoints+s.UnresolvedDeclarations+s.TestReferencedOnly+s.NoReferenceObserved {
		return conflictError(errors.New("project standards source usage summary accounting does not close"))
	}
	if s.ObservedConsumerPackages > ObservedConsumerMaximum {
		return contractError(errors.New("project standards source usage consumer count exceeds its bound"))
	}
	return nil
}

// PackageGroup is one product-authored package-map section.
type PackageGroup struct {
	ID      Identifier `json:"id"`
	Title   Name       `json:"title"`
	Purpose Text       `json:"purpose"`
}

func (g PackageGroup) Validate() error {
	return contractJoin(g.ID.Validate(), g.Title.Validate(), g.Purpose.Validate())
}

// PackageContribution binds one product feature to one package feature and
// its evidence surfaces without copying either definition.
type PackageContribution struct {
	Package    SourcePath   `json:"package"`
	FeatureID  Identifier   `json:"feature_id"`
	Role       Text         `json:"role"`
	SurfaceIDs []Identifier `json:"evidence_surface_ids"`
}

func (c PackageContribution) Validate() error {
	if len(c.SurfaceIDs) > EvidenceSurfaceMaximum {
		return contractError(errors.New("project standards contribution surface count exceeds its bound"))
	}
	if err := contractJoin(c.Package.Validate(), c.FeatureID.Validate(), c.Role.Validate()); err != nil {
		return err
	}
	return validateIdentifiers(c.SurfaceIDs)
}

// ProjectCapability assembles one product feature from package-owned feature
// contributions.
type ProjectCapability struct {
	FeatureID     Identifier            `json:"feature_id"`
	Contributions []PackageContribution `json:"contributions"`
}

func (c ProjectCapability) Validate() error {
	if len(c.Contributions) == 0 || len(c.Contributions) > PackageContributionMaximum {
		return contractError(errors.New("project standards capability contribution count is invalid"))
	}
	if err := c.FeatureID.Validate(); err != nil {
		return err
	}
	for index := range c.Contributions {
		if err := c.Contributions[index].Validate(); err != nil {
			return err
		}
		for previous := range index {
			left := c.Contributions[previous]
			right := c.Contributions[index]
			if left.Package == right.Package && left.FeatureID == right.FeatureID {
				return conflictError(errors.New("project standards capability contribution is duplicated"))
			}
		}
	}
	return nil
}

// PackageSnapshot is one bounded package Project standards record at a selected revision.
// Product knowledge remains authored; source usage and evidence are observed.
type PackageSnapshot struct {
	Code          Code     `json:"code"`
	Evidence      Evidence `json:"evidence"`
	Package       Package  `json:"package"`
	SchemaVersion uint16   `json:"schema_version"`
}

func (s PackageSnapshot) Validate() error {
	if s.SchemaVersion != SchemaVersion {
		return contractError(errors.New("project standards package snapshot schema version is unsupported"))
	}
	if err := contractJoin(s.Package.Validate(), s.Code.Validate(), s.Evidence.Validate()); err != nil {
		return err
	}
	if s.Code.Package != s.Package.Knowledge.Path || s.Evidence.Package != s.Package.Knowledge.Path {
		return conflictError(errors.New("project standards package, code, and evidence paths disagree"))
	}
	if s.Code.SourceUsage != nil && s.Code.SourceUsage.Revision != s.Package.Revision {
		return conflictError(errors.New("project standards source usage revision differs from package revision"))
	}
	return s.validateAuthoredSurfaceReferences()
}

func (e Evidence) validateSurfaces() error {
	for index := range e.Surfaces {
		if err := e.Surfaces[index].Validate(); err != nil {
			return err
		}
		for previous := range index {
			if e.Surfaces[previous].ID == e.Surfaces[index].ID {
				return conflictError(errors.New("project standards evidence surface identity is duplicated"))
			}
		}
	}
	return nil
}

func (s PackageSnapshot) validateAuthoredSurfaceReferences() error {
	for _, surface := range s.Evidence.Surfaces {
		if !sameSubject(surface.Subject, s.Package.Subject) {
			return conflictError(errors.New("project standards evidence surface subject differs from package subject"))
		}
	}
	for _, control := range [...]AssuranceControl{s.Package.Knowledge.AuthorAssurance.Policy, s.Package.Knowledge.AuthorAssurance.Validation, s.Package.Knowledge.AuthorAssurance.Effects, s.Package.Knowledge.AuthorAssurance.Proof} {
		for _, id := range control.SurfaceIDs {
			if !surfaceExists(s.Evidence.Surfaces, id) {
				return conflictError(errors.New("project standards assurance names an absent evidence surface"))
			}
		}
	}
	for _, claim := range s.Package.Knowledge.AuthorComplexity {
		if !surfaceExists(s.Evidence.Surfaces, claim.SurfaceID) {
			return conflictError(errors.New("project standards complexity claim names an absent evidence surface"))
		}
	}
	return nil
}

func (e Evidence) validateRequests() error {
	for index := range e.Requests {
		surface, ok := findSurface(e.Surfaces, e.Requests[index].SurfaceID)
		if !ok {
			return conflictError(errors.New("project standards request names an absent evidence surface"))
		}
		if err := e.Requests[index].ValidateFor(surface); err != nil {
			return err
		}
		for previous := range index {
			if e.Requests[previous].Request == e.Requests[index].Request {
				return conflictError(errors.New("project standards request identity is duplicated"))
			}
		}
	}
	return nil
}

func (e Evidence) validateObservations() error {
	for index := range e.Observations {
		request, ok := findRequest(e.Requests, e.Observations[index].Request)
		if !ok {
			return conflictError(errors.New("project standards observation names an absent request"))
		}
		if err := e.Observations[index].ValidateFor(request); err != nil {
			return err
		}
		if err := validateSelectionExpansion(e.Observations, e.Observations[index]); err != nil {
			return err
		}
		for previous := range index {
			if e.Observations[previous].Observation == e.Observations[index].Observation || e.Observations[previous].EnvelopeDigest == e.Observations[index].EnvelopeDigest {
				return conflictError(errors.New("project standards observation identity or envelope is duplicated"))
			}
		}
	}
	return nil
}

func validateSelectionExpansion(values []ProjectStandardsObservationReference, child ProjectStandardsObservationReference) error {
	if child.Probe.Parent == nil {
		return nil
	}
	for index := range values {
		candidate := values[index]
		if candidate.Request != child.Request || candidate.Kind != ObservationSelection || candidate.Selection == nil {
			continue
		}
		if candidate.Selection.ExpansionIdentity != child.Probe.Parent.ExpansionDigest {
			return conflictError(errors.New("project standards child experiment expansion differs from selection observation"))
		}
		return nil
	}
	return conflictError(errors.New("project standards child experiment has no selection observation"))
}

// PackageSummary is derived from PackageSnapshot for a compact project index.
type PackageSummary struct {
	SourceUsage          *SourceUsageSummary `json:"source_usage,omitempty"`
	Language             Name                `json:"language"`
	Title                Name                `json:"title"`
	Purpose              Text                `json:"purpose"`
	Value                Text                `json:"value"`
	GroupID              Identifier          `json:"group_id"`
	Key                  Identifier          `json:"key"`
	Runtime              Name                `json:"runtime"`
	Path                 SourcePath          `json:"path"`
	Changed              GitOrigin           `json:"changed"`
	Evidence             EvidenceSummary     `json:"evidence"`
	FeatureCount         uint16              `json:"feature_count"`
	ComponentCount       uint16              `json:"component_count"`
	ComplexityClaimCount uint16              `json:"complexity_claim_count"`
}

func (s PackageSummary) Validate() error {
	if s.FeatureCount > FeatureMaximum || s.ComponentCount > PackageComponentMaximum || s.ComplexityClaimCount > ComplexityClaimMaximum {
		return contractError(errors.New("project standards package summary count exceeds its bound"))
	}
	if err := contractJoin(s.Key.Validate(), s.Path.Validate(), s.Title.Validate(), s.Purpose.Validate(), s.Value.Validate(), s.GroupID.Validate(), s.Language.Validate(), s.Runtime.Validate(), s.Changed.Validate(), s.Evidence.Validate()); err != nil {
		return err
	}
	if s.SourceUsage != nil {
		return s.SourceUsage.Validate()
	}
	return nil
}

// Summary derives the compact project-index record from the package truth.
func (s PackageSnapshot) Summary() (PackageSummary, error) {
	if err := s.Validate(); err != nil {
		return PackageSummary{}, err
	}
	evidence, err := s.EvidenceSummary()
	if err != nil {
		return PackageSummary{}, err
	}
	usage, err := s.SourceUsageSummary()
	if err != nil {
		return PackageSummary{}, err
	}
	featureCount, featureErr := core.CheckedUint16FromInt(len(s.Package.Knowledge.AuthorFeatures))
	componentCount, componentErr := core.CheckedUint16FromInt(len(s.Code.Components))
	complexityCount, complexityErr := core.CheckedUint16FromInt(len(s.Package.Knowledge.AuthorComplexity))
	if err := errors.Join(featureErr, componentErr, complexityErr); err != nil {
		return PackageSummary{}, contractError(err)
	}
	return PackageSummary{
		Key: s.Package.Key, Path: s.Package.Knowledge.Path, Title: s.Package.Knowledge.AuthorTitle, Purpose: s.Package.Knowledge.AuthorPurpose,
		Value: s.Package.Knowledge.AuthorValue, GroupID: s.Package.GroupID, Language: s.Package.Language, Runtime: s.Package.Knowledge.AuthorRuntime,
		Changed: s.Package.Knowledge.Changed, Evidence: evidence, SourceUsage: usage,
		FeatureCount: featureCount, ComponentCount: componentCount,
		ComplexityClaimCount: complexityCount,
	}, nil
}

// EvidenceSummary derives accounting from exact request and observation refs.
func (s PackageSnapshot) EvidenceSummary() (EvidenceSummary, error) {
	if err := s.Validate(); err != nil {
		return EvidenceSummary{}, err
	}
	surfaceCount, surfaceErr := core.CheckedUint16FromInt(len(s.Evidence.Surfaces))
	requestedCount, requestedErr := core.CheckedUint16FromInt(len(s.Evidence.Requests))
	observedCount, observedErr := core.CheckedUint16FromInt(len(s.Evidence.Observations))
	if err := errors.Join(surfaceErr, requestedErr, observedErr); err != nil {
		return EvidenceSummary{}, contractError(err)
	}
	summary := EvidenceSummary{SurfaceCount: surfaceCount, RequestedCount: requestedCount, ObservedCount: observedCount}
	for _, request := range s.Evidence.Requests {
		if request.Disposition.Admitted != nil {
			summary.AdmittedCount++
		} else {
			summary.RefusedCount++
		}
	}
	for _, observation := range s.Evidence.Observations {
		if err := summary.addObservation(s.Package.Revision, observation); err != nil {
			return EvidenceSummary{}, err
		}
	}
	return summary, summary.Validate()
}

func (s *EvidenceSummary) addObservation(revision core.BuildCommit, observation ProjectStandardsObservationReference) error {
	if observation.Source.Commit == revision {
		s.CurrentCount++
	} else {
		s.StaleCount++
	}
	if observation.Kind == ObservationExperiment && observation.Experiment != nil {
		return s.addExperiment(*observation.Experiment)
	}
	if observation.Kind == ObservationSelection && observation.Selection != nil {
		s.SelectionCount++
		return nil
	}
	s.NonAcceptingCount++
	return nil
}

func (s *EvidenceSummary) addExperiment(observation ExperimentObservation) error {
	switch observation.Outcome {
	case OutcomePassed:
		s.PassedCount++
	case OutcomeFailed:
		s.FailedCount++
	case OutcomeSkipped, OutcomeUnavailable, OutcomeTimedOut, OutcomeCancelled, OutcomeSetupFailed, OutcomeInfrastructureFailed, OutcomeNotRun, OutcomeNonAccepting:
		s.NonAcceptingCount++
	case OutcomeUnknown:
		return contractError(errors.New("project standards experiment summary received an unknown outcome"))
	default:
		return contractError(errors.New("project standards experiment summary received an outcome outside its domain"))
	}
	benchmarks, benchmarkErr := core.CheckedUint16FromInt(len(observation.Measurements.Benchmarks))
	artifacts, artifactErr := core.CheckedUint16FromInt(len(observation.Artifacts))
	complexity, complexityErr := core.CheckedUint16FromInt(len(observation.Measurements.Complexity))
	if err := errors.Join(benchmarkErr, artifactErr, complexityErr); err != nil {
		return contractError(err)
	}
	s.BenchmarkCount += benchmarks
	s.ArtifactCount += artifacts
	s.ComplexityCaptureCount += complexity
	return nil
}

// SourceUsageSummary derives the compact source-use posture.
func (s PackageSnapshot) SourceUsageSummary() (*SourceUsageSummary, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	if s.Code.SourceUsage == nil {
		return nil, nil
	}
	usage := s.Code.SourceUsage
	consumerCount, err := core.CheckedUint16FromInt(len(usage.ObservedConsumerPackages))
	if err != nil {
		return nil, contractError(err)
	}
	summary := SourceUsageSummary{
		Generation: usage.Generation, Revision: usage.Revision, Completeness: usage.Completeness,
		DeclarationCount: usage.DeclarationCount, ProductionReferenced: usage.ProductionReferenced,
		RuntimeEntryPoints: usage.RuntimeEntryPoints, UnresolvedDeclarations: usage.UnresolvedDeclarations,
		TestReferencedOnly: usage.TestReferencedOnly, NoReferenceObserved: usage.NoReferenceObserved,
		ObservedConsumerPackages: consumerCount,
	}
	return &summary, summary.Validate()
}

// ProjectCode is the exact project-wide code inventory. Catalog validation
// proves its additive counters equal the package Code inventories.
type ProjectCode struct {
	Unattributed *Inventory `json:"unattributed,omitempty"`
	Inventory    Inventory  `json:"inventory"`
}

func (c ProjectCode) Validate() error {
	if err := c.Inventory.Validate(); err != nil {
		return err
	}
	if c.Unattributed == nil {
		return nil
	}
	if err := c.Unattributed.Validate(); err != nil {
		return err
	}
	if c.Unattributed.GoPackages != 0 || c.Unattributed.JavaScriptUnits != 0 || c.Unattributed.TestFiles != 0 || c.Unattributed.TestDeclarations != 0 || c.Unattributed.Benchmarks != 0 || c.Unattributed.FuzzTargets != 0 {
		return conflictError(errors.New("project standards unattributed project code contains executable units"))
	}
	return nil
}

// Project is the authored project record plus its derived package index and
// exact project-wide code inventory at one revision.
type Project struct {
	Code          ProjectCode          `json:"code"`
	Release       *core.ReleaseVersion `json:"release,omitempty"`
	Subject       SubjectIdentity      `json:"subject"`
	Usage         []Usage              `json:"usage"`
	Groups        []PackageGroup       `json:"groups"`
	Capabilities  []ProjectCapability  `json:"capabilities"`
	Packages      []PackageSummary     `json:"packages"`
	Knowledge     ProductKnowledge     `json:"knowledge"`
	SchemaVersion uint16               `json:"schema_version"`
	Revision      core.BuildCommit     `json:"revision"`
}

func (s Project) Validate() error {
	if !s.validShape() {
		return contractError(errors.New("project standards project snapshot shape is invalid"))
	}
	if err := contractJoin(s.Subject.Validate(), s.Revision.Validate(), s.Knowledge.Validate(), s.Code.Validate()); err != nil {
		return err
	}
	if err := s.validateRelease(); err != nil {
		return err
	}
	if err := validateUsage(s.Usage); err != nil {
		return err
	}
	if err := validateGroups(s.Groups); err != nil {
		return err
	}
	if err := validatePackageSummaries(s.Groups, s.Packages); err != nil {
		return err
	}
	return validateCapabilities(s.Knowledge.AuthorFeatures, s.Capabilities)
}

func (s Project) validShape() bool {
	return s.SchemaVersion == SchemaVersion && len(s.Groups) > 0 && len(s.Groups) <= GroupMaximum && len(s.Packages) > 0 && len(s.Packages) <= PackageMaximum
}

func (s Project) validateRelease() error {
	if s.Release == nil {
		return nil
	}
	if err := s.Release.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// Catalog proves that one project index and its package snapshots describe
// the same exact bounded set. It is not a persistence schema.
type Catalog struct {
	Packages []PackageSnapshot `json:"packages"`
	Project  Project           `json:"project"`
}

func (c Catalog) Validate() error {
	if err := c.Project.Validate(); err != nil {
		return err
	}
	if len(c.Packages) != len(c.Project.Packages) {
		return conflictError(errors.New("project standards catalog package count differs from project index"))
	}
	for index := range c.Packages {
		if err := c.validatePackage(index); err != nil {
			return err
		}
	}
	if err := c.validateExclusiveComponents(); err != nil {
		return err
	}
	if err := c.validateProjectCode(); err != nil {
		return err
	}
	return c.validateCapabilityBindings()
}

func (c Catalog) validateProjectCode() error {
	var total inventoryTotals
	for index := range c.Packages {
		inventory := c.Packages[index].Code.Inventory
		total.add(inventory)
	}
	if c.Project.Code.Unattributed != nil {
		total.add(*c.Project.Code.Unattributed)
	}
	want := c.Project.Code.Inventory
	if !total.matches(want) {
		return conflictError(errors.New("project standards project code inventory differs from package inventories"))
	}
	return nil
}

type inventoryTotals struct {
	goPackages, javaScriptUnits, files, testFiles        uint64
	documents, testDeclarations, benchmarks, fuzzTargets uint64
}

func (t *inventoryTotals) add(value Inventory) {
	t.goPackages += uint64(value.GoPackages)
	t.javaScriptUnits += uint64(value.JavaScriptUnits)
	t.files += uint64(value.Files)
	t.testFiles += uint64(value.TestFiles)
	t.documents += uint64(value.Documents)
	t.testDeclarations += uint64(value.TestDeclarations)
	t.benchmarks += uint64(value.Benchmarks)
	t.fuzzTargets += uint64(value.FuzzTargets)
}

func (t inventoryTotals) matches(value Inventory) bool {
	return t.goPackages == uint64(value.GoPackages) && t.javaScriptUnits == uint64(value.JavaScriptUnits) && t.files == uint64(value.Files) &&
		t.testFiles == uint64(value.TestFiles) && t.documents == uint64(value.Documents) && t.testDeclarations == uint64(value.TestDeclarations) &&
		t.benchmarks == uint64(value.Benchmarks) && t.fuzzTargets == uint64(value.FuzzTargets)
}

func (c Catalog) validatePackage(index int) error {
	current := c.Packages[index]
	if err := current.Validate(); err != nil {
		return err
	}
	if !sameSubject(current.Package.Subject, c.Project.Subject) || current.Package.Revision != c.Project.Revision {
		return conflictError(errors.New("project standards package snapshot differs from project source"))
	}
	summary, err := current.Summary()
	if err != nil {
		return err
	}
	if !packageSummaryEqual(summary, c.Project.Packages[index]) {
		return conflictError(errors.New("project standards package summary differs from package snapshot"))
	}
	return nil
}

func (c Catalog) validateExclusiveComponents() error {
	for packageIndex := range c.Packages {
		for componentIndex := range c.Packages[packageIndex].Code.Components {
			path := c.Packages[packageIndex].Code.Components[componentIndex].Path
			for candidateIndex := range c.Packages {
				if candidateIndex != packageIndex && pathWithin(c.Packages[candidateIndex].Package.Knowledge.Path, path, true) {
					return conflictError(errors.New("project standards component belongs to multiple packages"))
				}
			}
		}
	}
	return nil
}

func (c Catalog) validateCapabilityBindings() error {
	for _, capability := range c.Project.Capabilities {
		for _, contribution := range capability.Contributions {
			owner, ok := findPackage(c.Packages, contribution.Package)
			if !ok || !featureExists(owner.Package.Knowledge.AuthorFeatures, contribution.FeatureID) {
				return conflictError(errors.New("project standards capability contribution has no package feature"))
			}
			for _, surfaceID := range contribution.SurfaceIDs {
				if !surfaceExists(owner.Evidence.Surfaces, surfaceID) {
					return conflictError(errors.New("project standards capability contribution has no evidence surface"))
				}
			}
		}
	}
	return nil
}

func validateGroups(values []PackageGroup) error {
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return err
		}
		for previous := range index {
			if values[previous].ID == values[index].ID {
				return conflictError(errors.New("project standards package group is duplicated"))
			}
		}
	}
	return nil
}

func validatePackageSummaries(groups []PackageGroup, values []PackageSummary) error {
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return err
		}
		if !groupExists(groups, values[index].GroupID) {
			return conflictError(errors.New("project standards package summary has no package group"))
		}
		for previous := range index {
			if values[previous].Key == values[index].Key || values[previous].Path == values[index].Path {
				return conflictError(errors.New("project standards package summary is duplicated"))
			}
		}
	}
	return nil
}

func validateCapabilities(features []Feature, values []ProjectCapability) error {
	if len(values) != len(features) || len(values) > FeatureMaximum {
		return conflictError(errors.New("project standards project capabilities do not close over product features"))
	}
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return err
		}
		if !featureExists(features, values[index].FeatureID) {
			return conflictError(errors.New("project standards project capability names no product feature"))
		}
		for previous := range index {
			if values[previous].FeatureID == values[index].FeatureID {
				return conflictError(errors.New("project standards project capability is duplicated"))
			}
		}
	}
	return nil
}

func findSurface(values []ProjectStandardsEvidenceSurface, id Identifier) (ProjectStandardsEvidenceSurface, bool) {
	for index := range values {
		if values[index].ID == id {
			return values[index], true
		}
	}
	return ProjectStandardsEvidenceSurface{}, false
}

func surfaceExists(values []ProjectStandardsEvidenceSurface, id Identifier) bool {
	_, ok := findSurface(values, id)
	return ok
}

func findRequest(values []ProjectStandardsRequestReference, id RequestIdentity) (ProjectStandardsRequestReference, bool) {
	for index := range values {
		if values[index].Request == id {
			return values[index], true
		}
	}
	return ProjectStandardsRequestReference{}, false
}

func findPackage(values []PackageSnapshot, path SourcePath) (PackageSnapshot, bool) {
	for index := range values {
		if values[index].Package.Knowledge.Path == path {
			return values[index], true
		}
	}
	return PackageSnapshot{}, false
}

func featureExists(values []Feature, id Identifier) bool {
	for index := range values {
		if values[index].ID == id {
			return true
		}
	}
	return false
}

func groupExists(values []PackageGroup, id Identifier) bool {
	for index := range values {
		if values[index].ID == id {
			return true
		}
	}
	return false
}

func packageSummaryEqual(left, right PackageSummary) bool {
	checks := [...]bool{
		left.Key == right.Key,
		left.Path == right.Path,
		left.Title == right.Title,
		left.Purpose == right.Purpose,
		left.Value == right.Value,
		left.GroupID == right.GroupID,
		left.Language == right.Language,
		left.Runtime == right.Runtime,
		left.Changed == right.Changed,
		left.Evidence == right.Evidence,
		sourceUsageSummaryEqual(left.SourceUsage, right.SourceUsage),
		left.FeatureCount == right.FeatureCount,
		left.ComponentCount == right.ComponentCount,
		left.ComplexityClaimCount == right.ComplexityClaimCount,
	}
	for _, matches := range checks {
		if !matches {
			return false
		}
	}
	return true
}

func sourceUsageSummaryEqual(left, right *SourceUsageSummary) bool {
	if (left == nil) != (right == nil) {
		return false
	}
	return left == nil || *left == *right
}
