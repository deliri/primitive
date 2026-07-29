package contextstate

import (
	"context"

	"github.com/deliri/primitive/v2026/core"
)

type contextErrorObservation uint8

const (
	contextErrorObservationUnknown contextErrorObservation = iota
	contextErrorObservationSafe
	contextErrorObservationUnsafe
)

// Validate admits ctx only when it is usable now. It returns the exact standard
// terminal sentinel for a cancelled or expired context.
//
// Validate rejects an untyped nil context and contains a panicking one. It
// cannot detect a typed nil whose Err method is nil-safe: such a value reports
// no terminal state and is admitted as usable, exactly as the standard library
// would treat it. Detecting that case needs reflection, which this package
// excludes.
func Validate(ctx context.Context) error {
	state, err := Observe(ctx)
	if err != nil {
		return err
	}
	switch state {
	case StateNone:
		return nil
	case StateCancelled:
		return context.Canceled
	case StateDeadlineExceeded:
		return context.DeadlineExceeded
	default:
		return core.ErrContextStateContract
	}
}

// ObserveAfterDone returns the standard terminal state after the caller has
// received from ctx.Done. An active or nonstandard state is unobservable.
func ObserveAfterDone(ctx context.Context) (State, error) {
	state, err := Observe(ctx)
	if err != nil {
		return StateUnknown, err
	}
	if state == StateNone {
		return StateUnknown, core.ErrContextObservation
	}
	return state, nil
}

// Observe returns the current standard terminal state. An active context
// returns StateNone. A nil, panicking, or nonstandard context returns
// StateUnknown with a typed error.
func Observe(ctx context.Context) (State, error) {
	if ctx == nil {
		return StateUnknown, core.ErrNilContext
	}
	observation, terminal := readContextError(ctx)
	if observation != contextErrorObservationSafe {
		return StateUnknown, core.ErrContextObservation
	}
	if terminal == nil {
		return StateNone, nil
	}
	state, err := Classify(terminal)
	if err != nil {
		return StateUnknown, err
	}
	if state == StateNone {
		return StateUnknown, core.ErrContextObservation
	}
	return state, nil
}

func readContextError(ctx context.Context) (
	observation contextErrorObservation,
	terminal error,
) {
	observation = contextErrorObservationUnsafe
	defer func() {
		if recover() != nil {
			terminal = nil
			observation = contextErrorObservationUnsafe
		}
	}()
	return contextErrorObservationSafe, ctx.Err()
}
