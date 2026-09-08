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

// The receipt comes from an actual interrupted Write. A preserved hard link
// keeps the original inode alive, preventing allocator reuse from faking custody.
func FuzzWriteRecoveryHandoffSemanticCustody(f *testing.F) {
	emitted, err := filestore.FilestoreRoundTripSeedForTest(f.Context(), f.TempDir())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(emitted, false, false, true)
	for _, seed := range []struct {
		payload                    []byte
		replace, occupied, restore bool
	}{
		{payload: nil, restore: true},
		{payload: []byte{0, 255}, restore: true},
		{payload: []byte{0, 255}, occupied: true, restore: true},
		{payload: []byte{0, 255}, replace: true, occupied: true, restore: true},
		{payload: []byte{0, 255}, replace: true, restore: true},
		{payload: nil},
		{payload: []byte{0, 255}, occupied: true},
	} {
		f.Add(seed.payload, seed.replace, seed.occupied, seed.restore)
	}
	f.Fuzz(func(t *testing.T, payload []byte, replace, occupied, restore bool) {
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
		request := filestore.WriteRequest{Source: source, Location: filestore.Location{Root: root, Path: target}, Temporary: temporary, Mode: 0o600, Install: install, MaximumBytes: mustByteCount(t, uint64(max(len(payload), 1)))}
		if err := request.Validate(); err != nil {
			t.Fatal(err)
		}
		got, gotErr := filestore.Write(t.Context(), request)
		if source.swapErr != nil || !source.swapped {
			t.Fatalf("namespace substitution = (%t,%v), want real mutation", source.swapped, source.swapErr)
		}
		if !errors.Is(gotErr, core.ErrFilestoreActivationIndeterminate) || got.Validate() != nil || got.Target != target || got.Install != install || got.Staged.Path() != temporary || got.Staged.BytesWritten().Uint64() != uint64(len(payload)) {
			t.Fatalf("Write handoff = (%+v,%v), want exact indeterminate request", got, gotErr)
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
		if err := filestore.Recover(t.Context(), got); !errors.Is(err, core.ErrFilestoreActivationIndeterminate) {
			t.Fatalf("foreign Recover = %v, want indeterminate", err)
		}
		if err := filestore.Discard(t.Context(), got.Staged); !errors.Is(err, core.ErrFilestoreCleanup) || !errors.Is(err, core.ErrFilestoreConflict) {
			t.Fatalf("foreign Discard = %v, want cleanup and conflict", err)
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
		installed := false
		if restore {
			if err := root.Remove(temporary.String()); err != nil {
				t.Fatal(err)
			}
			if err := root.Rename("archive", temporary.String()); err != nil {
				t.Fatal(err)
			}
			gotErr := filestore.Recover(t.Context(), got)
			if occupied && !replace {
				if !errors.Is(gotErr, core.ErrFilestoreConflict) || !errors.Is(gotErr, fs.ErrExist) {
					t.Fatalf("create recovery = %v, want conflict and native existence", gotErr)
				}
				stageInfo, err := root.Lstat(temporary.String())
				if err != nil {
					t.Fatal(err)
				}
				stageBytes, err := os.ReadFile(filepath.Join(directory, temporary.String()))
				if err != nil || !os.SameFile(stageInfo, archived) || !bytes.Equal(stageBytes, payload) {
					t.Fatalf("refused recovery stage = (%v,%v,%v), want original inode and bytes", stageInfo, stageBytes, err)
				}
				if err := filestore.Discard(t.Context(), got.Staged); err != nil {
					t.Fatal(err)
				}
			} else {
				if gotErr != nil {
					t.Fatalf("restored recovery = %v, want nil", gotErr)
				}
				installed = true
				first, err := root.Lstat(target.String())
				if err != nil {
					t.Fatal(err)
				}
				if err := filestore.Recover(t.Context(), got); err != nil {
					t.Fatalf("completed recovery = %v, want idempotent success", err)
				}
				second, err := root.Lstat(target.String())
				if err != nil {
					t.Fatal(err)
				}
				if !os.SameFile(first, second) || first.Mode() != second.Mode() || !first.ModTime().Equal(second.ModTime()) {
					t.Fatalf("repeated recovery = %v, want unchanged %v", second, first)
				}
			}
			wantEntries = 2
			for _, name := range []string{temporary.String(), "archive"} {
				if _, err := root.Lstat(name); !errors.Is(err, fs.ErrNotExist) {
					t.Fatalf("settled %s = %v, want absent", name, err)
				}
			}
		} else {
			archiveBytes, err := os.ReadFile(filepath.Join(directory, "archive"))
			if err != nil || !bytes.Equal(archiveBytes, payload) {
				t.Fatalf("preserved original = (%v,%v), want %v", archiveBytes, err, payload)
			}
		}
		if installed || occupied {
			wantBytes, wantInfo := original, originalInfo
			if installed {
				wantBytes, wantInfo = payload, archived
			}
			info, err := root.Lstat(target.String())
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(directory, target.String()))
			if err != nil || !bytes.Equal(data, wantBytes) || !os.SameFile(info, wantInfo) || info.Mode() != wantInfo.Mode() || !info.ModTime().Equal(wantInfo.ModTime()) {
				t.Fatalf("target = (%v,%v,%v), want exact bytes and inode %v", info, data, err, wantInfo)
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
