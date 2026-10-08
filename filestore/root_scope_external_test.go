package filestore_test

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestRootScopeClosesBorrowedNativeRootOnReturnRefusalAndPanic(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name           string
		failure        error
		panicCallback  bool
		cancelCallback bool
	}{
		{name: "successful callback closes its borrowed root"},
		{name: "refusal preserves identity and closes its borrowed root", failure: io.ErrUnexpectedEOF},
		{name: "panic becomes a typed refusal after native cleanup", failure: core.ErrFilestoreContract, panicCallback: true},
		{name: "cancellation during callback preserves identity and closes root", failure: context.Canceled, cancelCallback: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			parent, err := hostfacts.TemporaryDirectory()
			if err != nil {
				t.Fatal(err)
			}
			err = filestore.WithScratchScope(t.Context(), filestore.ScratchScopeRequest{Parent: parent, Use: func(ctx context.Context, fixture *os.Root) error {
				dir, err := core.ParseAbsolutePath(fixture.Name())
				if err != nil {
					return err
				}
				var borrowed *os.Root
				calls := 0
				lifetime, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: ctx})
				if err != nil {
					return err
				}
				defer cancel(nil)
				result, err := filestore.WithRootScope(lifetime, filestore.RootScopeRequest{Directory: dir, Use: func(_ context.Context, root *os.Root) error {
					borrowed = root
					calls++
					if tc.panicCallback {
						panic(io.ErrShortWrite)
					}
					if tc.cancelCallback {
						cancel(nil)
					}
					return tc.failure
				}})
				if err != nil || result.Validate() != nil || result.CleanupError != nil || !errors.Is(result.OperationError, tc.failure) || calls != 1 {
					t.Errorf("root scope = (%+v, %v), calls=%d; want one callback, typed operation outcome and completed native cleanup", result, err, calls)
				}
				if borrowed == nil {
					t.Error("root scope did not lend a real root")
					return nil
				}
				file, openErr := borrowed.Open(".")
				if file != nil {
					return errors.Join(errors.New("borrowed root remained open after scope"), file.Close())
				}
				if !errors.Is(openErr, os.ErrClosed) {
					t.Errorf("borrowed root after return = %v, want closed identity", openErr)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("native fixture lifetime = %v, want nil", err)
			}
		})
	}
}

func TestRootScopeRejectsInvalidIngressWithoutCallingPolicy(t *testing.T) {
	t.Parallel()
	parent, err := hostfacts.TemporaryDirectory()
	if err != nil {
		t.Fatal(err)
	}
	canceled, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: t.Context()})
	if err != nil {
		t.Fatal(err)
	}
	cancel(nil)
	for _, tc := range []struct {
		name        string
		ctx         context.Context
		directory   core.AbsolutePath
		nilCallback bool
		want        error
	}{
		{name: "missing context", directory: parent, want: core.ErrNilContext},
		{name: "missing directory", ctx: t.Context(), want: core.ErrFilestoreContract},
		{name: "missing callback", ctx: t.Context(), directory: parent, nilCallback: true, want: core.ErrFilestoreContract},
		{name: "canceled ingress", ctx: canceled, directory: parent, want: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			request := filestore.RootScopeRequest{Directory: tc.directory, Use: func(context.Context, *os.Root) error { calls++; return nil }}
			if tc.nilCallback {
				request.Use = nil
			}
			result, err := filestore.WithRootScope(tc.ctx, request)
			if !errors.Is(err, tc.want) || !errors.Is(result.Validate(), core.ErrFilestoreContract) || calls != 0 {
				t.Fatalf("root scope admission = (%+v, %v), calls=%d; want invalid result, refusal %v and no callback", result, err, calls, tc.want)
			}
		})
	}
}
