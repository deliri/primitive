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

const filestoreFuzzPayloadMaximum = 1 << 20

func FuzzWriteReadRoundTrip(f *testing.F) {
	emitted, err := filestore.FilestoreRoundTripSeedForTest(f.Context(), f.TempDir())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(emitted)
	for _, payload := range [][]byte{{}, deterministicPayload(filestoreFuzzPayloadMaximum - 1), deterministicPayload(filestoreFuzzPayloadMaximum), deterministicPayload(filestoreFuzzPayloadMaximum + 1)} {
		f.Add(payload)
	}
	f.Fuzz(func(t *testing.T, payload []byte) {
		payload = payload[:min(len(payload), filestoreFuzzPayloadMaximum+1)]
		directory := t.TempDir()
		root := requireTestRoot(t, directory)
		neighbor := []byte{255, 0, 127, 1}
		if err := os.WriteFile(filepath.Join(directory, "neighbor"), neighbor, 0o640); err != nil {
			t.Fatal(err)
		}
		neighborBefore, err := root.Lstat("neighbor")
		if err != nil {
			t.Fatal(err)
		}
		maximum := mustByteCount(t, filestoreFuzzPayloadMaximum)
		source := bytes.NewReader(payload)
		location := filestore.Location{Root: root, Path: mustRelativePath(t, "target")}
		recovery, gotErr := filestore.Write(t.Context(), filestore.WriteRequest{Source: source, Location: location, Temporary: mustRelativePath(t, "stage"), Mode: 0o600, Install: filestore.InstallCreate, MaximumBytes: maximum})
		var wantErr error
		wantEntries := 2
		if len(payload) > filestoreFuzzPayloadMaximum {
			wantErr = core.ErrFilestoreSize
			wantEntries = 1
		}
		if !errors.Is(gotErr, wantErr) || errors.Is(gotErr, core.ErrFilestoreSource) || errors.Is(gotErr, core.ErrFilestoreActivation) || errors.Is(gotErr, core.ErrFilestoreDestination) || recovery != (filestore.CommitRequest{}) || source.Len() != 0 {
			t.Fatalf("Write = (%+v,%v,%d unread), want zero recovery, %v, consumed bounded input", recovery, gotErr, source.Len(), wantErr)
		}
		if _, err := root.Lstat("stage"); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("stage = %v, want absent", err)
		}
		if wantErr != nil {
			if _, err := root.Lstat("target"); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("refused target = %v, want absent", err)
			}
		} else {
			info, err := root.Lstat("target")
			if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || info.Size() != int64(len(payload)) {
				t.Fatalf("target = (%v,%v), want exact regular extent and mode", info, err)
			}
			native, err := os.ReadFile(filepath.Join(directory, "target"))
			if err != nil || !bytes.Equal(native, payload) {
				t.Fatalf("native bytes = (%d,%v), want %d exact bytes", len(native), err, len(payload))
			}
			var destination bytes.Buffer
			count, err := filestore.Read(t.Context(), filestore.ReadRequest{Location: location, Destination: &destination, MaximumBytes: maximum})
			if err != nil || count.Uint64() != uint64(len(payload)) || !bytes.Equal(destination.Bytes(), payload) {
				t.Fatalf("Read = (%d,%d,%v), want %d exact bytes", count.Uint64(), destination.Len(), err, len(payload))
			}
			after, err := root.Lstat("target")
			if err != nil || !os.SameFile(info, after) || info.Mode() != after.Mode() || info.ModTime().UnixNano() != after.ModTime().UnixNano() {
				t.Fatalf("read custody = (%v,%v), want preserved inode and metadata", after, err)
			}
		}
		neighborAfter, err := root.Lstat("neighbor")
		if err != nil || !os.SameFile(neighborBefore, neighborAfter) || neighborBefore.Mode() != neighborAfter.Mode() || neighborBefore.ModTime().UnixNano() != neighborAfter.ModTime().UnixNano() {
			t.Fatalf("neighbor = (%v,%v), want preserved inode and metadata", neighborAfter, err)
		}
		data, err := os.ReadFile(filepath.Join(directory, "neighbor"))
		if err != nil || !bytes.Equal(data, neighbor) {
			t.Fatalf("neighbor bytes = (%v,%v), want %v", data, err, neighbor)
		}
		entries, err := os.ReadDir(directory)
		if err != nil || len(entries) != wantEntries {
			t.Fatalf("namespace = (%v,%v), want %d exact entries", entries, err, wantEntries)
		}
	})
}

