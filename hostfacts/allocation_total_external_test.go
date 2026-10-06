package hostfacts_test

import (
	"context"
	"errors"
	"runtime"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/temporal"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestGoAllocationTotalObservesCumulativeRuntimeBytes(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{
		Hazard: core.TestIsolationHazardRuntimeAllocation,
		Scope:  core.TestIsolationScopePackageProcess,
	})
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	first, err := hostfacts.ObserveGoAllocationTotal(t.Context())
	runtime.ReadMemStats(&after)
	if err != nil || first.Validate() != nil || first.Uint64() < before.TotalAlloc || first.Uint64() > after.TotalAlloc {
		t.Fatalf("total = %v/%v, runtime interval [%d,%d]", first, err, before.TotalAlloc, after.TotalAlloc)
	}
	// The adapter is the subject. Runtime statistics independently prove that
	// its result counts allocations even while caller-owned storage stays live.
	allocation := make([]byte, 2<<20)
	allocation[len(allocation)-1] = 1
	second, err := hostfacts.ObserveGoAllocationTotal(t.Context())
	runtime.KeepAlive(allocation)
	if err != nil || second.Uint64() < first.Uint64()+uint64(len(allocation)) {
		t.Fatalf("total %d -> %d/%v, want allocation of at least %d bytes", first.Uint64(), second.Uint64(), err, len(allocation))
	}
	// Reclaim the fixture. Total allocation must preserve its history even
	// though live heap drops; this distinguishes the two runtime counters.
	runtime.GC()
	third, err := hostfacts.ObserveGoAllocationTotal(t.Context())
	if err != nil || third.Uint64() < second.Uint64() {
		t.Fatalf("total after reclamation = %d/%v, want >= %d", third.Uint64(), err, second.Uint64())
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
		total, err := hostfacts.ObserveGoAllocationTotal(ctx)
		cancel()
		if total.Uint64() != 0 || !errors.Is(err, want) {
			t.Fatalf("terminal total = %v/%v, want %v", total, err, want)
		}
	}
	if total, err := hostfacts.ObserveGoAllocationTotal(nil); total.Uint64() != 0 || err == nil {
		t.Fatalf("absent context = %v/%v, want refusal", total, err)
	}
}
