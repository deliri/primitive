package permit

import (
	"errors"
	"math"
	"math/big"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// The oracle uses arbitrary precision elapsed time and division, independently
// of the production implementation's reduced signed phases.
func FuzzReportScheduleSignedEpochArithmetic(f *testing.F) {
	for _, seed := range []struct {
		origin int64
		now    int64
		period uint64
		window uint64
	}{
		{origin: math.MinInt64, now: math.MaxInt64 - 1, period: 100, window: 9},
		{origin: math.MinInt64, now: 0, period: math.MaxInt64, window: math.MaxInt64 - 2},
		{origin: -101, now: -1, period: 100, window: 9},
		{origin: -101, now: 0, period: 100, window: 9},
		{origin: -101, now: 9, period: 100, window: 9},
		{origin: 100, now: 99, period: 100, window: 9},
		{origin: math.MaxInt64 - 100, now: math.MaxInt64 - 1, period: 100, window: 9},
	} {
		f.Add(seed.origin, seed.now, seed.period, seed.window)
	}
	f.Fuzz(func(t *testing.T, origin, now int64, periodInput, windowInput uint64) {
		period := max(int64(periodInput&math.MaxInt64), 2)
		window := int64(windowInput%uint64(period-1)) + 1
		origin = min(origin, math.MaxInt64-window)
		schedule := reportSchedule(t)
		schedule.NextReportAt = temporal.InstantFromNanoseconds(origin)
		schedule.RepeatInterval = reportDuration(t, period)
		schedule.WindowDuration = reportDuration(t, window)
		schedule.JitterMaximum = reportDuration(t, window-1)
		timing := ReportTiming{
			ObservedAt: temporal.InstantFromNanoseconds(now),
			NotBefore:  temporal.InstantFromNanoseconds(math.MinInt64),
			ExpiresAt:  temporal.InstantFromNanoseconds(math.MaxInt64),
		}
		want, wantErr := scheduleArithmeticOracle(origin, now, period, window)
		got, err := schedule.Occurrence(timing)
		if got != want || !errors.Is(err, wantErr) {
			t.Fatalf("origin=%d now=%d period=%d window=%d occurrence=%+v/%v, want %+v/%v", origin, now, period, window, got, err, want, wantErr)
		}
		if err != nil {
			return
		}
		open, openErr := want.Open.Nanoseconds()
		closeAt, closeErr := want.Close.Nanoseconds()
		if openErr != nil || closeErr != nil {
			t.Fatalf("oracle interval errors = %v/%v, want nil/nil", openErr, closeErr)
		}
		start := max(open, now)
		desired := new(big.Int).Add(big.NewInt(start), big.NewInt(window-1))
		last := big.NewInt(closeAt - 1)
		if desired.Cmp(last) > 0 {
			desired.Set(last)
		}
		send, sendErr := schedule.SendAt(timing, schedule.JitterMaximum)
		if sendErr != nil || send != temporal.InstantFromNanoseconds(desired.Int64()) {
			t.Fatalf("send at origin=%d now=%d period=%d window=%d = %v/%v, want %s/nil", origin, now, period, window, send, sendErr, desired)
		}
	})
}

func scheduleArithmeticOracle(origin, now, period, window int64) (ReportOccurrence, error) {
	open := big.NewInt(origin)
	if now >= origin {
		distance := new(big.Int).Sub(big.NewInt(now), open)
		quotient, remainder := new(big.Int), new(big.Int)
		quotient.QuoRem(distance, big.NewInt(period), remainder)
		open.Add(open, quotient.Mul(quotient, big.NewInt(period)))
		if remainder.Cmp(big.NewInt(window)) >= 0 {
			open.Add(open, big.NewInt(period))
		}
	}
	closeAt := new(big.Int).Add(new(big.Int).Set(open), big.NewInt(window))
	if !open.IsInt64() || !closeAt.IsInt64() {
		return ReportOccurrence{}, core.ErrReportOverflow
	}
	if now == math.MaxInt64 {
		return ReportOccurrence{}, core.ErrPermitValidity
	}
	return ReportOccurrence{Open: temporal.InstantFromNanoseconds(open.Int64()), Close: temporal.InstantFromNanoseconds(closeAt.Int64())}, nil
}
