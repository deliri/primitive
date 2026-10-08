package filestore_test

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestScratchWriterScopeClosesAfterSuccessRefusalPanicAndPrematureClose(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name           string
		failure        error
		panicCallback  bool
		closeCallback  bool
		cancelCallback bool
	}{
		{name: "successful producer closes exact native scratch"},
		{name: "producer refusal preserves accepted bytes and identity", failure: io.ErrShortWrite},
		{name: "producer panic closes native scratch and refuses", failure: core.ErrFilestoreContract, panicCallback: true},
		{name: "producer closes borrowed handle and native cleanup remains failed", closeCallback: true},
		{name: "producer cancellation preserves identity and closes native scratch", failure: context.Canceled, cancelCallback: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
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
				var borrowed io.Writer
				calls := 0
				lifetime, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: ctx})
				if err != nil {
					return err
				}
				defer cancel(nil)
				scope, err := filestore.WithScratchWriterScope(lifetime, filestore.ScratchWriterScopeRequest{Scratch: filestore.ScratchRequest{Location: location, Mode: 0o600}, Use: func(ctx context.Context, writer io.Writer) error {
					borrowed = writer
					calls++
					if _, err := writer.Write([]byte{0, 255, 31}); err != nil {
						return err
					}
					if tc.closeCallback {
						return writer.(io.Closer).Close()
					}
					if tc.panicCallback {
						panic(io.ErrUnexpectedEOF)
					}
					if tc.cancelCallback {
						cancel(context.Canceled)
						return ctx.Err()
					}
					return tc.failure
				}})
				if err != nil || scope.Validate() != nil || !errors.Is(scope.OperationError(), tc.failure) || calls != 1 || borrowed == nil {
					t.Fatalf("writer scope = (%+v,%v,%d calls), want completed close attempt and %v", scope, err, calls, tc.failure)
				}
				if tc.closeCallback {
					if !errors.Is(scope.CleanupError(), os.ErrClosed) || !errors.Is(scope.CleanupError(), core.ErrFilestoreCleanup) {
						t.Fatalf("native cleanup failure = %v, want closed and cleanup identities", scope.CleanupError())
					}
				} else if scope.CleanupError() != nil {
					return scope.CleanupError()
				}
				if _, err := borrowed.Write([]byte{14}); !errors.Is(err, os.ErrClosed) {
					t.Fatalf("writer after scope = %v, want closed", err)
				}
				read, err := filestore.WithReadScope(ctx, filestore.ReadScopeRequest{Location: location, Use: func(_ context.Context, reader io.ReadSeeker) error {
					var body [3]byte
					if _, err := io.ReadFull(reader, body[:]); err != nil {
						return err
					}
					if body != [3]byte{0, 255, 31} {
						t.Fatalf("native scratch body = %v, want conserved accepted bytes", body)
					}
					return nil
				}})
				return errors.Join(err, read.Validate(), read.OperationError(), read.CleanupError())
			}})
			if err != nil {
				t.Fatal(err)
			}
			if err := errors.Join(result.Validate(), result.OperationError, result.CleanupError); err != nil {
				t.Fatal(err)
			}
		})
	}
}
