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

func TestScratchDirectoryLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		fault   scratchFault
		wantErr error
	}{
		{name: "missing chain is created with exact final permissions"},
		{name: "existing directory retains its children on repeated preparation", fault: scratchExistingDirectory},
		{name: "file at final name cannot be replaced", fault: scratchExistingFile, wantErr: core.ErrFilestoreActivation},
		{name: "file in parent chain cannot be replaced", fault: scratchFileParent, wantErr: core.ErrFilestoreActivation},
		{name: "symlink to file cannot become directory", fault: scratchExistingLink, wantErr: core.ErrFilestoreActivation},
		{name: "ancestor link cannot create outside root", fault: scratchEscapeLink, wantErr: core.ErrFilestoreActivation},
		{name: "cancelled request creates no chain", fault: scratchCancelled, wantErr: context.Canceled},
		{name: "nil context creates no chain", fault: scratchNilContext, wantErr: core.ErrNilContext},
		{name: "closed capability creates no chain", fault: scratchClosedRoot, wantErr: core.ErrFilestoreActivation},
		{name: "nil capability creates no chain", fault: scratchNilRoot, wantErr: core.ErrFilestoreContract},
		{name: "root metadata cannot be rewritten", fault: scratchRootPath, wantErr: core.ErrFilestoreContract},
		{name: "unset path creates no chain", fault: scratchUnsetPath, wantErr: core.ErrFilestoreContract},
		{name: "zero permissions create no chain", fault: scratchZeroMode, wantErr: core.ErrFilestoreContract},
		{name: "type bits cannot change namespace kind", fault: scratchTypeMode, wantErr: core.ErrFilestoreContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			workspace := t.TempDir()
			directory := filepath.Join(workspace, "root")
			foreign := filepath.Join(workspace, "foreign")
			for _, name := range []string{directory, foreign} {
				if err := os.Mkdir(name, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			root := requireTestRoot(t, directory)
			payload := []byte("preserved")
			if err := os.WriteFile(filepath.Join(directory, "held"), payload, 0o600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			request := filestore.DirectoryRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, "parent/child")}, Mode: 0o750}
			switch tc.fault {
			case scratchNoFault:
			case scratchExistingDirectory:
				if err := os.MkdirAll(filepath.Join(directory, "parent/child"), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(directory, "parent/child/keep"), payload, 0o600); err != nil {
					t.Fatal(err)
				}
			case scratchExistingFile:
				request.Location.Path = mustRelativePath(t, "held")
			case scratchFileParent:
				request.Location.Path = mustRelativePath(t, "held/child")
			case scratchExistingLink:
				if err := os.Symlink("held", filepath.Join(directory, "link")); err != nil {
					t.Fatal(err)
				}
				request.Location.Path = mustRelativePath(t, "link")
			case scratchEscapeLink:
				if err := os.Symlink(foreign, filepath.Join(directory, "escape")); err != nil {
					t.Fatal(err)
				}
				request.Location.Path = mustRelativePath(t, "escape/child")
			case scratchCancelled:
				cancel()
			case scratchNilContext:
				ctx = nil
			case scratchClosedRoot:
				if err := root.Close(); err != nil {
					t.Fatal(err)
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
				t.Fatalf("unhandled fault %d", tc.fault)
			}
			before, err := os.ReadDir(directory)
			if err != nil {
				t.Fatal(err)
			}
			gotErr := filestore.EnsureScratchDirectory(ctx, request)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("EnsureScratchDirectory = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr == nil {
				info, err := os.Stat(filepath.Join(directory, request.Location.Path.String()))
				if err != nil || !info.IsDir() || info.Mode().Perm() != request.Mode {
					t.Fatalf("final directory = %v/%v, want directory mode %v", info, err, request.Mode)
				}
				if err := filestore.EnsureScratchDirectory(ctx, request); err != nil {
					t.Fatalf("repeat preparation = %v, want nil", err)
				}
				if tc.fault == scratchExistingDirectory {
					got, err := os.ReadFile(filepath.Join(directory, "parent/child/keep"))
					if err != nil || !bytes.Equal(got, payload) {
						t.Fatalf("retained child = %q/%v, want %q", got, err, payload)
					}
				}
			} else {
				after, err := os.ReadDir(directory)
				if err != nil || len(after) != len(before) {
					t.Fatalf("refusal namespace = %v/%v, want %v", after, err, before)
				}
				for index := range before {
					if before[index].Name() != after[index].Name() || before[index].Type() != after[index].Type() {
						t.Fatalf("entry = %v, want %v", after[index], before[index])
					}
				}
			}
			got, err := os.ReadFile(filepath.Join(directory, "held"))
			if err != nil || !bytes.Equal(got, payload) {
				t.Fatalf("held = %q/%v, want %q", got, err, payload)
			}
			foreignEntries, err := os.ReadDir(foreign)
			if err != nil || len(foreignEntries) != 0 {
				t.Fatalf("foreign entries = %v/%v, want empty", foreignEntries, err)
			}
		})
	}
}

