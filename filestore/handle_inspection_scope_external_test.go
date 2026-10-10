package filestore_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestInspectBorrowedScratchPreservesNativeExtentAndCursor(t *testing.T) {
	t.Parallel()
	// Cross byte, page, replay-buffer and former transcript-quota boundaries.
	for _, extent := range []int64{0, 1, 4095, 4096, 4097, 6<<20 - 1, 6 << 20, 6<<20 + 1, 1 << 40} {
		for _, cursor := range []int64{0, 1, 4095, 4096, 4097, 1 << 41} {
			t.Run(fmt.Sprintf("extent_%d_cursor_%d", extent, cursor), func(t *testing.T) {
				t.Parallel()
				directory, err := core.ParseAbsolutePath(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				path, err := core.ParseRelativePath("held")
				if err != nil {
					t.Fatal(err)
				}
				rootResult, err := filestore.WithRootScope(t.Context(), filestore.RootScopeRequest{Directory: directory, Use: func(ctx context.Context, root *os.Root) error {
					var borrowed filestore.ScratchReplayFile
					result, err := filestore.WithScratchReplayScope(ctx, filestore.ScratchReplayScopeRequest{Scratch: filestore.ScratchRequest{Location: filestore.Location{Root: root, Path: path}, Mode: 0o600}, Use: func(ctx context.Context, file filestore.ScratchReplayFile) error {
						borrowed = file
						if err := file.Truncate(extent); err != nil {
							return err
						}
						if _, err := file.Seek(cursor, io.SeekStart); err != nil {
							return err
						}
						got, err := filestore.InspectOpenFile(ctx, filestore.HandleInspectionRequest{File: file})
						if err != nil {
							return err
						}
						size, err := got.SizeBytes()
						if err != nil || size.Uint64() != uint64(extent) || got.Validate() != nil {
							t.Fatalf("held extent = %v/%v, want %d", size, err, extent)
						}
						position, err := file.Seek(0, io.SeekCurrent)
						if err != nil || position != cursor {
							t.Fatalf("held cursor = %d/%v, want %d", position, err, cursor)
						}
						_, err = file.Write([]byte{0x7f})
						return err
					}})
					if err := errors.Join(err, result.Validate(), result.OperationError(), result.CleanupError()); err != nil {
						return err
					}
					got, err := filestore.InspectOpenFile(ctx, filestore.HandleInspectionRequest{File: borrowed})
					if !errors.Is(err, fs.ErrClosed) || !errors.Is(err, core.ErrFilestoreSource) || got.Validate() == nil {
						t.Fatalf("after-scope observation = %+v/%v, want native closed refusal", got, err)
					}
					return nil
				}})
				if err := errors.Join(err, rootResult.Validate(), rootResult.OperationError, rootResult.CleanupError); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

type cancelingInspectionFile struct {
	filestore.ScratchReplayFile
	cancel  context.CancelFunc
	failure error
}

func (f cancelingInspectionFile) Stat() (fs.FileInfo, error) {
	info, err := f.ScratchReplayFile.Stat()
	f.cancel()
	return info, errors.Join(err, f.failure)
}

func TestInspectBorrowedScratchPreservesCancellationAndNativeCause(t *testing.T) {
	t.Parallel()
	for _, failure := range []error{nil, io.ErrUnexpectedEOF} {
		t.Run(fmt.Sprintf("native_cause_%v", failure), func(t *testing.T) {
			t.Parallel()
			file, err := os.CreateTemp(t.TempDir(), "held")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := file.Close(); err != nil {
					t.Error(err)
				}
			})
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			got, err := filestore.InspectOpenFile(ctx, filestore.HandleInspectionRequest{File: cancelingInspectionFile{ScratchReplayFile: file, cancel: cancel, failure: failure}})
			if !errors.Is(err, context.Canceled) || got.Validate() == nil {
				t.Fatalf("interrupted observation = %+v/%v, want cancellation and no facts", got, err)
			}
			if failure != nil && (!errors.Is(err, failure) || !errors.Is(err, core.ErrFilestoreSource)) {
				t.Fatalf("interrupted native cause = %v, want %v and source identity", err, failure)
			}
		})
	}
}
