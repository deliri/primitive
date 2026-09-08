//go:build darwin || linux

package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

type ensureDirectoryLeaf uint8

const (
	ensureDirectoryLeafMissing ensureDirectoryLeaf = iota
	ensureDirectoryLeafExisting
	ensureDirectoryLeafFile
	ensureDirectoryLeafConfinedLink
	ensureDirectoryLeafDanglingLink
	ensureDirectoryLeafOutsideLink
	ensureDirectoryLeafLoop
	ensureDirectoryLeafLimit
)

func FuzzEnsureDirectoryNativeNamespaceCustody(f *testing.F) {
	root, err := os.OpenRoot(f.TempDir())
	if err != nil {
		f.Fatal(err)
	}
	f.Cleanup(func() {
		if err := root.Close(); err != nil {
			f.Error(err)
		}
	})
	path, err := core.ParseRelativePath("target")
	if err != nil {
		f.Fatal(err)
	}
	seed := filestore.DirectoryRequest{Location: filestore.Location{Root: root, Path: path}, Mode: 0o750}
	if err := seed.Validate(); err != nil {
		f.Fatal(err)
	}
	if err := filestore.EnsureDirectory(f.Context(), seed); err != nil {
		f.Fatal(err)
	}
	observed, err := root.Stat(path.String())
	if err != nil || !observed.IsDir() {
		f.Fatalf("seed directory = (%v,%v), want native directory", observed, err)
	}
	emitted, err := filestore.FilestoreRoundTripSeedForTest(f.Context(), f.TempDir())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(uint8(0), uint8(0), uint8(ensureDirectoryLeafMissing), uint16(observed.Mode().Perm()), emitted, false)
	for leaf := range ensureDirectoryLeafLimit {
		f.Add(uint8(2), uint8(1), uint8(leaf), uint16(seed.Mode), []byte{0, 255}, false)
	}
	f.Add(uint8(0), uint8(0), uint8(ensureDirectoryLeafMissing), uint16(seed.Mode), []byte{}, false)
	f.Add(uint8(3), uint8(0), uint8(ensureDirectoryLeafMissing), uint16(seed.Mode), []byte{0, 255}, false)
	f.Add(uint8(2), uint8(1), uint8(ensureDirectoryLeafMissing), uint16(seed.Mode), []byte{0, 255}, true)
	f.Add(uint8(2), uint8(1), uint8(ensureDirectoryLeafExisting), uint16(seed.Mode), []byte{0, 255}, true)
	f.Add(uint8(2), uint8(1), uint8(ensureDirectoryLeafMissing), uint16(0), []byte{0, 255}, false)
	f.Fuzz(func(t *testing.T, rawDepth, rawExisting, rawLeaf uint8, rawMode uint16, payload []byte, canceled bool) {
		depth := 1 + int(rawDepth%4)
		existing := int(rawExisting) % depth
		leaf := ensureDirectoryLeaf(rawLeaf % uint8(ensureDirectoryLeafLimit))
		if leaf != ensureDirectoryLeafMissing {
			existing = depth - 1
		}
		payload = payload[:min(len(payload), 1024)]
		mode := fs.FileMode(0o700) | (fs.FileMode(rawMode) & 0o077)
		if rawMode == 0 {
			mode = 0
		}
		parts := make([]string, depth)
		paths := make([]string, depth)
		for i := range parts {
			parts[i] = fmt.Sprintf("entry-%d", i)
			paths[i] = filepath.Join(parts[:i+1]...)
		}
		outside := t.TempDir()
		outsideName := filepath.Join(outside, "retained")
		if err := os.WriteFile(outsideName, payload, 0o600); err != nil {
			t.Fatal(err)
		}
		outsideBefore, err := os.Stat(outside)
		if err != nil {
			t.Fatal(err)
		}
		fileBefore, err := os.Stat(outsideName)
		if err != nil {
			t.Fatal(err)
		}
		directories := [2]string{t.TempDir(), t.TempDir()}
		var roots [2]*os.Root
		for i, directory := range directories {
			roots[i], err = os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := roots[i].Close(); err != nil {
					t.Error(err)
				}
			})
			if err := os.WriteFile(filepath.Join(directory, "neighbor"), payload, 0o600); err != nil {
				t.Fatal(err)
			}
			for _, name := range paths[:existing] {
				if err := os.Mkdir(filepath.Join(directory, name), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			target := filepath.Join(directory, paths[depth-1])
			switch leaf {
			case ensureDirectoryLeafExisting:
				if err := os.Mkdir(target, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(target, "child"), payload, 0o600); err != nil {
					t.Fatal(err)
				}
			case ensureDirectoryLeafFile:
				if err := os.WriteFile(target, payload, 0o600); err != nil {
					t.Fatal(err)
				}
			case ensureDirectoryLeafConfinedLink:
				referent := filepath.Join(filepath.Dir(target), "referent")
				if err := os.Mkdir(referent, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(referent, "child"), payload, 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("referent", target); err != nil {
					t.Fatal(err)
				}
			case ensureDirectoryLeafDanglingLink:
				if err := os.Symlink("missing", target); err != nil {
					t.Fatal(err)
				}
			case ensureDirectoryLeafOutsideLink:
				if err := os.Symlink(outside, target); err != nil {
					t.Fatal(err)
				}
			case ensureDirectoryLeafLoop:
				if err := os.Symlink(parts[depth-1], target); err != nil {
					t.Fatal(err)
				}
			}
		}
		before, err := removalFixtureSnapshot(directories[0])
		if err != nil {
			t.Fatal(err)
		}
		infos := make([]fs.FileInfo, len(before))
		for i, entry := range before {
			infos[i], err = os.Lstat(filepath.Join(directories[0], entry.name))
			if err != nil {
				t.Fatal(err)
			}
		}
		path := mustRelativePath(t, paths[depth-1])
		request := filestore.DirectoryRequest{Location: filestore.Location{Root: roots[0], Path: path}, Mode: mode}
		ctx := t.Context()
		var wantBoundary, wantNative error
		switch {
		case canceled:
			var cancel context.CancelFunc
			ctx, cancel = context.WithCancel(ctx)
			cancel()
			wantBoundary = context.Canceled
		case mode == 0:
			wantBoundary = core.ErrFilestoreContract
		case leaf >= ensureDirectoryLeafFile && leaf != ensureDirectoryLeafConfinedLink:
			native, nativeErr := roots[1].OpenFile(path.String(), os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NONBLOCK, 0)
			if native != nil {
				if err := native.Close(); err != nil {
					t.Fatal(err)
				}
			}
			var nativePath *fs.PathError
			if !errors.As(nativeErr, &nativePath) {
				t.Fatalf("native hostile fixture = %v, want PathError", nativeErr)
			}
			wantBoundary = core.ErrFilestoreActivation
			wantNative = nativePath.Err
		default:
			if err := roots[1].MkdirAll(path.String(), mode); err != nil {
				t.Fatal(err)
			}
			if leaf == ensureDirectoryLeafMissing {
				for _, name := range paths[existing:] {
					if err := roots[1].Chmod(name, mode); err != nil {
						t.Fatal(err)
					}
				}
			} else {
				if err := roots[1].Chmod(path.String(), mode); err != nil {
					t.Fatal(err)
				}
			}
		}
		gotErr := filestore.EnsureDirectory(ctx, request)
		if !errors.Is(gotErr, wantBoundary) {
			t.Fatalf("ensure = %v, want %v", gotErr, wantBoundary)
		}
		if wantNative != nil {
			var gotPath *fs.PathError
			if !errors.Is(gotErr, wantNative) || !errors.As(gotErr, &gotPath) {
				t.Fatalf("cause = %v, want %v and PathError", gotErr, wantNative)
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
			t.Fatalf("namespace = %+v, want Go %+v", got, want)
		}
		for i := range want {
			if got[i].name != want[i].name || got[i].mode != want[i].mode || got[i].target != want[i].target || !bytes.Equal(got[i].data, want[i].data) {
				t.Fatalf("entry %d = %+v, want Go %+v", i, got[i], want[i])
			}
		}
		for i, entry := range before {
			after, err := os.Lstat(filepath.Join(directories[0], entry.name))
			if err != nil {
				t.Fatal(err)
			}
			changedChildren := wantBoundary == nil && leaf == ensureDirectoryLeafMissing && existing > 0 && entry.name == paths[existing-1]
			if !os.SameFile(infos[i], after) || (!changedChildren && infos[i].ModTime().UnixNano() != after.ModTime().UnixNano()) {
				t.Fatalf("retained inode/time %s = %v, want %v", entry.name, after, infos[i])
			}
		}
		outsideAfter, err := os.Stat(outside)
		if err != nil {
			t.Fatal(err)
		}
		fileAfter, err := os.Stat(outsideName)
		if err != nil {
			t.Fatal(err)
		}
		outsideEntries, err := os.ReadDir(outside)
		if err != nil {
			t.Fatal(err)
		}
		outsideBytes, err := os.ReadFile(outsideName)
		if err != nil || !bytes.Equal(outsideBytes, payload) || len(outsideEntries) != 1 || outsideEntries[0].Name() != "retained" || !os.SameFile(outsideBefore, outsideAfter) || outsideBefore.Mode() != outsideAfter.Mode() || outsideBefore.ModTime().UnixNano() != outsideAfter.ModTime().UnixNano() || !os.SameFile(fileBefore, fileAfter) || fileBefore.Mode() != fileAfter.Mode() || fileBefore.ModTime().UnixNano() != fileAfter.ModTime().UnixNano() {
			t.Fatalf("outside namespace, inode, metadata or bytes changed: %v", err)
		}
	})
}
