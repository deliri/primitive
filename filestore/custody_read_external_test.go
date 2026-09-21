package filestore_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestOpenCustodyReadNativeOwnershipLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr    error
		wantNative error
		name       string
		entry      nativeHandleEntry
		mutation   readUpdateMutation
		empty      bool
	}{
		{name: "regular entry returns its exact read-only inode"},
		{name: "empty entry returns EOF without inventing bytes", empty: true},
		{name: "confined symbolic leaf cannot substitute its referent", entry: nativeHandleConfinedLink, wantErr: core.ErrFilestoreSource, wantNative: fs.ErrInvalid},
		{name: "dangling symbolic leaf is refused without creating referent", entry: nativeHandleDanglingLink, wantErr: core.ErrFilestoreSource, wantNative: fs.ErrInvalid},
		{name: "outside symbolic leaf cannot escape root", entry: nativeHandleOutsideLink, wantErr: core.ErrFilestoreSource, wantNative: fs.ErrInvalid},
		{name: "missing entry remains missing", entry: nativeHandleMissing, wantErr: core.ErrFilestoreSource, wantNative: fs.ErrNotExist},
		{name: "directory cannot become a regular read handle", entry: nativeHandleDirectory, wantErr: core.ErrFilestoreSource, wantNative: fs.ErrInvalid},
		{name: "cancelled read exposes no handle", mutation: readUpdateCanceled, wantErr: context.Canceled},
		{name: "nil context exposes no handle", mutation: readUpdateNilContext, wantErr: core.ErrNilContext},
		{name: "nil root cannot substitute cwd", mutation: readUpdateNilRoot, wantErr: core.ErrFilestoreContract},
		{name: "unset path cannot select an entry", mutation: readUpdateZeroPath, wantErr: core.ErrFilestoreContract},
		{name: "root directory is not regular", mutation: readUpdateRootPath, wantErr: core.ErrFilestoreSource, wantNative: fs.ErrInvalid},
		{name: "closed root retains native closed refusal", mutation: readUpdateClosedRoot, wantErr: core.ErrFilestoreSource, wantNative: fs.ErrClosed},
		{name: "held inode survives later namespace replacement", mutation: readUpdateRenameAfterOpen},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			outside := t.TempDir()
			root := requireTestRoot(t, directory)
			payload := []byte{0, 255, 8, 31}
			if tc.empty {
				payload = nil
			}
			subject := filepath.Join(directory, "subject")
			outsidePath := filepath.Join(outside, "outside")
			for _, path := range []string{subject, outsidePath} {
				if err := os.WriteFile(path, payload, 0600); err != nil {
					t.Fatalf("fixture write = %v, want nil", err)
				}
			}
			entry := filepath.Join(directory, "entry")
			switch tc.entry {
			case nativeHandleRegular:
				if err := os.WriteFile(entry, payload, 0600); err != nil {
					t.Fatalf("entry write = %v, want nil", err)
				}
			case nativeHandleMissing:
			case nativeHandleDirectory:
				if err := os.Mkdir(entry, 0700); err != nil {
					t.Fatalf("entry directory = %v, want nil", err)
				}
			case nativeHandleConfinedLink, nativeHandleDanglingLink, nativeHandleOutsideLink:
				target := "subject"
				if tc.entry == nativeHandleDanglingLink {
					target = "absent"
				}
				if tc.entry == nativeHandleOutsideLink {
					target = outsidePath
				}
				if err := os.Symlink(target, entry); err != nil {
					t.Fatalf("entry symlink = %v, want nil", err)
				}
			default:
				t.Fatalf("entry = %d, want handled domain", tc.entry)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			request := filestore.ReadHandleRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, "entry")}}
			switch tc.mutation {
			case readUpdateUnchanged, readUpdateRenameAfterOpen:
			case readUpdateCanceled:
				cancel()
			case readUpdateNilContext:
				ctx = nil
			case readUpdateNilRoot:
				request.Location.Root = nil
			case readUpdateZeroPath:
				request.Location.Path = core.RelativePath{}
			case readUpdateRootPath:
				request.Location.Path = mustRelativePath(t, ".")
			case readUpdateClosedRoot:
				if err := root.Close(); err != nil {
					t.Fatalf("close root = %v, want nil", err)
				}
			default:
				t.Fatalf("mutation = %d, want handled domain", tc.mutation)
			}
			before, beforeErr := os.Lstat(entry)
			got, gotErr := filestore.OpenCustodyRead(ctx, request)
			if got != nil {
				t.Cleanup(func() { _ = got.Close() })
			}
			if !errors.Is(gotErr, tc.wantErr) || tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) {
				t.Fatalf("OpenCustodyRead = %v/%v, want %v/native %v", got, gotErr, tc.wantErr, tc.wantNative)
			}
			if tc.wantErr != nil {
				if got != nil {
					t.Fatalf("refused handle = %v, want nil", got)
				}
			} else {
				if got == nil || beforeErr != nil {
					t.Fatalf("accepted handle = %v/before %v, want existing inode", got, beforeErr)
				}
				held, err := got.Stat()
				if err != nil || !os.SameFile(before, held) {
					t.Fatalf("held inode = %v/%v, want %v", held, err, before)
				}
				if tc.mutation == readUpdateRenameAfterOpen {
					if err := os.Rename(entry, filepath.Join(directory, "original")); err != nil {
						t.Fatalf("rename = %v, want nil", err)
					}
					if err := os.WriteFile(entry, []byte("replacement"), 0600); err != nil {
						t.Fatalf("replacement = %v, want nil", err)
					}
				}
				data, err := io.ReadAll(io.LimitReader(got, int64(len(payload)+1)))
				if err != nil || !bytes.Equal(data, payload) {
					t.Fatalf("held bytes = %v/%v, want %v", data, err, payload)
				}
				n, writeErr := got.Write([]byte("forbidden"))
				var pathErr *fs.PathError
				if n != 0 || !errors.As(writeErr, &pathErr) {
					t.Fatalf("read-only write = %d/%v, want native path refusal", n, writeErr)
				}
				if err := got.Close(); err != nil {
					t.Fatalf("close = %v, want nil", err)
				}
				if _, err := got.Stat(); !errors.Is(err, fs.ErrClosed) {
					t.Fatalf("closed handle = %v, want closed", err)
				}
			}
			after, afterErr := os.Lstat(entry)
			if tc.mutation != readUpdateRenameAfterOpen {
				if beforeErr == nil && (afterErr != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size()) {
					t.Fatalf("entry after read = %v/%v, want preserved %v", after, afterErr, before)
				}
				if errors.Is(beforeErr, fs.ErrNotExist) && !errors.Is(afterErr, fs.ErrNotExist) {
					t.Fatalf("missing entry = %v, want absent", afterErr)
				}
			}
			for _, path := range []string{subject, outsidePath} {
				data, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(data, payload) {
					t.Fatalf("neighbor bytes = %v/%v, want %v", data, err, payload)
				}
			}
		})
	}
}

