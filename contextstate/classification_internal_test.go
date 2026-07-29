package contextstate

import (
	"context"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type budgetSingleError struct {
	child error
}

func (e budgetSingleError) Error() string { return "" }
func (e budgetSingleError) Unwrap() error { return e.child }

type budgetManyError struct {
	children []error
}

func (e budgetManyError) Error() string   { return "" }
func (e budgetManyError) Unwrap() []error { return e.children }

// TestObserveErrorGraphExactNodeBudget is an internal mechanism ratchet. Public
// semantic behavior is proved through Classify in classification_external_test.
func TestObserveErrorGraphExactNodeBudget(t *testing.T) {
	t.Parallel()

	cases := []struct {
		cause           error
		name            string
		wantState       State
		wantObservation errorGraphObservation
	}{
		{
			name:            "depth one below maximum is safe",
			cause:           linearGraph(errorGraphNodeMaximum-1, core.ErrPrimitiveContract),
			wantState:       StateNone,
			wantObservation: errorGraphObservationSafe,
		},
		{
			name:            "depth at maximum is safe",
			cause:           linearGraph(errorGraphNodeMaximum, core.ErrPrimitiveContract),
			wantState:       StateNone,
			wantObservation: errorGraphObservationSafe,
		},
		{
			name:            "deadline at depth maximum is safe",
			cause:           linearGraph(errorGraphNodeMaximum, context.DeadlineExceeded),
			wantState:       StateDeadlineExceeded,
			wantObservation: errorGraphObservationSafe,
		},
		{
			name:            "depth one above maximum is unsafe",
			cause:           linearGraph(errorGraphNodeMaximum+1, core.ErrPrimitiveContract),
			wantState:       StateUnknown,
			wantObservation: errorGraphObservationUnsafe,
		},
		{
			name:            "width one below maximum is safe",
			cause:           wideGraph(errorGraphNodeMaximum - 1),
			wantState:       StateNone,
			wantObservation: errorGraphObservationSafe,
		},
		{
			name:            "width at maximum is safe",
			cause:           wideGraph(errorGraphNodeMaximum),
			wantState:       StateNone,
			wantObservation: errorGraphObservationSafe,
		},
		{
			name:            "width one above maximum is unsafe",
			cause:           wideGraph(errorGraphNodeMaximum + 1),
			wantState:       StateUnknown,
			wantObservation: errorGraphObservationUnsafe,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotState, gotObservation := observeErrorGraph(tc.cause)
			if gotState != tc.wantState {
				t.Fatalf("observeErrorGraph() state = %v, want %v", gotState, tc.wantState)
			}
			if gotObservation != tc.wantObservation {
				t.Fatalf(
					"observeErrorGraph() observation = %d, want %d",
					gotObservation,
					tc.wantObservation,
				)
			}
		})
	}
}

func linearGraph(totalNodes int, leaf error) error {
	graph := leaf
	for node := 1; node < totalNodes; node++ {
		graph = budgetSingleError{child: graph}
	}
	return graph
}

func wideGraph(totalNodes int) error {
	children := make([]error, totalNodes-1)
	for index := range children {
		children[index] = core.ErrPrimitiveContract
	}
	return budgetManyError{children: children}
}
