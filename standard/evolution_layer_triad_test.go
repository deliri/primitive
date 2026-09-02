package standard

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestPackageEvolutionMarkdownLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive report names both revisions and the newly observable function", func(t *testing.T) {
		t.Parallel()
		before, after := evolutionFixture(t)
		evolution, evolutionErr := ComparePackageSnapshots(before, after)
		var output bytes.Buffer
		writeErr := WritePackageEvolutionMarkdown(&output, evolution)
		if err := errors.Join(evolutionErr, writeErr); err != nil {
			t.Fatalf("WritePackageEvolutionMarkdown(new visibility) error = %v, want nil", err)
		}
		for _, want := range []string{before.Package.Revision.String(), after.Package.Revision.String(), "Before analysis:** unavailable", "After analysis:** available", "Newly observable.** standard/evolution.go:unusedHelper"} {
			if !strings.Contains(output.String(), want) {
				t.Fatalf("package evolution Markdown missing %q in %q, want the user-visible derived fact", want, output.String())
			}
		}
	})

	t.Run("negative invalid evolution cannot cross the report boundary", func(t *testing.T) {
		t.Parallel()
		var output bytes.Buffer
		err := WritePackageEvolutionMarkdown(&output, PackageEvolution{})
		if !errors.Is(err, core.ErrStandardContract) || output.Len() != 0 {
			t.Fatalf("WritePackageEvolutionMarkdown(invalid) = (bytes %d, error %v), want (0, %v identity)", output.Len(), err, core.ErrStandardContract)
		}
	})

	t.Run("neutral identical snapshots report no invented candidate movement", func(t *testing.T) {
		t.Parallel()
		snapshot := fixtureCatalog(t).Packages[0]
		evolution, evolutionErr := ComparePackageSnapshots(snapshot, snapshot)
		var output bytes.Buffer
		writeErr := WritePackageEvolutionMarkdown(&output, evolution)
		if err := errors.Join(evolutionErr, writeErr); err != nil {
			t.Fatalf("WritePackageEvolutionMarkdown(identical) error = %v, want nil", err)
		}
		if !strings.Contains(output.String(), "No review-candidate movement observed.") {
			t.Fatalf("identical package evolution Markdown = %q, want explicit no-movement fact", output.String())
		}
	})
}

func TestPackageEvolutionLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive second Standard exposes newly available source analysis and its exact review candidate", func(t *testing.T) {
		t.Parallel()
		before, after := evolutionFixture(t)

		got, err := ComparePackageSnapshots(before, after)
		if err != nil {
			t.Fatalf("ComparePackageSnapshots(new source visibility) error = %v, want nil", err)
		}
		if got.BeforeRevision != before.Package.Revision || got.AfterRevision != after.Package.Revision {
			t.Fatalf("package evolution revisions = (%v, %v), want (%v, %v)", got.BeforeRevision, got.AfterRevision, before.Package.Revision, after.Package.Revision)
		}
		if got.SourceUsage.BeforeAvailable || !got.SourceUsage.AfterAvailable || got.SourceUsage.Declarations != 1 || got.SourceUsage.NoReferenceObserved != 1 {
			t.Fatalf("source visibility change = %+v, want unavailable->available with one declaration and one no-reference fact", got.SourceUsage)
		}
		if len(got.NewReviewCandidates) != 1 || got.NewReviewCandidates[0].Function.Symbol == nil || got.NewReviewCandidates[0].Function.Symbol.String() != "unusedHelper" {
			t.Fatalf("new review candidates = %+v, want exactly unusedHelper from the second About", got.NewReviewCandidates)
		}
		if len(got.FormerReviewCandidates) != 0 {
			t.Fatalf("former review candidates = %+v, want none when the first Standard had no source analysis", got.FormerReviewCandidates)
		}
	})

	t.Run("negative a different analyzer generation cannot masquerade as package evolution", func(t *testing.T) {
		t.Parallel()
		before, after := evolutionFixture(t)
		beforeUsage := *after.Code.SourceUsage
		beforeUsage.Revision = before.Package.Revision
		before.Code.SourceUsage = &beforeUsage
		before.Code.SourceUsage.Generation = fixtureIdentifier(t, "source-analysis-v1")
		after.Code.SourceUsage.Generation = fixtureIdentifier(t, "source-analysis-v2")

		got, err := ComparePackageSnapshots(before, after)
		if !errors.Is(err, core.ErrStandardConflict) || len(got.NewReviewCandidates) != 0 || len(got.FormerReviewCandidates) != 0 {
			t.Fatalf("ComparePackageSnapshots(analyzer drift) = (%+v, %v), want (zero, %v identity)", got, err, core.ErrStandardConflict)
		}
	})

	t.Run("neutral identical Standard snapshots produce zero movement and no invented review candidate", func(t *testing.T) {
		t.Parallel()
		before := fixtureCatalog(t).Packages[0]

		got, err := ComparePackageSnapshots(before, before)
		if err != nil {
			t.Fatalf("ComparePackageSnapshots(identical snapshot) error = %v, want nil", err)
		}
		if got.Inventory != (InventoryChange{}) || got.Evidence != (EvidenceChange{}) || got.SourceUsage != (SourceUsageChange{}) {
			t.Fatalf("identical package evolution = (inventory %+v, evidence %+v, source %+v), want zero movement", got.Inventory, got.Evidence, got.SourceUsage)
		}
		if len(got.NewReviewCandidates) != 0 || len(got.FormerReviewCandidates) != 0 {
			t.Fatalf("identical package candidate movement = (new %+v, former %+v), want no invented movement", got.NewReviewCandidates, got.FormerReviewCandidates)
		}
	})
}

func TestPackageSnapshotRejectsSourceAnalysisFromAnotherRevision(t *testing.T) {
	t.Parallel()

	_, got := evolutionFixture(t)
	got.Code.SourceUsage.Revision = fixtureCommit(t, strings.Repeat("c", 40))

	err := got.Validate()
	if !errors.Is(err, core.ErrStandardConflict) {
		t.Fatalf("PackageSnapshot.Validate(source analysis from another revision) error = %v, want %v identity", err, core.ErrStandardConflict)
	}
}

func evolutionFixture(t testing.TB) (PackageSnapshot, PackageSnapshot) {
	t.Helper()
	before := fixtureCatalog(t).Packages[0]
	after := before
	after.Package.Revision = fixtureCommit(t, strings.Repeat("b", 40))
	after.Package.Knowledge.Changed.Commit = after.Package.Revision
	symbol := fixtureName(t, "unusedHelper")
	usage := FunctionUsage{
		Function:        CodeReference{Path: fixturePath(t, "standard/evolution.go"), Symbol: &symbol},
		DeclarationLine: 10, ReferencePosture: FunctionNoReferenceObserved,
	}
	after.Code.SourceUsage = &PackageSourceUsage{
		Generation: fixtureIdentifier(t, "source-analysis-v1"), Revision: after.Package.Revision,
		Package: after.Code.Package, Completeness: SourceAnalysisComplete,
		DeclarationCount: 1, NoReferenceObserved: 1,
		ReviewCandidates: []FunctionUsage{usage}, AnalyzedAt: temporal.InstantFromNanoseconds(2_000_000),
	}
	if err := errors.Join(before.Validate(), after.Validate()); err != nil {
		t.Fatalf("package evolution fixture validation error = %v, want nil", err)
	}
	return before, after
}
