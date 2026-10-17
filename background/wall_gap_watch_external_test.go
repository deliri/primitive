package background_test

import (
	"context"
	"errors"
	"io"
	"math"
	"reflect"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/deliri/primitive/v2026/background"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestWallGapWatchWithdrawsSnapshotAfterNativeSamplingRefusal(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		interval, err := temporal.DurationFromNanoseconds(int64(time.Millisecond))
		if err != nil {
			t.Fatal(err)
		}
		watch, err := background.OpenWallGapWatch(background.WallGapWatchRequest{Parent: t.Context(), Start: temporal.InstantFromNanoseconds(math.MinInt64), Interval: interval})
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _, _ = watch.Close() }()
		time.Sleep(time.Millisecond)
		synctest.Wait()
		got, err := watch.Snapshot()
		if got != (background.WallGapSnapshot{}) || !errors.Is(err, core.ErrTemporalOverflow) {
			t.Fatalf("failed native sample = %+v / %v, want withdrawn observation and native cause", got, err)
		}
		result, closeErr := watch.Close()
		if closeErr != nil || !errors.Is(result.ActionError, err) {
			t.Fatalf("native join = %+v / %v, want conserved sampling cause %v", result, closeErr, err)
		}
	})
}

func TestWallGapWatchRefusesUnsetAndCopiedCapabilities(t *testing.T) {
	t.Parallel()
	interval, err := temporal.DurationFromNanoseconds(int64(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	watch, err := background.OpenWallGapWatch(background.WallGapWatchRequest{Parent: t.Context(), Start: temporal.InstantFromNanoseconds(1), Interval: interval})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = watch.Close() }()
	// Deliberately make an inadmissible shallow copy without copying a lock in
	// executable caller code. Every entry point must refuse before using it.
	copyValue := reflect.New(reflect.TypeFor[background.WallGapWatch]())
	copyValue.Elem().Set(reflect.ValueOf(watch).Elem())
	copied := copyValue.Interface().(*background.WallGapWatch)
	for _, candidate := range []*background.WallGapWatch{nil, {}, copied} {
		if err := candidate.Validate(); !errors.Is(err, core.ErrBackgroundContract) {
			t.Fatalf("Validate() = %v, want native capability refusal", err)
		}
		if snapshot, err := candidate.Snapshot(); !errors.Is(err, core.ErrBackgroundContract) || snapshot.Observed {
			t.Fatalf("Snapshot() = %+v / %v, want no observation and native refusal", snapshot, err)
		}
		if _, err := candidate.Close(); !errors.Is(err, core.ErrBackgroundContract) {
			t.Fatalf("Close() = %v, want native capability refusal", err)
		}
	}
	if err := watch.Validate(); err != nil {
		t.Fatalf("rejected copies changed original custody: %v", err)
	}
}

func TestWallGapWatchOwnsTickerSnapshotsAndConcurrentClose(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		observation, err := temporal.Observe()
		if err != nil {
			t.Fatal(err)
		}
		start, err := observation.Instant()
		if err != nil {
			t.Fatal(err)
		}
		interval, err := temporal.DurationFromNanoseconds(int64(time.Millisecond))
		if err != nil {
			t.Fatal(err)
		}
		watch, err := background.OpenWallGapWatch(background.WallGapWatchRequest{Parent: t.Context(), Start: start, Interval: interval})
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _, _ = watch.Close() }()
		// Advance the native clock in the synctest bubble; no wall-clock wait.
		time.Sleep(3 * time.Millisecond)
		synctest.Wait()
		before, err := watch.Snapshot()
		if err != nil || !before.Observed || before.Validate() != nil {
			t.Fatalf("native ticker snapshot = %+v / %v, want observed validated gap", before, err)
		}
		var closes sync.WaitGroup
		for range 4 {
			closes.Go(func() {
				if _, err := watch.Close(); err != nil {
					t.Errorf("Close() = %v, want nil", err)
				}
			})
		}
		closes.Wait()
		time.Sleep(3 * time.Millisecond)
		after, err := watch.Snapshot()
		if err != nil || after != before {
			t.Fatalf("joined snapshot = %+v / %v, want unchanged %+v", after, err, before)
		}
	})
}

