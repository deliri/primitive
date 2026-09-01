package projectstandards

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

type evolutionPrimaryClass uint8

const (
	evolutionClassUnknown evolutionPrimaryClass = iota
	evolutionClassCompatible
	evolutionClassRefusal
	evolutionClassBoundary
)

type evolutionField uint8

const (
	evolutionFieldNone evolutionField = iota
	evolutionFieldGoPackages
	evolutionFieldJavaScriptUnits
	evolutionFieldFiles
	evolutionFieldTestFiles
	evolutionFieldDocuments
	evolutionFieldTestDeclarations
	evolutionFieldBenchmarks
	evolutionFieldFuzzTargets
	evolutionFieldEvidenceSurfaces
)

type evolutionHostileCase struct {
	wantErr           error
	mutate            func(testing.TB, *PackageSnapshot, *PackageSnapshot)
	name              string
	wantChange        CountChange
	wantNewCandidates int
	primary           evolutionPrimaryClass
	wantField         evolutionField
	wantBeforeSource  bool
	wantAfterSource   bool
}

func TestComparePackageSnapshotsHostileBoundaries(t *testing.T) {
	t.Parallel()

	cases := evolutionCompatibleCases()
	cases = append(cases, evolutionRefusalCases()...)
	cases = append(cases, evolutionBoundaryCases()...)
	if got, want := len(cases), 40; got != want {
		t.Fatalf("hostile package-evolution case count = %d, want %d earned cases", got, want)
	}
	var primaryCounts [4]uint8
	for _, tc := range cases {
		primaryCounts[tc.primary]++
	}
	if primaryCounts[evolutionClassUnknown] != 0 || primaryCounts[evolutionClassCompatible] != 10 || primaryCounts[evolutionClassRefusal] != 10 || primaryCounts[evolutionClassBoundary] != 20 {
		t.Fatalf("hostile package-evolution primary classes = %v, want [0 10 10 20] with exclusive accounting", primaryCounts)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			before, after := evolutionMatrixBase(t)
			tc.mutate(t, &before, &after)
			got, gotErr := ComparePackageSnapshots(before, after)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("ComparePackageSnapshots() error = %v, want %v identity for %s", gotErr, tc.wantErr, tc.name)
				}
				if len(got.NewReviewCandidates) != 0 || len(got.FormerReviewCandidates) != 0 {
					t.Fatalf("ComparePackageSnapshots(rejected) candidate movement = (new %d, former %d), want (0, 0)", len(got.NewReviewCandidates), len(got.FormerReviewCandidates))
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("ComparePackageSnapshots() error = %v, want nil for %s", gotErr, tc.name)
			}
			gotChange := evolutionFieldChange(got, tc.wantField)
			if gotChange != tc.wantChange {
				t.Fatalf("ComparePackageSnapshots() %v change = %d, want %d for %s", tc.wantField, gotChange, tc.wantChange, tc.name)
			}
			if got.SourceUsage.BeforeAvailable != tc.wantBeforeSource || got.SourceUsage.AfterAvailable != tc.wantAfterSource {
				t.Fatalf("source availability = (%t, %t), want (%t, %t) for %s", got.SourceUsage.BeforeAvailable, got.SourceUsage.AfterAvailable, tc.wantBeforeSource, tc.wantAfterSource, tc.name)
			}
			if len(got.NewReviewCandidates) != tc.wantNewCandidates {
				t.Fatalf("new review candidate count = %d, want %d for %s", len(got.NewReviewCandidates), tc.wantNewCandidates, tc.name)
			}
		})
	}
}

