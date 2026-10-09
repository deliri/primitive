package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func FuzzFileScopesConserveNativeBytesAndCloseBorrowedHandles(f *testing.F) {
	f.Add([]byte{}, false, false)
	f.Add([]byte{0, 255, 31}, false, false)
	f.Add([]byte{31, 14, 0, 255}, true, false)
	f.Add([]byte("reader refusal"), false, true)
	f.Fuzz(func(t *testing.T, data []byte, writerPanic, readerRefusal bool) {
		// Known fixture bound, not a production stream-extent limit.
		if len(data) > 4096 {
			return
		}
		directory, err := core.ParseAbsolutePath(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		path, err := core.ParseRelativePath("subject")
		if err != nil {
			t.Fatal(err)
		}
		result, err := filestore.WithRootScope(t.Context(), filestore.RootScopeRequest{Directory: directory, Use: func(ctx context.Context, root *os.Root) error {
			location := filestore.Location{Root: root, Path: path}
			var borrowedWriter io.Writer
			written, err := filestore.WithScratchWriterScope(ctx, filestore.ScratchWriterScopeRequest{Scratch: filestore.ScratchRequest{Location: location, Mode: 0o600}, Use: func(_ context.Context, writer io.Writer) error {
				borrowedWriter = writer
				n, err := writer.Write(data)
				if err != nil {
					return err
				}
				if n != len(data) {
					t.Fatalf("native accepted bytes=%d,want %d", n, len(data))
				}
				if writerPanic {
					panic(io.ErrShortWrite)
				}
				return nil
			}})
			var wantWrite error
			if writerPanic {
				wantWrite = core.ErrFilestoreCallbackPanic
			}
			if err != nil || written.Validate() != nil || written.CleanupError() != nil || !errors.Is(written.OperationError(), wantWrite) || borrowedWriter == nil {
				t.Fatalf("writer observation=(%+v,%v),want closed native custody and %v", written, err, wantWrite)
			}
			if _, err := borrowedWriter.Write([]byte{14}); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("settled writer=%v,want closed", err)
			}
			var borrowedReader io.ReadSeeker
			read, err := filestore.WithReadScope(ctx, filestore.ReadScopeRequest{Location: location, Use: func(_ context.Context, reader io.ReadSeeker) error {
				borrowedReader = reader
				body, err := io.ReadAll(reader)
				if err != nil {
					return err
				}
				if !bytes.Equal(body, data) {
					t.Fatalf("native body=%v,want %v", body, data)
				}
				if _, err := reader.Seek(0, io.SeekStart); err != nil {
					return err
				}
				if readerRefusal {
					return io.ErrUnexpectedEOF
				}
				return nil
			}})
			var wantRead error
			if readerRefusal {
				wantRead = io.ErrUnexpectedEOF
			}
			if err != nil || read.Validate() != nil || read.CleanupError() != nil || !errors.Is(read.OperationError(), wantRead) || borrowedReader == nil {
				t.Fatalf("reader observation=(%+v,%v),want closed native custody and %v", read, err, wantRead)
			}
			var buffer [1]byte
			if _, err := borrowedReader.Read(buffer[:]); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("settled reader=%v,want closed", err)
			}
			return nil
		}})
		if err != nil {
			t.Fatal(err)
		}
		if err := errors.Join(result.Validate(), result.OperationError, result.CleanupError); err != nil {
			t.Fatal(err)
		}
	})
}
