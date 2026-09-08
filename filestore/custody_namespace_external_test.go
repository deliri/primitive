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
	"github.com/deliri/primitive/v2026/temporal"
)

type custodyNamespace uint8

const (
	custodyNamespaceOriginal custodyNamespace = iota
	custodyNamespaceHardLink
	custodyNamespaceRelativeLink
	custodyNamespaceAbsoluteLink
	custodyNamespaceDanglingLink
	custodyNamespaceSelfLink
	custodyNamespaceDirectory
	custodyNamespaceAbsent
)

func TestCustodyExactRegularNamespaceLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		entry      custodyNamespace
		payload    []byte
		mode       fs.FileMode
		wantNative error
	}{
		{name: "binary regular file retains bytes and inode", payload: []byte{0, 255, 7}, mode: 0o600},
		{name: "empty regular file remains an exact empty fact", mode: 0o600},
		{name: "read only regular file needs no write handle", payload: []byte{0, 255, 7}, mode: 0o400},
		{name: "hard link is a regular entry sharing the exact inode", entry: custodyNamespaceHardLink, payload: []byte{0, 255, 7}, mode: 0o600},
		{name: "confined relative symlink cannot borrow target custody", entry: custodyNamespaceRelativeLink, payload: []byte{0, 255, 7}, mode: 0o600, wantNative: fs.ErrInvalid},
		{name: "absolute symlink to the same file remains a link", entry: custodyNamespaceAbsoluteLink, payload: []byte{0, 255, 7}, mode: 0o600, wantNative: fs.ErrInvalid},
		{name: "dangling link is refused as a link before traversal", entry: custodyNamespaceDanglingLink, payload: []byte{0, 255, 7}, mode: 0o600, wantNative: fs.ErrInvalid},
		{name: "self link is refused before entering a traversal loop", entry: custodyNamespaceSelfLink, payload: []byte{0, 255, 7}, mode: 0o600, wantNative: fs.ErrInvalid},
		{name: "directory cannot receive a regular file custody stamp", entry: custodyNamespaceDirectory, payload: []byte{0, 255, 7}, mode: 0o600, wantNative: fs.ErrInvalid},
		{name: "absent leaf cannot become an empty durable fact", entry: custodyNamespaceAbsent, payload: []byte{0, 255, 7}, mode: 0o600, wantNative: fs.ErrNotExist},
	} {
		for _, operation := range []struct {
			name  string
			touch bool
		}{
			{name: "ConfirmDurable"},
			{name: "Touch", touch: true},
		} {
			t.Run(operation.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				directory := t.TempDir()
				root := requireTestRoot(t, directory)
				if err := os.WriteFile(filepath.Join(directory, "original"), tc.payload, tc.mode); err != nil {
					t.Fatal(err)
				}
				initial := temporal.InstantFromNanoseconds(2_000_000_000)
				stamp, err := initial.Time()
				if err != nil {
					t.Fatal(err)
				}
				if err := root.Chtimes("original", stamp, stamp); err != nil {
					t.Fatal(err)
				}
				name := "entry"
				switch tc.entry {
				case custodyNamespaceOriginal:
					name = "original"
				case custodyNamespaceHardLink:
					err = root.Link("original", name)
				case custodyNamespaceRelativeLink:
					err = root.Symlink("original", name)
				case custodyNamespaceAbsoluteLink:
					err = root.Symlink(filepath.Join(directory, "original"), name)
				case custodyNamespaceDanglingLink:
					err = root.Symlink("missing", name)
				case custodyNamespaceSelfLink:
					err = root.Symlink(name, name)
				case custodyNamespaceDirectory:
					err = root.Mkdir(name, 0o700)
				case custodyNamespaceAbsent:
				default:
					t.Fatalf("namespace = %d, want declared fixture", tc.entry)
				}
				if err != nil {
					t.Fatal(err)
				}
				before, err := root.Lstat("original")
				if err != nil {
					t.Fatal(err)
				}
				entryBefore, entryErr := root.Lstat(name)
				if tc.entry == custodyNamespaceAbsent {
					if !errors.Is(entryErr, fs.ErrNotExist) {
						t.Fatalf("fixture absence = %v, want not exist", entryErr)
					}
				} else if entryErr != nil {
					t.Fatal(entryErr)
				}
				entriesBefore, err := os.ReadDir(directory)
				if err != nil {
					t.Fatal(err)
				}
				location := filestore.Location{Root: root, Path: mustRelativePath(t, name)}
				instant := temporal.InstantFromNanoseconds(1_000_000_000)
				var gotErr error
				if operation.touch {
					gotErr = filestore.Touch(t.Context(), filestore.TouchRequest{Location: location, ModifiedAt: instant})
				} else {
					gotErr = filestore.ConfirmDurable(t.Context(), filestore.DurabilityRequest{Location: location})
				}
				if (gotErr == nil) != (tc.wantNative == nil) || tc.wantNative != nil && (!errors.Is(gotErr, core.ErrFilestoreSource) || !errors.Is(gotErr, tc.wantNative)) {
					t.Fatalf("custody error = %v, want source/%v", gotErr, tc.wantNative)
				}
				after, err := root.Lstat("original")
				if err != nil {
					t.Fatal(err)
				}
				wantStamp := before.ModTime().UnixNano()
				if operation.touch && tc.wantNative == nil {
					wantStamp = 1_000_000_000
				}
				if !os.SameFile(before, after) || after.Mode() != before.Mode() || after.ModTime().UnixNano() != wantStamp {
					t.Fatalf("original = (%v,%d), want preserved inode/mode and stamp %d", after.Mode(), after.ModTime().UnixNano(), wantStamp)
				}
				got, err := os.ReadFile(filepath.Join(directory, "original"))
				if err != nil || !bytes.Equal(got, tc.payload) {
					t.Fatalf("original bytes = (%v,%v), want %v", got, err, tc.payload)
				}
				entryAfter, entryErr := root.Lstat(name)
				if tc.entry == custodyNamespaceAbsent {
					if !errors.Is(entryErr, fs.ErrNotExist) {
						t.Fatalf("absence after refusal = %v, want not exist", entryErr)
					}
				} else {
					if entryErr != nil {
						t.Fatal(entryErr)
					}
					if !os.SameFile(entryBefore, entryAfter) || entryAfter.Mode() != entryBefore.Mode() {
						t.Fatalf("namespace entry changed: %v", entryAfter)
					}
					if tc.wantNative != nil && entryAfter.ModTime() != entryBefore.ModTime() {
						t.Fatalf("refused namespace stamp changed: %v", entryAfter.ModTime())
					}
				}
				entriesAfter, err := os.ReadDir(directory)
				if err != nil || len(entriesAfter) != len(entriesBefore) {
					t.Fatalf("directory = (%v,%v), want %d unchanged names", entriesAfter, err, len(entriesBefore))
				}
				for i, entry := range entriesAfter {
					if entry.Name() != entriesBefore[i].Name() {
						t.Fatalf("entry = %s, want %s", entry.Name(), entriesBefore[i].Name())
					}
				}
			})
		}
	}
}
