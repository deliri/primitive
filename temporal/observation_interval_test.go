package temporal

import (
	"errors"
	"math"
	"testing"
	"testing/synctest"
	"time"

	"github.com/deliri/primitive/v2026/core"
)

func TestObservationElapsedLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive exact observations preserve elapsed nanoseconds", func(t *testing.T) {
		t.Parallel()

		start, startErr := NewObservation(time.Unix(-1, 999_999_997).UTC())
		finish, finishErr := NewObservation(time.Unix(0, 2).UTC())
		got, gotErr := finish.Since(start)
		if startErr != nil || finishErr != nil || gotErr != nil || got.Nanoseconds() != 5 {
			t.Fatalf(
				"Observation.Since() = (%d, %v) after construction (%v, %v), want (5, nil)",
				got.Nanoseconds(),
				gotErr,
				startErr,
				finishErr,
			)
		}
	})

	t.Run("negative standard subtraction saturation is rejected", func(t *testing.T) {
		t.Parallel()

		start, startErr := NewObservation(time.Unix(0, math.MinInt64).UTC())
		finish, finishErr := NewObservation(time.Unix(0, math.MaxInt64).UTC())
		got, gotErr := finish.Since(start)
		if startErr != nil || finishErr != nil ||
			!errors.Is(gotErr, core.ErrTemporalOverflow) ||
			got != (Duration{}) {
			t.Fatalf(
				"Observation.Since(saturating span) = (%v, %v) after construction (%v, %v), want zero/%v",
				got,
				gotErr,
				startErr,
				finishErr,
				core.ErrTemporalOverflow,
			)
		}
	})

	t.Run("neutral same observation produces no elapsed time", func(t *testing.T) {
		t.Parallel()

		point, pointErr := NewObservation(time.Unix(0, 7).UTC())
		got, gotErr := point.Since(point)
		if pointErr != nil || gotErr != nil || !got.IsZero() {
			t.Fatalf("Observation.Since(same) = (%v, %v) after %v, want zero/nil", got, gotErr, pointErr)
		}
	})
}

func TestObservationUsesGoClockWithoutClockFramework(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		start, startErr := Observe()
		startWall, startWallErr := start.Instant()
		timer := time.NewTimer(17 * time.Nanosecond)
		<-timer.C
		finish, finishErr := Observe()
		got, gotErr := finish.Since(start)
		if startErr != nil || startWallErr != nil || finishErr != nil || gotErr != nil ||
			got.Nanoseconds() != 17 {
			t.Fatalf(
				"real Go clock observations = (%d, %v, %v, %v, %v), want (17, nil, nil, nil, nil)",
				got.Nanoseconds(),
				startErr,
				startWallErr,
				finishErr,
				gotErr,
			)
		}

		fiveNanoseconds, durationErr := DurationFromNanoseconds(5)
		correctedWall, wallErr := startWall.Add(fiveNanoseconds)
		corrected, correctedErr := finish.WithWall(correctedWall)
		correctedElapsed, elapsedErr := corrected.Since(start)
		projectedWall, projectedErr := corrected.Instant()
		if durationErr != nil || wallErr != nil || correctedErr != nil ||
			elapsedErr != nil || projectedErr != nil ||
			correctedElapsed.Nanoseconds() != 17 ||
			projectedWall != correctedWall {
			t.Fatalf(
				"wall-corrected observation = (wall:%v elapsed:%d errors:%v/%v/%v/%v/%v), want %v/17 and nil errors",
				projectedWall,
				correctedElapsed.Nanoseconds(),
				durationErr,
				wallErr,
				correctedErr,
				elapsedErr,
				projectedErr,
				correctedWall,
			)
		}

		rejected, rejectedErr := finish.WithWall(Instant{})
		if !errors.Is(rejectedErr, core.ErrTemporalContract) ||
			rejected != (Observation{}) {
			t.Fatalf(
				"Observation.WithWall(unset) = (%v, %v), want zero/%v",
				rejected,
				rejectedErr,
				core.ErrTemporalContract,
			)
		}
	})
}

func TestIntervalConstructionLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive observed interval derives a closed end", func(t *testing.T) {
		t.Parallel()

		start, _ := NewObservation(time.Unix(-1, 999_999_997).UTC())
		finish, _ := NewObservation(time.Unix(0, 2).UTC())
		got, gotErr := NewInterval(IntervalRequest{Start: start, Finish: finish})
		gotStart, gotStartErr := got.Start()
		gotEnd, gotEndErr := got.End()
		gotElapsed, gotElapsedErr := got.Elapsed()
		startNanoseconds, _ := gotStart.Nanoseconds()
		endNanoseconds, _ := gotEnd.Nanoseconds()
		if gotErr != nil || gotStartErr != nil || gotEndErr != nil ||
			gotElapsedErr != nil || startNanoseconds != -3 ||
			endNanoseconds != 2 || gotElapsed.Nanoseconds() != 5 {
			t.Fatalf(
				"NewInterval() = (start:%d end:%d elapsed:%d errors:%v/%v/%v/%v), want -3/2/5 and nil errors",
				startNanoseconds,
				endNanoseconds,
				gotElapsed.Nanoseconds(),
				gotErr,
				gotStartErr,
				gotEndErr,
				gotElapsedErr,
			)
		}
	})

	t.Run("negative reversed bounds are rejected", func(t *testing.T) {
		t.Parallel()

		got, gotErr := IntervalFromBounds(IntervalBounds{
			Start: InstantFromNanoseconds(1),
			End:   InstantFromNanoseconds(0),
		})
		if !errors.Is(gotErr, core.ErrTemporalContract) || got != (Interval{}) {
			t.Fatalf("IntervalFromBounds(reversed) = (%v, %v), want zero/%v", got, gotErr, core.ErrTemporalContract)
		}
	})

	t.Run("neutral point bounds produce a zero closed interval", func(t *testing.T) {
		t.Parallel()

		point := InstantFromNanoseconds(math.MinInt64)
		got, gotErr := IntervalFromBounds(IntervalBounds{Start: point, End: point})
		gotElapsed, gotElapsedErr := got.Elapsed()
		gotBounds, gotBoundsErr := got.Bounds()
		if gotErr != nil || gotElapsedErr != nil || gotBoundsErr != nil ||
			!gotElapsed.IsZero() || gotBounds.Start != point || gotBounds.End != point {
			t.Fatalf(
				"point IntervalFromBounds() = (elapsed:%v bounds:%v errors:%v/%v/%v), want zero and identical bounds",
				gotElapsed,
				gotBounds,
				gotErr,
				gotElapsedErr,
				gotBoundsErr,
			)
		}
	})
}

func TestIntervalRejectsEveryContradictoryOwnedFact(t *testing.T) {
	t.Parallel()

	start := InstantFromNanoseconds(10)
	end := InstantFromNanoseconds(20)
	ten, _ := DurationFromNanoseconds(10)
	nine, _ := DurationFromNanoseconds(9)
	eleven, _ := DurationFromNanoseconds(11)
	cases := []struct {
		wantErr  error
		name     string
		interval Interval
	}{
		{name: "zero interval has no set start", interval: Interval{}, wantErr: core.ErrTemporalContract},
		{name: "missing end is rejected", interval: Interval{start: start, elapsed: ten}, wantErr: core.ErrTemporalContract},
		{name: "missing start is rejected", interval: Interval{end: end, elapsed: ten}, wantErr: core.ErrTemporalContract},
		{name: "elapsed one below end difference is rejected", interval: Interval{start: start, end: end, elapsed: nine}, wantErr: core.ErrTemporalContract},
		{name: "elapsed one above end difference is rejected", interval: Interval{start: start, end: end, elapsed: eleven}, wantErr: core.ErrTemporalContract},
		{name: "end one below derived value is rejected", interval: Interval{start: start, end: InstantFromNanoseconds(19), elapsed: ten}, wantErr: core.ErrTemporalContract},
		{name: "end one above derived value is rejected", interval: Interval{start: start, end: InstantFromNanoseconds(21), elapsed: ten}, wantErr: core.ErrTemporalContract},
		{name: "coherent facts are admitted", interval: Interval{start: start, end: end, elapsed: ten}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.interval.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Interval.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func TestInternalDurationValidationBlocksCorruptArithmetic(t *testing.T) {
	t.Parallel()

	corrupt := Duration{nanoseconds: -1}
	valid, _ := DurationFromNanoseconds(1)
	cases := []struct {
		run  func() error
		name string
	}{
		{name: "validation rejects negative storage", run: corrupt.Validate},
		{name: "stdlib projection rejects negative storage", run: func() error { _, err := corrupt.Stdlib(); return err }},
		{name: "addition rejects corrupt receiver", run: func() error { _, err := corrupt.Add(valid); return err }},
		{name: "addition rejects corrupt operand", run: func() error { _, err := valid.Add(corrupt); return err }},
		{name: "subtraction rejects corrupt receiver", run: func() error { _, err := corrupt.Subtract(valid); return err }},
		{name: "comparison rejects corrupt operand", run: func() error { _, err := valid.Compare(corrupt); return err }},
		{name: "aggregate addition rejects corrupt duration", run: func() error {
			_, err := AggregateDuration{}.AddDuration(corrupt)
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if gotErr := tc.run(); !errors.Is(gotErr, core.ErrTemporalContract) {
				t.Fatalf("%s error = %v, want %v", tc.name, gotErr, core.ErrTemporalContract)
			}
		})
	}
}
