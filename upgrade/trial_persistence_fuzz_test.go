package upgrade

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// Both terminal operations must re-read the durable receipt. Fuzzed bytes
// cannot substitute a different candidate or prior selector for the typed target.
func FuzzTrialReceiptTerminalOperations(f *testing.F) {
	fixture := publicFixture(f)
	prior := artifactForTest(f, []byte("installed"), 1)
	candidate := artifactForTest(f, []byte("candidate"), 2)
	receipt := trialDocument{Revision: trialRevisionCurrent, Prior: selectionDocument{Revision: selectionRevisionCurrent, Slot: SlotA, Artifact: prior}, Candidate: candidate}
	canonical, err := encodeTrial(receipt)
	if err != nil {
		f.Fatalf("encodeTrial seed error = %v, want nil", err)
	}
	f.Add(canonical, false)
	f.Add(canonical, true)
	f.Add([]byte{}, false)
	f.Add([]byte(`{}`), true)
	f.Fuzz(func(t *testing.T, data []byte, discard bool) {
		root, dir := stageRootForTest(t, t.TempDir())
		installed, err := Bootstrap(t.Context(), BootstrapRequest{Root: root, Directory: dir, Build: fixture.build, Manifest: fixture.installed, Source: bytes.NewReader([]byte("installed"))})
		if err != nil {
			t.Fatalf("Bootstrap error = %v, want nil", err)
		}
		source := stageDownloadSourceForTest(t, stageDownloadSourceFixture{objectName: "bucket/candidate", transport: &stageDownloadTransport{payload: []byte("candidate")}})
		target, err := Stage(t.Context(), StageRequest{Root: root, Directory: dir, Prepared: fixture.prepared, Source: source})
		if err != nil {
			t.Fatalf("Stage error = %v, want nil", err)
		}
		path, err := trialPath(target.slot)
		if err != nil {
			t.Fatalf("trialPath error = %v, want nil", err)
		}
		persisted, err := root.ReadFile(path.String())
		if err != nil || !bytes.Equal(persisted, canonical) {
			t.Fatalf("Stage receipt = (%q,%v), want canonical %q", persisted, err, canonical)
		}
		if err := root.WriteFile(path.String(), data, documentMode); err != nil {
			t.Fatalf("write mutated receipt error = %v, want nil", err)
		}
		var primary Primary
		var terminalErr error
		if discard {
			terminalErr = DiscardTrial(t.Context(), DiscardTrialRequest{Root: root, Directory: dir, Target: target})
		} else {
			promotion, err := CompleteTrial(TrialReport{Target: target, Observed: candidate.Build(), Outcome: TrialPassed, Observation: temporal.InstantFromNanoseconds(4000)})
			if err != nil {
				t.Fatalf("CompleteTrial error = %v, want nil", err)
			}
			primary, terminalErr = Promote(t.Context(), PromoteRequest{Root: root, Directory: dir, Promotion: promotion})
		}
		unchanged := !bytes.Equal(data, canonical)
		if unchanged {
			if !errors.Is(terminalErr, core.ErrUpgradeConflict) || primary != (Primary{}) {
				t.Fatalf("terminal foreign receipt = (%v,%v), want zero and conflict", primary, terminalErr)
			}
			after, err := root.ReadFile(path.String())
			if err != nil || !bytes.Equal(after, data) {
				t.Fatalf("refused receipt = (%q,%v), want preserved %q", after, err, data)
			}
		} else if terminalErr != nil {
			t.Fatalf("terminal canonical receipt error = %v, want nil", terminalErr)
		}
		resolved, err := ResolvePrimary(t.Context(), ResolveRequest{Root: root, Directory: dir})
		want := installed.Artifact()
		if !discard && !unchanged {
			want = candidate
		}
		if err != nil || resolved.Artifact() != want {
			t.Fatalf("terminal selector = (%v,%v), want %v", resolved, err, want)
		}
	})
}
