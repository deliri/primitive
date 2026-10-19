package filestore_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestCreatedDirectoryScopeRefusesBeforeBorrowing(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		want       error
		wantNative error
		fault      scratchFault
		missingUse bool
	}{
		{name: "missing consumer cannot create a tree", missingUse: true, want: core.ErrFilestoreContract},
		{name: "missing context cannot create a tree", fault: scratchNilContext, want: core.ErrNilContext},
		{name: "cancelled context cannot create a tree", fault: scratchCancelled, want: context.Canceled},
		{name: "closed parent preserves native refusal", fault: scratchClosedRoot, want: core.ErrFilestoreActivation},
		{name: "missing parent cannot borrow cwd", fault: scratchNilRoot, want: core.ErrFilestoreContract},
		{name: "root itself cannot be adopted", fault: scratchRootPath, want: core.ErrFilestoreContract},
		{name: "unset path cannot select a tree", fault: scratchUnsetPath, want: core.ErrFilestoreContract},
		{name: "zero mode cannot create an unreadable tree", fault: scratchZeroMode, want: core.ErrFilestoreContract},
		{name: "file type bits cannot enter directory mode", fault: scratchTypeMode, want: core.ErrFilestoreContract},
		{name: "existing directory is never adopted", fault: scratchExistingDirectory, want: core.ErrFilestoreConflict, wantNative: fs.ErrExist},
		{name: "existing regular file is never removed", fault: scratchExistingFile, want: core.ErrFilestoreConflict, wantNative: fs.ErrExist},
		{name: "existing symbolic link is never followed", fault: scratchExistingLink, want: core.ErrFilestoreConflict, wantNative: fs.ErrExist},
		{name: "file parent preserves not-directory refusal", fault: scratchFileParent, want: core.ErrFilestoreActivation},
		{name: "outside ancestor cannot create outside rooted boundary", fault: scratchEscapeLink, want: core.ErrFilestoreActivation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			outside := t.TempDir()
			parent := requireTestRoot(t, directory)
			path := filepath.Join(directory, "entry")
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			calls := 0
			request := filestore.CreatedDirectoryScopeRequest{
				Directory: filestore.DirectoryRequest{Location: filestore.Location{Root: parent, Path: mustRelativePath(t, "entry")}, Mode: 0700},
				Use:       func(context.Context, *os.Root) error { calls++; return nil },
			}
			if tc.missingUse {
				request.Use = nil
			}
			switch tc.fault {
			case scratchNoFault:
			case scratchNilContext:
				ctx = nil
			case scratchCancelled:
				cancel()
			case scratchClosedRoot:
				if err := parent.Close(); err != nil {
					t.Fatal(err)
				}
			case scratchNilRoot:
				request.Directory.Location.Root = nil
			case scratchRootPath:
				request.Directory.Location.Path = mustRelativePath(t, ".")
			case scratchUnsetPath:
				request.Directory.Location.Path = core.RelativePath{}
			case scratchZeroMode:
				request.Directory.Mode = 0
			case scratchTypeMode:
				request.Directory.Mode |= fs.ModeSymlink
			case scratchExistingDirectory:
				if err := os.Mkdir(path, 0750); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(path, "child"), []byte("original"), 0600); err != nil {
					t.Fatal(err)
				}
			case scratchExistingFile, scratchFileParent:
				if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
					t.Fatal(err)
				}
				if tc.fault == scratchFileParent {
					request.Directory.Location.Path = mustRelativePath(t, "entry/child")
				}
			case scratchExistingLink, scratchEscapeLink:
				if err := os.Symlink(outside, path); err != nil {
					t.Fatal(err)
				}
				if tc.fault == scratchEscapeLink {
					request.Directory.Location.Path = mustRelativePath(t, "entry/child")
				}
			default:
				t.Fatalf("unhandled native setup fault %d", tc.fault)
			}
			before, beforeErr := os.Lstat(path)
			gotErr := filestore.WithCreatedDirectoryScope(ctx, request)
			if !errors.Is(gotErr, tc.want) || tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) || calls != 0 {
				t.Fatalf("scope=%v, visits=%d, want %v/native %v and no borrowed root", gotErr, calls, tc.want, tc.wantNative)
			}
			if !tc.missingUse {
				destination, err := filestore.OpenCreatedDirectoryDestination(ctx, request.Directory)
				if destination != nil || !errors.Is(err, tc.want) || tc.wantNative != nil && !errors.Is(err, tc.wantNative) {
					t.Fatalf("direct native acquisition=%v/%v, want no custody and %v/native %v", destination, err, tc.want, tc.wantNative)
				}
			}
			after, afterErr := os.Lstat(path)
			if beforeErr == nil {
				if afterErr != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() {
					t.Fatalf("refusal changed original namespace: %v/%v -> %v/%v", before, beforeErr, after, afterErr)
				}
			} else if !errors.Is(afterErr, fs.ErrNotExist) {
				t.Fatalf("refusal created native entry: %v", afterErr)
			}
			outsideEntries, err := os.ReadDir(outside)
			if err != nil || len(outsideEntries) != 0 {
				t.Fatalf("root refusal escaped into outside tree: %v/%v", outsideEntries, err)
			}
		})
	}
}
