package filestore_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

type rootIdentityFixture uint8

const (
	rootIdentityOriginal rootIdentityFixture = iota
	rootIdentityAlias
	rootIdentityRenamedMissing
	rootIdentityRenamedOwned
	rootIdentityReplacedSameBytes
	rootIdentityReplacedFile
	rootIdentityReplacedLinkToOwned
	rootIdentityNil
	rootIdentityClosed
	rootIdentityZeroPath
	rootIdentityMissingParent
	rootIdentityBelowFile
	rootIdentityLoop
)

func TestRootIdentityNativeCustodyLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                  string
		fixture               rootIdentityFixture
		wantErr, wantExcluded error
		native                bool
	}{
		{name: "descriptor-backed root proves its original native directory", fixture: rootIdentityOriginal},
		{name: "different symlink spelling retains the same root identity", fixture: rootIdentityAlias},
		{name: "renamed-away original path cannot claim current custody", fixture: rootIdentityRenamedMissing, wantErr: core.ErrFilestoreSource, native: true},
		{name: "renamed owned directory remains provable under its new name", fixture: rootIdentityRenamedOwned},
		{name: "same names modes and bytes cannot replace native directory identity", fixture: rootIdentityReplacedSameBytes, wantErr: core.ErrFilestoreContract, wantExcluded: core.ErrFilestoreSource},
		{name: "regular file at the former root name is a different object", fixture: rootIdentityReplacedFile, wantErr: core.ErrFilestoreContract, wantExcluded: core.ErrFilestoreSource},
		{name: "root identity follows a final link to the exact held directory", fixture: rootIdentityReplacedLinkToOwned},
		{name: "nil root cannot claim any directory identity", fixture: rootIdentityNil, wantErr: core.ErrFilestoreContract, wantExcluded: core.ErrFilestoreSource},
		{name: "closed rooted capability retains native closure", fixture: rootIdentityClosed, wantErr: core.ErrFilestoreSource, native: true},
		{name: "zero absolute path cannot borrow the root diagnostic name", fixture: rootIdentityZeroPath, wantErr: core.ErrFilestoreContract, wantExcluded: core.ErrFilestoreSource},
		{name: "absent parent preserves native absence", fixture: rootIdentityMissingParent, wantErr: core.ErrFilestoreSource, native: true},
		{name: "regular parent preserves native not-directory refusal", fixture: rootIdentityBelowFile, wantErr: core.ErrFilestoreSource, native: true},
		{name: "looping absolute alias preserves native link refusal", fixture: rootIdentityLoop, wantErr: core.ErrFilestoreSource, native: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			container := t.TempDir()
			directory := filepath.Join(container, "root")
			archive := filepath.Join(container, "archive")
			payload := []byte{0, 255, 1}
			if err := os.Mkdir(directory, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, "child"), payload, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(container, "neighbor"), payload, 0o600); err != nil {
				t.Fatal(err)
			}
			path, err := core.ParseAbsolutePath(directory)
			if err != nil {
				t.Fatal(err)
			}
			root, err := filestore.OpenRoot(t.Context(), path)
			if err != nil {
				t.Fatal(err)
			}
			if tc.fixture != rootIdentityClosed {
				t.Cleanup(func() {
					if err := root.Close(); err != nil {
						t.Error(err)
					}
				})
			}
			original, err := os.Stat(directory)
			if err != nil {
				t.Fatal(err)
			}
			childBefore, err := root.Stat("child")
			if err != nil {
				t.Fatal(err)
			}
			candidate := root
			candidatePath := directory
			switch tc.fixture {
			case rootIdentityOriginal:
			case rootIdentityAlias:
				candidatePath = filepath.Join(container, "alias")
				if err := os.Symlink("root", candidatePath); err != nil {
					t.Fatal(err)
				}
			case rootIdentityRenamedMissing, rootIdentityRenamedOwned, rootIdentityReplacedSameBytes, rootIdentityReplacedFile, rootIdentityReplacedLinkToOwned:
				if err := os.Rename(directory, archive); err != nil {
					t.Fatal(err)
				}
				switch tc.fixture {
				case rootIdentityRenamedOwned:
					candidatePath = archive
				case rootIdentityReplacedSameBytes:
					if err := os.Mkdir(directory, 0o700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(directory, "child"), payload, 0o600); err != nil {
						t.Fatal(err)
					}
					foreign, err := os.Stat(directory)
					if err != nil || os.SameFile(original, foreign) {
						t.Fatalf("foreign directory = (%v,%v), want distinct inode", foreign, err)
					}
				case rootIdentityReplacedFile:
					if err := os.WriteFile(directory, payload, 0o600); err != nil {
						t.Fatal(err)
					}
				case rootIdentityReplacedLinkToOwned:
					if err := os.Symlink("archive", directory); err != nil {
						t.Fatal(err)
					}
				}
			case rootIdentityNil:
				candidate = nil
			case rootIdentityClosed:
				if err := root.Close(); err != nil {
					t.Fatal(err)
				}
			case rootIdentityZeroPath:
			case rootIdentityMissingParent:
				candidatePath = filepath.Join(container, "missing", "child")
			case rootIdentityBelowFile:
				candidatePath = filepath.Join(container, "neighbor", "child")
			case rootIdentityLoop:
				candidatePath = filepath.Join(container, "loop")
				if err := os.Symlink("loop", candidatePath); err != nil {
					t.Fatal(err)
				}
			}
			path = core.AbsolutePath{}
			if tc.fixture != rootIdentityZeroPath {
				path, err = core.ParseAbsolutePath(candidatePath)
				if err != nil {
					t.Fatal(err)
				}
			}
			before, err := removalFixtureSnapshot(container)
			if err != nil {
				t.Fatal(err)
			}
			infos := make([]fs.FileInfo, len(before))
			for i, entry := range before {
				infos[i], err = os.Lstat(filepath.Join(container, entry.name))
				if err != nil {
					t.Fatal(err)
				}
			}
			var wantNative error
			if tc.native {
				if tc.fixture == rootIdentityClosed {
					_, wantNative = root.Stat(".")
				} else {
					_, wantNative = os.Stat(candidatePath)
				}
				var native *fs.PathError
				if !errors.As(wantNative, &native) {
					t.Fatalf("native refusal = %v, want PathError", wantNative)
				}
				wantNative = native.Err
			}
			gotErr := filestore.ValidateRootIdentity(candidate, path)
			if !errors.Is(gotErr, tc.wantErr) || (tc.wantExcluded != nil && errors.Is(gotErr, tc.wantExcluded)) {
				t.Fatalf("root identity = %v, want %v without %v", gotErr, tc.wantErr, tc.wantExcluded)
			}
			if tc.native {
				var native *fs.PathError
				if !errors.Is(gotErr, wantNative) || !errors.As(gotErr, &native) {
					t.Fatalf("native cause = %v, want %v and PathError", gotErr, wantNative)
				}
			}
			if tc.fixture == rootIdentityClosed {
				if _, err := root.Stat("."); !errors.Is(err, fs.ErrClosed) {
					t.Fatalf("closed caller root = %v, want closed", err)
				}
			} else {
				after, err := root.Stat(".")
				if err != nil || !os.SameFile(original, after) || original.Mode() != after.Mode() || original.ModTime().UnixNano() != after.ModTime().UnixNano() {
					t.Fatalf("caller-owned root = (%v,%v), want unchanged %v", after, err, original)
				}
				childAfter, err := root.Stat("child")
				if err != nil || !os.SameFile(childBefore, childAfter) {
					t.Fatalf("held child = (%v,%v), want unchanged inode", childAfter, err)
				}
				child, err := root.ReadFile("child")
				if err != nil || !bytes.Equal(child, payload) {
					t.Fatalf("held child bytes = (%v,%v), want %v", child, err, payload)
				}
			}
			after, err := removalFixtureSnapshot(container)
			if err != nil {
				t.Fatal(err)
			}
			if len(after) != len(before) {
				t.Fatalf("namespace = %+v, want unchanged %+v", after, before)
			}
			for i, entry := range before {
				if after[i].name != entry.name || after[i].mode != entry.mode || after[i].target != entry.target || !bytes.Equal(after[i].data, entry.data) {
					t.Fatalf("entry %d = %+v, want unchanged %+v", i, after[i], entry)
				}
				info, err := os.Lstat(filepath.Join(container, entry.name))
				if err != nil || !os.SameFile(infos[i], info) || infos[i].ModTime().UnixNano() != info.ModTime().UnixNano() {
					t.Fatalf("retained metadata %s = (%v,%v), want %v", entry.name, info, err, infos[i])
				}
			}
		})
	}
}
