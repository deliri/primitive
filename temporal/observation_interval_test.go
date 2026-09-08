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
	for _, tc := range []struct {
		name                string
		start, finish, want int64
		wantErr             error
	}{
		{name: "point observation is neutral", start: 7, finish: 7},
		{name: "elapsed crosses epoch without rounding", start: -3, finish: 2, want: 5},
		{name: "maximum elapsed before epoch is exact", start: math.MinInt64, finish: -1, want: math.MaxInt64},
		{name: "maximum elapsed after epoch is exact", finish: math.MaxInt64, want: math.MaxInt64},
		{name: "one above maximum elapsed cannot saturate", start: math.MinInt64, wantErr: core.ErrTemporalOverflow},
		{name: "full signed span cannot saturate", start: math.MinInt64, finish: math.MaxInt64, wantErr: core.ErrTemporalOverflow},
		{name: "reversed observations cannot become positive elapsed", start: 1, wantErr: core.ErrTemporalContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			start, startErr := NewObservation(time.Unix(0, tc.start))
			finish, finishErr := NewObservation(time.Unix(0, tc.finish))
			if startErr != nil || finishErr != nil {
				t.Fatalf("observation fixtures = (%v,%v), want nil", startErr, finishErr)
			}
			got, gotErr := finish.Since(start)
			if !errors.Is(gotErr, tc.wantErr) || got.Nanoseconds() != tc.want {
				t.Fatalf("elapsed = (%d,%v), want (%d,%v)", got.Nanoseconds(), gotErr, tc.want, tc.wantErr)
			}
		})
	}
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

func TestObservationRejectsEveryUnusableCarrierBeforeProjection(t *testing.T) {
	t.Parallel()

	validValue := time.Unix(0, 7).UTC()
	validWall := InstantFromNanoseconds(7)
	cases := []struct {
		wantErr     error
		name        string
		observation Observation
	}{
		{name: "zero observation has neither carrier nor wall", wantErr: core.ErrTemporalContract},
		{name: "wall without a representable Go carrier is rejected", observation: Observation{wall: validWall}, wantErr: core.ErrTemporalOverflow},
		{name: "Go carrier without a set wall is rejected", observation: Observation{value: validValue}, wantErr: core.ErrTemporalContract},
		{name: "minimum signed carrier and wall are admitted", observation: Observation{value: time.Unix(0, math.MinInt64).UTC(), wall: InstantFromNanoseconds(math.MinInt64)}},
		{name: "epoch carrier and wall are admitted", observation: Observation{value: time.Unix(0, 0).UTC(), wall: InstantFromNanoseconds(0)}},
		{name: "maximum signed carrier and wall are admitted", observation: Observation{value: time.Unix(0, math.MaxInt64).UTC(), wall: InstantFromNanoseconds(math.MaxInt64)}},
		{name: "corrected wall may differ from the elapsed carrier", observation: Observation{value: validValue, wall: InstantFromNanoseconds(-11)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.observation.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Observation.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
			gotInstant, gotInstantErr := tc.observation.Instant()
			if tc.wantErr != nil {
				if !errors.Is(gotInstantErr, tc.wantErr) || gotInstant.IsSet() {
					t.Fatalf("Observation.Instant() = (%v, %v), want unset/%v", gotInstant, gotInstantErr, tc.wantErr)
				}
				return
			}
			if gotInstantErr != nil || gotInstant != tc.observation.wall {
				t.Fatalf("Observation.Instant() = (%v, %v), want (%v, nil)", gotInstant, gotInstantErr, tc.observation.wall)
			}
		})
	}

	outsideLow, lowErr := NewObservation(time.Unix(0, math.MinInt64).Add(-time.Nanosecond))
	outsideHigh, highErr := NewObservation(time.Unix(0, math.MaxInt64).Add(time.Nanosecond))
	if !errors.Is(lowErr, core.ErrTemporalOverflow) || outsideLow != (Observation{}) ||
		!errors.Is(highErr, core.ErrTemporalOverflow) || outsideHigh != (Observation{}) {
		t.Fatalf(
			"NewObservation(outside signed range) = (low:%v/%v high:%v/%v), want zero/%v for both",
			outsideLow,
			lowErr,
			outsideHigh,
			highErr,
			core.ErrTemporalOverflow,
		)
	}
}

func TestIntervalBoundsConstructionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name             string
		start, end, want int64
		wantErr          error
	}{
		{name: "minimum point is neutral", start: math.MinInt64, end: math.MinInt64},
		{name: "maximum point is neutral", start: math.MaxInt64, end: math.MaxInt64},
		{name: "epoch crossing preserves exact endpoints", start: -3, end: 2, want: 5},
		{name: "maximum elapsed ending before epoch", start: math.MinInt64, end: -1, want: math.MaxInt64},
		{name: "maximum elapsed starting at epoch", end: math.MaxInt64, want: math.MaxInt64},
		{name: "one beyond elapsed domain is refused", start: math.MinInt64, wantErr: core.ErrTemporalOverflow},
		{name: "reversed endpoints are refused", start: 1, wantErr: core.ErrTemporalContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			bounds := IntervalBounds{Start: InstantFromNanoseconds(tc.start), End: InstantFromNanoseconds(tc.end)}
			got, gotErr := IntervalFromBounds(bounds)
			if !errors.Is(gotErr, tc.wantErr) || !errors.Is(bounds.Validate(), tc.wantErr) {
				t.Fatalf("bounds admission = (%v,%v), want %v", gotErr, bounds.Validate(), tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (Interval{}) {
					t.Fatalf("refused interval = %v, want zero", got)
				}
				return
			}
			start, startErr := got.Start()
			end, endErr := got.End()
			elapsed, elapsedErr := got.Elapsed()
			projected, projectErr := got.Bounds()
			if startErr != nil || endErr != nil || elapsedErr != nil || projectErr != nil || start != bounds.Start || end != bounds.End || elapsed.Nanoseconds() != tc.want || projected != bounds {
				t.Fatalf("interval facts = (%v,%v,%v,%v,%v,%v,%v,%v), want exact %v and elapsed %d", start, end, elapsed, projected, startErr, endErr, elapsedErr, projectErr, bounds, tc.want)
			}
		})
	}
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

func TestIntervalIngressExhaustsMissingOrderingAndOverflowBoundaries(t *testing.T) {
	t.Parallel()

	minimumObservation, _ := NewObservation(time.Unix(0, math.MinInt64).UTC())
	epochObservation, _ := NewObservation(time.Unix(0, 0).UTC())
	maximumObservation, _ := NewObservation(time.Unix(0, math.MaxInt64).UTC())
	minimum := InstantFromNanoseconds(math.MinInt64)
	epoch := InstantFromNanoseconds(0)
	maximum := InstantFromNanoseconds(math.MaxInt64)

	cases := []struct {
		validate func() error
		wantErr  error
		name     string
	}{
		{name: "zero observation request rejects missing start", validate: (IntervalRequest{}).Validate, wantErr: core.ErrTemporalContract},
		{name: "observation request rejects missing start", validate: (IntervalRequest{Finish: epochObservation}).Validate, wantErr: core.ErrTemporalContract},
		{name: "observation request rejects missing finish", validate: (IntervalRequest{Start: epochObservation}).Validate, wantErr: core.ErrTemporalContract},
		{name: "observation request rejects reversed order", validate: (IntervalRequest{Start: maximumObservation, Finish: epochObservation}).Validate, wantErr: core.ErrTemporalContract},
		{name: "observation request rejects elapsed overflow", validate: (IntervalRequest{Start: minimumObservation, Finish: maximumObservation}).Validate, wantErr: core.ErrTemporalOverflow},
		{name: "observation request admits a point", validate: (IntervalRequest{Start: epochObservation, Finish: epochObservation}).Validate},
		{name: "observation request admits maximum bounded elapsed", validate: (IntervalRequest{Start: minimumObservation, Finish: requireObservation(t, time.Unix(0, -1).UTC())}).Validate},
		{name: "zero bounds reject missing start", validate: (IntervalBounds{}).Validate, wantErr: core.ErrTemporalContract},
		{name: "bounds reject missing start", validate: (IntervalBounds{End: epoch}).Validate, wantErr: core.ErrTemporalContract},
		{name: "bounds reject missing end", validate: (IntervalBounds{Start: epoch}).Validate, wantErr: core.ErrTemporalContract},
		{name: "bounds reject reversed order", validate: (IntervalBounds{Start: maximum, End: epoch}).Validate, wantErr: core.ErrTemporalContract},
		{name: "bounds reject elapsed overflow", validate: (IntervalBounds{Start: minimum, End: maximum}).Validate, wantErr: core.ErrTemporalOverflow},
		{name: "bounds admit a point at signed minimum", validate: (IntervalBounds{Start: minimum, End: minimum}).Validate},
		{name: "bounds admit a point at signed maximum", validate: (IntervalBounds{Start: maximum, End: maximum}).Validate},
		{name: "bounds admit maximum bounded elapsed", validate: (IntervalBounds{Start: minimum, End: InstantFromNanoseconds(-1)}).Validate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("interval ingress Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func requireObservation(t *testing.T, value time.Time) Observation {
	t.Helper()
	got, gotErr := NewObservation(value)
	if gotErr != nil {
		t.Fatalf("NewObservation(%v) error = %v, want nil", value, gotErr)
	}
	return got
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
		{name: "subtraction rejects corrupt operand", run: func() error { _, err := valid.Subtract(corrupt); return err }},
		{name: "zero multiplication cannot hide corrupt receiver", run: func() error { _, err := corrupt.Multiply(0); return err }},
		{name: "comparison rejects corrupt receiver", run: func() error { _, err := corrupt.Compare(valid); return err }},
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
