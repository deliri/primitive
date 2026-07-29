package filestore_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestDirectoryEffectLayerTriad(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		setup     func(*testing.T, string)
		path      string
		wantErr   error
		wantNames []string
	}{
		{
			name:      "positive nested directory chain is created with an exact final mode",
			path:      filepath.Join("a", "b"),
			wantNames: []string{"a"},
		},
		{
			name: "negative file obstacle remains a file and preserves native activation identity",
			setup: func(t *testing.T, rootDirectory string) {
				if err := os.WriteFile(filepath.Join(rootDirectory, "a"), []byte("obstacle"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			path:      filepath.Join("a", "b"),
			wantErr:   core.ErrFilestoreActivation,
			wantNames: []string{"a"},
		},
		{
			name: "neutral existing directory is synchronized without creating sibling noise",
			setup: func(t *testing.T, rootDirectory string) {
				if err := os.Mkdir(filepath.Join(rootDirectory, "a"), 0o700); err != nil {
					t.Fatal(err)
				}
			},
			path:      "a",
			wantNames: []string{"a"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rootDirectory := t.TempDir()
			if tc.setup != nil {
				tc.setup(t, rootDirectory)
			}
			root := requireTestRoot(t, rootDirectory)
			gotErr := filestore.EnsureDirectory(t.Context(), filestore.DirectoryRequest{
				Location: filestore.Location{Root: root, Path: mustRelativePath(t, tc.path)},
				Mode:     0o750,
			})
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("EnsureDirectory() error = %v, want %v", gotErr, tc.wantErr)
				}
			} else if gotErr != nil {
				t.Fatalf("EnsureDirectory() error = %v, want nil", gotErr)
			}
			requireDirectoryEntryNames(t, rootDirectory, tc.wantNames)
			if tc.wantErr == nil {
				info, err := os.Stat(filepath.Join(rootDirectory, tc.path))
				if err != nil {
					t.Fatal(err)
				}
				if !info.IsDir() || info.Mode().Perm() != 0o750 {
					t.Fatalf("directory = mode:%v permissions:%#o, want directory/%#o", info.Mode(), info.Mode().Perm(), os.FileMode(0o750))
				}
			}
		})
	}
}

func TestBoundedReadLayerTriad(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		source     []byte
		path       string
		want       []byte
		wantErr    error
		wantNative error
	}{
		{
			name:   "positive regular file streams its exact bytes and count",
			path:   "source",
			source: []byte("ledger\n"),
			want:   []byte("ledger\n"),
		},
		{
			name:       "negative missing source preserves typed and native not-exist identities",
			path:       "missing",
			wantErr:    core.ErrFilestoreSource,
			wantNative: os.ErrNotExist,
		},
		{
			name:   "neutral empty regular file emits zero bytes and no fabricated output",
			path:   "empty",
			source: []byte{},
			want:   []byte{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rootDirectory := t.TempDir()
			if tc.path != "missing" {
				if err := os.WriteFile(filepath.Join(rootDirectory, tc.path), tc.source, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			root := requireTestRoot(t, rootDirectory)
			var destination bytes.Buffer
			gotCount, gotErr := filestore.Read(t.Context(), filestore.ReadRequest{
				Destination:  &destination,
				Location:     filestore.Location{Root: root, Path: mustRelativePath(t, tc.path)},
				MaximumBytes: mustByteCount(t, uint64(max(len(tc.source), 1))),
			})
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || !errors.Is(gotErr, tc.wantNative) {
					t.Fatalf("Read() error = %v, want %v and %v", gotErr, tc.wantErr, tc.wantNative)
				}
			} else if gotErr != nil {
				t.Fatalf("Read() error = %v, want nil", gotErr)
			}
			if gotCount.Uint64() != uint64(len(tc.want)) {
				t.Fatalf("Read() count = %d, want %d", gotCount.Uint64(), len(tc.want))
			}
			if !bytes.Equal(destination.Bytes(), tc.want) {
				t.Fatalf("Read() bytes = %q, want %q", destination.Bytes(), tc.want)
			}
		})
	}
}

