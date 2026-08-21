package upgrade

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"hash/crc32"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/release"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestUpgradeDurableWriterLayerTriadSelectionDocumentCanonicalClosure(t *testing.T) {
	t.Parallel()

	artifact := artifactForTest(t, []byte("old"), 1)
	document := selectionDocument{
		Revision: selectionRevisionCurrent,
		Slot:     SlotA,
		Artifact: artifact,
	}
	encoded, err := encodeSelection(document)
	if err != nil {
		t.Fatalf("encodeSelection() error = %v, want nil", err)
	}
	decoded, err := decodeSelection(encoded)
	if err != nil || decoded != document {
		t.Fatalf("decodeSelection(encode) = (%v, %v), want (%v, nil)", decoded, err, document)
	}

	if len(encoded) >= selectionDocumentMaximumBytes {
		t.Fatalf("canonical selection extent = %d, want < the owned bound %d",
			len(encoded), selectionDocumentMaximumBytes)
	}

	swapped := bytes.Replace(encoded,
		[]byte(`"revision":1,"slot":`), []byte(`"slot":`), 1)
	swapped = append(swapped[:len(swapped)-1], []byte(`,"revision":1}`)...)

	for _, tc := range []struct {
		name string
		data []byte
	}{
		{name: "nil document", data: nil},
		{name: "empty document", data: []byte(``)},
		{name: "truncated final byte", data: encoded[:len(encoded)-1]},
		{name: "truncated to half", data: encoded[:len(encoded)/2]},
		{name: "trailing space breaks canonical equality", data: append(slices.Clone(encoded), ' ')},
		{name: "trailing newline breaks canonical equality", data: append(slices.Clone(encoded), '\n')},
		{name: "leading space breaks canonical equality", data: append([]byte{' '}, encoded...)},
		{name: "trailing garbage after the document", data: append(slices.Clone(encoded), '{')},
		{name: "two concatenated documents", data: append(slices.Clone(encoded), encoded...)},
		{name: "empty object omits every field", data: []byte(`{}`)},
		{name: "json null", data: []byte(`null`)},
		{name: "json array", data: []byte(`[]`)},
		{name: "json string", data: []byte(`"slot-a"`)},
		{name: "json number", data: []byte(`1`)},
		{name: "null artifact", data: []byte(`{"revision":1,"slot":"slot-a","artifact":null}`)},
		{name: "empty artifact object", data: []byte(`{"revision":1,"slot":"slot-a","artifact":{}}`)},
		{name: "unknown slot token", data: []byte(`{"revision":1,"slot":"slot-c","artifact":{}}`)},
		{name: "bare slot letter is not the wire token", data: []byte(`{"revision":1,"slot":"a","artifact":{}}`)},
		{name: "slot encoded as its ordinal", data: []byte(`{"revision":1,"slot":1,"artifact":{}}`)},
		{name: "revision zero is below the only revision", data: []byte(`{"revision":0,"slot":"slot-a","artifact":{}}`)},
		{name: "revision two is above the only revision", data: []byte(`{"revision":2,"slot":"slot-a","artifact":{}}`)},
		{name: "revision as a string token", data: []byte(`{"revision":"1","slot":"slot-a","artifact":{}}`)},
		{name: "revision one point zero is not the integer one", data: []byte(`{"revision":1.0,"slot":"slot-a","artifact":{}}`)},
		{name: "reordered fields are not canonical", data: swapped},
		{name: "duplicate revision field", data: bytes.Replace(encoded,
			[]byte(`{"revision":1,`), []byte(`{"revision":1,"revision":1,`), 1)},
		{name: "unknown extra field", data: bytes.Replace(encoded,
			[]byte(`{"revision":1,`), []byte(`{"future":true,"revision":1,`), 1)},
		{name: "revision field removed", data: bytes.Replace(encoded,
			[]byte(`"revision":1,`), nil, 1)},
		{name: "document exceeds the owned bound", data: append(
			slices.Clone(encoded),
			bytes.Repeat([]byte("x"), selectionDocumentMaximumBytes)...,
		)},
		{name: "nesting bomb", data: append(
			bytes.Repeat([]byte(`{"artifact":`), 200),
			bytes.Repeat([]byte(`}`), 200)...,
		)},
		{name: "invalid utf8 in the slot token", data: []byte(
			"{\"revision\":1,\"slot\":\"\xff\xfe\",\"artifact\":{}}")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, decodeErr := decodeSelection(tc.data)
			requireRejection(
				t, decodeErr, core.ErrUpgradeContract, diagnosticUnknown,
			)
			if !errors.Is(decodeErr, core.ErrJSONContract) {
				t.Fatalf("decodeSelection() error = %v, want %v",
					decodeErr, core.ErrJSONContract)
			}
			if got != (selectionDocument{}) {
				t.Fatalf("decodeSelection() = %v, want the zero document", got)
			}
		})
	}
}

