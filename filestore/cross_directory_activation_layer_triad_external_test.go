package filestore_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// TestCrossDirectoryActivationLayerTriad proves target-late publication within
// one real rooted capability. The staged receipt and target may occupy
// different existing directories; the OS still owns link/rename arbitration.
func TestCrossDirectoryActivationLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive create and replace publish the staged inode across directories", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			initial string
			name    string
			install filestore.InstallMode
		}{
			{name: "create links an absent digest target", install: filestore.InstallCreate},
			{name: "replace renames over an occupied digest target", install: filestore.InstallReplace, initial: "old"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				rootDirectory := t.TempDir()
				root := requireTestRoot(t, rootDirectory)
				if err := os.Mkdir(filepath.Join(rootDirectory, "staging"), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(filepath.Join(rootDirectory, "objects"), 0o700); err != nil {
					t.Fatal(err)
				}
				targetPath := filepath.Join(rootDirectory, "objects", "digest")
				if tc.initial != "" {
					if err := os.WriteFile(targetPath, []byte(tc.initial), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				staged := mustStage(t, root, filepath.Join("staging", ".stage"), "candidate")
				stagedInfo, err := os.Stat(filepath.Join(rootDirectory, "staging", ".stage"))
				if err != nil {
					t.Fatal(err)
				}
				gotErr := filestore.Commit(t.Context(), filestore.CommitRequest{
					Staged:  staged,
					Target:  mustRelativePath(t, filepath.Join("objects", "digest")),
					Install: tc.install,
				})
				if gotErr != nil {
					t.Fatalf("Commit(cross-directory) error = %v, want nil", gotErr)
				}
				targetInfo, err := os.Stat(targetPath)
				if err != nil {
					t.Fatal(err)
				}
				if !os.SameFile(stagedInfo, targetInfo) {
					t.Fatalf("target identity = %v, want staged identity %v", targetInfo.Name(), stagedInfo.Name())
				}
				requireOptionalFile(t, targetPath, "candidate")
				requireDirectoryEntryNames(t, filepath.Join(rootDirectory, "staging"), nil)
				requireDirectoryEntryNames(t, filepath.Join(rootDirectory, "objects"), []string{"digest"})
			})
		}
	})

	t.Run("negative absent target parent preserves the staged owner for both install modes", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			install filestore.InstallMode
		}{
			{name: "create cannot invent the target directory", install: filestore.InstallCreate},
			{name: "replace cannot invent the target directory", install: filestore.InstallReplace},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				rootDirectory := t.TempDir()
				root := requireTestRoot(t, rootDirectory)
				if err := os.Mkdir(filepath.Join(rootDirectory, "staging"), 0o700); err != nil {
					t.Fatal(err)
				}
				staged := mustStage(t, root, filepath.Join("staging", ".stage"), "candidate")
				gotErr := filestore.Commit(t.Context(), filestore.CommitRequest{
					Staged:  staged,
					Target:  mustRelativePath(t, filepath.Join("missing", "digest")),
					Install: tc.install,
				})
				if !errors.Is(gotErr, core.ErrFilestoreActivation) ||
					!errors.Is(gotErr, fs.ErrNotExist) {
					t.Fatalf("Commit(absent target parent) error = %v, want %v and %v", gotErr, core.ErrFilestoreActivation, fs.ErrNotExist)
				}
				requireOptionalFile(t, filepath.Join(rootDirectory, "staging", ".stage"), "candidate")
				if _, err := os.Stat(filepath.Join(rootDirectory, "missing")); !errors.Is(err, fs.ErrNotExist) {
					t.Fatalf("Stat(missing target parent) error = %v, want %v", err, fs.ErrNotExist)
				}
			})
		}
	})

	t.Run("neutral recovery recognizes already-landed cross-directory effects idempotently", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			land    func(*testing.T, *os.Root)
			name    string
			install filestore.InstallMode
		}{
			{
				name:    "create observes its link before staged-name cleanup",
				install: filestore.InstallCreate,
				land: func(t *testing.T, root *os.Root) {
					t.Helper()
					if err := root.Link(filepath.Join("staging", ".stage"), filepath.Join("objects", "digest")); err != nil {
						t.Fatal(err)
					}
				},
			},
			{
				name:    "replace observes its rename after staged-name consumption",
				install: filestore.InstallReplace,
				land: func(t *testing.T, root *os.Root) {
					t.Helper()
					if err := root.Rename(filepath.Join("staging", ".stage"), filepath.Join("objects", "digest")); err != nil {
						t.Fatal(err)
					}
				},
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				rootDirectory := t.TempDir()
				root := requireTestRoot(t, rootDirectory)
				if err := os.Mkdir(filepath.Join(rootDirectory, "staging"), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(filepath.Join(rootDirectory, "objects"), 0o700); err != nil {
					t.Fatal(err)
				}
				staged := mustStage(t, root, filepath.Join("staging", ".stage"), "candidate")
				tc.land(t, root)
				request := filestore.CommitRequest{
					Staged:  staged,
					Target:  mustRelativePath(t, filepath.Join("objects", "digest")),
					Install: tc.install,
				}
				for attempt := range 3 {
					if gotErr := filestore.Recover(t.Context(), request); gotErr != nil {
						t.Fatalf("Recover(cross-directory) attempt %d error = %v, want nil", attempt, gotErr)
					}
					requireOptionalFile(t, filepath.Join(rootDirectory, "objects", "digest"), "candidate")
					requireDirectoryEntryNames(t, filepath.Join(rootDirectory, "staging"), nil)
				}
			})
		}
	})
}
