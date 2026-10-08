package hostfacts

import (
	"errors"
	"math"
	"runtime/debug"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/testserial"
)

func FuzzGoMemoryLimitNativeSemanticClosure(f *testing.F) {
	for _, value := range []uint64{0, 1, 1 << 30, math.MaxInt64, uint64(math.MaxInt64) + 1, math.MaxUint64} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value uint64) {
		testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
		limit, constructErr := core.NewByteLength(value)
		if constructErr != nil {
			if value <= math.MaxInt64 || !errors.Is(constructErr, core.ErrNumericOverflow) || limit != (core.ByteLength{}) {
				t.Fatalf("unrepresentable native limit %d = (%v, %v), want zero length and typed overflow", value, limit, constructErr)
			}
			return
		}
		before := debug.SetMemoryLimit(-1)
		got, err := ApplyGoMemoryLimit(t.Context(), GoMemoryLimitRequest{Limit: limit})
		actual := debug.SetMemoryLimit(before) // Restore before any diagnostic allocation.
		previous := got.Previous.Uint64()
		applied := got.Applied.Uint64()
		if err != nil || got.Validate() != nil || previous != uint64(before) || applied != value || actual != int64(value) {
			t.Fatalf("admitted limit %d = (%v, %v), native:%d previous:%d, want exact application and prior limit", value, got, err, actual, before)
		}
	})
}
