package filestore_test

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// The measured lifecycle creates exactly 128 bytes through the exposed Go file,
// finishes custody, atomically activates that inode, independently reads it,
// then durably removes the target. All those native effects are timed work.
func BenchmarkStageDestinationCommitRemoveExactBytes(b *testing.B) {
	directory := b.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			b.Error(err)
		}
	}()
	temporary, err := core.ParseRelativePath("stage")
	if err != nil {
		b.Fatal(err)
	}
	target, err := core.ParseRelativePath("target")
	if err != nil {
		b.Fatal(err)
	}
	payload := bytes.Repeat([]byte{0, 255, 7, 31}, 32)
	extent, err := core.NewByteLength(uint64(len(payload)))
	if err != nil {
		b.Fatal(err)
	}
	plan := filestore.ActivationRequest{Temporary: filestore.Location{Root: root, Path: temporary}, Target: target, ExpectedBytes: new(extent), Mode: 0o600, Install: filestore.InstallCreate}
	removal := filestore.RemovalRequest{Location: filestore.Location{Root: root, Path: target}}
	if err := plan.Validate(); err != nil {
		b.Fatal(err)
	}
	if err := removal.Validate(); err != nil {
		b.Fatal(err)
	}
	gotBytes := make([]byte, len(payload)+1)
	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	for b.Loop() {
		destination, err := filestore.OpenStageDestination(b.Context(), plan.StageDestination())
		if err != nil {
			b.Fatal(err)
		}
		file, err := destination.File()
		if err != nil {
			b.Fatal(err)
		}
		if n, err := file.Write(payload); err != nil || n != len(payload) {
			b.Fatalf("write = (%d,%v), want %d", n, err, len(payload))
		}
		original, err := file.Stat()
		if err != nil {
			b.Fatal(err)
		}
		staged, err := filestore.FinishStageDestination(b.Context(), destination)
		if err != nil || staged.BytesWritten() != extent {
			b.Fatalf("stage = (%+v,%v), want exact receipt", staged, err)
		}
		if _, err := file.Stat(); !errors.Is(err, fs.ErrClosed) {
			b.Fatalf("producer = %v, want closed", err)
		}
		commit, err := plan.CommitRequest(staged)
		if err != nil {
			b.Fatal(err)
		}
		if err := filestore.Commit(b.Context(), commit); err != nil {
			b.Fatal(err)
		}
		reader, err := root.Open(target.String())
		if err != nil {
			b.Fatal(err)
		}
		installed, statErr := reader.Stat()
		n, readErr := io.ReadFull(reader, gotBytes)
		closeErr := reader.Close()
		if statErr != nil || !errors.Is(readErr, io.ErrUnexpectedEOF) || closeErr != nil || n != len(payload) || !bytes.Equal(gotBytes[:n], payload) || !os.SameFile(original, installed) || installed.Mode().Perm() != plan.Mode {
			b.Fatalf("installed = (%v,%d,%v,%v,%v), want exact inode, bytes and mode", installed, n, statErr, readErr, closeErr)
		}
		if err := filestore.Remove(b.Context(), removal); err != nil {
			b.Fatal(err)
		}
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 0 {
		b.Fatalf("settled namespace = (%v,%v), want empty", entries, err)
	}
}