func evolutionCompatibleCases() []evolutionHostileCase {
	return []evolutionHostileCase{
		{name: "compatible exact revision movement changes no derived count", primary: evolutionClassCompatible, mutate: noEvolutionMutation},
		{name: "compatible authored runtime upgrade changes no observed count", primary: evolutionClassCompatible, mutate: mutateEvolutionRuntime},
		{name: "compatible additional Go package changes only Go package count", primary: evolutionClassCompatible, mutate: mutateEvolutionGoPackage, wantField: evolutionFieldGoPackages, wantChange: 1},
		{name: "compatible additional JavaScript unit changes only JavaScript count", primary: evolutionClassCompatible, mutate: mutateEvolutionJavaScript, wantField: evolutionFieldJavaScriptUnits, wantChange: 1},
		{name: "compatible additional source file changes file count", primary: evolutionClassCompatible, mutate: mutateEvolutionFile, wantField: evolutionFieldFiles, wantChange: 1},
		{name: "compatible additional test file changes test-file count", primary: evolutionClassCompatible, mutate: mutateEvolutionTestFile, wantField: evolutionFieldTestFiles, wantChange: 1},
		{name: "compatible additional document changes document count", primary: evolutionClassCompatible, mutate: mutateEvolutionDocument, wantField: evolutionFieldDocuments, wantChange: 1},
		{name: "compatible additional benchmark changes benchmark count", primary: evolutionClassCompatible, mutate: mutateEvolutionBenchmark, wantField: evolutionFieldBenchmarks, wantChange: 1},
		{name: "compatible additional fuzz target changes fuzz-target count", primary: evolutionClassCompatible, mutate: mutateEvolutionFuzz, wantField: evolutionFieldFuzzTargets, wantChange: 1},
		{name: "compatible source analysis appearance exposes one review candidate", primary: evolutionClassCompatible, mutate: mutateEvolutionOneCandidate, wantBeforeSource: false, wantAfterSource: true, wantNewCandidates: 1},
	}
}

func evolutionRefusalCases() []evolutionHostileCase {
	return []evolutionHostileCase{
		{name: "refusal package key differs", primary: evolutionClassRefusal, mutate: mutateEvolutionPackageKey, wantErr: core.ErrProjectStandardsConflict},
		{name: "refusal subject project differs", primary: evolutionClassRefusal, mutate: mutateEvolutionSubjectProject, wantErr: core.ErrProjectStandardsConflict},
		{name: "refusal subject repository differs", primary: evolutionClassRefusal, mutate: mutateEvolutionSubjectRepository, wantErr: core.ErrProjectStandardsConflict},
		{name: "refusal package group differs", primary: evolutionClassRefusal, mutate: mutateEvolutionGroup, wantErr: core.ErrProjectStandardsConflict},
		{name: "refusal package language differs", primary: evolutionClassRefusal, mutate: mutateEvolutionLanguage, wantErr: core.ErrProjectStandardsConflict},
		{name: "refusal code package path differs", primary: evolutionClassRefusal, mutate: mutateEvolutionCodePath, wantErr: core.ErrProjectStandardsConflict},
		{name: "refusal source analyzer generation differs", primary: evolutionClassRefusal, mutate: mutateEvolutionAnalyzerGeneration, wantErr: core.ErrProjectStandardsConflict},
		{name: "refusal before schema is unset", primary: evolutionClassRefusal, mutate: mutateEvolutionBeforeSchema, wantErr: core.ErrProjectStandardsContract},
		{name: "refusal after schema is future", primary: evolutionClassRefusal, mutate: mutateEvolutionAfterSchema, wantErr: core.ErrProjectStandardsContract},
		{name: "refusal source analysis names another revision", primary: evolutionClassRefusal, mutate: mutateEvolutionSourceRevision, wantErr: core.ErrProjectStandardsConflict},
	}
}

