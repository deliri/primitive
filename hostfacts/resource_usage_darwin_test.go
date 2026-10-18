//go:build darwin

package hostfacts

import (
	"errors"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestMacResourceRecordsLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, text string
		want       uint64
		bad        bool
	}{
		{"last interval replaces since boot average", "CPU usage: 100.00% user, 0.00% sys, 0.00% idle\nCPU usage: 0.00% user, 0.00% sys, 100.00% idle\n", 0, false},
		{"half occupied capacity", "CPU usage: 30.00% user, 20.00% sys, 50.00% idle\n", 5000, false},
		{"fully occupied capacity", "CPU usage: 50.00% user, 50.00% sys, 0.00% idle\n", 10000, false},
		{"missing CPU record refuses", "unrelated\n", 0, true},
		{"missing percentage sign refuses", "CPU usage: 0.00% user, 0.00% sys, 100 idle\n", 0, true},
		{"negative idle refuses", "CPU usage: 0.00% user, 0.00% sys, -0.01% idle\n", 0, true},
		{"idle above 100 refuses", "CPU usage: 0.00% user, 0.00% sys, 100.01% idle\n", 0, true},
		{"NaN idle refuses", "CPU usage: 0.00% user, 0.00% sys, NaN% idle\n", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseResourceMacCPU(t.Context(), []byte(tc.text), LogicalCPUCount{count: 10})
			if tc.bad {
				if !errors.Is(err, core.ErrHostFactsObservation) || got != 0 {
					t.Fatalf("Mac CPU=(%d,%v), want zero refusal", got, err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("Mac CPU=(%d,%v), want %d", got, err, tc.want)
			}
		})
	}
	total, err := core.NewByteLength(4096)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, text string
		want       uint64
		bad        bool
	}{
		{"free inactive and speculative contribute once", "Mach Virtual Memory Statistics: (page size of 1024 bytes)\nPages free: 1.\nPages inactive: 2.\nPages speculative: 1.\n", 4096, false},
		{"zero available remains zero", "Mach Virtual Memory Statistics: (page size of 1024 bytes)\nPages free: 0.\nPages inactive: 0.\nPages speculative: 0.\n", 0, false},
		{"duplicate page header refuses", "Mach Virtual Memory Statistics: (page size of 1024 bytes)\nMach Virtual Memory Statistics: (page size of 1024 bytes)\nPages free: 0.\nPages inactive: 0.\nPages speculative: 0.\n", 0, true},
		{"duplicate count refuses", "Mach Virtual Memory Statistics: (page size of 1024 bytes)\nPages free: 0.\nPages free: 1.\nPages inactive: 0.\nPages speculative: 0.\n", 0, true},
		{"zero page size refuses", "Mach Virtual Memory Statistics: (page size of 0 bytes)\nPages free: 0.\nPages inactive: 0.\nPages speculative: 0.\n", 0, true},
		{"counts above physical memory refuse", "Mach Virtual Memory Statistics: (page size of 1024 bytes)\nPages free: 5.\nPages inactive: 0.\nPages speculative: 0.\n", 0, true},
		{"missing speculative is unavailable", "Mach Virtual Memory Statistics: (page size of 1024 bytes)\nPages free: 1.\nPages inactive: 0.\n", 0, true},
		{"page multiplication cannot wrap", "Mach Virtual Memory Statistics: (page size of 16384 bytes)\nPages free: 18446744073709551615.\nPages inactive: 0.\nPages speculative: 0.\n", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseResourceMacMemory(t.Context(), []byte(tc.text), total)
			if tc.bad {
				if !errors.Is(err, core.ErrHostFactsObservation) || got.Uint64() != 0 {
					t.Fatalf("Mac memory=(%v,%v), want zero refusal", got, err)
				}
				return
			}
			if err != nil || got.Uint64() != tc.want {
				t.Fatalf("Mac memory=(%d,%v), want %d", got.Uint64(), err, tc.want)
			}
		})
	}
}

func FuzzResourceMacCPUSemanticClosure(f *testing.F) {
	f.Add([]byte("CPU usage: 30.00% user, 20.00% sys, 50.00% idle\n"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := parseResourceMacCPU(t.Context(), data, LogicalCPUCount{count: 10})
		if err != nil {
			if !errors.Is(err, core.ErrHostFactsObservation) || got != 0 {
				t.Fatalf("refused CPU=(%d,%v)", got, err)
			}
			return
		}
		if got > 10000 {
			t.Fatalf("CPU=%d, want at most 10000", got)
		}
	})
}

func FuzzResourceMacMemorySemanticClosure(f *testing.F) {
	f.Add([]byte("Mach Virtual Memory Statistics: (page size of 1024 bytes)\nPages free: 1.\nPages inactive: 2.\nPages speculative: 1.\n"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		total, err := core.NewByteLength(64 << 30)
		if err != nil {
			t.Fatal(err)
		}
		got, err := parseResourceMacMemory(t.Context(), data, total)
		if err != nil {
			if !errors.Is(err, core.ErrHostFactsObservation) || got.Uint64() != 0 {
				t.Fatalf("refused memory=(%v,%v)", got, err)
			}
			return
		}
		if got.Uint64() > total.Uint64() {
			t.Fatalf("available exceeds physical memory")
		}
		canonical := []byte("Mach Virtual Memory Statistics: (page size of 1 bytes)\nPages free: " + strconv.FormatUint(got.Uint64(), 10) + ".\nPages inactive: 0.\nPages speculative: 0.\n")
		again, err := parseResourceMacMemory(t.Context(), canonical, total)
		if err != nil || again != got {
			t.Fatalf("memory round trip=(%v,%v), want %v", again, err, got)
		}
	})
}
