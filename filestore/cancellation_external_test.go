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
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

type contextEffectFixture struct {
	run    func(context.Context) error
	verify func()
}

func TestOperationsRejectTerminalAndNilContextsBeforeFilesystemEffects(t *testing.T) {
	t.Parallel()

	contextCases := []struct {
		wantErr error
		context func() context.Context
		name    string
	}{
		{
			name: "cancelled context",
			context: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			wantErr: context.Canceled,
		},
		{
			name:    "nil context",
			context: func() context.Context { return nil },
			wantErr: core.ErrNilContext,
		},
	}
	operations := []struct {
		build func(*testing.T, string) contextEffectFixture
		name  string
	}{
		{name: "ensure directory", build: buildEnsureDirectoryContextFixture},
		{name: "read", build: buildReadContextFixture},
		{name: "write", build: buildWriteContextFixture},
		{name: "stage", build: buildStageContextFixture},
		{name: "commit", build: buildCommitContextFixture},
		{name: "recover", build: buildRecoverContextFixture},
		{name: "discard", build: buildDiscardContextFixture},
		{name: "open append", build: buildOpenAppendContextFixture},
		{name: "rotate append", build: buildRotateAppendContextFixture},
		{name: "remove", build: buildRemoveContextFixture},
	}
	for _, operation := range operations {
		for _, contextCase := range contextCases {
			t.Run(operation.name+"/"+contextCase.name, func(t *testing.T) {
				t.Parallel()

				rootDirectory := t.TempDir()
				fixture := operation.build(t, rootDirectory)
				gotErr := fixture.run(contextCase.context())
				if !errors.Is(gotErr, contextCase.wantErr) {
					t.Fatalf("%s error = %v, want %v", operation.name, gotErr, contextCase.wantErr)
				}
				fixture.verify()
			})
		}
	}
}

func TestStageCancellationAfterRealPipeBytesCleansOwnedTemporary(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	root := requireTestRoot(t, rootDirectory)
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil && !errors.Is(closeErr, os.ErrClosed) {
			t.Errorf("pipe reader Close() error = %v, want nil or %v", closeErr, os.ErrClosed)
		}
		if closeErr := writer.Close(); closeErr != nil && !errors.Is(closeErr, os.ErrClosed) {
			t.Errorf("pipe writer Close() error = %v, want nil or %v", closeErr, os.ErrClosed)
		}
	})
	payload := deterministicPayload(1 << 20)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	request := filestore.StageRequest{
		Source:       reader,
		Temporary:    filestore.Location{Root: root, Path: mustRelativePath(t, ".stage")},
		Mode:         0o600,
		MaximumBytes: mustByteCount(t, uint64(len(payload)+1)),
	}
	stageDone := make(chan error, 1)
	go func() {
		_, stageErr := filestore.Stage(ctx, request)
		stageDone <- stageErr
	}()
	writeDone := make(chan error, 1)
	go func() {
		count, writeErr := writer.Write(payload)
		if writeErr == nil && count != len(payload) {
			writeErr = io.ErrShortWrite
		}
		writeDone <- writeErr
	}()
	writeErr, writeFinished := receiveOwnedError(writeDone, time.Minute)
	if !writeFinished {
		cancel()
		_ = writer.Close()
		_ = reader.Close()
		_, writerExited := receiveOwnedError(writeDone, time.Minute)
		_, stageExited := receiveOwnedError(stageDone, time.Minute)
		t.Fatalf(
			"pipe writer completion exceeded %s; after cancellation writer exited=%t stage exited=%t",
			time.Minute,
			writerExited,
			stageExited,
		)
	}
	if writeErr != nil {
		cancel()
		_ = writer.Close()
		_ = reader.Close()
		_, stageExited := receiveOwnedError(stageDone, time.Minute)
		t.Fatalf("pipe Write() error = %v, want nil; stage exited after cancellation=%t", writeErr, stageExited)
	}
	cancel()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	gotErr, stageFinished := receiveOwnedError(stageDone, time.Minute)
	if !stageFinished {
		_ = reader.Close()
		gotErr, stageFinished = receiveOwnedError(stageDone, time.Minute)
		if !stageFinished {
			t.Fatalf("Stage() did not exit within %s after cancellation and pipe close", time.Minute)
		}
		t.Fatalf("Stage() required source closure after exceeding %s; terminal error = %v", time.Minute, gotErr)
	}
	if !errors.Is(gotErr, context.Canceled) {
		t.Fatalf("Stage() after streamed-byte cancellation error = %v, want %v", gotErr, context.Canceled)
	}
	requireDirectoryEntryNames(t, rootDirectory, nil)
}

