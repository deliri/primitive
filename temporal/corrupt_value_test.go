package temporal

import (
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestCorruptDurationCannotEscapeThroughEffectsOrJSON(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		value Duration
	}{
		{name: "one below nonnegative storage", value: Duration{nanoseconds: -1}},
		{name: "signed minimum cannot wrap to elapsed time", value: Duration{nanoseconds: math.MinInt64}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, cancel, timeoutErr := WithTimeout(TimeoutRequest{Parent: t.Context(), Duration: tc.value})
			if cancel != nil {
				cancel()
			}
			ticker, tickerErr := OpenTicker(TickerRequest{Interval: tc.value})
			if ticker != nil {
				ticker.Stop()
			}
			waitErr := Wait(WaitRequest{Context: t.Context(), Duration: tc.value})
			wire, wireErr := tc.value.MarshalJSON()
			projection, projectionErr := NewNumericDuration(tc.value)
			for _, result := range []struct {
				name   string
				gotErr error
			}{
				{"timeout", timeoutErr}, {"ticker", tickerErr}, {"wait", waitErr}, {"JSON", wireErr}, {"numeric construction", projectionErr},
			} {
				if !errors.Is(result.gotErr, core.ErrTemporalContract) {
					t.Fatalf("%s error = %v, want %v", result.name, result.gotErr, core.ErrTemporalContract)
				}
			}
			if got != nil || cancel != nil || ticker != nil || wire != nil || projection != (NumericDuration{}) {
				t.Fatalf("corrupt duration emitted (%v,%v,%v,%q,%v), want no capability, bytes or numeric value", got, cancel, ticker, wire, projection)
			}
			numeric := NumericDuration{value: tc.value}
			numericWire, numericErr := numeric.MarshalJSON()
			if numericWire != nil || !errors.Is(numericErr, core.ErrTemporalContract) {
				t.Fatalf("corrupt numeric JSON = (%q,%v), want nil typed refusal", numericWire, numericErr)
			}
		})
	}
}

func TestInvalidIntervalCannotProjectPartialFacts(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		value   Interval
		wantErr error
	}{
		{name: "missing start", value: Interval{end: InstantFromNanoseconds(0)}, wantErr: core.ErrTemporalContract},
		{name: "missing end", value: Interval{start: InstantFromNanoseconds(0)}, wantErr: core.ErrTemporalContract},
		{name: "negative elapsed", value: Interval{start: InstantFromNanoseconds(0), end: InstantFromNanoseconds(0), elapsed: Duration{nanoseconds: -1}}, wantErr: core.ErrTemporalContract},
		{name: "derived end overflow", value: Interval{start: InstantFromNanoseconds(math.MaxInt64), end: InstantFromNanoseconds(0), elapsed: Duration{nanoseconds: 1}}, wantErr: core.ErrTemporalOverflow},
		{name: "end contradicts valid elapsed", value: Interval{start: InstantFromNanoseconds(0), end: InstantFromNanoseconds(1)}, wantErr: core.ErrTemporalContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			start, startErr := tc.value.Start()
			end, endErr := tc.value.End()
			elapsed, elapsedErr := tc.value.Elapsed()
			bounds, boundsErr := tc.value.Bounds()
			if !errors.Is(tc.value.Validate(), tc.wantErr) || !errors.Is(startErr, tc.wantErr) || !errors.Is(endErr, tc.wantErr) || !errors.Is(elapsedErr, tc.wantErr) || !errors.Is(boundsErr, tc.wantErr) {
				t.Fatalf("invalid interval errors = (%v,%v,%v,%v), want %v", startErr, endErr, elapsedErr, boundsErr, tc.wantErr)
			}
			if start != (Instant{}) || end != (Instant{}) || elapsed != (Duration{}) || bounds != (IntervalBounds{}) {
				t.Fatalf("invalid interval facts = (%v,%v,%v,%v), want all zero", start, end, elapsed, bounds)
			}
		})
	}
}
