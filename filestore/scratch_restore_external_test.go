package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"math"
	"os"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestScratchRestoreConservesNativePrefixAndWritePosition(t *testing.T) {
	t.Parallel()
	for _, size := range []uint64{0, 1, 6, 11} {
		t.Run(strconv.FormatUint(size, 10), func(t *testing.T) {
			t.Parallel()
			withSnapshotChunkFile(t, func(ctx context.Context, file filestore.ScratchReplayFile) {
				body := []byte("prefix-tail")
				if _, err := filestore.WriteChunk(ctx, filestore.ChunkWriteRequest{Destination: file, Bytes: body}); err != nil {
					t.Fatal(err)
				}
				prefix, err := core.NewByteLength(size)
				if err != nil {
					t.Fatal(err)
				}
				for range 2 {
					if err := filestore.RestoreScratchPrefix(ctx, filestore.ScratchRestoreRequest{File: file, Prefix: prefix}); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := filestore.WriteChunk(ctx, filestore.ChunkWriteRequest{Destination: file, Bytes: []byte("next")}); err != nil {
					t.Fatal(err)
				}
				if _, err := file.Seek(0, io.SeekStart); err != nil {
					t.Fatal(err)
				}
				actual, err := io.ReadAll(file)
				want := append(bytes.Clone(body[:size]), []byte("next")...)
				if err != nil || !bytes.Equal(actual, want) {
					t.Fatalf("restored bytes = %q/%v, want %q", actual, err, want)
				}
			})
		})
	}
}

func TestScratchRestoreRefusalAndPartialMutationBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		want    error
		name    string
		failure scratchRestoreFailure
		prefix  uint64
		changed bool
	}{
		{name: "native initial stat", failure: scratchRestoreStat, prefix: 2, want: os.ErrPermission},
		{name: "native truncate", failure: scratchRestoreTruncate, prefix: 2, want: io.ErrClosedPipe},
		{name: "native seek after truncate", failure: scratchRestoreSeek, prefix: 2, want: io.ErrNoProgress, changed: true},
		{name: "native final stat after truncate", failure: scratchRestoreFinalStat, prefix: 2, want: io.ErrUnexpectedEOF, changed: true},
		{name: "wrong returned seek coordinate", failure: scratchRestoreWrongOffset, prefix: 2, want: core.ErrFilestoreContract, changed: true},
		{name: "wrong final native extent", failure: scratchRestoreWrongExtent, prefix: 2, want: core.ErrFilestoreContract, changed: true},
		{name: "missing initial metadata", failure: scratchRestoreNilStat, prefix: 2, want: core.ErrFilestoreContract},
		{name: "missing final metadata", failure: scratchRestoreNilFinalStat, prefix: 2, want: core.ErrFilestoreContract, changed: true},
		{name: "nonregular native metadata", failure: scratchRestoreNonregular, prefix: 2, want: core.ErrFilestoreContract},
		{name: "negative native extent", failure: scratchRestoreNegativeExtent, prefix: 2, want: core.ErrFilestoreContract},
		{name: "prefix cannot extend file", prefix: 5, want: core.ErrFilestoreContract},
		{name: "largest signed prefix cannot extend file", prefix: math.MaxInt64, want: core.ErrFilestoreContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			withSnapshotChunkFile(t, func(ctx context.Context, file filestore.ScratchReplayFile) {
				if _, err := filestore.WriteChunk(ctx, filestore.ChunkWriteRequest{Destination: file, Bytes: []byte("keep")}); err != nil {
					t.Fatal(err)
				}
				prefix, err := core.NewByteLength(tc.prefix)
				if err != nil {
					t.Fatal(err)
				}
				borrowed := &scratchRestoreFaultFile{ScratchReplayFile: file, failure: tc.failure}
				err = filestore.RestoreScratchPrefix(ctx, filestore.ScratchRestoreRequest{File: borrowed, Prefix: prefix})
				if !errors.Is(err, tc.want) {
					t.Fatalf("restore = %v, want original %v", err, tc.want)
				}
				info, err := file.Stat()
				if err != nil {
					t.Fatal(err)
				}
				wantSize := int64(4)
				if tc.changed {
					wantSize = int64(tc.prefix)
				}
				if info.Size() != wantSize {
					t.Fatalf("native size = %d, want %d after refusal", info.Size(), wantSize)
				}
				if _, err := file.Seek(0, io.SeekStart); err != nil {
					t.Fatal(err)
				}
				actual, err := io.ReadAll(file)
				if err != nil || string(actual) != "keep"[:wantSize] {
					t.Fatalf("native refused bytes = %q/%v, want original prefix", actual, err)
				}
			})
		})
	}
}

