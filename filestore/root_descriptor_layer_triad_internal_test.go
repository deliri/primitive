//go:build darwin || linux

package filestore

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

type rootDescriptorFixture uint8

const (
	rootDescriptorEmpty rootDescriptorFixture = iota
	rootDescriptorPopulated
	rootDescriptorRenamed
	rootDescriptorReleaseFile
	rootDescriptorReleaseRoot
	rootDescriptorNil
	rootDescriptorClosed
	rootDescriptorRegular
)

// This leaf borrows a Go file and creates an independently owned Go root.
// These rows pressure object identity and each ownership transition, including
// the empty-directory neutral case. They do not impose new path semantics.
func TestRootFromHeldDescriptorLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name           string
		fixture        rootDescriptorFixture
		wantErr        error
		wantPathErr    bool
		wantChildren   int
		wantControlErr bool
	}{
		{name: "empty directory creates no children", fixture: rootDescriptorEmpty},
		{name: "binary child stays under the borrowed directory", fixture: rootDescriptorPopulated, wantChildren: 1},
		{name: "renamed descriptor cannot reopen the replacement path", fixture: rootDescriptorRenamed, wantChildren: 1},
		{name: "releasing acquisition file does not invalidate derived root", fixture: rootDescriptorReleaseFile, wantChildren: 1},
		{name: "releasing derived root does not consume borrowed file", fixture: rootDescriptorReleaseRoot, wantChildren: 1},
		{name: "nil acquisition cannot fabricate a root", fixture: rootDescriptorNil, wantErr: fs.ErrInvalid},
		{name: "closed acquisition cannot resurrect a descriptor", fixture: rootDescriptorClosed, wantControlErr: true},
		{name: "regular file cannot become a directory capability", fixture: rootDescriptorRegular, wantPathErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			parent := t.TempDir()
			original := filepath.Join(parent, "original")
			if err := os.Mkdir(original, 0o700); err != nil {
				t.Fatal(err)
			}
			payload := []byte{0, 255, '\n', 1}
			if tc.wantChildren != 0 {
				if err := os.WriteFile(filepath.Join(original, "child"), payload, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var file *os.File
			if tc.fixture != rootDescriptorNil {
				path := original
				if tc.fixture == rootDescriptorRegular {
					path = filepath.Join(parent, "regular")
					if err := os.WriteFile(path, payload, 0o600); err != nil {
						t.Fatal(err)
					}
				}
				var err error
				file, err = os.Open(path)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := file.Close(); err != nil && !errors.Is(err, fs.ErrClosed) {
						t.Error(err)
					}
				})
			}
			want, err := os.Stat(original)
			if err != nil {
				t.Fatal(err)
			}
			if tc.fixture == rootDescriptorRenamed {
				if err := os.Rename(original, filepath.Join(parent, "retained")); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(original, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(original, "child"), []byte{42}, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if tc.fixture == rootDescriptorClosed {
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
			}
			wantErr := tc.wantErr
			if tc.wantControlErr {
				connection, err := file.SyscallConn()
				if err != nil {
					t.Fatal(err)
				}
				called := false
				wantErr = connection.Control(func(uintptr) { called = true })
				if called || wantErr == nil {
					t.Fatalf("Go closed control = (%t,%v), want no callback and native refusal", called, wantErr)
				}
			}
			root, gotErr := rootFromDirectoryFile(file)
			if root != nil {
				t.Cleanup(func() {
					if err := root.Close(); err != nil && !errors.Is(err, fs.ErrClosed) {
						t.Error(err)
					}
				})
			}
			if wantErr != nil || tc.wantPathErr {
				var native *fs.PathError
				if root != nil || gotErr == nil || (wantErr != nil && !errors.Is(gotErr, wantErr)) || (tc.wantPathErr && !errors.As(gotErr, &native)) {
					t.Fatalf("derived root/error = (%v,%v), want nil and identity %v/native path refusal %t", root, gotErr, wantErr, tc.wantPathErr)
				}
				if tc.fixture == rootDescriptorRegular {
					got, err := file.Stat()
					if err != nil || !got.Mode().IsRegular() || got.Size() != int64(len(payload)) {
						t.Fatalf("rejected borrowed file = (%v,%v), want live unchanged regular file", got, err)
					}
				}
				return
			}
			if gotErr != nil || root == nil {
				t.Fatalf("derived root/error = (%v,%v), want live root and nil", root, gotErr)
			}
			if tc.fixture == rootDescriptorReleaseFile {
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
			}
			got, err := root.Stat(".")
			if err != nil || !os.SameFile(got, want) {
				t.Fatalf("root identity = (%v,%v), want original inode %v", got, err, want)
			}
			if tc.wantChildren != 0 {
				got, err := root.ReadFile("child")
				if err != nil || !bytes.Equal(got, payload) {
					t.Fatalf("root child = (%v,%v), want original bytes %v", got, err, payload)
				}
			}
			if tc.fixture == rootDescriptorReleaseRoot {
				if err := root.Close(); err != nil {
					t.Fatal(err)
				}
				if _, err := root.Stat("."); !errors.Is(err, fs.ErrClosed) {
					t.Fatalf("released root stat = %v, want closed", err)
				}
			}
			if tc.fixture != rootDescriptorReleaseFile {
				got, err := file.Stat()
				if err != nil || !os.SameFile(got, want) {
					t.Fatalf("borrowed file identity = (%v,%v), want live original inode", got, err)
				}
				entries, err := file.ReadDir(-1)
				if err != nil || len(entries) != tc.wantChildren {
					t.Fatalf("directory entries = (%v,%v), want %d unchanged children", entries, err, tc.wantChildren)
				}
			}
			if tc.fixture == rootDescriptorRenamed {
				got, err := os.ReadFile(filepath.Join(original, "child"))
				if err != nil || !bytes.Equal(got, []byte{42}) {
					t.Fatalf("replacement child = (%v,%v), want untouched replacement", got, err)
				}
			}
		})
	}
}
