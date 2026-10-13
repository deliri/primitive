package background

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func FuzzBackgroundCallbackCauseSemanticClosure(f *testing.F) {
	// These are validated internal causes, not a new external byte protocol.
	// The actual worker and completion callback must preserve each admitted
	// cause, including a native error-valued panic.
	if err := core.ErrReportConflict.Validate(); err != nil {
		f.Fatal(err)
	}
	f.Add(uint16(core.ErrReportConflict), false)
	f.Add(uint16(core.ErrReportConflict), true)
	f.Add(uint16(core.ErrBackgroundCallbackPanic), false)
	f.Fuzz(func(t *testing.T, ordinal uint16, panics bool) {
		cause := core.ErrorIdentity(ordinal)
		if cause.Validate() != nil {
			return
		}
		duration, err := temporal.DurationFromSeconds(5)
		if err != nil {
			t.Fatal(err)
		}
		e, err := Open(OpenRequest{Parent: t.Context(), Policy: Policy{1, 1, duration}})
		if err != nil {
			t.Fatal(err)
		}
		var runs, completions atomic.Uint32
		completed := make(chan error, 1)
		if err := e.Submit(Work{Run: func(context.Context) error {
			runs.Add(1)
			if panics {
				panic(cause)
			}
			return cause
		}, Complete: func(err error) { completions.Add(1); completed <- err }}); err != nil {
			t.Fatal(err)
		}
		got := <-completed
		if err := e.Stop(t.Context()); err != nil {
			t.Fatal(err)
		}
		wantPanic := panics || errors.Is(cause, core.ErrBackgroundCallbackPanic)
		if !errors.Is(got, cause) || errors.Is(got, core.ErrBackgroundCallbackPanic) != wantPanic || runs.Load() != 1 || completions.Load() != 1 {
			t.Fatalf("native callback=(%v,%d/%d),want exact cause %v panic=%t and one action/observation", got, runs.Load(), completions.Load(), cause, panics)
		}
	})
}