func TestScratchRestoreAdmissionAndNativeLifetime(t *testing.T) {
	t.Parallel()
	var typedNil *scratchRestoreFaultFile
	for _, source := range []filestore.ScratchResetFile{nil, typedNil} {
		if err := filestore.RestoreScratchPrefix(t.Context(), filestore.ScratchRestoreRequest{File: source}); !errors.Is(err, core.ErrFilestoreContract) {
			t.Fatalf("absent file = %v", err)
		}
	}
	var expired filestore.ScratchReplayFile
	withSnapshotChunkFile(t, func(ctx context.Context, file filestore.ScratchReplayFile) {
		expired = file
		if _, err := filestore.WriteChunk(ctx, filestore.ChunkWriteRequest{Destination: file, Bytes: []byte("keep")}); err != nil {
			t.Fatal(err)
		}
		if err := filestore.RestoreScratchPrefix(nil, filestore.ScratchRestoreRequest{File: file}); !errors.Is(err, core.ErrNilContext) {
			t.Fatalf("nil context = %v", err)
		}
		stopped, cancel := context.WithCancelCause(ctx)
		cancel(os.ErrPermission)
		if err := filestore.RestoreScratchPrefix(stopped, filestore.ScratchRestoreRequest{File: file}); !errors.Is(err, context.Canceled) || !errors.Is(err, os.ErrPermission) {
			t.Fatalf("stopped context = %v, want both causes", err)
		}
		info, err := file.Stat()
		if err != nil || info.Size() != 4 {
			t.Fatalf("refused admission changed file = %v/%v", info, err)
		}
	})
	if err := filestore.RestoreScratchPrefix(t.Context(), filestore.ScratchRestoreRequest{File: expired}); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("expired scope = %v, want native closed handle", err)
	}
}

func TestScratchRestorePreservesMidOperationCancellationCause(t *testing.T) {
	t.Parallel()
	withSnapshotChunkFile(t, func(ctx context.Context, file filestore.ScratchReplayFile) {
		if _, err := filestore.WriteChunk(ctx, filestore.ChunkWriteRequest{Destination: file, Bytes: []byte("keep")}); err != nil {
			t.Fatal(err)
		}
		stopped, cancel := context.WithCancelCause(ctx)
		borrowed := &scratchRestoreCancelFile{ScratchReplayFile: file, cancel: cancel}
		err := filestore.RestoreScratchPrefix(stopped, filestore.ScratchRestoreRequest{File: borrowed})
		if !errors.Is(err, context.Canceled) || !errors.Is(err, io.ErrNoProgress) {
			t.Fatalf("mid-operation cancellation = %v, want native and original context cause", err)
		}
		info, err := file.Stat()
		if err != nil || info.Size() != 4 {
			t.Fatalf("failed truncate changed native extent = %v/%v", info, err)
		}
	})
}

