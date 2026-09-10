package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func FuzzStageDestinationNativeWriterCustody(f *testing.F) {
	emitted, err := filestore.FilestoreRoundTripSeedForTest(f.Context(), f.TempDir())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(emitted, uint16(len(emitted)), uint16(0), false, false, false, false)
	f.Add(emitted, uint16(1<<15), uint16(1), false, false, false, false)
	f.Add([]byte{}, uint16(1<<15), uint16(1), false, false, false, false)
	for _, seed := range []struct {
		payload                              []byte
		extent, fragment                     uint16
		abandon, copyHandle, cancel, foreign bool
	}{
		{payload: nil, fragment: 1},
		{payload: []byte{0, 255}, extent: 2, fragment: 1, copyHandle: true},
		{payload: []byte{0, 255}, extent: 1, fragment: 2},
		{payload: []byte{0, 255}, extent: 3, fragment: 2},
		{payload: nil, fragment: 1, abandon: true},
		{payload: []byte{0, 255}, extent: 2, fragment: 1, cancel: true},
		{payload: nil, fragment: 1, foreign: true},
		{payload: []byte{0, 255}, extent: 2, fragment: 2, foreign: true},
		{payload: []byte{0, 255}, extent: 2, fragment: 1, foreign: true, abandon: true},
	} {
		f.Add(seed.payload, seed.extent, seed.fragment, seed.abandon, seed.copyHandle, seed.cancel, seed.foreign)
	}
	f.Fuzz(func(t *testing.T, payload []byte, rawExtent, rawFragment uint16, abandon, copyHandle, canceled, foreign bool) {
		payload = payload[:min(len(payload), 2048)]
		extent, err := core.NewByteLength(uint64((rawExtent & 0x7fff) % 2049))
		if err != nil {
			t.Fatal(err)
		}
		fragment := int(rawFragment%65) + 1
		directory := t.TempDir()
		root := requireTestRoot(t, directory)
		stagePath := mustRelativePath(t, "stage")
		targetPath := mustRelativePath(t, "target")
		plan := filestore.ActivationRequest{Temporary: filestore.Location{Root: root, Path: stagePath}, Target: targetPath, ExpectedBytes: new(extent), Mode: 0o600, Install: filestore.InstallCreate}
		if rawExtent&(1<<15) != 0 {
			plan.ExpectedBytes = nil
		}
		if err := plan.Validate(); err != nil {
			t.Fatal(err)
		}
		neighbor := []byte{31, 0, 255, 9}
		if err := os.WriteFile(filepath.Join(directory, "neighbor"), neighbor, 0o600); err != nil {
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
		before, err := file.Stat()
		if err != nil {
			t.Fatal(err)
		}
		for offset := 0; offset < len(payload); {
			end := min(offset+fragment, len(payload))
			n, err := file.Write(payload[offset:end])
			if err != nil || n != end-offset {
				t.Fatalf("native write = (%d,%v), want %d", n, err, end-offset)
			}
			offset = end
		}
		if copyHandle {
			copied := *destination
			gotFile, fileErr := copied.File()
			gotCopy, finishErr := filestore.FinishStageDestination(t.Context(), &copied)
			abandonErr := filestore.AbandonStageDestination(&copied)
			if gotFile != nil || gotCopy != (filestore.StagedFile{}) || !errors.Is(fileErr, core.ErrFilestoreContract) || !errors.Is(finishErr, core.ErrFilestoreContract) || !errors.Is(abandonErr, core.ErrFilestoreContract) {
				t.Fatalf("copied custody = (%v,%+v,%v,%v,%v), want three refusals and zero capabilities", gotFile, gotCopy, fileErr, finishErr, abandonErr)
			}
			gotOriginal, err := destination.File()
			if err != nil || gotOriginal != file {
				t.Fatalf("original custody = (%v,%v), want retained %v", gotOriginal, err, file)
			}
			info, err := file.Stat()
			if err != nil || info.Size() != int64(len(payload)) {
				t.Fatalf("original handle = (%v,%v), want live exact extent", info, err)
			}
		}
		stranger := bytes.Clone(payload)
		if len(stranger) > 0 {
			stranger[0] ^= 255
		}
		if foreign {
			if err := root.Rename(stagePath.String(), "archive"); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, stagePath.String()), stranger, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		var foreignInfo fs.FileInfo
		if foreign {
			foreignInfo, err = root.Lstat(stagePath.String())
			if err != nil {
				t.Fatal(err)
			}
			if os.SameFile(before, foreignInfo) {
				t.Fatalf("foreign inode = %v, want distinct from %v", foreignInfo, before)
			}
		}
		ctx := t.Context()
		if canceled {
			var cancel context.CancelFunc
			ctx, cancel = context.WithCancel(ctx)
			cancel()
		}
		var got filestore.StagedFile
		var gotErr error
		if abandon {
			gotErr = filestore.AbandonStageDestination(destination)
		} else {
			got, gotErr = filestore.FinishStageDestination(ctx, destination)
		}
		var wantErr error
		switch {
		case abandon && foreign:
			wantErr = core.ErrFilestoreCleanup
		case abandon:
		case canceled:
			wantErr = context.Canceled
		case plan.ExpectedBytes != nil && extent.Uint64() != uint64(len(payload)):
			wantErr = core.ErrFilestoreSize
		case foreign:
			wantErr = core.ErrFilestoreActivationIndeterminate
		}
		if (gotErr == nil) != (wantErr == nil) || wantErr != nil && !errors.Is(gotErr, wantErr) {
			t.Fatalf("settlement = (%+v,%v), want %v", got, gotErr, wantErr)
		}
		if foreign && (!errors.Is(gotErr, core.ErrFilestoreCleanup) || !errors.Is(gotErr, core.ErrFilestoreConflict)) {
			t.Fatalf("foreign cleanup = %v, want cleanup/conflict without deletion", gotErr)
		}
		if _, err := file.Stat(); !errors.Is(err, fs.ErrClosed) {
			t.Fatalf("producer handle = %v, want closed after settlement", err)
		}
		gotFile, fileErr := destination.File()
		if gotFile != nil || !errors.Is(fileErr, core.ErrFilestoreContract) {
			t.Fatalf("settled capability = (%v,%v), want nil and contract refusal", gotFile, fileErr)
		}
		wantEntries := 1
		if foreign {
			wantEntries = 3
			gotForeign, err := os.ReadFile(filepath.Join(directory, stagePath.String()))
			if err != nil || !bytes.Equal(gotForeign, stranger) {
				t.Fatalf("stranger = (%v,%v), want %v", gotForeign, err, stranger)
			}
			retained, err := root.Lstat(stagePath.String())
			if err != nil {
				t.Fatal(err)
			}
			if !os.SameFile(foreignInfo, retained) || foreignInfo.Mode() != retained.Mode() || !foreignInfo.ModTime().Equal(retained.ModTime()) {
				t.Fatalf("retained foreign = %v, want original metadata %v", retained, foreignInfo)
			}
			archived, err := root.Lstat("archive")
			if err != nil {
				t.Fatal(err)
			}
			gotArchive, err := os.ReadFile(filepath.Join(directory, "archive"))
			if err != nil || !bytes.Equal(gotArchive, payload) || !os.SameFile(before, archived) {
				t.Fatalf("archive = (%v,%v,%v), want original inode and %v", archived, gotArchive, err, payload)
			}
		} else if wantErr == nil && !abandon {
			wantEntries = 2
			if got.Validate() != nil || got.Path() != stagePath || got.BytesWritten().Uint64() != uint64(len(payload)) {
				t.Fatalf("completed stage = %+v, want exact validated receipt", got)
			}
			reader, err := filestore.OpenStagedRead(t.Context(), got)
			if err != nil {
				t.Fatal(err)
			}
			gotBytes, readErr := io.ReadAll(io.LimitReader(reader, int64(len(payload)+1)))
			closeErr := reader.Close()
			if readErr != nil || closeErr != nil || !bytes.Equal(gotBytes, payload) {
				t.Fatalf("staged read = (%v,%v,%v), want %v", gotBytes, readErr, closeErr, payload)
			}
			commit, err := plan.CommitRequest(got)
			if err != nil {
				t.Fatal(err)
			}
			if err := filestore.Commit(t.Context(), commit); err != nil {
				t.Fatal(err)
			}
			installed, err := root.Lstat(targetPath.String())
			if err != nil {
				t.Fatal(err)
			}
			stored, err := os.ReadFile(filepath.Join(directory, targetPath.String()))
			if err != nil || !bytes.Equal(stored, payload) || !os.SameFile(before, installed) || installed.Mode().Perm() != plan.Mode || installed.Size() != int64(len(payload)) {
				t.Fatalf("activation = (%v,%v,%v), want exact original inode and %v", installed, stored, err, payload)
			}
		}
		if wantErr != nil || abandon {
			if got != (filestore.StagedFile{}) {
				t.Fatalf("uncompleted receipt = %+v, want zero", got)
			}
		}
		if !foreign {
			if _, err := root.Lstat(stagePath.String()); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("settled temporary = %v, want absent", err)
			}
		}
		gotNeighbor, err := os.ReadFile(filepath.Join(directory, "neighbor"))
		if err != nil || !bytes.Equal(gotNeighbor, neighbor) {
			t.Fatalf("neighbor = (%v,%v), want %v", gotNeighbor, err, neighbor)
		}
		entries, err := os.ReadDir(directory)
		if err != nil || len(entries) != wantEntries {
			t.Fatalf("namespace = (%v,%v), want %d exact entries", entries, err, wantEntries)
		}
	})
}
