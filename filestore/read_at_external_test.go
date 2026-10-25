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

func TestReadAtBorrowedNativeLayerTriad(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		size   int
		offset int64
		window int
	}{
		{name: "empty source retains native EOF", window: 1},
		{name: "empty window is a native no-op", size: 5},
		{name: "first byte", size: 5, window: 1},
		{name: "interior window", size: 5, offset: 1, window: 3},
		{name: "exact extent has no EOF", size: 5, window: 5},
		{name: "partial count retains exact EOF", size: 5, offset: 3, window: 4},
		{name: "at extent retains EOF", size: 5, offset: 5, window: 1},
		{name: "beyond extent retains EOF", size: 5, offset: 6, window: 1},
		{name: "negative offset retains native refusal", size: 5, offset: -1, window: 1},
		{name: "native signed maximum offset", size: 5, offset: 1<<63 - 1, window: 1},
		{name: "many hash windows continue", size: 131073, offset: 65535, window: 65537},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			path := filepath.Join(directory, "source")
			payload := bytes.Repeat([]byte{0, 255, 31}, (row.size+2)/3)[:row.size]
			if err := os.WriteFile(path, payload, 0o600); err != nil {
				t.Fatal(err)
			}
			file, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := file.Close(); err != nil {
					t.Error(err)
				}
			})
			if _, err := file.Seek(2, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			got := bytes.Repeat([]byte{19}, row.window)
			want := bytes.Clone(got)
			wantCount, wantErr := file.ReadAt(want, row.offset)
			gotCount, gotErr := filestore.ReadAt(t.Context(), filestore.ReadAtRequest{File: file, Data: got, Offset: row.offset})
			if gotCount != wantCount || !bytes.Equal(got, want) || !sameReadAtNativeError(gotErr, wantErr, row.offset) {
				t.Fatalf("ReadAt = (%d,%v,%v), want native (%d,%v,%v)", gotCount, got, gotErr, wantCount, want, wantErr)
			}
			position, err := file.Seek(0, io.SeekCurrent)
			if position != 2 || err != nil {
				t.Fatalf("borrowed cursor = (%d,%v), want (2,nil)", position, err)
			}
		})
	}
}

func sameReadAtNativeError(got, want error, offset int64) bool {
	if want == nil {
		return got == nil
	}
	if want == io.EOF {
		return got == io.EOF
	}
	native, ok := errors.AsType[*os.PathError](want)
	observed, found := errors.AsType[*os.PathError](got)
	if !ok || !found || native.Op != observed.Op || native.Path != observed.Path {
		return false
	}
	if errors.Is(got, native.Err) {
		return true
	}
	// Go constructs a fresh, non-sentinel negative-offset cause on every call.
	// Its stable contract is the typed operation/path and an opaque cause;
	// neither prose nor identity across separate native calls is authority.
	return offset < 0 && native.Op == "readat" && native.Err != nil && observed.Err != nil
}

func TestReadAtRetainsBorrowedInodeAfterRename(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	path := filepath.Join(directory, "source")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := os.Rename(path, filepath.Join(directory, "retained")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("stranger"), 0o600); err != nil {
		t.Fatal(err)
	}
	var got [8]byte
	n, err := filestore.ReadAt(t.Context(), filestore.ReadAtRequest{File: file, Data: got[:]})
	if n != len(got) || err != nil || string(got[:]) != "original" {
		t.Fatalf("held read = (%d,%q,%v), want original inode", n, got, err)
	}
	var next [1]byte
	n, err = file.Read(next[:])
	if n != 1 || err != nil || next[0] != 'o' {
		t.Fatalf("borrowed source remains usable at original cursor = (%d,%q,%v), want o", n, next, err)
	}
}

