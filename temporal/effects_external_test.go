package temporal_test

import (
	"context"
	"errors"
	"math"
	"testing"
	"testing/synctest"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

type temporalContextKey uint8

const temporalTestContextKey temporalContextKey = iota

func TestTimeoutEffectLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                                                                 string
		duration, parentDuration, wantDeadline, wantElapsed                  time.Duration
		parentDeadline, parentCancelled, childCancelled, detached, nilParent bool
		wantErr, wantTerminal                                                error
	}{
		{name: "zero expires at construction", wantTerminal: context.DeadlineExceeded},
		{name: "minimum positive timeout fires exactly", duration: 1, wantDeadline: 1, wantElapsed: 1, wantTerminal: context.DeadlineExceeded},
		{name: "maximum duration reaches Go without narrowing", duration: math.MaxInt64, wantDeadline: math.MaxInt64, childCancelled: true, wantTerminal: context.Canceled},
		{name: "caller cancellation precedes timeout", duration: 9, wantDeadline: 9, childCancelled: true, wantTerminal: context.Canceled},
		{name: "cancelled parent dominates future timeout", duration: 9, wantDeadline: 9, parentCancelled: true, wantTerminal: context.Canceled},
		{name: "cancelled parent dominates zero timeout", parentCancelled: true, wantTerminal: context.Canceled},
		{name: "earlier parent deadline is inherited", duration: 9, parentDeadline: true, parentDuration: 8, wantDeadline: 8, wantElapsed: 8, wantTerminal: context.DeadlineExceeded},
		{name: "equal parent deadline remains exact", duration: 9, parentDeadline: true, parentDuration: 9, wantDeadline: 9, wantElapsed: 9, wantTerminal: context.DeadlineExceeded},
		{name: "later parent deadline cannot extend child", duration: 9, parentDeadline: true, parentDuration: 10, wantDeadline: 9, wantElapsed: 9, wantTerminal: context.DeadlineExceeded},
		{name: "expired parent dominates future child", duration: 9, parentDeadline: true, parentDuration: -1, wantDeadline: -1, wantTerminal: context.DeadlineExceeded},
		{name: "WithoutCancel retains values but removes parent cancellation", duration: 1, wantDeadline: 1, wantElapsed: 1, parentCancelled: true, detached: true, wantTerminal: context.DeadlineExceeded},
		{name: "nil parent cannot yield capabilities", duration: 1, nilParent: true, wantErr: core.ErrNilContext},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				start, err := temporal.Observe()
				if err != nil {
					t.Fatal(err)
				}
				wall, err := start.Instant()
				if err != nil {
					t.Fatal(err)
				}
				startTime, err := wall.Time()
				if err != nil {
					t.Fatal(err)
				}
				parent := context.WithValue(t.Context(), temporalTestContextKey, wall)
				parent, parentCancel := context.WithCancelCause(parent)
				defer parentCancel(nil)
				if tc.parentCancelled {
					parentCancel(core.ErrTemporalContract)
				}
				if tc.parentDeadline {
					var cancel context.CancelFunc
					parent, cancel = context.WithDeadline(parent, startTime.Add(tc.parentDuration))
					defer cancel()
				}
				if tc.detached {
					parent = context.WithoutCancel(parent)
				}
				if tc.nilParent {
					parent = nil
				}
				duration, err := temporal.NewDuration(tc.duration)
				if err != nil {
					t.Fatal(err)
				}
				got, cancel, gotErr := temporal.WithTimeout(temporal.TimeoutRequest{Parent: parent, Duration: duration})
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("timeout error = %v, want %v", gotErr, tc.wantErr)
				}
				if tc.wantErr != nil {
					if got != nil || cancel != nil || !errors.Is(gotErr, core.ErrTemporalContract) {
						t.Fatalf("refusal = (%v,%v,%v), want no capabilities and Temporal identity", got, cancel, gotErr)
					}
					return
				}
				if got == nil || cancel == nil {
					t.Fatalf("timeout capabilities = (%v,%v), want both present", got, cancel)
				}
				defer cancel()
				deadline, set := got.Deadline()
				if !set || !deadline.Equal(startTime.Add(tc.wantDeadline)) || got.Value(temporalTestContextKey) != wall {
					t.Fatalf("deadline/value = (%v,%t,%v), want exact inherited facts", deadline, set, got.Value(temporalTestContextKey))
				}
				if tc.childCancelled {
					cancel()
					cancel()
				}
				<-got.Done()
				finish, finishErr := temporal.Observe()
				elapsed, elapsedErr := finish.Since(start)
				if finishErr != nil || elapsedErr != nil || elapsed.Nanoseconds() != int64(tc.wantElapsed) || got.Err() != tc.wantTerminal {
					t.Fatalf("completion = (%v,%v,%v,%v), want elapsed %v and %v", elapsed, finishErr, elapsedErr, got.Err(), tc.wantElapsed, tc.wantTerminal)
				}
				wantCause := tc.wantTerminal
				if tc.parentCancelled && !tc.detached {
					wantCause = core.ErrTemporalContract
				}
				if context.Cause(got) != wantCause {
					t.Fatalf("cause = %v, want %v", context.Cause(got), wantCause)
				}
			})
		})
	}
}

func TestDeadlineEffectLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                                     string
		delta                                    time.Duration
		unset, nilParent, cancelled, cancelChild bool
		wantElapsed                              time.Duration
		wantErr, wantTerminal                    error
	}{
		{name: "one nanosecond before present is expired", delta: -1, wantTerminal: context.DeadlineExceeded},
		{name: "present is already expired", wantTerminal: context.DeadlineExceeded},
		{name: "one nanosecond after present fires exactly", delta: 1, wantElapsed: 1, wantTerminal: context.DeadlineExceeded},
		{name: "caller can cancel before future deadline", delta: 9, cancelChild: true, wantTerminal: context.Canceled},
		{name: "cancelled parent dominates future deadline", delta: 9, cancelled: true, wantTerminal: context.Canceled},
		{name: "cancelled parent dominates expired deadline", delta: -1, cancelled: true, wantTerminal: context.Canceled},
		{name: "unset deadline refuses capabilities", unset: true, wantErr: core.ErrTemporalContract},
		{name: "nil parent refuses capabilities", nilParent: true, wantErr: core.ErrNilContext},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				start, err := temporal.Observe()
				if err != nil {
					t.Fatal(err)
				}
				wall, err := start.Instant()
				if err != nil {
					t.Fatal(err)
				}
				startTime, err := wall.Time()
				if err != nil {
					t.Fatal(err)
				}
				deadline, err := temporal.NewInstant(startTime.Add(tc.delta))
				if err != nil {
					t.Fatal(err)
				}
				if tc.unset {
					deadline = temporal.Instant{}
				}
				parent, parentCancel := context.WithCancel(t.Context())
				defer parentCancel()
				if tc.cancelled {
					parentCancel()
				}
				if tc.nilParent {
					parent = nil
				}
				got, cancel, gotErr := temporal.WithDeadline(temporal.DeadlineRequest{Parent: parent, Deadline: deadline})
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("deadline error = %v, want %v", gotErr, tc.wantErr)
				}
				if tc.wantErr != nil {
					if got != nil || cancel != nil || !errors.Is(gotErr, core.ErrTemporalContract) {
						t.Fatalf("refusal = (%v,%v,%v), want no capabilities and Temporal identity", got, cancel, gotErr)
					}
					return
				}
				if got == nil || cancel == nil {
					t.Fatalf("deadline capabilities = (%v,%v), want both present", got, cancel)
				}
				defer cancel()
				gotDeadline, set := got.Deadline()
				if !set || !gotDeadline.Equal(startTime.Add(tc.delta)) {
					t.Fatalf("deadline = (%v,%t), want %v", gotDeadline, set, startTime.Add(tc.delta))
				}
				if tc.cancelChild {
					cancel()
				}
				<-got.Done()
				finish, finishErr := temporal.Observe()
				elapsed, elapsedErr := finish.Since(start)
				if finishErr != nil || elapsedErr != nil || elapsed.Nanoseconds() != int64(tc.wantElapsed) || got.Err() != tc.wantTerminal {
					t.Fatalf("completion = (%v,%v,%v,%v), want %v and %v", elapsed, finishErr, elapsedErr, got.Err(), tc.wantElapsed, tc.wantTerminal)
				}
			})
		})
	}
}

func TestWaitEffectLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                                  string
		duration, parentDuration, wantElapsed time.Duration
		parentDeadline, cancelled, nilParent  bool
		wantErr                               error
	}{
		{name: "zero wait does not advance Go time"},
		{name: "minimum positive wait cannot return early", duration: 1, wantElapsed: 1},
		{name: "timer completes before later context deadline", duration: 8, parentDuration: 9, parentDeadline: true, wantElapsed: 8},
		{name: "earlier deadline cancels a longer timer", duration: 9, parentDuration: 8, parentDeadline: true, wantElapsed: 8, wantErr: context.DeadlineExceeded},
		{name: "maximum duration remains cancellable", duration: math.MaxInt64, parentDuration: 1, parentDeadline: true, wantElapsed: 1, wantErr: context.DeadlineExceeded},
		{name: "cancelled context rejects even zero wait", cancelled: true, wantErr: context.Canceled},
		{name: "expired parent rejects before timer", duration: 1, parentDeadline: true, parentDuration: -1, wantErr: context.DeadlineExceeded},
		{name: "nil context refuses even zero wait", nilParent: true, wantErr: core.ErrNilContext},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				start, err := temporal.Observe()
				if err != nil {
					t.Fatal(err)
				}
				parent, cancel := context.WithCancel(t.Context())
				defer cancel()
				if tc.cancelled {
					cancel()
				}
				if tc.parentDeadline {
					var stop context.CancelFunc
					parent, stop = context.WithTimeout(parent, tc.parentDuration)
					defer stop()
				}
				if tc.nilParent {
					parent = nil
				}
				duration, err := temporal.NewDuration(tc.duration)
				if err != nil {
					t.Fatal(err)
				}
				gotErr := temporal.Wait(temporal.WaitRequest{Context: parent, Duration: duration})
				finish, finishErr := temporal.Observe()
				elapsed, elapsedErr := finish.Since(start)
				if !errors.Is(gotErr, tc.wantErr) || finishErr != nil || elapsedErr != nil || elapsed.Nanoseconds() != int64(tc.wantElapsed) {
					t.Fatalf("wait = (%v,%v,%v,%v), want error %v and elapsed %v", gotErr, elapsed, finishErr, elapsedErr, tc.wantErr, tc.wantElapsed)
				}
			})
		})
	}
}

func TestTickerEffectLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		interval time.Duration
		ticks    int
		wantErr  error
	}{
		{name: "zero interval is refused before Go can panic", wantErr: core.ErrTemporalContract},
		{name: "minimum interval retains nanosecond separation", interval: 1, ticks: 2},
		{name: "caller stop prevents the first tick", interval: 7},
		{name: "caller stop prevents subsequent ticks", interval: 7, ticks: 1},
		{name: "maximum interval is transferred without narrowing", interval: math.MaxInt64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				duration, err := temporal.NewDuration(tc.interval)
				if err != nil {
					t.Fatal(err)
				}
				got, gotErr := temporal.OpenTicker(temporal.TickerRequest{Interval: duration})
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("ticker error = %v, want %v", gotErr, tc.wantErr)
				}
				if tc.wantErr != nil {
					if got != nil {
						got.Stop()
						t.Fatalf("refused ticker = %v, want nil", got)
					}
					return
				}
				if got == nil || got.Validate() != nil || got.Ticks() == nil {
					t.Fatalf("ticker = %v, want usable caller-owned capability", got)
				}
				defer got.Stop()
				channel := got.Ticks()
				start, err := temporal.Observe()
				if err != nil {
					t.Fatal(err)
				}
				wall, err := start.Instant()
				if err != nil {
					t.Fatal(err)
				}
				now, err := wall.Time()
				if err != nil {
					t.Fatal(err)
				}
				for tick := range tc.ticks {
					reading := <-channel
					want := now.Add(time.Duration(tick+1) * tc.interval)
					if !reading.Equal(want) {
						t.Fatalf("tick = %v, want %v", reading, want)
					}
				}
				got.Stop()
				got.Stop()
				if got.Ticks() != channel {
					t.Fatalf("stopped channel = %v, want original %v", got.Ticks(), channel)
				}
				backstop := time.NewTimer(time.Nanosecond)
				defer backstop.Stop()
				select {
				case value, open := <-channel:
					t.Fatalf("stopped ticker read = (%v,%t), want neither tick nor channel closure", value, open)
				case <-backstop.C:
				}
			})
		})
	}
}

