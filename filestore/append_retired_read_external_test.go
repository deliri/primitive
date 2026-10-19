package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestFailedRotationReadsOnlyItsOriginalRetiredNativeFile(t *testing.T) {
	t.Parallel()
	owner, location, path := appendReadOwner(t)
	body := []byte("actual retiring prefix\n")
	if _, err := owner.Write(body); err != nil {
		t.Fatal(err)
	}
	next := location
	next.Path = mustRelativePath(t, "blocked")
	if err := os.Mkdir(filepath.Join(filepath.Dir(path), "blocked"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := owner.Rotate(t.Context(), filestore.AppendRequest{Location: next, Mode: 0600, Append: filestore.AppendCreate}); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("native incoming refusal=%v", err)
	}
	observed, err := owner.ObserveAppend()
	if err != nil || observed.Custody != filestore.AppendCustodyReleased || !errors.Is(observed.RotationError, fs.ErrExist) {
		t.Fatalf("native cutover=%+v/%v", observed, err)
	}
	if _, err := owner.Write([]byte("forbidden")); !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("read observation revived append: %v", err)
	}
	proveRetiredRead(t, owner, location, body)
	if err := os.Rename(path, path+"-original"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	called := false
	scope, err := owner.WithInspectedReadScope(t.Context(), filestore.InspectedReadScopeRequest{Location: location, Use: func(context.Context, filestore.InspectedRead) error { called = true; return nil }})
	if err != nil || !errors.Is(scope.OperationError(), core.ErrFilestoreContract) || called {
		t.Fatalf("same-byte foreign replacement borrowed retired identity: %+v/%v/%t", scope, err, called)
	}
	location.Path = mustRelativePath(t, "original-original")
	proveRetiredRead(t, owner, location, body)
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.WithInspectedReadScope(t.Context(), filestore.InspectedReadScopeRequest{Location: location, Use: func(context.Context, filestore.InspectedRead) error { return nil }}); !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("explicit close retained retired source authority: %v", err)
	}
	actual, err := os.ReadFile(path + "-original")
	if err != nil || !bytes.Equal(actual, body) {
		t.Fatalf("read settlement changed retired Native file: %q/%v", actual, err)
	}
}

func proveRetiredRead(t testing.TB, owner *filestore.AppendDestination, location filestore.Location, want []byte) {
	t.Helper()
	var borrowed filestore.InspectedReadSource
	scope, err := owner.WithInspectedReadScope(t.Context(), filestore.InspectedReadScopeRequest{Location: location, Use: func(_ context.Context, read filestore.InspectedRead) error {
		borrowed = read.Source
		actual, err := io.ReadAll(read.Source)
		if err != nil {
			return err
		}
		if !bytes.Equal(actual, want) {
			t.Fatalf("retired Native bytes=%q, want %q", actual, want)
		}
		return nil
	}})
	if err != nil || scope.Validate() != nil || scope.OperationError() != nil || scope.CleanupError() != nil {
		t.Fatalf("actual retired read=%+v/%v", scope, err)
	}
	if _, err := borrowed.Read(make([]byte, 1)); !errors.Is(err, fs.ErrClosed) {
		t.Fatalf("retired native read escaped cleanup: %v", err)
	}
}

func FuzzRetiredAppendSourceAgreesWithOriginalNativeIdentity(f *testing.F) {
	for _, body := range [][]byte{nil, []byte("row\n"), {0, 255, 1}} {
		f.Add(body, false)
		f.Add(body, true)
	}
	f.Fuzz(func(t *testing.T, body []byte, replace bool) {
		owner, location, path := appendReadOwner(t)
		if _, err := owner.Write(body); err != nil {
			t.Fatal(err)
		}
		next := location
		next.Path = mustRelativePath(t, "blocked")
		if err := os.Mkdir(filepath.Join(filepath.Dir(path), "blocked"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := owner.Rotate(t.Context(), filestore.AppendRequest{Location: next, Mode: 0600, Append: filestore.AppendCreate}); !errors.Is(err, fs.ErrExist) {
			t.Fatal(err)
		}
		if replace {
			if err := os.Rename(path, path+"-held"); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, body, 0600); err != nil {
				t.Fatal(err)
			}
			called := false
			scope, err := owner.WithInspectedReadScope(t.Context(), filestore.InspectedReadScopeRequest{Location: location, Use: func(context.Context, filestore.InspectedRead) error { called = true; return nil }})
			if err != nil || called || !errors.Is(scope.OperationError(), core.ErrFilestoreContract) {
				t.Fatalf("retired replacement invented authority: %+v/%v/%t", scope, err, called)
			}
		} else {
			proveRetiredRead(t, owner, location, body)
		}
	})
}
