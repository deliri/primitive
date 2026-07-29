package contextstate

import (
	"context"

	"github.com/deliri/primitive/v2026/core"
)

const errorGraphNodeMaximum = 128

type errorGraphObservation uint8

const (
	errorGraphObservationUnknown errorGraphObservation = iota
	errorGraphObservationSafe
	errorGraphObservationUnsafe
)

type errorGraphTraversal struct {
	remaining int
}

// Classify safely projects a caller-supplied error graph onto State.
// Cancellation takes precedence over deadline expiration. A hostile, cyclic,
// or over-budget graph returns StateUnknown and ErrContextObservation.
//
// Precedence is total only over a graph that is safely observable throughout.
// Traversal stops at the first cancellation and at the first unobservable node,
// so a hostile sibling positioned before a cancellation makes the whole graph
// unobservable while the same sibling positioned after it does not. Recovering
// per node would replace single-recover containment with wider machinery, so
// the package accepts the position dependence and states it here.
func Classify(cause error) (State, error) {
	state, observation := observeErrorGraph(cause)
	if observation != errorGraphObservationSafe {
		return StateUnknown, core.ErrContextObservation
	}
	return state, nil
}

func observeErrorGraph(cause error) (
	state State,
	observation errorGraphObservation,
) {
	state = StateUnknown
	observation = errorGraphObservationUnsafe
	defer func() {
		if recover() != nil {
			state = StateUnknown
			observation = errorGraphObservationUnsafe
		}
	}()

	traversal := errorGraphTraversal{remaining: errorGraphNodeMaximum}
	return traversal.observe(cause)
}

func (t *errorGraphTraversal) observe(
	node error,
) (State, errorGraphObservation) {
	if node == nil {
		return StateNone, errorGraphObservationSafe
	}
	if t.remaining == 0 {
		return StateUnknown, errorGraphObservationUnsafe
	}
	t.remaining--

	nodeState := classifyErrorNode(node)
	if nodeState == StateCancelled {
		return StateCancelled, errorGraphObservationSafe
	}
	childState, observation := t.observeChildren(node)
	return mergeObservedStates(nodeState, childState, observation)
}

func (t *errorGraphTraversal) observeChildren(
	node error,
) (State, errorGraphObservation) {
	switch value := node.(type) {
	case interface{ Unwrap() error }:
		return t.observe(value.Unwrap())
	case interface{ Unwrap() []error }:
		return t.observeMany(value.Unwrap())
	default:
		return StateNone, errorGraphObservationSafe
	}
}

func (t *errorGraphTraversal) observeMany(
	children []error,
) (State, errorGraphObservation) {
	state := StateNone
	for _, child := range children {
		childState, observation := t.observe(child)
		state, observation = mergeObservedStates(
			state,
			childState,
			observation,
		)
		if observation != errorGraphObservationSafe ||
			state == StateCancelled {
			return state, observation
		}
	}
	return state, errorGraphObservationSafe
}

func classifyErrorNode(node error) State {
	if errorNodeMatches(node, context.Canceled) {
		return StateCancelled
	}
	if errorNodeMatches(node, context.DeadlineExceeded) {
		return StateDeadlineExceeded
	}
	return StateNone
}

func errorNodeMatches(node, target error) bool {
	if node == target {
		return true
	}
	matcher, ok := node.(interface{ Is(error) bool })
	return ok && matcher.Is(target)
}

func mergeObservedStates(
	left State,
	right State,
	observation errorGraphObservation,
) (State, errorGraphObservation) {
	if observation != errorGraphObservationSafe {
		return StateUnknown, observation
	}
	if left == StateCancelled || right == StateCancelled {
		return StateCancelled, observation
	}
	if left == StateDeadlineExceeded || right == StateDeadlineExceeded {
		return StateDeadlineExceeded, observation
	}
	return StateNone, observation
}
