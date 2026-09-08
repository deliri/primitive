package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

type readUpdateMutation uint8

const (
	readUpdateUnchanged readUpdateMutation = iota
	readUpdateNilContext
	readUpdateCanceled
	readUpdateNilRoot
	readUpdateZeroPath
	readUpdateRootPath
	readUpdateClosedRoot
	readUpdateBelowFile
	readUpdateLoopingLink
	readUpdateRenameAfterOpen
)

func TestReadAndUpdateNativeHandleLayerTriad(t *testing.T) {
	t.Parallel()
	for _, operation := range []struct {
		name         string
		door         nativeHandleDoor
		flag         int
		wantBoundary error
	}{
		{name: "read", door: nativeHandleRead, flag: os.O_RDONLY, wantBoundary: core.ErrFilestoreSource},
		{name: "update", door: nativeHandleUpdate, flag: os.O_RDWR, wantBoundary: core.ErrFilestoreActivation},
	} {
		t.Run(operation.name, func(t *testing.T) {
			t.Parallel()
			for _, tc := range []struct {
				name               string
				entry              nativeHandleEntry
				mutation           readUpdateMutation
				empty, wantRefusal bool
				wantErr            error
			}{
				{name: "binary extent cannot be truncated during acquisition"},
				{name: "empty file yields a real capability without invented bytes", empty: true},
				{name: "confined symlink opens exactly its regular referent", entry: nativeHandleConfinedLink},
				{name: "absent entry cannot be created by read or update", entry: nativeHandleMissing, wantRefusal: true},
				{name: "directory cannot be presented as regular bytes", entry: nativeHandleDirectory, wantRefusal: true},
				{name: "dangling link cannot create its missing referent", entry: nativeHandleDanglingLink, wantRefusal: true},
				{name: "outside link cannot escape the rooted capability", entry: nativeHandleOutsideLink, wantRefusal: true},
				{name: "path below a file retains native not-directory refusal", mutation: readUpdateBelowFile, wantRefusal: true},
				{name: "looping link retains native loop refusal", mutation: readUpdateLoopingLink, wantRefusal: true},
				{name: "nil context cannot return an OS capability", mutation: readUpdateNilContext, wantErr: core.ErrNilContext, wantRefusal: true},
				{name: "canceled context cannot return an OS capability", mutation: readUpdateCanceled, wantErr: context.Canceled, wantRefusal: true},
				{name: "nil root cannot substitute process cwd", mutation: readUpdateNilRoot, wantErr: core.ErrFilestoreContract, wantRefusal: true},
				{name: "zero path cannot select a default entry", mutation: readUpdateZeroPath, wantErr: core.ErrFilestoreContract, wantRefusal: true},
				{name: "root entry cannot be handed out as a regular file", mutation: readUpdateRootPath, wantRefusal: true},
				{name: "closed root retains native closed identity", mutation: readUpdateClosedRoot, wantRefusal: true},
				{name: "held file survives same-byte namespace replacement", mutation: readUpdateRenameAfterOpen},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					directory := t.TempDir()
					root := requireTestRoot(t, directory)
					payload := []byte{0, 255, 8, 0, 31}
					if tc.empty {
						payload = nil
					}
					neighbor := []byte{41, 0, 255}
					if err := os.WriteFile(filepath.Join(directory, "neighbor"), neighbor, 0o600); err != nil {
						t.Fatal(err)
					}
					outside := t.TempDir()
					outsidePath := filepath.Join(outside, "outside")
					if err := os.WriteFile(outsidePath, payload, 0o600); err != nil {
						t.Fatal(err)
					}
					outsideBefore, err := os.Lstat(outsidePath)
					if err != nil {
						t.Fatal(err)
					}
					physical := "entry"
					if tc.entry == nativeHandleConfinedLink || tc.entry == nativeHandleDanglingLink {
						physical = "subject"
					}
					switch tc.entry {
					case nativeHandleRegular:
						if err := os.WriteFile(filepath.Join(directory, "entry"), payload, 0o600); err != nil {
							t.Fatal(err)
						}
					case nativeHandleMissing:
					case nativeHandleDirectory:
						if err := root.Mkdir("entry", 0o700); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(filepath.Join(directory, "entry", "child"), payload, 0o600); err != nil {
							t.Fatal(err)
						}
					case nativeHandleConfinedLink, nativeHandleDanglingLink:
						if tc.entry == nativeHandleConfinedLink {
							if err := os.WriteFile(filepath.Join(directory, "subject"), payload, 0o600); err != nil {
								t.Fatal(err)
							}
						}
						if err := root.Symlink("subject", "entry"); err != nil {
							t.Fatal(err)
						}
					case nativeHandleOutsideLink:
						if err := root.Symlink(outsidePath, "entry"); err != nil {
							t.Fatal(err)
						}
					default:
						t.Fatalf("entry = %d, want declared fixture", tc.entry)
					}
					ctx := t.Context()
					location := filestore.Location{Root: root, Path: mustRelativePath(t, "entry")}
					switch tc.mutation {
					case readUpdateUnchanged, readUpdateRenameAfterOpen:
					case readUpdateNilContext:
						ctx = nil
					case readUpdateCanceled:
						var cancel context.CancelFunc
						ctx, cancel = context.WithCancel(ctx)
						cancel()
					case readUpdateNilRoot:
						location.Root = nil
					case readUpdateZeroPath:
						location.Path = core.RelativePath{}
					case readUpdateRootPath:
						location.Path = mustRelativePath(t, ".")
					case readUpdateClosedRoot:
						if err := root.Close(); err != nil {
							t.Fatal(err)
						}
					case readUpdateBelowFile:
						location.Path = mustRelativePath(t, "entry/child")
					case readUpdateLoopingLink:
						if err := root.Remove("entry"); err != nil {
							t.Fatal(err)
						}
						if err := root.Symlink("entry", "entry"); err != nil {
							t.Fatal(err)
						}
					default:
						t.Fatalf("mutation = %d, want declared request", tc.mutation)
					}
					want, err := removalFixtureSnapshot(directory)
					if err != nil {
						t.Fatal(err)
					}
					var before fs.FileInfo
					if !tc.wantRefusal {
						before, err = os.Stat(filepath.Join(directory, physical))
						if err != nil {
							t.Fatal(err)
						}
					}
					wantErr := tc.wantErr
					var wantNative error
					if tc.mutation == readUpdateRootPath && operation.door == nativeHandleUpdate {
						wantErr = core.ErrFilestoreContract
					}
					if wantErr == nil {
						native, nativeErr := root.OpenFile(location.Path.String(), operation.flag, 0)
						if nativeErr == nil {
							info, err := native.Stat()
							closeErr := native.Close()
							if err != nil || closeErr != nil {
								t.Fatalf("native fixture observation = (%v,%v), want nil", err, closeErr)
							}
							if !info.Mode().IsRegular() {
								nativeErr = fs.ErrInvalid
							}
						}
						if (nativeErr != nil) != tc.wantRefusal {
							t.Fatalf("native acquisition = %v, want refusal %t", nativeErr, tc.wantRefusal)
						}
						if nativeErr != nil {
							wantErr = operation.wantBoundary
							wantNative = nativeErr
							if nativePath, ok := errors.AsType[*fs.PathError](nativeErr); ok {
								wantNative = nativePath.Err
							}
						}
					}
					var got *os.File
					var gotErr error
					if operation.door == nativeHandleRead {
						got, gotErr = filestore.OpenRead(ctx, filestore.ReadHandleRequest{Location: location})
					} else {
						got, gotErr = filestore.OpenUpdate(ctx, filestore.UpdateHandleRequest{Location: location})
					}
					if got != nil {
						t.Cleanup(func() { _ = got.Close() })
					}
					if !errors.Is(gotErr, wantErr) || wantNative != nil && !errors.Is(gotErr, wantNative) {
						t.Fatalf("acquisition = (%v,%v), want %v and native %v", got, gotErr, wantErr, wantNative)
					}
					if tc.wantRefusal {
						if got != nil {
							t.Fatalf("refused capability = %v, want nil", got)
						}
					} else {
						if got == nil {
							t.Fatal("accepted capability = nil, want real Go file")
						}
						info, err := got.Stat()
						if err != nil {
							t.Fatal(err)
						}
						if !os.SameFile(info, before) || info.Size() != int64(len(payload)) || info.Mode() != before.Mode() || !info.ModTime().Equal(before.ModTime()) {
							t.Fatalf("acquired inode = %v, want unchanged %v", info, before)
						}
						readBytes, err := io.ReadAll(io.LimitReader(got, int64(len(payload)+1)))
						if err != nil || !bytes.Equal(readBytes, payload) {
							t.Fatalf("acquired bytes = (%v,%v), want %v", readBytes, err, payload)
						}
						if tc.mutation == readUpdateRenameAfterOpen {
							if err := root.Rename(physical, "archive"); err != nil {
								t.Fatal(err)
							}
							if err := os.WriteFile(filepath.Join(directory, physical), payload, 0o600); err != nil {
								t.Fatal(err)
							}
							replacement, err := root.Lstat(physical)
							if err != nil {
								t.Fatal(err)
							}
							if os.SameFile(replacement, before) {
								t.Fatalf("replacement inode = %v, want distinct from %v", replacement, before)
							}
							physical = "archive"
							want, err = removalFixtureSnapshot(directory)
							if err != nil {
								t.Fatal(err)
							}
						}
						wantBytes := bytes.Clone(payload)
						if operation.door == nativeHandleRead {
							native, err := os.Open(filepath.Join(directory, physical))
							if err != nil {
								t.Fatal(err)
							}
							_, nativeErr := native.Write([]byte{255})
							closeErr := native.Close()
							if closeErr != nil {
								t.Fatal(closeErr)
							}
							n, writeErr := got.Write([]byte{255})
							var nativePath *fs.PathError
							if !errors.As(nativeErr, &nativePath) || n != 0 || !errors.Is(writeErr, nativePath.Err) {
								t.Fatalf("read-only write = (%d,%v), want native refusal %v", n, writeErr, nativeErr)
							}
						} else {
							change := []byte{255, 0}
							n, err := got.WriteAt(change, 1)
							if err != nil || n != len(change) {
								t.Fatalf("update = (%d,%v), want %d", n, err, len(change))
							}
							if len(wantBytes) < 3 {
								wantBytes = append(wantBytes, make([]byte, 3-len(wantBytes))...)
							}
							copy(wantBytes[1:], change)
							for i := range want {
								if want[i].name == physical {
									want[i].data = wantBytes
								}
							}
						}
						held, err := got.Stat()
						if err != nil {
							t.Fatal(err)
						}
						if !os.SameFile(held, before) || held.Size() != int64(len(wantBytes)) || held.Mode() != before.Mode() {
							t.Fatalf("held inode = %v, want original with exact extent %d", held, len(wantBytes))
						}
						if err := got.Close(); err != nil {
							t.Fatal(err)
						}
						if _, err := got.Stat(); !errors.Is(err, fs.ErrClosed) {
							t.Fatalf("closed handle = %v, want native closed", err)
						}
					}
					gotNamespace, err := removalFixtureSnapshot(directory)
					if err != nil {
						t.Fatal(err)
					}
					if len(gotNamespace) != len(want) {
						t.Fatalf("namespace = %+v, want %+v", gotNamespace, want)
					}
					for i, got := range gotNamespace {
						if got.name != want[i].name || got.mode != want[i].mode || got.target != want[i].target || !bytes.Equal(got.data, want[i].data) {
							t.Fatalf("entry %d = %+v, want %+v", i, got, want[i])
						}
					}
					outsideAfter, err := os.Lstat(outsidePath)
					if err != nil {
						t.Fatal(err)
					}
					outsideBytes, err := os.ReadFile(outsidePath)
					if err != nil || !bytes.Equal(outsideBytes, payload) || !os.SameFile(outsideBefore, outsideAfter) || outsideBefore.Mode() != outsideAfter.Mode() || !outsideBefore.ModTime().Equal(outsideAfter.ModTime()) {
						t.Fatalf("outside = (%v,%v,%v), want untouched bytes and metadata", outsideAfter, outsideBytes, err)
					}
				})
			}
		})
	}
}
