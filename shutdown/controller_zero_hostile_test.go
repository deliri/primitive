package shutdown_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/shutdown"
)

// TestControllerRefusesAHandleThatSkippedWatch closes the degenerate-receiver
// gap on Close: new(shutdown.Controller) compiles, is not nil, and holds no
// channels, so before this contract Close closed a nil channel and would then
// have waited on one forever. Watch is the one constructor; skipping it is a
// caller defect to report loudly, not a panic and not a deadlock.
func TestControllerRefusesAHandleThatSkippedWatch(t *testing.T) {
	t.Parallel()

	if err := new(shutdown.Controller).Close(); !errors.Is(err, core.ErrShutdownContract) {
		t.Fatalf("Close(unconstructed controller) error = %v, want errors.Is %v", err, core.ErrShutdownContract)
	}

	var absent *shutdown.Controller
	if err := absent.Close(); !errors.Is(err, core.ErrShutdownContract) {
		t.Fatalf("Close(nil controller) error = %v, want errors.Is %v", err, core.ErrShutdownContract)
	}
}
