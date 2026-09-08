//go:build darwin || linux

package filestore_test

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// Unix ftruncate creates sparse extents without a multi-gigabyte payload.
// This test is confined to that substrate; Windows needs explicit sparse-file
// control before an equivalent fixture can safely claim bounded disk use.
func TestInspectNativeExtentCrossesEvery32BitConversionBoundary(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		extent int64
	}{
		{name: "signed ceiling remains exact", extent: math.MaxInt32},
		{name: "first unsigned signed-bit extent cannot become negative", extent: math.MaxInt32 + 1},
		{name: "one above signed transition retains its low bit", extent: math.MaxInt32 + 2},
		{name: "unsigned ceiling remains exact", extent: math.MaxUint32},
		{name: "first extent outside unsigned 32 bits cannot become zero", extent: math.MaxUint32 + 1},
		{name: "one above unsigned transition retains its high bit", extent: math.MaxUint32 + 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			name := filepath.Join(t.TempDir(), "extent")
			file, err := os.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := file.Close(); err != nil {
					t.Error(err)
				}
			})
			if err := file.Truncate(tc.extent); err != nil {
				t.Fatal(err)
			}
			before, err := file.Stat()
			if err != nil || before.Size() != tc.extent {
				t.Fatalf("native extent = (%v,%v), want %d", before, err, tc.extent)
			}
			path, err := core.ParseAbsolutePath(name)
			if err != nil {
				t.Fatal(err)
			}
			observation, err := filestore.Inspect(t.Context(), path)
			if err != nil {
				t.Fatal(err)
			}
			kind, kindErr := observation.Kind()
			size, sizeErr := observation.SizeBytes()
			if kindErr != nil || sizeErr != nil || kind != filestore.PathKindRegularFile || size.Uint64() != uint64(tc.extent) {
				t.Fatalf("observation = (%v,%v,%v,%v), want regular file with %d exact bytes", kind, size, kindErr, sizeErr, tc.extent)
			}
			after, err := file.Stat()
			if err != nil || !os.SameFile(before, after) || after.Size() != tc.extent || after.Mode() != before.Mode() || !after.ModTime().Equal(before.ModTime()) {
				t.Fatalf("after = (%v,%v), want unchanged native file", after, err)
			}
		})
	}
}
