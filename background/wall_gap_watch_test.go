package background

import (
	"context"
	"errors"
	"math"
	"testing"
	"testing/synctest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestWallGapWatchCloseJoinsItsNativeTaskLoan(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		release, entered := make(chan struct{}), make(chan struct{})
		task, err := StartTask(TaskRequest{Parent: t.Context(), Work: Work{Run: func(context.Context) error { close(entered); <-release; return nil }, Complete: func(error) {}}})
		if err != nil {
			t.Fatal(err)
		}
		watch := &WallGapWatch{task: task}
		watch.self = watch
		<-entered
		closed := make(chan error, 1)
		go func() { _, err := watch.Close(); closed <- err }()
		synctest.Wait()
		premature := false
		select {
		case <-closed:
			premature = true
		default:
		}
		close(release)
		synctest.Wait()
		if premature {
			t.Fatal("Close returned while its native task loan remained active")
		}
		if err := <-closed; err != nil {
			t.Fatalf("Close() = %v, want joined native task", err)
		}
	})
}

func TestWallGapObservationsKeepExactMaximumAndCorrectionBaseline(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                        string
		start                       int64
		steps                       []int64
		wantStart, wantEnd, wantGap int64
		wantErr                     error
	}{
		{name: "positive observations retain largest interval", start: 1, steps: []int64{5, 11, 13}, wantStart: 5, wantEnd: 11, wantGap: 6},
		{name: "equal gap keeps original coordinate", start: 1, steps: []int64{5, 9}, wantStart: 1, wantEnd: 5, wantGap: 4},
		{name: "backwards correction becomes next baseline", start: 10, steps: []int64{5, 8}, wantStart: 5, wantEnd: 8, wantGap: 3},
		{name: "equal observations remain absent", start: 5, steps: []int64{5}},
		{name: "signed bounds preserve exact representable difference", start: math.MinInt64, steps: []int64{-1}, wantStart: math.MinInt64, wantEnd: -1, wantGap: math.MaxInt64},
		{name: "unrepresentable difference refuses without partial observation", start: math.MinInt64, steps: []int64{0}, wantErr: core.ErrTemporalOverflow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			watch := WallGapWatch{previous: temporal.InstantFromNanoseconds(tc.start)}
			var gotErr error
			for _, value := range tc.steps {
				gotErr = watch.observe(temporal.InstantFromNanoseconds(value))
				if gotErr != nil {
					break
				}
			}
			if !errors.Is(gotErr, tc.wantErr) || watch.snapshot.Validate() != nil {
				t.Fatalf("observe() = %+v / %v, want valid snapshot and %v", watch.snapshot, gotErr, tc.wantErr)
			}
			if tc.wantGap == 0 {
				if watch.snapshot.Observed {
					t.Fatalf("snapshot = %+v, want absent observation", watch.snapshot)
				}
				return
			}
			got := watch.snapshot.Maximum
			gotStart, _ := got.Start()
			gotEnd, _ := got.End()
			elapsed, _ := got.Elapsed()
			if !watch.snapshot.Observed || gotStart != temporal.InstantFromNanoseconds(tc.wantStart) || gotEnd != temporal.InstantFromNanoseconds(tc.wantEnd) || elapsed.Nanoseconds() != tc.wantGap {
				t.Fatalf("snapshot = %+v, want exact [%d,%d] / %d", watch.snapshot, tc.wantStart, tc.wantEnd, tc.wantGap)
			}
		})
	}
}
