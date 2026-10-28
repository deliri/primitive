//go:build darwin

package hostfacts

import (
	"context"
	"errors"
	"github.com/deliri/primitive/v2026/temporal"
	"math"
	"slices"
	"syscall"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts/testdata/machrefs"
)

func TestMacResourceCPUCounterBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		before, after resourceMachCPUState
		want          uint64
		bad           bool
	}{
		{"half busy", resourceMachCPUState{1, 1, 2, 0}, resourceMachCPUState{3, 3, 6, 0}, 2000, false},
		{"all idle", resourceMachCPUState{0, 0, 1, 0}, resourceMachCPUState{0, 0, 11, 0}, 0, false},
		{"nice is busy", resourceMachCPUState{0, 0, 1, 0}, resourceMachCPUState{0, 0, 1, 10}, 4000, false},
		{"four maximum counters widen before sum", resourceMachCPUState{math.MaxUint32 - 1, math.MaxUint32 - 1, math.MaxUint32 - 1, math.MaxUint32 - 1}, resourceMachCPUState{math.MaxUint32, math.MaxUint32, math.MaxUint32, math.MaxUint32}, 3000, false},
		{"zero delta refuses", resourceMachCPUState{1, 1, 1, 1}, resourceMachCPUState{1, 1, 1, 1}, 0, true},
		{"individual regression despite growing total", resourceMachCPUState{100, 100, 100, 100}, resourceMachCPUState{99, 200, 200, 200}, 0, true},
		{"counter wrap despite growing total", resourceMachCPUState{math.MaxUint32, 0, 0, 0}, resourceMachCPUState{0, math.MaxUint32, math.MaxUint32, math.MaxUint32}, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resourceMachBusy(resourceMachCPUCounters{tc.before, {}, {}, {}}, resourceMachCPUCounters{tc.after, {}, {}, {}}, LogicalCPUCount{count: 4})
			if tc.bad {
				if !errors.Is(err, core.ErrHostFactsObservation) || got != 0 {
					t.Fatalf("native counter refusal=%d/%v", got, err)
				}
			} else if err != nil || got != tc.want {
				t.Fatalf("native counter=%d/%v, want%d", got, err, tc.want)
			}
		})
	}
	if err := resourceMachResult(0, syscall.EINTR); !errors.Is(err, syscall.EINTR) || !errors.Is(err, core.ErrHostFactsObservation) {
		t.Fatalf("native errno cause lost: %v", err)
	}
	if err := resourceMachResult(uintptr(4), 0); !errors.Is(err, core.ErrHostFactsObservation) {
		t.Fatalf("native kernel status lost: %v", err)
	} else if status, okay := errors.AsType[resourceMachStatus](err); !okay || status != 4 {
		t.Fatalf("native kernel status=%v/%v", status, err)
	}
}

func TestMacResourceNativeErrorReleasesHostRight(t *testing.T) {
	// Serial: compare the actual kernel refcount, including any preexisting
	// references, while this fixture keeps exactly its own host right alive.
	ref, err := machrefs.Hold()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ref.Close(); err != nil {
			t.Error(err)
		}
	}()
	before, err := ref.Count()
	if err != nil || before == 0 {
		t.Fatalf("native baseline refs=%d/%v", before, err)
	}
	got, err := readResourceMachCPU(t.Context(), uintptr(math.MaxUint32))
	if !errors.Is(err, core.ErrHostFactsObservation) || len(got) != 0 {
		t.Fatalf("actual invalid kernel flavor=%v/%v", got, err)
	}
	if status, okay := errors.AsType[resourceMachStatus](err); !okay || status == 0 {
		t.Fatalf("actual kernel cause missing: %v", err)
	}
	after, err := ref.Count()
	if err != nil || after != before {
		t.Fatalf("native error leaked host right: refs%d→%d/%v", before, after, err)
	}
	if _, err := readResourceCPU(t.Context()); err != nil {
		t.Fatal(err)
	}
	after, err = ref.Count()
	if err != nil || after != before {
		t.Fatalf("native successful read leaked host right: refs%d→%d/%v", before, after, err)
	}
}