func FuzzStageCommitRoundTrip(f *testing.F) {
	emitted, err := filestore.FilestoreRoundTripSeedForTest(f.Context(), f.TempDir())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(emitted)
	for _, payload := range [][]byte{{}, deterministicPayload(filestoreFuzzPayloadMaximum - 1), deterministicPayload(filestoreFuzzPayloadMaximum), deterministicPayload(filestoreFuzzPayloadMaximum + 1)} {
		f.Add(payload)
	}
	f.Fuzz(func(t *testing.T, payload []byte) {
		payload = payload[:min(len(payload), filestoreFuzzPayloadMaximum+1)]
		directory := t.TempDir()
		root := requireTestRoot(t, directory)
		maximum := mustByteCount(t, filestoreFuzzPayloadMaximum)
		neighbor := []byte{255, 0, 127, 1}
		if err := os.WriteFile(filepath.Join(directory, "neighbor"), neighbor, 0o640); err != nil {
			t.Fatal(err)
		}
		neighborBefore, err := root.Lstat("neighbor")
		if err != nil {
			t.Fatal(err)
		}
		source := bytes.NewReader(payload)
		stagePath := mustRelativePath(t, "stage")
		staged, gotErr := filestore.Stage(t.Context(), filestore.StageRequest{Source: source, Temporary: filestore.Location{Root: root, Path: stagePath}, Mode: 0o600, MaximumBytes: maximum})
		var wantErr error
		wantEntries := 2
		if len(payload) > filestoreFuzzPayloadMaximum {
			wantErr = core.ErrFilestoreSize
			wantEntries = 1
		}
		if !errors.Is(gotErr, wantErr) || errors.Is(gotErr, core.ErrFilestoreSource) || errors.Is(gotErr, core.ErrFilestoreActivation) || errors.Is(gotErr, core.ErrFilestoreDestination) || source.Len() != 0 {
			t.Fatalf("Stage = (%+v,%v,%d unread), want %v and consumed bounded input", staged, gotErr, source.Len(), wantErr)
		}
		if wantErr != nil {
			if staged != (filestore.StagedFile{}) {
				t.Fatalf("refused staged receipt = %+v, want zero", staged)
			}
		} else {
			if err := staged.Validate(); err != nil {
				t.Fatal(err)
			}
			count := staged.BytesWritten()
			if count.Validate() != nil || count.Uint64() != uint64(len(payload)) {
				t.Fatalf("staged extent = %v, want admitted %d", count, len(payload))
			}
			path := staged.Path()
			if path.Validate() != nil || path != stagePath {
				t.Fatalf("staged path = %v, want admitted %v", path, stagePath)
			}
			before, err := root.Lstat("stage")
			if err != nil || !before.Mode().IsRegular() || before.Mode().Perm() != 0o600 || before.Size() != int64(len(payload)) {
				t.Fatalf("staged inode = (%v,%v), want exact regular extent/mode", before, err)
			}
			request := filestore.CommitRequest{Staged: staged, Target: mustRelativePath(t, "target"), Install: filestore.InstallCreate}
			if err := request.Validate(); err != nil {
				t.Fatal(err)
			}
			if err := filestore.Commit(t.Context(), request); err != nil {
				t.Fatalf("Commit = %v, want nil", err)
			}
			after, err := root.Lstat("target")
			if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() || before.ModTime().UnixNano() != after.ModTime().UnixNano() {
				t.Fatalf("committed inode = (%v,%v), want exact staged inode and metadata", after, err)
			}
			data, err := os.ReadFile(filepath.Join(directory, "target"))
			if err != nil || !bytes.Equal(data, payload) {
				t.Fatalf("committed bytes = (%d,%v), want %d exact bytes", len(data), err, len(payload))
			}
			// The consumed receipt cannot issue a second activation acknowledgment.
			err = filestore.Commit(t.Context(), request)
			if !errors.Is(err, core.ErrFilestoreActivation) || errors.Is(err, core.ErrFilestoreActivationIndeterminate) || !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("repeated Commit = %v, want missing staged Activation without indeterminate effect", err)
			}
		}
		if _, err := root.Lstat("stage"); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("stage = %v, want absent after cleanup/activation", err)
		}
		if wantErr != nil {
			if _, err := root.Lstat("target"); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("refused target = %v, want absent", err)
			}
		}
		neighborAfter, err := root.Lstat("neighbor")
		if err != nil || !os.SameFile(neighborBefore, neighborAfter) || neighborBefore.Mode() != neighborAfter.Mode() || neighborBefore.ModTime().UnixNano() != neighborAfter.ModTime().UnixNano() {
			t.Fatalf("neighbor = (%v,%v), want original inode and metadata", neighborAfter, err)
		}
		data, err := os.ReadFile(filepath.Join(directory, "neighbor"))
		if err != nil || !bytes.Equal(data, neighbor) {
			t.Fatalf("neighbor bytes = (%v,%v), want %v", data, err, neighbor)
		}
		entries, err := os.ReadDir(directory)
		if err != nil || len(entries) != wantEntries {
			t.Fatalf("namespace = (%v,%v), want %d entries", entries, err, wantEntries)
		}
	})
}
