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

func TestTimeoutEffectLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive timeout delegates an exact deadline to context", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			duration, _ := temporal.DurationFromNanoseconds(5)
			start, _ := temporal.Observe()
			ctx, cancel, gotErr := temporal.WithTimeout(temporal.TimeoutRequest{
				Parent:   t.Context(),
				Duration: duration,
			})
			if gotErr != nil || ctx == nil || cancel == nil {
				t.Fatalf("WithTimeout() = (%v, %v, %v), want context/cancel/nil", ctx, cancel, gotErr)
			}
			defer cancel()
			<-ctx.Done()
			finish, _ := temporal.Observe()
			elapsed, elapsedErr := finish.Since(start)
			if elapsedErr != nil || elapsed.Nanoseconds() != 5 ||
				!errors.Is(ctx.Err(), context.DeadlineExceeded) {
				t.Fatalf(
					"timeout completion = (elapsed:%d/%v error:%v), want 5/nil/%v",
					elapsed.Nanoseconds(),
					elapsedErr,
					ctx.Err(),
					context.DeadlineExceeded,
				)
			}
		})
	})

	t.Run("negative nil parent is rejected without a cancel capability", func(t *testing.T) {
		t.Parallel()

		ctx, cancel, gotErr := temporal.WithTimeout(temporal.TimeoutRequest{})
		if ctx != nil || cancel != nil ||
			!errors.Is(gotErr, core.ErrTemporalContract) ||
			!errors.Is(gotErr, core.ErrNilContext) {
			t.Fatalf("WithTimeout(nil) = (%v, %v, %v), want nil/nil temporal+nil-context error", ctx, cancel, gotErr)
		}
	})

	t.Run("neutral terminal parent remains standard cancellation", func(t *testing.T) {
		t.Parallel()

		parent, parentCancel := context.WithCancel(context.Background())
		parentCancel()
		duration, _ := temporal.DurationFromHours(1)
		ctx, cancel, gotErr := temporal.WithTimeout(temporal.TimeoutRequest{
			Parent:   parent,
			Duration: duration,
		})
		if gotErr != nil || ctx == nil || cancel == nil {
			t.Fatalf("WithTimeout(cancelled parent) = (%v, %v, %v), want context/cancel/nil", ctx, cancel, gotErr)
		}
		defer cancel()
		<-ctx.Done()
		if !errors.Is(ctx.Err(), context.Canceled) {
			t.Fatalf("WithTimeout(cancelled parent) error = %v, want %v", ctx.Err(), context.Canceled)
		}
	})
}

func TestDeadlineEffectLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive exact future deadline fires once owned by context", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			start, _ := temporal.Observe()
			startInstant, _ := start.Instant()
			duration, _ := temporal.DurationFromNanoseconds(9)
			deadline, deadlineErr := startInstant.Add(duration)
			ctx, cancel, gotErr := temporal.WithDeadline(temporal.DeadlineRequest{
				Parent:   t.Context(),
				Deadline: deadline,
			})
			if deadlineErr != nil || gotErr != nil || ctx == nil || cancel == nil {
				t.Fatalf("WithDeadline() = (%v, %v, %v) after %v, want context/cancel/nil", ctx, cancel, gotErr, deadlineErr)
			}
			defer cancel()
			<-ctx.Done()
			finish, _ := temporal.Observe()
			elapsed, elapsedErr := finish.Since(start)
			if elapsedErr != nil || elapsed.Nanoseconds() != 9 ||
				!errors.Is(ctx.Err(), context.DeadlineExceeded) {
				t.Fatalf(
					"deadline completion = (elapsed:%d/%v error:%v), want 9/nil/%v",
					elapsed.Nanoseconds(),
					elapsedErr,
					ctx.Err(),
					context.DeadlineExceeded,
				)
			}
		})
	})

	t.Run("negative unset deadline is rejected without capability", func(t *testing.T) {
		t.Parallel()

		ctx, cancel, gotErr := temporal.WithDeadline(temporal.DeadlineRequest{
			Parent: context.Background(),
		})
		if ctx != nil || cancel != nil || !errors.Is(gotErr, core.ErrTemporalContract) {
			t.Fatalf("WithDeadline(unset) = (%v, %v, %v), want nil/nil/%v", ctx, cancel, gotErr, core.ErrTemporalContract)
		}
	})

	t.Run("neutral deadline at present is already expired", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			point, _ := temporal.Observe()
			deadline, _ := point.Instant()
			ctx, cancel, gotErr := temporal.WithDeadline(temporal.DeadlineRequest{
				Parent:   t.Context(),
				Deadline: deadline,
			})
			if gotErr != nil {
				t.Fatalf("WithDeadline(present) error = %v, want nil", gotErr)
			}
			defer cancel()
			<-ctx.Done()
			if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
				t.Fatalf("WithDeadline(present) context error = %v, want %v", ctx.Err(), context.DeadlineExceeded)
			}
		})
	})
}

func TestWaitEffectLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive wait completes at exact Go timer duration", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			duration, _ := temporal.DurationFromNanoseconds(13)
			start, _ := temporal.Observe()
			gotErr := temporal.Wait(temporal.WaitRequest{
				Context:  t.Context(),
				Duration: duration,
			})
			finish, _ := temporal.Observe()
			elapsed, elapsedErr := finish.Since(start)
			if gotErr != nil || elapsedErr != nil || elapsed.Nanoseconds() != 13 {
				t.Fatalf("Wait() = (%v, elapsed:%d/%v), want nil/13/nil", gotErr, elapsed.Nanoseconds(), elapsedErr)
			}
		})
	})

	t.Run("negative cancellation outranks a long timer", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Nanosecond)
			defer cancel()
			duration, _ := temporal.DurationFromHours(1)
			gotErr := temporal.Wait(temporal.WaitRequest{Context: ctx, Duration: duration})
			if !errors.Is(gotErr, context.DeadlineExceeded) {
				t.Fatalf("Wait(deadline during wait) error = %v, want %v", gotErr, context.DeadlineExceeded)
			}
		})
	})

	t.Run("neutral zero wait creates no elapsed delay", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			zero, _ := temporal.DurationFromNanoseconds(0)
			start, _ := temporal.Observe()
			gotErr := temporal.Wait(temporal.WaitRequest{
				Context:  t.Context(),
				Duration: zero,
			})
			finish, _ := temporal.Observe()
			elapsed, elapsedErr := finish.Since(start)
			if gotErr != nil || elapsedErr != nil || !elapsed.IsZero() {
				t.Fatalf("Wait(zero) = (%v, elapsed:%d/%v), want nil/0/nil", gotErr, elapsed.Nanoseconds(), elapsedErr)
			}
		})
	})
}

func TestTickerEffectLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive ticker exposes caller-owned real ticks", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			interval, _ := temporal.DurationFromNanoseconds(11)
			ticker, gotErr := temporal.OpenTicker(temporal.TickerRequest{Interval: interval})
			if gotErr != nil || ticker == nil {
				t.Fatalf("OpenTicker() = (%v, %v), want nonnil ticker/nil", ticker, gotErr)
			}
			gotValidationErr := ticker.Validate()
			gotTicks := ticker.Ticks()
			if gotValidationErr != nil || gotTicks == nil {
				t.Fatalf("opened ticker = (validate:%v ticks:%v), want (nil, nonnil)", gotValidationErr, gotTicks)
			}
			defer ticker.Stop()
			first := <-gotTicks
			second := <-ticker.Ticks()
			if got := second.Sub(first); got != 11*time.Nanosecond {
				t.Fatalf("two real ticker readings differ by %v, want %v", got, 11*time.Nanosecond)
			}
		})
	})

	t.Run("negative zero interval is rejected before standard panic", func(t *testing.T) {
		t.Parallel()

		ticker, gotErr := temporal.OpenTicker(temporal.TickerRequest{})
		if ticker != nil || !errors.Is(gotErr, core.ErrTemporalContract) {
			t.Fatalf("OpenTicker(zero) = (%v, %v), want nil/%v", ticker, gotErr, core.ErrTemporalContract)
		}
	})

	t.Run("neutral caller stop prevents further ticks", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			interval, _ := temporal.DurationFromNanoseconds(7)
			ticker, gotErr := temporal.OpenTicker(temporal.TickerRequest{Interval: interval})
			if gotErr != nil {
				t.Fatalf("OpenTicker() error = %v, want nil", gotErr)
			}
			<-ticker.Ticks()
			ticker.Stop()
			backstop := time.NewTimer(14 * time.Nanosecond)
			defer backstop.Stop()
			select {
			case got := <-ticker.Ticks():
				t.Fatalf("stopped ticker emitted %v, want no further tick", got)
			case <-backstop.C:
			}
		})
	})
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
			gotTicks := tc.value.Ticks()
			tc.value.Stop()
			tc.value.Stop()
			if !errors.Is(gotErr, core.ErrTemporalContract) || gotTicks != nil {
				t.Fatalf("unset Ticker = (validate:%v ticks:%v), want (%v, nil)", gotErr, gotTicks, core.ErrTemporalContract)
			}
		})
	}
}
