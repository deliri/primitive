package filestore_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// OS materialization and nominal admission are separate boundaries: Darwin can
// store an empty target, while other kernels refuse it at symlink creation.
func TestReadSymbolicLinkNativeTargetAdmissionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, target string
		wantAdmitted bool
	}{
		{name: "empty native target cannot become an admitted observation"},
		{name: "embedded NUL cannot silently truncate the stored target", target: "x\x00y"},
		{name: "one opaque byte remains exact despite absent referent", target: "?", wantAdmitted: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			name := filepath.Join(directory, "link")
			createErr := os.Symlink(tc.target, name)
			if createErr != nil {
				var cause *os.LinkError
				if !errors.As(createErr, &cause) || tc.wantAdmitted {
					t.Fatalf("native fixture creation = %v, want admissible target or native LinkError", createErr)
				}
			}
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := root.Close(); err != nil {
					t.Error(err)
				}
			})
			relative, err := core.ParseRelativePath("link")
			if err != nil {
				t.Fatal(err)
			}
			native, nativeErr := root.Readlink(relative.String())
			got, gotErr := filestore.ReadSymbolicLink(t.Context(), filestore.Location{Root: root, Path: relative})
			switch {
			case createErr != nil:
				if !errors.Is(nativeErr, os.ErrNotExist) || !errors.Is(gotErr, os.ErrNotExist) || !errors.Is(gotErr, core.ErrFilestoreSource) {
					t.Fatalf("absent rejected fixture = (%q,%v), want native absence %v", got.String(), gotErr, nativeErr)
				}
			case tc.wantAdmitted:
				if nativeErr != nil || native != tc.target || gotErr != nil || got.Validate() != nil || got.String() != tc.target {
					t.Fatalf("native target admission = (%q,%v), native (%q,%v), want %q", got.String(), gotErr, native, nativeErr, tc.target)
				}
			default:
				if nativeErr != nil || native != tc.target || !errors.Is(gotErr, core.ErrFilestoreContract) || errors.Is(gotErr, core.ErrFilestoreSource) {
					t.Fatalf("native target admission = (%q,%v), native (%q,%v), want exact typed refusal for %q", got.String(), gotErr, native, nativeErr, tc.target)
				}
			}
			if gotErr != nil && got != (filestore.SymbolicLinkTarget{}) {
				t.Fatalf("refused target = %+v, want zero", got)
			}
			entries, err := os.ReadDir(directory)
			if err != nil {
				t.Fatal(err)
			}
			if createErr != nil {
				if len(entries) != 0 {
					t.Fatalf("refused namespace = %v, want empty", entries)
				}
			} else {
				if len(entries) != 1 || entries[0].Name() != "link" {
					t.Fatalf("retained namespace = %v, want only link", entries)
				}
				after, err := os.Readlink(name)
				if err != nil || after != tc.target {
					t.Fatalf("retained target = (%q,%v), want %q", after, err, tc.target)
				}
			}
		})
	}
}
