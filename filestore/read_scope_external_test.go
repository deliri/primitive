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
	"github.com/deliri/primitive/v2026/temporal"
)

func TestReadScopeClosesBorrowedReaderAfterSuccessRefusalAndPanic(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name           string
		failure        error
		panicCallback  bool
		cancelCallback bool
	}{
		{name: "successful read closes native handle"},
		{name: "callback refusal preserves identity and closes native handle", failure: io.ErrUnexpectedEOF},
		{name: "callback panic closes native handle and returns typed refusal", failure: core.ErrFilestoreContract, panicCallback: true},
		{name: "callback cancellation preserves identity and closes native handle", failure: context.Canceled, cancelCallback: true},
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
			temporary, err := core.ParseRelativePath("temporary")
			if err != nil {
				t.Fatal(err)
			}
			result, err := filestore.WithRootScope(t.Context(), filestore.RootScopeRequest{Directory: directory, Use: func(ctx context.Context, root *os.Root) error {
				location := filestore.Location{Root: root, Path: path}
				if _, err := filestore.Write(ctx, filestore.WriteRequest{Location: location, Temporary: temporary, Source: bytes.NewReader([]byte{0, 255, 31}), Mode: 0o600, Install: filestore.InstallCreate}); err != nil {
					return err
				}
				var borrowed io.ReadSeeker
				calls := 0
				lifetime, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: ctx})
				if err != nil {
					return err
				}
				defer cancel(nil)
				scope, err := filestore.WithReadScope(lifetime, filestore.ReadScopeRequest{Location: location, Use: func(ctx context.Context, reader io.ReadSeeker) error {
					borrowed = reader
					calls++
					var body [3]byte
					if _, err := io.ReadFull(reader, body[:]); err != nil {
						return err
					}
					if body != [3]byte{0, 255, 31} {
						t.Fatalf("borrowed source = %v, want exact native bytes", body)
					}
					if tc.panicCallback {
						panic(io.ErrShortWrite)
					}
					if tc.cancelCallback {
						cancel(context.Canceled)
						return ctx.Err()
					}
					return tc.failure
				}})
				if err != nil || scope.Validate() != nil || scope.CleanupError() != nil || !errors.Is(scope.OperationError(), tc.failure) || calls != 1 || borrowed == nil {
					t.Fatalf("read scope = (%+v,%v,%d calls), want completed native cleanup and %v", scope, err, calls, tc.failure)
				}
				var buffer [1]byte
				if _, err := borrowed.Read(buffer[:]); !errors.Is(err, os.ErrClosed) {
					t.Fatalf("reader after scope = %v, want closed", err)
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
}
