//go:build darwin

package hostfacts

import (
	"errors"
	"math"
	"syscall"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts/testdata/machrefs"
)

func TestMacResourceCPUCounterBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		before, after resourceMachCPUCounters
		want          uint64
		bad           bool
	}{
		{"half busy", resourceMachCPUCounters{1, 1, 2, 0}, resourceMachCPUCounters{3, 3, 6, 0}, 2000, false},
		{"all idle", resourceMachCPUCounters{0, 0, 1, 0}, resourceMachCPUCounters{0, 0, 11, 0}, 0, false},
		{"nice is busy", resourceMachCPUCounters{0, 0, 1, 0}, resourceMachCPUCounters{0, 0, 1, 10}, 4000, false},
		{"four maximum counters widen before sum", resourceMachCPUCounters{math.MaxUint32 - 1, math.MaxUint32 - 1, math.MaxUint32 - 1, math.MaxUint32 - 1}, resourceMachCPUCounters{math.MaxUint32, math.MaxUint32, math.MaxUint32, math.MaxUint32}, 3000, false},
		{"zero delta refuses", resourceMachCPUCounters{1, 1, 1, 1}, resourceMachCPUCounters{1, 1, 1, 1}, 0, true},
		{"individual regression despite growing total", resourceMachCPUCounters{100, 100, 100, 100}, resourceMachCPUCounters{99, 200, 200, 200}, 0, true},
		{"counter wrap despite growing total", resourceMachCPUCounters{math.MaxUint32, 0, 0, 0}, resourceMachCPUCounters{0, math.MaxUint32, math.MaxUint32, math.MaxUint32}, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resourceMachBusy(tc.before, tc.after, LogicalCPUCount{count: 4})
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
	if !errors.Is(err, core.ErrHostFactsObservation) || got != (resourceMachCPUCounters{}) {
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