func TestTemporalEffectRequestsExhaustTheirTypedIngressBoundaries(t *testing.T) {
	t.Parallel()

	zero, _ := temporal.DurationFromNanoseconds(0)
	one, _ := temporal.DurationFromNanoseconds(1)
	maximum, _ := temporal.DurationFromNanoseconds(temporal.DurationMaximumNanoseconds)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, expire := context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer expire()

	cases := []struct {
		validate func() error
		wantErr  error
		name     string
	}{
		{name: "timeout admits neutral zero duration", validate: func() error {
			return (temporal.TimeoutRequest{Parent: context.Background(), Duration: zero}).Validate()
		}},
		{name: "timeout admits maximum duration", validate: func() error {
			return (temporal.TimeoutRequest{Parent: context.Background(), Duration: maximum}).Validate()
		}},
		{name: "timeout rejects nil parent", validate: func() error {
			return (temporal.TimeoutRequest{Duration: one}).Validate()
		}, wantErr: core.ErrNilContext},
		{name: "deadline admits minimum signed instant", validate: func() error {
			return (temporal.DeadlineRequest{Parent: context.Background(), Deadline: temporal.InstantFromNanoseconds(math.MinInt64)}).Validate()
		}},
		{name: "deadline admits maximum signed instant", validate: func() error {
			return (temporal.DeadlineRequest{Parent: context.Background(), Deadline: temporal.InstantFromNanoseconds(math.MaxInt64)}).Validate()
		}},
		{name: "deadline rejects unset instant", validate: func() error {
			return (temporal.DeadlineRequest{Parent: context.Background()}).Validate()
		}, wantErr: core.ErrTemporalContract},
		{name: "deadline rejects nil parent", validate: func() error {
			return (temporal.DeadlineRequest{Deadline: temporal.InstantFromNanoseconds(0)}).Validate()
		}, wantErr: core.ErrNilContext},
		{name: "wait admits neutral zero duration", validate: func() error {
			return (temporal.WaitRequest{Context: context.Background(), Duration: zero}).Validate()
		}},
		{name: "wait admits maximum duration", validate: func() error {
			return (temporal.WaitRequest{Context: context.Background(), Duration: maximum}).Validate()
		}},
		{name: "wait rejects nil context", validate: func() error {
			return (temporal.WaitRequest{Duration: one}).Validate()
		}, wantErr: core.ErrNilContext},
		{name: "wait rejects cancelled context", validate: func() error {
			return (temporal.WaitRequest{Context: cancelled, Duration: one}).Validate()
		}, wantErr: context.Canceled},
		{name: "wait rejects expired context", validate: func() error {
			return (temporal.WaitRequest{Context: expired, Duration: one}).Validate()
		}, wantErr: context.DeadlineExceeded},
		{name: "ticker admits minimum positive interval", validate: func() error {
			return (temporal.TickerRequest{Interval: one}).Validate()
		}},
		{name: "ticker admits maximum interval", validate: func() error {
			return (temporal.TickerRequest{Interval: maximum}).Validate()
		}},
		{name: "ticker rejects zero interval", validate: func() error {
			return (temporal.TickerRequest{Interval: zero}).Validate()
		}, wantErr: core.ErrTemporalContract},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("request.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func TestTickerCapabilityExhaustsUnsetRepresentations(t *testing.T) {
	t.Parallel()

	cases := []struct {
		value *temporal.Ticker
		name  string
	}{
		{name: "nil ticker pointer is unset"},
		{name: "allocated zero ticker is unset", value: new(temporal.Ticker)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.value.Validate()
			tc.value.Stop()
			tc.value.Stop()
			if !errors.Is(gotErr, core.ErrTemporalContract) {
				t.Fatalf("unset Ticker.Validate = %v, want %v", gotErr, core.ErrTemporalContract)
			}
			defer func() {
				got, ok := recover().(error)
				if !ok || !errors.Is(got, core.ErrTemporalContract) {
					t.Fatalf("unset Ticker.Ticks panic = %v, want typed contract refusal", got)
				}
			}()
			_ = tc.value.Ticks()
		})
	}
}
