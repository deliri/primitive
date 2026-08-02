package contextstate_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

type panickingIdentityError struct{}

func (panickingIdentityError) Error() string { return "panicking identity" }
func (panickingIdentityError) Is(error) bool { panic("identity panic") }

func TestObserveErrorPressuresTerminalIdentityBoundary(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		err       error
		wantState contextstate.State
		wantErr   error
	}{
		{name: "nil remains active", wantState: contextstate.StateNone},
		{name: "cancellation is exact", err: context.Canceled, wantState: contextstate.StateCancelled},
		{name: "wrapped cancellation remains cancellation", err: fmt.Errorf("boundary: %w", context.Canceled), wantState: contextstate.StateCancelled},
		{name: "deadline is exact", err: context.DeadlineExceeded, wantState: contextstate.StateDeadlineExceeded},
		{name: "wrapped deadline remains deadline", err: fmt.Errorf("boundary: %w", context.DeadlineExceeded), wantState: contextstate.StateDeadlineExceeded},
		{name: "unrelated error is rejected", err: errors.New("unrelated"), wantErr: core.ErrContextObservation},
		{name: "panicking identity graph is contained", err: panickingIdentityError{}, wantErr: core.ErrContextObservation},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotState, gotErr := contextstate.ObserveError(tc.err)
			if gotState != tc.wantState {
				t.Fatalf("ObserveError() state = %v, want %v", gotState, tc.wantState)
			}
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("ObserveError() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}
