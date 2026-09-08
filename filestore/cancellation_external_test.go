package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

type contextEffectDoor uint8

const (
	contextEffectEnsure contextEffectDoor = iota
	contextEffectRead
	contextEffectWrite
	contextEffectStage
	contextEffectCommit
	contextEffectRecover
	contextEffectDiscard
	contextEffectAppend
	contextEffectRotate
	contextEffectRemove
)

func TestOperationsRejectTerminalAndNilContextsBeforeFilesystemEffects(t *testing.T) {
	t.Parallel()
	for _, operation := range []struct {
		name string
		door contextEffectDoor
	}{
		{"ensure cannot create a directory prefix", contextEffectEnsure},
		{"read cannot consume source bytes", contextEffectRead},
		{"write cannot stage caller bytes", contextEffectWrite},
		{"stage cannot return a receipt", contextEffectStage},
		{"commit cannot consume a synchronized stage", contextEffectCommit},
		{"recover cannot settle partially linked custody", contextEffectRecover},
		{"discard cannot remove caller custody", contextEffectDiscard},
		{"append cannot acquire an incoming handle", contextEffectAppend},
		{"rotation cannot close outgoing caller custody", contextEffectRotate},
		{"remove cannot unlink existing bytes", contextEffectRemove},
	} {
		for _, tc := range []struct {
			name       string
			nilContext bool
			wantErr    error
		}{
			{name: "canceled context", wantErr: context.Canceled},
			{name: "nil context", nilContext: true, wantErr: core.ErrNilContext},
		} {
			t.Run(operation.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				directory := t.TempDir()
				root := requireTestRoot(t, directory)
				payload := []byte{0, 255, 1, 127}
				source := bytes.NewReader(payload)
				var destination bytes.Buffer
				path := mustRelativePath(t, "target")
				stagePath := mustRelativePath(t, "stage")
				maximum := mustByteCount(t, uint64(len(payload)))
				if err := os.WriteFile(filepath.Join(directory, "neighbor"), payload, 0o640); err != nil {
					t.Fatal(err)
				}
				if operation.door == contextEffectRead || operation.door == contextEffectRemove {
					if err := os.WriteFile(filepath.Join(directory, "target"), payload, 0o600); err != nil {
						t.Fatal(err)
					}
				}
				var staged filestore.StagedFile
				if operation.door == contextEffectCommit || operation.door == contextEffectRecover || operation.door == contextEffectDiscard {
					var err error
					staged, err = filestore.Stage(t.Context(), filestore.StageRequest{Source: bytes.NewReader(payload), Temporary: filestore.Location{Root: root, Path: stagePath}, Mode: 0o600, MaximumBytes: maximum})
					if err != nil {
						t.Fatal(err)
					}
					if operation.door == contextEffectRecover {
						if err := root.Link(stagePath.String(), path.String()); err != nil {
							t.Fatal(err)
						}
					}
				}
				var outgoing *os.File
				if operation.door == contextEffectRotate {
					var err error
					outgoing, err = filestore.OpenAppend(t.Context(), filestore.AppendRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, "outgoing")}, Mode: 0o600, Append: filestore.AppendCreate})
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if err := outgoing.Close(); err != nil {
							t.Error(err)
						}
					})
					if n, err := outgoing.Write(payload); err != nil || n != len(payload) {
						t.Fatalf("outgoing seed = (%d,%v), want %d bytes", n, err, len(payload))
					}
				}
				before, err := removalFixtureSnapshot(directory)
				if err != nil {
					t.Fatal(err)
				}
				identities := make([]fs.FileInfo, len(before))
				for i, entry := range before {
					identities[i], err = os.Lstat(filepath.Join(directory, entry.name))
					if err != nil {
						t.Fatal(err)
					}
				}
				ctx, cancel := context.WithCancel(t.Context())
				cancel()
				if tc.nilContext {
					ctx = nil
				}
				var gotErr error
				var count core.ByteLength
				var gotStage filestore.StagedFile
				var recovery filestore.CommitRequest
				var incoming *os.File
				location := filestore.Location{Root: root, Path: path}
				switch operation.door {
				case contextEffectEnsure:
					gotErr = filestore.EnsureDirectory(ctx, filestore.DirectoryRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, filepath.Join("parent", "leaf"))}, Mode: 0o700})
				case contextEffectRead:
					count, gotErr = filestore.Read(ctx, filestore.ReadRequest{Location: location, Destination: &destination, MaximumBytes: maximum})
				case contextEffectWrite:
					recovery, gotErr = filestore.Write(ctx, filestore.WriteRequest{Source: source, Location: location, Temporary: stagePath, Mode: 0o600, Install: filestore.InstallCreate, MaximumBytes: maximum})
				case contextEffectStage:
					gotStage, gotErr = filestore.Stage(ctx, filestore.StageRequest{Source: source, Temporary: filestore.Location{Root: root, Path: stagePath}, Mode: 0o600, MaximumBytes: maximum})
				case contextEffectCommit:
					gotErr = filestore.Commit(ctx, filestore.CommitRequest{Staged: staged, Target: path, Install: filestore.InstallCreate})
				case contextEffectRecover:
					gotErr = filestore.Recover(ctx, filestore.CommitRequest{Staged: staged, Target: path, Install: filestore.InstallCreate})
				case contextEffectDiscard:
					gotErr = filestore.Discard(ctx, staged)
				case contextEffectAppend:
					incoming, gotErr = filestore.OpenAppend(ctx, filestore.AppendRequest{Location: location, Mode: 0o600, Append: filestore.AppendCreate})
				case contextEffectRotate:
					incoming, gotErr = filestore.RotateAppend(ctx, filestore.RotationRequest{Outgoing: outgoing, Incoming: filestore.AppendRequest{Location: location, Mode: 0o600, Append: filestore.AppendCreate}})
				case contextEffectRemove:
					gotErr = filestore.Remove(ctx, filestore.RemovalRequest{Location: location})
				default:
					t.Fatalf("door = %v, want declared fixture", operation.door)
				}
				if incoming != nil && incoming != outgoing {
					t.Cleanup(func() {
						if err := incoming.Close(); err != nil && !errors.Is(err, fs.ErrClosed) {
							t.Error(err)
						}
					})
				}
				if !errors.Is(gotErr, tc.wantErr) || count != (core.ByteLength{}) || gotStage != (filestore.StagedFile{}) || recovery != (filestore.CommitRequest{}) || incoming != nil || destination.Len() != 0 || source.Len() != len(payload) {
					t.Fatalf("refusal = (%v,count %v,stage %+v,recovery %+v,handle %v,output %d,unread %d), want %v and zero effects/results", gotErr, count, gotStage, recovery, incoming, destination.Len(), source.Len(), tc.wantErr)
				}
				for _, class := range []error{core.ErrFilestoreSource, core.ErrFilestoreDestination, core.ErrFilestoreActivation, core.ErrFilestoreCleanup, core.ErrFilestoreConflict, core.ErrFilestoreSize} {
					if errors.Is(gotErr, class) {
						t.Fatalf("context refusal = %v, want no %v execution classification", gotErr, class)
					}
				}
				after, err := removalFixtureSnapshot(directory)
				if err != nil || len(after) != len(before) {
					t.Fatalf("namespace = (%v,%v), want preserved entries", after, err)
				}
				for i, entry := range before {
					info, err := os.Lstat(filepath.Join(directory, entry.name))
					if err != nil || !os.SameFile(identities[i], info) || identities[i].ModTime().UnixNano() != info.ModTime().UnixNano() || after[i].name != entry.name || after[i].mode != entry.mode || after[i].target != entry.target || !bytes.Equal(after[i].data, entry.data) {
						t.Fatalf("entry %d = (%+v,%v), want exact original bytes, inode and metadata %+v", i, after[i], err, entry)
					}
				}
				if outgoing != nil {
					probe := []byte{99}
					if n, err := outgoing.Write(probe); err != nil || n != len(probe) {
						t.Fatalf("retained outgoing write = (%d,%v), want %d", n, err, len(probe))
					}
					data, err := os.ReadFile(filepath.Join(directory, "outgoing"))
					want := append(bytes.Clone(payload), probe...)
					if err != nil || !bytes.Equal(data, want) {
						t.Fatalf("retained append bytes = (%v,%v), want %v", data, err, want)
					}
				}
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
	writeErr, writeFinished := receiveOwnedError(t, writeDone, time.Minute)
	if !writeFinished {
		cancel()
		_ = writer.Close()
		_ = reader.Close()
		_, writerExited := receiveOwnedError(t, writeDone, time.Minute)
		_, stageExited := receiveOwnedError(t, stageDone, time.Minute)
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
		_, stageExited := receiveOwnedError(t, stageDone, time.Minute)
		t.Fatalf("pipe Write() error = %v, want nil; stage exited after cancellation=%t", writeErr, stageExited)
	}
	cancel()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	gotErr, stageFinished := receiveOwnedError(t, stageDone, time.Minute)
	if !stageFinished {
		_ = reader.Close()
		gotErr, stageFinished = receiveOwnedError(t, stageDone, time.Minute)
		if !stageFinished {
			t.Fatalf("Stage() did not exit within %s after cancellation and pipe close", time.Minute)
		}
		t.Fatalf("Stage() required source closure after exceeding %s; terminal error = %v", time.Minute, gotErr)
	}
	if !errors.Is(gotErr, context.Canceled) {
		t.Fatalf("Stage() after streamed-byte cancellation error = %v, want %v", gotErr, context.Canceled)
	}
	{
		got, err := directoryEntryNames(rootDirectory)
		want := []string(nil)
		if err != nil || !slices.Equal(got, want) {
			t.Fatalf("directory entries = (%v,%v), want %v", got, err, want)
		}
	}
}

func receiveOwnedError(t testing.TB, channel <-chan error, timeout time.Duration) (error, bool) {
	t.Helper()
	ctx, cancel := newFilesystemBackstop(context.Background(), t, timeout)
	defer cancel()
	select {
	case err := <-channel:
		return err, true
	case <-ctx.Done():
		return nil, false
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

func directoryEntryNames(directory string) ([]string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	got := make([]string, len(entries))
	for i, entry := range entries {
		got[i] = entry.Name()
	}
	return got, nil
}
