package filelock_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/filelock"
)

func BenchmarkImmediateExclusiveAcquireRelease(b *testing.B) {
	path := filepath.Join(b.TempDir(), "benchmark.lock")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		b.Fatalf("OpenFile(%s) error = %v, want nil", path, err)
	}
	b.Cleanup(func() { _ = file.Close() })
	ctx := context.Background()
	request := filelock.Request{
		File: file, Exclusivity: filelock.Exclusive, Patience: filelock.Immediate,
	}
	if err := request.Validate(); err != nil {
		b.Fatalf("Request.Validate() error = %v, want nil", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		acquisition, acquireErr := filelock.Acquire(ctx, request)
		if acquireErr != nil {
			b.Fatalf("Acquire() error = %v, want nil", acquireErr)
		}
		held, heldErr := acquisition.Held()
		if heldErr != nil || !held {
			b.Fatalf("Acquisition.Held() = (%t, %v), want (true, nil)", held, heldErr)
		}
		if releaseErr := filelock.Release(ctx, file); releaseErr != nil {
			b.Fatalf("Release() error = %v, want nil", releaseErr)
		}
	}
}
