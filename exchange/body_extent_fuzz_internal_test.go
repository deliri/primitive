package exchange

import (
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// FuzzParseDeclaredBodyLength proves the transport framing oracle: accepted
// declarations preserve their extent and absence carries none.
func FuzzParseDeclaredBodyLength(f *testing.F) {
	for _, seed := range []int64{math.MinInt64, -2, -1, 0, 1, 4096, math.MaxInt64} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value int64) {
		declared, parseErr := parseDeclaredBodyLength(value)
		if value < declaredBodyLengthAbsent {
			if !errors.Is(parseErr, core.ErrExchangeContract) {
				t.Fatalf(
					"parseDeclaredBodyLength(%d) error = %v, want %v",
					value,
					parseErr,
					core.ErrExchangeContract,
				)
			}
			if declared != (declaredBodyLength{}) {
				t.Fatalf("parseDeclaredBodyLength(%d) refused with %+v, want zero", value, declared)
			}
			return
		}
		if parseErr != nil {
			t.Fatalf("parseDeclaredBodyLength(%d) error = %v, want nil", value, parseErr)
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

	})
}
