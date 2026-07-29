package filestore_test

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestEnsureDirectoryPreservesNativeFileObstacle(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	root := requireTestRoot(t, rootDirectory)
	if err := os.WriteFile(filepath.Join(rootDirectory, "obstacle"), []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	gotErr := filestore.EnsureDirectory(t.Context(), filestore.DirectoryRequest{
		Location: filestore.Location{
			Root: root,
			Path: mustRelativePath(t, filepath.Join("obstacle", "child")),
		},
		Mode: 0o700,
	})
	var pathErr *os.PathError
	if !errors.Is(gotErr, core.ErrFilestoreActivation) || !errors.As(gotErr, &pathErr) {
		t.Fatalf("EnsureDirectory(file obstacle) error = %v, want %v and *os.PathError", gotErr, core.ErrFilestoreActivation)
	}
	got, err := os.ReadFile(filepath.Join(rootDirectory, "obstacle"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "file" {
		t.Fatalf("obstacle bytes = %q, want %q", got, "file")
	}
}

func TestReadHostileSourceAndDestinationMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		native  error
		prepare func(*testing.T, string) (string, *os.File)
		name    string
	}{
		{
			name: "missing source preserves not-exist",
			prepare: func(_ *testing.T, _ string) (string, *os.File) {
				return "missing", nil
			},
			wantErr: core.ErrFilestoreSource,
			native:  fs.ErrNotExist,
		},
		{
			name: "directory source is not a regular byte stream",
			prepare: func(t *testing.T, rootDirectory string) (string, *os.File) {
				if err := os.Mkdir(filepath.Join(rootDirectory, "directory"), 0o700); err != nil {
					t.Fatal(err)
				}
				return "directory", nil
			},
			wantErr: core.ErrFilestoreSource,
			native:  fs.ErrInvalid,
		},
		{
			name: "closed real destination preserves closed-handle identity",
			prepare: func(t *testing.T, rootDirectory string) (string, *os.File) {
				if err := os.WriteFile(filepath.Join(rootDirectory, "source"), []byte("payload"), 0o600); err != nil {
					t.Fatal(err)
				}
				destination, err := os.Create(filepath.Join(rootDirectory, "destination"))
				if err != nil {
					t.Fatal(err)
				}
				if err := destination.Close(); err != nil {
					t.Fatal(err)
				}
				return "source", destination
			},
			wantErr: core.ErrFilestoreDestination,
			native:  os.ErrClosed,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rootDirectory := t.TempDir()
			root := requireTestRoot(t, rootDirectory)
			path, fileDestination := tc.prepare(t, rootDirectory)
			var buffer bytes.Buffer
			var destination io.Writer = &buffer
			if fileDestination != nil {
				destination = fileDestination
			}
			gotCount, gotErr := filestore.Read(t.Context(), filestore.ReadRequest{
				Destination: destination,
				Location: filestore.Location{
					Root: root,
					Path: mustRelativePath(t, path),
				},
				MaximumBytes: mustByteCount(t, 64),
			})
			if !errors.Is(gotErr, tc.wantErr) || !errors.Is(gotErr, tc.native) {
				t.Fatalf("Read() error = %v, want %v and %v", gotErr, tc.wantErr, tc.native)
			}
			if gotCount.Uint64() != 0 {
				t.Fatalf("Read() count = %d, want 0", gotCount.Uint64())
			}
		})
	}
}

