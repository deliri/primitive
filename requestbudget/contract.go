package requestbudget

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// Capacity is the consumer's fixed maximum of simultaneously retained keys.
// A key includes both owner and budget class; this is not an owner count.
type Capacity uint16

// NewCapacity admits the consumer's explicit fixed key capacity.
func NewCapacity(value uint16) (Capacity, error) {
	capacity := Capacity(value)
	if err := capacity.Validate(); err != nil {
		return 0, err
	}
	return capacity, nil
}

func (c Capacity) Validate() error {
	if c == 0 {
		return core.ErrRequestBudgetContract
	}
	return nil
}

// Key is a consumer-derived, opaque, fixed-width identity for one budget.
// Derivation must include its namespace, owner and class. No text is retained.
type Key struct{ value [core.SHA256DigestBytes]byte }

func NewKey(value [core.SHA256DigestBytes]byte) (Key, error) {
	key := Key{value: value}
	if err := key.Validate(); err != nil {
		return Key{}, err
	}
	return key, nil
}

func (k Key) Validate() error {
	if k.value == ([core.SHA256DigestBytes]byte{}) {
		return core.ErrRequestBudgetContract
	}
	return nil
}

// Bytes projects the validated identity for the consumer's typed adapter.
func (k Key) Bytes() ([core.SHA256DigestBytes]byte, error) {
	if err := k.Validate(); err != nil {
		return [core.SHA256DigestBytes]byte{}, err
	}
	return k.value, nil
}

// Window is an authoritative half-open interval, not a Primitive daily policy.
type Window struct{ Start, End temporal.Instant }

func (w Window) Validate() error {
	if err := errors.Join(w.Start.Validate(), w.End.Validate()); err != nil {
		return errors.Join(core.ErrRequestBudgetContract, err)
	}
	order, err := w.Start.Compare(w.End)
	if err != nil || order != core.ComparisonLess {
		return errors.Join(core.ErrRequestBudgetContract, err)
	}
	return nil
}

// Reservation is the exact typed request passed to the durable authority.
// A zero-credit successful response means authoritative exhaustion.
type Reservation struct {
	Key    Key
	Window Window
	Batch  uint16
}

func (r Reservation) Validate() error {
	if err := errors.Join(r.Key.Validate(), r.Window.Validate()); err != nil {
		return err
	}
	if r.Batch == 0 {
		return core.ErrRequestBudgetContract
	}
	return nil
}

type Request struct {
	Reservation Reservation
	Observed    temporal.Instant
}

func (r Request) Validate() error {
	if err := errors.Join(r.Reservation.Validate(), r.Observed.Validate()); err != nil {
		return errors.Join(core.ErrRequestBudgetContract, err)
	}
	before, beforeErr := r.Observed.Compare(r.Reservation.Window.Start)
	after, afterErr := r.Observed.Compare(r.Reservation.Window.End)
	if err := errors.Join(beforeErr, afterErr); err != nil {
		return errors.Join(core.ErrRequestBudgetContract, err)
	}
	if before == core.ComparisonLess || after != core.ComparisonLess {
		return core.ErrRequestBudgetWindow
	}
	return nil
}

// Grant must bind the exact reservation. Credits may be a partial final batch.
type Grant struct {
	Reservation Reservation
	Credits     uint16
}

func (g Grant) Validate() error {
	if err := g.Reservation.Validate(); err != nil {
		return err
	}
	if g.Credits > g.Reservation.Batch {
		return core.ErrRequestBudgetContract
	}
	return nil
}

// Reserve is a synchronous, context-bound durable transaction callback.
// It must not call Admit on this executor recursively. Errors never mean
// exhaustion, and any credits returned beside an error are discarded.
type Reserve func(context.Context, Reservation) (Grant, error)

type Decision uint8

const (
	decisionUnknown Decision = iota
	Admitted
	Exhausted
)

func (d Decision) Validate() error {
	if d != Admitted && d != Exhausted {
		return core.ErrRequestBudgetContract
	}
	return nil
}

func (Decision) OffWireEnum() {}