func TestWallGapWatchRefusesInvalidIngressAndPreservesParentCause(t *testing.T) {
	t.Parallel()
	interval, _ := temporal.DurationFromNanoseconds(1)
	start := temporal.InstantFromNanoseconds(1)
	parent, cancel := context.WithCancelCause(t.Context())
	cancel(io.ErrNoProgress)
	for _, tc := range []struct {
		name    string
		request background.WallGapWatchRequest
		want    error
	}{
		{name: "absent parent", request: background.WallGapWatchRequest{Start: start, Interval: interval}, want: core.ErrNilContext},
		{name: "terminal parent", request: background.WallGapWatchRequest{Parent: parent, Start: start, Interval: interval}, want: io.ErrNoProgress},
		{name: "unset start", request: background.WallGapWatchRequest{Parent: t.Context(), Interval: interval}, want: core.ErrTemporalContract},
		{name: "zero cadence", request: background.WallGapWatchRequest{Parent: t.Context(), Start: start}, want: core.ErrTemporalContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := background.OpenWallGapWatch(tc.request)
			if got != nil || !errors.Is(err, tc.want) {
				t.Fatalf("OpenWallGapWatch() = %v / %v, want nil / %v", got, err, tc.want)
			}
		})
	}
	active, stop := context.WithCancelCause(t.Context())
	watch, err := background.OpenWallGapWatch(background.WallGapWatchRequest{Parent: active, Start: start, Interval: interval})
	if err != nil {
		t.Fatal(err)
	}
	stop(io.ErrUnexpectedEOF)
	if result, err := watch.Close(); err != nil || !errors.Is(result.ActionError, io.ErrUnexpectedEOF) || !errors.Is(result.ActionError, context.Canceled) {
		t.Fatalf("Close(parent canceled) = %+v / %v, want native action cause and completed join", result, err)
	}
}

func FuzzWallGapWatchAdmissionAndJoinedSnapshot(f *testing.F) {
	f.Add(int64(1), int64(1000000), false)
	f.Add(int64(-1), int64(0), false)
	f.Add(int64(1), int64(1), true)
	f.Fuzz(func(t *testing.T, startNs, cadenceNs int64, canceled bool) {
		parent, cancel := context.WithCancelCause(t.Context())
		defer cancel(nil)
		if canceled {
			cancel(io.ErrNoProgress)
		}
		interval, err := temporal.DurationFromNanoseconds(cadenceNs)
		if err != nil {
			return
		}
		request := background.WallGapWatchRequest{Parent: parent, Start: temporal.InstantFromNanoseconds(startNs), Interval: interval}
		watch, err := background.OpenWallGapWatch(request)
		if cadenceNs == 0 || canceled {
			want := error(core.ErrTemporalContract)
			if canceled {
				want = io.ErrNoProgress
			}
			if watch != nil || !errors.Is(err, want) {
				t.Fatalf("refused ingress = %v / %v, want nil / %v", watch, err, want)
			}
			return
		}
		if err != nil {
			t.Fatalf("valid ingress = %v, want nil", err)
		}
		result, closeErr := watch.Close()
		if closeErr != nil || (result.ActionError != nil && !errors.Is(result.ActionError, core.ErrTemporalOverflow) && !errors.Is(result.ActionError, context.Canceled)) {
			t.Fatalf("Close() = %v, want nil or native interval overflow", closeErr)
		}
		before, err := watch.Snapshot()
		if errors.Is(result.ActionError, core.ErrTemporalOverflow) {
			if before != (background.WallGapSnapshot{}) || !errors.Is(err, core.ErrTemporalOverflow) {
				t.Fatalf("failed joined snapshot = %+v / %v, want no observation and sampling cause", before, err)
			}
			return
		}
		if err != nil || before.Validate() != nil {
			t.Fatalf("closed snapshot = %+v / %v", before, err)
		}
		if again, err := watch.Close(); err != nil || !errors.Is(again.ActionError, result.ActionError) {
			t.Fatalf("repeated Close() = %v, want first native result %v", again, result.ActionError)
		}
		after, err := watch.Snapshot()
		if err != nil || after != before {
			t.Fatalf("joined snapshot changed = %+v / %v, want %+v", after, err, before)
		}
	})
}
