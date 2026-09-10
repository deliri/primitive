//go:build windows

package filelock

import (
	"context"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"testing"
)

// This adapter test opens an existing Filestore-owned fixture with the native
// overlapped flag because the asynchronous handle itself is under test.
func windowsAsyncFixture(t *testing.T, path string) *os.File {
	t.Helper()
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OVERLAPPED, 0)
	if err != nil {
		t.Fatal(err)
	}
	file := os.NewFile(uintptr(handle), path)
	if file == nil {
		t.Fatalf("Go file = nil, want owned handle; close error=%v", windows.CloseHandle(handle))
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Errorf("async close=%v, want nil", err)
		}
	})
	return file
}
func TestWindowsPendingLockJoinsNativeCompletion(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		holder Exclusivity
		flags  uint32
	}{
		{"exclusive_waits_for_exclusive", Exclusive, windows.LOCKFILE_EXCLUSIVE_LOCK},
		{"exclusive_waits_for_shared", Shared, windows.LOCKFILE_EXCLUSIVE_LOCK},
		{"shared_waits_for_exclusive", Exclusive, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path, err := core.ParseAbsolutePath(filepath.Join(t.TempDir(), "overlapped.lock"))
			if err != nil {
				t.Fatal(err)
			}
			location, err := filestore.OpenParent(t.Context(), path)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := location.Root.Close(); err != nil {
					t.Errorf("root close=%v, want nil", err)
				}
			}()
			holder, err := filestore.OpenLockFile(t.Context(), filestore.LockFileRequest{Location: location, Mode: 0600})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := holder.Close(); err != nil {
					t.Errorf("holder close=%v, want nil", err)
				}
			}()
			held, err := Acquire(t.Context(), Request{File: holder, Exclusivity: tc.holder, Patience: Immediate})
			if err != nil || held.Validate() != nil {
				t.Fatalf("fixture outcome=%v error=%v, want valid acquisition", held, err)
			}
			acquired, err := held.Held()
			if err != nil || !acquired {
				t.Fatalf("fixture held=%v error=%v, want true,nil", acquired, err)
			}
			waiter := windowsAsyncFixture(t, path.String())
			duration, err := temporal.DurationFromSeconds(10)
			if err != nil {
				t.Fatal(err)
			}
			watchdog, stop, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: t.Context(), Duration: duration})
			if err != nil {
				t.Fatal(err)
			}
			defer stop()
			pending := make(chan error, 1)
			done := make(chan error, 1)
			go func() {
				done <- controlFile(waiter, func(fd uintptr) error {
					handle := windows.Handle(fd)
					return windowsLockOperation(handle, func(ol *windows.Overlapped) error {
						err := windows.LockFileEx(handle, tc.flags, 0, lockRegionLength, lockRegionLength, ol)
						pending <- err
						return err
					})
				})
			}()
			var nativeErr error
			select {
			case nativeErr = <-pending:
			case <-watchdog.Done():
				t.Fatal("native lock entered=false, want true")
			}
			releaseErr := Release(context.Background(), holder)
			var completionErr error
			select {
			case completionErr = <-done:
			case <-watchdog.Done():
				t.Fatal("native completion joined=false, want true")
			}
			if !errors.Is(nativeErr, windows.ERROR_IO_PENDING) || releaseErr != nil || completionErr != nil {
				t.Fatalf("native=%v release=%v completion=%v, want pending,nil,nil", nativeErr, releaseErr, completionErr)
			}
			// The pending result must represent a real acquired range.
			probe, err := Acquire(t.Context(), Request{File: holder, Exclusivity: Exclusive, Patience: Immediate})
			acquired, heldErr := probe.Held()
			if err != nil || heldErr != nil || acquired {
				t.Fatalf("probe held=%v errors=%v/%v, want contention", acquired, err, heldErr)
			}
			if err := Release(t.Context(), waiter); err != nil {
				t.Fatal(err)
			}
			probe, err = Acquire(t.Context(), Request{File: holder, Exclusivity: Exclusive, Patience: Immediate})
			acquired, heldErr = probe.Held()
			if err != nil || heldErr != nil || !acquired {
				t.Fatalf("released probe held=%v errors=%v/%v, want true,nil,nil", acquired, err, heldErr)
			}
		})
	}
}
