package filestore_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestScratchScopeOwnsNativeLifetime(t *testing.T) {
	t.Parallel()
	refusal := errors.New("scope consumer refused")
	for _, tc := range []struct {
		name   string
		fail   error
		cancel bool
	}{
		{"completed native write", nil, false},
		{"failed consumer preserves identity", refusal, false},
		{"canceled consumer still cleans namespace", context.Canceled, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			parent, err := core.ParseAbsolutePath(directory)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var borrowed *os.Root
			err = filestore.WithScratchScope(ctx, filestore.ScratchScopeRequest{Parent: parent, Use: func(ctx context.Context, root *os.Root) error {
				borrowed = root
				path, err := core.ParseRelativePath("owned")
				if err != nil {
					return err
				}
				file, err := filestore.OpenScratch(ctx, filestore.ScratchRequest{Location: filestore.Location{Root: root, Path: path}, Mode: 0600})
				if err != nil {
					return err
				}
				if err := file.Close(); err != nil {
					return err
				}
				if tc.cancel {
					cancel()
				}
				return tc.fail
			}})
			if !errors.Is(err, tc.fail) {
				t.Fatalf("WithScratchScope() = %v, want %v", err, tc.fail)
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != 0 {
				t.Fatalf("scope residue = %d entries, %v", len(entries), err)
			}
			if borrowed == nil {
				t.Fatal("scope was never lent")
			}
			if _, err := borrowed.Stat("."); err == nil {
				t.Fatal("borrowed root remained usable after scope returned")
			}
		})
	}
}

func TestScratchScopeRefusesInvalidIntentBeforeUse(t *testing.T) {
	t.Parallel()
	parent, err := core.ParseAbsolutePath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	for _, tc := range []struct {
		name       string
		ctx        context.Context
		parent     core.AbsolutePath
		missingUse bool
		want       error
	}{
		{"nil context", nil, parent, false, core.ErrNilContext},
		{"canceled ingress", canceled, parent, false, context.Canceled},
		{"missing parent", t.Context(), core.AbsolutePath{}, false, core.ErrFilestoreContract},
		{"missing consumer", t.Context(), parent, true, core.ErrFilestoreContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			use := func(context.Context, *os.Root) error { calls++; return nil }
			if tc.missingUse {
				use = nil
			}
			err := filestore.WithScratchScope(tc.ctx, filestore.ScratchScopeRequest{Parent: tc.parent, Use: use})
			if !errors.Is(err, tc.want) || calls != 0 {
				t.Fatalf("scope = %v, calls=%d; want %v and no execution", err, calls, tc.want)
			}
		})
	}
}
