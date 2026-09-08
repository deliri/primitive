package filestore_test

import (
	"context"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/filestore"
)

// The caller joins the cancellation callback before closing its own handles.
// This is a deadlock backstop, never evidence of elapsed-time performance.
func newPipeBackstop(t testing.TB, pipe filestore.Pipe) func() error {
	t.Helper()
	ctx, cancel := newFilesystemBackstop(t.Context(), t, time.Minute)
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		if pipe.Reader != nil {
			_ = pipe.Reader.Close()
		}
		if pipe.Writer != nil && pipe.Writer != pipe.Reader {
			_ = pipe.Writer.Close()
		}
		close(done)
	})
	return func() error {
		stopped := stop()
		if !stopped {
			<-done
		}
		err := ctx.Err()
		cancel()
		return err
	}
}
