package requestbudget

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func fixtureRequest(t *testing.T, identity byte) Request {
	t.Helper()
	key, err := NewKey([core.SHA256DigestBytes]byte{identity})
	if err != nil {
		t.Fatalf("NewKey() error = %v, want nil", err)
	}
	return Request{Reservation: Reservation{Key: key, Window: Window{Start: temporal.InstantFromNanoseconds(0), End: temporal.InstantFromNanoseconds(10)}, Batch: 2}, Observed: temporal.InstantFromNanoseconds(1)}
}

func fixtureExecutor(t *testing.T, capacity uint16, reserve Reserve) *Executor {
	t.Helper()
	admitted, err := NewCapacity(capacity)
	if err != nil {
		t.Fatalf("NewCapacity() error = %v, want nil", err)
	}
	e, err := New(admitted, reserve)
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	return e
}

func TestCapacityWindowLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		change    func(*Request)
		want      Decision
		wantErr   error
		wantCalls int
	}{
		{"cached exhaustion never re-reserves", func(*Request) {}, Exhausted, nil, 1},
		{"different live key refuses capacity", func(r *Request) { r.Reservation.Key, _ = NewKey([32]byte{2}) }, decisionUnknown, core.ErrRequestBudgetCapacity, 1},
		{"changed batch refuses binding", func(r *Request) { r.Reservation.Batch++ }, decisionUnknown, core.ErrRequestBudgetBinding, 1},
		{"overlapping window refuses binding", func(r *Request) { r.Reservation.Window.End = temporal.InstantFromNanoseconds(11) }, decisionUnknown, core.ErrRequestBudgetBinding, 1},
		{"same key next window reserves anew", func(r *Request) {
			r.Reservation.Window = Window{Start: temporal.InstantFromNanoseconds(10), End: temporal.InstantFromNanoseconds(20)}
			r.Observed = temporal.InstantFromNanoseconds(10)
		}, Exhausted, nil, 2},
		{"expired slot admits different key", func(r *Request) {
			r.Reservation.Key, _ = NewKey([32]byte{2})
			r.Reservation.Window = Window{Start: temporal.InstantFromNanoseconds(10), End: temporal.InstantFromNanoseconds(20)}
			r.Observed = temporal.InstantFromNanoseconds(10)
		}, Exhausted, nil, 2},
		{"one below window end keeps exhaustion", func(r *Request) { r.Observed = temporal.InstantFromNanoseconds(9) }, Exhausted, nil, 1},
		{"exact window end refuses observation", func(r *Request) { r.Observed = temporal.InstantFromNanoseconds(10) }, decisionUnknown, core.ErrRequestBudgetWindow, 1},
		{"one above window end refuses observation", func(r *Request) { r.Observed = temporal.InstantFromNanoseconds(11) }, decisionUnknown, core.ErrRequestBudgetWindow, 1},
		{"one below start refuses observation", func(r *Request) { r.Observed = temporal.InstantFromNanoseconds(-1) }, decisionUnknown, core.ErrRequestBudgetWindow, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			e := fixtureExecutor(t, 1, func(_ context.Context, r Reservation) (Grant, error) { calls++; return Grant{Reservation: r}, nil })
			r := fixtureRequest(t, 1)
			if got, err := e.Admit(context.Background(), r); got != Exhausted || err != nil {
				t.Fatalf("initial Admit() = (%v,%v), want (Exhausted,nil)", got, err)
			}
			tc.change(&r)
			for range 3 {
				got, err := e.Admit(context.Background(), r)
				if got != tc.want || !errors.Is(err, tc.wantErr) {
					t.Fatalf("Admit() = (%v,%v), want (%v,%v)", got, err, tc.want, tc.wantErr)
				}
			}
			if calls != tc.wantCalls {
				t.Fatalf("calls = %d, want %d", calls, tc.wantCalls)
			}
		})
	}
}

