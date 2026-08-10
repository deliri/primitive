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

// TestStageDestinationDurableWriterLayerTriad proves the real handoff layer:
// a standard-library file receives fragmented bytes and commits atomically, a
// substituted custody name fails before a receipt escapes, and abandonment
// after no producer work leaves no filesystem noise.
func TestStageDestinationDurableWriterLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive standard file stream seals exact bytes mode and namespace", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir()
		root := requireTestRoot(t, directory)
		payload := deterministicPayload((32 << 10) + 7)
		destination, err := filestore.OpenStageDestination(t.Context(), filestore.StageDestinationRequest{
			Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, ".incoming")},
			Mode:      0o640, ExpectedBytes: stageDestinationLength(t, uint64(len(payload))),
		})
		if err != nil {
			t.Fatalf("OpenStageDestination() error = %v, want nil", err)
		}
		file, err := destination.File()
		if err != nil {
			t.Fatalf("StageDestination.File() error = %v, want nil", err)
		}
		for offset := 0; offset < len(payload); {
			end := min(offset+8191, len(payload))
			gotWritten, gotErr := file.Write(payload[offset:end])
			wantWritten := end - offset
			if gotErr != nil || gotWritten != wantWritten {
				t.Fatalf("os.File.Write(offset %d) = (%d, %v), want (%d, nil)",
					offset, gotWritten, gotErr, wantWritten)
			}
			offset = end
		}
		staged, err := filestore.FinishStageDestination(t.Context(), destination)
		if err != nil {
			t.Fatalf("FinishStageDestination() error = %v, want nil", err)
		}
		if err := filestore.Commit(t.Context(), filestore.CommitRequest{
			Staged: staged, Target: mustRelativePath(t, "evidence.bin"), Install: filestore.InstallCreate,
		}); err != nil {
			t.Fatalf("Commit() error = %v, want nil", err)
		}
		got, err := os.ReadFile(filepath.Join(directory, "evidence.bin"))
		if err != nil {
			t.Fatalf("ReadFile(committed target) error = %v, want nil", err)
		}
		if !bytes.Equal(got, payload) {
			t.Fatalf("committed bytes length = %d, want exact payload length %d", len(got), len(payload))
		}
		info, err := os.Stat(filepath.Join(directory, "evidence.bin"))
		if err != nil {
			t.Fatalf("Stat(committed target) error = %v, want nil", err)
		}
		if gotMode := info.Mode().Perm(); gotMode != 0o640 {
			t.Fatalf("committed target mode = %v, want %v", gotMode, fs.FileMode(0o640))
		}
		if _, err := os.Stat(filepath.Join(directory, ".incoming")); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("Stat(stage after commit) error = %v, want errors.Is %v", err, fs.ErrNotExist)
		}
	})

	t.Run("negative substituted custody name fails without deleting stranger", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir()
		root := requireTestRoot(t, directory)
		destination, err := filestore.OpenStageDestination(t.Context(), filestore.StageDestinationRequest{
			Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, ".owned")},
			Mode:      0o600, ExpectedBytes: stageDestinationLength(t, 1),
		})
		if err != nil {
			t.Fatalf("OpenStageDestination() error = %v, want nil", err)
		}
		file, err := destination.File()
		if err != nil {
			t.Fatalf("StageDestination.File() error = %v, want nil", err)
		}
		if gotWritten, gotErr := file.Write([]byte{1}); gotWritten != 1 || gotErr != nil {
			t.Fatalf("os.File.Write() = (%d, %v), want (1, nil)", gotWritten, gotErr)
		}
		if err := root.Rename(".owned", ".moved"); err != nil {
			t.Fatalf("os.Root.Rename(owned, moved) error = %v, want nil", err)
		}
		replacement, err := root.OpenFile(".owned", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			t.Fatalf("os.Root.OpenFile(replacement) error = %v, want nil", err)
		}
		if _, err := replacement.Write([]byte{2}); err != nil {
			t.Fatalf("replacement.Write() error = %v, want nil", err)
		}
		if err := replacement.Close(); err != nil {
			t.Fatalf("replacement.Close() error = %v, want nil", err)
		}
		gotStaged, gotErr := filestore.FinishStageDestination(t.Context(), destination)
		if !errors.Is(gotErr, core.ErrFilestoreActivationIndeterminate) ||
			!errors.Is(gotErr, core.ErrFilestoreCleanup) ||
			!errors.Is(gotErr, core.ErrFilestoreConflict) {
			t.Fatalf("FinishStageDestination(substituted name) error = %v, want errors.Is %v, %v, and %v",
				gotErr, core.ErrFilestoreActivationIndeterminate, core.ErrFilestoreCleanup, core.ErrFilestoreConflict)
		}
		if gotErr := gotStaged.Validate(); !errors.Is(gotErr, core.ErrFilestoreContract) {
			t.Fatalf("rejected StagedFile.Validate() error = %v, want errors.Is %v", gotErr, core.ErrFilestoreContract)
		}
		got, err := os.ReadFile(filepath.Join(directory, ".owned"))
		if err != nil {
			t.Fatalf("ReadFile(replacement) error = %v, want nil", err)
		}
		if !bytes.Equal(got, []byte{2}) {
			t.Fatalf("replacement bytes = %v, want [2]", got)
		}
	})

	t.Run("neutral abandoned destination after no writes leaves no names", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir()
		root := requireTestRoot(t, directory)
		destination, err := filestore.OpenStageDestination(t.Context(), filestore.StageDestinationRequest{
			Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, ".unused")},
			Mode:      0o600, ExpectedBytes: stageDestinationLength(t, 0),
		})
		if err != nil {
			t.Fatalf("OpenStageDestination() error = %v, want nil", err)
		}
		if err := filestore.AbandonStageDestination(destination); err != nil {
			t.Fatalf("AbandonStageDestination() error = %v, want nil", err)
		}
		for _, name := range []string{".unused", "evidence.bin"} {
			if _, err := os.Stat(filepath.Join(directory, name)); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("Stat(%q) error = %v, want errors.Is %v", name, err, fs.ErrNotExist)
			}
		}
	})
}

func stageDestinationLength(t *testing.T, value uint64) core.ByteLength {
	t.Helper()
	length, err := core.NewByteLength(value)
	if err != nil {
		t.Fatalf("core.NewByteLength(%d) error = %v, want nil", value, err)
	}
	return length
}
