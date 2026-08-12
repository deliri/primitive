package upgrade

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/release"
	"github.com/deliri/primitive/v2026/temporal"
)

// TestValidateUpgradePairAdmitsOnlyAStrictlyNewerSameTargetBuild exhausts the
// gate that decides whether one artifact may replace another. Every public
// entry point routes through it, and it had no direct coverage.
func TestValidateUpgradePairAdmitsOnlyAStrictlyNewerSameTargetBuild(t *testing.T) {
	t.Parallel()

	platform := core.Platform{
		OperatingSystem: core.OperatingSystemLinux,
		Architecture:    core.CPUArchitectureAMD64,
	}
	other := core.Platform{
		OperatingSystem: core.OperatingSystemWindows,
		Architecture:    core.CPUArchitectureAMD64,
	}
	if platform == other {
		other = core.Platform{
			OperatingSystem: core.OperatingSystemLinux,
			Architecture:    core.CPUArchitectureAMD64,
		}
	}

	installed := core.NewReleaseVersion(2, 4, 6)
	for _, tc := range []struct {
		name     string
		version  core.ReleaseVersion
		offering core.Offering
		platform core.Platform
		wantOK   bool
	}{
		{name: "one patch newer is admitted", version: core.NewReleaseVersion(2, 4, 7), wantOK: true},
		{name: "one minor newer is admitted", version: core.NewReleaseVersion(2, 5, 0), wantOK: true},
		{name: "one major newer is admitted", version: core.NewReleaseVersion(3, 0, 0), wantOK: true},
		{name: "a large major jump is admitted", version: core.NewReleaseVersion(99, 0, 0), wantOK: true},
		{name: "the maximum patch is admitted", version: core.NewReleaseVersion(2, 4, 65535), wantOK: true},
		{name: "a newer major with older minor and patch is admitted", version: core.NewReleaseVersion(3, 0, 0), wantOK: true},
		{name: "a newer minor with older patch is admitted", version: core.NewReleaseVersion(2, 5, 0), wantOK: true},
		{name: "the identical version is not an upgrade", version: installed},
		{name: "one patch older is rejected", version: core.NewReleaseVersion(2, 4, 5)},
		{name: "one minor older is rejected", version: core.NewReleaseVersion(2, 3, 6)},
		{name: "one major older is rejected", version: core.NewReleaseVersion(1, 4, 6)},
		{name: "the zero version is rejected", version: core.NewReleaseVersion(0, 0, 0)},
		{name: "an older major with newer minor and patch is rejected", version: core.NewReleaseVersion(1, 99, 99)},
		{name: "an older minor with newer patch is rejected", version: core.NewReleaseVersion(2, 3, 99)},
		{
			name:    "a newer version of a different offering is rejected",
			version: core.NewReleaseVersion(3, 0, 0), offering: core.OfferingWitness,
		},
		{
			name:    "a newer version for a different platform is rejected",
			version: core.NewReleaseVersion(3, 0, 0), platform: other,
		},
		{
			name:    "the identical version of a different offering is rejected",
			version: installed, offering: core.OfferingWitness,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			offering := tc.offering
			if offering == core.OfferingUnknown {
				offering = core.OfferingBug
			}
			target := tc.platform
			if target == (core.Platform{}) {
				target = platform
			}
			from := buildArtifactForTest(
				t, []byte("installed"), core.OfferingBug, installed, platform,
			)
			to := buildArtifactForTest(
				t, []byte("candidate"), offering, tc.version, target,
			)
			err := validateUpgradePair(from, to)
			if tc.wantOK {
				if err != nil {
					t.Fatalf("validateUpgradePair() error = %v, want nil", err)
				}
				return
			}
			requireRejection(
				t, err, core.ErrUpgradeContract, diagnosticUnknown,
			)
		})
	}
	peachfuzzInstalled := buildArtifactForTest(
		t, []byte("peachfuzz-installed"), core.OfferingPeachfuzz, installed, platform,
	)
	peachfuzzCandidate := buildArtifactForTest(
		t, []byte("peachfuzz-candidate"), core.OfferingPeachfuzz,
		core.NewReleaseVersion(2, 4, 7), platform,
	)
	if err := validateUpgradePair(peachfuzzInstalled, peachfuzzCandidate); err != nil {
		t.Fatalf("validateUpgradePair(Peachfuzz) error = %v, want nil", err)
	}

	if err := validateUpgradePair(
		release.Artifact{}, artifactForTest(t, []byte("candidate"), 2),
	); !errors.Is(err, core.ErrUpgradeContract) {
		t.Fatalf("validateUpgradePair(zero installed) error = %v, want %v",
			err, core.ErrUpgradeContract)
	}
	if err := validateUpgradePair(
		artifactForTest(t, []byte("installed"), 1), release.Artifact{},
	); !errors.Is(err, core.ErrUpgradeContract) {
		t.Fatalf("validateUpgradePair(zero candidate) error = %v, want %v",
			err, core.ErrUpgradeContract)
	}
}

