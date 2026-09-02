package process_test

import (
	"testing"

	"github.com/deliri/primitive/v2026/process"
)

func TestDiscardDeviceArgumentProjectsOneValidatedPlatformIdentity(t *testing.T) {
	t.Parallel()

	got, gotErr := process.DiscardDeviceArgument()
	if gotErr != nil {
		t.Fatalf("process.DiscardDeviceArgument() error = %v, want nil", gotErr)
	}
	value, valueErr := got.Value()
	if valueErr != nil || value == "" {
		t.Fatalf("process.DiscardDeviceArgument().Value() = (%q, %v), want nonempty and nil", value, valueErr)
	}
}
