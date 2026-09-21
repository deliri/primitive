package filelock_test

import (
	"context"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filelock"
	"github.com/deliri/primitive/v2026/filestore"
	"os"
	"path/filepath"
	"testing"
)

func openFixtureLock(t testing.TB, path string) *os.File {
	t.Helper()
	absolute, err := core.ParseAbsolutePath(path)
	if err != nil {
		t.Fatal(err)
	}
	location, err := filestore.OpenParent(context.Background(), absolute)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := location.Root.Close(); err != nil {
			t.Errorf("root close = %v, want nil", err)
		}
	}()
	file, err := filestore.OpenLockFile(context.Background(), filestore.LockFileRequest{Location: location, Mode: 0600})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			t.Errorf("file close = %v, want nil or previously closed", err)
		}
	})
	return file
}
func BenchmarkAdvisoryLock(b *testing.B) {
	b.ReportAllocs()
	for _, tc := range []struct {
		name     string
		holder   filelock.Exclusivity
		request  filelock.Exclusivity
		wantHeld bool
	}{
		{name: "exclusive_acquire_release", holder: filelock.ExclusivityUnknown, request: filelock.Exclusive, wantHeld: true},
		{name: "shared_acquire_release", holder: filelock.ExclusivityUnknown, request: filelock.Shared, wantHeld: true},
		{name: "exclusive_contention", holder: filelock.Exclusive, request: filelock.Exclusive, wantHeld: false},
		{name: "shared_compatible_holder", holder: filelock.Shared, request: filelock.Shared, wantHeld: true},
	} {
		b.Run(tc.name, func(b *testing.B) {
			path := filepath.Join(b.TempDir(), "benchmark.lock")
			holder, file := openFixtureLock(b, path), openFixtureLock(b, path)
			ctx := b.Context()
			if tc.holder != filelock.ExclusivityUnknown {
				acquired, err := filelock.Acquire(ctx, filelock.Request{File: holder, Exclusivity: tc.holder, Patience: filelock.Immediate})
				if err != nil {
					b.Fatal(err)
				}
				held, err := acquired.Held()
				if err != nil || !held {
					b.Fatalf("holder=%v error=%v, want held", held, err)
				}
			}
			request := filelock.Request{File: file, Exclusivity: tc.request, Patience: filelock.Immediate}
			if err := request.Validate(); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			for b.Loop() {
				acquired, err := filelock.Acquire(ctx, request)
				if err != nil {
					b.Fatal(err)
				}
				held, err := acquired.Held()
				if err != nil || held != tc.wantHeld {
					b.Fatalf("held=%v error=%v, want %v", held, err, tc.wantHeld)
				}
				if held {
					if err := filelock.Release(ctx, file); err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}
