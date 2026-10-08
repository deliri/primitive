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

func FuzzRootScopeNativeLifetime(f *testing.F) {
	for mode := byte(0); mode < 4; mode++ {
		f.Add(mode)
	}
	f.Fuzz(func(t *testing.T, mode byte) {
		parent, err := hostfacts.TemporaryDirectory()
		if err != nil {
			t.Fatal(err)
		}
		err = filestore.WithScratchScope(t.Context(), filestore.ScratchScopeRequest{Parent: parent, Use: func(ctx context.Context, fixture *os.Root) error {
			dir, err := core.ParseAbsolutePath(fixture.Name())
			if err != nil {
				return err
			}
			lifetime, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: ctx})
			if err != nil {
				return err
			}
			defer cancel(nil)
			var borrowed *os.Root
			var want error
			switch mode % 4 {
			case 1:
				want = io.ErrUnexpectedEOF
			case 2:
				want = core.ErrFilestoreContract
			case 3:
				want = context.Canceled
			}
			result, err := filestore.WithRootScope(lifetime, filestore.RootScopeRequest{Directory: dir, Use: func(_ context.Context, root *os.Root) error {
				borrowed = root
				if mode%4 == 2 {
					panic(io.ErrShortWrite)
				}
				if mode%4 == 3 {
					cancel(nil)
				}
				return want
			}})
			if err != nil || result.Validate() != nil || result.CleanupError != nil || !errors.Is(result.OperationError, want) || borrowed == nil {
				t.Errorf("root lifetime = (%+v, %v), borrowed=%v; want completed cleanup and cause %v", result, err, borrowed != nil, want)
				return nil
			}
			file, err := borrowed.Open(".")
			if file != nil {
				return errors.Join(errors.New("root lifetime left borrowed root open"), file.Close())
			}
			if !errors.Is(err, os.ErrClosed) {
				t.Errorf("borrowed root = %v, want closed identity", err)
			}
			return nil
		}})
		if err != nil {
			t.Fatalf("native fixture cleanup = %v, want nil", err)
		}
	})
}
