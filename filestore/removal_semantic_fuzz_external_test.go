package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

type removalFixtureEntry struct {
	name   string
	mode   fs.FileMode
	data   []byte
	target string
}

// This oracle walks only the bounded test fixture, never caller datasets.
// Go's independent filesystem walk observes every retained name and byte;
// comparisons stay in the fuzz callback.
func removalFixtureSnapshot(directory string) ([]removalFixtureEntry, error) {
	var entries []removalFixtureEntry
	err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == directory {
			return nil
		}
		name, err := filepath.Rel(directory, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		observation := removalFixtureEntry{name: name, mode: info.Mode()}
		if info.Mode().IsRegular() {
			observation.data, err = os.ReadFile(path)
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			observation.target, err = os.Readlink(path)
		}
		if err != nil {
			return err
		}
		entries = append(entries, observation)
		return nil
	})
	return entries, err
}

func FuzzRemovalNativeNamespaceSemanticClosure(f *testing.F) {
	emitted, err := filestore.FilestoreRoundTripSeedForTest(f.Context(), f.TempDir())
	if err != nil {
		f.Fatal(err)
	}

	kinds := []filestore.PathKind{filestore.PathKindRegularFile, filestore.PathKindDirectory, filestore.PathKindSymbolicLink, filestore.PathKindAbsent, filestore.PathKindUnreachable}
	f.Add(uint8(slices.Index(kinds, filestore.PathKindRegularFile)), emitted, uint8(0), false, false)
	for _, kind := range kinds {
		for _, tree := range []bool{false, true} {
			f.Add(uint8(slices.Index(kinds, kind)), []byte{0, 255}, uint8(2), tree, false)
		}
	}
	f.Add(uint8(slices.Index(kinds, filestore.PathKindDirectory)), []byte{}, uint8(0), true, false)
	f.Add(uint8(slices.Index(kinds, filestore.PathKindDirectory)), []byte{0, 255}, uint8(4), true, true)
	f.Fuzz(func(t *testing.T, rawKind uint8, payload []byte, rawChildren uint8, tree, canceled bool) {
		kind := kinds[int(rawKind)%len(kinds)]
		payload = payload[:min(len(payload), 2048)]
		children := int(rawChildren % 5)
		outside := t.TempDir()
		if err := os.WriteFile(filepath.Join(outside, "retained"), payload, 0o600); err != nil {
			t.Fatal(err)
		}
		outsideBefore, err := os.Lstat(filepath.Join(outside, "retained"))
		if err != nil {
			t.Fatal(err)
		}
		directories := [2]string{t.TempDir(), t.TempDir()}
		var roots [2]*os.Root
		pathText := "target"
		if kind == filestore.PathKindUnreachable {
			pathText = filepath.Join("missing", "target")
		}
		for i, directory := range directories {
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			roots[i] = root
			t.Cleanup(func() {
				if err := root.Close(); err != nil {
					t.Error(err)
				}
			})
			if err := os.WriteFile(filepath.Join(directory, "neighbor"), payload, 0o600); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(directory, "target")
			switch kind {
			case filestore.PathKindRegularFile:
				if err := os.WriteFile(target, payload, 0o600); err != nil {
					t.Fatal(err)
				}
			case filestore.PathKindDirectory:
				if err := os.Mkdir(target, 0o700); err != nil {
					t.Fatal(err)
				}
				for index := range children {
					if err := os.WriteFile(filepath.Join(target, fmt.Sprintf("child-%d", index)), payload, 0o600); err != nil {
						t.Fatal(err)
					}
				}
				if children > 0 {
					if err := os.Symlink(outside, filepath.Join(target, "outside")); err != nil {
						t.Fatal(err)
					}
				}
			case filestore.PathKindSymbolicLink:
				if err := os.Symlink(outside, target); err != nil {
					t.Fatal(err)
				}
			}
		}
		path := mustRelativePath(t, pathText)
		ctx := t.Context()
		var wantErr error
		if canceled {
			var cancel context.CancelFunc
			ctx, cancel = context.WithCancel(ctx)
			cancel()
			wantErr = context.Canceled
		} else if tree {
			wantErr = roots[1].RemoveAll(path.String())
		} else {
			wantErr = roots[1].Remove(path.String())
		}
		if errors.Is(wantErr, fs.ErrNotExist) {
			wantErr = nil
		}
		var gotErr error
		location := filestore.Location{Root: roots[0], Path: path}
		if tree {
			gotErr = filestore.RemoveTree(ctx, filestore.TreeRemovalRequest{Location: location})
		} else {
			gotErr = filestore.Remove(ctx, filestore.RemovalRequest{Location: location})
		}
		if (gotErr == nil) != (wantErr == nil) {
			t.Fatalf("removal = %v, want Go native result %v", gotErr, wantErr)
		}
		if wantErr != nil {
			if canceled {
				if !errors.Is(gotErr, context.Canceled) {
					t.Fatalf("canceled removal = %v, want canceled", gotErr)
				}
			} else {
				var nativePath, gotPath *fs.PathError
				if !errors.As(wantErr, &nativePath) || !errors.As(gotErr, &gotPath) || !errors.Is(gotErr, nativePath.Err) || !errors.Is(gotErr, core.ErrFilestoreCleanup) {
					t.Fatalf("removal cause = %v, want cleanup with Go native cause %v", gotErr, wantErr)
				}
			}
		}
		got, err := removalFixtureSnapshot(directories[0])
		if err != nil {
			t.Fatal(err)
		}
		want, err := removalFixtureSnapshot(directories[1])
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(want) {
			t.Fatalf("retained namespace = %+v, want native %+v", got, want)
		}
		for i := range want {
			if got[i].name != want[i].name || got[i].mode != want[i].mode || got[i].target != want[i].target || !bytes.Equal(got[i].data, want[i].data) {
				t.Fatalf("entry %d = %+v, want native %+v", i, got[i], want[i])
			}
		}
		gotOutside, err := os.ReadFile(filepath.Join(outside, "retained"))
		if err != nil || !bytes.Equal(gotOutside, payload) {
			t.Fatalf("outside bytes = (%v,%v), want %v", gotOutside, err, payload)
		}
		outsideAfter, err := os.Lstat(filepath.Join(outside, "retained"))
		if err != nil {
			t.Fatal(err)
		}
		if !os.SameFile(outsideBefore, outsideAfter) || outsideBefore.Mode() != outsideAfter.Mode() || outsideBefore.ModTime().UnixNano() != outsideAfter.ModTime().UnixNano() {
			t.Fatalf("outside metadata = %v, want unchanged %v", outsideAfter, outsideBefore)
		}
	})
}