// TestFailedSlotCreationIsReportedAsPersistenceNotCleanup ratchets the phase
// honesty contract. Creating the candidate slot is not cleanup, and reporting
// it as cleanup made an operator believe the failure came after the bytes
// landed.
func TestFailedSlotCreationIsReportedAsPersistenceNotCleanup(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatalf("os.OpenRoot() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = root.Close() })

	oldBytes := []byte("old")
	oldArtifact := artifactForTest(t, oldBytes, 1)
	candidate := artifactForTest(t, []byte("candidate"), 2)
	installArtifactForTest(t, root, SlotA, oldArtifact, oldBytes)
	prior := selectionDocument{
		Revision: selectionRevisionCurrent, Slot: SlotA, Artifact: oldArtifact,
	}
	target, err := newTrialTarget(absolutePathForTest(t, directory), prior, candidate)
	if err != nil {
		t.Fatalf("newTrialTarget() error = %v, want nil", err)
	}
	// A regular file occupying the slot name makes directory creation fail.
	if err := root.WriteFile(SlotB.String(), []byte("not a directory"), documentMode); err != nil {
		t.Fatalf("os.Root.WriteFile(blocking file) error = %v, want nil", err)
	}

	installErr := installCandidate(
		t.Context(), StageRequest{Root: root, Directory: absolutePathForTest(t, directory)},
		target,
	)
	var attempt AttemptError
	if !errors.As(installErr, &attempt) {
		t.Fatalf("installCandidate() error = %v, want an AttemptError", installErr)
	}
	if attempt.Phase() != FailurePhasePersistence {
		t.Fatalf("failure phase = %v, want %v", attempt.Phase(), FailurePhasePersistence)
	}
	if !errors.Is(installErr, core.ErrUpgradePersistence) {
		t.Fatalf("installCandidate() error = %v, want %v",
			installErr, core.ErrUpgradePersistence)
	}
	if errors.Is(installErr, core.ErrUpgradeCleanup) {
		t.Fatalf("installCandidate() error = %v, want no cleanup identity", installErr)
	}
	if attempt.Candidate() != candidate.Build() {
		t.Fatalf("attempt candidate = %v, want %v",
			attempt.Candidate(), candidate.Build())
	}
	if err := attempt.Validate(); err != nil {
		t.Fatalf("AttemptError.Validate() error = %v, want nil", err)
	}
}

// TestSettledCleanupCompletesAfterTheCallerContextIsDone proves the proof and
// removal paths that run after their own effect already landed. A cancelled
// caller must not turn a committed selector into a reported failure or strand
// the former slot, because the next candidate needs it.
func TestSettledCleanupCompletesAfterTheCallerContextIsDone(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatalf("os.OpenRoot() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = root.Close() })

	oldBytes, newBytes := []byte("old"), []byte("new")
	oldArtifact := artifactForTest(t, oldBytes, 1)
	newArtifact := artifactForTest(t, newBytes, 2)
	installArtifactForTest(t, root, SlotA, oldArtifact, oldBytes)
	installArtifactForTest(t, root, SlotB, newArtifact, newBytes)
	prior := selectionDocument{
		Revision: selectionRevisionCurrent, Slot: SlotA, Artifact: oldArtifact,
	}
	if err := writeSelection(t.Context(), root, prior, filestore.InstallCreate); err != nil {
		t.Fatalf("writeSelection(prior) error = %v, want nil", err)
	}
	target, err := newTrialTarget(absolutePathForTest(t, directory), prior, newArtifact)
	if err != nil {
		t.Fatalf("newTrialTarget() error = %v, want nil", err)
	}
	if _, err := ensureTrialReceipt(t.Context(), root, target); err != nil {
		t.Fatalf("ensureTrialReceipt() error = %v, want nil", err)
	}
	promotion, err := CompleteTrial(TrialReport{
		Target: target, Observed: newArtifact.Build(),
		Observation: temporal.InstantFromNanoseconds(10), Outcome: TrialPassed,
	})
	if err != nil {
		t.Fatalf("CompleteTrial() error = %v, want nil", err)
	}

	// Promote end to end. Whether the post-commit removal itself carries
	// recoveryContext is a compiler-visible fact proved by
	// TestEverySettledRemovalCarriesTheRecoveryContext; no in-package
	// behavioural test can reach that call with a done context.
	cancellable, cancel := context.WithCancel(t.Context())
	primary, err := Promote(cancellable, PromoteRequest{
		Root: root, Directory: absolutePathForTest(t, directory), Promotion: promotion,
	})
	if err != nil {
		t.Fatalf("Promote() error = %v, want nil", err)
	}
	cancel()
	if primary.Slot() != SlotB {
		t.Fatalf("promoted slot = %v, want %v", primary.Slot(), SlotB)
	}

	committed, committedCancel := context.WithCancel(t.Context())
	committedCancel()
	resolved, err := resolveCommittedPrimary(committed, ResolveRequest{
		Root: root, Directory: absolutePathForTest(t, directory),
	})
	if err != nil || resolved != primary {
		t.Fatalf("resolveCommittedPrimary(done context) = (%v, %v), want (%v, nil)",
			resolved, err, primary)
	}

	// Direct ratchet on the settled-removal helper itself: a done context must
	// not stop it, because the effect it settles already happened.
	directory2 := t.TempDir()
	root2, err := os.OpenRoot(directory2)
	if err != nil {
		t.Fatalf("os.OpenRoot() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = root2.Close() })
	installArtifactForTest(t, root2, SlotA, oldArtifact, oldBytes)
	done, cancel2 := context.WithCancel(t.Context())
	cancel2()
	if err := removeArtifact(
		recoveryContext(done), root2, SlotA, oldArtifact,
	); err != nil {
		t.Fatalf("removeArtifact(settled) error = %v, want nil", err)
	}
	path, err := binaryPath(SlotA, oldArtifact.Build())
	if err != nil {
		t.Fatalf("binaryPath() error = %v, want nil", err)
	}
	if _, err := root2.Stat(path.String()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("settled removal stat error = %v, want %v", err, os.ErrNotExist)
	}
	// The same removal under the raw done context is refused, which is the
	// reason recoveryContext exists at all.
	installArtifactForTest(t, root2, SlotA, oldArtifact, oldBytes)
	if err := removeArtifact(done, root2, SlotA, oldArtifact); !errors.Is(err, context.Canceled) {
		t.Fatalf("removeArtifact(done context) error = %v, want %v", err, context.Canceled)
	}
}

// TestSlotProjectionsRefuseEveryValueOutsideTheClosedDomain proves that no
// invalid slot can ever name a directory, path, or command.
func TestSlotProjectionsRefuseEveryValueOutsideTheClosedDomain(t *testing.T) {
	t.Parallel()

	directory := absolutePathForTest(t, t.TempDir())
	build := artifactForTest(t, []byte("candidate"), 2).Build()
	for raw := 0; raw <= 255; raw++ {
		slot := Slot(raw)
		if slot.IsValid() {
			continue
		}
		if _, err := slotPath(slot); !errors.Is(err, core.ErrUpgradeContract) {
			t.Fatalf("slotPath(Slot(%d)) error = %v, want %v",
				raw, err, core.ErrUpgradeContract)
		}
		if _, err := binaryPath(slot, build); !errors.Is(err, core.ErrUpgradeContract) {
			t.Fatalf("binaryPath(Slot(%d)) error = %v, want %v",
				raw, err, core.ErrUpgradeContract)
		}
		if got, err := absoluteBinaryPath(directory, slot, build); !errors.Is(err, core.ErrUpgradeContract) || got != (core.AbsolutePath{}) {
			t.Fatalf("absoluteBinaryPath(Slot(%d)) = (%v, %v), want zero and %v", raw, got, err, core.ErrUpgradeContract)
		}
		if _, err := slot.other(); !errors.Is(err, core.ErrUpgradeContract) {
			t.Fatalf("Slot(%d).other() error = %v, want %v",
				raw, err, core.ErrUpgradeContract)
		}
	}
	for _, slot := range []Slot{SlotA, SlotB} {
		opposite, err := slot.other()
		if err != nil {
			t.Fatalf("Slot(%v).other() error = %v, want nil", slot, err)
		}
		roundTrip, err := opposite.other()
		if err != nil || roundTrip != slot {
			t.Fatalf("Slot(%v).other().other() = (%v, %v), want (%v, nil)",
				slot, roundTrip, err, slot)
		}
		if opposite == slot {
			t.Fatalf("Slot(%v).other() = %v, want the opposite slot", slot, opposite)
		}
	}
}
