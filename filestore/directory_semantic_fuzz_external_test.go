package filestore_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// The booleans change real acquisition and namespace facts, while fuzz bytes
// change the exact child identity and content observed through the capability.
func FuzzDirectoryAcquisitionNamespaceCustody(f *testing.F) {
	emitted, err := filestore.FilestoreRoundTripSeedForTest(f.Context(), f.TempDir())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(emitted, true, false, false)
	for _, seed := range []struct {
		payload []byte
		held    bool
		link    bool
		rename  bool
	}{
		{payload: nil, held: true},
		{payload: []byte{0, 255}, held: true, rename: true},
		{payload: []byte{0, 255}, held: true, link: true},
		{payload: nil},
		{payload: []byte{0, 255}, rename: true},
		{payload: []byte{0, 255}, link: true, rename: true},
	} {
		f.Add(seed.payload, seed.held, seed.link, seed.rename)
	}
	f.Fuzz(func(t *testing.T, payload []byte, held, link, rename bool) {
		payload = payload[:min(len(payload), 4096)]
		parent := t.TempDir()
		original := filepath.Join(parent, "original")
		if err := os.Mkdir(original, 0o700); err != nil {
			t.Fatal(err)
		}
		child := "child-" + hex.EncodeToString(payload[:min(len(payload), 16)])
		if err := os.WriteFile(filepath.Join(original, child), payload, 0o600); err != nil {
			t.Fatal(err)
		}
		want, err := os.Stat(original)
		if err != nil {
			t.Fatal(err)
		}
		wantChild, err := os.Stat(filepath.Join(original, child))
		if err != nil {
			t.Fatal(err)
		}
		path := original
		if link {
			path = filepath.Join(parent, "link")
			if err := os.Symlink("original", path); err != nil {
				t.Fatal(err)
			}
		}
		request, err := core.ParseAbsolutePath(path)
		if err != nil {
			t.Fatal(err)
		}
		var directory *filestore.HeldDirectory
		var root *os.Root
		var gotErr error
		if held {
			directory, gotErr = filestore.OpenDirectory(t.Context(), request)
		} else {
			root, gotErr = filestore.OpenRoot(t.Context(), request)
		}
		if directory != nil {
			t.Cleanup(func() {
				if err := directory.Close(); err != nil {
					t.Error(err)
				}
			})
		}
		if root != nil {
			t.Cleanup(func() {
				if err := root.Close(); err != nil {
					t.Error(err)
				}
			})
		}
		if held && link {
			var native *fs.PathError
			if directory != nil || !errors.Is(gotErr, core.ErrFilestoreSource) || !errors.As(gotErr, &native) {
				t.Fatalf("held link = (%v,%v), want no capability and source/native refusal", directory, gotErr)
			}
		} else if gotErr != nil || (held && directory == nil) || (!held && root == nil) {
			t.Fatalf("acquisition = (%v,%v,%v), want exact live capability", directory, root, gotErr)
		}
		retained := original
		if rename {
			retained = filepath.Join(parent, "retained")
			if err := os.Rename(original, retained); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(original, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(original, child), []byte{42}, 0o600); err != nil {
				t.Fatal(err)
			}
			changed, err := os.Stat(original)
			if err != nil || os.SameFile(want, changed) {
				t.Fatalf("replacement inode = (%v,%v), want distinct live inode", changed, err)
			}
		}
		if directory != nil {
			file, err := directory.File()
			if err != nil {
				t.Fatal(err)
			}
			identity, err := directory.Filesystem()
			if err != nil || identity.Validate() != nil {
				t.Fatalf("filesystem = (%v,%v), want observed identity", identity, err)
			}
			got, err := file.Stat()
			if err != nil || !os.SameFile(want, got) {
				t.Fatalf("held inode = (%v,%v), want original directory", got, err)
			}
			// Go Readdir observes metadata relative to this live descriptor.
			// ReadDir's lazy DirEntry.Info can resolve the original pathname.
			entries, err := file.Readdir(-1)
			if err != nil || len(entries) != 1 || entries[0].Name() != child {
				t.Fatalf("held entries = (%v,%v), want exact child %q", entries, err, child)
			}
			gotChild := entries[0]
			if !os.SameFile(wantChild, gotChild) || gotChild.Size() != int64(len(payload)) {
				t.Fatalf("held child = %v, want original inode and extent %d", gotChild, len(payload))
			}
		}
		if root != nil {
			got, err := root.Stat(".")
			if err != nil || !os.SameFile(want, got) {
				t.Fatalf("root inode = (%v,%v), want original directory", got, err)
			}
			gotBytes, err := root.ReadFile(child)
			if err != nil || !bytes.Equal(gotBytes, payload) {
				t.Fatalf("root bytes = (%v,%v), want original payload %v", gotBytes, err, payload)
			}
		}
		gotBytes, err := os.ReadFile(filepath.Join(retained, child))
		if err != nil || !bytes.Equal(gotBytes, payload) {
			t.Fatalf("retained bytes = (%v,%v), want untouched %v", gotBytes, err, payload)
		}
		if rename {
			gotBytes, err := os.ReadFile(filepath.Join(original, child))
			if err != nil || !bytes.Equal(gotBytes, []byte{42}) {
				t.Fatalf("replacement bytes = (%v,%v), want untouched replacement", gotBytes, err)
			}
		}
		if link {
			got, err := os.Readlink(path)
			if err != nil || got != "original" {
				t.Fatalf("link = (%q,%v), want retained original target", got, err)
			}
		}
		entries, err := os.ReadDir(parent)
		wantEntries := 1
		if rename {
			wantEntries++
		}
		if link {
			wantEntries++
		}
		if err != nil || len(entries) != wantEntries {
			t.Fatalf("namespace = (%v,%v), want %d exact entries", entries, err, wantEntries)
		}
	})
}
