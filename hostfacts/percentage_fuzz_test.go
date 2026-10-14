package hostfacts

import (
	"errors"
	"math"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestPercentExhaustsWholeDomain(t *testing.T) {
	t.Parallel()
	for raw := range math.MaxUint8 + 1 {
		t.Run("whole percentage "+strconv.Itoa(raw), func(t *testing.T) {
			t.Parallel()
			got, err := NewPercent(uint8(raw))
			if raw == 0 || raw > percentDenominator {
				if got != (Percent{}) || !errors.Is(err, core.ErrHostFactsContract) {
					t.Fatalf("percentage %d = %v/%v, want zero typed refusal", raw, got, err)
				}
				return
			}
			value, valueErr := got.Uint8()
			if err != nil || valueErr != nil || int(value) != raw {
				t.Fatalf("percentage %d = %d/%v/%v, want exact value", raw, value, err, valueErr)
			}
		})
	}
}

func FuzzPercentProjectionSemanticClosure(f *testing.F) {
	f.Add(uint8(1), uint64(1))
	f.Add(uint8(percentDenominator), uint64(math.MaxUint64))
	f.Add(uint8(0), uint64(0))
	f.Add(uint8(100), uint64(0))
	f.Add(uint8(100), uint64(math.MaxInt64))
	f.Fuzz(func(t *testing.T, raw uint8, limit uint64) {
		percent, err := NewPercent(raw)
		if raw == 0 || raw > percentDenominator {
			if percent != (Percent{}) || !errors.Is(err, core.ErrHostFactsContract) {
				t.Fatalf("percentage = %v/%v, want zero typed refusal", percent, err)
			}
			return
		}
		if err != nil {
			t.Fatalf("admitted percentage error = %v, want nil", err)
		}
		count, countErr := core.NewByteLength(limit)
		if countErr != nil {
			if count != (core.ByteLength{}) || !errors.Is(countErr, core.ErrNumericOverflow) {
				t.Fatalf("unrepresentable native limit = %v/%v, want zero numeric refusal", count, countErr)
			}
			return
		}
		got, gotErr := (GoMemoryPressurePolicy{TriggerPercent: percent}).TriggerBytes(count)
		value, valueErr := got.Uint64(), got.Validate()
		want := ceilingPercentOracle(limit, raw)
		if gotErr != nil || valueErr != nil || value != want {
			t.Fatalf("percentage projection = %d/%v/%v, want independent integer ceiling %d", value, gotErr, valueErr, want)
		}
	})
}
