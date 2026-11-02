package temporal

import (
	"errors"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
)

func TestInstantCeilingNativeOracleAtSignedBoundaries(t *testing.T) {
	t.Parallel()
	for _, precision := range []Precision{PrecisionNanosecond, PrecisionMicrosecond, PrecisionMillisecond, PrecisionSecond} {
		unit, err := precision.nanoseconds()
		if err != nil {
			t.Fatal(err)
		}
		for _, nanos := range []int64{math.MinInt64, math.MinInt64 + 1, math.MinInt64 + unit - 1, math.MinInt64 + unit, -unit - 1, -unit, -unit + 1, -1, 0, 1, unit - 1, unit, unit + 1, math.MaxInt64 - unit + 1, math.MaxInt64 - 1, math.MaxInt64} {
			t.Run(precision.String()+"_"+strconv.FormatInt(nanos, 10), func(t *testing.T) {
				t.Parallel()
				input := InstantFromNanoseconds(nanos)
				native := time.Unix(0, nanos).UTC()
				wantNative := native.Truncate(time.Duration(unit))
				if wantNative.Before(native) {
					wantNative = wantNative.Add(time.Duration(unit))
				}
				want, wantErr := NewInstant(wantNative)
				got, err := input.Ceiling(precision)
				if wantErr != nil {
					if !errors.Is(err, core.ErrTemporalOverflow) || got != (Instant{}) {
						t.Fatalf("Ceiling()=(%v,%v), want zero/native overflow", got, err)
					}
					return
				}
				if err != nil || got != want {
					t.Fatalf("Ceiling()=(%v,%v), want native %v", got, err, want)
				}
				if got.nanoseconds < nanos || got.nanoseconds%unit != 0 {
					t.Fatalf("Ceiling()=%d, want aligned and >=%d", got.nanoseconds, nanos)
				}
				again, err := got.Ceiling(precision)
				if err != nil || again != got {
					t.Fatalf("Ceiling(replay)=(%v,%v), want same value", again, err)
				}
			})
		}
	}
}

func TestInstantCeilingClosedPrecisionAndUnset(t *testing.T) {
	t.Parallel()
	for value := range 256 {
		p := Precision(value)
		if p.Validate() == nil {
			continue
		}
		got, err := InstantFromNanoseconds(1).Ceiling(p)
		if !errors.Is(err, core.ErrTemporalContract) || got != (Instant{}) {
			t.Fatalf("Ceiling(precision%d)=(%v,%v), want zero/contract", value, got, err)
		}
	}
	got, err := (Instant{}).Ceiling(PrecisionSecond)
	if !errors.Is(err, core.ErrTemporalContract) || got != (Instant{}) {
		t.Fatalf("Ceiling(unset)=(%v,%v), want zero/contract", got, err)
	}
}
