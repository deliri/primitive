//go:build darwin || linux

package filestore_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

const (
	childRootDescriptorEnvironment = "PRIMITIVE_FILESTORE_CHILD_ROOT_DESCRIPTOR_PROBE"
	childRootDescriptorMarker      = "child-root-descriptors"
	childRootDescriptorRounds      = 256
)

type childRootEntry uint8

const (
	childRootNested childRootEntry = iota
	childRootConfinedLink
	childRootOutsideLink
	childRootDanglingLink
	childRootRegular
	childRootFIFO
	childRootMissing
)

type childRootMutation uint8

const (
	childRootUnchanged childRootMutation = iota
	childRootCanceled
	childRootNilContext
	childRootNilRoot
	childRootZeroPath
	childRootClosedParent
	childRootParentRenamedAfterOpen
)

func TestOpenChildRootNativeLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr    error
		wantNative error
		name       string
		entry      childRootEntry
		mutation   childRootMutation
	}{
		{name: "nested directory becomes its own confined root"},
		{name: "confined final directory link is followed like OpenRoot", entry: childRootConfinedLink},
		{name: "outside directory link cannot escape the parent root", entry: childRootOutsideLink, wantErr: core.ErrFilestoreSource},
		{name: "dangling directory link is refused", entry: childRootDanglingLink, wantErr: core.ErrFilestoreSource},
		{name: "regular file is not a directory", entry: childRootRegular, wantErr: core.ErrFilestoreSource, wantNative: syscall.ENOTDIR},
		{name: "FIFO is refused without blocking", entry: childRootFIFO, wantErr: core.ErrFilestoreSource, wantNative: syscall.ENOTDIR},
		{name: "missing directory remains missing", entry: childRootMissing, wantErr: core.ErrFilestoreSource, wantNative: fs.ErrNotExist},
		{name: "cancelled acquisition exposes no root", mutation: childRootCanceled, wantErr: context.Canceled},
		{name: "nil context exposes no root", mutation: childRootNilContext, wantErr: core.ErrNilContext},
		{name: "nil parent root cannot substitute cwd", mutation: childRootNilRoot, wantErr: core.ErrFilestoreContract},
		{name: "unset path cannot select a directory", mutation: childRootZeroPath, wantErr: core.ErrFilestoreContract},
		{name: "closed parent root retains native closed refusal", mutation: childRootClosedParent, wantErr: core.ErrFilestoreSource, wantNative: fs.ErrClosed},
		{name: "held child keeps naming the opened directory after its parent moves", mutation: childRootParentRenamedAfterOpen},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			backstop, stop := newFilesystemBackstop(t.Context(), t, time.Minute)
			defer stop()
			directory := t.TempDir()
			outside := t.TempDir()
			root := requireTestRoot(t, directory)
			payload := []byte{0, 255, 8, 31}
			nested := filepath.Join(directory, "a", "b")
			if err := os.MkdirAll(nested, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(nested, "f"), payload, 0o600); err != nil {
				t.Fatal(err)
			}
			name := "a/b"
			entry := filepath.Join(directory, "entry")
			switch tc.entry {
			case childRootNested:
			case childRootConfinedLink, childRootOutsideLink, childRootDanglingLink:
				target := filepath.Join("a", "b")
				if tc.entry == childRootOutsideLink {
					target = outside
				}
				if tc.entry == childRootDanglingLink {
					target = "absent"
				}
				if err := os.Symlink(target, entry); err != nil {
					t.Fatal(err)
				}
				name = "entry"
			case childRootRegular:
				if err := os.WriteFile(entry, payload, 0o600); err != nil {
					t.Fatal(err)
				}
				name = "entry"
			case childRootFIFO:
				if err := syscall.Mkfifo(entry, 0o600); err != nil {
					t.Fatal(err)
				}
				name = "entry"
			case childRootMissing:
				name = "absent"
			default:
				t.Fatalf("entry = %d, want handled domain", tc.entry)
			}
			ctx, cancel := context.WithCancel(backstop)
			defer cancel()
			request := filestore.ChildRootRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, name)}}
			switch tc.mutation {
			case childRootUnchanged, childRootParentRenamedAfterOpen:
			case childRootCanceled:
				cancel()
			case childRootNilContext:
				ctx = nil
			case childRootNilRoot:
				request.Location.Root = nil
			case childRootZeroPath:
				request.Location.Path = core.RelativePath{}
			case childRootClosedParent:
				if err := root.Close(); err != nil {
					t.Fatal(err)
				}
			default:
				t.Fatalf("mutation = %d, want handled domain", tc.mutation)
			}
			child, err := filestore.OpenChildRoot(ctx, request)
			if child != nil {
				t.Cleanup(func() { _ = child.Close() })
			}
			if backstop.Err() != nil {
				t.Fatalf("acquisition outlived its backstop: %v", backstop.Err())
			}
			if !errors.Is(err, tc.wantErr) || tc.wantNative != nil && !errors.Is(err, tc.wantNative) {
				t.Fatalf("OpenChildRoot = %v/%v, want %v/native %v", child, err, tc.wantErr, tc.wantNative)
			}
			if tc.wantErr != nil {
				if child != nil {
					t.Fatalf("refused root = %v, want nil", child)
				}
				return
			}
			want, err := os.Stat(nested)
			if err != nil {
				t.Fatal(err)
			}
			if tc.mutation == childRootParentRenamedAfterOpen {
				if err := os.Rename(filepath.Join(directory, "a"), filepath.Join(directory, "a.moved")); err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(nested, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(nested, "f"), []byte("replacement"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			held, err := child.Stat(".")
			if err != nil || !os.SameFile(want, held) {
				t.Fatalf("child identity = %v/%v, want the opened directory %v", held, err, want)
			}
			data, err := child.ReadFile("f")
			if err != nil || !bytes.Equal(data, payload) {
				t.Fatalf("child read = %v/%v, want the opened directory's %v", data, err, payload)
			}
			if _, err := child.Open(filepath.Join("..", "b", "f")); err == nil {
				t.Fatal("child escaped through .., want confinement refusal")
			}
			if err := filestore.CloseRoot(filestore.RootHandleRequest{Root: child}); err != nil {
				t.Fatalf("CloseRoot = %v, want nil", err)
			}
			if _, err := child.Stat("."); !errors.Is(err, fs.ErrClosed) {
				t.Fatalf("closed child = %v, want closed", err)
			}
		})
	}
}

