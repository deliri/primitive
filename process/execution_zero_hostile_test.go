package process

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// TestExecutionRefusesAHandleThatSkippedBegin closes the degenerate-receiver
// gap on the supervision doors: new(process.Execution) compiles, is not nil,
// and holds no started child, so before this contract Deliver dereferenced a
// nil exec.Cmd and Wait deferred a nil cancel. Every door must refuse the
// unstarted handle loudly, because Begin is the one constructor and skipping
// it is a caller defect to report, not a crash.
func TestExecutionRefusesAHandleThatSkippedBegin(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		handle *Execution
	}{
		{name: "negative/nil handle cannot supervise a child"},
		{name: "neutral/allocated zero handle is not a begun execution", handle: new(Execution)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.handle.Deliver(CancelSignalKill); !errors.Is(err, core.ErrProcessContract) {
				t.Fatalf("Deliver=%v; want contract refusal", err)
			}
			if err := tc.handle.Terminate(); !errors.Is(err, core.ErrProcessContract) {
				t.Fatalf("Terminate=%v; want contract refusal", err)
			}
			if err := tc.handle.Sweep(); !errors.Is(err, core.ErrProcessContract) {
				t.Fatalf("Sweep=%v; want contract refusal", err)
			}
			identity, err := tc.handle.Identity()
			if identity != 0 || !errors.Is(err, core.ErrProcessContract) {
				t.Fatalf("Identity=%v, %v; want zero and contract refusal", identity, err)
			}
			result, err := tc.handle.Wait()
			if result != (Result{}) || !errors.Is(err, core.ErrProcessContract) {
				t.Fatalf("Wait=%+v, %v; want zero and contract refusal", result, err)
			}
			again, againErr := tc.handle.Wait()
			if again != (Result{}) || !errors.Is(againErr, core.ErrProcessContract) {
				t.Fatalf("repeated Wait=%+v, %v; refusal must not admit the zero handle", again, againErr)
			}
		})
	}
}
