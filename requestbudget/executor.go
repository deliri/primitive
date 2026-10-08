package requestbudget

import (
	"context"
	"sync"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// Executor must be constructed with New and must not be copied. The global
// mutex protects bounded slot selection, never the external reservation call.
// Same-key callers wait on a completion channel; distinct keys progress alone.
type Executor struct {
	mu        sync.Mutex
	slots     []slot
	reserve   Reserve
	highWater temporal.Instant
}

type slot struct {
	reservation Reservation
	credits     uint16
	exhausted   bool
	pending     chan struct{}
}

func New(capacity Capacity, reserve Reserve) (*Executor, error) {
	if capacity.Validate() != nil || reserve == nil {
		return nil, core.ErrRequestBudgetContract
	}
	return &Executor{slots: make([]slot, int(capacity)), reserve: reserve}, nil
}

func (e *Executor) Admit(ctx context.Context, request Request) (Decision, error) {
	if err := e.validateCall(ctx, request); err != nil {
		return decisionUnknown, err
	}
	for {
		if err := ctx.Err(); err != nil {
			return decisionUnknown, err
		}
		e.mu.Lock()
		index, err := e.selectSlot(request)
		if err != nil {
			e.mu.Unlock()
			return decisionUnknown, err
		}
		s := &e.slots[index]
		if pending := s.pending; pending != nil {
			e.mu.Unlock()
			select {
			case <-pending:
				continue
			case <-ctx.Done():
				return decisionUnknown, ctx.Err()
			}
		}
		if decision, err := consumeSlot(ctx, s); decision != decisionUnknown || err != nil {
			e.mu.Unlock()
			return decision, err
		}
		s.pending = make(chan struct{})
		e.mu.Unlock()
		return e.reserveSlot(ctx, index, request.Reservation)
	}
}

// selectSlot is called only under mu. A high-water observation prevents an
// old window from being recreated after its slot has been safely reclaimed.
func (e *Executor) selectSlot(request Request) (int, error) {
	if err := e.observe(request); err != nil {
		return 0, err
	}
	available := -1
	for i := range e.slots {
		s := &e.slots[i]
		if s.reservation.Key == request.Reservation.Key {
			if err := replaceWindow(s, request.Reservation); err != nil {
				return 0, err
			}
			return i, nil
		}
		reusable, err := reusableSlot(s, e.highWater)
		if err != nil {
			return 0, err
		}
		if reusable {
			available = i
		}
	}
	if available < 0 {
		return 0, core.ErrRequestBudgetCapacity
	}
	e.slots[available] = slot{reservation: request.Reservation}
	return available, nil
}

func (e *Executor) reserveSlot(ctx context.Context, index int, reservation Reservation) (Decision, error) {
	grant, err := e.callReserve(ctx, index, reservation)
	e.mu.Lock()
	defer e.mu.Unlock()
	s := &e.slots[index]
	pending := s.pending
	defer close(pending)
	s.pending = nil
	if err != nil {
		*s = slot{}
		return decisionUnknown, err
	}
	if err := grant.Validate(); err != nil {
		*s = slot{}
		return decisionUnknown, err
	}
	if grant.Reservation != reservation {
		*s = slot{}
		return decisionUnknown, core.ErrRequestBudgetBinding
	}
	s.credits = grant.Credits
	s.exhausted = grant.Credits == 0
	// Durable reservation has happened. Retain every unused credit even when
	// cancellation prevents this caller from consuming one.
	if err := ctx.Err(); err != nil {
		return decisionUnknown, err
	}
	order, compareErr := reservation.Window.End.Compare(e.highWater)
	if compareErr != nil {
		return decisionUnknown, compareErr
	}
	if order != core.ComparisonGreater {
		return decisionUnknown, core.ErrRequestBudgetWindow
	}
	if s.exhausted {
		return Exhausted, nil
	}
	s.credits--
	return Admitted, nil
}

var (
	_ core.Validatable = Capacity(0)
	_ core.Validatable = Key{}
	_ core.Validatable = Window{}
	_ core.Validatable = Reservation{}
	_ core.Validatable = Request{}
	_ core.Validatable = Grant{}
	_ core.OffWireEnum = Decision(0)
)

// Callback panic is a programming failure and propagates to its caller. Its
// pending slot still must be released so other callers cannot remain wedged.
func (e *Executor) callReserve(ctx context.Context, index int, reservation Reservation) (Grant, error) {
	returned := false
	defer func() {
		if returned {
			return
		}
		e.mu.Lock()
		pending := e.slots[index].pending
		e.slots[index] = slot{}
		close(pending)
		e.mu.Unlock()
	}()
	grant, err := e.reserve(ctx, reservation)
	returned = true
	return grant, err
}

func (e *Executor) validateCall(ctx context.Context, request Request) error {
	if ctx == nil || e == nil || e.reserve == nil || len(e.slots) == 0 {
		return core.ErrRequestBudgetContract
	}
	return request.Validate()
}

func consumeSlot(ctx context.Context, s *slot) (Decision, error) {
	if err := ctx.Err(); err != nil {
		return decisionUnknown, err
	}
	if s.exhausted {
		return Exhausted, nil
	}
	if s.credits == 0 {
		return decisionUnknown, nil
	}
	s.credits--
	return Admitted, nil
}

func (e *Executor) observe(request Request) error {
	if e.highWater.IsSet() {
		order, err := request.Reservation.Window.End.Compare(e.highWater)
		if err != nil {
			return err
		}
		if order != core.ComparisonGreater {
			return core.ErrRequestBudgetWindow
		}
	}
	if !e.highWater.IsSet() {
		e.highWater = request.Observed
	} else {
		order, err := request.Observed.Compare(e.highWater)
		if err != nil {
			return err
		}
		if order == core.ComparisonGreater {
			e.highWater = request.Observed
		}
	}

	return nil
}

func replaceWindow(s *slot, reservation Reservation) error {
	if s.reservation.Window == reservation.Window {
		if s.reservation.Batch != reservation.Batch {
			return core.ErrRequestBudgetBinding
		}
		return nil
	}
	order, err := s.reservation.Window.End.Compare(reservation.Window.Start)
	if err != nil {
		return err
	}
	if order == core.ComparisonGreater {
		return core.ErrRequestBudgetBinding
	}
	if s.pending == nil {
		*s = slot{reservation: reservation}
	}
	return nil
}

func reusableSlot(s *slot, observed temporal.Instant) (bool, error) {
	if s.pending != nil {
		return false, nil
	}
	if s.reservation == (Reservation{}) {
		return true, nil
	}
	order, err := s.reservation.Window.End.Compare(observed)
	return order != core.ComparisonGreater, err
}
