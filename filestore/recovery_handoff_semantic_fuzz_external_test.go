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

// A pre-activation conflict returns no recovery receipt. A preserved hard link
// keeps the original inode alive, preventing allocator reuse from faking custody.
func FuzzWriteRecoveryHandoffSemanticCustody(f *testing.F) {
	emitted, err := filestore.FilestoreRoundTripSeedForTest(f.Context(), f.TempDir())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(emitted, false, false)
	for _, seed := range []struct {
		payload           []byte
		replace, occupied bool
	}{
		{payload: nil},
		{payload: []byte{0, 255}},
		{payload: []byte{0, 255}, occupied: true},
		{payload: []byte{0, 255}, replace: true, occupied: true},
		{payload: []byte{0, 255}, replace: true},
	} {
		f.Add(seed.payload, seed.replace, seed.occupied)
	}
	f.Fuzz(func(t *testing.T, payload []byte, replace, occupied bool) {
		payload = payload[:min(len(payload), 2048)]
		directory := t.TempDir()
		root := requireTestRoot(t, directory)
		temporary := mustRelativePath(t, "stage")
		target := mustRelativePath(t, "target")
		install := filestore.InstallCreate
		if replace {
			install = filestore.InstallReplace
		}
		original := []byte{16, 0, 255, 42}
		neighbor := []byte{39, 0, 255, 8}
		if err := os.WriteFile(filepath.Join(directory, "neighbor"), neighbor, 0o600); err != nil {
			t.Fatal(err)
		}
		var originalInfo fs.FileInfo
		if occupied {
			if err := os.WriteFile(filepath.Join(directory, target.String()), original, 0o640); err != nil {
				t.Fatal(err)
			}
			var err error
			originalInfo, err = root.Lstat(target.String())
			if err != nil {
				t.Fatal(err)
			}
		}
		foreign := bytes.Clone(payload)
		if len(foreign) > 0 {
			foreign[0] ^= 255
		}
		source := &stageIdentitySwapSource{remaining: payload, foreign: foreign, directory: directory, name: temporary.String(), preserve: "archive"}
		request := filestore.WriteRequest{Source: source, Location: filestore.Location{Root: root, Path: target}, Temporary: temporary, Mode: 0o600, Install: install}
		if err := request.Validate(); err != nil {
			t.Fatal(err)
		}
		got, gotErr := filestore.Write(t.Context(), request)
		if source.swapErr != nil || !source.swapped {
			t.Fatalf("namespace substitution = (%t,%v), want real mutation", source.swapped, source.swapErr)
		}
		if !errors.Is(gotErr, core.ErrFilestoreConflict) || errors.Is(gotErr, core.ErrFilestoreActivationIndeterminate) || got != (filestore.CommitRequest{}) {
			t.Fatalf("Write handoff = (%+v,%v), want conflict and zero recovery", got, gotErr)
		}
		archived, err := root.Lstat("archive")
		if err != nil {
			t.Fatal(err)
		}
		stranger, err := root.Lstat(temporary.String())
		if err != nil {
			t.Fatal(err)
		}
		if os.SameFile(archived, stranger) {
			t.Fatalf("substituted inode = %v, want distinct from %v", stranger, archived)
		}
		kept, err := root.Lstat(temporary.String())
		if err != nil {
			t.Fatal(err)
		}
		keptBytes, err := os.ReadFile(filepath.Join(directory, temporary.String()))
		if err != nil || !os.SameFile(stranger, kept) || !bytes.Equal(keptBytes, foreign) || kept.Mode() != stranger.Mode() || !kept.ModTime().Equal(stranger.ModTime()) {
			t.Fatalf("foreign custody = (%v,%v,%v), want unchanged inode, metadata and bytes", kept, keptBytes, err)
		}
		wantEntries := 3
		if occupied {
			wantEntries++
		}
		archiveBytes, err := os.ReadFile(filepath.Join(directory, "archive"))
		if err != nil || !bytes.Equal(archiveBytes, payload) {
			t.Fatalf("preserved original = (%v,%v), want %v", archiveBytes, err, payload)
		}
		if occupied {
			info, statErr := root.Lstat(target.String())
			data, readErr := root.ReadFile(target.String())
			if statErr != nil || readErr != nil || !bytes.Equal(data, original) || !os.SameFile(info, originalInfo) {
				t.Fatalf("unactivated target = (%v,%x,%v,%v), want exact original target", info, data, statErr, readErr)
			}
		} else if _, err := root.Lstat(target.String()); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("unpublished target = %v, want absent", err)
		}
		keptNeighbor, err := os.ReadFile(filepath.Join(directory, "neighbor"))
		if err != nil || !bytes.Equal(keptNeighbor, neighbor) {
			t.Fatalf("neighbor = (%v,%v), want %v", keptNeighbor, err, neighbor)
		}
		entries, err := os.ReadDir(directory)
		if err != nil || len(entries) != wantEntries {
			t.Fatalf("namespace = (%v,%v), want %d exact entries", entries, err, wantEntries)
		}
	})
}
