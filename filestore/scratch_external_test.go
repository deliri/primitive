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

type scratchFault uint8

const (
	scratchNoFault scratchFault = iota
	scratchCancelled
	scratchNilContext
	scratchNilRoot
	scratchUnsetPath
	scratchRootPath
	scratchZeroMode
	scratchTypeMode
	scratchClosedRoot
	scratchMissingParent
	scratchFileParent
	scratchExistingFile
	scratchExistingDirectory
	scratchExistingLink
	scratchEscapeLink
)

// This exhausts the distinct ingress/namespace decisions of exclusive scratch
// creation. It does not claim coverage of durable publication: no Stage or
// Commit is involved. Each refusal preserves the existing source bytes.
func TestScratchFileLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		fault      scratchFault
		wantErr    error
		wantNative error
	}{
		{name: "created bytes are visible to an independent reader before close"},
		{name: "cancelled context creates no entry", fault: scratchCancelled, wantErr: context.Canceled},
		{name: "nil context creates no entry", fault: scratchNilContext, wantErr: core.ErrNilContext},
		{name: "unset capability creates no entry", fault: scratchNilRoot, wantErr: core.ErrFilestoreContract},
		{name: "unset relative path creates no entry", fault: scratchUnsetPath, wantErr: core.ErrFilestoreContract},
		{name: "root cannot become a file", fault: scratchRootPath, wantErr: core.ErrFilestoreContract},
		{name: "zero permission is refused", fault: scratchZeroMode, wantErr: core.ErrFilestoreContract},
		{name: "file type bits cannot become permissions", fault: scratchTypeMode, wantErr: core.ErrFilestoreContract},
		{name: "closed root preserves native closed identity", fault: scratchClosedRoot, wantErr: core.ErrFilestoreActivation, wantNative: fs.ErrClosed},
		{name: "missing parent is not implicitly created", fault: scratchMissingParent, wantErr: core.ErrFilestoreActivation, wantNative: fs.ErrNotExist},
		{name: "file parent cannot become directory", fault: scratchFileParent, wantErr: core.ErrFilestoreActivation},
		{name: "existing file cannot be truncated", fault: scratchExistingFile, wantErr: core.ErrFilestoreConflict, wantNative: fs.ErrExist},
		{name: "existing directory cannot be replaced", fault: scratchExistingDirectory, wantErr: core.ErrFilestoreConflict, wantNative: fs.ErrExist},
		{name: "existing symlink cannot redirect writes", fault: scratchExistingLink, wantErr: core.ErrFilestoreConflict, wantNative: fs.ErrExist},
		{name: "ancestor symlink cannot escape rooted namespace", fault: scratchEscapeLink, wantErr: core.ErrFilestoreActivation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			workspace := t.TempDir()
			directory := filepath.Join(workspace, "root")
			if err := os.Mkdir(directory, 0o700); err != nil {
				t.Fatal(err)
			}
			root := requireTestRoot(t, directory)
			original := []byte("preserve existing source")
			foreign := filepath.Join(workspace, "foreign")
			if err := os.Mkdir(foreign, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(foreign, "held"), original, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, "held"), original, 0o600); err != nil {
				t.Fatal(err)
			}
			request := filestore.ScratchRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, "created")}, Mode: 0o600}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			switch tc.fault {
			case scratchNoFault:
			case scratchCancelled:
				cancel()
			case scratchNilContext:
				ctx = nil
			case scratchNilRoot:
				request.Location.Root = nil
			case scratchUnsetPath:
				request.Location.Path = core.RelativePath{}
			case scratchRootPath:
				request.Location.Path = mustRelativePath(t, ".")
			case scratchZeroMode:
				request.Mode = 0
			case scratchTypeMode:
				request.Mode |= fs.ModeDir
			case scratchClosedRoot:
				if err := root.Close(); err != nil {
					t.Fatal(err)
				}
			case scratchMissingParent:
				request.Location.Path = mustRelativePath(t, "missing/created")
			case scratchFileParent:
				request.Location.Path = mustRelativePath(t, "held/created")
			case scratchExistingFile:
				request.Location.Path = mustRelativePath(t, "held")
			case scratchExistingDirectory:
				if err := os.Mkdir(filepath.Join(directory, "created"), 0o700); err != nil {
					t.Fatal(err)
				}
			case scratchExistingLink:
				if err := os.Symlink("held", filepath.Join(directory, "created")); err != nil {
					t.Fatal(err)
				}
			case scratchEscapeLink:
				if err := os.Symlink(foreign, filepath.Join(directory, "escape")); err != nil {
					t.Fatal(err)
				}
				request.Location.Path = mustRelativePath(t, "escape/created")
			default:
				t.Fatalf("unhandled fault %d", tc.fault)
			}
			before, err := os.ReadDir(directory)
			if err != nil {
				t.Fatal(err)
			}
			file, gotErr := filestore.OpenScratch(ctx, request)
			if !errors.Is(gotErr, tc.wantErr) || tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) {
				t.Fatalf("OpenScratch = %v, want %v and native %v", gotErr, tc.wantErr, tc.wantNative)
			}
			if tc.wantErr != nil {
				if file != nil {
					t.Fatalf("refused handle = %v, want nil", file)
				}
				after, err := os.ReadDir(directory)
				if err != nil || len(after) != len(before) {
					t.Fatalf("refused namespace = %v/%v, want %v", after, err, before)
				}
				for index := range before {
					if before[index].Name() != after[index].Name() || before[index].Type() != after[index].Type() {
						t.Fatalf("entry after refusal = %v, want %v", after[index], before[index])
					}
				}
			} else {
				if file == nil {
					t.Fatal("successful handle = nil")
				}
				payload := []byte("compiler reads exact scratch bytes")
				if _, err := file.Write(payload); err != nil {
					t.Fatal(err)
				}
				got, readErr := os.ReadFile(filepath.Join(directory, "created"))
				closeErr := file.Close()
				if readErr != nil || closeErr != nil || !bytes.Equal(got, payload) {
					t.Fatalf("scratch bytes = %q/%v/%v, want %q", got, readErr, closeErr, payload)
				}
				info, err := os.Stat(filepath.Join(directory, "created"))
				if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != request.Mode {
					t.Fatalf("scratch mode = %v/%v, want regular %v", info, err, request.Mode)
				}
			}
			for _, base := range []string{directory, foreign} {
				got, err := os.ReadFile(filepath.Join(base, "held"))
				if err != nil || !bytes.Equal(got, original) {
					t.Fatalf("held bytes = %q/%v, want %q", got, err, original)
				}
			}
			foreignEntries, err := os.ReadDir(foreign)
			if err != nil || len(foreignEntries) != 1 || foreignEntries[0].Name() != "held" {
				t.Fatalf("foreign entries = %v/%v, want only held", foreignEntries, err)
			}
		})
	}
}