func receiveOwnedError(channel <-chan error, timeout time.Duration) (error, bool) {
	select {
	case err := <-channel:
		return err, true
	case <-time.After(timeout):
		return nil, false
	}
}

func buildEnsureDirectoryContextFixture(t *testing.T, rootDirectory string) contextEffectFixture {
	t.Helper()

	root := requireTestRoot(t, rootDirectory)
	return contextEffectFixture{
		run: func(ctx context.Context) error {
			return filestore.EnsureDirectory(ctx, filestore.DirectoryRequest{
				Location: filestore.Location{Root: root, Path: mustRelativePath(t, filepath.Join("a", "b"))},
				Mode:     0o700,
			})
		},
		verify: func() {
			if _, err := os.Stat(filepath.Join(rootDirectory, "a")); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("Stat(a) error = %v, want %v", err, fs.ErrNotExist)
			}
		},
	}
}

func buildReadContextFixture(t *testing.T, rootDirectory string) contextEffectFixture {
	t.Helper()

	root := requireTestRoot(t, rootDirectory)
	if err := os.WriteFile(filepath.Join(rootDirectory, "target"), []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	var destination bytes.Buffer
	return contextEffectFixture{
		run: func(ctx context.Context) error {
			_, err := filestore.Read(ctx, filestore.ReadRequest{
				Destination:  &destination,
				Location:     filestore.Location{Root: root, Path: mustRelativePath(t, "target")},
				MaximumBytes: mustByteCount(t, 6),
			})
			return err
		},
		verify: func() {
			if destination.Len() != 0 {
				t.Fatalf("cancelled Read() destination length = %d, want 0", destination.Len())
			}
		},
	}
}

func buildWriteContextFixture(t *testing.T, rootDirectory string) contextEffectFixture {
	t.Helper()

	root := requireTestRoot(t, rootDirectory)
	return contextEffectFixture{
		run: func(ctx context.Context) error {
			_, err := filestore.Write(ctx, filestore.WriteRequest{
				Source:       bytes.NewReader([]byte("source")),
				Location:     filestore.Location{Root: root, Path: mustRelativePath(t, "target")},
				Temporary:    mustRelativePath(t, ".target-stage"),
				Mode:         0o600,
				Install:      filestore.InstallCreate,
				MaximumBytes: mustByteCount(t, 6),
			})
			return err
		},
		verify: func() {
			requireDirectoryEntryNames(t, rootDirectory, nil)
		},
	}
}

func buildStageContextFixture(t *testing.T, rootDirectory string) contextEffectFixture {
	t.Helper()

	root := requireTestRoot(t, rootDirectory)
	return contextEffectFixture{
		run: func(ctx context.Context) error {
			_, err := filestore.Stage(ctx, filestore.StageRequest{
				Source:       bytes.NewReader([]byte("source")),
				Temporary:    filestore.Location{Root: root, Path: mustRelativePath(t, ".target-stage")},
				Mode:         0o600,
				MaximumBytes: mustByteCount(t, 6),
			})
			return err
		},
		verify: func() {
			requireDirectoryEntryNames(t, rootDirectory, nil)
		},
	}
}

func buildCommitContextFixture(t *testing.T, rootDirectory string) contextEffectFixture {
	t.Helper()

	root := requireTestRoot(t, rootDirectory)
	staged := mustStage(t, root, ".target-stage", "source")
	return contextEffectFixture{
		run: func(ctx context.Context) error {
			return filestore.Commit(ctx, filestore.CommitRequest{
				Staged:  staged,
				Target:  mustRelativePath(t, "target"),
				Install: filestore.InstallCreate,
			})
		},
		verify: func() {
			requireDirectoryEntryNames(t, rootDirectory, []string{".target-stage"})
		},
	}
}

func buildRecoverContextFixture(t *testing.T, rootDirectory string) contextEffectFixture {
	t.Helper()

	root := requireTestRoot(t, rootDirectory)
	staged := mustStage(t, root, ".target-stage", "source")
	target := mustRelativePath(t, "target")
	if err := root.Link(staged.Path().String(), target.String()); err != nil {
		t.Fatal(err)
	}
	return contextEffectFixture{
		run: func(ctx context.Context) error {
			return filestore.Recover(ctx, filestore.CommitRequest{
				Staged:  staged,
				Target:  target,
				Install: filestore.InstallCreate,
			})
		},
		verify: func() {
			requireDirectoryEntryNames(t, rootDirectory, []string{".target-stage", "target"})
		},
	}
}

func buildDiscardContextFixture(t *testing.T, rootDirectory string) contextEffectFixture {
	t.Helper()

	root := requireTestRoot(t, rootDirectory)
	staged := mustStage(t, root, ".target-stage", "source")
	return contextEffectFixture{
		run: func(ctx context.Context) error {
			return filestore.Discard(ctx, staged)
		},
		verify: func() {
			requireDirectoryEntryNames(t, rootDirectory, []string{".target-stage"})
		},
	}
}

func buildOpenAppendContextFixture(t *testing.T, rootDirectory string) contextEffectFixture {
	t.Helper()

	root := requireTestRoot(t, rootDirectory)
	return contextEffectFixture{
		run: func(ctx context.Context) error {
			file, err := filestore.OpenAppend(ctx, filestore.AppendRequest{
				Location: filestore.Location{Root: root, Path: mustRelativePath(t, "ledger")},
				Mode:     0o600,
			})
			if file != nil {
				if closeErr := file.Close(); closeErr != nil {
					return errors.Join(err, closeErr)
				}
			}
			return err
		},
		verify: func() {
			requireDirectoryEntryNames(t, rootDirectory, nil)
		},
	}
}

func buildRotateAppendContextFixture(t *testing.T, rootDirectory string) contextEffectFixture {
	t.Helper()

	root := requireTestRoot(t, rootDirectory)
	outgoing, err := filestore.OpenAppend(t.Context(), filestore.AppendRequest{
		Location: filestore.Location{Root: root, Path: mustRelativePath(t, "ledger-0001")},
		Mode:     0o600,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := outgoing.Close(); closeErr != nil && !errors.Is(closeErr, os.ErrClosed) {
			t.Errorf("outgoing Close() error = %v, want nil or %v", closeErr, os.ErrClosed)
		}
	})
	return contextEffectFixture{
		run: func(ctx context.Context) error {
			incoming, rotateErr := filestore.RotateAppend(ctx, filestore.RotationRequest{
				Outgoing: outgoing,
				Incoming: filestore.AppendRequest{
					Location: filestore.Location{Root: root, Path: mustRelativePath(t, "ledger-0002")},
					Mode:     0o600,
				},
			})
			if incoming != nil {
				if closeErr := incoming.Close(); closeErr != nil {
					return errors.Join(rotateErr, closeErr)
				}
			}
			return rotateErr
		},
		verify: func() {
			if _, err := outgoing.Write([]byte("still caller owned")); err != nil {
				t.Fatalf("outgoing Write() after rejected rotation error = %v, want nil", err)
			}
			requireDirectoryEntryNames(t, rootDirectory, []string{"ledger-0001"})
		},
	}
}

func buildRemoveContextFixture(t *testing.T, rootDirectory string) contextEffectFixture {
	t.Helper()

	root := requireTestRoot(t, rootDirectory)
	if err := os.WriteFile(filepath.Join(rootDirectory, "target"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	return contextEffectFixture{
		run: func(ctx context.Context) error {
			return filestore.Remove(ctx, filestore.RemovalRequest{
				Location: filestore.Location{Root: root, Path: mustRelativePath(t, "target")},
			})
		},
		verify: func() {
			got, err := os.ReadFile(filepath.Join(rootDirectory, "target"))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != "keep" {
				t.Fatalf("target after rejected Remove() = %q, want %q", got, "keep")
			}
		},
	}
}

func requireTestRoot(t *testing.T, rootDirectory string) *os.Root {
	t.Helper()

	root, err := os.OpenRoot(rootDirectory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("os.Root.Close() error = %v, want nil", closeErr)
		}
	})
	return root
}

func mustStage(t *testing.T, root *os.Root, path, content string) filestore.StagedFile {
	t.Helper()

	staged, err := filestore.Stage(t.Context(), filestore.StageRequest{
		Source:       bytes.NewReader([]byte(content)),
		Temporary:    filestore.Location{Root: root, Path: mustRelativePath(t, path)},
		Mode:         0o600,
		MaximumBytes: mustByteCount(t, uint64(max(len(content), 1))),
	})
	if err != nil {
		t.Fatalf("Stage() error = %v, want nil", err)
	}
	return staged
}

func requireDirectoryEntryNames(t *testing.T, directory string, want []string) {
	t.Helper()

	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(entries))
	for index, entry := range entries {
		got[index] = entry.Name()
	}
	if len(got) != len(want) {
		t.Fatalf("directory entries = %v, want %v", got, want)
	}
	for index := range got {
		if got[index] != want[index] {
			t.Fatalf("directory entries = %v, want %v", got, want)
		}
	}
}
