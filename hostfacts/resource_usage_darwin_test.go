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

// Anonymous pages are process memory: one page size, one count, never more
// than physical memory and never wrapped by the page multiplication.
func TestMacResourceProcessMemoryBoundaries(t *testing.T) {
	t.Parallel()
	total, err := core.NewByteLength(4096)
	if err != nil {
		t.Fatal(err)
	}
	const header = "Mach Virtual Memory Statistics: (page size of 1024 bytes)\n"
	for _, tc := range []struct {
		name, text string
		want       uint64
		bad        bool
	}{
		{"anonymous pages times page size", header + "Pages free: 1.\nAnonymous pages: 3.\n", 3072, false},
		{"all physical memory held by processes", header + "Anonymous pages: 4.\n", 4096, false},
		{"no process memory is zero", header + "Anonymous pages: 0.\n", 0, false},
		{"one page above physical memory refuses", header + "Anonymous pages: 5.\n", 0, true},
		{"missing anonymous count refuses", header + "Pages free: 1.\n", 0, true},
		{"missing page size refuses", "Anonymous pages: 1.\n", 0, true},
		{"duplicate anonymous count refuses", header + "Anonymous pages: 1.\nAnonymous pages: 1.\n", 0, true},
		{"duplicate page header refuses", header + header + "Anonymous pages: 1.\n", 0, true},
		{"zero page size refuses", "Mach Virtual Memory Statistics: (page size of 0 bytes)\nAnonymous pages: 0.\n", 0, true},
		{"negative count refuses", header + "Anonymous pages: -1.\n", 0, true},
		{"page multiplication cannot wrap", "Mach Virtual Memory Statistics: (page size of 16384 bytes)\nAnonymous pages: 18446744073709551615.\n", 0, true},
		{"similar key is not the count", header + "Anonymous pages purgeable: 1.\nAnonymous pages: 2.\n", 2048, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseResourceMacProcess(t.Context(), []byte(tc.text), total)
			if tc.bad {
				if !errors.Is(err, core.ErrHostFactsObservation) || got.Uint64() != 0 {
					t.Fatalf("Mac process memory=(%v,%v), want zero refusal", got, err)
				}
				return
			}
			if err != nil || got.Uint64() != tc.want {
				t.Fatalf("Mac process memory=(%d,%v), want %d", got.Uint64(), err, tc.want)
			}
		})
	}
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

// macOS has no cgroup CPU counters, so a scope request is refused rather than
// answered with a sample that could not be interval-matched.
func TestResourceUsageRefusesScopesOnDarwin(t *testing.T) {
	t.Parallel()
	name, err := core.ParsePathComponent("primitive-darwin.scope")
	if err != nil {
		t.Fatal(err)
	}
	got, err := ObserveResourceUsage(t.Context(), ResourceUsageRequest{Interval: resourceDuration(t, "1s"), Scopes: []SystemdScopeRequest{{Unit: name, UserManager: true}}})
	if !errors.Is(err, core.ErrHostFactsUnsupported) || !emptyResourceUsage(got) {
		t.Fatalf("darwin scopes=(%+v,%v), want unsupported and no sample", got, err)
	}
}
