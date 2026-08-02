package contextstate

import (
	"context"

	"github.com/deliri/primitive/v2026/core"
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
		return stateUnknown, err
	}
	if state == StateNone {
		return stateUnknown, core.ErrContextObservation
	}
	return state, nil
}

// Observe returns the current standard terminal state. An active context
// returns StateNone. A nil, panicking, or nonstandard context returns the zero
// State with a typed error.
func Observe(ctx context.Context) (State, error) {
	if ctx == nil {
		return stateUnknown, core.ErrNilContext
	}
	terminal, err := readContextError(ctx)
	if err != nil {
		return stateUnknown, err
	}
	if terminal == nil {
		return StateNone, nil
	}
	return classifyContextTerminal(terminal)
}

func classifyContextTerminal(terminal error) (
	state State,
	err error,
) {
	state = stateUnknown
	err = core.ErrContextObservation
	defer func() {
		if recover() != nil {
			state = stateUnknown
			err = core.ErrContextObservation
		}
	}()
	switch terminal {
	case nil:
		return StateNone, nil
	case context.Canceled:
		return StateCancelled, nil
	case context.DeadlineExceeded:
		return StateDeadlineExceeded, nil
	default:
		return stateUnknown, core.ErrContextObservation
	}
}

func readContextError(ctx context.Context) (
	terminal error,
	err error,
) {
	err = core.ErrContextObservation
	defer func() {
		if recover() != nil {
			terminal = nil
			err = core.ErrContextObservation
		}
	}()
	return ctx.Err(), nil
}
