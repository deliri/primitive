package release

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestAdvanceLatestIdentityPrecedesGeneration(t *testing.T) {
	t.Parallel()
	retained := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 9)
	other := newReleaseFixtureForOffering(
		t, releaseOffering(t, 1), core.NewReleaseVersion(2026, 7, 31), 1,
	)

	_, err := AdvanceLatest(AdvanceLatestRequest{
		Retained: retained.verifiedLatest, Proposed: other.verifiedLatest,
	})
	if !errors.Is(err, core.ErrReleaseConflict) || errors.Is(err, core.ErrReleaseRollback) {
		t.Fatalf("AdvanceLatest(cross offering lower generation) error = %v, want conflict before rollback", err)
	}
}

func TestAdvanceLatestFullOrderRatchets(t *testing.T) {
	t.Parallel()
	retained := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 5)

	replay, err := AdvanceLatest(AdvanceLatestRequest{Retained: retained.verifiedLatest, Proposed: retained.verifiedLatest})
	if err != nil || replay.State() != LatestAdvanceReplay {
		t.Fatalf("AdvanceLatest(replay) = (%v, %v), want (%v, nil)", replay.State(), err, LatestAdvanceReplay)
	}

	lower := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 31), 4)
	_, err = AdvanceLatest(AdvanceLatestRequest{Retained: retained.verifiedLatest, Proposed: lower.verifiedLatest})
	if !errors.Is(err, core.ErrReleaseRollback) {
		t.Fatalf("AdvanceLatest(lower generation) error = %v, want %v", err, core.ErrReleaseRollback)
	}

	next := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 31), 6)
	advanced, err := AdvanceLatest(AdvanceLatestRequest{Retained: retained.verifiedLatest, Proposed: next.verifiedLatest})
	if err != nil || advanced.State() != LatestAdvanceAdvanced {
		t.Fatalf("AdvanceLatest(greater version) = (%v, %v), want (%v, nil)", advanced.State(), err, LatestAdvanceAdvanced)
	}
}

func TestEvaluateSelectionUsesClosedInstalledIdentity(t *testing.T) {
	t.Parallel()
	installed := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 1)
	candidate := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 31), 2)

	cached, err := NewCachedLatest(candidate.verifiedLatest)
	if err != nil {
		t.Fatalf("NewCachedLatest() error = %v", err)
	}
	got, err := evaluateWithInstalled(EvaluateRequest{
		InstalledManifest: installed.verified,
		Latest:            cached,
		Time:              latestTimeEvidenceAt(t, 3_000),
	}, installed.builds[2])
	if err != nil {
		t.Fatalf("evaluateWithInstalled() error = %v", err)
	}
	available, ok := got.Available()
	if !ok {
		t.Fatalf("Result.Available() ok = false, state = %v", got.State())
	}
	preparation, err := available.Prepare(latestTimeEvidenceAt(t, 3_001))
	if err != nil {
		t.Fatalf("AvailableRelease.PrepareAt() error = %v", err)
	}
	prepared, ok := preparation.Ready()
	if !ok || prepared.Validate() != nil {
		t.Fatalf("Preparation.Ready() = (%v, %v), want valid proof", prepared, ok)
	}

	outsideTarget := core.Platform{
		OperatingSystem: core.OperatingSystemDarwin,
		Architecture:    core.CPUArchitectureAMD64,
	}
	if err := outsideTarget.Validate(); err != nil {
		t.Fatalf("outside core.Platform.Validate() error = %v", err)
	}
	differentInstallation, err := core.NewBuildIdentity(core.BuildIdentityRequest{
		Offering: installed.builds[2].Offering(),
		Version:  installed.builds[2].Version(),
		Commit:   installed.builds[2].Commit(),
		Platform: outsideTarget,
	})
	if err != nil {
		t.Fatalf("core.NewBuildIdentity(outside target set) error = %v", err)
	}
	_, err = evaluateWithInstalled(EvaluateRequest{
		InstalledManifest: installed.verified,
		Latest:            cached,
		Time:              latestTimeEvidenceAt(t, 3_000),
	}, differentInstallation)
	if !errors.Is(err, core.ErrReleaseConflict) {
		t.Fatalf("evaluateWithInstalled(wrong platform identity) error = %v, want %v", err, core.ErrReleaseConflict)
	}
}
