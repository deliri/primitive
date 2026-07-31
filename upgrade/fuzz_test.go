package upgrade

import (
	"bytes"
	"errors"
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
				!errors.Is(decodeErr, core.ErrJSONContract) {
				t.Fatalf("decodeSelection failure = %v, want Upgrade and JSON identities",
					decodeErr)
			}
			return
		}
		if err := document.Validate(); err != nil {
			t.Fatalf("authenticated selection Validate error = %v, want nil", err)
		}
		canonical, err := encodeSelection(document)
		if err != nil {
			t.Fatalf("encodeSelection(authenticated) error = %v, want nil", err)
		}
		if !bytes.Equal(canonical, data) {
			t.Fatalf("authenticated selection differs from exact canonical input")
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
				!errors.Is(decodeErr, core.ErrJSONContract) {
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
