package upgrade

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzSelectionDocumentJSON(f *testing.F) {
	artifact := artifactForTest(f, []byte("fuzz candidate"), 1)
	fixture, err := encodeSelection(selectionDocument{
		Revision: selectionRevisionCurrent, Slot: SlotA, Artifact: artifact,
	})
	if err != nil {
		f.Fatalf("encodeSelection(seed) error = %v, want nil", err)
	}
	f.Add(fixture)
	f.Add([]byte(`{}`))
	f.Add(append(append([]byte{}, fixture...), ' '))

	f.Fuzz(func(t *testing.T, data []byte) {
		document, decodeErr := decodeSelection(data)
		if decodeErr != nil {
			if !errors.Is(decodeErr, core.ErrUpgradeContract) ||
				!errors.Is(decodeErr, core.ErrJSONContract) || document != (selectionDocument{}) {
				t.Fatalf("decodeSelection failure = %v, want Upgrade and JSON identities",
					decodeErr)
			}
			return
		}
		if err := document.Validate(); err != nil {
			t.Fatalf("canonical selection Validate error = %v, want nil", err)
		}
		canonical, err := encodeSelection(document)
		if err != nil {
			t.Fatalf("encodeSelection(canonical) error = %v, want nil", err)
		}
		if !bytes.Equal(canonical, data) {
			t.Fatalf("canonical selection differs from exact canonical input")
		}
		if len(canonical) > selectionDocumentMaximumBytes {
			t.Fatalf("canonical selection extent = %d, want <= %d",
				len(canonical), selectionDocumentMaximumBytes)
		}
	})
}

func FuzzTrialDocumentJSON(f *testing.F) {
	prior := artifactForTest(f, []byte("installed"), 1)
	candidate := artifactForTest(f, []byte("candidate"), 2)
	fixture, err := encodeTrial(trialDocument{
		Revision: trialRevisionCurrent,
		Prior: selectionDocument{
			Revision: selectionRevisionCurrent, Slot: SlotA, Artifact: prior,
		},
		Candidate: candidate,
	})
	if err != nil {
		f.Fatalf("encodeTrial(seed) error = %v, want nil", err)
	}
	f.Add(fixture)
	f.Add([]byte(`{}`))
	f.Add(append(append([]byte{}, fixture...), ' '))

	f.Fuzz(func(t *testing.T, data []byte) {
		document, decodeErr := decodeTrial(data)
		if decodeErr != nil {
			if !errors.Is(decodeErr, core.ErrUpgradeContract) ||
				!errors.Is(decodeErr, core.ErrJSONContract) || document != (trialDocument{}) {
				t.Fatalf("decodeTrial failure = %v, want Upgrade and JSON identities",
					decodeErr)
			}
			return
		}
		if err := document.Validate(); err != nil {
			t.Fatalf("accepted trial receipt Validate error = %v, want nil", err)
		}
		canonical, err := encodeTrial(document)
		if err != nil {
			t.Fatalf("encodeTrial(accepted) error = %v, want nil", err)
		}
		if !bytes.Equal(canonical, data) {
			t.Fatalf("accepted trial receipt extent = %d, want exact canonical input extent %d",
				len(canonical), len(data))
		}
		if len(canonical) > trialDocumentMaximumBytes {
			t.Fatalf("canonical trial extent = %d, want <= %d",
				len(canonical), trialDocumentMaximumBytes)
		}
	})
}

// The public enum decoder must preserve the receiver on refusal and agree with
// the standard JSON string decoder for every accepted representation.
func FuzzSlotJSONSemanticClosure(f *testing.F) {
	for _, slot := range []Slot{SlotA, SlotB} {
		seed, err := slot.MarshalJSON()
		if err != nil {
			f.Fatalf("Slot.MarshalJSON error = %v, want nil", err)
		}
		f.Add(seed)
	}
	f.Add([]byte{})
	f.Add([]byte(`null`))
	f.Add([]byte(`"slot-c"`))
	f.Fuzz(func(t *testing.T, data []byte) {
		got := SlotB
		err := got.UnmarshalJSON(data)
		if err != nil {
			if got != SlotB || !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrUpgradeContract) {
				t.Fatalf("Slot.UnmarshalJSON refusal = (%v,%v), want unchanged and typed refusal", got, err)
			}
			return
		}
		var token string
		if err := json.Unmarshal(data, &token); err != nil || token != got.String() || got.Validate() != nil {
			t.Fatalf("Slot.UnmarshalJSON accepted = (%v,%q,%v), want exact valid token", got, token, err)
		}
		canonical, err := got.MarshalJSON()
		if err != nil {
			t.Fatalf("Slot.MarshalJSON error = %v, want nil", err)
		}
		var roundtrip Slot
		if err := roundtrip.UnmarshalJSON(canonical); err != nil || roundtrip != got {
			t.Fatalf("Slot canonical roundtrip = (%v,%v), want %v", roundtrip, err, got)
		}
	})
}

// Reads go through the real filesystem and public resolver; a canonical
// selector is insufficient authority for bytes that do not exist or differ.
func FuzzResolvePrimaryPersistedSelection(f *testing.F) {
	payload := []byte("installed")
	artifact := artifactForTest(f, payload, 1)
	document := selectionDocument{Revision: selectionRevisionCurrent, Slot: SlotA, Artifact: artifact}
	seed, err := encodeSelection(document)
	if err != nil {
		f.Fatalf("encodeSelection seed error = %v, want nil", err)
	}
	f.Add(seed)
	f.Add([]byte{})
	f.Add([]byte(`{}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		root, dir := stageRootForTest(t, t.TempDir())
		installArtifactForTest(t, root, SlotA, artifact, payload)
		if err := root.WriteFile(selectionFilename, data, documentMode); err != nil {
			t.Fatalf("write selector error = %v, want nil", err)
		}
		got, err := ResolvePrimary(t.Context(), ResolveRequest{Root: root, Directory: dir})
		if err != nil {
			if got != (Primary{}) || (!errors.Is(err, core.ErrUpgradePersistence) && !errors.Is(err, core.ErrUpgradeVerification)) {
				t.Fatalf("ResolvePrimary refused = (%v,%v), want zero and typed refusal", got, err)
			}
		} else {
			if got.Artifact() != artifact || got.slot != SlotA {
				t.Fatalf("ResolvePrimary accepted = %v, want exact installed artifact and slot", got)
			}
			if !bytes.Equal(data, seed) {
				t.Fatalf("ResolvePrimary accepted selector = %q, want exact typed seed %q", data, seed)
			}
		}
		persisted, readErr := root.ReadFile(selectionFilename)
		if readErr != nil || !bytes.Equal(data, persisted) {
			t.Fatalf("selector after resolve = (%d,%v), want unchanged %d bytes", len(persisted), readErr, len(data))
		}
		for _, scratch := range []string{selectionTemporaryFilename, SlotB.String()} {
			if _, err := root.Stat(scratch); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("resolve scratch %s error = %v, want absent", scratch, err)
			}
		}
	})
}