func evolutionBoundaryCases() []evolutionHostileCase {
	return []evolutionHostileCase{
		{name: "boundary files decrease by one", primary: evolutionClassBoundary, mutate: mutateEvolutionFilesOne, wantField: evolutionFieldFiles, wantChange: -1},
		{name: "boundary files remain exact", primary: evolutionClassBoundary, mutate: mutateEvolutionFilesTwo, wantField: evolutionFieldFiles},
		{name: "boundary files increase by one", primary: evolutionClassBoundary, mutate: mutateEvolutionFilesThree, wantField: evolutionFieldFiles, wantChange: 1},
		{name: "boundary files reach uint32 extreme", primary: evolutionClassBoundary, mutate: mutateEvolutionFilesExtreme, wantField: evolutionFieldFiles, wantChange: CountChange(math.MaxUint32 - 2)},
		{name: "boundary test declarations reach zero", primary: evolutionClassBoundary, mutate: mutateEvolutionDeclarationsZero, wantField: evolutionFieldTestDeclarations, wantChange: -1},
		{name: "boundary test declarations reach exact per-file maximum", primary: evolutionClassBoundary, mutate: mutateEvolutionDeclarationsExact, wantField: evolutionFieldTestDeclarations, wantChange: 255},
		{name: "boundary test declarations exceed per-file maximum by one", primary: evolutionClassBoundary, mutate: mutateEvolutionDeclarationsAbove, wantErr: core.ErrProjectStandardsConflict},
		{name: "boundary test declarations reach uint32 extreme", primary: evolutionClassBoundary, mutate: mutateEvolutionDeclarationsExtreme, wantErr: core.ErrProjectStandardsConflict},
		{name: "boundary coverage remains absent", primary: evolutionClassBoundary, mutate: mutateEvolutionCoverageAbsent},
		{name: "boundary coverage appears at zero basis points", primary: evolutionClassBoundary, mutate: mutateEvolutionCoverageZero},
		{name: "boundary coverage reaches exact maximum basis points", primary: evolutionClassBoundary, mutate: mutateEvolutionCoverageExact},
		{name: "boundary coverage exceeds maximum by one", primary: evolutionClassBoundary, mutate: mutateEvolutionCoverageAbove, wantErr: core.ErrProjectStandardsContract},
		{name: "boundary review candidates remain empty", primary: evolutionClassBoundary, mutate: mutateEvolutionCandidateCountZero, wantAfterSource: true},
		{name: "boundary review candidates contain one exact function", primary: evolutionClassBoundary, mutate: mutateEvolutionOneCandidate, wantAfterSource: true, wantNewCandidates: 1},
		{name: "boundary review candidates reach exact maximum", primary: evolutionClassBoundary, mutate: mutateEvolutionCandidateCountMaximum, wantAfterSource: true, wantNewCandidates: FunctionUsageMaximum},
		{name: "boundary review candidates exceed maximum by one", primary: evolutionClassBoundary, mutate: mutateEvolutionCandidateCountAbove, wantErr: core.ErrProjectStandardsContract},
		{name: "boundary evidence surfaces are absent", primary: evolutionClassBoundary, mutate: mutateEvolutionSurfacesZero, wantErr: core.ErrProjectStandardsContract},
		{name: "boundary evidence surfaces remain at minimum", primary: evolutionClassBoundary, mutate: mutateEvolutionSurfacesOne, wantField: evolutionFieldEvidenceSurfaces},
		{name: "boundary evidence surfaces reach exact maximum", primary: evolutionClassBoundary, mutate: mutateEvolutionSurfacesMaximum, wantField: evolutionFieldEvidenceSurfaces, wantChange: EvidenceSurfaceMaximum - 1},
		{name: "boundary evidence surfaces exceed maximum by one", primary: evolutionClassBoundary, mutate: mutateEvolutionSurfacesAbove, wantErr: core.ErrProjectStandardsContract},
	}
}

func evolutionMatrixBase(t testing.TB) (PackageSnapshot, PackageSnapshot) {
	t.Helper()
	before := fixtureCatalog(t).Packages[0]
	after := before
	after.Package.Revision = fixtureCommit(t, strings.Repeat("b", 40))
	after.Package.Knowledge.Changed.Commit = after.Package.Revision
	return before, after
}

func noEvolutionMutation(testing.TB, *PackageSnapshot, *PackageSnapshot) {}

func mutateEvolutionRuntime(t testing.TB, _ *PackageSnapshot, after *PackageSnapshot) {
	after.Package.Knowledge.AuthorRuntime = fixtureName(t, "Go 1.28")
}

func mutateEvolutionGoPackage(_ testing.TB, _ *PackageSnapshot, after *PackageSnapshot) {
	after.Code.Inventory.GoPackages++
	after.Code.Inventory.Files++
}

func mutateEvolutionJavaScript(_ testing.TB, _ *PackageSnapshot, after *PackageSnapshot) {
	after.Code.Inventory.JavaScriptUnits++
	after.Code.Inventory.Files++
}

func mutateEvolutionFile(_ testing.TB, _ *PackageSnapshot, after *PackageSnapshot) {
	after.Code.Inventory.Files++
}

func mutateEvolutionTestFile(_ testing.TB, _ *PackageSnapshot, after *PackageSnapshot) {
	after.Code.Inventory.TestFiles++
	after.Code.Inventory.Files++
}

func mutateEvolutionDocument(_ testing.TB, _ *PackageSnapshot, after *PackageSnapshot) {
	after.Code.Inventory.Documents++
	after.Code.Inventory.Files++
}

func mutateEvolutionBenchmark(_ testing.TB, _ *PackageSnapshot, after *PackageSnapshot) {
	after.Code.Inventory.Benchmarks++
	after.Code.Inventory.TestDeclarations++
}

func mutateEvolutionFuzz(_ testing.TB, _ *PackageSnapshot, after *PackageSnapshot) {
	after.Code.Inventory.FuzzTargets++
	after.Code.Inventory.TestDeclarations++
}

func mutateEvolutionPackageKey(t testing.TB, _ *PackageSnapshot, after *PackageSnapshot) {
	after.Package.Key = fixtureIdentifier(t, "another-package")
}