func TestWriteAndStageRefuseCallerNamedTemporaryConflicts(t *testing.T) {
	t.Parallel()

	cases := []struct {
		run  func(*testing.T, *os.Root) error
		name string
	}{
		{
			name: "one-shot write",
			run: func(t *testing.T, root *os.Root) error {
				_, err := filestore.Write(t.Context(), filestore.WriteRequest{
					Source:       strings.NewReader("new"),
					Location:     filestore.Location{Root: root, Path: mustRelativePath(t, "target")},
					Temporary:    mustRelativePath(t, ".stage"),
					Mode:         0o600,
					Install:      filestore.InstallCreate,
					MaximumBytes: mustByteCount(t, 3),
				})
				return err
			},
		},
		{
			name: "target-late stage",
			run: func(t *testing.T, root *os.Root) error {
				_, err := filestore.Stage(t.Context(), filestore.StageRequest{
					Source:       strings.NewReader("new"),
					Temporary:    filestore.Location{Root: root, Path: mustRelativePath(t, ".stage")},
					Mode:         0o600,
					MaximumBytes: mustByteCount(t, 3),
				})
				return err
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rootDirectory := t.TempDir()
			root := requireTestRoot(t, rootDirectory)
			if err := os.WriteFile(filepath.Join(rootDirectory, ".stage"), []byte("owned elsewhere"), 0o600); err != nil {
				t.Fatal(err)
			}
			gotErr := tc.run(t, root)
			if !errors.Is(gotErr, core.ErrFilestoreConflict) || !errors.Is(gotErr, os.ErrExist) {
				t.Fatalf("%s temporary conflict error = %v, want %v and %v", tc.name, gotErr, core.ErrFilestoreConflict, os.ErrExist)
			}
			got, err := os.ReadFile(filepath.Join(rootDirectory, ".stage"))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != "owned elsewhere" {
				t.Fatalf("%s temporary bytes = %q, want %q", tc.name, got, "owned elsewhere")
			}
			requireDirectoryEntryNames(t, rootDirectory, []string{".stage"})
		})
	}
}

func TestWriteReplaceFailurePreservesTargetDirectoryAndCleansStage(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	root := requireTestRoot(t, rootDirectory)
	if err := os.Mkdir(filepath.Join(rootDirectory, "target"), 0o700); err != nil {
		t.Fatal(err)
	}
	_, gotErr := filestore.Write(t.Context(), filestore.WriteRequest{
		Source:       strings.NewReader("payload"),
		Location:     filestore.Location{Root: root, Path: mustRelativePath(t, "target")},
		Temporary:    mustRelativePath(t, ".stage"),
		Mode:         0o600,
		Install:      filestore.InstallReplace,
		MaximumBytes: mustByteCount(t, 7),
	})
	var linkErr *os.LinkError
	if !errors.Is(gotErr, core.ErrFilestoreActivation) || !errors.As(gotErr, &linkErr) {
		t.Fatalf("Write(replace directory) error = %v, want %v and *os.LinkError", gotErr, core.ErrFilestoreActivation)
	}
	info, err := os.Stat(filepath.Join(rootDirectory, "target"))
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatalf("target mode = %v, want directory", info.Mode())
	}
	requireDirectoryEntryNames(t, rootDirectory, []string{"target"})
}

