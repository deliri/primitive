package filestore_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// Other closed-root effects live in their full operation tables. These rows
// retain append/remove's native refusal, nil capability, and exact no-effect.
func TestClosedOSRootNativeErrorMatrix(t *testing.T) {
	t.Parallel()
	type door uint8
	const (
		appendCreate door = iota
		appendExisting
		remove
		removeTree
	)
	for _, tc := range []struct {
		name    string
		door    door
		wantErr error
	}{
		{name: "closed root cannot create append handle", door: appendCreate, wantErr: core.ErrFilestoreActivation},
		{name: "closed root cannot reopen append handle", door: appendExisting, wantErr: core.ErrFilestoreActivation},
		{name: "closed root cannot remove owned file", door: remove, wantErr: core.ErrFilestoreCleanup},
		{name: "closed root cannot remove owned directory tree", door: removeTree, wantErr: core.ErrFilestoreCleanup},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			root := requireTestRoot(t, directory)
			payload := []byte{0, 255, 1}
			if err := root.WriteFile("target", payload, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := root.Mkdir("tree", 0o700); err != nil {
				t.Fatal(err)
			}
			if err := root.WriteFile(filepath.Join("tree", "child"), payload, 0o600); err != nil {
				t.Fatal(err)
			}
			before, err := removalFixtureSnapshot(directory)
			if err != nil {
				t.Fatal(err)
			}
			infos := make([]fs.FileInfo, len(before))
			for i, entry := range before {
				infos[i], err = os.Lstat(filepath.Join(directory, entry.name))
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := root.Close(); err != nil {
				t.Fatal(err)
			}
			location := filestore.Location{Root: root, Path: mustRelativePath(t, "target")}
			var gotErr error
			switch tc.door {
			case appendCreate, appendExisting:
				intent := filestore.AppendCreate
				if tc.door == appendExisting {
					intent = filestore.AppendExisting
				}
				file, err := filestore.OpenAppend(t.Context(), filestore.AppendRequest{Location: location, Mode: 0o600, Append: intent})
				gotErr = err
				if file != nil {
					t.Cleanup(func() {
						if err := file.Close(); err != nil {
							t.Error(err)
						}
					})
					t.Fatalf("refused append handle = %v, want nil", file)
				}
			case remove:
				gotErr = filestore.Remove(t.Context(), filestore.RemovalRequest{Location: location})
			case removeTree:
				location.Path = mustRelativePath(t, "tree")
				gotErr = filestore.RemoveTree(t.Context(), filestore.TreeRemovalRequest{Location: location})
			default:
				t.Fatalf("door = %v, want table-owned operation", tc.door)
			}
			if !errors.Is(gotErr, tc.wantErr) || !errors.Is(gotErr, fs.ErrClosed) {
				t.Fatalf("closed effect = %v, want %v and %v", gotErr, tc.wantErr, fs.ErrClosed)
			}
			for _, class := range []error{core.ErrFilestoreContract, core.ErrFilestoreActivation, core.ErrFilestoreCleanup, core.ErrFilestoreConflict, core.ErrFilestoreActivationIndeterminate} {
				if errors.Is(gotErr, class) != errors.Is(tc.wantErr, class) {
					t.Fatalf("error = %v, want exactly %v", gotErr, tc.wantErr)
				}
			}
			after, err := removalFixtureSnapshot(directory)
			if err != nil || len(after) != len(before) {
				t.Fatalf("namespace = (%v,%v), want %v", after, err, before)
			}
			for i, entry := range before {
				info, err := os.Lstat(filepath.Join(directory, entry.name))
				if err != nil || !os.SameFile(infos[i], info) || !infos[i].ModTime().Equal(info.ModTime()) || after[i].name != entry.name || after[i].mode != entry.mode || !bytes.Equal(after[i].data, entry.data) {
					t.Fatalf("retained entry = (%v,%v), want %+v with original inode/time", after[i], err, entry)
				}
			}
		})
	}
}