func TestSelectionWriteReclaimsOnlyItsFixedCrashTemporary(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatalf("os.OpenRoot() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = root.Close() })

	document := selectionDocument{
		Revision: selectionRevisionCurrent,
		Slot:     SlotA,
		Artifact: artifactForTest(t, []byte("primary"), 1),
	}
	temporary, err := selectionTemporaryPath()
	if err != nil {
		t.Fatalf("selectionTemporaryPath() error = %v, want nil", err)
	}
	if err := root.WriteFile(
		temporary.String(), []byte("interrupted selector"), documentMode,
	); err != nil {
		t.Fatalf("os.Root.WriteFile(stale temporary) error = %v, want nil", err)
	}
	if err := writeSelection(
		t.Context(), root, document, filestore.InstallCreate,
	); err != nil {
		t.Fatalf("writeSelection(after crash temporary) error = %v, want nil", err)
	}
	got, err := readSelection(t.Context(), root)
	if err != nil || got != document {
		t.Fatalf("readSelection() = (%v, %v), want (%v, nil)",
			got, err, document)
	}
	if _, err := root.Stat(temporary.String()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("selection temporary stat error = %v, want %v",
			err, os.ErrNotExist)
	}
}

// requireRejection proves the load-bearing typed class first and only then the
// operator-facing diagnostic. The diagnostic never stands alone as the
// rejection proof, so a regression to a different typed error that happens to
// render the same prose still fails.
func requireRejection(
	t *testing.T,
	got error,
	wantIdentity error,
	wantDiagnostic diagnostic,
) {
	t.Helper()
	if !errors.Is(got, wantIdentity) {
		t.Fatalf("error = %v, want identity %v", got, wantIdentity)
	}
	if wantDiagnostic != diagnosticUnknown &&
		!errors.Is(got, wantDiagnostic) {
		t.Fatalf("error = %v, want typed diagnostic %v", got, wantDiagnostic)
	}
}

func TestTrialReportBindsExactCandidateAndProducesPromotionOnlyOnPass(t *testing.T) {
	t.Parallel()

	oldArtifact := artifactForTest(t, []byte("old"), 1)
	newArtifact := artifactForTest(t, []byte("new"), 2)
	root := absolutePathForTest(t, t.TempDir())
	target, err := newTrialTarget(
		root,
		selectionDocument{
			Revision: selectionRevisionCurrent,
			Slot:     SlotA,
			Artifact: oldArtifact,
		},
		newArtifact,
	)
	if err != nil {
		t.Fatalf("newTrialTarget() error = %v, want nil", err)
	}
	wantCommand, err := core.ParseAbsolutePath(
		filepath.Join(root.String(), target.Path().String()),
	)
	if err != nil || target.Command() != wantCommand {
		t.Fatalf("trial command projection = (%v, %v), want %v",
			target.Command(), err, wantCommand)
	}
	observation := temporal.InstantFromNanoseconds(7)

	promotion, err := CompleteTrial(TrialReport{
		Target:      target,
		Observed:    newArtifact.Build(),
		Observation: observation,
		Outcome:     TrialPassed,
	})
	if err != nil || promotion.Validate() != nil {
		t.Fatalf("CompleteTrial(passed) = (%v, %v), want valid promotion", promotion, err)
	}

	for _, tc := range []struct {
		wantIdentity   error
		name           string
		report         TrialReport
		wantDiagnostic diagnostic
	}{
		{
			name: "zero report names an unset trial target", report: TrialReport{},
			wantIdentity: core.ErrUpgradeContract, wantDiagnostic: diagnosticTrialTarget,
		},
		{
			name: "unset outcome names the closed outcome domain",
			report: TrialReport{
				Target: target, Observed: newArtifact.Build(), Observation: observation,
			},
			wantIdentity: core.ErrUpgradeContract, wantDiagnostic: diagnosticTrialOutcome,
		},
		{
			name: "outcome above the closed domain names the same rejection",
			report: TrialReport{
				Target: target, Observed: newArtifact.Build(),
				Observation: observation, Outcome: TrialOutcome(255),
			},
			wantIdentity: core.ErrUpgradeContract, wantDiagnostic: diagnosticTrialOutcome,
		},
		{
			name: "a pass observed on the primary build is not a candidate trial",
			report: TrialReport{
				Target: target, Observed: oldArtifact.Build(),
				Observation: observation, Outcome: TrialPassed,
			},
			wantIdentity: core.ErrUpgradeContract, wantDiagnostic: diagnosticObservedBuild,
		},
		{
			name: "unset observed build cannot bind the candidate",
			report: TrialReport{
				Target: target, Observation: observation, Outcome: TrialPassed,
			},
			wantIdentity: core.ErrUpgradeContract, wantDiagnostic: diagnosticTrialReport,
		},
		{
			name: "zero observation instant is rejected before the outcome",
			report: TrialReport{
				Target: target, Observed: newArtifact.Build(), Outcome: TrialPassed,
			},
			wantIdentity: core.ErrUpgradeContract, wantDiagnostic: diagnosticTrialReport,
		},
		{
			name: "a failed trial produces the typed trial identity",
			report: TrialReport{
				Target: target, Observed: newArtifact.Build(),
				Observation: observation, Outcome: TrialFailed,
			},
			wantIdentity: core.ErrUpgradeTrial,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, trialErr := CompleteTrial(tc.report)
			requireRejection(t, trialErr, tc.wantIdentity, tc.wantDiagnostic)
			if got != (Promotion{}) {
				t.Fatalf("CompleteTrial() = %v, want the zero promotion", got)
			}
		})
	}

	failed, trialErr := CompleteTrial(TrialReport{
		Target: target, Observed: newArtifact.Build(),
		Observation: observation, Outcome: TrialFailed,
	})
	var attempt AttemptError
	if failed != (Promotion{}) ||
		!errors.Is(trialErr, core.ErrUpgradeTrial) ||
		!errors.As(trialErr, &attempt) ||
		attempt.Phase() != FailurePhaseTrial ||
		attempt.Candidate() != newArtifact.Build() ||
		attempt.Validate() != nil {
		t.Fatalf("CompleteTrial(failed) = (%v, %v, %+v), want typed trial attempt",
			failed, trialErr, attempt)
	}
}

func TestPromoteChangesOnlyTheSelectorThenRemovesTheOldFixedSlot(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatalf("os.OpenRoot() error = %v, want nil", err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Errorf("os.Root.Close() error = %v, want nil", err)
		}
	})

	oldBytes := []byte("old executable")
	newBytes := []byte("new executable")
	oldArtifact := artifactForTest(t, oldBytes, 1)
	newArtifact := artifactForTest(t, newBytes, 2)
	installArtifactForTest(t, root, SlotA, oldArtifact, oldBytes)
	installArtifactForTest(t, root, SlotB, newArtifact, newBytes)
	prior := selectionDocument{
		Revision: selectionRevisionCurrent,
		Slot:     SlotA,
		Artifact: oldArtifact,
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

	primary, err := Promote(t.Context(), PromoteRequest{
		Root: root, Directory: absolutePathForTest(t, directory), Promotion: promotion,
	})
	if err != nil {
		t.Fatalf("Promote() error = %v, want nil", err)
	}
	if primary.Artifact() != newArtifact || primary.Slot() != SlotB {
		t.Fatalf("Promote() primary = %v/%v, want candidate in slot B",
			primary.Artifact(), primary.Slot())
	}
	wantCommand, err := core.ParseAbsolutePath(
		filepath.Join(directory, primary.Path().String()),
	)
	if err != nil || primary.Command() != wantCommand {
		t.Fatalf("primary command projection = (%v, %v), want %v",
			primary.Command(), err, wantCommand)
	}
	resolved, err := ResolvePrimary(t.Context(), ResolveRequest{
		Root: root, Directory: absolutePathForTest(t, directory),
	})
	if err != nil || resolved.Artifact() != newArtifact {
		t.Fatalf("ResolvePrimary() = (%v, %v), want new artifact", resolved, err)
	}
	oldPath, err := binaryPath(SlotA, oldArtifact.Build())
	if err != nil {
		t.Fatalf("binaryPath(old) error = %v, want nil", err)
	}
	if _, err := root.Stat(oldPath.String()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old slot stat error = %v, want %v", err, os.ErrNotExist)
	}
	trialReceipt, err := trialPath(SlotB)
	if err != nil {
		t.Fatalf("trialPath(promoted) error = %v, want nil", err)
	}
	if _, err := root.Stat(trialReceipt.String()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("promoted trial receipt stat error = %v, want %v",
			err, os.ErrNotExist)
	}
}