// Both public creation doors consume the same permission domain. The filesystem
// is the oracle for accepted modes; repeated file creation must preserve the
// original bytes while repeated directory creation must preserve its child.
func FuzzScratchCreationModesSemanticClosure(f *testing.F) {
	f.Add(uint32(0o600))
	f.Add(uint32(0o777))
	// Owner write/search without read can refuse reopening after MkdirAll;
	// the native umask may still have removed group write at that point.
	f.Add(uint32(0o370))
	f.Add(uint32(0))
	f.Add(uint32(fs.ModeSymlink))
	f.Fuzz(func(t *testing.T, mode uint32) {
		directory := t.TempDir()
		root := requireTestRoot(t, directory)
		path := mustRelativePath(t, "scratch")
		request := filestore.ScratchRequest{Location: filestore.Location{Root: root, Path: path}, Mode: fs.FileMode(mode)}
		file, err := filestore.OpenScratch(t.Context(), request)
		directoryRequest := filestore.DirectoryRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, "tree")}, Mode: request.Mode}
		directoryErr := filestore.EnsureScratchDirectory(t.Context(), directoryRequest)
		if mode == 0 || fs.FileMode(mode)&^fs.ModePerm != 0 {
			if !errors.Is(err, core.ErrFilestoreContract) || file != nil || !errors.Is(directoryErr, core.ErrFilestoreContract) {
				t.Fatalf("invalid mode = %v/%v/%v, want nil handles and typed refusals", file, err, directoryErr)
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != 0 {
				t.Fatalf("rejected entries = %v/%v, want empty", entries, err)
			}
			return
		}
		if err != nil || file == nil {
			t.Fatalf("accepted mode = %v/%v/%v", file, err, directoryErr)
		}
		// The OS may refuse to reopen a directory whose requested mode removes
		// read permission. Preserve that native refusal; it is not invalid input.
		if directoryErr != nil && (!errors.Is(directoryErr, core.ErrFilestoreActivation) || !errors.Is(directoryErr, fs.ErrPermission)) {
			t.Fatalf("directory refusal = %v, want native permission refusal", directoryErr)
		}
		info, statErr := file.Stat()
		payload := []byte{1, 3, 5, 7}
		_, writeErr := file.Write(payload)
		closeErr := file.Close()
		if statErr != nil || writeErr != nil || closeErr != nil || info.Mode().Perm() != request.Mode {
			t.Fatalf("file mode/write/close = %v/%v/%v/%v", info, statErr, writeErr, closeErr)
		}
		duplicate, repeatErr := filestore.OpenScratch(t.Context(), request)
		if duplicate != nil || !errors.Is(repeatErr, core.ErrFilestoreConflict) || !errors.Is(repeatErr, fs.ErrExist) {
			t.Fatalf("repeat creation = %v/%v, want nil/existing conflict", duplicate, repeatErr)
		}
		// Some valid modes remove read/search permission. Restore only for the
		// independent content oracle, after exact production modes were checked.
		if err := os.Chmod(filepath.Join(directory, "scratch"), 0o600); err != nil {
			t.Fatal(err)
		}
		got, readErr := os.ReadFile(filepath.Join(directory, "scratch"))
		if readErr != nil || !bytes.Equal(got, payload) {
			t.Fatalf("preserved bytes = %v/%v, want %v", got, readErr, payload)
		}
		dirInfo, statErr := os.Stat(filepath.Join(directory, "tree"))
		if statErr != nil || !dirInfo.IsDir() {
			t.Fatalf("created directory = %v/%v, want a directory after successful MkdirAll", dirInfo, statErr)
		}
		if directoryErr == nil && dirInfo.Mode().Perm() != request.Mode {
			t.Fatalf("successful directory mode = %v, want %v", dirInfo.Mode().Perm(), request.Mode)
		}
		if directoryErr != nil && dirInfo.Mode().Perm()&^request.Mode != 0 {
			t.Fatalf("refused directory mode = %v, want no permissions beyond requested %v", dirInfo.Mode().Perm(), request.Mode)
		}
	})
}
