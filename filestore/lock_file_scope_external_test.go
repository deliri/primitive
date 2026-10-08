package filestore_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestLockFileScopeClosesEveryConsumerOutcome(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		failure       error
		panicConsumer bool
	}{
		{name: "successful consumer"},
		{name: "refused consumer", failure: context.DeadlineExceeded},
		{name: "panicking consumer", failure: core.ErrFilestoreContract, panicConsumer: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory, err := core.ParseAbsolutePath(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			path, err := core.ParseRelativePath("carrier")
			if err != nil {
				t.Fatal(err)
			}
			rootScope, err := filestore.WithRootScope(t.Context(), filestore.RootScopeRequest{Directory: directory, Use: func(ctx context.Context, root *os.Root) error {
				var borrowed *os.File
				calls := 0
				scope, err := filestore.WithLockFileScope(ctx, filestore.LockFileScopeRequest{Carrier: filestore.LockFileRequest{Location: filestore.Location{Root: root, Path: path}, Mode: 0o600}, Use: func(_ context.Context, file *os.File) error {
					borrowed = file
					calls++
					if tc.panicConsumer {
						panic(context.DeadlineExceeded)
					}
					return tc.failure
				}})
				if err != nil || scope.Validate() != nil || !errors.Is(scope.OperationError(), tc.failure) || scope.CleanupError() != nil || calls != 1 || borrowed == nil {
					t.Fatalf("carrier scope = %+v / %v / %d calls, want completed closure with %v", scope, err, calls, tc.failure)
				}
				if _, err := borrowed.Stat(); !errors.Is(err, os.ErrClosed) {
					t.Fatalf("native carrier after scope = %v, want os.ErrClosed", err)
				}
				return nil
			}})
			if err := errors.Join(err, rootScope.Validate(), rootScope.OperationError, rootScope.CleanupError); err != nil {
				t.Fatal(err)
			}
		})
	}
}
