package filestore_test

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// childRootFuzzDepthMaximum bounds generated nesting.
const childRootFuzzDepthMaximum = 6

// Arbitrary bytes at a bounded nesting depth either arrive through the child
// root with the native directory's identity and exact content, or an escaping
// link is refused with no root.
func FuzzOpenChildRootConfinedDirectoryIdentity(f *testing.F) {
	f.Add([]byte{0, 255, 8, 31}, uint8(1), false)
	f.Add([]byte{}, uint8(5), false)
	f.Add([]byte("escape"), uint8(2), true)
	f.Fuzz(func(t *testing.T, data []byte, depth uint8, outside bool) {
		directory := t.TempDir()
		root := requireTestRoot(t, directory)
		components := make([]string, 1+int(depth)%childRootFuzzDepthMaximum)
		for i := range components {
			components[i] = "d" + string(rune('0'+i))
		}
		nested := filepath.Join(append([]string{directory}, components...)...)
		if err := os.MkdirAll(nested, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(nested, "f"), data, 0o600); err != nil {
			t.Fatal(err)
		}
		name := strings.Join(components, "/")
		if outside {
			if err := os.Symlink(t.TempDir(), filepath.Join(directory, "link")); err != nil {
				t.Fatal(err)
			}
			name = "link"
		}
		child, err := filestore.OpenChildRoot(t.Context(), filestore.ChildRootRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, name)}})
		if child != nil {
			t.Cleanup(func() { _ = child.Close() })
		}
		if outside {
			if child != nil || !errors.Is(err, core.ErrFilestoreSource) {
				t.Fatalf("escaping link = %v/%v, want nil/source", child, err)
			}
			return
		}
		if err != nil || child == nil {
			t.Fatalf("nested acquisition = %v/%v, want root/nil", child, err)
		}
		want, err := os.Stat(nested)
		if err != nil {
			t.Fatal(err)
		}
		held, err := child.Stat(".")
		if err != nil || !os.SameFile(want, held) {
			t.Fatalf("child identity = %v/%v, want %v", held, err, want)
		}
		file, err := child.Open("f")
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.New()
		n, readErr := io.Copy(hash, file)
		closeErr := file.Close()
		sum := sha256.Sum256(data)
		if readErr != nil || closeErr != nil || n != int64(len(data)) || !bytes.Equal(hash.Sum(nil), sum[:]) {
			t.Fatalf("child content = %d/%v/%v, want exact hash/extent", n, readErr, closeErr)
		}
		if err := filestore.CloseRoot(filestore.RootHandleRequest{Root: child}); err != nil {
			t.Fatalf("CloseRoot = %v, want nil", err)
		}
	})
}

// Arbitrary bytes read through a held child either return the freshly observed
// inode's exact content, or, once the logical parent was replaced, the held
// directory's stale file is refused with no handle and both trees keep their
// bytes.
func FuzzOpenMatchedCustodyReadExactInode(f *testing.F) {
	f.Add([]byte{0, 255, 8, 31}, []byte("new"), false)
	f.Add([]byte{0, 255, 8, 31}, []byte("new"), true)
	f.Add([]byte{}, []byte{}, true)
	f.Fuzz(func(t *testing.T, data, replacement []byte, replaceParent bool) {
		directory := t.TempDir()
		root := requireTestRoot(t, directory)
		parent := filepath.Join(directory, "a")
		if err := os.Mkdir(parent, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(parent, "f"), data, 0o600); err != nil {
			t.Fatal(err)
		}
		child, err := filestore.OpenChildRoot(t.Context(), filestore.ChildRootRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, "a")}})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = child.Close() })
		current := data
		if replaceParent {
			if err := os.Rename(parent, filepath.Join(directory, "a.old")); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(parent, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(parent, "f"), replacement, 0o600); err != nil {
				t.Fatal(err)
			}
			current = replacement
		}
		before, err := os.Lstat(filepath.Join(parent, "f"))
		if err != nil {
			t.Fatal(err)
		}
		got, gotErr := filestore.OpenMatchedCustodyRead(t.Context(), filestore.MatchedReadHandleRequest{
			Observation: filestore.Location{Root: root, Path: mustRelativePath(t, "a/f")},
			Source:      filestore.Location{Root: child, Path: mustRelativePath(t, "f")},
		})
		if got != nil {
			t.Cleanup(func() { _ = got.Close() })
		}
		if replaceParent {
			if got != nil || !errors.Is(gotErr, core.ErrFilestoreSource) || !errors.Is(gotErr, fs.ErrInvalid) {
				t.Fatalf("stale held file = %v/%v, want nil/source/invalid", got, gotErr)
			}
		} else {
			if got == nil || gotErr != nil {
				t.Fatalf("matched acquisition = %v/%v, want handle/nil", got, gotErr)
			}
			held, statErr := got.Stat()
			hash := sha256.New()
			n, readErr := io.Copy(hash, got)
			closeErr := got.Close()
			sum := sha256.Sum256(data)
			if statErr != nil || !os.SameFile(before, held) || readErr != nil || closeErr != nil || n != int64(len(data)) || !bytes.Equal(hash.Sum(nil), sum[:]) {
				t.Fatalf("held content = %v/%v/%d/%v/%v, want the observed inode's exact hash/extent", held, statErr, n, readErr, closeErr)
			}
		}
		native, err := os.ReadFile(filepath.Join(parent, "f"))
		if err != nil || !bytes.Equal(native, current) {
			t.Fatalf("current namespace bytes = %v/%v, want unchanged", native, err)
		}
		if replaceParent {
			stale, err := os.ReadFile(filepath.Join(directory, "a.old", "f"))
			if err != nil || !bytes.Equal(stale, data) {
				t.Fatalf("held directory bytes = %v/%v, want unchanged", stale, err)
			}
		}
	})
}