func TestCreditGrantHandoffExhaustive(t *testing.T) {
	t.Parallel()
	// Exhaust the entire uint16 credit domain and both binding/error states.
	// Primary classes are contradiction (foreign/over-batch), typed refusal
	// (provider error), neutral (zero), boundary (positive exact arithmetic).
	// This complete 262144-case domain replaces a padded fifty-row sampler.
	providerErr := errors.New("test durable authority unavailable")
	for raw := uint32(0); raw <= math.MaxUint16; raw++ {
		for _, foreign := range []bool{false, true} {
			for _, refused := range []bool{false, true} {
				r := fixtureRequest(t, 1)
				e := fixtureExecutor(t, 1, func(_ context.Context, res Reservation) (Grant, error) {
					if foreign {
						res.Key, _ = NewKey([32]byte{2})
					}
					g := Grant{Reservation: res, Credits: uint16(raw)}
					if refused {
						return g, providerErr
					}
					return g, nil
				})
				got, err := e.Admit(context.Background(), r)
				want := decisionUnknown
				var wantErr error
				switch {
				case refused:
					wantErr = providerErr
				case raw > uint32(r.Reservation.Batch):
					wantErr = core.ErrRequestBudgetContract
				case foreign:
					wantErr = core.ErrRequestBudgetBinding
				case raw == 0:
					want = Exhausted
				default:
					want = Admitted
				}
				if got != want || !errors.Is(err, wantErr) {
					t.Fatalf("credits=%d foreign=%t refused=%t Admit() = (%v,%v), want (%v,%v)", raw, foreign, refused, got, err, want, wantErr)
				}
				if wantErr != nil && e.slots[0] != (slot{}) {
					t.Fatalf("rejected grant slot = %+v, want zero", e.slots[0])
				}
				if wantErr == nil {
					wantCredits := uint16(raw)
					if raw > 0 {
						wantCredits--
					}
					if e.slots[0].credits != wantCredits || e.slots[0].exhausted != (raw == 0) {
						t.Fatalf("retained slot = %+v, want credits %d and exhausted %t", e.slots[0], wantCredits, raw == 0)
					}
				}
			}
		}
	}
}

func TestConcurrentCreditConservation(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		remaining, calls := uint16(96), 0
		e := fixtureExecutor(t, 1, func(ctx context.Context, r Reservation) (Grant, error) {
			if err := ctx.Err(); err != nil {
				return Grant{}, err
			}
			calls++
			grant := min(r.Batch, remaining)
			remaining -= grant
			return Grant{Reservation: r, Credits: grant}, nil
		})
		r := fixtureRequest(t, 1)
		r.Reservation.Batch = 32
		type result struct {
			decision Decision
			err      error
		}
		results := make(chan result, 256)
		var workers sync.WaitGroup
		for range 256 {
			workers.Go(func() { got, err := e.Admit(ctx, r); results <- result{got, err} })
		}
		workers.Wait()
		close(results)
		admitted, exhausted := 0, 0
		for got := range results {
			if got.err != nil {
				t.Fatalf("concurrent Admit() error = %v, want nil", got.err)
			}
			switch got.decision {
			case Admitted:
				admitted++
			case Exhausted:
				exhausted++
			default:
				t.Fatalf("decision = %v, want admitted/exhausted", got.decision)
			}
		}
		if admitted != 96 || exhausted != 160 || calls != 4 || remaining != 0 {
			t.Fatalf("conservation = admitted%d exhausted%d calls%d remaining%d, want 96/160/4/0", admitted, exhausted, calls, remaining)
		}
	})
}

