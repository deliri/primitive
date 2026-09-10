//go:build darwin || linux

package filelock_test

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filelock"
	"github.com/deliri/primitive/v2026/filestore"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"testing"
)

func fixtureFlags(t *testing.T, file *os.File) int {
	t.Helper()
	conn, err := file.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var flags int
	var nativeErr error
	if err := conn.Control(func(fd uintptr) { flags, nativeErr = unix.FcntlInt(fd, unix.F_GETFL, 0) }); err != nil {
		t.Fatal(err)
	}
	if nativeErr != nil {
		t.Fatal(nativeErr)
	}
	return flags
}
func TestUnixBorrowPreservesDescriptorModeLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		pipe    bool
		release bool
	}{
		{"pipe_acquisition_preserves_nonblocking_io", true, false},
		{"pipe_unlock_preserves_nonblocking_io", true, true},
		{"regular_file_acquisition_preserves_flags", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file := openFixtureLock(t, filepath.Join(t.TempDir(), "flags.lock"))
			if tc.pipe {
				pipe, err := filestore.OpenPipe(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := errors.Join(pipe.Reader.Close(), pipe.Writer.Close()); err != nil {
						t.Errorf("pipe close = %v, want nil", err)
					}
				})
				file = pipe.Reader
			}
			before := fixtureFlags(t, file)
			if tc.pipe && before&unix.O_NONBLOCK == 0 {
				t.Fatal("pipe nonblocking = false, want true before production call")
			}
			conn, err := file.SyscallConn()
			if err != nil {
				t.Fatal(err)
			}
			flags := unix.LOCK_EX | unix.LOCK_NB
			if tc.release {
				flags = unix.LOCK_UN
			}
			var wantErr error
			if err := conn.Control(func(fd uintptr) {
				wantErr = unix.Flock(int(fd), flags)
				if wantErr == nil {
					wantErr = unix.Flock(int(fd), unix.LOCK_UN)
				}
			}); err != nil {
				t.Fatal(err)
			}
			var gotErr error
			if tc.release {
				gotErr = filelock.Release(t.Context(), file)
			} else {
				var got filelock.Acquisition
				got, gotErr = filelock.Acquire(t.Context(), filelock.Request{File: file, Exclusivity: filelock.Exclusive, Patience: filelock.Immediate})
				if gotErr == nil {
					held, err := got.Held()
					if err != nil || !held {
						t.Fatalf("held=%v error=%v, want native acquired outcome", held, err)
					}
					if err := filelock.Release(t.Context(), file); err != nil {
						t.Fatal(err)
					}
				} else if got != (filelock.Acquisition{}) {
					t.Fatalf("refused outcome=%v, want zero", got)
				}
			}
			after := fixtureFlags(t, file)
			if !errors.Is(gotErr, wantErr) || after != before {
				t.Fatalf("error=%v descriptor flags=%#x, want %v and unchanged %#x", gotErr, after, wantErr, before)
			}
		})
	}
}
func TestClosedFilePreservesGoControlFailureLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		exclusivity filelock.Exclusivity
		patience    filelock.Patience
		release     bool
	}{
		{"exclusive_immediate", filelock.Exclusive, filelock.Immediate, false},
		{"shared_immediate", filelock.Shared, filelock.Immediate, false},
		{"exclusive_blocking", filelock.Exclusive, filelock.Blocking, false},
		{"shared_blocking", filelock.Shared, filelock.Blocking, false},
		{"closed_release", filelock.ExclusivityUnknown, filelock.PatienceUnknown, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file := openFixtureLock(t, filepath.Join(t.TempDir(), "closed.lock"))
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			conn, err := file.SyscallConn()
			if err != nil {
				t.Fatal(err)
			}
			nativeErr := conn.Control(func(uintptr) { t.Error("closed descriptor callback reached = true, want false") })
			if nativeErr == nil {
				t.Fatal("closed native control error = nil, want refusal")
			}
			got := filelock.Acquisition{}
			if tc.release {
				err = filelock.Release(t.Context(), file)
			} else {
				got, err = filelock.Acquire(t.Context(), filelock.Request{File: file, Exclusivity: tc.exclusivity, Patience: tc.patience})
			}
			if !errors.Is(err, nativeErr) || !errors.Is(err, core.ErrFileLockUnavailable) || got != (filelock.Acquisition{}) {
				t.Fatalf("outcome=%v error=%v, want zero and Go control failure %v", got, err, nativeErr)
			}
		})
	}
}
