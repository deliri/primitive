package filestore_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// This materializes the public ceiling. Small-ceiling tables alone cannot
// detect uint16 narrowing or a full-limit off-by-one in the actual walker.
func TestLexicalWalkMaximumMaterializedBoundaryLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		count   uint32
		wantErr error
	}{
		{name: "one below largest admitted directory delivers every entry", count: filestore.DirectoryEntryMaximumLimit - 1},
		{name: "largest admitted directory delivers its final entry", count: filestore.DirectoryEntryMaximumLimit},
		{name: "one above largest directory delivers no prefix", count: filestore.DirectoryEntryMaximumLimit + 1, wantErr: core.ErrFilestoreContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			for i := range tc.count {
				if err := os.WriteFile(filepath.Join(directory, fmt.Sprintf("entry-%05d", tc.count-1-i)), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			maximum, err := filestore.NewDirectoryEntryMaximum(filestore.DirectoryEntryMaximumLimit)
			if err != nil {
				t.Fatal(err)
			}
			delivered := uint32(0)
			request := filestore.WalkRequest{Location: filestore.Location{Root: requireTestRoot(t, directory), Path: mustRelativePath(t, ".")}, Order: filestore.WalkOrderLexical, DirectoryEntryMaximum: maximum, Visit: func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
				want := fmt.Sprintf("entry-%05d", delivered)
				if err := entry.Validate(); err != nil {
					return filestore.WalkDirectiveUnknown, err
				}
				if entry.Path.String() != want || entry.Entry.Name() != want || !entry.Entry.Type().IsRegular() {
					t.Errorf("entry %d = (%v,%v), want regular %q", delivered, entry.Path, entry.Entry, want)
					return filestore.WalkDirectiveUnknown, core.ErrFilestoreContract
				}
				delivered++
				return filestore.WalkContinue, nil
			}}
			gotErr := filestore.Walk(t.Context(), request)
			wantCount := tc.count
			if tc.wantErr != nil {
				wantCount = 0
			}
			if !errors.Is(gotErr, tc.wantErr) || errors.Is(gotErr, core.ErrFilestoreSource) || delivered != wantCount {
				t.Fatalf("Walk = (%d,%v), want (%d,%v)", delivered, gotErr, wantCount, tc.wantErr)
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