func TestCancellationReservationLayerTriad(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		entered := make(chan struct{})
		release := make(chan struct{})
		calls := 0
		e := fixtureExecutor(t, 2, func(ctx context.Context, r Reservation) (Grant, error) {
			calls++
			if r.Key == fixtureRequest(t, 1).Reservation.Key {
				close(entered)
				select {
				case <-release:
				case <-ctx.Done():
					return Grant{Reservation: r, Credits: 2}, nil
				}
			}
			return Grant{Reservation: r, Credits: 2}, nil
		})
		r := fixtureRequest(t, 1)
		first := make(chan error, 1)
		go func() { _, err := e.Admit(ctx, r); first <- err }()
		<-entered
		waiterCtx, waiterCancel := context.WithCancel(context.Background())
		defer waiterCancel()
		waiter := make(chan error, 1)
		go func() { _, err := e.Admit(waiterCtx, r); waiter <- err }()
		synctest.Wait()
		waiterCancel()
		if err := <-waiter; !errors.Is(err, context.Canceled) {
			t.Fatalf("waiting Admit() error = %v, want canceled", err)
		}
		// Independent key progresses while the first callback remains blocked.
		if got, err := e.Admit(context.Background(), fixtureRequest(t, 2)); got != Admitted || err != nil {
			t.Fatalf("independent Admit() = (%v,%v), want admitted/nil", got, err)
		}
		cancel()
		if err := <-first; !errors.Is(err, context.Canceled) {
			t.Fatalf("completed canceled Admit() error = %v, want canceled", err)
		}
		close(release)
		for range 2 {
			if got, err := e.Admit(context.Background(), r); got != Admitted || err != nil {
				t.Fatalf("retained credit Admit() = (%v,%v), want admitted/nil", got, err)
			}
		}
		if calls != 2 {
			t.Fatalf("callbacks = %d, want 2 (all canceled caller credits preserved)", calls)
		}
	})
}

func TestWindowAdvanceRejectsLateGrantAndRollback(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		entered, release := make(chan struct{}), make(chan struct{})
		e := fixtureExecutor(t, 2, func(ctx context.Context, r Reservation) (Grant, error) {
			if r.Key == fixtureRequest(t, 1).Reservation.Key {
				close(entered)
				select {
				case <-release:
				case <-ctx.Done():
					return Grant{}, ctx.Err()
				}
			}
			return Grant{Reservation: r, Credits: 2}, nil
		})
		old := fixtureRequest(t, 1)
		done := make(chan error, 1)
		go func() { _, err := e.Admit(ctx, old); done <- err }()
		<-entered
		next := fixtureRequest(t, 2)
		next.Reservation.Window = Window{Start: temporal.InstantFromNanoseconds(10), End: temporal.InstantFromNanoseconds(20)}
		next.Observed = temporal.InstantFromNanoseconds(10)
		if got, err := e.Admit(ctx, next); got != Admitted || err != nil {
			t.Fatalf("new window Admit() = (%v,%v), want admitted/nil", got, err)
		}
		close(release)
		if err := <-done; !errors.Is(err, core.ErrRequestBudgetWindow) {
			t.Fatalf("late grant error = %v, want window error", err)
		}
		if got, err := e.Admit(ctx, old); got != decisionUnknown || !errors.Is(err, core.ErrRequestBudgetWindow) {
			t.Fatalf("rollback Admit() = (%v,%v), want zero/window error", got, err)
		}
	})
}

func TestProviderFailureAndPanicNeverBecomeExhaustion(t *testing.T) {
	t.Parallel()
	r := fixtureRequest(t, 1)
	failure := errors.New("test provider failure")
	calls := 0
	e := fixtureExecutor(t, 1, func(_ context.Context, res Reservation) (Grant, error) {
		calls++
		if calls == 1 {
			return Grant{Reservation: res, Credits: 2}, failure
		}
		return Grant{Reservation: res, Credits: 2}, nil
	})
	if got, err := e.Admit(context.Background(), r); got != decisionUnknown || !errors.Is(err, failure) {
		t.Fatalf("failed Admit() = (%v,%v), want zero/provider failure", got, err)
	}
	if got, err := e.Admit(context.Background(), r); got != Admitted || err != nil {
		t.Fatalf("retry Admit() = (%v,%v), want admitted/nil", got, err)
	}
	panicValue := "test callback panic"
	panicCalls := 0
	p := fixtureExecutor(t, 1, func(_ context.Context, res Reservation) (Grant, error) {
		panicCalls++
		if panicCalls == 1 {
			panic(panicValue)
		}
		return Grant{Reservation: res, Credits: 1}, nil
	})
	func() {
		defer func() {
			if got := recover(); got != panicValue {
				t.Fatalf("callback panic = %v, want %q", got, panicValue)
			}
		}()
		_, _ = p.Admit(context.Background(), r)
	}()
	if got, err := p.Admit(context.Background(), r); got != Admitted || err != nil {
		t.Fatalf("after panic Admit() = (%v,%v), want admitted/nil", got, err)
	}
}

