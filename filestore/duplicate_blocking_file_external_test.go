//go:build darwin || linux

package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestDuplicateBlockingFileNativeLayerTriad(t *testing.T) {
	t.Parallel()
	for _, closeSource := range []bool{false, true} {
		name := "duplicate closure leaves source usable"
		if closeSource {
			name = "source closure leaves duplicate usable through EOF"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			source, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer source.Close()
			defer writer.Close()
			before := duplicateNativeFlags(t, source)
			if before&syscall.O_NONBLOCK == 0 {
				t.Fatalf("initial pipe flags=%#x, want Go nonblocking pipe", before)
			}
			duplicate, err := filestore.DuplicateBlockingFile(t.Context(), filestore.FileHandleRequest{File: source})
			if err != nil || duplicate == nil {
				t.Fatalf("DuplicateBlockingFile = %v/%v, want native duplicate", duplicate, err)
			}
			defer duplicate.Close()
			if source == duplicate {
				t.Fatalf("returned handle=%v, want independently owned duplicate", duplicate)
			}
			if got := duplicateNativeFlags(t, source); got&syscall.O_NONBLOCK != 0 {
				t.Fatalf("source flags=%#x, want shared blocking description", got)
			}
			if got := duplicateNativeFlags(t, duplicate); got&syscall.O_NONBLOCK != 0 {
				t.Fatalf("duplicate flags=%#x, want blocking description", got)
			}
			reader := duplicate
			if closeSource {
				if err := source.Close(); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := duplicate.Close(); err != nil {
					t.Fatal(err)
				}
				reader = source
			}
			want := []byte{0, 255, 10, 47, 31}
			if count, err := writer.Write(want); count != len(want) || err != nil {
				t.Fatalf("native peer write=%d/%v, want %d", count, err, len(want))
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			got, err := io.ReadAll(reader)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("remaining native bytes=%v/%v, want %v followed by EOF", got, err, want)
			}
		})
	}
}

func TestDuplicateBlockingFileSharesNativeOffset(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	path := filepath.Join(directory, "source")
	if err := os.WriteFile(path, []byte("first-tail"), 0o600); err != nil {
		t.Fatal(err)
	}
	source, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	duplicate, err := filestore.DuplicateBlockingFile(t.Context(), filestore.FileHandleRequest{File: source})
	if err != nil {
		t.Fatal(err)
	}
	defer duplicate.Close()
	first := make([]byte, 6)
	if count, err := io.ReadFull(source, first); count != len(first) || err != nil || string(first) != "first-" {
		t.Fatalf("source prefix=%q/%v, want first-", first, err)
	}
	got, err := io.ReadAll(duplicate)
	if err != nil || string(got) != "tail" {
		t.Fatalf("duplicate shared-offset bytes=%q/%v, want tail", got, err)
	}
}

func TestDuplicateBlockingFileRefusalLayerTriad(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"nil borrowed file", "closed borrowed file", "cancelled context preserves cause", "nil context"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			source, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer source.Close()
			defer writer.Close()
			before := duplicateNativeFlags(t, source)
			ctx := t.Context()
			request := filestore.FileHandleRequest{File: source}
			want := error(core.ErrFilestoreContract)
			var native error
			switch name {
			case "nil borrowed file":
				request.File = nil
			case "closed borrowed file":
				if err := source.Close(); err != nil {
					t.Fatal(err)
				}
				connection, err := source.SyscallConn()
				if err != nil {
					t.Fatal(err)
				}
				native = connection.Control(func(uintptr) { t.Errorf("closed native descriptor callback ran=%t, want false", true) })
				if native == nil {
					t.Fatal("closed native control error=nil, want refusal")
				}
				want = core.ErrFilestoreSource
			case "cancelled context preserves cause":
				cancelled, cancel := context.WithCancelCause(ctx)
				cancel(fs.ErrPermission)
				ctx = cancelled
				want, native = context.Canceled, fs.ErrPermission
			case "nil context":
				ctx = nil
				want = core.ErrNilContext
			default:
				t.Fatalf("fixture name=%q, want declared refusal", name)
			}
			duplicate, err := filestore.DuplicateBlockingFile(ctx, request)
			if duplicate != nil {
				_ = duplicate.Close()
				t.Fatalf("refused duplicate=%v, want nil", duplicate)
			}
			if !errors.Is(err, want) || native != nil && !errors.Is(err, native) {
				t.Fatalf("DuplicateBlockingFile = %v, want %v and native %v", err, want, native)
			}
			if name != "closed borrowed file" {
				if got := duplicateNativeFlags(t, source); got != before {
					t.Fatalf("refused source flags=%#x, want unchanged %#x", got, before)
				}
				if _, err := source.Stat(); err != nil {
					t.Fatalf("refused source=%v, want still open", err)
				}
			}
		})
	}
}

// Observe flags through RawConn without calling Fd, which would itself perform
// the blocking transition and make this oracle unable to detect its omission.
func duplicateNativeFlags(t testing.TB, file *os.File) uintptr {
	t.Helper()
	connection, err := file.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var flags uintptr
	var native syscall.Errno
	err = connection.Control(func(descriptor uintptr) {
		flags, _, native = syscall.Syscall(syscall.SYS_FCNTL, descriptor, syscall.F_GETFL, 0)
	})
	if err != nil || native != 0 {
		t.Fatalf("native descriptor flags=%#x/%v/%v, want held F_GETFL observation", flags, err, native)
	}
	return flags
}
