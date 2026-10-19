// Package admission borrows one immediate execution slot for a synchronous
// operation. It retains one counter, has no queue, and owns no goroutine.
// Limits are caller policy; the counter and scope lifetime use Go's standard
// library. Admission is process-local and does not promise fleet-wide quotas.
package admission

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// Limit owns a positive native concurrent-slot count, without an arbitrary
// maximum below the native uint64 representation.
type Limit struct{ maximum uint64 }

// NewLimit admits one caller-owned capacity policy.
func NewLimit(maximum uint64) (Limit, error) {
	value := Limit{maximum: maximum}
	if err := value.Validate(); err != nil {
		return Limit{}, err
	}
	return value, nil
}

// Validate rejects an absent capacity policy.
func (l Limit) Validate() error {
	if l.maximum == 0 {
		return core.ErrAdmissionContract
	}
	return nil
}

// Maximum returns the admitted native slot count.
func (l Limit) Maximum() uint64 { return l.maximum }

// Observation carries the exact post-reservation count, rather than a later
// approximate sample. It is a native observation, not a durable entitlement.
type Observation struct {
	inUse uint64
	limit Limit
}

// Validate checks the reservation observation's admitted counting agreement.
func (o Observation) Validate() error {
	if o.limit.Validate() != nil || o.inUse == 0 || o.inUse > o.limit.maximum {
		return core.ErrAdmissionContract
	}
	return nil
}

// InUse returns the count at the successful reservation.
func (o Observation) InUse() uint64 { return o.inUse }

// Limit returns the capacity bound that admitted this reservation.
func (o Observation) Limit() Limit { return o.limit }

// Gate is one native process-local counter capability. It must not be copied
// after first use; sync/atomic owns that requirement.
// Source: https://pkg.go.dev/sync/atomic#Uint64
type Gate struct {
	used  atomic.Uint64
	limit Limit
}

// NewGate creates an empty gate from an admitted limit. It starts no worker and
// reserves no slot until WithSlot reaches its synchronous operation.
func NewGate(limit Limit) (*Gate, error) {
	if err := limit.Validate(); err != nil {
		return nil, err
	}
	return &Gate{limit: limit}, nil
}

// Validate rejects nil or uninitialized counter capabilities.
func (g *Gate) Validate() error {
	if g == nil {
		return core.ErrAdmissionContract
	}
	return g.limit.Validate()
}

// WithSlot either refuses immediately or executes operation once in the
// caller's goroutine. It never waits for capacity, queues an operation, retries
// a callback, or invents a timeout. The reservation is released by defer on
// normal return, error, panic and runtime.Goexit. Callback errors remain exact.
// Cancellation before execution produces no callback; cancellation during an
// operation is cooperative, and its context remains owned by the caller.
// Source: https://pkg.go.dev/sync/atomic#Uint64.CompareAndSwap
// Source: https://go.dev/ref/spec#Defer_statements
func (g *Gate) WithSlot(ctx context.Context, operation func(context.Context, Observation) error) error {
	if err := g.Validate(); err != nil {
		return err
	}
	if operation == nil {
		return core.ErrAdmissionContract
	}
	if err := usableContext(ctx); err != nil {
		return err
	}
	for {
		current := g.used.Load()
		if current >= g.limit.maximum {
			return core.ErrAdmissionCapacity
		}
		if g.used.CompareAndSwap(current, current+1) {
			return g.runReserved(ctx, operation, current+1)
		}
		if err := usableContext(ctx); err != nil {
			return err
		}
	}
}

// runReserved owns exactly one successful reservation. Keeping its release in
// a straight-line function lets Go open-code the defer instead of allocating a
// heap defer record inside WithSlot's retry loop; the release still runs on
// normal return, error, panic and runtime.Goexit.
func (g *Gate) runReserved(ctx context.Context, operation func(context.Context, Observation) error, inUse uint64) error {
	defer g.used.Add(^uint64(0))
	if err := usableContext(ctx); err != nil {
		return err
	}
	return operation(ctx, Observation{inUse: inUse, limit: g.limit})
}

// Context termination is a native cause, not an admission-contract failure.
// Source: https://pkg.go.dev/context#Cause
func usableContext(ctx context.Context) error {
	err := contextstate.Validate(ctx)
	if err == context.Canceled || err == context.DeadlineExceeded {
		cause := context.Cause(ctx)
		if cause != nil && cause != err {
			return errors.Join(err, cause)
		}
	}
	return err
}
