package filestore_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// TestOpenParentNamesTheEntryInsideItsParent proves the split is the one every
// caller was writing by hand: the root is the parent directory and the path is
// the entry's own name, whatever depth the entry sits at.
func TestOpenParentNamesTheEntryInsideItsParent(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	renameMakeDirectory("nested/deeper")(t, directory)
	renameWriteFile("nested/deeper/entry")(t, directory)

	cases := []struct {
		name     string
		suffix   string
		wantName string
	}{
		{name: "entry directly in the directory", suffix: "top", wantName: "top"},
		{name: "entry one level down", suffix: "nested/child", wantName: "child"},
		{name: "entry two levels down", suffix: "nested/deeper/entry", wantName: "entry"},
		{name: "absent entry still names its parent", suffix: "nested/missing", wantName: "missing"},
		{name: "directory names itself inside its parent", suffix: "nested", wantName: "nested"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			path, err := core.ParseAbsolutePath(filepath.Join(directory, tc.suffix))
			if err != nil {
				t.Fatalf("ParseAbsolutePath() error = %v, want nil", err)
			}
			location, err := filestore.OpenParent(t.Context(), path)
			if err != nil {
				t.Fatalf("OpenParent(%s) error = %v, want nil", tc.suffix, err)
			}
			defer func() {
				if closeErr := location.Root.Close(); closeErr != nil {
					t.Errorf("Close() error = %v, want nil", closeErr)
				}
			}()

			if location.Path.String() != tc.wantName {
				t.Fatalf("OpenParent(%s) path = %q, want %q", tc.suffix, location.Path.String(), tc.wantName)
			}
			wantRoot := filepath.Dir(filepath.Join(directory, tc.suffix))
			if location.Root.Name() != wantRoot {
				t.Fatalf("OpenParent(%s) root = %q, want %q", tc.suffix, location.Root.Name(), wantRoot)
			}
			if err := location.Validate(); err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
		})
	}
}

// TestOpenParentRefusesWhatItCannotOpen proves a failure hands back nothing to
// close. A caller that received a half-built Location would either leak the
// handle or close a nil one.
func TestOpenParentRefusesWhatItCannotOpen(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	renameWriteFile("occupied")(t, directory)

	cases := []struct {
		wantErr error
		build   func(*testing.T) core.AbsolutePath
		name    string
	}{
		{
			name:    "unvalidated path",
			build:   func(*testing.T) core.AbsolutePath { return core.AbsolutePath{} },
			wantErr: core.ErrFilestoreContract,
		},
		{
			name: "parent does not exist",
			build: func(t *testing.T) core.AbsolutePath {
				path, err := core.ParseAbsolutePath(filepath.Join(directory, "missing", "entry"))
				if err != nil {
					t.Fatalf("ParseAbsolutePath() error = %v, want nil", err)
				}
				return path
			},
			wantErr: core.ErrFilestoreSource,
		},
		{
			name: "parent is a regular file",
			build: func(t *testing.T) core.AbsolutePath {
				path, err := core.ParseAbsolutePath(filepath.Join(directory, "occupied", "entry"))
				if err != nil {
					t.Fatalf("ParseAbsolutePath() error = %v, want nil", err)
				}
				return path
			},
			wantErr: core.ErrFilestoreSource,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			location, err := filestore.OpenParent(t.Context(), tc.build(t))
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("OpenParent() error = %v, want errors.Is %v", err, tc.wantErr)
			}
			if location.Root != nil {
				_ = location.Root.Close()
				t.Fatalf("OpenParent() returned a root alongside a refusal, want none")
			}
		})
	}
}

// TestOpenParentComposesWithTheOperationsItFeeds proves the returned Location
// is directly usable, which is the entire point: the caller should not have to
// touch it before handing it to a filestore request.
func TestOpenParentComposesWithTheOperationsItFeeds(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	renameWriteFile("entry")(t, directory)
	path, err := core.ParseAbsolutePath(filepath.Join(directory, "entry"))
	if err != nil {
		t.Fatalf("ParseAbsolutePath() error = %v, want nil", err)
	}

	location, err := filestore.OpenParent(t.Context(), path)
	if err != nil {
		t.Fatalf("OpenParent() error = %v, want nil", err)
	}
	handle, err := filestore.OpenRead(t.Context(), filestore.ReadHandleRequest{Location: location})
	if err != nil {
		t.Fatalf("OpenRead(location from OpenParent) error = %v, want nil", err)
	}
	if closeErr := errors.Join(handle.Close(), location.Root.Close()); closeErr != nil {
		t.Fatalf("Close() error = %v, want nil", closeErr)
	}

	if _, statErr := os.Lstat(filepath.Join(directory, "entry")); statErr != nil {
		t.Fatalf("Lstat() error = %v, want the entry untouched", statErr)
	}
}