func TestTargetLateWriterLayerTriad(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr       error
		name          string
		initial       string
		source        string
		wantTarget    string
		install       filestore.InstallMode
		wantStageLeft bool
	}{
		{
			name:       "positive target is selected after staging and durably created",
			source:     "digest-derived payload",
			install:    filestore.InstallCreate,
			wantTarget: "digest-derived payload",
		},
		{
			name:          "negative create conflict preserves both existing target and owned stage",
			initial:       "existing",
			source:        "candidate",
			install:       filestore.InstallCreate,
			wantTarget:    "existing",
			wantErr:       core.ErrFilestoreConflict,
			wantStageLeft: true,
		},
		{
			name:       "neutral zero-byte stage publishes an exact empty target",
			source:     "",
			install:    filestore.InstallCreate,
			wantTarget: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rootDirectory := t.TempDir()
			root := requireTestRoot(t, rootDirectory)
			if tc.initial != "" {
				if err := os.WriteFile(filepath.Join(rootDirectory, "target"), []byte(tc.initial), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			staged := mustStage(t, root, ".stage", tc.source)
			gotErr := filestore.Commit(t.Context(), filestore.CommitRequest{
				Staged:  staged,
				Target:  mustRelativePath(t, "target"),
				Install: tc.install,
			})
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("Commit() error = %v, want %v", gotErr, tc.wantErr)
				}
			} else if gotErr != nil {
				t.Fatalf("Commit() error = %v, want nil", gotErr)
			}
			got, err := os.ReadFile(filepath.Join(rootDirectory, "target"))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.wantTarget {
				t.Fatalf("target bytes = %q, want %q", got, tc.wantTarget)
			}
			_, stageErr := os.Stat(filepath.Join(rootDirectory, ".stage"))
			if tc.wantStageLeft {
				if stageErr != nil {
					t.Fatalf("Stat(stage) error = %v, want nil", stageErr)
				}
				return
			}
			if !errors.Is(stageErr, os.ErrNotExist) {
				t.Fatalf("Stat(stage) error = %v, want %v", stageErr, os.ErrNotExist)
			}
		})
	}
}

func TestRecoveryLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive create partial effect is completed from real names and file identity", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		staged := mustStage(t, root, ".stage", "candidate")
		target := mustRelativePath(t, "target")
		if err := root.Link(staged.Path().String(), target.String()); err != nil {
			t.Fatal(err)
		}
		gotErr := filestore.Recover(t.Context(), filestore.CommitRequest{
			Staged: staged, Target: target, Install: filestore.InstallCreate,
		})
		if gotErr != nil {
			t.Fatalf("Recover() error = %v, want nil", gotErr)
		}
		requireDirectoryEntryNames(t, rootDirectory, []string{"target"})
	})
	t.Run("negative foreign stage identity is rejected without consuming the foreign file", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		staged := mustStage(t, root, ".stage", "candidate")
		if err := root.Remove(staged.Path().String()); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(rootDirectory, ".stage"), []byte("foreign"), 0o600); err != nil {
			t.Fatal(err)
		}
		gotErr := filestore.Recover(t.Context(), filestore.CommitRequest{
			Staged: staged, Target: mustRelativePath(t, "target"), Install: filestore.InstallCreate,
		})
		if !errors.Is(gotErr, core.ErrFilestoreActivationIndeterminate) {
			t.Fatalf("Recover() error = %v, want %v", gotErr, core.ErrFilestoreActivationIndeterminate)
		}
		got, err := os.ReadFile(filepath.Join(rootDirectory, ".stage"))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "foreign" {
			t.Fatalf("foreign stage bytes = %q, want %q", got, "foreign")
		}
	})
	t.Run("neutral repeated recovery of an already activated replacement creates no residue", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		staged := mustStage(t, root, ".stage", "replacement")
		target := mustRelativePath(t, "target")
		if err := root.Rename(staged.Path().String(), target.String()); err != nil {
			t.Fatal(err)
		}
		request := filestore.CommitRequest{
			Staged: staged, Target: target, Install: filestore.InstallReplace,
		}
		for attempt := range 3 {
			if gotErr := filestore.Recover(t.Context(), request); gotErr != nil {
				t.Fatalf("Recover() attempt %d error = %v, want nil", attempt, gotErr)
			}
		}
		requireDirectoryEntryNames(t, rootDirectory, []string{"target"})
	})
}

func TestDiscardLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive owned stage is removed while an unrelated sibling survives", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		staged := mustStage(t, root, ".stage", "candidate")
		if err := os.WriteFile(filepath.Join(rootDirectory, "sibling"), []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
		if gotErr := filestore.Discard(t.Context(), staged); gotErr != nil {
			t.Fatalf("Discard() error = %v, want nil", gotErr)
		}
		requireDirectoryEntryNames(t, rootDirectory, []string{"sibling"})
	})
	t.Run("negative foreign replacement is rejected and preserved", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		staged := mustStage(t, root, ".stage", "candidate")
		if err := root.Remove(staged.Path().String()); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(rootDirectory, ".stage"), []byte("foreign"), 0o600); err != nil {
			t.Fatal(err)
		}
		gotErr := filestore.Discard(t.Context(), staged)
		if !errors.Is(gotErr, core.ErrFilestoreCleanup) ||
			!errors.Is(gotErr, core.ErrFilestoreConflict) {
			t.Fatalf("Discard() error = %v, want %v and %v", gotErr, core.ErrFilestoreCleanup, core.ErrFilestoreConflict)
		}
		got, err := os.ReadFile(filepath.Join(rootDirectory, ".stage"))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "foreign" {
			t.Fatalf("foreign stage bytes = %q, want %q", got, "foreign")
		}
	})
	t.Run("neutral repeated discard of an absent owned name creates no filesystem noise", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		staged := mustStage(t, root, ".stage", "")
		for attempt := range 3 {
			if gotErr := filestore.Discard(t.Context(), staged); gotErr != nil {
				t.Fatalf("Discard() attempt %d error = %v, want nil", attempt, gotErr)
			}
		}
		requireDirectoryEntryNames(t, rootDirectory, nil)
	})
}

func TestRemovalLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive one named file is removed while its sibling survives", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		if err := os.WriteFile(filepath.Join(rootDirectory, "remove"), []byte("gone"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(rootDirectory, "keep"), []byte("kept"), 0o600); err != nil {
			t.Fatal(err)
		}
		gotErr := filestore.Remove(t.Context(), filestore.RemovalRequest{
			Location: filestore.Location{Root: root, Path: mustRelativePath(t, "remove")},
		})
		if gotErr != nil {
			t.Fatalf("Remove() error = %v, want nil", gotErr)
		}
		requireDirectoryEntryNames(t, rootDirectory, []string{"keep"})
	})
	t.Run("negative nonempty directory is preserved instead of recursively removed", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		if err := os.Mkdir(filepath.Join(rootDirectory, "tree"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(rootDirectory, "tree", "child"), []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
		gotErr := filestore.Remove(t.Context(), filestore.RemovalRequest{
			Location: filestore.Location{Root: root, Path: mustRelativePath(t, "tree")},
		})
		if !errors.Is(gotErr, core.ErrFilestoreCleanup) {
			t.Fatalf("Remove() error = %v, want %v", gotErr, core.ErrFilestoreCleanup)
		}
		got, err := os.ReadFile(filepath.Join(rootDirectory, "tree", "child"))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "keep" {
			t.Fatalf("preserved child bytes = %q, want %q", got, "keep")
		}
	})
	t.Run("neutral missing leaf and missing parent are idempotent no-ops that preserve siblings", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		if err := os.WriteFile(filepath.Join(rootDirectory, "keep"), []byte("kept"), 0o600); err != nil {
			t.Fatal(err)
		}
		missingPaths := []string{"missing", filepath.Join("missing-parent", "missing")}
		for _, missingPath := range missingPaths {
			gotErr := filestore.Remove(t.Context(), filestore.RemovalRequest{
				Location: filestore.Location{Root: root, Path: mustRelativePath(t, missingPath)},
			})
			if gotErr != nil {
				t.Fatalf("Remove(%q) error = %v, want nil", missingPath, gotErr)
			}
		}
		requireDirectoryEntryNames(t, rootDirectory, []string{"keep"})
	})
}