func TestPromotionReverifiesCandidateAndLeavesPrimaryUntouchedOnFailure(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatalf("os.OpenRoot() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = root.Close() })

	oldArtifact := artifactForTest(t, []byte("old"), 1)
	newArtifact := artifactForTest(t, []byte("new"), 2)
	installArtifactForTest(t, root, SlotA, oldArtifact, []byte("old"))
	installArtifactForTest(t, root, SlotB, newArtifact, []byte("tampered"))
	prior := selectionDocument{
		Revision: selectionRevisionCurrent,
		Slot:     SlotA,
		Artifact: oldArtifact,
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

	primary, promoteErr := Promote(t.Context(), PromoteRequest{
		Root: root, Directory: absolutePathForTest(t, directory), Promotion: promotion,
	})
	if !errors.Is(promoteErr, core.ErrUpgradeVerification) ||
		primary != (Primary{}) {
		t.Fatalf("Promote(tampered) = (%v, %v), want zero/%v",
			primary, promoteErr, core.ErrUpgradeVerification)
	}
	resolved, err := ResolvePrimary(t.Context(), ResolveRequest{
		Root: root, Directory: absolutePathForTest(t, directory),
	})
	if err != nil || resolved.Artifact() != oldArtifact {
		t.Fatalf("ResolvePrimary(after refusal) = (%v, %v), want old artifact", resolved, err)
	}
}

func TestPromotionConflictNeverOverwritesAChangedPrimary(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatalf("os.OpenRoot() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = root.Close() })

	oldArtifact := artifactForTest(t, []byte("old"), 1)
	newArtifact := artifactForTest(t, []byte("new"), 2)
	installArtifactForTest(t, root, SlotA, oldArtifact, []byte("old"))
	installArtifactForTest(t, root, SlotB, newArtifact, []byte("new"))
	prior := selectionDocument{
		Revision: selectionRevisionCurrent, Slot: SlotA, Artifact: oldArtifact,
	}
	if err := writeSelection(
		t.Context(), root, prior, filestore.InstallCreate,
	); err != nil {
		t.Fatalf("writeSelection(prior) error = %v, want nil", err)
	}
	target, err := newTrialTarget(absolutePathForTest(t, directory), prior, newArtifact)
	if err != nil {
		t.Fatalf("newTrialTarget() error = %v, want nil", err)
	}
	promotion, err := CompleteTrial(TrialReport{
		Target: target, Observed: newArtifact.Build(),
		Observation: temporal.InstantFromNanoseconds(10), Outcome: TrialPassed,
	})
	if err != nil {
		t.Fatalf("CompleteTrial() error = %v, want nil", err)
	}
	changed := selectionDocument{
		Revision: selectionRevisionCurrent, Slot: SlotB, Artifact: newArtifact,
	}
	if err := writeSelection(
		t.Context(), root, changed, filestore.InstallReplace,
	); err != nil {
		t.Fatalf("writeSelection(changed) error = %v, want nil", err)
	}

	primary, promoteErr := Promote(t.Context(), PromoteRequest{
		Root: root, Directory: absolutePathForTest(t, directory), Promotion: promotion,
	})
	if primary != (Primary{}) ||
		!errors.Is(promoteErr, core.ErrUpgradeConflict) {
		t.Fatalf("Promote(stale) = (%v, %v), want zero/%v",
			primary, promoteErr, core.ErrUpgradeConflict)
	}
	resolved, err := ResolvePrimary(t.Context(), ResolveRequest{
		Root: root, Directory: absolutePathForTest(t, directory),
	})
	if err != nil || resolved.Artifact() != newArtifact {
		t.Fatalf("ResolvePrimary(after conflict) = (%v, %v), want changed primary",
			resolved, err)
	}
}

func TestStaleTrialTargetCannotMutateADifferentDurableTrial(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		run  func(context.Context, *os.Root, core.AbsolutePath, TrialTarget) error
		name string
	}{
		{
			name: "promotion cannot select the stale in-memory target",
			run: func(
				ctx context.Context,
				root *os.Root,
				directory core.AbsolutePath,
				target TrialTarget,
			) error {
				promotion, err := CompleteTrial(TrialReport{
					Target: target, Observed: target.candidate.Build(),
					Observation: temporal.InstantFromNanoseconds(10),
					Outcome:     TrialPassed,
				})
				if err != nil {
					return err
				}
				_, err = Promote(ctx, PromoteRequest{
					Root: root, Directory: directory, Promotion: promotion,
				})
				return err
			},
		},
		{
			name: "discard cannot remove the stale in-memory target",
			run: func(
				ctx context.Context,
				root *os.Root,
				directory core.AbsolutePath,
				target TrialTarget,
			) error {
				return DiscardTrial(ctx, DiscardTrialRequest{
					Root: root, Directory: directory, Target: target,
				})
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			directory := t.TempDir()
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatalf("os.OpenRoot() error = %v, want nil", err)
			}
			t.Cleanup(func() { _ = root.Close() })

			sharedBytes := []byte("same bytes, different signed builds")
			primary := artifactForTest(t, []byte("old"), 1)
			stale := artifactForTest(t, sharedBytes, 2)
			active := artifactForTest(t, sharedBytes, 3)
			prior := selectionDocument{
				Revision: selectionRevisionCurrent,
				Slot:     SlotA,
				Artifact: primary,
			}
			if err := writeSelection(
				t.Context(), root, prior, filestore.InstallCreate,
			); err != nil {
				t.Fatalf("writeSelection(prior) error = %v, want nil", err)
			}
			installArtifactForTest(t, root, SlotA, primary, []byte("old"))
			installArtifactForTest(t, root, SlotB, stale, sharedBytes)
			staleTarget, err := newTrialTarget(
				absolutePathForTest(t, directory), prior, stale,
			)
			if err != nil {
				t.Fatalf("newTrialTarget(stale) error = %v, want nil", err)
			}
			activeDocument := trialDocument{
				Revision: trialRevisionCurrent,
				Prior:    prior, Candidate: active,
			}
			if err := writeTrial(
				t.Context(), root, SlotB, activeDocument,
			); err != nil {
				t.Fatalf("writeTrial(active) error = %v, want nil", err)
			}

			gotErr := tc.run(
				t.Context(), root,
				absolutePathForTest(t, directory), staleTarget,
			)
			requireRejection(
				t, gotErr, core.ErrUpgradeConflict, diagnosticActiveTrial,
			)
			gotSelection, err := readSelection(t.Context(), root)
			if err != nil || gotSelection != prior {
				t.Fatalf("selection after stale authority = (%v, %v), want (%v, nil)",
					gotSelection, err, prior)
			}
			gotTrial, err := readTrial(t.Context(), root, SlotB)
			if err != nil || gotTrial != activeDocument {
				t.Fatalf("trial after stale authority = (%v, %v), want (%v, nil)",
					gotTrial, err, activeDocument)
			}
			gotBytes, err := root.ReadFile(staleTarget.Path().String())
			if err != nil || !bytes.Equal(gotBytes, sharedBytes) {
				t.Fatalf("candidate after stale authority = (%q, %v), want %q unchanged",
					gotBytes, err, sharedBytes)
			}
		})
	}
}

func TestCleanupFailureReturnsTheAlreadySelectedPrimaryTruth(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatalf("os.OpenRoot() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = root.Close() })

	oldArtifact := artifactForTest(t, []byte("old"), 1)
	newArtifact := artifactForTest(t, []byte("new"), 2)
	installArtifactForTest(t, root, SlotA, oldArtifact, []byte("old"))
	installArtifactForTest(t, root, SlotB, newArtifact, []byte("new"))
	if err := root.WriteFile(SlotA.String()+string(os.PathSeparator)+"foreign", []byte("x"), documentMode); err != nil {
		t.Fatalf("os.Root.WriteFile(foreign) error = %v, want nil", err)
	}
	prior := selectionDocument{
		Revision: selectionRevisionCurrent, Slot: SlotA, Artifact: oldArtifact,
	}
	if err := writeSelection(
		t.Context(), root, prior, filestore.InstallCreate,
	); err != nil {
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

	primary, promoteErr := Promote(t.Context(), PromoteRequest{
		Root: root, Directory: absolutePathForTest(t, directory), Promotion: promotion,
	})
	if primary.Artifact() != newArtifact ||
		!errors.Is(promoteErr, core.ErrUpgradeCleanup) {
		t.Fatalf("Promote(cleanup failure) = (%v, %v), want new primary/%v",
			primary, promoteErr, core.ErrUpgradeCleanup)
	}
	resolved, err := ResolvePrimary(t.Context(), ResolveRequest{
		Root: root, Directory: absolutePathForTest(t, directory),
	})
	if err != nil || resolved.Artifact() != newArtifact {
		t.Fatalf("ResolvePrimary(after cleanup failure) = (%v, %v), want new primary",
			resolved, err)
	}
	trialReceipt, err := trialPath(SlotB)
	if err != nil {
		t.Fatalf("trialPath(promoted) error = %v, want nil", err)
	}
	if _, err := root.Stat(trialReceipt.String()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("promoted trial receipt stat error = %v, want %v",
			err, os.ErrNotExist)
	}
}

func TestVerificationRejectsEveryExtentAndDigestContradiction(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		data    []byte
		install bool
		wantOK  bool
	}{
		{name: "the exact authentic bytes are admitted", data: []byte("good"), install: true, wantOK: true},
		{name: "absent file", install: false},
		{name: "empty file", data: nil, install: true},
		{name: "one byte", data: []byte("g"), install: true},
		{name: "one byte below the extent", data: []byte("goo"), install: true},
		{name: "one byte above the extent", data: []byte("goodx"), install: true},
		{name: "exact extent with every byte wrong", data: []byte("baad"), install: true},
		{name: "exact extent with the first byte flipped", data: []byte("Good"), install: true},
		{name: "exact extent with the last byte flipped", data: []byte("gooD"), install: true},
		{name: "exact extent reordered", data: []byte("doog"), install: true},
		{name: "authentic bytes with one appended zero", data: []byte("good\x00"), install: true},
		{name: "authentic bytes doubled", data: []byte("goodgood"), install: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatalf("os.OpenRoot() error = %v, want nil", err)
			}
			t.Cleanup(func() { _ = root.Close() })
			artifact := artifactForTest(t, []byte("good"), 1)
			if tc.install {
				installArtifactForTest(t, root, SlotA, artifact, tc.data)
			}
			err = verifyArtifact(t.Context(), root, SlotA, artifact)
			if tc.wantOK {
				if err != nil {
					t.Fatalf("verifyArtifact(%q) error = %v, want nil", tc.data, err)
				}
				return
			}
			requireRejection(
				t, err, core.ErrUpgradeVerification,
				diagnosticCandidateBytes,
			)
		})
	}
}

