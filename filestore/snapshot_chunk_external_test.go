package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func withSnapshotChunkFile(t testing.TB, use func(context.Context, filestore.ScratchReplayFile)) {
	t.Helper()
	directory, err := core.ParseAbsolutePath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root, err := filestore.OpenRoot(t.Context(), directory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	}()
	name, err := core.ParseRelativePath("capture")
	if err != nil {
		t.Fatal(err)
	}
	scope, err := filestore.WithScratchReplayScope(t.Context(), filestore.ScratchReplayScopeRequest{Scratch: filestore.ScratchRequest{Location: filestore.Location{Root: root, Path: name}, Mode: 0600}, Use: func(ctx context.Context, file filestore.ScratchReplayFile) error { use(ctx, file); return nil }})
	if err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(scope.Validate(), scope.OperationError(), scope.CleanupError()); err != nil {
		t.Fatal(err)
	}
}

func TestReadSnapshotNativeExtentCursorAndLaterWrites(t *testing.T) {
	t.Parallel()
	for _, size := range []int{0, 1, 4095, 4096, 4097, 65537} {
		t.Run(fmt.Sprintf("bytes_%d", size), func(t *testing.T) {
			t.Parallel()
			withSnapshotChunkFile(t, func(ctx context.Context, file filestore.ScratchReplayFile) {
				body := bytes.Repeat([]byte{'x'}, size)
				accepted, err := filestore.WriteChunk(ctx, filestore.ChunkWriteRequest{Destination: file, Bytes: body})
				if err != nil || accepted.Uint64() != uint64(size) {
					t.Fatalf("native write = (%d,%v)", accepted.Uint64(), err)
				}
				first, err := filestore.ObserveReadSnapshot(ctx, filestore.ReadSnapshotRequest{Source: file})
				if err != nil || first.Validate() != nil {
					t.Fatalf("first snapshot = (%+v,%v)", first, err)
				}
				second, err := filestore.ObserveReadSnapshot(ctx, filestore.ReadSnapshotRequest{Source: file})
				if err != nil {
					t.Fatal(err)
				}
				if first.Extent.Uint64() != uint64(size) || second.Extent != first.Extent {
					t.Fatalf("snapshot extent = %d/%d, want %d", first.Extent.Uint64(), second.Extent.Uint64(), size)
				}
				if pos, err := file.Seek(0, io.SeekCurrent); err != nil || pos != int64(size) {
					t.Fatalf("snapshot changed write position = (%d,%v)", pos, err)
				}
				if _, err := filestore.WriteChunk(ctx, filestore.ChunkWriteRequest{Destination: file, Bytes: []byte("tail")}); err != nil {
					t.Fatal(err)
				}
				for _, snapshot := range []filestore.ReadSnapshot{first, second} {
					if size == 0 {
						if snapshot.Reader != nil {
							t.Fatal("empty snapshot fabricated a reader")
						}
						continue
					}
					got, err := io.ReadAll(snapshot.Reader)
					if err != nil || !bytes.Equal(got, body) {
						t.Fatalf("snapshot = (%q,%v), want original body without later writes", got, err)
					}
				}
				latest, err := filestore.ObserveReadSnapshot(ctx, filestore.ReadSnapshotRequest{Source: file})
				if err != nil {
					t.Fatal(err)
				}
				got, err := io.ReadAll(latest.Reader)
				if err != nil || !bytes.Equal(got, append(bytes.Clone(body), []byte("tail")...)) {
					t.Fatalf("fresh snapshot = (%q,%v), want actual current native bytes", got, err)
				}
				if err := filestore.ResetScratch(ctx, filestore.ScratchResetRequest{File: file}); err != nil {
					t.Fatal(err)
				}
				empty, err := filestore.ObserveReadSnapshot(ctx, filestore.ReadSnapshotRequest{Source: file})
				if err != nil || empty.Reader != nil || empty.Extent.Uint64() != 0 {
					t.Fatalf("reset snapshot = (%+v,%v), want native empty extent", empty, err)
				}
			})
		})
	}
}

