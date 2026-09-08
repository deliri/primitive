package filestore_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

type heldDirectoryFixture uint8

const (
	heldDirectoryEmpty heldDirectoryFixture = iota
	heldDirectoryRenamed
	heldDirectoryRegular
	heldDirectoryMissing
	heldDirectoryZeroPath
	heldDirectoryCancelled
	heldDirectoryNilContext
	heldDirectoryBorrowerClosed
)

func TestOpenDirectoryLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		fixture       heldDirectoryFixture
		wantErr       error
		wantNativeErr error
	}{
		{name: "empty directory acquisition and release create no children", fixture: heldDirectoryEmpty},
		{name: "renamed directory retains original inode and child through replaced path", fixture: heldDirectoryRenamed},
		{name: "regular file cannot become a directory capability", fixture: heldDirectoryRegular, wantErr: core.ErrFilestoreSource},
		{name: "missing directory preserves native absence without creating it", fixture: heldDirectoryMissing, wantErr: core.ErrFilestoreSource, wantNativeErr: fs.ErrNotExist},
		{name: "zero path cannot acquire the working directory", fixture: heldDirectoryZeroPath, wantErr: core.ErrFilestoreContract},
		{name: "cancelled context refuses before filesystem acquisition", fixture: heldDirectoryCancelled, wantErr: context.Canceled},
		{name: "nil context cannot acquire an existing directory", fixture: heldDirectoryNilContext, wantErr: core.ErrNilContext},
		{name: "borrower closing the file cannot turn owner close into success", fixture: heldDirectoryBorrowerClosed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			parent := t.TempDir()
			name := filepath.Join(parent, "directory")
			if err := os.Mkdir(name, 0o700); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(name)
			if err != nil {
				t.Fatal(err)
			}
			path := openRootAbsolute(t, name)
			ctx := t.Context()
			switch tc.fixture {
			case heldDirectoryRegular:
				name = filepath.Join(parent, "regular")
				if err := os.WriteFile(name, []byte{0, 255}, 0o600); err != nil {
					t.Fatal(err)
				}
				path = openRootAbsolute(t, name)
			case heldDirectoryMissing:
				name = filepath.Join(parent, "missing")
				path = openRootAbsolute(t, name)
			case heldDirectoryZeroPath:
				path = core.AbsolutePath{}
			case heldDirectoryCancelled:
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case heldDirectoryNilContext:
				ctx = nil
			}
			directory, gotErr := filestore.OpenDirectory(ctx, path)
			if directory != nil {
				t.Cleanup(func() {
					if directory.Validate() == nil {
						if err := directory.Close(); err != nil {
							t.Error(err)
						}
					}
				})
			}
			if tc.wantErr != nil {
				if directory != nil || !errors.Is(gotErr, tc.wantErr) || (tc.wantNativeErr != nil && !errors.Is(gotErr, tc.wantNativeErr)) {
					t.Fatalf("directory/error = (%v,%v), want nil and (%v,%v)", directory, gotErr, tc.wantErr, tc.wantNativeErr)
				}
				if errors.Is(tc.wantErr, core.ErrFilestoreSource) {
					if _, ok := errors.AsType[*fs.PathError](gotErr); !ok {
						t.Fatalf("source refusal = %v, want native PathError", gotErr)
					}
				}
				entries, err := os.ReadDir(parent)
				wantEntries := 1
				if tc.fixture == heldDirectoryRegular {
					wantEntries++
				}
				if err != nil || len(entries) != wantEntries {
					t.Fatalf("refused namespace = (%v,%v), want %d unchanged entries", entries, err, wantEntries)
				}
				if tc.fixture == heldDirectoryRegular {
					got, err := os.ReadFile(name)
					if err != nil || len(got) != 2 || got[0] != 0 || got[1] != 255 {
						t.Fatalf("refused file = (%v,%v), want untouched binary payload", got, err)
					}
				}
				return
			}
			if gotErr != nil || directory == nil {
				t.Fatalf("directory/error = (%v,%v), want live capability and nil", directory, gotErr)
			}
			file, err := directory.File()
			if err != nil {
				t.Fatal(err)
			}
			filesystem, err := directory.Filesystem()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := filesystem.Uint64(); err != nil {
				t.Fatal(err)
			}
			wantEntries := 0
			if tc.fixture == heldDirectoryRenamed {
				if err := os.WriteFile(filepath.Join(name, "original-child"), []byte{0, 255}, 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(name, filepath.Join(parent, "retained")); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(name, 0o700); err != nil {
					t.Fatal(err)
				}
				wantEntries = 1
			}
			got, err := file.Stat()
			if err != nil || !os.SameFile(before, got) {
				t.Fatalf("borrowed inode = (%v,%v), want original directory", got, err)
			}
			entries, err := file.ReadDir(-1)
			if err != nil || len(entries) != wantEntries {
				t.Fatalf("held children = (%v,%v), want %d original children", entries, err, wantEntries)
			}
			if wantEntries == 1 && entries[0].Name() != "original-child" {
				t.Fatalf("held child = %q, want original-child", entries[0].Name())
			}
			if tc.fixture == heldDirectoryBorrowerClosed {
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
			}
			closeErr := directory.Close()
			if tc.fixture == heldDirectoryBorrowerClosed {
				if !errors.Is(closeErr, core.ErrFilestoreSource) || !errors.Is(closeErr, fs.ErrClosed) {
					t.Fatalf("owner close = %v, want source/native closed", closeErr)
				}
			} else if closeErr != nil {
				t.Fatal(closeErr)
			}
			if _, err := file.Stat(); !errors.Is(err, fs.ErrClosed) {
				t.Fatalf("borrowed file after close = %v, want closed", err)
			}
			if err := directory.Validate(); !errors.Is(err, core.ErrFilestoreContract) {
				t.Fatalf("released directory validation = %v, want contract refusal", err)
			}
			if file, err := directory.File(); file != nil || !errors.Is(err, core.ErrFilestoreContract) {
				t.Fatalf("released file = (%v,%v), want absent/contract", file, err)
			}
			if identity, err := directory.Filesystem(); identity != (filestore.FilesystemIdentity{}) || !errors.Is(err, core.ErrFilestoreContract) {
				t.Fatalf("released filesystem = (%v,%v), want zero/contract", identity, err)
			}
			if err := directory.Close(); !errors.Is(err, core.ErrFilestoreContract) {
				t.Fatalf("second close = %v, want contract refusal", err)
			}
			if tc.fixture == heldDirectoryRenamed {
				entries, err := os.ReadDir(name)
				if err != nil || len(entries) != 0 {
					t.Fatalf("replacement directory = (%v,%v), want empty and untouched", entries, err)
				}
			}
		})
	}
}
