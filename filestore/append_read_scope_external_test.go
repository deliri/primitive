package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func appendReadOwner(t testing.TB) (*filestore.AppendDestination, filestore.Location, string) {
	t.Helper()
	directory := t.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	relative, err := core.ParseRelativePath("original")
	if err != nil {
		t.Fatal(err)
	}
	location := filestore.Location{Root: root, Path: relative}
	owner, err := filestore.OpenAppendDestination(t.Context(), filestore.AppendRequest{Location: location, Mode: 0600, Append: filestore.AppendCreate})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := owner.Close(); err != nil {
			t.Error(err)
		}
	})
	return owner, location, filepath.Join(directory, "original")
}

func TestAppendReadScopeBindsHeldIdentityBeforeFlushing(t *testing.T) {
	t.Parallel()
	owner, location, path := appendReadOwner(t)
	if _, err := owner.Write([]byte("pending actual bytes\n")); err != nil {
		t.Fatal(err)
	}
	twin := filepath.Join(filepath.Dir(path), "twin")
	if err := os.WriteFile(twin, nil, 0600); err != nil {
		t.Fatal(err)
	}
	wrong := location
	wrong.Path = mustRelativePath(t, "twin")
	called := false
	scope, err := owner.WithInspectedReadScope(t.Context(), filestore.InspectedReadScopeRequest{Location: wrong, Use: func(context.Context, filestore.InspectedRead) error { called = true; return nil }})
	if err != nil || scope.Validate() != nil || !errors.Is(scope.OperationError(), core.ErrFilestoreContract) || scope.CleanupError() != nil || called {
		t.Fatalf("same-size different file admitted: %+v/%v/called=%v", scope, err, called)
	}
	actual, err := os.ReadFile(path)
	if err != nil || len(actual) != 0 {
		t.Fatalf("identity refusal flushed unrelated intent: %q/%v", actual, err)
	}
	// A hard link is the same native object; admitting it flushes and lends the
	// original pending bytes, without granting closure or write authority.
	link := filepath.Join(filepath.Dir(path), "same-object")
	if err := os.Link(path, link); err != nil {
		t.Fatal(err)
	}
	same := location
	same.Path = mustRelativePath(t, "same-object")
	var borrowed filestore.InspectedReadSource
	scope, err = owner.WithInspectedReadScope(t.Context(), filestore.InspectedReadScopeRequest{Location: same, Use: func(_ context.Context, observed filestore.InspectedRead) error {
		borrowed = observed.Source
		actual, err := io.ReadAll(observed.Source)
		if err != nil {
			return err
		}
		if !bytes.Equal(actual, []byte("pending actual bytes\n")) {
			t.Fatalf("held read did not flush original accepted bytes: %q", actual)
		}
		extent, err := observed.Metadata.SizeBytes()
		if err != nil {
			return err
		}
		if extent.Uint64() != uint64(len(actual)) {
			t.Fatalf("metadata/held bytes disagree: %d/%d", extent.Uint64(), len(actual))
		}
		return nil
	}})
	if err != nil || scope.Validate() != nil || scope.OperationError() != nil || scope.CleanupError() != nil {
		t.Fatalf("original native object refused: %+v/%v", scope, err)
	}
	if _, err := borrowed.Read(make([]byte, 1)); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("borrowed read lifetime escaped: %v", err)
	}
	if _, err := owner.Write([]byte("still owned\n")); err != nil {
		t.Fatal(err)
	}
	if err := owner.Sync(); err != nil {
		t.Fatal(err)
	}
	actual, err = os.ReadFile(path)
	if err != nil || string(actual) != "pending actual bytes\nstill owned\n" {
		t.Fatalf("read scope stole append custody: %q/%v", actual, err)
	}
}