func TestReadSnapshotUsesHeldInodeAndNativeClosure(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	absolute, err := core.ParseAbsolutePath(directory)
	if err != nil {
		t.Fatal(err)
	}
	root, err := filestore.OpenRoot(t.Context(), absolute)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	name, err := core.ParseRelativePath("source")
	if err != nil {
		t.Fatal(err)
	}
	file, err := filestore.OpenAppend(t.Context(), filestore.AppendRequest{Location: filestore.Location{Root: root, Path: name}, Mode: 0600, Append: filestore.AppendCreate})
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	readable, err := filestore.OpenRead(t.Context(), filestore.ReadHandleRequest{Location: filestore.Location{Root: root, Path: name}})
	if err != nil {
		t.Fatal(err)
	}
	defer readable.Close()
	if err := os.WriteFile(filepath.Join(directory, "source"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(directory, "source"), filepath.Join(directory, "held")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "source"), []byte("foreign"), 0600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := filestore.ObserveReadSnapshot(t.Context(), filestore.ReadSnapshotRequest{Source: readable})
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(snapshot.Reader)
	if err != nil || string(got) != "original" {
		t.Fatalf("held snapshot = (%q,%v), want original inode", got, err)
	}
	if err := readable.Close(); err != nil {
		t.Fatal(err)
	}
	failed, err := filestore.ObserveReadSnapshot(t.Context(), filestore.ReadSnapshotRequest{Source: readable})
	var native *fs.PathError
	if failed.Reader != nil || failed.Extent.Uint64() != 0 || !errors.Is(err, fs.ErrClosed) || !errors.As(err, &native) {
		t.Fatalf("closed snapshot = (%+v,%v), want no loan and native PathError", failed, err)
	}
}

type chunkBoundaryWriter struct {
	count int
	cause error
}

func (w chunkBoundaryWriter) Write([]byte) (int, error) { return w.count, w.cause }

func TestWriteChunkPreservesNativePartialCountsAndRefusals(t *testing.T) {
	t.Parallel()
	for size := 0; size <= 5; size++ {
		for count := -1; count <= size+1; count++ {
			for _, cause := range []error{nil, fs.ErrPermission} {
				t.Run(fmt.Sprintf("bytes_%d/count_%d/cause_%t", size, count, cause != nil), func(t *testing.T) {
					t.Parallel()
					got, err := filestore.WriteChunk(t.Context(), filestore.ChunkWriteRequest{Destination: chunkBoundaryWriter{count: count, cause: cause}, Bytes: make([]byte, size)})
					valid := count >= 0 && count <= size
					wantCount := count
					if !valid {
						wantCount = 0
					}
					if got.Uint64() != uint64(wantCount) {
						t.Fatalf("accepted = %d, want native count %d", got.Uint64(), wantCount)
					}
					if cause != nil && !errors.Is(err, cause) {
						t.Fatalf("native cause disappeared: %v", err)
					}
					if !valid || count < size {
						if !errors.Is(err, io.ErrShortWrite) {
							t.Fatalf("invalid/short count = %v, want ShortWrite", err)
						}
					} else if !errors.Is(err, cause) {
						t.Fatalf("write refusal = %v, want %v", err, cause)
					}
				})
			}
		}
	}
}

func TestSnapshotAndChunkAdmissionPreserveCallerCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(fs.ErrPermission)
	var destination bytes.Buffer
	accepted, err := filestore.WriteChunk(ctx, filestore.ChunkWriteRequest{Destination: &destination, Bytes: []byte("refused")})
	if accepted.Uint64() != 0 || destination.Len() != 0 || !errors.Is(err, context.Canceled) || !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("cancelled write = (%d,%d,%v), want no effect and both causes", accepted.Uint64(), destination.Len(), err)
	}
	snapshot, err := filestore.ObserveReadSnapshot(ctx, filestore.ReadSnapshotRequest{})
	if snapshot.Reader != nil || snapshot.Extent.Uint64() != 0 || !errors.Is(err, context.Canceled) || !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("cancelled snapshot = (%+v,%v)", snapshot, err)
	}
	if _, err := filestore.WriteChunk(t.Context(), filestore.ChunkWriteRequest{}); !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("missing destination = %v", err)
	}
	if _, err := filestore.ObserveReadSnapshot(t.Context(), filestore.ReadSnapshotRequest{}); !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("missing source = %v", err)
	}
	if _, err := filestore.WriteChunk(nil, filestore.ChunkWriteRequest{Destination: &destination}); !errors.Is(err, core.ErrNilContext) {
		t.Fatalf("nil write context = %v", err)
	}
	if _, err := filestore.ObserveReadSnapshot(nil, filestore.ReadSnapshotRequest{}); !errors.Is(err, core.ErrNilContext) {
		t.Fatalf("nil read context = %v", err)
	}
}

func FuzzReadSnapshotAndChunkMatchNativeByteSequence(f *testing.F) {
	f.Add([]byte("prefix"), []byte("suffix"))
	f.Add([]byte{}, []byte{0, 255, '\n'})
	f.Fuzz(func(t *testing.T, before, after []byte) {
		withSnapshotChunkFile(t, func(ctx context.Context, file filestore.ScratchReplayFile) {
			if _, err := filestore.WriteChunk(ctx, filestore.ChunkWriteRequest{Destination: file, Bytes: before}); err != nil {
				t.Fatal(err)
			}
			snapshot, err := filestore.ObserveReadSnapshot(ctx, filestore.ReadSnapshotRequest{Source: file})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := filestore.WriteChunk(ctx, filestore.ChunkWriteRequest{Destination: file, Bytes: after}); err != nil {
				t.Fatal(err)
			}
			if len(before) == 0 {
				if snapshot.Reader != nil {
					t.Fatal("empty source fabricated reader")
				}
				return
			}
			offset := len(before) / 2
			positional := make([]byte, len(before)-offset+1)
			count, readErr := snapshot.Reader.ReadAt(positional, int64(offset))
			if count != len(before)-offset || !errors.Is(readErr, io.EOF) || !bytes.Equal(positional[:count], before[offset:]) {
				t.Fatalf("positional snapshot = %x/%d/%v, want bounded original suffix %x", positional, count, readErr, before[offset:])
			}
			got, err := io.ReadAll(snapshot.Reader)
			if err != nil || !bytes.Equal(got, before) || snapshot.Extent.Uint64() != uint64(len(before)) {
				t.Fatalf("snapshot = (%q,%v), want exact pre-write source %q", got, err, before)
			}
			if position, err := snapshot.Reader.Seek(int64(offset), io.SeekStart); err != nil || position != int64(offset) {
				t.Fatalf("snapshot seek = %d/%v", position, err)
			}
			remaining, err := io.ReadAll(snapshot.Reader)
			if err != nil || !bytes.Equal(remaining, before[offset:]) {
				t.Fatalf("seek snapshot = %x/%v, want %x", remaining, err, before[offset:])
			}
		})
	})
}