func TestPendingWindowRolloverAndDeadline(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		entered, release := make(chan struct{}), make(chan struct{})
		calls := 0
		e := fixtureExecutor(t, 1, func(ctx context.Context, res Reservation) (Grant, error) {
			calls++
			if calls == 1 {
				close(entered)
				select {
				case <-release:
				case <-ctx.Done():
					return Grant{}, ctx.Err()
				}
			}
			return Grant{Reservation: res, Credits: res.Batch}, nil
		})
		old := fixtureRequest(t, 1)
		oldDone := make(chan error, 1)
		go func() { _, err := e.Admit(ctx, old); oldDone <- err }()
		<-entered
		deadlineCtx, deadlineCancel := context.WithTimeout(ctx, time.Second)
		defer deadlineCancel()
		deadlineDone := make(chan error, 1)
		go func() { _, err := e.Admit(deadlineCtx, old); deadlineDone <- err }()
		if err := <-deadlineDone; !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("waiting deadline error = %v, want deadline exceeded", err)
		}
		next := old
		next.Reservation.Window = Window{Start: temporal.InstantFromNanoseconds(10), End: temporal.InstantFromNanoseconds(20)}
		next.Observed = temporal.InstantFromNanoseconds(10)
		nextDone := make(chan error, 1)
		go func() {
			got, err := e.Admit(ctx, next)
			if err == nil && got != Admitted {
				err = core.ErrRequestBudgetContract
			}
			nextDone <- err
		}()
		synctest.Wait()
		close(release)
		if err := <-oldDone; !errors.Is(err, core.ErrRequestBudgetWindow) {
			t.Fatalf("old pending window error = %v, want window rejection", err)
		}
		if err := <-nextDone; err != nil {
			t.Fatalf("next pending window error = %v, want nil", err)
		}
		if calls != 2 {
			t.Fatalf("window callback count = %d, want 2", calls)
		}
	})
}

func TestGrantBindingFieldsAndMaximumCredits(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		change func(*Reservation)
	}{
		{"foreign key cannot mint credits", func(r *Reservation) { r.Key, _ = NewKey([32]byte{2}) }},
		{"foreign start cannot mint credits", func(r *Reservation) { r.Window.Start = temporal.InstantFromNanoseconds(-1) }},
		{"foreign end cannot mint credits", func(r *Reservation) { r.Window.End = temporal.InstantFromNanoseconds(11) }},
		{"foreign batch cannot mint credits", func(r *Reservation) { r.Batch = 3 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := fixtureExecutor(t, 1, func(_ context.Context, r Reservation) (Grant, error) {
				tc.change(&r)
				return Grant{Reservation: r, Credits: 1}, nil
			})
			if got, err := e.Admit(context.Background(), fixtureRequest(t, 1)); got != decisionUnknown || !errors.Is(err, core.ErrRequestBudgetBinding) || e.slots[0] != (slot{}) {
				t.Fatalf("foreign grant = (%v,%v,%+v), want zero/binding/untouched", got, err, e.slots[0])
			}
		})
	}
	r := fixtureRequest(t, 1)
	r.Reservation.Batch = math.MaxUint16
	calls := 0
	e := fixtureExecutor(t, 1, func(_ context.Context, res Reservation) (Grant, error) {
		calls++
		return Grant{Reservation: res, Credits: math.MaxUint16}, nil
	})
	for range math.MaxUint16 {
		if got, err := e.Admit(context.Background(), r); got != Admitted || err != nil {
			t.Fatalf("maximum batch Admit() = (%v,%v), want admitted/nil", got, err)
		}
	}
	if calls != 1 || e.slots[0].credits != 0 || e.slots[0].exhausted {
		t.Fatalf("maximum batch state = calls%d slot%+v, want 1/zero credits/unknown exhaustion", calls, e.slots[0])
	}
}
