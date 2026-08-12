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

	for _, handle := range []*Execution{nil, new(Execution)} {
		if err := handle.Deliver(CancelSignalKill); !errors.Is(err, core.ErrProcessContract) {
			t.Fatalf("Deliver(unstarted %v) error = %v, want errors.Is %v", handle, err, core.ErrProcessContract)
		}
		if err := handle.Terminate(); !errors.Is(err, core.ErrProcessContract) {
			t.Fatalf("Terminate(unstarted %v) error = %v, want errors.Is %v", handle, err, core.ErrProcessContract)
		}
		result, err := handle.Wait()
		if !errors.Is(err, core.ErrProcessContract) {
			t.Fatalf("Wait(unstarted %v) error = %v, want errors.Is %v", handle, err, core.ErrProcessContract)
		}
		if err := result.Validate(); !errors.Is(err, core.ErrProcessContract) {
			t.Fatalf("Wait(unstarted %v) result validation error = %v, want errors.Is %v", handle, err, core.ErrProcessContract)
		}
	}

	for _, handle := range []*Execution{nil, new(Execution)} {
		if _, err := handle.Identity(); !errors.Is(err, core.ErrProcessContract) {
			t.Fatalf("Identity(unstarted %v) error = %v, want errors.Is %v", handle, err, core.ErrProcessContract)
		}
	}
}
