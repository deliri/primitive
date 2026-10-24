package filestore

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestAppendSpoolSealRefusesNativeDescriptorExtentDrift(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name  string
		delta int64
	}{
		{name: "same descriptor was truncated below accepted bytes", delta: -1},
		{name: "same descriptor was extended beyond accepted bytes", delta: 1},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			location := spoolLocation(t, t.TempDir())
			data := []byte("accepted\x00prefix\xff")
			var original fs.FileInfo
			var writer *os.File
			result, err := WithAppendSpoolScope(t.Context(), AppendSpoolScopeRequest{Location: location, Mode: 0600, Use: func(ctx context.Context, spool *AppendSpool) error {
				writer = spool.file
				if written, err := spool.Write(data); err != nil || written != len(data) {
					t.Fatalf("accepted write=(%d,%v), want (%d,nil)", written, err, len(data))
				}
				var err error
				original, err = writer.Stat()
				if err != nil || original.Size() != int64(len(data)) {
					t.Fatalf("accepted native extent=(%v,%v), want %d bytes", original, err, len(data))
				}
				wantSize := original.Size() + row.delta
				if err := writer.Truncate(wantSize); err != nil {
					return err
				}
				changed, err := writer.Stat()
				if err != nil || !os.SameFile(original, changed) || changed.Size() != wantSize || changed.Size() == original.Size() {
					t.Fatalf("native drift=(%v,%v), want same descriptor identity with changed extent %d", changed, err, wantSize)
				}
				content, sealErr := spool.Seal(ctx, nil)
				if content != (ContentIndexEntry{}) || !errors.Is(sealErr, core.ErrFilestoreSize) || !errors.Is(sealErr, io.ErrUnexpectedEOF) {
					t.Fatalf("drifted seal=(%+v,%v), want zero content and typed size/EOF refusal", content, sealErr)
				}
				return nil
			}})
			content, contentErr := result.Content()
			if err != nil || result.Validate() != nil || result.CleanupError() != nil || content != (ContentIndexEntry{}) || !errors.Is(result.OperationError(), core.ErrFilestoreSize) || !errors.Is(contentErr, core.ErrFilestoreSize) || !errors.Is(contentErr, io.ErrUnexpectedEOF) {
				t.Fatalf("drifted scope=(%+v,%v,%v), want closed lifetime, zero content and preserved size/EOF refusal", content, contentErr, err)
			}
			if _, err := writer.Stat(); !errors.Is(err, fs.ErrClosed) {
				t.Fatalf("original descriptor after scope=%v, want closed", err)
			}
			retained, err := location.Root.Stat("spool")
			if err != nil || !os.SameFile(original, retained) || retained.Size() != int64(len(data))+row.delta {
				t.Fatalf("retained drifted file=(%v,%v), want original inode and observed changed extent", retained, err)
			}
		})
	}
}

func TestAppendSpoolScopeExitsRetainAcceptedBytes(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name      string
		finish    func(context.Context, *AppendSpool, context.CancelCauseFunc) error
		wantErr   error
		wantCause error
	}{
		{name: "successful production retains the owned file", finish: func(ctx context.Context, spool *AppendSpool, _ context.CancelCauseFunc) error {
			_, err := spool.Seal(ctx, nil)
			return err
		}},
		{name: "producer error retains every accepted byte", wantErr: core.ErrFilestoreSource, wantCause: io.ErrNoProgress, finish: func(ctx context.Context, spool *AppendSpool, _ context.CancelCauseFunc) error {
			_, err := spool.Seal(ctx, io.ErrNoProgress)
			return err
		}},
		{name: "unsealed return retains every accepted byte", wantErr: core.ErrFilestoreSource, finish: func(context.Context, *AppendSpool, context.CancelCauseFunc) error {
			return nil
		}},
		{name: "post write cancellation retains every accepted byte", wantErr: context.Canceled, wantCause: fs.ErrPermission, finish: func(_ context.Context, _ *AppendSpool, cancel context.CancelCauseFunc) error {
			cancel(fs.ErrPermission)
			return nil
		}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			location := spoolLocation(t, t.TempDir())
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			data := []byte("accepted\x00prefix\xffretained")
			var original fs.FileInfo
			var writer *os.File
			result, err := WithAppendSpoolScope(ctx, AppendSpoolScopeRequest{Location: location, Mode: 0600, Use: func(scope context.Context, spool *AppendSpool) error {
				writer = spool.file
				for _, part := range [][]byte{data[:8], data[8:]} {
					if written, err := spool.Write(part); err != nil || written != len(part) {
						t.Fatalf("accepted write=(%d,%v), want (%d,nil)", written, err, len(part))
					}
				}
				var err error
				original, err = writer.Stat()
				if err != nil || original.Size() != int64(len(data)) {
					t.Fatalf("accepted native extent=(%v,%v), want %d bytes", original, err, len(data))
				}
				return row.finish(scope, spool, cancel)
			}})
			content, contentErr := result.Content()
			if err != nil || result.Validate() != nil || result.CleanupError() != nil {
				t.Fatalf("scope lifetime=(%v,%v,%v), want completed native close without cleanup failure", err, result.Validate(), result.CleanupError())
			}
			if row.wantErr == nil {
				if result.OperationError() != nil || contentErr != nil || content.Digest != core.SHA256Of(data) || content.Extent.Uint64() != uint64(len(data)) {
					t.Fatalf("successful scope content=(%+v,%v), operation=%v, want exact accepted bytes", content, contentErr, result.OperationError())
				}
			} else if content != (ContentIndexEntry{}) || !errors.Is(result.OperationError(), row.wantErr) || !errors.Is(contentErr, row.wantErr) || row.wantCause != nil && (!errors.Is(result.OperationError(), row.wantCause) || !errors.Is(contentErr, row.wantCause)) {
				t.Fatalf("failed scope content=(%+v,%v), operation=%v, want zero with %v and cause %v", content, contentErr, result.OperationError(), row.wantErr, row.wantCause)
			}
			if _, err := writer.Stat(); !errors.Is(err, fs.ErrClosed) {
				t.Fatalf("original descriptor after scope=%v, want closed", err)
			}
			retained, err := location.Root.Stat("spool")
			if err != nil || !os.SameFile(original, retained) || retained.Size() != int64(len(data)) {
				t.Fatalf("retained file=(%v,%v), want original inode and %d accepted bytes", retained, err, len(data))
			}
			got, err := location.Root.ReadFile("spool")
			if err != nil || !bytes.Equal(got, data) {
				t.Fatalf("retained bytes=(%x,%v), want exact accepted %x", got, err, data)
			}
		})
	}
}
