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

func heldReadOwner(t testing.TB, access filestore.AppendAccess) (*filestore.AppendDestination, filestore.Location, string) {
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
	owner, err := filestore.OpenAppendDestination(t.Context(), filestore.AppendRequest{Location: location, Mode: 0600, Append: filestore.AppendCreate, Access: access})
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

func TestHeldAppendReadUsesOriginalObjectAcrossNamespaceLossAndRotation(t *testing.T) {
	t.Parallel()
	owner, location, path := heldReadOwner(t, filestore.AppendAccessReadWrite)
	body := []byte("original buffered source\n")
	if _, err := owner.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, path+"-held"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("foreign name occupant"), 0600); err != nil {
		t.Fatal(err)
	}
	proveHeldRead(t, owner, body)
	actual, err := os.ReadFile(path + "-held")
	if err != nil || !bytes.Equal(actual, body) {
		t.Fatalf("original native object=%q/%v", actual, err)
	}
	if err := os.Remove(path + "-held"); err != nil {
		t.Fatal(err)
	}
	proveHeldRead(t, owner, body)
	if _, err := owner.Write([]byte("continued\n")); err != nil {
		t.Fatal(err)
	}
	proveHeldRead(t, owner, append(bytes.Clone(body), []byte("continued\n")...))
	next := location
	next.Path = mustRelativePath(t, "incoming")
	if err := owner.Rotate(t.Context(), filestore.AppendRequest{Location: next, Mode: 0600, Append: filestore.AppendCreate, Access: filestore.AppendAccessReadWrite}); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Write([]byte("rotated\n")); err != nil {
		t.Fatal(err)
	}
	proveHeldRead(t, owner, []byte("rotated\n"))
}

func proveHeldRead(t testing.TB, owner *filestore.AppendDestination, want []byte) {
	t.Helper()
	var borrowed filestore.InspectedReadSource
	err := owner.WithHeldReadScope(t.Context(), filestore.HeldAppendReadRequest{Use: func(_ context.Context, read filestore.InspectedRead) error {
		borrowed = read.Source
		if _, writable := read.Source.(io.Writer); writable {
			t.Fatal("borrowed writer authority")
		}
		if _, closable := read.Source.(io.Closer); closable {
			t.Fatal("borrowed close authority")
		}
		actual, err := io.ReadAll(read.Source)
		if err != nil {
			return err
		}
		extent, err := read.Metadata.SizeBytes()
		if err != nil {
			return err
		}
		if !bytes.Equal(actual, want) || extent.Uint64() != uint64(len(want)) {
			t.Fatalf("held bytes/metadata=%q/%d, want %q/%d", actual, extent.Uint64(), want, len(want))
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := borrowed.Read(make([]byte, 1)); !errors.Is(err, fs.ErrClosed) {
		t.Fatalf("read escaped native lifetime: %v", err)
	}
	if _, err := borrowed.ReadAt(make([]byte, 1), 0); !errors.Is(err, fs.ErrClosed) {
		t.Fatalf("positional read escaped native lifetime: %v", err)
	}
	if _, err := borrowed.Seek(0, io.SeekStart); !errors.Is(err, fs.ErrClosed) {
		t.Fatalf("seek escaped native lifetime: %v", err)
	}
}

func TestHeldAppendReadPreservesNativeAccessAndLifecycleRefusals(t *testing.T) {
	t.Parallel()
	t.Run("write only native access", func(t *testing.T) {
		owner, _, _ := heldReadOwner(t, filestore.AppendAccessWriteOnly)
		if _, err := owner.Write([]byte("visible native bytes")); err != nil {
			t.Fatal(err)
		}
		err := owner.WithHeldReadScope(t.Context(), filestore.HeldAppendReadRequest{Use: func(_ context.Context, read filestore.InspectedRead) error {
			_, err := read.Source.Read(make([]byte, 1))
			return err
		}})
		var native *fs.PathError
		if !errors.As(err, &native) {
			t.Fatalf("write-only descriptor invented readable bytes: %v", err)
		}
	})
	t.Run("cancelled and panicking callback", func(t *testing.T) {
		for _, panicCallback := range []bool{false, true} {
			owner, _, _ := heldReadOwner(t, filestore.AppendAccessReadWrite)
			ctx, cancel := context.WithCancelCause(t.Context())
			var borrowed filestore.InspectedReadSource
			err := owner.WithHeldReadScope(ctx, filestore.HeldAppendReadRequest{Use: func(_ context.Context, read filestore.InspectedRead) error {
				borrowed = read.Source
				if panicCallback {
					panic("native borrower")
				}
				cancel(fs.ErrPermission)
				return nil
			}})
			cancel(nil)
			if panicCallback {
				if !errors.Is(err, core.ErrFilestoreCallbackPanic) {
					t.Fatalf("native callback panic=%v", err)
				}
			} else if !errors.Is(err, context.Canceled) || !errors.Is(err, fs.ErrPermission) {
				t.Fatalf("native cancellation lost cause=%v", err)
			}
			if _, err := borrowed.Read(make([]byte, 1)); err == nil {
				t.Fatal("refused callback retained usable source")
			}
			if _, err := owner.Write([]byte("still owned")); err != nil {
				t.Fatalf("borrower stole append custody: %v", err)
			}
		}
	})
	t.Run("invalid and settled owner", func(t *testing.T) {
		owner, _, _ := heldReadOwner(t, filestore.AppendAccessReadWrite)
		copied := *owner
		called := false
		request := filestore.HeldAppendReadRequest{Use: func(context.Context, filestore.InspectedRead) error { called = true; return nil }}
		for _, candidate := range []*filestore.AppendDestination{nil, &copied} {
			if err := candidate.WithHeldReadScope(t.Context(), request); !errors.Is(err, core.ErrFilestoreContract) || called {
				t.Fatalf("invalid owner borrowed original: %v", err)
			}
		}
		if err := owner.Close(); err != nil {
			t.Fatal(err)
		}
		if err := owner.WithHeldReadScope(t.Context(), request); !errors.Is(err, core.ErrFilestoreContract) || called {
			t.Fatalf("settled owner borrowed original: %v", err)
		}
	})
}

func FuzzHeldAppendReadMatchesIndependentBytesAfterNameRemoval(f *testing.F) {
	for _, body := range [][]byte{nil, []byte("a\nb\n"), {0, 255, 1}, bytes.Repeat([]byte("row\n"), 1025)} {
		f.Add(body, false)
		f.Add(body, true)
	}
	f.Fuzz(func(t *testing.T, body []byte, remove bool) {
		owner, _, path := heldReadOwner(t, filestore.AppendAccessReadWrite)
		if _, err := owner.Write(body); err != nil {
			t.Fatal(err)
		}
		if remove {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		}
		proveHeldRead(t, owner, body)
	})
}

func TestAppendAccessHasOnlyNativeDescriptorModes(t *testing.T) {
	t.Parallel()
	for raw := 0; raw < 256; raw++ {
		access := filestore.AppendAccess(raw)
		want := raw == 0 || raw == 1
		if access.IsValid() != want || (access.Validate() == nil) != want {
			t.Fatalf("native descriptor mode %d validation", raw)
		}
	}
	if err := (filestore.HeldAppendReadRequest{}).Validate(); !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("absent read consumer admitted: %v", err)
	}
}
