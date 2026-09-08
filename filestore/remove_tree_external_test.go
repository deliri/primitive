package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestRemoveTreeNamespaceLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		kind     filestore.PathKind
		nested   bool
		dangling bool
		rootPath bool
		canceled bool
		closed   bool
		wantErr  error
	}{
		{name: "binary regular file is removed without touching neighbor", kind: filestore.PathKindRegularFile},
		{name: "empty directory is removed without invented descendants", kind: filestore.PathKindDirectory},
		{name: "nested tree is removed without following its outside link", kind: filestore.PathKindDirectory, nested: true},
		{name: "final directory link is removed without traversing its target", kind: filestore.PathKindSymbolicLink},
		{name: "dangling final link is removed as an occupied entry", kind: filestore.PathKindSymbolicLink, dangling: true},
		{name: "absent entry under a real parent remains a quiet no-op", kind: filestore.PathKindAbsent},
		{name: "absent parent cannot turn a removal no-op into failure", kind: filestore.PathKindUnreachable},
		{name: "root-naming request cannot erase the capability", kind: filestore.PathKindDirectory, nested: true, rootPath: true, wantErr: core.ErrFilestoreContract},
		{name: "canceled removal preserves every nested byte", kind: filestore.PathKindDirectory, nested: true, canceled: true, wantErr: context.Canceled},
		{name: "closed root cannot be mistaken for absent storage", kind: filestore.PathKindDirectory, nested: true, closed: true, wantErr: core.ErrFilestoreCleanup},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			outside := t.TempDir()
			payload := []byte{0, 255, 7, 31}
			if err := os.WriteFile(filepath.Join(outside, "evidence"), payload, 0o600); err != nil {
				t.Fatal(err)
			}
			outsideBefore, err := os.Stat(filepath.Join(outside, "evidence"))
			if err != nil {
				t.Fatal(err)
			}
			directories := [2]string{t.TempDir(), t.TempDir()}
			var roots [2]*os.Root
			pathText := "tree"
			if tc.kind == filestore.PathKindUnreachable {
				pathText = filepath.Join("missing", "tree")
			}
			if tc.rootPath {
				pathText = "."
			}
			for i, directory := range directories {
				root, err := os.OpenRoot(directory)
				if err != nil {
					t.Fatal(err)
				}
				roots[i] = root
				t.Cleanup(func() {
					if err := root.Close(); err != nil && !errors.Is(err, fs.ErrClosed) {
						t.Error(err)
					}
				})
				if err := os.WriteFile(filepath.Join(directory, "neighbor"), payload, 0o600); err != nil {
					t.Fatal(err)
				}
				tree := filepath.Join(directory, "tree")
				switch tc.kind {
				case filestore.PathKindRegularFile:
					if err := os.WriteFile(tree, payload, 0o600); err != nil {
						t.Fatal(err)
					}
				case filestore.PathKindDirectory:
					if err := os.Mkdir(tree, 0o700); err != nil {
						t.Fatal(err)
					}
					if tc.nested {
						if err := os.Mkdir(filepath.Join(tree, "nested"), 0o700); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(filepath.Join(tree, "nested", "bytes"), payload, 0o600); err != nil {
							t.Fatal(err)
						}
						if err := os.Symlink(outside, filepath.Join(tree, "outside")); err != nil {
							t.Fatal(err)
						}
					}
				case filestore.PathKindSymbolicLink:
					target := outside
					if tc.dangling {
						target = filepath.Join(outside, "missing")
					}
					if err := os.Symlink(target, tree); err != nil {
						t.Fatal(err)
					}
				}
			}
			path := mustRelativePath(t, pathText)
			request := filestore.TreeRemovalRequest{Location: filestore.Location{Root: roots[0], Path: path}}
			ctx := t.Context()
			if tc.canceled {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			if tc.closed {
				for _, root := range roots {
					if err := root.Close(); err != nil {
						t.Fatal(err)
					}
				}
			}
			var nativeErr error
			if !tc.rootPath && !tc.canceled {
				nativeErr = roots[1].RemoveAll(path.String())
			}
			gotErr := filestore.RemoveTree(ctx, request)
			if (gotErr == nil) != (tc.wantErr == nil) || tc.wantErr != nil && !errors.Is(gotErr, tc.wantErr) {
				t.Errorf("RemoveTree = %v, want %v", gotErr, tc.wantErr)
			}
			if nativeErr != nil {
				var nativePath *fs.PathError
				if !errors.As(nativeErr, &nativePath) {
					t.Fatal(nativeErr)
				}
				var gotPath *fs.PathError
				if !errors.As(gotErr, &gotPath) || !errors.Is(gotErr, nativePath.Err) {
					t.Errorf("native refusal = %v, want Go cause %v", gotErr, nativePath.Err)
				}
			} else if !tc.rootPath && !tc.canceled && gotErr != nil {
				t.Errorf("Primitive result = %v, want Go RemoveAll no-error outcome", gotErr)
			}
			wantEntries := 1
			if tc.wantErr != nil {
				wantEntries++
			}
			entries, err := os.ReadDir(directories[0])
			if err != nil || len(entries) != wantEntries {
				t.Errorf("namespace = (%v,%v), want %d retained entries", entries, err, wantEntries)
			}
			if tc.wantErr == nil {
				if _, err := os.Lstat(filepath.Join(directories[0], path.String())); !errors.Is(err, fs.ErrNotExist) {
					t.Errorf("removed path = %v, want absent", err)
				}
				if err := filestore.RemoveTree(ctx, request); err != nil {
					t.Errorf("repeated removal = %v, want quiet no-op", err)
				}
			} else {
				got, err := os.ReadFile(filepath.Join(directories[0], "tree", "nested", "bytes"))
				if err != nil || !bytes.Equal(got, payload) {
					t.Errorf("refused nested bytes = (%v,%v), want %v", got, err, payload)
				}
			}
			gotNeighbor, err := os.ReadFile(filepath.Join(directories[0], "neighbor"))
			if err != nil || !bytes.Equal(gotNeighbor, payload) {
				t.Errorf("neighbor = (%v,%v), want %v", gotNeighbor, err, payload)
			}
			gotOutside, err := os.ReadFile(filepath.Join(outside, "evidence"))
			if err != nil || !bytes.Equal(gotOutside, payload) {
				t.Errorf("outside bytes = (%v,%v), want %v", gotOutside, err, payload)
			}
			outsideAfter, err := os.Stat(filepath.Join(outside, "evidence"))
			if err != nil {
				t.Fatal(err)
			}
			if !os.SameFile(outsideBefore, outsideAfter) || outsideBefore.Mode() != outsideAfter.Mode() || outsideBefore.ModTime().UnixNano() != outsideAfter.ModTime().UnixNano() {
				t.Errorf("outside identity = %v, want retained %v", outsideAfter, outsideBefore)
			}
		})
	}
}
