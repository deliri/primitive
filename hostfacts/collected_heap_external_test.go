package hostfacts_test

import (
	"context"
	"errors"
	"runtime"
	"runtime/debug"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/temporal"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestCollectedGoHeapOwnsRuntimeCollectionAndObservation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{
		Hazard: core.TestIsolationHazardRuntimeAllocation,
		Scope:  core.TestIsolationScopePackageProcess,
	})
	// This test attacks Hostfacts's runtime adapter itself. Disable automatic
	// collection to distinguish an explicit collection from incidental GC.
	previous := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(previous)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	heap, err := hostfacts.ObserveCollectedGoHeap(t.Context())
	runtime.ReadMemStats(&after)
	if err != nil || heap.Validate() != nil || heap.Uint64() == 0 || after.NumGC <= before.NumGC {
		t.Fatalf("heap observation = %v/%v, collections %d -> %d, want admitted collected heap", heap, err, before.NumGC, after.NumGC)
	}
	if heap.Uint64() > after.HeapAlloc {
		t.Fatalf("observed heap = %d, after heap = %d, want no unexplained later decrease without automatic GC", heap.Uint64(), after.HeapAlloc)
	}
	for _, cancelled := range []bool{false, true} {
		duration := temporal.Duration{}
		if cancelled {
			var err error
			duration, err = temporal.DurationFromSeconds(60)
			if err != nil {
				t.Fatal(err)
			}
		}
		ctx, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: t.Context(), Duration: duration})
		if err != nil {
			t.Fatal(err)
		}
		want := context.DeadlineExceeded
		if cancelled {
			cancel()
			want = context.Canceled
		}
		runtime.ReadMemStats(&before)
		heap, err := hostfacts.ObserveCollectedGoHeap(ctx)
		runtime.ReadMemStats(&after)
		cancel()
		if heap.Uint64() != 0 || !errors.Is(err, want) || after.NumGC != before.NumGC {
			t.Fatalf("terminal heap = %v/%v, collections %d -> %d, want %v before collection", heap, err, before.NumGC, after.NumGC, want)
		}
	}
	if heap, err := hostfacts.ObserveCollectedGoHeap(nil); heap.Uint64() != 0 || err == nil {
		t.Fatalf("absent context = %v/%v, want refusal", heap, err)
	}
}