func TestAppendReadScopeRefusesNamespaceReplacementAndPreservesCauses(t *testing.T) {
	t.Parallel()
	t.Run("replaced path", func(t *testing.T) {
		owner, location, path := appendReadOwner(t)
		if err := os.Rename(path, path+"-held"); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0600); err != nil {
			t.Fatal(err)
		}
		called := false
		scope, err := owner.WithInspectedReadScope(t.Context(), filestore.InspectedReadScopeRequest{Location: location, Use: func(context.Context, filestore.InspectedRead) error { called = true; return nil }})
		if err != nil || !errors.Is(scope.OperationError(), core.ErrFilestoreContract) || called {
			t.Fatalf("replacement borrowed held identity: %+v/%v/%v", scope, err, called)
		}
	})
	t.Run("cancelled borrower", func(t *testing.T) {
		owner, location, _ := appendReadOwner(t)
		ctx, cancel := context.WithCancelCause(t.Context())
		defer cancel(nil)
		var borrowed filestore.InspectedReadSource
		scope, err := owner.WithInspectedReadScope(ctx, filestore.InspectedReadScopeRequest{Location: location, Use: func(_ context.Context, read filestore.InspectedRead) error {
			borrowed = read.Source
			cancel(core.ErrFilestoreDestination)
			return nil
		}})
		if err != nil || !errors.Is(scope.OperationError(), context.Canceled) || !errors.Is(scope.OperationError(), core.ErrFilestoreDestination) || scope.CleanupError() != nil {
			t.Fatalf("cancelled native scope lost cause/cleanup: %+v/%v", scope, err)
		}
		if _, err := borrowed.Read(make([]byte, 1)); !errors.Is(err, os.ErrClosed) {
			t.Fatalf("cancelled borrower survived close: %v", err)
		}
	})
	t.Run("owner changed during borrower", func(t *testing.T) {
		owner, location, _ := appendReadOwner(t)
		next := location
		next.Path = mustRelativePath(t, "incoming")
		scope, err := owner.WithInspectedReadScope(t.Context(), filestore.InspectedReadScopeRequest{Location: location, Use: func(ctx context.Context, _ filestore.InspectedRead) error {
			return owner.Rotate(ctx, filestore.AppendRequest{Location: next, Mode: 0600, Append: filestore.AppendCreate})
		}})
		if err != nil || !errors.Is(scope.OperationError(), core.ErrFilestoreContract) || scope.CleanupError() != nil {
			t.Fatalf("changed owner published read scope: %+v/%v", scope, err)
		}
	})
	t.Run("callback failure and panic close source", func(t *testing.T) {
		for _, panicCallback := range []bool{false, true} {
			owner, location, _ := appendReadOwner(t)
			var borrowed filestore.InspectedReadSource
			scope, err := owner.WithInspectedReadScope(t.Context(), filestore.InspectedReadScopeRequest{Location: location, Use: func(_ context.Context, read filestore.InspectedRead) error {
				borrowed = read.Source
				if panicCallback {
					panic("read borrower")
				}
				return core.ErrFilestoreSource
			}})
			want := core.ErrFilestoreSource
			if panicCallback {
				want = core.ErrFilestoreCallbackPanic
			}
			if err != nil || !errors.Is(scope.OperationError(), want) || scope.CleanupError() != nil {
				t.Fatalf("borrower outcome=%+v/%v", scope, err)
			}
			if _, err := borrowed.Read(make([]byte, 1)); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("failed borrower escaped close: %v", err)
			}
		}
	})
}

func FuzzAppendReadScopeMatchesOriginalNativeObject(f *testing.F) {
	for _, seed := range [][]byte{{}, {0, 255, '\n'}, []byte("a\nb\n"), bytes.Repeat([]byte("row\n"), 1025)} {
		f.Add(seed, false)
		f.Add(seed, true)
	}
	f.Fuzz(func(t *testing.T, body []byte, replace bool) {
		owner, location, path := appendReadOwner(t)
		if n, err := owner.Write(body); err != nil || n != len(body) {
			t.Fatalf("actual append=%d/%v", n, err)
		}
		if replace {
			if err := os.Rename(path, path+"-held"); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, body, 0600); err != nil {
				t.Fatal(err)
			}
		}
		called := false
		scope, err := owner.WithInspectedReadScope(t.Context(), filestore.InspectedReadScopeRequest{Location: location, Use: func(_ context.Context, read filestore.InspectedRead) error {
			called = true
			actual, err := io.ReadAll(read.Source)
			if err != nil {
				return err
			}
			if !bytes.Equal(actual, body) {
				t.Fatalf("native byte oracle mismatch: got %x want %x", actual, body)
			}
			extent, err := read.Metadata.SizeBytes()
			if err != nil {
				return err
			}
			if extent.Uint64() != uint64(len(body)) {
				t.Fatalf("native extent=%d want %d", extent.Uint64(), len(body))
			}
			return nil
		}})
		if err != nil || scope.Validate() != nil || scope.CleanupError() != nil {
			t.Fatalf("native read scope=%+v/%v", scope, err)
		}
		if replace {
			if called || !errors.Is(scope.OperationError(), core.ErrFilestoreContract) {
				t.Fatalf("same bytes on another inode borrowed identity: called=%v scope=%+v", called, scope)
			}
		} else if !called || scope.OperationError() != nil {
			t.Fatalf("original held bytes refused: called=%v scope=%+v", called, scope)
		}
	})
}

func TestAppendReadScopeRejectsNilCopiedAndSettledOwnerBeforeBorrow(t *testing.T) {
	t.Parallel()
	owner, location, _ := appendReadOwner(t)
	copied := *owner
	called := false
	request := filestore.InspectedReadScopeRequest{Location: location, Use: func(context.Context, filestore.InspectedRead) error { called = true; return nil }}
	for _, candidate := range []*filestore.AppendDestination{nil, &copied} {
		scope, err := candidate.WithInspectedReadScope(t.Context(), request)
		if !errors.Is(err, core.ErrFilestoreContract) || scope.Validate() == nil || called {
			t.Fatalf("invalid owner acquired source: %+v/%v/%v", scope, err, called)
		}
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	scope, err := owner.WithInspectedReadScope(t.Context(), request)
	if !errors.Is(err, core.ErrFilestoreContract) || scope.Validate() == nil || called {
		t.Fatalf("settled owner acquired source: %+v/%v/%v", scope, err, called)
	}
}