func TestReclaimAdoptsExactBytesAndRemovesOnlyItsInterruptedBytes(t *testing.T) {
	t.Parallel()

	candidateBytes := []byte("candidate under trial")
	for _, tc := range []struct {
		name        string
		occupant    []byte
		occupied    bool
		wantAdopted bool
	}{
		{name: "an empty slot has nothing to adopt or remove"},
		{
			name:     "the exact authenticated candidate is adopted rather than redownloaded",
			occupant: candidateBytes, occupied: true, wantAdopted: true,
		},
		{name: "a truncated interrupted download is removed", occupant: candidateBytes[:5], occupied: true},
		{name: "an empty interrupted download is removed", occupant: nil, occupied: true},
		{name: "foreign bytes of the same extent are removed", occupant: bytes.Repeat([]byte("x"), len(candidateBytes)), occupied: true},
		{name: "an overlong occupant is removed", occupant: append(slices.Clone(candidateBytes), 'x'), occupied: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatalf("os.OpenRoot() error = %v, want nil", err)
			}
			t.Cleanup(func() { _ = root.Close() })

			oldBytes := []byte("old")
			oldArtifact := artifactForTest(t, oldBytes, 1)
			candidate := artifactForTest(t, candidateBytes, 2)
			installArtifactForTest(t, root, SlotA, oldArtifact, oldBytes)
			target, err := newTrialTarget(
				absolutePathForTest(t, directory),
				selectionDocument{
					Revision: selectionRevisionCurrent, Slot: SlotA, Artifact: oldArtifact,
				},
				candidate,
			)
			if err != nil {
				t.Fatalf("newTrialTarget() error = %v, want nil", err)
			}
			if err := prepareCandidateSlot(t.Context(), root, target); err != nil {
				t.Fatalf("prepareCandidateSlot() error = %v, want nil", err)
			}
			receipt, err := ensureTrialReceipt(t.Context(), root, target)
			if err != nil {
				t.Fatalf("ensureTrialReceipt() error = %v, want nil", err)
			}
			if tc.occupied {
				if err := root.WriteFile(
					target.Path().String(), tc.occupant, executableMode,
				); err != nil {
					t.Fatalf("os.Root.WriteFile(occupant) error = %v, want nil", err)
				}
			}

			adopted, err := reclaimCandidateSlot(
				t.Context(), root, target, receipt,
			)
			if err != nil || adopted != tc.wantAdopted {
				t.Fatalf("reclaimCandidateSlot() = (%t, %v), want (%t, nil)",
					adopted, err, tc.wantAdopted)
			}
			_, statErr := root.Stat(target.Path().String())
			if tc.wantAdopted && statErr != nil {
				t.Fatalf("adopted candidate stat error = %v, want the file kept", statErr)
			}
			if !tc.wantAdopted && !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("reclaimed slot stat error = %v, want %v", statErr, os.ErrNotExist)
			}
			// Reclamation must never reach across to the primary slot.
			primaryPath, err := binaryPath(SlotA, oldArtifact.Build())
			if err != nil {
				t.Fatalf("binaryPath(primary) error = %v, want nil", err)
			}
			got, err := root.ReadFile(primaryPath.String())
			if err != nil || !bytes.Equal(got, oldBytes) {
				t.Fatalf("primary after reclamation = (%q, %v), want %q unchanged",
					got, err, oldBytes)
			}
		})
	}
}

