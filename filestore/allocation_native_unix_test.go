//go:build darwin || linux

package filestore_test

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"os"
	"path/filepath"
	"testing"
)

// Actual Go Stat_t blocks are the oracle. Compression and sparse-file policy
// belong to the filesystem; a dense allocation need not exceed logical size.
func TestInspectReportsRealStorageBehindARegularFile(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		kind   filestore.PathKind
		extent int64
		dense  bool
	}{
		{name: "empty regular file reports native zero allocation", kind: filestore.PathKindRegularFile},
		{name: "dense bytes cannot substitute logical size for allocation", kind: filestore.PathKindRegularFile, extent: 96 << 10, dense: true},
		{name: "sparse extent reports actual backing rather than its claim", kind: filestore.PathKindRegularFile, extent: 8 << 20},
		{name: "directory cannot fabricate regular allocation", kind: filestore.PathKindDirectory},
		{name: "absent file cannot fabricate reported zero", kind: filestore.PathKindAbsent},
		{name: "symlink cannot borrow target allocation", kind: filestore.PathKindSymbolicLink},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			name := filepath.Join(directory, "entry")
			switch tc.kind {
			case filestore.PathKindRegularFile:
				file, err := os.Create(name)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := file.Close(); err != nil {
						t.Error(err)
					}
				})
				if tc.dense {
					payload := deterministicPayload(int(tc.extent))
					if n, err := file.Write(payload); err != nil || n != len(payload) {
						t.Fatalf("fixture bytes = (%d,%v), want %d", n, err, len(payload))
					}
				} else if err := file.Truncate(tc.extent); err != nil {
					t.Fatal(err)
				}
			case filestore.PathKindDirectory:
				if err := os.Mkdir(name, 0o700); err != nil {
					t.Fatal(err)
				}
			case filestore.PathKindSymbolicLink:
				if err := os.Symlink("missing", name); err != nil {
					t.Fatal(err)
				}
			case filestore.PathKindAbsent:
			default:
				t.Fatalf("fixture kind = %v, want table-owned native kind", tc.kind)
			}
			before, beforeErr := os.Lstat(name)
			path, err := core.ParseAbsolutePath(name)
			if err != nil {
				t.Fatal(err)
			}
			got, err := filestore.Inspect(t.Context(), path)
			if err != nil || got.Validate() != nil {
				t.Fatalf("Inspect = (%v,%v), want admitted observation", got, err)
			}
			allocation, gotErr := got.Allocation()
			if tc.kind != filestore.PathKindRegularFile {
				if !errors.Is(gotErr, core.ErrFilestoreContract) || allocation != (filestore.Allocation{}) {
					t.Fatalf("non-regular allocation = (%v,%v), want zero and contract refusal", allocation, gotErr)
				}
			} else {
				if beforeErr != nil {
					t.Fatal(beforeErr)
				}
				native, err := nativeInspectionStorage(before)
				if err != nil {
					t.Fatal(err)
				}
				allocated, bytesErr := allocation.Bytes()
				size, sizeErr := got.SizeBytes()
				if gotErr != nil || bytesErr != nil || sizeErr != nil || !allocation.Reported() || allocated.Uint64() != native.bytes || size.Uint64() != uint64(tc.extent) {
					t.Fatalf("allocation/extent = (%v,%v,%v,%v,%v), want native %d and extent %d", allocation, size, gotErr, bytesErr, sizeErr, native.bytes, tc.extent)
				}
			}
			after, err := os.Lstat(name)
			if tc.kind == filestore.PathKindAbsent {
				if !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("absent after = %v, want native absence", err)
				}
			} else if beforeErr != nil || err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
				t.Fatalf("entry custody = (%v,%v), want unchanged native entry", after, err)
			}
		})
	}
}
