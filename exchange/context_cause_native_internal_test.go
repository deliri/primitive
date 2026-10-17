package exchange

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
)

type valuePanickingTerminalContext struct{ context.Context }

func (valuePanickingTerminalContext) Value(any) any { panic(core.ErrContextObservation) }

func TestExchangeTerminalObservationPreservesNativeCause(t *testing.T) {
	t.Parallel()
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	caused, cancelCause := context.WithCancelCause(t.Context())
	cancelCause(errors.Join(io.ErrClosedPipe, io.ErrUnexpectedEOF))
	expired, cancelDeadline := context.WithDeadlineCause(t.Context(), time.Unix(0, 0), io.ErrNoProgress)
	defer cancelDeadline()
	for _, tc := range []struct {
		name     string
		ctx      context.Context
		terminal error
		cause    error
	}{
		{name: "active context remains neutral", ctx: t.Context()},
		{name: "native cancel identity", ctx: canceled, terminal: context.Canceled, cause: context.Canceled},
		{name: "joined custom cancel causes", ctx: caused, terminal: context.Canceled, cause: io.ErrClosedPipe},
		{name: "native deadline with custom cause", ctx: expired, terminal: context.DeadlineExceeded, cause: io.ErrNoProgress},
		{name: "terminal retained if foreign Value panics", ctx: valuePanickingTerminalContext{Context: canceled}, terminal: context.Canceled, cause: core.ErrContextObservation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := exchangeContextError(tc.ctx)
			if tc.terminal == nil {
				if got != nil {
					t.Fatalf("active observation=%v,want nil", got)
				}
				return
			}
			if !errors.Is(got, core.ErrExchangeCancelled) || !errors.Is(got, tc.terminal) || !errors.Is(got, tc.cause) {
				t.Fatalf("terminal observation=%v,want exchange cancellation, %v and %v", got, tc.terminal, tc.cause)
			}
			if tc.ctx == caused && !errors.Is(got, io.ErrUnexpectedEOF) {
				t.Fatalf("second joined cause lost: %v", got)
			}
		})
	}
}
