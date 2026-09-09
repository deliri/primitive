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

	cases := []struct {
		name  string
		value *shutdown.Controller
	}{
		{name: "unconstructed controller refuses close", value: new(shutdown.Controller)},
		{name: "nil controller refuses close"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.value.Close(); !errors.Is(err, core.ErrShutdownContract) {
				t.Fatalf("Close = %v, want shutdown contract", err)
			}
			if tc.value.Context() != nil || tc.value.Done() != nil || tc.value.Escalated() != nil {
				t.Fatalf("unconstructed accessors = (%v,%v,%v), want nil/nil/nil", tc.value.Context(), tc.value.Done(), tc.value.Escalated())
			}
		})
	}
}
