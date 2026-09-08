package temporal

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
)

func TestIntervalRequestWallCorrectionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                                                       string
		start, finish, startWall, finishWall, wantEnd, wantElapsed int64
		wantErr                                                    error
	}{
		{name: "backward finish wall does not reverse elapsed carrier", start: 0, finish: 1, startWall: 10, finishWall: -10, wantEnd: 11, wantElapsed: 1},
		{name: "forward finish wall cannot invent elapsed time", start: 0, finish: 1, startWall: 10, finishWall: math.MaxInt64, wantEnd: 11, wantElapsed: 1},
		{name: "point at maximum remains representable", startWall: math.MaxInt64, finishWall: math.MinInt64, wantEnd: math.MaxInt64},
		{name: "derived end one below maximum is exact", finish: 1, startWall: math.MaxInt64 - 2, wantEnd: math.MaxInt64 - 1, wantElapsed: 1},
		{name: "derived end exactly maximum is exact", finish: 1, startWall: math.MaxInt64 - 1, wantEnd: math.MaxInt64, wantElapsed: 1},
		{name: "derived end one above maximum must fail validation", finish: 1, startWall: math.MaxInt64, wantErr: core.ErrTemporalOverflow},
		{name: "full elapsed domain fits after minimum wall", finish: math.MaxInt64, startWall: math.MinInt64, wantEnd: -1, wantElapsed: math.MaxInt64},
		{name: "large elapsed cannot hide corrected wall overflow", finish: math.MaxInt64, startWall: 1, wantErr: core.ErrTemporalOverflow},
		{name: "reversed carrier remains invalid despite ordered walls", start: 1, finish: 0, startWall: 0, finishWall: 1, wantErr: core.ErrTemporalContract},
		{name: "carrier overflow remains invalid despite equal walls", start: math.MinInt64, finish: 0, wantErr: core.ErrTemporalOverflow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			start, err := NewObservation(time.Unix(0, tc.start))
			if err != nil {
				t.Fatal(err)
			}
			finish, err := NewObservation(time.Unix(0, tc.finish))
			if err != nil {
				t.Fatal(err)
			}
			start, err = start.WithWall(InstantFromNanoseconds(tc.startWall))
			if err != nil {
				t.Fatal(err)
			}
			finish, err = finish.WithWall(InstantFromNanoseconds(tc.finishWall))
			if err != nil {
				t.Fatal(err)
			}
			request := IntervalRequest{Start: start, Finish: finish}
			validationErr := request.Validate()
			got, gotErr := NewInterval(request)
			if !errors.Is(validationErr, tc.wantErr) || !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("interval admission = (%v,%v), want both %v", validationErr, gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (Interval{}) {
					t.Fatalf("refused interval = %v, want zero", got)
				}
				return
			}
			bounds, boundsErr := got.Bounds()
			elapsed, elapsedErr := got.Elapsed()
			if boundsErr != nil || elapsedErr != nil || bounds.Start != InstantFromNanoseconds(tc.startWall) || bounds.End != InstantFromNanoseconds(tc.wantEnd) || elapsed.Nanoseconds() != tc.wantElapsed {
				t.Fatalf("interval facts = (%v,%v,%v,%v), want start %d end %d elapsed %d", bounds, elapsed, boundsErr, elapsedErr, tc.startWall, tc.wantEnd, tc.wantElapsed)
			}
		})
	}
}
