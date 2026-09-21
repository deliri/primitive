package filestore_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestCreateDirectoryNativeOwnershipLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr    error
		wantNative error
		name       string
		fault      scratchFault
	}{
		{name: "absent leaf creates exactly one owned directory"},
		{name: "existing directory and child survive exclusive refusal", fault: scratchExistingDirectory, wantErr: core.ErrFilestoreConflict, wantNative: fs.ErrExist},
		{name: "existing regular file survives exclusive refusal", fault: scratchExistingFile, wantErr: core.ErrFilestoreConflict, wantNative: fs.ErrExist},
		{name: "existing symbolic link is never adopted", fault: scratchExistingLink, wantErr: core.ErrFilestoreConflict, wantNative: fs.ErrExist},
		{name: "file parent remains a native not-directory refusal", fault: scratchFileParent, wantErr: core.ErrFilestoreActivation},
		{name: "outside ancestor cannot create outside root", fault: scratchEscapeLink, wantErr: core.ErrFilestoreActivation},
		{name: "cancelled creation creates no entry", fault: scratchCancelled, wantErr: context.Canceled},
		{name: "missing context creates no entry", fault: scratchNilContext, wantErr: core.ErrNilContext},
		{name: "closed root cannot create an entry", fault: scratchClosedRoot, wantErr: core.ErrFilestoreActivation},
		{name: "absent root cannot substitute cwd", fault: scratchNilRoot, wantErr: core.ErrFilestoreContract},
		{name: "root itself cannot be created or chmodded", fault: scratchRootPath, wantErr: core.ErrFilestoreContract},
		{name: "unset path cannot select a directory", fault: scratchUnsetPath, wantErr: core.ErrFilestoreContract},
		{name: "zero mode is refused before effect", fault: scratchZeroMode, wantErr: core.ErrFilestoreContract},
		{name: "type bits are refused before effect", fault: scratchTypeMode, wantErr: core.ErrFilestoreContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			outside := t.TempDir()
			root := requireTestRoot(t, directory)
			path := filepath.Join(directory, "entry")
			request := filestore.DirectoryRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, "entry")}, Mode: 0700}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			switch tc.fault {
			case scratchNoFault:
			case scratchExistingDirectory:
				if err := os.Mkdir(path, 0750); err != nil {
					t.Fatalf("existing directory = %v, want nil", err)
				}
				if err := os.WriteFile(filepath.Join(path, "child"), []byte("original"), 0600); err != nil {
					t.Fatalf("existing child = %v, want nil", err)
				}
			case scratchExistingFile, scratchFileParent:
				if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
					t.Fatalf("existing file = %v, want nil", err)
				}
				if tc.fault == scratchFileParent {
					request.Location.Path = mustRelativePath(t, "entry/child")
				}
			case scratchExistingLink, scratchEscapeLink:
				if err := os.Symlink(outside, path); err != nil {
					t.Fatalf("existing link = %v, want nil", err)
				}
				if tc.fault == scratchEscapeLink {
					request.Location.Path = mustRelativePath(t, "entry/child")
				}
			case scratchCancelled:
				cancel()
			case scratchNilContext:
				ctx = nil
			case scratchClosedRoot:
				if err := root.Close(); err != nil {
					t.Fatalf("close root = %v, want nil", err)
				}
			case scratchNilRoot:
				request.Location.Root = nil
			case scratchRootPath:
				request.Location.Path = mustRelativePath(t, ".")
			case scratchUnsetPath:
				request.Location.Path = core.RelativePath{}
			case scratchZeroMode:
				request.Mode = 0
			case scratchTypeMode:
				request.Mode |= fs.ModeSymlink
			default:
				t.Fatalf("fixture fault = %d, want handled domain", tc.fault)
			}
			before, beforeErr := os.Lstat(path)
			gotErr := filestore.CreateDirectory(ctx, request)
			if !errors.Is(gotErr, tc.wantErr) || tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) {
				t.Fatalf("CreateDirectory = %v, want %v/native %v", gotErr, tc.wantErr, tc.wantNative)
			}
			after, afterErr := os.Lstat(path)
			if tc.wantErr == nil {
				if afterErr != nil || !after.IsDir() || after.Mode().Perm() != request.Mode {
					t.Fatalf("created directory = %v/%v, want mode %v", after, afterErr, request.Mode)
				}
				if err := filestore.CreateDirectory(t.Context(), request); !errors.Is(err, core.ErrFilestoreConflict) || !errors.Is(err, fs.ErrExist) {
					t.Fatalf("repeated exclusive create = %v, want conflict/exist", err)
				}
			} else if beforeErr == nil {
				if afterErr != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() {
					t.Fatalf("refused entry = %v/%v, want original inode and mode %v", after, afterErr, before)
				}
			} else if !errors.Is(afterErr, fs.ErrNotExist) {
				t.Fatalf("refused absent entry = %v, want absent", afterErr)
			}
			if tc.fault == scratchExistingDirectory {
				data, err := os.ReadFile(filepath.Join(path, "child"))
				if err != nil || string(data) != "original" {
					t.Fatalf("existing child = %q/%v, want original/nil", data, err)
				}
			}
			if tc.fault == scratchExistingFile || tc.fault == scratchFileParent {
				data, err := os.ReadFile(path)
				if err != nil || string(data) != "original" {
					t.Fatalf("existing file = %q/%v, want original/nil", data, err)
				}
			}
			entries, err := os.ReadDir(outside)
			if err != nil || len(entries) != 0 {
				t.Fatalf("outside namespace = %v/%v, want empty", entries, err)
			}
		})
	}
}

