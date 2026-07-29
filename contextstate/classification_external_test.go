package contextstate_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

type singleWrappedError struct {
	child error
}

func (e singleWrappedError) Error() string { return "" }
func (e singleWrappedError) Unwrap() error { return e.child }

type manyWrappedError struct {
	children []error
}

func (e manyWrappedError) Error() string   { return "" }
func (e manyWrappedError) Unwrap() []error { return e.children }

type matchingError struct {
	cancelled bool
	deadline  bool
}

func (e matchingError) Error() string { return "" }

func (e matchingError) Is(target error) bool {
	switch target {
	case context.Canceled:
		return e.cancelled
	case context.DeadlineExceeded:
		return e.deadline
	default:
		return false
	}
}

type nonComparableError []byte

func (nonComparableError) Error() string { return "" }

type nilReceiverError struct{}

func (*nilReceiverError) Error() string { return "" }
func (e *nilReceiverError) Unwrap() error {
	if e == nil {
		return nil
	}
	return core.ErrPrimitiveContract
}

type panickingIsError struct{}

func (panickingIsError) Error() string { return "" }
func (panickingIsError) Is(error) bool { panic(core.ErrContextObservation) }

type panickingSingleUnwrapError struct{}

func (panickingSingleUnwrapError) Error() string { return "" }
func (panickingSingleUnwrapError) Unwrap() error {
	panic(core.ErrContextObservation)
}

type panickingManyUnwrapError struct{}

func (panickingManyUnwrapError) Error() string { return "" }
func (panickingManyUnwrapError) Unwrap() []error {
	panic(core.ErrContextObservation)
}

type deadlineAfterCancelledProbe struct{}

func (deadlineAfterCancelledProbe) Error() string { return "" }
func (deadlineAfterCancelledProbe) Is(target error) bool {
	if target == context.Canceled {
		panic(core.ErrContextObservation)
	}
	return target == context.DeadlineExceeded
}

type cycleError struct {
	child error
}

func (*cycleError) Error() string { return "" }
func (e *cycleError) Unwrap() error {
	return e.child
}

func TestClassifyPublicSemanticMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		cause     func() error
		wantErr   error
		name      string
		wantState contextstate.State
	}{
		{name: "nil is safely non-terminal", cause: fixedCause(nil), wantState: contextstate.StateNone},
		{name: "unrelated stable error is safely non-terminal", cause: fixedCause(core.ErrPrimitiveContract), wantState: contextstate.StateNone},
		{name: "non-comparable error is safely non-terminal", cause: fixedCause(nonComparableError{}), wantState: contextstate.StateNone},
		{name: "typed nil with safe unwrap is safely non-terminal", cause: fixedCause((*nilReceiverError)(nil)), wantState: contextstate.StateNone},
		{name: "exact cancellation is cancelled", cause: fixedCause(context.Canceled), wantState: contextstate.StateCancelled},
		{name: "exact deadline is deadline exceeded", cause: fixedCause(context.DeadlineExceeded), wantState: contextstate.StateDeadlineExceeded},
		{name: "single wrapped cancellation is cancelled", cause: fixedCause(singleWrappedError{child: context.Canceled}), wantState: contextstate.StateCancelled},
		{name: "single wrapped deadline is deadline exceeded", cause: fixedCause(singleWrappedError{child: context.DeadlineExceeded}), wantState: contextstate.StateDeadlineExceeded},
		{name: "custom cancellation identity is cancelled", cause: fixedCause(matchingError{cancelled: true}), wantState: contextstate.StateCancelled},
		{name: "custom deadline identity is deadline exceeded", cause: fixedCause(matchingError{deadline: true}), wantState: contextstate.StateDeadlineExceeded},
		{name: "custom contradictory identity gives cancellation precedence", cause: fixedCause(matchingError{cancelled: true, deadline: true}), wantState: contextstate.StateCancelled},
		{name: "joined cancellation before deadline gives cancellation precedence", cause: fixedCause(errors.Join(context.Canceled, context.DeadlineExceeded)), wantState: contextstate.StateCancelled},
		{name: "joined deadline before cancellation gives cancellation precedence", cause: fixedCause(errors.Join(context.DeadlineExceeded, context.Canceled)), wantState: contextstate.StateCancelled},
		{name: "deep cancellation gives cancellation precedence", cause: fixedCause(singleWrappedError{child: manyWrappedError{children: []error{context.DeadlineExceeded, singleWrappedError{child: context.Canceled}}}}), wantState: contextstate.StateCancelled},
		{name: "duplicate deadlines remain deadline exceeded", cause: fixedCause(manyWrappedError{children: []error{context.DeadlineExceeded, context.DeadlineExceeded}}), wantState: contextstate.StateDeadlineExceeded},
		{name: "nil children are safely skipped", cause: fixedCause(manyWrappedError{children: []error{nil, core.ErrPrimitiveContract}}), wantState: contextstate.StateNone},
		{name: "cancellation before hostile sibling is decisive", cause: fixedCause(manyWrappedError{children: []error{context.Canceled, panickingIsError{}}}), wantState: contextstate.StateCancelled},
		{name: "panicking identity is unobservable", cause: fixedCause(panickingIsError{}), wantState: contextstate.StateUnknown, wantErr: core.ErrContextObservation},
		{name: "panicking single unwrap is unobservable", cause: fixedCause(panickingSingleUnwrapError{}), wantState: contextstate.StateUnknown, wantErr: core.ErrContextObservation},
		{name: "panicking multi unwrap is unobservable", cause: fixedCause(panickingManyUnwrapError{}), wantState: contextstate.StateUnknown, wantErr: core.ErrContextObservation},
		{name: "self cycle is unobservable", cause: selfCycleCause, wantState: contextstate.StateUnknown, wantErr: core.ErrContextObservation},
		{name: "two node cycle is unobservable", cause: twoNodeCycleCause, wantState: contextstate.StateUnknown, wantErr: core.ErrContextObservation},
		{name: "deadline before hostile sibling is unobservable", cause: fixedCause(manyWrappedError{children: []error{context.DeadlineExceeded, panickingIsError{}}}), wantState: contextstate.StateUnknown, wantErr: core.ErrContextObservation},
		{name: "hostile sibling before cancellation is unobservable", cause: fixedCause(manyWrappedError{children: []error{panickingIsError{}, context.Canceled}}), wantState: contextstate.StateUnknown, wantErr: core.ErrContextObservation},
		{name: "cycle before cancellation is unobservable", cause: fixedCause(cycleBeforeCause(context.Canceled)), wantState: contextstate.StateUnknown, wantErr: core.ErrContextObservation},
		{name: "deadline before cycle is unobservable", cause: fixedCause(cycleAfterCause(context.DeadlineExceeded)), wantState: contextstate.StateUnknown, wantErr: core.ErrContextObservation},
		{name: "panic while probing cancellation blocks later deadline match", cause: fixedCause(deadlineAfterCancelledProbe{}), wantState: contextstate.StateUnknown, wantErr: core.ErrContextObservation},
		{name: "nested panicking unwrap is unobservable", cause: fixedCause(singleWrappedError{child: panickingManyUnwrapError{}}), wantState: contextstate.StateUnknown, wantErr: core.ErrContextObservation},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotState, gotErr := contextstate.Classify(tc.cause())
			if gotState != tc.wantState {
				t.Fatalf("Classify() state = %v, want %v", gotState, tc.wantState)
			}
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Classify() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func fixedCause(cause error) func() error {
	return func() error {
		return cause
	}
}

func selfCycleCause() error {
	cycle := &cycleError{}
	cycle.child = cycle
	return cycle
}

func twoNodeCycleCause() error {
	first := &cycleError{}
	second := &cycleError{child: first}
	first.child = second
	return first
}

func cycleBeforeCause(after error) error {
	return manyWrappedError{children: []error{selfCycleCause(), after}}
}

func cycleAfterCause(before error) error {
	return manyWrappedError{children: []error{before, selfCycleCause()}}
}