// A real device node: the directory-only nonblocking acquisition refuses it
// without opening it as a device.
func TestOpenChildRootRefusesNativeDevice(t *testing.T) {
	t.Parallel()
	devices := requireTestRoot(t, "/dev")
	child, err := filestore.OpenChildRoot(t.Context(), filestore.ChildRootRequest{Location: filestore.Location{Root: devices, Path: mustRelativePath(t, "null")}})
	if child != nil {
		t.Cleanup(func() { _ = child.Close() })
	}
	if child != nil || !errors.Is(err, core.ErrFilestoreSource) || !errors.Is(err, syscall.ENOTDIR) {
		t.Fatalf("OpenChildRoot(/dev/null) = %v/%v, want nil/source/ENOTDIR", child, err)
	}
}

type matchedMutation uint8

const (
	matchedUnchanged matchedMutation = iota
	matchedParentReplaced
	matchedParentRemoved
	matchedParentEscapes
	matchedClosedSource
	matchedFileReplacedInPlace
	matchedDifferentObservation
	matchedSymbolicObservation
	matchedDirectoryObservation
	matchedMissingSource
	matchedCanceled
	matchedNilContext
	matchedNilObservationRoot
	matchedNilSourceRoot
	matchedZeroObservationPath
	matchedZeroSourcePath
	matchedReplacedAfterOpen
)

func TestOpenMatchedCustodyReadNativeLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr    error
		wantNative error
		name       string
		want       []byte
		mutation   matchedMutation
	}{
		{name: "held child opens the freshly observed inode", want: []byte{0, 255, 8, 31}},
		{name: "replaced logical parent cannot admit the held directory's stale file", mutation: matchedParentReplaced, wantErr: core.ErrFilestoreSource, wantNative: fs.ErrInvalid},
		{name: "removed logical parent is refused by the fresh observation", mutation: matchedParentRemoved, wantErr: core.ErrFilestoreSource, wantNative: fs.ErrNotExist},
		{name: "original parent replaced by an escaping link is refused", mutation: matchedParentEscapes, wantErr: core.ErrFilestoreSource},
		{name: "closed held child retains native closed refusal", mutation: matchedClosedSource, wantErr: core.ErrFilestoreSource, wantNative: fs.ErrClosed},
		{name: "file replaced in the held directory is the current inode", mutation: matchedFileReplacedInPlace, want: []byte("in-place replacement")},
		{name: "observation of a different regular file is refused", mutation: matchedDifferentObservation, wantErr: core.ErrFilestoreSource, wantNative: fs.ErrInvalid},
		{name: "symbolic observation leaf is refused", mutation: matchedSymbolicObservation, wantErr: core.ErrFilestoreSource, wantNative: fs.ErrInvalid},
		{name: "directory observation is not regular", mutation: matchedDirectoryObservation, wantErr: core.ErrFilestoreSource, wantNative: fs.ErrInvalid},
		{name: "missing source remains missing", mutation: matchedMissingSource, wantErr: core.ErrFilestoreSource, wantNative: fs.ErrNotExist},
		{name: "cancelled read exposes no handle", mutation: matchedCanceled, wantErr: context.Canceled},
		{name: "nil context exposes no handle", mutation: matchedNilContext, wantErr: core.ErrNilContext},
		{name: "nil observation root cannot substitute cwd", mutation: matchedNilObservationRoot, wantErr: core.ErrFilestoreContract},
		{name: "nil source root cannot substitute cwd", mutation: matchedNilSourceRoot, wantErr: core.ErrFilestoreContract},
		{name: "unset observation path is refused", mutation: matchedZeroObservationPath, wantErr: core.ErrFilestoreContract},
		{name: "unset source path is refused", mutation: matchedZeroSourcePath, wantErr: core.ErrFilestoreContract},
		{name: "held inode survives later namespace replacement", mutation: matchedReplacedAfterOpen, want: []byte{0, 255, 8, 31}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			root := requireTestRoot(t, directory)
			payload := []byte{0, 255, 8, 31}
			nested := filepath.Join(directory, "a", "b")
			if err := os.MkdirAll(nested, 0o700); err != nil {
				t.Fatal(err)
			}
			subject := filepath.Join(nested, "f")
			neighbor := filepath.Join(nested, "g")
			if err := os.WriteFile(subject, payload, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(neighbor, []byte("neighbor"), 0o600); err != nil {
				t.Fatal(err)
			}
			child, err := filestore.OpenChildRoot(t.Context(), filestore.ChildRootRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, "a/b")}})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = child.Close() })
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			request := filestore.MatchedReadHandleRequest{
				Observation: filestore.Location{Root: root, Path: mustRelativePath(t, "a/b/f")},
				Source:      filestore.Location{Root: child, Path: mustRelativePath(t, "f")},
			}
			switch tc.mutation {
			case matchedUnchanged, matchedReplacedAfterOpen:
			case matchedParentReplaced:
				if err := os.Rename(filepath.Join(directory, "a"), filepath.Join(directory, "a.old")); err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(nested, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(subject, []byte("new namespace"), 0o600); err != nil {
					t.Fatal(err)
				}
			case matchedParentRemoved:
				if err := os.Rename(filepath.Join(directory, "a"), filepath.Join(directory, "a.old")); err != nil {
					t.Fatal(err)
				}
			case matchedParentEscapes:
				outside := t.TempDir()
				if err := os.MkdirAll(filepath.Join(outside, "b"), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(outside, "b", "f"), payload, 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(filepath.Join(directory, "a"), filepath.Join(directory, "a.old")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, filepath.Join(directory, "a")); err != nil {
					t.Fatal(err)
				}
			case matchedClosedSource:
				if err := child.Close(); err != nil {
					t.Fatal(err)
				}
			case matchedFileReplacedInPlace:
				if err := os.Rename(subject, filepath.Join(nested, "f.old")); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(subject, tc.want, 0o600); err != nil {
					t.Fatal(err)
				}
			case matchedDifferentObservation:
				request.Observation.Path = mustRelativePath(t, "a/b/g")
			case matchedSymbolicObservation:
				if err := os.Rename(subject, filepath.Join(nested, "f.old")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("f.old", subject); err != nil {
					t.Fatal(err)
				}
			case matchedDirectoryObservation:
				request.Observation.Path = mustRelativePath(t, "a/b")
			case matchedMissingSource:
				request.Source.Path = mustRelativePath(t, "absent")
			case matchedCanceled:
				cancel()
			case matchedNilContext:
				ctx = nil
			case matchedNilObservationRoot:
				request.Observation.Root = nil
			case matchedNilSourceRoot:
				request.Source.Root = nil
			case matchedZeroObservationPath:
				request.Observation.Path = core.RelativePath{}
			case matchedZeroSourcePath:
				request.Source.Path = core.RelativePath{}
			default:
				t.Fatalf("mutation = %d, want handled domain", tc.mutation)
			}
			before, beforeErr := os.Lstat(subject)
			got, gotErr := filestore.OpenMatchedCustodyRead(ctx, request)
			if got != nil {
				t.Cleanup(func() { _ = got.Close() })
			}
			if !errors.Is(gotErr, tc.wantErr) || tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) {
				t.Fatalf("OpenMatchedCustodyRead = %v/%v, want %v/native %v", got, gotErr, tc.wantErr, tc.wantNative)
			}
			if tc.wantErr != nil {
				if got != nil {
					t.Fatalf("refused handle = %v, want nil", got)
				}
			} else {
				held, err := got.Stat()
				if beforeErr != nil || err != nil || !os.SameFile(before, held) {
					t.Fatalf("held inode = %v/%v, want the freshly observed %v/%v", held, err, before, beforeErr)
				}
				if tc.mutation == matchedReplacedAfterOpen {
					if err := os.Rename(subject, filepath.Join(nested, "f.old")); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(subject, []byte("replacement"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				data, err := io.ReadAll(io.LimitReader(got, int64(len(tc.want)+1)))
				if err != nil || !bytes.Equal(data, tc.want) {
					t.Fatalf("held bytes = %v/%v, want %v", data, err, tc.want)
				}
				if n, err := got.Write([]byte("forbidden")); n != 0 || err == nil {
					t.Fatalf("read-only write = %d/%v, want native refusal", n, err)
				}
				if err := got.Close(); err != nil {
					t.Fatal(err)
				}
			}
			data, err := os.ReadFile(neighbor)
			if err != nil || !bytes.Equal(data, []byte("neighbor")) {
				t.Fatalf("neighbor bytes = %v/%v, want unchanged", data, err)
			}
		})
	}
}

// Only the owned subprocess counts its descriptors: no parallel test shares
// the process, so every acquisition, refusal and mismatch must return the
// table to exactly its prior size.
func TestChildRootDescriptorCleanupNative(t *testing.T) {
	if os.Getenv(childRootDescriptorEnvironment) == childRootDescriptorMarker {
		runChildRootDescriptorCleanup(t)
		return
	}
	t.Parallel()
	ctx, cancel := newFilesystemBackstop(t.Context(), t, time.Minute)
	defer cancel()
	symbol := runtime.FuncForPC(reflect.ValueOf(TestChildRootDescriptorCleanupNative).Pointer()).Name()
	name := symbol[strings.LastIndexByte(symbol, '.')+1:]
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^"+name+"$", "-test.count=1", "-test.v")
	command.Env = append(os.Environ(), childRootDescriptorEnvironment+"="+childRootDescriptorMarker)
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	err := command.Run()
	t.Logf("child-root descriptor subprocess output (base64): %s", base64.StdEncoding.EncodeToString(output.Bytes()))
	if err != nil || ctx.Err() != nil || !bytes.Contains(output.Bytes(), []byte("CHILD_ROOT_DESCRIPTORS balanced")) {
		t.Fatalf("descriptor subprocess = (%v,%v), want a complete balanced probe", err, ctx.Err())
	}
}

func runChildRootDescriptorCleanup(t *testing.T) {
	t.Helper()
	directory := t.TempDir()
	root := requireTestRoot(t, directory)
	if err := os.MkdirAll(filepath.Join(directory, "a", "b"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"a/b/f", "a/b/g", "regular"} {
		if err := os.WriteFile(filepath.Join(directory, file), []byte(file), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := syscall.Mkfifo(filepath.Join(directory, "fifo"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(directory, "outside")); err != nil {
		t.Fatal(err)
	}
	location := func(root *os.Root, path string) filestore.Location {
		return filestore.Location{Root: root, Path: mustRelativePath(t, path)}
	}
	round := func() {
		child, err := filestore.OpenChildRoot(t.Context(), filestore.ChildRootRequest{Location: location(root, "a/b")})
		if err != nil {
			t.Fatal(err)
		}
		for _, refused := range []string{"regular", "fifo", "outside", "absent"} {
			if got, err := filestore.OpenChildRoot(t.Context(), filestore.ChildRootRequest{Location: location(root, refused)}); got != nil || !errors.Is(err, core.ErrFilestoreSource) {
				t.Fatalf("OpenChildRoot(%s) = %v/%v, want nil/source", refused, got, err)
			}
		}
		file, err := filestore.OpenMatchedCustodyRead(t.Context(), filestore.MatchedReadHandleRequest{Observation: location(root, "a/b/f"), Source: location(child, "f")})
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		if got, err := filestore.OpenMatchedCustodyRead(t.Context(), filestore.MatchedReadHandleRequest{Observation: location(root, "a/b/g"), Source: location(child, "f")}); got != nil || !errors.Is(err, fs.ErrInvalid) {
			t.Fatalf("mismatched read = %v/%v, want nil/invalid", got, err)
		}
		if err := filestore.CloseRoot(filestore.RootHandleRequest{Root: child}); err != nil {
			t.Fatal(err)
		}
	}
	// Warm every native path before taking the baseline.
	round()
	before := childRootDescriptors(t)
	for range childRootDescriptorRounds {
		round()
	}
	after := childRootDescriptors(t)
	if after != before {
		t.Fatalf("descriptors %d before and %d after %d rounds, want balanced", before, after, childRootDescriptorRounds)
	}
	t.Logf("CHILD_ROOT_DESCRIPTORS balanced before=%d after=%d rounds=%d", before, after, childRootDescriptorRounds)
}

func childRootDescriptors(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir("/dev/fd")
	if err != nil {
		t.Fatal(err)
	}
	return len(entries)
}
