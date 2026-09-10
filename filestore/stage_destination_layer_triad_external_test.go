package filestore_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

type destinationNamespace uint8

const (
	destinationOwned destinationNamespace = iota
	destinationMissing
	destinationForeignFile
	destinationForeignDirectory
	destinationSymbolicOwner
	destinationHardLinkedOwner
)

// Exhaust the distinct custody-name observations for finish and abandon.
// Identical bytes are deliberate: only the real inode can authorize settlement.
func TestStageDestinationDurableWriterLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                                                     string
		namespace                                                destinationNamespace
		abandon, empty, wantReceipt, wantTemporary, wantConflict bool
		wantErr, wantNative                                      error
	}{
		{name: "fragmented binary producer transfers original inode", wantReceipt: true},
		{name: "empty producer transfers a real empty inode", empty: true, wantReceipt: true},
		{name: "abandon populated producer closes and removes owned name", abandon: true},
		{name: "abandon empty producer invents no target", empty: true, abandon: true},
		{name: "missing custody name cannot yield a receipt", namespace: destinationMissing, wantErr: core.ErrFilestoreActivation, wantNative: fs.ErrNotExist},
		{name: "abandon missing name leaves archived original untouched", namespace: destinationMissing, abandon: true},
		{name: "same bytes in foreign inode cannot authorize finish", namespace: destinationForeignFile, wantErr: core.ErrFilestoreActivationIndeterminate, wantTemporary: true, wantConflict: true},
		{name: "abandon same-byte stranger cannot erase it", namespace: destinationForeignFile, abandon: true, wantErr: core.ErrFilestoreCleanup, wantTemporary: true, wantConflict: true},
		{name: "foreign directory and child cannot be finalized", namespace: destinationForeignDirectory, wantErr: core.ErrFilestoreActivationIndeterminate, wantTemporary: true, wantConflict: true},
		{name: "abandon foreign directory preserves its child", namespace: destinationForeignDirectory, abandon: true, wantErr: core.ErrFilestoreCleanup, wantTemporary: true, wantConflict: true},
		{name: "symlink resolving to original inode is not owned entry", namespace: destinationSymbolicOwner, wantErr: core.ErrFilestoreActivationIndeterminate, wantTemporary: true, wantConflict: true},
		{name: "abandon does not follow symlink to owned archive", namespace: destinationSymbolicOwner, abandon: true, wantErr: core.ErrFilestoreCleanup, wantTemporary: true, wantConflict: true},
		{name: "real hard link retains inode custody through finish", namespace: destinationHardLinkedOwner, wantReceipt: true},
		{name: "abandon hard link removes only the requested name", namespace: destinationHardLinkedOwner, abandon: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			root := requireTestRoot(t, directory)
			payload := []byte{0, 255, 31, 8, 0}
			if tc.empty {
				payload = nil
			}
			neighbor := []byte{7, 0, 255}
			if err := os.WriteFile(filepath.Join(directory, "neighbor"), neighbor, 0o600); err != nil {
				t.Fatal(err)
			}
			plan := filestore.ActivationRequest{Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, "stage")}, Target: mustRelativePath(t, "target"), ExpectedBytes: new(stageDestinationLength(t, uint64(len(payload)))), Mode: 0o600, Install: filestore.InstallCreate}
			if err := plan.Validate(); err != nil {
				t.Fatal(err)
			}
			destination, err := filestore.OpenStageDestination(t.Context(), plan.StageDestination())
			if err != nil {
				t.Fatal(err)
			}
			file, err := destination.File()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = file.Close() })
			for offset := 0; offset < len(payload); offset++ {
				if n, err := file.Write(payload[offset : offset+1]); err != nil || n != 1 {
					t.Fatalf("fragment write = (%d,%v), want one byte", n, err)
				}
			}
			before, err := file.Stat()
			if err != nil {
				t.Fatal(err)
			}
			if tc.namespace != destinationOwned {
				if err := root.Rename("stage", "archive"); err != nil {
					t.Fatal(err)
				}
			}
			switch tc.namespace {
			case destinationOwned, destinationMissing:
			case destinationForeignFile:
				if err := os.WriteFile(filepath.Join(directory, "stage"), payload, 0o600); err != nil {
					t.Fatal(err)
				}
			case destinationForeignDirectory:
				if err := root.Mkdir("stage", 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(directory, "stage", "child"), neighbor, 0o600); err != nil {
					t.Fatal(err)
				}
			case destinationSymbolicOwner:
				if err := root.Symlink("archive", "stage"); err != nil {
					t.Fatal(err)
				}
			case destinationHardLinkedOwner:
				if err := root.Link("archive", "stage"); err != nil {
					t.Fatal(err)
				}
			default:
				t.Fatalf("namespace fixture = %d, want declared case", tc.namespace)
			}
			var namespaceInfo fs.FileInfo
			if tc.namespace != destinationMissing {
				namespaceInfo, err = root.Lstat("stage")
				if err != nil {
					t.Fatal(err)
				}
				if gotOwned := os.SameFile(before, namespaceInfo); gotOwned == tc.wantConflict {
					t.Fatalf("fixture inode ownership = %t, want conflict %t", gotOwned, tc.wantConflict)
				}
			}
			want, err := removalFixtureSnapshot(directory)
			if err != nil {
				t.Fatal(err)
			}
			if !tc.wantTemporary {
				want = slices.DeleteFunc(want, func(entry removalFixtureEntry) bool { return entry.name == "stage" })
			}
			if tc.wantReceipt {
				want = append(want, removalFixtureEntry{name: "target", mode: before.Mode(), data: bytes.Clone(payload)})
			}
			slices.SortFunc(want, func(a, b removalFixtureEntry) int {
				if a.name < b.name {
					return -1
				}
				if a.name > b.name {
					return 1
				}
				return 0
			})
			var got filestore.StagedFile
			var gotErr error
			if tc.abandon {
				gotErr = filestore.AbandonStageDestination(destination)
			} else {
				got, gotErr = filestore.FinishStageDestination(t.Context(), destination)
			}
			if !errors.Is(gotErr, tc.wantErr) || tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) {
				t.Fatalf("settlement = (%+v,%v), want %v with native %v", got, gotErr, tc.wantErr, tc.wantNative)
			}
			if tc.wantConflict && (!errors.Is(gotErr, core.ErrFilestoreCleanup) || !errors.Is(gotErr, core.ErrFilestoreConflict)) {
				t.Fatalf("foreign custody = %v, want cleanup and conflict", gotErr)
			}
			if _, err := file.Stat(); !errors.Is(err, fs.ErrClosed) {
				t.Fatalf("producer handle = %v, want closed", err)
			}
			gotFile, fileErr := destination.File()
			if gotFile != nil || !errors.Is(fileErr, core.ErrFilestoreContract) {
				t.Fatalf("settled handle = (%v,%v), want no file and contract refusal", gotFile, fileErr)
			}
			if tc.wantReceipt {
				if err := got.Validate(); err != nil || got.Path() != plan.Temporary.Path || got.BytesWritten() != *plan.ExpectedBytes {
					t.Fatalf("receipt = (%+v,%v), want exact validated stage", got, err)
				}
				commit, err := plan.CommitRequest(got)
				if err != nil {
					t.Fatal(err)
				}
				if err := filestore.Commit(t.Context(), commit); err != nil {
					t.Fatal(err)
				}
				installed, err := root.Lstat("target")
				if err != nil {
					t.Fatal(err)
				}
				if !os.SameFile(before, installed) || !installed.ModTime().Equal(before.ModTime()) {
					t.Fatalf("installed inode = %v, want original %v", installed, before)
				}
			} else if got != (filestore.StagedFile{}) {
				t.Fatalf("refused receipt = %+v, want zero", got)
			}
			if tc.wantTemporary {
				after, err := root.Lstat("stage")
				if err != nil {
					t.Fatal(err)
				}
				if !os.SameFile(namespaceInfo, after) || !namespaceInfo.ModTime().Equal(after.ModTime()) {
					t.Fatalf("foreign entry = %v, want same inode and metadata %v", after, namespaceInfo)
				}
			}
			if tc.namespace != destinationOwned {
				archive, err := root.Lstat("archive")
				if err != nil {
					t.Fatal(err)
				}
				if !os.SameFile(archive, before) || !archive.ModTime().Equal(before.ModTime()) {
					t.Fatalf("archive = %v, want original inode %v", archive, before)
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
		})
	}
}

func stageDestinationLength(t *testing.T, value uint64) core.ByteLength {
	t.Helper()
	length, err := core.NewByteLength(value)
	if err != nil {
		t.Fatal(err)
	}
	return length
}