func FuzzScratchRestoreMatchesNativePrefix(f *testing.F) {
	f.Add([]byte("prefix-tail"), uint64(6))
	f.Add([]byte{}, uint64(0))
	f.Add([]byte{0, 255, '\n'}, uint64(3))
	f.Fuzz(func(t *testing.T, body []byte, coordinate uint64) {
		withSnapshotChunkFile(t, func(ctx context.Context, file filestore.ScratchReplayFile) {
			if _, err := filestore.WriteChunk(ctx, filestore.ChunkWriteRequest{Destination: file, Bytes: body}); err != nil {
				t.Fatal(err)
			}
			size := coordinate % uint64(len(body)+1)
			prefix, err := core.NewByteLength(size)
			if err != nil {
				t.Fatal(err)
			}
			if err := filestore.RestoreScratchPrefix(ctx, filestore.ScratchRestoreRequest{File: file, Prefix: prefix}); err != nil {
				t.Fatal(err)
			}
			if _, err := filestore.WriteChunk(ctx, filestore.ChunkWriteRequest{Destination: file, Bytes: []byte("next")}); err != nil {
				t.Fatal(err)
			}
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			actual, err := io.ReadAll(file)
			want := append(bytes.Clone(body[:size]), []byte("next")...)
			if err != nil || !bytes.Equal(actual, want) {
				t.Fatalf("native restore = %x/%v, want %x", actual, err, want)
			}
		})
	})
}

type scratchRestoreFailure uint8

const (
	scratchRestoreNone scratchRestoreFailure = iota
	scratchRestoreStat
	scratchRestoreTruncate
	scratchRestoreSeek
	scratchRestoreFinalStat
	scratchRestoreWrongOffset
	scratchRestoreWrongExtent
	scratchRestoreNilStat
	scratchRestoreNilFinalStat
	scratchRestoreNonregular
	scratchRestoreNegativeExtent
)

type scratchRestoreFaultFile struct {
	filestore.ScratchReplayFile
	failure scratchRestoreFailure
	stats   int
}

func (f *scratchRestoreFaultFile) Stat() (fs.FileInfo, error) {
	f.stats++
	if f.failure == scratchRestoreStat {
		return nil, os.ErrPermission
	}
	if f.failure == scratchRestoreNilStat || f.failure == scratchRestoreNilFinalStat && f.stats > 1 {
		return nil, nil
	}
	if f.failure == scratchRestoreFinalStat && f.stats > 1 {
		return nil, io.ErrUnexpectedEOF
	}
	info, err := f.ScratchReplayFile.Stat()
	if err != nil {
		return nil, err
	}
	if f.failure == scratchRestoreNonregular || f.failure == scratchRestoreNegativeExtent || f.failure == scratchRestoreWrongExtent && f.stats > 1 {
		return scratchRestoreFaultInfo{FileInfo: info, failure: f.failure}, nil
	}
	return info, nil
}
func (f *scratchRestoreFaultFile) Truncate(size int64) error {
	if f.failure == scratchRestoreTruncate {
		return io.ErrClosedPipe
	}
	return f.ScratchReplayFile.Truncate(size)
}
func (f *scratchRestoreFaultFile) Seek(offset int64, whence int) (int64, error) {
	if f.failure == scratchRestoreSeek {
		return 0, io.ErrNoProgress
	}
	position, err := f.ScratchReplayFile.Seek(offset, whence)
	if err == nil && f.failure == scratchRestoreWrongOffset {
		return position + 1, nil
	}
	return position, err
}

type scratchRestoreFaultInfo struct {
	fs.FileInfo
	failure scratchRestoreFailure
}

func (i scratchRestoreFaultInfo) Mode() fs.FileMode {
	if i.failure == scratchRestoreNonregular {
		return fs.ModeDir | 0700
	}
	return i.FileInfo.Mode()
}
func (i scratchRestoreFaultInfo) Size() int64 {
	if i.failure == scratchRestoreNegativeExtent {
		return -1
	}
	if i.failure == scratchRestoreWrongExtent {
		return i.FileInfo.Size() + 1
	}
	return i.FileInfo.Size()
}

type scratchRestoreCancelFile struct {
	filestore.ScratchReplayFile
	cancel context.CancelCauseFunc
}

func (f *scratchRestoreCancelFile) Truncate(int64) error {
	f.cancel(io.ErrNoProgress)
	return context.Canceled
}
