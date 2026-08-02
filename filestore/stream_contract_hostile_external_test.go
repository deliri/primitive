package filestore_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

const consecutiveEmptyReadLimit = 100

type emptyReadSequence struct {
	emptyReads int
	reads      int
}

func (r *emptyReadSequence) Read([]byte) (int, error) {
	if r.reads == r.emptyReads {
		return 0, io.EOF
	}
	r.reads++
	return 0, nil
}

type impossibleReadCount uint8

const (
	impossibleReadNegative impossibleReadCount = iota + 1
	impossibleReadExcessive
)

func (r impossibleReadCount) Read(buffer []byte) (int, error) {
	if r == impossibleReadNegative {
		return -1, nil
	}
	return len(buffer) + 1, nil
}

type hostileWriteBehavior uint8

const (
	hostileWriteNegative hostileWriteBehavior = iota + 1
	hostileWriteExcessive
	hostileWriteZeroProgress
	hostileWritePartialError
)

func (w hostileWriteBehavior) Write(buffer []byte) (int, error) {
	switch w {
	case hostileWriteNegative:
		return -1, nil
	case hostileWriteExcessive:
		return len(buffer) + 1, nil
	case hostileWriteZeroProgress:
		return 0, nil
	case hostileWritePartialError:
		return min(3, len(buffer)), io.ErrClosedPipe
	default:
		return 0, io.ErrUnexpectedEOF
	}
}

type enospcPrefixWriter struct {
	destination io.Writer
	prefixBytes int
}

func (w enospcPrefixWriter) Write(data []byte) (int, error) {
	count, err := w.destination.Write(data[:min(w.prefixBytes, len(data))])
	if err != nil {
		return count, err
	}
	return count, syscall.ENOSPC
}

func TestStageNoProgressThresholdBoundaryMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr    error
		name       string
		emptyReads int
	}{
		{
			name:       "two below consecutive empty-read limit reaches eof",
			emptyReads: consecutiveEmptyReadLimit - 2,
		},
		{
			name:       "one below consecutive empty-read limit reaches eof",
			emptyReads: consecutiveEmptyReadLimit - 1,
		},
		{
			name:       "exact consecutive empty-read limit rejects no progress",
			emptyReads: consecutiveEmptyReadLimit,
			wantErr:    io.ErrNoProgress,
		},
		{
			name:       "one above consecutive empty-read limit rejects at the exact limit",
			emptyReads: consecutiveEmptyReadLimit + 1,
			wantErr:    io.ErrNoProgress,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rootDirectory := t.TempDir()
			root := requireTestRoot(t, rootDirectory)
			staged, gotErr := filestore.Stage(t.Context(), filestore.StageRequest{
				Source:       &emptyReadSequence{emptyReads: tc.emptyReads},
				Temporary:    filestore.Location{Root: root, Path: mustRelativePath(t, ".stage")},
				Mode:         0o600,
				MaximumBytes: mustByteCount(t, 1),
			})
			if tc.wantErr != nil {
				if !errors.Is(gotErr, core.ErrFilestoreSource) ||
					!errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("Stage() error = %v, want %v and %v", gotErr, core.ErrFilestoreSource, tc.wantErr)
				}
				requireDirectoryEntryNames(t, rootDirectory, nil)
				return
			}
			if gotErr != nil {
				t.Fatalf("Stage() error = %v, want nil", gotErr)
			}
			if staged.BytesWritten().Uint64() != 0 {
				t.Fatalf("StagedFile.BytesWritten() = %d, want 0", staged.BytesWritten().Uint64())
			}
			requireDirectoryEntryNames(t, rootDirectory, []string{".stage"})
		})
	}
}

func TestStageRejectsImpossibleReaderCountsWithoutResidue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		source io.Reader
		name   string
	}{
		{
			name:   "negative reader count violates io reader contract",
			source: impossibleReadNegative,
		},
		{
			name:   "reader count above supplied buffer violates io reader contract",
			source: impossibleReadExcessive,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rootDirectory := t.TempDir()
			root := requireTestRoot(t, rootDirectory)
			_, gotErr := filestore.Stage(t.Context(), filestore.StageRequest{
				Source:       tc.source,
				Temporary:    filestore.Location{Root: root, Path: mustRelativePath(t, ".stage")},
				Mode:         0o600,
				MaximumBytes: mustByteCount(t, 1),
			})
			if !errors.Is(gotErr, core.ErrFilestoreSource) {
				t.Fatalf("Stage() error = %v, want %v", gotErr, core.ErrFilestoreSource)
			}
			requireDirectoryEntryNames(t, rootDirectory, nil)
		})
	}
}