func TestCreateDirectoryConcurrentOwnership(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	root := requireTestRoot(t, directory)
	request := filestore.DirectoryRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, "entry")}, Mode: 0700}
	ctx, cancel := newFilesystemBackstop(t.Context(), t, 10*time.Second)
	defer cancel()
	const contenders = 8
	start := make(chan struct{})
	results := make(chan error, contenders)
	for range contenders {
		go func() {
			select {
			case <-start:
				results <- filestore.CreateDirectory(ctx, request)
			case <-ctx.Done():
				results <- ctx.Err()
			}
		}()
	}
	close(start)
	winners := 0
	for range contenders {
		select {
		case err := <-results:
			if err == nil {
				winners++
			} else if !errors.Is(err, core.ErrFilestoreConflict) || !errors.Is(err, fs.ErrExist) {
				t.Errorf("contender = %v, want nil or conflict/exist", err)
			}
		case <-ctx.Done():
			t.Fatalf("contender completion = %v, want all owned calls joined", ctx.Err())
		}
	}
	if winners != 1 {
		t.Fatalf("exclusive owners = %d, want exactly one", winners)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 || entries[0].Name() != "entry" || !entries[0].IsDir() {
		t.Fatalf("created namespace = %v/%v, want one entry directory", entries, err)
	}
}

func TestCreateDirectoryRequiresExistingParent(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	root := requireTestRoot(t, directory)
	request := filestore.DirectoryRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, "missing/leaf")}, Mode: 0700}
	err := filestore.CreateDirectory(t.Context(), request)
	if !errors.Is(err, core.ErrFilestoreActivation) || !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing parent = %v, want activation/not-exist", err)
	}
	entries, readErr := os.ReadDir(directory)
	if readErr != nil || len(entries) != 0 {
		t.Fatalf("partial namespace = %v/%v, want empty", entries, readErr)
	}
	if err := os.Mkdir(filepath.Join(directory, "missing"), 0750); err != nil {
		t.Fatalf("parent fixture = %v, want nil", err)
	}
	before, err := os.Stat(filepath.Join(directory, "missing"))
	if err != nil {
		t.Fatalf("parent stat = %v, want nil", err)
	}
	if err := filestore.CreateDirectory(t.Context(), request); err != nil {
		t.Fatalf("existing parent = %v, want nil", err)
	}
	after, err := os.Stat(filepath.Join(directory, "missing"))
	if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() {
		t.Fatalf("parent after create = %v/%v, want unchanged inode and mode %v", after, err, before)
	}
}

func FuzzCreateDirectoryNativeModeAndExclusivity(f *testing.F) {
	// Seed the semantic boundary from a real validated creation request.
	directory := f.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		f.Fatalf("seed root = %v, want nil", err)
	}
	f.Cleanup(func() { openRootClose(f, root) })
	request := filestore.DirectoryRequest{Location: filestore.Location{Root: root, Path: openRootRelative(f, "seed")}, Mode: 0700}
	if err := filestore.CreateDirectory(f.Context(), request); err != nil {
		f.Fatalf("seed creation = %v, want nil", err)
	}
	f.Add(uint32(request.Mode))
	f.Add(uint32(0777))
	f.Add(uint32(0370))
	f.Add(uint32(0))
	f.Add(uint32(fs.ModeSymlink))
	f.Fuzz(func(t *testing.T, mode uint32) {
		directory := t.TempDir()
		path := filepath.Join(directory, "entry")
		request := filestore.DirectoryRequest{Location: filestore.Location{Root: requireTestRoot(t, directory), Path: mustRelativePath(t, "entry")}, Mode: fs.FileMode(mode)}
		err := filestore.CreateDirectory(t.Context(), request)
		info, statErr := os.Lstat(path)
		if mode == 0 || fs.FileMode(mode)&^fs.ModePerm != 0 {
			if !errors.Is(err, core.ErrFilestoreContract) || !errors.Is(statErr, fs.ErrNotExist) {
				t.Fatalf("invalid mode creation = %v/stat %v, want contract/absent", err, statErr)
			}
			return
		}
		if statErr != nil || !info.IsDir() {
			t.Fatalf("native creation = %v/%v, want real directory", info, statErr)
		}
		if err != nil && (!errors.Is(err, core.ErrFilestoreActivation) || !errors.Is(err, fs.ErrPermission)) {
			t.Fatalf("post-create refusal = %v, want native permission failure", err)
		}
		if err == nil && info.Mode().Perm() != request.Mode {
			t.Fatalf("created mode = %v, want %v", info.Mode().Perm(), request.Mode)
		}
		if err != nil && info.Mode().Perm()&^request.Mode != 0 {
			t.Fatalf("partial mode = %v, want no permissions beyond %v", info.Mode().Perm(), request.Mode)
		}
		// Restore accessibility only after observing native mode effects.
		if err := os.Chmod(path, 0700); err != nil {
			t.Fatalf("fixture accessibility = %v, want nil", err)
		}
		marker := filepath.Join(path, "preserved")
		if err := os.WriteFile(marker, []byte("original"), 0600); err != nil {
			t.Fatalf("child marker = %v, want nil", err)
		}
		if err := filestore.CreateDirectory(t.Context(), request); !errors.Is(err, core.ErrFilestoreConflict) || !errors.Is(err, fs.ErrExist) {
			t.Fatalf("repeated create = %v, want conflict/exist", err)
		}
		after, statErr := os.Lstat(path)
		data, readErr := os.ReadFile(marker)
		if statErr != nil || !os.SameFile(info, after) || after.Mode().Perm() != 0700 || readErr != nil || string(data) != "original" {
			t.Fatalf("occupied namespace = %v/%v/%q/%v, want same inode and preserved mode/child", after, statErr, data, readErr)
		}
	})
}
