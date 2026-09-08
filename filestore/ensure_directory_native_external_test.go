//go:build darwin || linux

package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

type ensureDirectoryMutation uint8

const (
	ensureDirectoryUnchanged ensureDirectoryMutation = iota
	ensureDirectoryNilContext
	ensureDirectoryCanceled
	ensureDirectoryNilRoot
	ensureDirectoryClosedRoot
	ensureDirectoryZeroPath
	ensureDirectoryMovedRoot
)

func TestEnsureDirectoryNativeNamespaceLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, path string
		mode       fs.FileMode
		mutation   ensureDirectoryMutation
		created    []string
		changed    string
		wantErr    error
		native     bool
	}{
		{name: "one missing entry gets exact mode and no sibling", path: "new", mode: 0o750, created: []string{"new"}},
		{name: "new directory restores requested group and other write bits after native umask", path: "new", mode: 0o777, created: []string{"new"}},
		{name: "every new component receives exact permissions", path: filepath.Join("new", "leaf"), mode: 0o750, created: []string{"new", filepath.Join("new", "leaf")}},
		{name: "existing intermediate mode is not widened", path: filepath.Join("real", "new"), mode: 0o750, created: []string{filepath.Join("real", "new")}},
		{name: "existing final mode changes without losing child bytes", path: "real", mode: 0o750, changed: "real"},
		{name: "same final mode creates no namespace noise", path: "real", mode: 0o700, changed: "real"},
		{name: "confined ancestor link retains its spelling and target", path: filepath.Join("link", "new"), mode: 0o750, created: []string{filepath.Join("real", "new")}},
		{name: "confined final link changes only its actual directory", path: "link", mode: 0o750, changed: "real"},
		{name: "confined parent traversal remains owned by Go", path: filepath.Join("real", "back", "new"), mode: 0o750, created: []string{"new"}},
		{name: "held root survives replacement of its external name", path: "new", mode: 0o750, mutation: ensureDirectoryMovedRoot, created: []string{"new"}},
		{name: "space is a real path component not trimming policy", path: " ", mode: 0o750, created: []string{" "}},
		{name: "final regular file is not chmodded into a directory", path: "file", mode: 0o750, wantErr: core.ErrFilestoreActivation, native: true},
		{name: "intermediate file does not acquire a child", path: filepath.Join("file", "child"), mode: 0o750, wantErr: core.ErrFilestoreActivation, native: true},
		{name: "symlink to regular file keeps inode and bytes", path: "file-link", mode: 0o750, wantErr: core.ErrFilestoreActivation, native: true},
		{name: "dangling final link must not create its referent", path: "dangling", mode: 0o750, wantErr: core.ErrFilestoreActivation, native: true},
		{name: "dangling ancestor link must not grow a foreign chain", path: filepath.Join("dangling", "child"), mode: 0o750, wantErr: core.ErrFilestoreActivation, native: true},
		{name: "escaping final link preserves the outside directory", path: "outside", mode: 0o750, wantErr: core.ErrFilestoreActivation, native: true},
		{name: "escaping ancestor link cannot add outside child", path: filepath.Join("outside", "new"), mode: 0o750, wantErr: core.ErrFilestoreActivation, native: true},
		{name: "looping link terminates with the native cause", path: "loop", mode: 0o750, wantErr: core.ErrFilestoreActivation, native: true},
		{name: "nil context cannot create a chain", path: "new", mode: 0o750, mutation: ensureDirectoryNilContext, wantErr: core.ErrNilContext},
		{name: "cancellation precedes namespace effects", path: "new", mode: 0o750, mutation: ensureDirectoryCanceled, wantErr: context.Canceled},
		{name: "cancellation prevents existing directory mode changes", path: "real", mode: 0o750, mutation: ensureDirectoryCanceled, wantErr: context.Canceled},
		{name: "nil root cannot imply ambient filesystem access", path: "new", mode: 0o750, mutation: ensureDirectoryNilRoot, wantErr: core.ErrFilestoreContract},
		{name: "closed root retains native closure identity", path: "new", mode: 0o750, mutation: ensureDirectoryClosedRoot, wantErr: core.ErrFilestoreActivation, native: true},
		{name: "zero path cannot select the root", mode: 0o750, mutation: ensureDirectoryZeroPath, wantErr: core.ErrFilestoreContract},
		{name: "explicit root path cannot change the root mode", path: ".", mode: 0o750, wantErr: core.ErrFilestoreContract},
		{name: "zero permission request cannot create a partial entry", path: "new", wantErr: core.ErrFilestoreContract},
		{name: "file type bits cannot masquerade as permissions", path: "new", mode: fs.ModeDir | 0o750, wantErr: core.ErrFilestoreContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			container := t.TempDir()
			directory := filepath.Join(container, "root")
			if err := os.Mkdir(directory, 0o700); err != nil {
				t.Fatal(err)
			}
			outside := t.TempDir()
			payload := []byte{0, 255, 1, 0}
			for _, dir := range []string{filepath.Join(directory, "real")} {
				if err := os.Mkdir(dir, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			for _, name := range []string{filepath.Join(directory, "neighbor"), filepath.Join(directory, "file"), filepath.Join(directory, "real", "child"), filepath.Join(outside, "retained")} {
				if err := os.WriteFile(name, payload, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			for _, link := range []struct{ name, target string }{{"link", "real"}, {"file-link", "file"}, {"dangling", "missing"}, {"outside", outside}, {"loop", "loop"}, {filepath.Join("real", "back"), ".."}} {
				if err := os.Symlink(link.target, filepath.Join(directory, link.name)); err != nil {
					t.Fatal(err)
				}
			}
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			if tc.mutation != ensureDirectoryClosedRoot {
				t.Cleanup(func() {
					if err := root.Close(); err != nil {
						t.Error(err)
					}
				})
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
			outsideInfo, err := os.Stat(outside)
			if err != nil {
				t.Fatal(err)
			}
			outsideFileInfo, err := os.Stat(filepath.Join(outside, "retained"))
			if err != nil {
				t.Fatal(err)
			}
			path := core.RelativePath{}
			if tc.mutation != ensureDirectoryZeroPath {
				path = mustRelativePath(t, tc.path)
			}
			request := filestore.DirectoryRequest{Location: filestore.Location{Root: root, Path: path}, Mode: tc.mode}
			ctx := t.Context()
			switch tc.mutation {
			case ensureDirectoryNilContext:
				ctx = nil
			case ensureDirectoryCanceled:
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case ensureDirectoryNilRoot:
				request.Location.Root = nil
			case ensureDirectoryClosedRoot:
				if err := root.Close(); err != nil {
					t.Fatal(err)
				}
			case ensureDirectoryMovedRoot:
				moved := filepath.Join(container, "moved")
				if err := os.Rename(directory, moved); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(directory, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(directory, "foreign"), payload, 0o600); err != nil {
					t.Fatal(err)
				}
				directory = moved
			}
			var wantNative error
			if tc.native {
				native, nativeErr := root.OpenFile(path.String(), os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NONBLOCK, 0)
				if native != nil {
					if err := native.Close(); err != nil {
						t.Fatal(err)
					}
				}
				var pathErr *fs.PathError
				if !errors.As(nativeErr, &pathErr) {
					t.Fatalf("native refusal = %v, want PathError", nativeErr)
				}
				wantNative = pathErr.Err
			}
			gotErr := filestore.EnsureDirectory(ctx, request)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("ensure = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.native {
				var pathErr *fs.PathError
				if !errors.Is(gotErr, wantNative) || !errors.As(gotErr, &pathErr) {
					t.Fatalf("native cause = %v, want %v and PathError", gotErr, wantNative)
				}
			}
			want := slices.Clone(before)
			for i := range want {
				if want[i].name == tc.changed {
					want[i].mode = fs.ModeDir | tc.mode
				}
			}
			for _, name := range tc.created {
				want = append(want, removalFixtureEntry{name: name, mode: fs.ModeDir | tc.mode})
			}
			slices.SortFunc(want, func(a, b removalFixtureEntry) int { return strings.Compare(a.name, b.name) })
			got, err := removalFixtureSnapshot(directory)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(want) {
				t.Fatalf("namespace = %+v, want %+v", got, want)
			}
			for i := range want {
				if got[i].name != want[i].name || got[i].mode != want[i].mode || got[i].target != want[i].target || !bytes.Equal(got[i].data, want[i].data) {
					t.Fatalf("entry %d = %+v, want %+v", i, got[i], want[i])
				}
			}
			for i, entry := range before {
				after, err := os.Lstat(filepath.Join(directory, entry.name))
				if err != nil {
					t.Fatal(err)
				}
				changedChildren := false
				for _, created := range tc.created {
					if filepath.Dir(created) == entry.name {
						changedChildren = true
					}
				}
				if !os.SameFile(infos[i], after) || (!changedChildren && infos[i].ModTime().UnixNano() != after.ModTime().UnixNano()) {
					t.Fatalf("retained inode/time %s = %v, want %v", entry.name, after, infos[i])
				}
			}
			outsideAfter, err := os.Stat(outside)
			if err != nil {
				t.Fatal(err)
			}
			outsideFileAfter, err := os.Stat(filepath.Join(outside, "retained"))
			if err != nil {
				t.Fatal(err)
			}
			outsideEntries, err := os.ReadDir(outside)
			if err != nil {
				t.Fatal(err)
			}
			outsideBytes, err := os.ReadFile(filepath.Join(outside, "retained"))
			if err != nil || !bytes.Equal(outsideBytes, payload) || len(outsideEntries) != 1 || outsideEntries[0].Name() != "retained" || !os.SameFile(outsideInfo, outsideAfter) || outsideInfo.Mode() != outsideAfter.Mode() || outsideInfo.ModTime().UnixNano() != outsideAfter.ModTime().UnixNano() || !os.SameFile(outsideFileInfo, outsideFileAfter) || outsideFileInfo.Mode() != outsideFileAfter.Mode() || outsideFileInfo.ModTime().UnixNano() != outsideFileAfter.ModTime().UnixNano() {
				t.Fatalf("outside bytes/metadata/namespace changed: %v", err)
			}
			if tc.mutation == ensureDirectoryMovedRoot {
				foreign, err := removalFixtureSnapshot(filepath.Join(container, "root"))
				if err != nil || len(foreign) != 1 || foreign[0].name != "foreign" || foreign[0].mode != 0o600 || !bytes.Equal(foreign[0].data, payload) {
					t.Fatalf("replacement root = (%+v,%v), want untouched foreign file", foreign, err)
				}
			}
		})
	}
}
