package textrepair

import (
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestPrefixIgnoredMalformedSuffixDoesNotAllocate(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	want := strings.Repeat("x", 32)
	request := Request{Source: want + strings.Repeat("\xff", 1<<20), MaximumBytes: prefixBudget(t, 32)}
	var got string
	var gotErr error
	allocations := testing.AllocsPerRun(100, func() { got, gotErr = Prefix(request) })
	if got != want || gotErr != nil {
		t.Fatalf("Prefix = %q/error %v, want %q/nil", got, gotErr, want)
	}
	if allocations != 0 {
		t.Fatalf("ignored-suffix allocations = %v, want 0", allocations)
	}
}

func TestPrefixMultiwidthEveryByteCeiling(t *testing.T) {
	t.Parallel()
	// Literal byte extents independently pin each complete rune boundary.
	const source = "aé€𝄞z"
	for budget, want := range []string{"", "a", "a", "aé", "aé", "aé", "aé€", "aé€", "aé€", "aé€", "aé€𝄞", source, source} {
		got, err := Prefix(Request{Source: source, MaximumBytes: prefixBudget(t, uint64(budget))})
		if err != nil || got != want {
			t.Fatalf("Prefix budget %d = %q/error %v, want %q/nil", budget, got, err, want)
		}
	}
}