func mutateEvolutionSubjectProject(_ testing.TB, _ *PackageSnapshot, after *PackageSnapshot) {
	after.Package.Subject.Project = core.Offering{Token: "another-product"}
	for index := range after.Evidence.Surfaces {
		after.Evidence.Surfaces[index].Subject = after.Package.Subject
	}
}

func mutateEvolutionSubjectRepository(t testing.TB, _ *PackageSnapshot, after *PackageSnapshot) {
	after.Package.Subject.Repository = fixtureRepository(t, "github.com/deliri/another")
	for index := range after.Evidence.Surfaces {
		after.Evidence.Surfaces[index].Subject = after.Package.Subject
	}
}

func mutateEvolutionGroup(t testing.TB, _ *PackageSnapshot, after *PackageSnapshot) {
	after.Package.GroupID = fixtureIdentifier(t, "another-group")
}

func mutateEvolutionLanguage(t testing.TB, _ *PackageSnapshot, after *PackageSnapshot) {
	after.Package.Language = fixtureName(t, "Another language")
}

func mutateEvolutionCodePath(t testing.TB, _ *PackageSnapshot, after *PackageSnapshot) {
	path := fixturePath(t, "another")
	after.Package.Knowledge.Path = path
	after.Code.Package = path
	after.Evidence.Package = path
}

func mutateEvolutionAnalyzerGeneration(t testing.TB, before, after *PackageSnapshot) {
	before.Code.SourceUsage = evolutionSourceUsage(t, *before, 0)
	after.Code.SourceUsage = evolutionSourceUsage(t, *after, 0)
	after.Code.SourceUsage.Generation = fixtureIdentifier(t, "source-analysis-v2")
}

func mutateEvolutionBeforeSchema(_ testing.TB, before, _ *PackageSnapshot) {
	before.SchemaVersion = 0
}

func mutateEvolutionAfterSchema(_ testing.TB, _ *PackageSnapshot, after *PackageSnapshot) {
	after.SchemaVersion = SchemaVersion + 1
}

func mutateEvolutionSourceRevision(t testing.TB, _ *PackageSnapshot, after *PackageSnapshot) {
	after.Code.SourceUsage = evolutionSourceUsage(t, *after, 0)
	after.Code.SourceUsage.Revision = fixtureCommit(t, strings.Repeat("c", 40))
}

func prepareFileBoundary(before, after *PackageSnapshot) {
	before.Code.Inventory = Inventory{GoPackages: 1, Files: 2}
	after.Code.Inventory = before.Code.Inventory
}

func mutateEvolutionFilesOne(_ testing.TB, before, after *PackageSnapshot) {
	prepareFileBoundary(before, after)
	after.Code.Inventory.Files = 1
}

func mutateEvolutionFilesTwo(_ testing.TB, before, after *PackageSnapshot) {
	prepareFileBoundary(before, after)
}

func mutateEvolutionFilesThree(_ testing.TB, before, after *PackageSnapshot) {
	prepareFileBoundary(before, after)
	after.Code.Inventory.Files = 3
}

func mutateEvolutionFilesExtreme(_ testing.TB, before, after *PackageSnapshot) {
	prepareFileBoundary(before, after)
	after.Code.Inventory.Files = math.MaxUint32
}

func mutateEvolutionDeclarationsZero(_ testing.TB, _ *PackageSnapshot, after *PackageSnapshot) {
	after.Code.Inventory.TestDeclarations = 0
}

func mutateEvolutionDeclarationsExact(_ testing.TB, _ *PackageSnapshot, after *PackageSnapshot) {
	after.Code.Inventory.TestDeclarations = 256
}

func mutateEvolutionDeclarationsAbove(_ testing.TB, _ *PackageSnapshot, after *PackageSnapshot) {
	after.Code.Inventory.TestDeclarations = 257
}

func mutateEvolutionDeclarationsExtreme(_ testing.TB, _ *PackageSnapshot, after *PackageSnapshot) {
	after.Code.Inventory.TestDeclarations = math.MaxUint32
}

func mutateEvolutionCoverageAbsent(_ testing.TB, _ *PackageSnapshot, _ *PackageSnapshot) {}

func mutateEvolutionCoverageZero(_ testing.TB, _ *PackageSnapshot, after *PackageSnapshot) {
	value := uint16(0)
	after.Code.Inventory.CoverageBasisPoints = &value
}

func mutateEvolutionCoverageExact(_ testing.TB, _ *PackageSnapshot, after *PackageSnapshot) {
	value := uint16(10_000)
	after.Code.Inventory.CoverageBasisPoints = &value
}