func TestReadAtPreservesNativeHandleFailures(t *testing.T) {
	t.Parallel()
	for _, role := range []string{"closed", "write-only", "directory"} {
		t.Run(role, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			path := filepath.Join(directory, "source")
			if err := os.WriteFile(path, []byte("retained"), 0o600); err != nil {
				t.Fatal(err)
			}
			flags := os.O_RDONLY
			if role == "write-only" {
				flags = os.O_WRONLY
			}
			if role == "directory" {
				path = directory
			}
			file, err := os.OpenFile(path, flags, 0)
			if err != nil {
				t.Fatal(err)
			}
			if role == "closed" {
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
			} else {
				t.Cleanup(func() {
					if err := file.Close(); err != nil {
						t.Error(err)
					}
				})
			}
			got, want := []byte{17}, []byte{17}
			wantCount, wantErr := file.ReadAt(want, 0)
			gotCount, gotErr := filestore.ReadAt(t.Context(), filestore.ReadAtRequest{File: file, Data: got})
			if wantErr == nil || gotCount != wantCount || !bytes.Equal(got, want) || !sameReadAtNativeError(gotErr, wantErr, 0) {
				t.Fatalf("ReadAt = (%d,%v,%v), want native (%d,%v,%v)", gotCount, got, gotErr, wantCount, want, wantErr)
			}
		})
	}
}

func TestReadAtRefusesBeforeTouchingDestination(t *testing.T) {
	t.Parallel()
	for _, role := range []string{"absent file", "nil context", "cancelled context"} {
		t.Run(role, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			path := filepath.Join(directory, "source")
			if err := os.WriteFile(path, []byte("visible"), 0o600); err != nil {
				t.Fatal(err)
			}
			file, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := file.Close(); err != nil {
					t.Error(err)
				}
			})
			ctx := t.Context()
			var want error = core.ErrFilestoreContract
			request := filestore.ReadAtRequest{File: file, Data: []byte{17}}
			switch role {
			case "absent file":
				request.File = nil
			case "nil context":
				ctx = nil
				want = core.ErrNilContext
			case "cancelled context":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
				want = context.Canceled
			}
			n, err := filestore.ReadAt(ctx, request)
			if n != 0 || !errors.Is(err, want) || !bytes.Equal(request.Data, []byte{17}) {
				t.Fatalf("refused read = (%d,%v,%v), want zero, %v and untouched window", n, request.Data, err, want)
			}
		})
	}
}

// terminalReadAtContext cancels a real context only after the native read has
// changed the borrowed destination. Additional admission checks do not trigger
// cancellation. The test uses this context and destination synchronously.
type terminalReadAtContext struct {
	context.Context
	cancel context.CancelCauseFunc
	data   []byte
}

func (c *terminalReadAtContext) Err() error {
	if len(c.data) != 0 && c.data[0] == 'a' {
		c.cancel(io.ErrClosedPipe)
	}
	return c.Context.Err()
}

func TestReadAtTerminalObservationRetainsCountAndBothCauses(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		window int
	}{
		{name: "complete native read retains terminal cause", window: 1},
		{name: "partial native read retains EOF and terminal cause", window: 4},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			path := filepath.Join(directory, "source")
			if err := os.WriteFile(path, []byte("ab"), 0o600); err != nil {
				t.Fatal(err)
			}
			file, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := file.Close(); err != nil {
					t.Error(err)
				}
			})
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			got := bytes.Repeat([]byte{19}, row.window)
			terminal := &terminalReadAtContext{Context: ctx, cancel: cancel, data: got}
			want := bytes.Clone(got)
			wantCount, nativeErr := file.ReadAt(want, 0)
			n, err := filestore.ReadAt(terminal, filestore.ReadAtRequest{File: file, Data: got})
			if n != wantCount || !bytes.Equal(got, want) || !errors.Is(err, context.Canceled) || !errors.Is(err, io.ErrClosedPipe) || nativeErr != nil && !errors.Is(err, nativeErr) {
				t.Fatalf("terminal read = (%d,%v,%v), want native (%d,%v,%v) plus cancelled and original cause", n, got, err, wantCount, want, nativeErr)
			}
		})
	}
}
