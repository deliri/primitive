package filestore_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// This finite fixture crosses the former directory quota. Every native name
// must be delivered exactly once; fixture storage is owned by the test.
const formerDirectoryQuotaFixture uint32 = 1 << 16

func TestWalkFormerCardinalityBoundaryLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		count uint32
	}{
		{name: "one below former quota delivers every entry", count: formerDirectoryQuotaFixture - 1},
		{name: "former quota delivers its final entry", count: formerDirectoryQuotaFixture},
		{name: "one above former quota delivers every entry", count: formerDirectoryQuotaFixture + 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			for i := range tc.count {
				if err := os.WriteFile(filepath.Join(directory, fmt.Sprintf("entry-%05d", tc.count-1-i)), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			delivered := uint32(0)
			seen := make([]bool, tc.count)
			request := filestore.WalkRequest{Location: filestore.Location{Root: requireTestRoot(t, directory), Path: mustRelativePath(t, ".")}, Visit: func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
				index, parseErr := strconv.ParseUint(strings.TrimPrefix(entry.Entry.Name(), "entry-"), 10, 32)
				if err := entry.Validate(); err != nil {
					return filestore.WalkDirectiveUnknown, err
				}
				if parseErr != nil || index >= uint64(tc.count) || seen[index] || entry.Path.String() != entry.Entry.Name() || !entry.Entry.Type().IsRegular() {
					t.Errorf("entry %d = (%v,%v), want unique regular fixture entry", delivered, entry.Path, entry.Entry)
					return filestore.WalkDirectiveUnknown, core.ErrFilestoreContract
				}
				seen[index] = true
				delivered++
				return filestore.WalkContinue, nil
			}}
			gotErr := filestore.Walk(t.Context(), request)
			wantCount := tc.count
			if gotErr != nil || delivered != wantCount {
				t.Fatalf("Walk = (%d,%v), want (%d,nil)", delivered, gotErr, wantCount)
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != int(tc.count) {
				t.Fatalf("namespace cardinality = (%d,%v), want %d", len(entries), err, tc.count)
			}
			for i, entry := range entries {
				want := fmt.Sprintf("entry-%05d", i)
				info, err := entry.Info()
				if err != nil || entry.Name() != want || !info.Mode().IsRegular() || info.Size() != 0 || info.Mode().Perm() != 0o600 {
					t.Fatalf("retained entry %d = (%v,%v), want empty regular %q at 0600", i, info, err, want)
				}
			}
		})
	}
}