// Every native allocation is tested by address, not an RSS or successful-call
// proxy. The returned Go snapshot remains valid after its OOL source is gone.
func TestMacResourceCPUBufferOwnership(t *testing.T) {
	ref, err := machrefs.Hold()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ref.Close(); err != nil {
			t.Error(err)
		}
	}()
	baseline, err := ref.Count()
	if err != nil {
		t.Fatal(err)
	}
	for _, cancelled := range []bool{false, true} {
		ctx, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: t.Context()})
		if err != nil {
			t.Fatal(err)
		}
		buffer, err := openResourceMachCPU(ctx, resourceMachProcessorCPULoadInfo)
		if err != nil {
			cancel(nil)
			t.Fatal(err)
		}
		address := uintptr(buffer.data)
		snapshot, copyErr := buffer.snapshot()
		copied := slices.Clone(snapshot)
		readable, readErr := machrefs.Readable(address)
		if cancelled {
			cancel(context.Canceled)
		}
		closeErr := buffer.close()
		cancel(nil)
		absent, absentErr := machrefs.Readable(address)
		if copyErr != nil || readErr != nil || !readable || closeErr != nil || absentErr != nil || absent || len(snapshot) == 0 || !slices.Equal(snapshot, copied) {
			t.Fatalf("native VM lifecycle cancelled=%t before=%t/%v copied=%d/%v close=%v afterReadable=%t/%v", cancelled, readable, readErr, len(snapshot), copyErr, closeErr, absent, absentErr)
		}
		if err := buffer.close(); err != nil {
			t.Fatalf("closed owner repeated cleanup: %v", err)
		}
		count, err := ref.Count()
		if err != nil || count != baseline {
			t.Fatalf("native host refs %d -> %d/%v", baseline, count, err)
		}
		t.Logf("native OOL region readable before/released after; copiedCPUCount=%d cancelled=%t hostRefs=%d", len(snapshot), cancelled, count)
	}
}

// Twelve ordinary quarter-second pairs exceed one cache window without a
// saturation burst or extra old probes. Both routes use this exact comparison;
// an old-route control earns causality only on identical nonzero native data.
func TestMacResourceCPURepeatedQuarterWindows(t *testing.T) {
	interval := resourceDuration(t, "250ms")
	cpus, err := ObserveLogicalCPUCount()
	if err != nil {
		t.Fatal(err)
	}
	for sample := 0; sample < 12; sample++ {
		start, err := temporal.Observe()
		if err != nil {
			t.Fatal(err)
		}
		before, err := readResourceCPU(t.Context())
		if err != nil {
			t.Fatalf("native before sample%d: %v", sample, err)
		}
		if err := temporal.Wait(temporal.WaitRequest{Context: t.Context(), Duration: interval}); err != nil {
			t.Fatal(err)
		}
		after, err := readResourceCPU(t.Context())
		if err != nil {
			t.Fatalf("native after sample%d: %v", sample, err)
		}
		end, err := temporal.Observe()
		if err != nil {
			t.Fatal(err)
		}
		window, err := end.Since(start)
		if err != nil {
			t.Fatal(err)
		}
		busy, err := resourceMachBusy(before, after, cpus)
		if err != nil {
			if len(before) == cpus.Int() && slices.Equal(before, after) {
				t.Fatalf("native cached zero-delta sample%d window_ns=%d before=%v after=%v: %v", sample, window.Nanoseconds(), before, after, err)
			}
			t.Fatalf("native different failure sample%d before=%v after=%v: %v", sample, before, after, err)
		}
		if window.Nanoseconds() < interval.Nanoseconds() || busy > uint64(cpus.Int())*resourceCPUMilli {
			t.Fatalf("native bounds sample%d window=%v busy=%d", sample, window, busy)
		}
		t.Logf("native quarter sample=%d window_ns=%d cpu_count=%d busy_milli=%d", sample, window.Nanoseconds(), cpus.Int(), busy)
	}
}

func TestMacResourcePerCPUClosure(t *testing.T) {
	t.Parallel()
	cpus := LogicalCPUCount{count: 2}
	for _, row := range []struct {
		name          string
		before, after resourceMachCPUCounters
	}{
		{"hidden individual regression", resourceMachCPUCounters{{100, 100, 100, 100}, {1, 1, 1, 1}}, resourceMachCPUCounters{{99, 100, 100, 100}, {1000, 1000, 1000, 1000}}},
		{"changed CPU count", resourceMachCPUCounters{{1, 1, 1, 1}, {1, 1, 1, 1}}, resourceMachCPUCounters{{2, 2, 2, 2}}},
		{"missing CPU", nil, nil},
	} {
		t.Run(row.name, func(t *testing.T) {
			if busy, err := resourceMachBusy(row.before, row.after, cpus); !errors.Is(err, core.ErrHostFactsObservation) || busy != 0 {
				t.Fatalf("per-CPU refusal=%d/%v", busy, err)
			}
		})
	}
}
