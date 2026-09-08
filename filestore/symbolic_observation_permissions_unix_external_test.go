//go:build darwin || linux

package filestore_test

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestSymbolicObservationNativeSearchPermissionLayerTriad(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root bypasses native owner search refusals")
	}
	for _, tc := range []struct {
		mode            fs.FileMode
		wantObservation bool
	}{
		{mode: 0o000}, {mode: 0o100, wantObservation: true}, {mode: 0o200}, {mode: 0o300, wantObservation: true},
		{mode: 0o400}, {mode: 0o500, wantObservation: true}, {mode: 0o600}, {mode: 0o700, wantObservation: true},
	} {
		t.Run(fmt.Sprintf("owner %03o preserves native search capability", tc.mode), func(t *testing.T) {
			t.Parallel()
			container := t.TempDir()
			directory := filepath.Join(container, "parent")
			if err := os.Mkdir(directory, 0o700); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Chmod(directory, 0o700); err != nil {
					t.Error(err)
				}
			})
			payload := []byte{0, 255, 1, 127}
			if err := os.WriteFile(filepath.Join(directory, "file"), payload, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("file", filepath.Join(directory, "link")); err != nil {
				t.Fatal(err)
			}
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := root.Close(); err != nil {
					t.Error(err)
				}
			})
			path, err := core.ParseRelativePath("link")
			if err != nil {
				t.Fatal(err)
			}
			absolute, err := core.ParseAbsolutePath(filepath.Join(directory, "link"))
			if err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(directory)
			if err != nil {
				t.Fatal(err)
			}
			linkBefore, err := os.Lstat(absolute.String())
			if err != nil {
				t.Fatal(err)
			}
			fileBefore, err := os.Stat(filepath.Join(directory, "file"))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(directory, tc.mode); err != nil {
				t.Fatal(err)
			}
			nativeTarget, nativeLinkErr := root.Readlink(path.String())
			nativeCanonical, nativeCanonicalErr := filepath.EvalSymlinks(absolute.String())
			gotTarget, linkErr := filestore.ReadSymbolicLink(t.Context(), filestore.Location{Root: root, Path: path})
			gotCanonical, canonicalErr := filestore.Canonicalize(t.Context(), absolute)
			if tc.wantObservation {
				if nativeLinkErr != nil || nativeCanonicalErr != nil || nativeTarget != "file" || linkErr != nil || canonicalErr != nil || gotTarget.Validate() != nil || gotTarget.String() != nativeTarget || gotCanonical.Validate() != nil || gotCanonical.String() != nativeCanonical {
					t.Fatalf("native observations = (%q,%v,%q,%v), got (%q,%v,%v,%v), want exact admitted results", nativeTarget, nativeLinkErr, nativeCanonical, nativeCanonicalErr, gotTarget.String(), linkErr, gotCanonical, canonicalErr)
				}
			} else {
				var nativeLink, gotLink, nativeCanonicalPath, gotCanonicalPath *os.PathError
				if !errors.Is(nativeLinkErr, fs.ErrPermission) || !errors.Is(linkErr, core.ErrFilestoreSource) || !errors.As(nativeLinkErr, &nativeLink) || !errors.As(linkErr, &gotLink) || !errors.Is(linkErr, nativeLink.Err) || gotLink.Op != nativeLink.Op || gotLink.Path != nativeLink.Path || gotTarget != (filestore.SymbolicLinkTarget{}) {
					t.Fatalf("link refusal = (%q,%v), want zero and native permission %v", gotTarget.String(), linkErr, nativeLinkErr)
				}
				if !errors.Is(nativeCanonicalErr, fs.ErrPermission) || !errors.Is(canonicalErr, core.ErrFilestoreSource) || !errors.As(nativeCanonicalErr, &nativeCanonicalPath) || !errors.As(canonicalErr, &gotCanonicalPath) || !errors.Is(canonicalErr, nativeCanonicalPath.Err) || gotCanonicalPath.Op != nativeCanonicalPath.Op || gotCanonicalPath.Path != nativeCanonicalPath.Path || gotCanonical != (core.AbsolutePath{}) {
					t.Fatalf("canonical refusal = (%v,%v), want zero and native permission %v", gotCanonical, canonicalErr, nativeCanonicalErr)
				}
			}
			after, err := os.Stat(directory)
			if err != nil || !os.SameFile(before, after) || after.Mode().Perm() != tc.mode || before.ModTime().UnixNano() != after.ModTime().UnixNano() {
				t.Fatalf("parent after observation = (%v,%v), want original inode/time and mode %#o", after, err, tc.mode)
			}
			if err := os.Chmod(directory, 0o700); err != nil {
				t.Fatal(err)
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != 2 || entries[0].Name() != "file" || entries[1].Name() != "link" {
				t.Fatalf("retained namespace = (%v,%v), want file and link only", entries, err)
			}
			linkAfter, err := os.Lstat(absolute.String())
			if err != nil || !os.SameFile(linkBefore, linkAfter) || linkBefore.Mode() != linkAfter.Mode() || linkBefore.ModTime().UnixNano() != linkAfter.ModTime().UnixNano() {
				t.Fatalf("retained link = (%v,%v), want %v", linkAfter, err, linkBefore)
			}
			target, err := os.Readlink(absolute.String())
			if err != nil || target != "file" {
				t.Fatalf("retained target = (%q,%v), want file", target, err)
			}
			fileAfter, err := os.Stat(filepath.Join(directory, "file"))
			if err != nil || !os.SameFile(fileBefore, fileAfter) || fileBefore.Mode() != fileAfter.Mode() || fileBefore.ModTime().UnixNano() != fileAfter.ModTime().UnixNano() {
				t.Fatalf("retained file = (%v,%v), want %v", fileAfter, err, fileBefore)
			}
			data, err := os.ReadFile(filepath.Join(directory, "file"))
			if err != nil || !bytes.Equal(data, payload) {
				t.Fatalf("retained bytes = (%v,%v), want %v", data, err, payload)
			}
		})
	}
}
