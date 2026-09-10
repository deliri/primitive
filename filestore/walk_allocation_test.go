package filestore

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// The batch size comes from the owning implementation constant. The public
// oracle is the exact set of native relative paths, including descent control.
func TestNativeWalkBatchAndSkipLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		count        int
		branch, skip bool
	}{
		{name: "empty directory emits no entry"},
		{name: "one below native batch retains every name", count: walkDirectoryBatchEntries - 1},
		{name: "exact native batch retains its final name", count: walkDirectoryBatchEntries},
		{name: "first entry beyond batch is not dropped", count: walkDirectoryBatchEntries + 1},
		{name: "one below second batch retains all names", count: 2*walkDirectoryBatchEntries - 1},
		{name: "exact second batch terminates without duplication", count: 2 * walkDirectoryBatchEntries},
		{name: "partial third batch retains its last entry", count: 2*walkDirectoryBatchEntries + 1},
		{name: "skip refuses descent while retaining the observed directory", count: 2*walkDirectoryBatchEntries + 1, branch: true, skip: true},
		{name: "continue delivers child once across native batches", count: 2*walkDirectoryBatchEntries + 1, branch: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			var want []string
			for i := range tc.count {
				name := fmt.Sprintf("entry-%03d", i)
				if err := os.WriteFile(filepath.Join(directory, name), nil, 0o600); err != nil {
					t.Fatal(err)
				}
				want = append(want, name)
			}
			if tc.branch {
				if err := os.Mkdir(filepath.Join(directory, "branch"), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(directory, "branch", "child"), nil, 0o600); err != nil {
					t.Fatal(err)
				}
				want = append(want, "branch")
				if !tc.skip {
					want = append(want, filepath.Join("branch", "child"))
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
			path, err := core.ParseRelativePath(".")
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			gotErr := Walk(t.Context(), WalkRequest{Location: Location{Root: root, Path: path}, Visit: func(entry WalkEntry) (WalkDirective, error) {
				if err := entry.Validate(); err != nil {
					return WalkDirectiveUnknown, err
				}
				got = append(got, entry.Path.String())
				if entry.Entry.IsDir() && tc.skip {
					return WalkSkipDirectory, nil
				}
				return WalkContinue, nil
			}})
			slices.Sort(got)
			slices.Sort(want)
			if gotErr != nil || !slices.Equal(got, want) {
				t.Fatalf("Walk = (%v,%v), want exact %v", got, gotErr, want)
			}
			entries, err := os.ReadDir(directory)
			wantCount := tc.count
			if tc.branch {
				wantCount++
			}
			if err != nil || len(entries) != wantCount {
				t.Fatalf("namespace = (%v,%v), want %d retained entries", entries, err, wantCount)
			}
			if tc.branch {
				info, err := root.Stat(filepath.Join("branch", "child"))
				if err != nil || !info.Mode().IsRegular() || info.Size() != 0 {
					t.Fatalf("retained child = (%v,%v), want original empty file", info, err)
				}
			}
		})
	}
}
