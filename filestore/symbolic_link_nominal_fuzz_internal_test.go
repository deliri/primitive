package filestore

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// Native filesystems may impose a smaller limit than the nominal 64 KiB
// observation ceiling. This direct nominal ratchet exercises that full ceiling;
// the external fuzz target separately drives native ReadSymbolicLink.
func FuzzSymbolicLinkTargetOpaqueNominalClosure(f *testing.F) {
	directory := f.TempDir()
	if err := os.Symlink("opaque:[1]", filepath.Join(directory, "link")); err != nil {
		f.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		f.Fatal(err)
	}
	path, err := core.ParseRelativePath("link")
	if err != nil {
		f.Fatal(err)
	}
	location := Location{Root: root, Path: path}
	if err := location.Validate(); err != nil {
		f.Fatal(err)
	}
	seed, err := ReadSymbolicLink(f.Context(), location)
	if err != nil {
		f.Fatal(err)
	}
	if err := root.Close(); err != nil {
		f.Fatal(err)
	}
	f.Add(seed.String())
	for _, value := range []string{"", "x\x00y", "\xff", strings.Repeat("x", SymbolicLinkTargetMaximumBytes-1), strings.Repeat("x", SymbolicLinkTargetMaximumBytes), strings.Repeat("x", SymbolicLinkTargetMaximumBytes+1)} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		raw = raw[:min(len(raw), SymbolicLinkTargetMaximumBytes+1)]
		got := SymbolicLinkTarget{value: raw}
		// Independent byte scan; no path or Unicode grammar applies to this value.
		admissible := len(raw) > 0 && len(raw) <= SymbolicLinkTargetMaximumBytes
		for i := range len(raw) {
			if raw[i] == 0 {
				admissible = false
				break
			}
		}
		err := got.Validate()
		if admissible {
			if err != nil || got.String() != raw {
				t.Fatalf("opaque nominal = (%q,%v), want exact %q", got.String(), err, raw)
			}
		} else if !errors.Is(err, core.ErrFilestoreContract) || errors.Is(err, core.ErrFilestoreSource) || got.String() != "" {
			t.Fatalf("refused nominal = (%q,%v), want empty projection and exact contract refusal", got.String(), err)
		}
		if got.value != raw {
			t.Fatalf("retained nominal = %q, want original %q", got.value, raw)
		}
	})
}