func TestReclaimNeverDeletesAStillAuthorizedDifferentTrial(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatalf("os.OpenRoot() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = root.Close() })

	oldArtifact := artifactForTest(t, []byte("old"), 1)
	activeBytes := []byte("candidate still under trial")
	activeArtifact := artifactForTest(t, activeBytes, 2)
	laterArtifact := artifactForTest(t, []byte("later candidate"), 3)
	prior := selectionDocument{
		Revision: selectionRevisionCurrent, Slot: SlotA, Artifact: oldArtifact,
	}
	active, err := newTrialTarget(
		absolutePathForTest(t, directory), prior, activeArtifact,
	)
	if err != nil {
		t.Fatalf("newTrialTarget(active) error = %v, want nil", err)
	}
	later, err := newTrialTarget(
		absolutePathForTest(t, directory), prior, laterArtifact,
	)
	if err != nil {
		t.Fatalf("newTrialTarget(later) error = %v, want nil", err)
	}
	installArtifactForTest(t, root, active.slot, activeArtifact, activeBytes)
	receipt, err := ensureTrialReceipt(t.Context(), root, active)
	if err != nil {
		t.Fatalf("ensureTrialReceipt(active) error = %v, want nil", err)
	}

	adopted, reclaimErr := reclaimCandidateSlot(
		t.Context(), root, later, receipt,
	)
	if adopted || !errors.Is(reclaimErr, core.ErrUpgradeConflict) {
		t.Fatalf("reclaimCandidateSlot(different live trial) = (%t, %v), want false/%v",
			adopted, reclaimErr, core.ErrUpgradeConflict)
	}
	got, err := root.ReadFile(active.Path().String())
	if err != nil || !bytes.Equal(got, activeBytes) {
		t.Fatalf("active trial after different reclaim = (%q, %v), want %q unchanged",
			got, err, activeBytes)
	}
}

// TestPreparingAnOccupiedTrialSlotNeverDeletesItsCandidate pins the two halves
// of the occupied-slot contract that reclamation depends on: preparing the slot
// is purely additive, and the download open is exclusive-create so it can never
// append to or truncate an existing occupant. Clearing an occupied slot is
// reclaimCandidateSlot's job, proved separately.
func TestPreparingAnOccupiedTrialSlotNeverDeletesItsCandidate(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatalf("os.OpenRoot() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = root.Close() })
	oldArtifact := artifactForTest(t, []byte("old"), 1)
	newBytes := []byte("candidate under trial")
	newArtifact := artifactForTest(t, newBytes, 2)
	installArtifactForTest(t, root, SlotB, newArtifact, newBytes)
	target, err := newTrialTarget(
		absolutePathForTest(t, directory),
		selectionDocument{
			Revision: selectionRevisionCurrent, Slot: SlotA, Artifact: oldArtifact,
		},
		newArtifact,
	)
	if err != nil {
		t.Fatalf("newTrialTarget() error = %v, want nil", err)
	}
	if err := prepareCandidateSlot(t.Context(), root, target); err != nil {
		t.Fatalf("prepareCandidateSlot(occupied) error = %v, want nil", err)
	}
	download, downloadErr := downloadCandidate(
		t.Context(), StageRequest{Root: root}, target,
	)
	if !errors.Is(downloadErr, core.ErrFilestoreConflict) || download.owned {
		t.Fatalf("downloadCandidate(occupied) = (%v, %v), want unowned errors.Is %v",
			download, downloadErr, core.ErrFilestoreConflict)
	}
	if err := cleanupOwnedCandidate(
		t.Context(), root, target, download,
	); err != nil {
		t.Fatalf("cleanupOwnedCandidate(unowned) error = %v, want nil", err)
	}
	got, err := root.ReadFile(target.Path().String())
	if err != nil || string(got) != string(newBytes) {
		t.Fatalf("occupied candidate after preparation = (%q, %v), want unchanged",
			got, err)
	}
}

func TestBootstrapCollisionNeverDeletesAnExistingPrimaryArtifact(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatalf("os.OpenRoot() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = root.Close() })
	data := []byte("working primary")
	artifact := artifactForTest(t, data, 1)
	installArtifactForTest(t, root, SlotA, artifact, data)

	write, writeErr := writeBootstrapArtifact(
		t.Context(), root, artifact, bytes.NewReader(data),
	)
	if !errors.Is(writeErr, core.ErrFilestoreConflict) || write.owned {
		t.Fatalf("writeBootstrapArtifact(collision) = (%v, %v), want unowned errors.Is %v",
			write, writeErr, core.ErrFilestoreConflict)
	}
	if err := cleanupBootstrapArtifact(
		t.Context(), root, artifact, write,
	); err != nil {
		t.Fatalf("cleanupBootstrapArtifact(unowned) error = %v, want nil", err)
	}
	path, err := binaryPath(SlotA, artifact.Build())
	if err != nil {
		t.Fatalf("binaryPath() error = %v, want nil", err)
	}
	got, err := root.ReadFile(path.String())
	if err != nil || string(got) != string(data) {
		t.Fatalf("existing primary after collision = (%q, %v), want unchanged",
			got, err)
	}
}

func TestOwnedFailedDownloadAndSuccessfulBootstrapWriteCleanExactlyTheirBytes(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatalf("os.OpenRoot() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = root.Close() })
	oldArtifact := artifactForTest(t, []byte("old"), 1)
	candidate := artifactForTest(t, []byte("candidate"), 2)
	target, err := newTrialTarget(
		absolutePathForTest(t, directory),
		selectionDocument{
			Revision: selectionRevisionCurrent, Slot: SlotA, Artifact: oldArtifact,
		},
		candidate,
	)
	if err != nil {
		t.Fatalf("newTrialTarget() error = %v, want nil", err)
	}
	if err := prepareCandidateSlot(t.Context(), root, target); err != nil {
		t.Fatalf("prepareCandidateSlot() error = %v, want nil", err)
	}
	download, downloadErr := downloadCandidate(
		t.Context(), StageRequest{Root: root}, target,
	)
	if !errors.Is(downloadErr, core.ErrObjectStoreContract) || !download.owned {
		t.Fatalf("downloadCandidate(invalid source) = (%v, %v), want owned errors.Is %v",
			download, downloadErr, core.ErrObjectStoreContract)
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if err := cleanupOwnedCandidate(
		cancelled, root, target, download,
	); err != nil {
		t.Fatalf("cleanupOwnedCandidate(cancelled) error = %v, want nil", err)
	}
	if _, err := root.Stat(target.Path().String()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("owned failed candidate stat error = %v, want %v", err, os.ErrNotExist)
	}

	data := []byte("bootstrap")
	artifact := artifactForTest(t, data, 1)
	write, err := writeBootstrapArtifact(
		t.Context(), root, artifact, bytes.NewReader(data),
	)
	if err != nil || !write.owned {
		t.Fatalf("writeBootstrapArtifact() = (%v, %v), want owned success", write, err)
	}
	if err := cleanupBootstrapArtifact(
		cancelled, root, artifact, write,
	); err != nil {
		t.Fatalf("cleanupBootstrapArtifact(cancelled) error = %v, want nil", err)
	}
	path, err := binaryPath(SlotA, artifact.Build())
	if err != nil {
		t.Fatalf("binaryPath() error = %v, want nil", err)
	}
	if _, err := root.Stat(path.String()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cleaned bootstrap artifact stat error = %v, want %v", err, os.ErrNotExist)
	}
}

func TestDiscardTrialRemovesOnlyTheCandidateAndPreservesPrimary(t *testing.T) {
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
	if err := writeSelection(
		t.Context(), root, prior, filestore.InstallCreate,
	); err != nil {
		t.Fatalf("writeSelection(prior) error = %v, want nil", err)
	}
	target, err := newTrialTarget(
		absolutePathForTest(t, directory), prior, newArtifact,
	)
	if err != nil {
		t.Fatalf("newTrialTarget() error = %v, want nil", err)
	}
	if _, err := ensureTrialReceipt(t.Context(), root, target); err != nil {
		t.Fatalf("ensureTrialReceipt() error = %v, want nil", err)
	}

	if err := DiscardTrial(t.Context(), DiscardTrialRequest{
		Root: root, Directory: absolutePathForTest(t, directory), Target: target,
	}); err != nil {
		t.Fatalf("DiscardTrial() error = %v, want nil", err)
	}
	resolved, err := ResolvePrimary(t.Context(), ResolveRequest{
		Root: root, Directory: absolutePathForTest(t, directory),
	})
	if err != nil || resolved.Artifact() != oldArtifact {
		t.Fatalf("ResolvePrimary(after discard) = (%v, %v), want old primary",
			resolved, err)
	}
	if _, err := root.Stat(target.Path().String()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("discarded candidate stat error = %v, want %v", err, os.ErrNotExist)
	}
}

func TestDiscardTrialRefusesTamperedCandidateWithoutDeletingIt(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatalf("os.OpenRoot() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = root.Close() })
	oldArtifact := artifactForTest(t, []byte("old"), 1)
	newArtifact := artifactForTest(t, []byte("new"), 2)
	installArtifactForTest(t, root, SlotA, oldArtifact, []byte("old"))
	installArtifactForTest(t, root, SlotB, newArtifact, []byte("tampered"))
	prior := selectionDocument{
		Revision: selectionRevisionCurrent, Slot: SlotA, Artifact: oldArtifact,
	}
	if err := writeSelection(
		t.Context(), root, prior, filestore.InstallCreate,
	); err != nil {
		t.Fatalf("writeSelection(prior) error = %v, want nil", err)
	}
	target, err := newTrialTarget(
		absolutePathForTest(t, directory), prior, newArtifact,
	)
	if err != nil {
		t.Fatalf("newTrialTarget() error = %v, want nil", err)
	}
	if _, err := ensureTrialReceipt(t.Context(), root, target); err != nil {
		t.Fatalf("ensureTrialReceipt() error = %v, want nil", err)
	}

	discardErr := DiscardTrial(t.Context(), DiscardTrialRequest{
		Root: root, Directory: absolutePathForTest(t, directory), Target: target,
	})
	if !errors.Is(discardErr, core.ErrUpgradeVerification) {
		t.Fatalf("DiscardTrial(tampered) error = %v, want %v",
			discardErr, core.ErrUpgradeVerification)
	}
	got, err := root.ReadFile(target.Path().String())
	if err != nil || string(got) != "tampered" {
		t.Fatalf("tampered candidate after refusal = (%q, %v), want unchanged",
			got, err)
	}
}

func TestPublicIngressAndOpaqueZeroValuesReject(t *testing.T) {
	t.Parallel()

	// Every Upgrade identity parents to ErrUpgradeContract, so the umbrella
	// identity alone cannot separate an unset primary from an unset root. Each
	// row therefore names the diagnostic its own rejection must carry.
	for _, tc := range []struct {
		err            error
		name           string
		wantDiagnostic diagnostic
	}{
		{
			name: "zero primary is unset before any projection is checked",
			err:  (Primary{}).Validate(), wantDiagnostic: diagnosticPrimary,
		},
		{
			name: "zero trial target is unset",
			err:  (TrialTarget{}).Validate(), wantDiagnostic: diagnosticTrialTarget,
		},
		{
			name: "zero promotion carries no passing trial",
			err:  (Promotion{}).Validate(), wantDiagnostic: diagnosticPromotion,
		},
		{
			name: "zero trial report names its unset target first",
			err:  (TrialReport{}).Validate(), wantDiagnostic: diagnosticTrialTarget,
		},
		{
			name: "zero stage request names the missing root",
			err:  (StageRequest{}).Validate(), wantDiagnostic: diagnosticRoot,
		},
		{
			name: "zero bootstrap request names the missing root",
			err:  (BootstrapRequest{}).Validate(), wantDiagnostic: diagnosticRoot,
		},
		{
			name: "zero resolve request names the missing root",
			err:  (ResolveRequest{}).Validate(), wantDiagnostic: diagnosticRoot,
		},
		{
			name: "zero promote request names the missing root",
			err:  (PromoteRequest{}).Validate(), wantDiagnostic: diagnosticRoot,
		},
		{
			name: "zero discard request names the missing root",
			err:  (DiscardTrialRequest{}).Validate(), wantDiagnostic: diagnosticRoot,
		},
		{
			name: "zero attempt error names the unset failure phase",
			err:  (AttemptError{}).Validate(), wantDiagnostic: diagnosticFailurePhase,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			requireRejection(t, tc.err, core.ErrUpgradeContract, tc.wantDiagnostic)
		})
	}

	if got, err := Stage(t.Context(), StageRequest{}); !errors.Is(err, core.ErrUpgradeContract) || got != (TrialTarget{}) {
		t.Fatalf("Stage(zero) = (%v, %v), want zero and %v", got, err, core.ErrUpgradeContract)
	}
	if got, err := Bootstrap(t.Context(), BootstrapRequest{}); !errors.Is(err, core.ErrUpgradeContract) || got != (Primary{}) {
		t.Fatalf("Bootstrap(zero) = (%v, %v), want zero and %v", got, err, core.ErrUpgradeContract)
	}
}

func TestEveryDiagnosticCarriesDistinctNonEmptyText(t *testing.T) {
	t.Parallel()

	seen := make(map[string]diagnostic, diagnosticLimit)
	for value := diagnosticUnknown + 1; value < diagnosticLimit; value++ {
		text := value.Error()
		if text == "" || text == diagnosticTexts()[diagnosticUnknown] {
			t.Fatalf("diagnostic(%d) text = %q, want its own non-empty rejection",
				value, text)
		}
		if earlier, ok := seen[text]; ok {
			t.Fatalf("diagnostic(%d) text = %q, already used by diagnostic(%d)",
				value, text, earlier)
		}
		seen[text] = value
	}
	for _, outside := range []diagnostic{diagnosticUnknown, diagnosticLimit, 255} {
		if got := outside.Error(); got != diagnosticTexts()[diagnosticUnknown] {
			t.Fatalf("diagnostic(%d).Error() = %q, want %q",
				outside, got, diagnosticTexts()[diagnosticUnknown])
		}
	}
}

func TestAttemptErrorPhaseAndCoreIdentityMustAgree(t *testing.T) {
	t.Parallel()

	build := artifactForTest(t, []byte("candidate"), 2).Build()
	identities := []core.ErrorIdentity{
		core.ErrUpgradeDownload,
		core.ErrUpgradeCapacity,
		core.ErrUpgradeVerification,
		core.ErrUpgradeTrial,
		core.ErrUpgradePromotion,
		core.ErrUpgradePersistence,
		core.ErrUpgradeCleanup,
		core.ErrUpgradeConflict,
	}
	cases := []struct {
		accepted []core.ErrorIdentity
		phase    FailurePhase
	}{
		{phase: FailurePhaseBootstrap, accepted: []core.ErrorIdentity{core.ErrUpgradePersistence}},
		{phase: FailurePhaseCapacity, accepted: []core.ErrorIdentity{core.ErrUpgradeCapacity}},
		{phase: FailurePhaseDownload, accepted: []core.ErrorIdentity{core.ErrUpgradeDownload}},
		{phase: FailurePhaseVerification, accepted: []core.ErrorIdentity{core.ErrUpgradeVerification}},
		{phase: FailurePhaseTrial, accepted: []core.ErrorIdentity{core.ErrUpgradeTrial}},
		{
			phase: FailurePhasePromotion,
			accepted: []core.ErrorIdentity{
				core.ErrUpgradePromotion, core.ErrUpgradeConflict,
			},
		},
		{
			phase: FailurePhasePersistence,
			accepted: []core.ErrorIdentity{
				core.ErrUpgradePersistence, core.ErrUpgradeConflict,
			},
		},
		{
			phase: FailurePhaseCleanup,
			accepted: []core.ErrorIdentity{
				core.ErrUpgradeCleanup, core.ErrUpgradeConflict,
			},
		},
	}
	for _, tc := range cases {
		for _, identity := range identities {
			attempt := AttemptError{
				phase: tc.phase, candidate: build,
				cause: upgradeError(identity),
			}
			wantValid := slices.Contains(tc.accepted, identity)
			if (attempt.Validate() == nil) != wantValid {
				t.Fatalf("AttemptError phase=%v identity=%v validity = %t, want %t",
					tc.phase, identity, attempt.Validate() == nil, wantValid)
			}
		}
	}
}

func TestAttemptErrorNamesCandidateWithoutRenderingItsNativeCause(t *testing.T) {
	t.Parallel()

	candidate := artifactForTest(t, []byte("candidate"), 2).Build()
	native := errors.New("https://signed.example.invalid/private-token")
	got := newAttemptError(attemptErrorRequest{phase: FailurePhaseDownload, candidate: candidate, identity: core.ErrUpgradeDownload},
		native)

	var attempt AttemptError
	if !errors.As(got, &attempt) ||
		!errors.Is(got, core.ErrUpgradeDownload) ||
		!errors.Is(got, native) {
		t.Fatalf("attempt error = %v, want typed attempt, download identity, and native cause",
			got)
	}
	rendered := got.Error()
	for _, fact := range []string{
		FailurePhaseDownload.String(),
		candidate.Offering().String(),
		candidate.Version().String(),
		candidate.Platform().String(),
		candidate.Commit().String(),
	} {
		if !strings.Contains(rendered, fact) {
			t.Fatalf("attempt diagnostic = %q, want candidate fact %q",
				rendered, fact)
		}
	}
	if strings.Contains(rendered, native.Error()) {
		t.Fatalf("attempt diagnostic = %q, want native download authority redacted",
			rendered)
	}
}

func artifactForTest(t testing.TB, data []byte, version uint32) release.Artifact {
	t.Helper()
	platform := core.Platform{
		OperatingSystem: core.OperatingSystemLinux,
		Architecture:    core.CPUArchitectureAMD64,
	}
	commit, err := core.ParseBuildCommit("0123456789abcdef0123456789abcdef01234567")
	if err != nil {
		t.Fatalf("core.ParseBuildCommit() error = %v, want nil", err)
	}
	build, err := core.NewBuildIdentity(core.BuildIdentityRequest{
		Offering: upgradeOffering(t, 4),
		Version:  core.NewReleaseVersion(version, 0, 0),
		Commit:   commit,
		Platform: platform,
	})
	if err != nil {
		t.Fatalf("core.NewBuildIdentity() error = %v, want nil", err)
	}
	extent, err := core.NewByteCount(uint64(len(data)))
	if err != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
	returnArtifact, err := release.NewArtifact(release.ArtifactRequest{
		Extent: extent,
		Build:  build,
		SHA256: core.NewSHA256Digest(sha256.Sum256(data)),
		CRC32C: core.NewCRC32C(crc32.Checksum(data, crc32.MakeTable(crc32.Castagnoli))),
	})
	if err != nil {
		t.Fatalf("release.NewArtifact() error = %v, want nil", err)
	}
	return returnArtifact
}

func absolutePathForTest(t testing.TB, value string) core.AbsolutePath {
	t.Helper()
	path, err := core.ParseAbsolutePath(value)
	if err != nil {
		t.Fatalf("core.ParseAbsolutePath(%q) error = %v, want nil", value, err)
	}
	return path
}

func installArtifactForTest(
	t testing.TB,
	root *os.Root,
	slot Slot,
	artifact release.Artifact,
	data []byte,
) {
	t.Helper()
	if err := root.Mkdir(slot.String(), directoryMode); err != nil {
		t.Fatalf("os.Root.Mkdir(%v) error = %v, want nil", slot, err)
	}
	path, err := binaryPath(slot, artifact.Build())
	if err != nil {
		t.Fatalf("binaryPath() error = %v, want nil", err)
	}
	if err := root.WriteFile(path.String(), data, executableMode); err != nil {
		t.Fatalf("os.Root.WriteFile(%v) error = %v, want nil", path, err)
	}
}

func isUpgradeContract(err error) bool {
	return errors.Is(err, core.ErrUpgradeContract)
}

func buildArtifactForTest(
	t testing.TB,
	data []byte,
	offering core.Offering,
	version core.ReleaseVersion,
	platform core.Platform,
) release.Artifact {
	t.Helper()
	commit, err := core.ParseBuildCommit("0123456789abcdef0123456789abcdef01234567")
	if err != nil {
		t.Fatalf("core.ParseBuildCommit() error = %v, want nil", err)
	}
	build, err := core.NewBuildIdentity(core.BuildIdentityRequest{
		Offering: offering, Version: version, Commit: commit, Platform: platform,
	})
	if err != nil {
		t.Fatalf("core.NewBuildIdentity() error = %v, want nil", err)
	}
	extent, err := core.NewByteCount(uint64(len(data)))
	if err != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
	artifact, err := release.NewArtifact(release.ArtifactRequest{
		Extent: extent,
		Build:  build,
		SHA256: core.NewSHA256Digest(sha256.Sum256(data)),
		CRC32C: core.NewCRC32C(crc32.Checksum(data, crc32.MakeTable(crc32.Castagnoli))),
	})
	if err != nil {
		t.Fatalf("release.NewArtifact() error = %v, want nil", err)
	}
	return artifact
}
