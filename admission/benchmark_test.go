package admission

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

var benchmarkObservationSink Observation

// BenchmarkGateWithSlot measures the synchronous per-operation admission path:
// one reservation, the caller's callback and the deferred release.
func BenchmarkGateWithSlot(b *testing.B) {
	gate := admissionFixture(b, 4)
	ctx := context.Background()
	calls := 0
	operation := func(_ context.Context, observation Observation) error {
		calls++
		benchmarkObservationSink = observation
		return nil
	}
	b.ReportAllocs()
	for b.Loop() {
		if err := gate.WithSlot(ctx, operation); err != nil {
			b.Fatal(err)
		}
	}
	if calls != b.N || benchmarkObservationSink.InUse() != 1 || gate.used.Load() != 0 {
		b.Fatalf("calls=%d inUse=%d retained=%d, want %d calls, one slot each and full release", calls, benchmarkObservationSink.InUse(), gate.used.Load(), b.N)
	}
}

// BenchmarkGateWithSlotCapacityRefusal measures the immediate refusal when
// every slot is already held; the callback must never run.
func BenchmarkGateWithSlotCapacityRefusal(b *testing.B) {
	gate := admissionFixture(b, 1)
	gate.used.Store(1)
	ctx := context.Background()
	calls := 0
	operation := func(context.Context, Observation) error {
		calls++
		return nil
	}
	var last error
	b.ReportAllocs()
	for b.Loop() {
		last = gate.WithSlot(ctx, operation)
	}
	if !errors.Is(last, core.ErrAdmissionCapacity) || calls != 0 || gate.used.Load() != 1 {
		b.Fatalf("refusal=%v calls=%d held=%d, want capacity refusal without callback", last, calls, gate.used.Load())
	}
}
