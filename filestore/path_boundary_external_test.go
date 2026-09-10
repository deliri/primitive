package filestore_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/filestore"
)

func TestEnsureDirectoryAdmittedPathBoundaryMatrix(t *testing.T) {
	t.Parallel()

	depth255 := strings.Repeat("d"+string(filepath.Separator), 256-2) + "d"
	depth256 := strings.Repeat("d"+string(filepath.Separator), 256-1) + "d"
	cases := []struct {
		name string
		path string
	}{
		{
			name: "255 native directory components",
			path: depth255,
		},
		{
			name: "256 native directory components",
			path: depth256,
		},
		{name: "257 native directory components continue", path: depth256 + string(filepath.Separator) + "d"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rootDirectory := t.TempDir()
			root := requireTestRoot(t, rootDirectory)
			gotErr := filestore.EnsureDirectory(t.Context(), filestore.DirectoryRequest{
				Location: filestore.Location{Root: root, Path: mustRelativePath(t, tc.path)},
				Mode:     0o700,
			})
			if gotErr != nil {
				t.Fatalf("EnsureDirectory() error = %v, want nil", gotErr)
			}
			visited := 0
			leafSeen := false
			err := filestore.Walk(t.Context(), filestore.WalkRequest{
				Location: filestore.Location{Root: root, Path: mustRelativePath(t, ".")},
				Visit: func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
					visited++
					leafSeen = leafSeen || entry.Path.String() == tc.path
					return filestore.WalkContinue, nil
				},
			})
			wantVisited := strings.Count(tc.path, string(filepath.Separator)) + 1
			if err != nil || visited != wantVisited || !leafSeen {
				t.Fatalf("walk = %d entries, leaf=%t, error=%v; want %d, true, nil", visited, leafSeen, err, wantVisited)
			}
			info, err := os.Stat(filepath.Join(rootDirectory, tc.path))
			if err != nil {
				t.Fatal(err)
			}
			if !info.IsDir() || info.Mode().Perm() != 0o700 {
				t.Fatalf("directory = mode:%v permissions:%#o, want directory/%#o", info.Mode(), info.Mode().Perm(), os.FileMode(0o700))
			}
		})
	}
}

func deterministicPayload(size int) []byte {
	payload := make([]byte, size)
	for index := range payload {
		payload[index] = byte((index*131 + size) % 251)
	}
	return payload
}