func TestReadRejectsImpossibleWriterCountsWithExactAccounting(t *testing.T) {
	t.Parallel()

	cases := []struct {
		writer     io.Writer
		wantNative error
		name       string
		wantCount  uint64
	}{
		{
			name:       "negative writer count is a short write",
			writer:     hostileWriteNegative,
			wantNative: io.ErrShortWrite,
		},
		{
			name:       "writer count above supplied buffer is a short write",
			writer:     hostileWriteExcessive,
			wantNative: io.ErrShortWrite,
		},
		{
			name:       "zero writer progress is a short write",
			writer:     hostileWriteZeroProgress,
			wantNative: io.ErrShortWrite,
		},
		{
			name:       "partial writer error preserves written-byte accounting",
			writer:     hostileWritePartialError,
			wantNative: io.ErrClosedPipe,
			wantCount:  3,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rootDirectory := t.TempDir()
			payload := []byte("destination-pressure")
			if err := os.WriteFile(filepath.Join(rootDirectory, "source"), payload, 0o600); err != nil {
				t.Fatal(err)
			}
			root := requireTestRoot(t, rootDirectory)
			gotCount, gotErr := filestore.Read(t.Context(), filestore.ReadRequest{
				Destination:  tc.writer,
				Location:     filestore.Location{Root: root, Path: mustRelativePath(t, "source")},
				MaximumBytes: mustByteCount(t, uint64(len(payload))),
			})
			if !errors.Is(gotErr, core.ErrFilestoreDestination) ||
				!errors.Is(gotErr, tc.wantNative) {
				t.Fatalf("Read() error = %v, want %v and %v", gotErr, core.ErrFilestoreDestination, tc.wantNative)
			}
			if gotCount.Uint64() != tc.wantCount {
				t.Fatalf("Read() count = %d, want %d", gotCount.Uint64(), tc.wantCount)
			}
		})
	}
}

func TestReadPreservesExactDestinationPrefixAndENOSPCIdentity(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	payload := []byte("destination-pressure")
	if err := os.WriteFile(filepath.Join(rootDirectory, "source"), payload, 0o600); err != nil {
		t.Fatal(err)
	}
	root := requireTestRoot(t, rootDirectory)
	var destination bytes.Buffer
	const acceptedPrefixBytes = 7
	gotCount, gotErr := filestore.Read(t.Context(), filestore.ReadRequest{
		Destination: enospcPrefixWriter{
			destination: &destination,
			prefixBytes: acceptedPrefixBytes,
		},
		Location:     filestore.Location{Root: root, Path: mustRelativePath(t, "source")},
		MaximumBytes: mustByteCount(t, uint64(len(payload))),
	})
	if !errors.Is(gotErr, core.ErrFilestoreDestination) ||
		!errors.Is(gotErr, syscall.ENOSPC) {
		t.Fatalf(
			"Read(partial ENOSPC destination) error = %v, want %v and %v",
			gotErr,
			core.ErrFilestoreDestination,
			syscall.ENOSPC,
		)
	}
	if errors.Is(gotErr, io.ErrShortWrite) {
		t.Fatalf(
			"Read(partial ENOSPC destination) error = %v, want no synthetic %v overlay",
			gotErr,
			io.ErrShortWrite,
		)
	}
	if gotCount.Uint64() != acceptedPrefixBytes {
		t.Fatalf("Read(partial ENOSPC destination) count = %d, want %d", gotCount.Uint64(), acceptedPrefixBytes)
	}
	if !bytes.Equal(destination.Bytes(), payload[:acceptedPrefixBytes]) {
		t.Fatalf(
			"Read(partial ENOSPC destination) bytes = %q, want exact source prefix %q",
			destination.Bytes(),
			payload[:acceptedPrefixBytes],
		)
	}
}
