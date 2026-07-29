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

func TestStageSupportsTargetLateCreateWithoutAWriterWrapper(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	if err := os.Mkdir(filepath.Join(rootDirectory, "objects"), 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(rootDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("os.Root.Close() error = %v, want nil", closeErr)
		}
	}()
	payload := []byte("digest known after streaming")
	staged, err := filestore.Stage(t.Context(), filestore.StageRequest{
		Source: bytes.NewReader(payload),
		Temporary: filestore.Location{
			Root: root,
			Path: mustRelativePath(t, filepath.Join("objects", ".object-stage")),
		},
		Mode:         0o640,
		MaximumBytes: mustByteCount(t, uint64(len(payload))),
	})
	if err != nil {
		t.Fatalf("Stage() error = %v, want nil", err)
	}
	if staged.BytesWritten().Uint64() != uint64(len(payload)) {
		t.Fatalf("StagedFile.BytesWritten() = %d, want %d", staged.BytesWritten().Uint64(), len(payload))
	}
	if err := filestore.Commit(t.Context(), filestore.CommitRequest{
		Staged:  staged,
		Target:  mustRelativePath(t, filepath.Join("objects", "digest")),
		Install: filestore.InstallCreate,
	}); err != nil {
		t.Fatalf("Commit() error = %v, want nil", err)
	}
	got, err := os.ReadFile(filepath.Join(rootDirectory, "objects", "digest"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("committed bytes = %q, want %q", got, payload)
	}
	if _, err := os.Stat(filepath.Join(rootDirectory, staged.Path().String())); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Stat(staged path) error = %v, want %v", err, fs.ErrNotExist)
	}
}

func TestRecoverFinishesRealCreateOnlyPartialEffect(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	root, err := os.OpenRoot(rootDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("os.Root.Close() error = %v, want nil", closeErr)
		}
	}()
	staged, err := filestore.Stage(t.Context(), filestore.StageRequest{
		Source: bytes.NewReader([]byte("payload")),
		Temporary: filestore.Location{
			Root: root,
			Path: mustRelativePath(t, ".target-stage"),
		},
		Mode:         0o600,
		MaximumBytes: mustByteCount(t, 7),
	})
	if err != nil {
		t.Fatalf("Stage() error = %v, want nil", err)
	}
	target := mustRelativePath(t, "target")
	if err := root.Link(staged.Path().String(), target.String()); err != nil {
		t.Fatalf("os.Root.Link() partial effect error = %v, want nil", err)
	}
	request := filestore.CommitRequest{
		Staged:  staged,
		Target:  target,
		Install: filestore.InstallCreate,
	}
	if err := filestore.Recover(t.Context(), request); err != nil {
		t.Fatalf("Recover() error = %v, want nil", err)
	}
	got, err := os.ReadFile(filepath.Join(rootDirectory, "target"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "payload" {
		t.Fatalf("recovered target = %q, want %q", got, "payload")
	}
	if _, err := os.Stat(filepath.Join(rootDirectory, staged.Path().String())); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Stat(staged path after recovery) error = %v, want %v", err, fs.ErrNotExist)
	}
}