func FuzzOpenCustodyReadExactInodeAndBytes(f *testing.F) {
	directory := f.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		f.Fatalf("seed root = %v, want nil", err)
	}
	f.Cleanup(func() { openRootClose(f, root) })
	seed := []byte{0, 255, 8, 31}
	request := filestore.ScratchRequest{Location: filestore.Location{Root: root, Path: openRootRelative(f, "seed")}, Mode: 0600}
	file, err := filestore.OpenScratch(f.Context(), request)
	if err != nil {
		f.Fatalf("seed producer = %v, want nil", err)
	}
	_, writeErr := file.Write(seed)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		f.Fatalf("seed write/close = %v/%v, want nil", writeErr, closeErr)
	}
	emitted, err := os.ReadFile(filepath.Join(directory, "seed"))
	if err != nil {
		f.Fatalf("seed read = %v, want nil", err)
	}
	f.Add(emitted, false)
	f.Add(emitted, true)
	f.Add([]byte{}, false)
	f.Fuzz(func(t *testing.T, data []byte, symbolic bool) {
		directory := t.TempDir()
		root := requireTestRoot(t, directory)
		subject := filepath.Join(directory, "subject")
		if err := os.WriteFile(subject, data, 0600); err != nil {
			t.Fatalf("subject write = %v, want nil", err)
		}
		name := "subject"
		if symbolic {
			name = "link"
			if err := os.Symlink("subject", filepath.Join(directory, name)); err != nil {
				t.Fatalf("symbolic mutation = %v, want nil", err)
			}
		}
		before, err := os.Lstat(filepath.Join(directory, name))
		if err != nil || (before.Mode()&fs.ModeSymlink != 0) != symbolic {
			t.Fatalf("mutated entry = %v/%v, want symbolic %t", before, err, symbolic)
		}
		got, gotErr := filestore.OpenCustodyRead(t.Context(), filestore.ReadHandleRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, name)}})
		if got != nil {
			t.Cleanup(func() { _ = got.Close() })
		}
		if symbolic {
			if got != nil || !errors.Is(gotErr, core.ErrFilestoreSource) || !errors.Is(gotErr, fs.ErrInvalid) {
				t.Fatalf("symbolic acquisition = %v/%v, want nil/source/invalid", got, gotErr)
			}
		} else {
			if gotErr != nil || got == nil {
				t.Fatalf("regular acquisition = %v/%v, want handle/nil", got, gotErr)
			}
			held, statErr := got.Stat()
			hash := sha256.New()
			n, readErr := io.Copy(hash, got)
			closeErr := got.Close()
			want := sha256.Sum256(data)
			if statErr != nil || !os.SameFile(before, held) || readErr != nil || closeErr != nil || n != int64(len(data)) || !bytes.Equal(hash.Sum(nil), want[:]) {
				t.Fatalf("held content = %v/%v/%d/%v/%v, want exact original inode/hash/extent", held, statErr, n, readErr, closeErr)
			}
		}
		after, err := os.Lstat(filepath.Join(directory, name))
		if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() {
			t.Fatalf("entry after acquisition = %v/%v, want unchanged %v", after, err, before)
		}
		native, err := os.Open(subject)
		if err != nil {
			t.Fatalf("native content observation = %v, want nil", err)
		}
		hash := sha256.New()
		n, readErr := io.Copy(hash, native)
		closeErr := native.Close()
		want := sha256.Sum256(data)
		if readErr != nil || closeErr != nil || n != int64(len(data)) || !bytes.Equal(hash.Sum(nil), want[:]) {
			t.Fatalf("preserved content = %d/%v/%v, want original hash/extent", n, readErr, closeErr)
		}
	})
}