func TestRemoveRefusesRecursivePolicyAndPreservesNonemptyTree(t *testing.T) {
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
	var pathErr *os.PathError
	if !errors.Is(gotErr, core.ErrFilestoreCleanup) || !errors.As(gotErr, &pathErr) {
		t.Fatalf("Remove(nonempty directory) error = %v, want %v and *os.PathError", gotErr, core.ErrFilestoreCleanup)
	}
	got, err := os.ReadFile(filepath.Join(rootDirectory, "tree", "child"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "keep" {
		t.Fatalf("tree child bytes = %q, want %q", got, "keep")
	}
}

func TestReadUsesSafeInRootSymlinkResolutionOwnedByOSRoot(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	root := requireTestRoot(t, rootDirectory)
	if err := os.WriteFile(filepath.Join(rootDirectory, "target"), []byte("inside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target", filepath.Join(rootDirectory, "alias")); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}
	var destination bytes.Buffer
	gotCount, gotErr := filestore.Read(t.Context(), filestore.ReadRequest{
		Destination:  &destination,
		Location:     filestore.Location{Root: root, Path: mustRelativePath(t, "alias")},
		MaximumBytes: mustByteCount(t, 6),
	})
	if gotErr != nil {
		t.Fatalf("Read(in-root symlink) error = %v, want nil", gotErr)
	}
	if gotCount.Uint64() != 6 || destination.String() != "inside" {
		t.Fatalf("Read(in-root symlink) = count:%d bytes:%q, want 6/%q", gotCount.Uint64(), destination.String(), "inside")
	}
}

func TestClosedOSRootNativeErrorMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		run     func(*testing.T, *os.Root) error
		name    string
	}{
		{
			name:    "ensure directory",
			wantErr: core.ErrFilestoreActivation,
			run: func(t *testing.T, root *os.Root) error {
				return filestore.EnsureDirectory(t.Context(), filestore.DirectoryRequest{
					Location: filestore.Location{Root: root, Path: mustRelativePath(t, "directory")},
					Mode:     0o700,
				})
			},
		},
		{
			name:    "read",
			wantErr: core.ErrFilestoreSource,
			run: func(t *testing.T, root *os.Root) error {
				_, err := filestore.Read(t.Context(), filestore.ReadRequest{
					Destination:  io.Discard,
					Location:     filestore.Location{Root: root, Path: mustRelativePath(t, "target")},
					MaximumBytes: mustByteCount(t, 1),
				})
				return err
			},
		},
		{
			name:    "write",
			wantErr: core.ErrFilestoreActivation,
			run: func(t *testing.T, root *os.Root) error {
				_, err := filestore.Write(t.Context(), filestore.WriteRequest{
					Source:       strings.NewReader("x"),
					Location:     filestore.Location{Root: root, Path: mustRelativePath(t, "target")},
					Temporary:    mustRelativePath(t, ".stage"),
					Mode:         0o600,
					Install:      filestore.InstallCreate,
					MaximumBytes: mustByteCount(t, 1),
				})
				return err
			},
		},
		{
			name:    "stage",
			wantErr: core.ErrFilestoreActivation,
			run: func(t *testing.T, root *os.Root) error {
				_, err := filestore.Stage(t.Context(), filestore.StageRequest{
					Source:       strings.NewReader("x"),
					Temporary:    filestore.Location{Root: root, Path: mustRelativePath(t, ".stage")},
					Mode:         0o600,
					MaximumBytes: mustByteCount(t, 1),
				})
				return err
			},
		},
		{
			name:    "open append",
			wantErr: core.ErrFilestoreActivation,
			run: func(t *testing.T, root *os.Root) error {
				file, err := filestore.OpenAppend(t.Context(), filestore.AppendRequest{
					Location: filestore.Location{Root: root, Path: mustRelativePath(t, "ledger")},
					Mode:     0o600,
					Append:   filestore.AppendCreate,
				})
				if file != nil {
					if closeErr := file.Close(); closeErr != nil {
						return errors.Join(err, closeErr)
					}
				}
				return err
			},
		},
		{
			name:    "remove",
			wantErr: core.ErrFilestoreCleanup,
			run: func(t *testing.T, root *os.Root) error {
				return filestore.Remove(t.Context(), filestore.RemovalRequest{
					Location: filestore.Location{Root: root, Path: mustRelativePath(t, "target")},
				})
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rootDirectory := t.TempDir()
			root, err := os.OpenRoot(rootDirectory)
			if err != nil {
				t.Fatal(err)
			}
			if err := root.Close(); err != nil {
				t.Fatal(err)
			}
			gotErr := tc.run(t, root)
			if !errors.Is(gotErr, tc.wantErr) || !errors.Is(gotErr, os.ErrClosed) {
				t.Fatalf("%s with closed os.Root error = %v, want %v and %v", tc.name, gotErr, tc.wantErr, os.ErrClosed)
			}
			requireDirectoryEntryNames(t, rootDirectory, nil)
		})
	}
}