func mutateEvolutionCoverageAbove(_ testing.TB, _ *PackageSnapshot, after *PackageSnapshot) {
	value := uint16(10_001)
	after.Code.Inventory.CoverageBasisPoints = &value
}

func mutateEvolutionCandidateCountZero(t testing.TB, _ *PackageSnapshot, after *PackageSnapshot) {
	after.Code.SourceUsage = evolutionSourceUsage(t, *after, 0)
}

func mutateEvolutionOneCandidate(t testing.TB, _ *PackageSnapshot, after *PackageSnapshot) {
	after.Code.SourceUsage = evolutionSourceUsage(t, *after, 1)
}

func mutateEvolutionCandidateCountMaximum(t testing.TB, _ *PackageSnapshot, after *PackageSnapshot) {
	after.Code.SourceUsage = evolutionSourceUsage(t, *after, FunctionUsageMaximum)
}

func mutateEvolutionCandidateCountAbove(t testing.TB, _ *PackageSnapshot, after *PackageSnapshot) {
	after.Code.SourceUsage = evolutionSourceUsage(t, *after, FunctionUsageMaximum+1)
}

func evolutionSourceUsage(t testing.TB, snapshot PackageSnapshot, count int) *PackageSourceUsage {
	t.Helper()
	values := make([]FunctionUsage, count)
	for index := range values {
		symbol := fixtureName(t, fmt.Sprintf("candidate%02d", index))
		values[index] = FunctionUsage{
			Function:        CodeReference{Path: fixturePath(t, "projectstandards/evolution.go"), Symbol: &symbol},
			DeclarationLine: uint32(index + 1), ReferencePosture: FunctionNoReferenceObserved,
		}
	}
	return &PackageSourceUsage{
		Generation: fixtureIdentifier(t, "source-analysis-v1"), Revision: snapshot.Package.Revision,
		Package: snapshot.Code.Package, Completeness: SourceAnalysisComplete,
		DeclarationCount: uint32(count), NoReferenceObserved: uint32(count),
		ReviewCandidates: values, AnalyzedAt: temporal.InstantFromNanoseconds(3_000_000),
	}
}

func mutateEvolutionSurfacesZero(_ testing.TB, _ *PackageSnapshot, after *PackageSnapshot) {
	after.Evidence.Surfaces = nil
}

func mutateEvolutionSurfacesOne(_ testing.TB, _ *PackageSnapshot, _ *PackageSnapshot) {}

func mutateEvolutionSurfacesMaximum(t testing.TB, _ *PackageSnapshot, after *PackageSnapshot) {
	after.Evidence.Surfaces = evolutionSurfaces(t, after.Evidence.Surfaces[0], EvidenceSurfaceMaximum)
}

func mutateEvolutionSurfacesAbove(t testing.TB, _ *PackageSnapshot, after *PackageSnapshot) {
	after.Evidence.Surfaces = evolutionSurfaces(t, after.Evidence.Surfaces[0], EvidenceSurfaceMaximum+1)
}

func evolutionSurfaces(t testing.TB, template ProjectStandardsEvidenceSurface, count int) []ProjectStandardsEvidenceSurface {
	t.Helper()
	values := make([]ProjectStandardsEvidenceSurface, count)
	for index := range values {
		values[index] = template
		if index > 0 {
			values[index].ID = fixtureIdentifier(t, fmt.Sprintf("surface-%02d", index))
		}
		values[index].EligibleKinds = append([]ProbeKind(nil), template.EligibleKinds...)
		values[index].Profiles = append([]ProfileIdentity(nil), template.Profiles...)
	}
	return values
}

func evolutionFieldChange(got PackageEvolution, field evolutionField) CountChange {
	switch field {
	case evolutionFieldNone:
		return 0
	case evolutionFieldGoPackages:
		return got.Inventory.GoPackages
	case evolutionFieldJavaScriptUnits:
		return got.Inventory.JavaScriptUnits
	case evolutionFieldFiles:
		return got.Inventory.Files
	case evolutionFieldTestFiles:
		return got.Inventory.TestFiles
	case evolutionFieldDocuments:
		return got.Inventory.Documents
	case evolutionFieldTestDeclarations:
		return got.Inventory.TestDeclarations
	case evolutionFieldBenchmarks:
		return got.Inventory.Benchmarks
	case evolutionFieldFuzzTargets:
		return got.Inventory.FuzzTargets
	case evolutionFieldEvidenceSurfaces:
		return got.Evidence.Surfaces
	}
	return 0
}
