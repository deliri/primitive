package filestore

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// The finite fuzz budget crosses the former nominal quota. Native filesystem
// constraints are exercised separately through ReadSymbolicLink.
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
	for _, value := range []string{"", "x\x00y", "\xff", strings.Repeat("x", symbolicLinkTargetFixtureBytes-1), strings.Repeat("x", symbolicLinkTargetFixtureBytes), strings.Repeat("x", symbolicLinkTargetFixtureBytes+1)} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		raw = raw[:min(len(raw), symbolicLinkTargetFixtureBytes+1)]
		got := SymbolicLinkTarget{value: raw}
		// Independent byte scan; no path or Unicode grammar applies to this value.
		admissible := len(raw) > 0
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
