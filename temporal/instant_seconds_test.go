package temporal_test

import (
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestUnixSecondsInstantLayerTriad(t *testing.T) {
	t.Parallel()
	const scale = int64(temporal.NanosecondsPerSecond)
	for _, tc := range []struct {
		name    string
		seconds int64
		wantErr error
	}{
		{"minimum_exact_second", math.MinInt64 / scale, nil},
		{"second_below_minimum", math.MinInt64/scale - 1, core.ErrTemporalOverflow},
		{"before_epoch", -1, nil},
		{"epoch_is_set", 0, nil},
		{"after_epoch", 1, nil},
		{"maximum_exact_second", math.MaxInt64 / scale, nil},
		{"second_above_maximum", math.MaxInt64/scale + 1, core.ErrTemporalOverflow},
		{"native_minimum_cannot_wrap", math.MinInt64, core.ErrTemporalOverflow},
		{"native_maximum_cannot_wrap", math.MaxInt64, core.ErrTemporalOverflow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := temporal.InstantFromUnixSeconds(tc.seconds)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("instant error=%v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (temporal.Instant{}) {
					t.Fatalf("refused instant=%v, want zero", got)
				}
				return
			}
			want := temporal.InstantFromNanoseconds(tc.seconds * scale)
			if got != want || got.Validate() != nil {
				t.Fatalf("instant=%v, want %v", got, want)
			}
		})
	}
}
func FuzzUnixSecondsInstantSemanticClosure(f *testing.F) {
	f.Add(int64(0))
	f.Add(int64(-1))
	f.Add(int64(math.MinInt64))
	f.Add(int64(math.MaxInt64))
	f.Fuzz(func(t *testing.T, seconds int64) {
		const scale = int64(temporal.NanosecondsPerSecond)
		got, err := temporal.InstantFromUnixSeconds(seconds)
		valid := seconds >= math.MinInt64/scale && seconds <= math.MaxInt64/scale
		if !valid {
			if !errors.Is(err, core.ErrTemporalOverflow) || got != (temporal.Instant{}) {
				t.Fatalf("outside representation instant=%v error=%v, want zero overflow", got, err)
			}
			return
		}
		if err != nil || got != temporal.InstantFromNanoseconds(seconds*scale) || got.Validate() != nil {
			t.Fatalf("instant=%v error=%v, want exact second %d", got, err, seconds)
		}
	})
}