func TestRecoverFinishesRealReplacePartialEffectAndIsIdempotent(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	root := requireTestRoot(t, rootDirectory)
	if err := os.WriteFile(filepath.Join(rootDirectory, "target"), []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	staged := mustStage(t, root, ".target-stage", "replacement")
	target := mustRelativePath(t, "target")
	if err := root.Rename(staged.Path().String(), target.String()); err != nil {
		t.Fatalf("os.Root.Rename() partial effect error = %v, want nil", err)
	}
	request := filestore.CommitRequest{
		Staged:  staged,
		Target:  target,
		Install: filestore.InstallReplace,
	}
	for attempt := range 3 {
		if err := filestore.Recover(t.Context(), request); err != nil {
			t.Fatalf("Recover() attempt %d error = %v, want nil", attempt, err)
		}
	}
	got, err := os.ReadFile(filepath.Join(rootDirectory, "target"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "replacement" {
		t.Fatalf("recovered replacement target = %q, want %q", got, "replacement")
	}
	requireDirectoryEntryNames(t, rootDirectory, []string{"target"})
}

func TestRecoverRejectsDifferentTargetAfterAmbiguousReplace(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	root := requireTestRoot(t, rootDirectory)
	if err := os.WriteFile(filepath.Join(rootDirectory, "target"), []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	staged := mustStage(t, root, ".target-stage", "replacement")
	target := mustRelativePath(t, "target")
	if err := root.Rename(staged.Path().String(), target.String()); err != nil {
		t.Fatal(err)
	}
	activatedHandle, err := os.Open(filepath.Join(rootDirectory, "target"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := activatedHandle.Close(); closeErr != nil {
			t.Errorf("activated handle Close() error = %v, want nil", closeErr)
		}
	})
	if err := os.Remove(filepath.Join(rootDirectory, "target")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootDirectory, "target"), []byte("different owner"), 0o600); err != nil {
		t.Fatal(err)
	}
	gotErr := filestore.Recover(t.Context(), filestore.CommitRequest{
		Staged:  staged,
		Target:  target,
		Install: filestore.InstallReplace,
	})
	if !errors.Is(gotErr, core.ErrFilestoreActivationIndeterminate) {
		t.Fatalf("Recover(different replacement target) error = %v, want %v", gotErr, core.ErrFilestoreActivationIndeterminate)
	}
	got, err := os.ReadFile(filepath.Join(rootDirectory, "target"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "different owner" {
		t.Fatalf("different replacement target = %q, want %q", got, "different owner")
	}
	requireDirectoryEntryNames(t, rootDirectory, []string{"target"})
}

func TestRecoverRejectsUnrelatedCreateTargetWithoutConsumingStage(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	root := requireTestRoot(t, rootDirectory)
	staged := mustStage(t, root, ".target-stage", "candidate")
	if err := os.WriteFile(filepath.Join(rootDirectory, "target"), []byte("winner"), 0o600); err != nil {
		t.Fatal(err)
	}
	gotErr := filestore.Recover(t.Context(), filestore.CommitRequest{
		Staged:  staged,
		Target:  mustRelativePath(t, "target"),
		Install: filestore.InstallCreate,
	})
	if !errors.Is(gotErr, core.ErrFilestoreConflict) || !errors.Is(gotErr, os.ErrExist) {
		t.Fatalf("Recover(unrelated create target) error = %v, want %v and %v", gotErr, core.ErrFilestoreConflict, os.ErrExist)
	}
	gotTarget, err := os.ReadFile(filepath.Join(rootDirectory, "target"))
	if err != nil {
		t.Fatal(err)
	}
	gotStage, err := os.ReadFile(filepath.Join(rootDirectory, ".target-stage"))
	if err != nil {
		t.Fatal(err)
	}
	if string(gotTarget) != "winner" || string(gotStage) != "candidate" {
		t.Fatalf("Recover conflict bytes = target:%q stage:%q, want %q/%q", gotTarget, gotStage, "winner", "candidate")
	}
}

func TestDiscardRemovesOnlyTheNamedStage(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	root, err := os.OpenRoot(rootDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("os.Root.Close() error = %v, want nil", closeErr)
		}
	}()
	staged, err := filestore.Stage(t.Context(), filestore.StageRequest{
		Source: bytes.NewReader([]byte("discard")),
		Temporary: filestore.Location{
			Root: root,
			Path: mustRelativePath(t, ".discard-stage"),
		},
		Mode:         0o600,
		MaximumBytes: mustByteCount(t, 7),
	})
	if err != nil {
		t.Fatalf("Stage() error = %v, want nil", err)
	}
	if err := os.WriteFile(filepath.Join(rootDirectory, "unrelated"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := filestore.Discard(t.Context(), staged); err != nil {
		t.Fatalf("Discard() error = %v, want nil", err)
	}
	if _, err := os.Stat(filepath.Join(rootDirectory, staged.Path().String())); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Stat(discarded stage) error = %v, want %v", err, fs.ErrNotExist)
	}
	got, err := os.ReadFile(filepath.Join(rootDirectory, "unrelated"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "keep" {
		t.Fatalf("unrelated bytes = %q, want %q", got, "keep")
	}
	if err := filestore.Discard(t.Context(), staged); err != nil {
		t.Fatalf("repeated Discard() error = %v, want nil", err)
	}
}

func TestStageRejectsOverflowWithoutResidue(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	root, err := os.OpenRoot(rootDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("os.Root.Close() error = %v, want nil", closeErr)
		}
	}()
	_, gotErr := filestore.Stage(t.Context(), filestore.StageRequest{
		Source: bytes.NewReader([]byte("12345")),
		Temporary: filestore.Location{
			Root: root,
			Path: mustRelativePath(t, ".overflow-stage"),
		},
		Mode:         0o600,
		MaximumBytes: mustByteCount(t, 4),
	})
	if !errors.Is(gotErr, core.ErrFilestoreSize) {
		t.Fatalf("Stage() error = %v, want %v", gotErr, core.ErrFilestoreSize)
	}
	entries, err := os.ReadDir(rootDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("stage directory entries after overflow = %v, want none", entries)
	}
}
