package exchange

import (
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// FuzzParseDeclaredBodyLength proves the transport framing oracle: accepted
// declarations preserve their extent, absence carries none, and reservation
// never exceeds the caller's authorized bound.
func FuzzParseDeclaredBodyLength(f *testing.F) {
	for _, seed := range []int64{math.MinInt64, -2, -1, 0, 1, 4096, math.MaxInt64} {
		f.Add(seed)
	}
	limit, err := core.NewByteCount(512 * 1024)
	if err != nil {
		f.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
	allowed, err := limit.Uint64()
	if err != nil {
		f.Fatalf("ByteCount.Uint64() error = %v, want nil", err)
	}

	f.Fuzz(func(t *testing.T, value int64) {
		declared, parseErr := parseDeclaredBodyLength(value)
		if parseErr != nil {
			if declared != (declaredBodyLength{}) {
				t.Fatalf("parseDeclaredBodyLength(%d) refused with %+v, want zero", value, declared)
			}
			return
		}
		if gotErr := declared.Validate(); gotErr != nil {
			t.Fatalf("parseDeclaredBodyLength(%d).Validate() error = %v, want nil", value, gotErr)
		}
		if declared.present != (value >= 0) {
			t.Fatalf("parseDeclaredBodyLength(%d).present = %t, want %t", value, declared.present, value >= 0)
		}
		if declared.present && declared.length.Uint64() != uint64(value) {
			t.Fatalf("parseDeclaredBodyLength(%d).length = %d, want %d", value, declared.length.Uint64(), value)
		}
		if !declared.present && declared.length.Uint64() != 0 {
			t.Fatalf("absent parseDeclaredBodyLength(%d).length = %d, want 0", value, declared.length.Uint64())
		}
		reserved, gotErr := declared.reservedExtent(limit)
		if gotErr != nil {
			t.Fatalf("reservedExtent() error = %v, want nil", gotErr)
		}
		if reserved < 0 || uint64(reserved) > allowed {
			t.Fatalf("parseDeclaredBodyLength(%d).reservedExtent() = %d, want within [0, %d]", value, reserved, allowed)
		}
	})
}
