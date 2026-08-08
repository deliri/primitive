//go:build !windows

package process_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

// TestObserveProcessesRefusesWhereNoSnapshotExists pins the platform split:
// a POSIX host offers no kernel process snapshot, and the refusal points at
// composing the platform's own lister through Run instead of inventing one.
func TestObserveProcessesRefusesWhereNoSnapshotExists(t *testing.T) {
	t.Parallel()

	err := process.ObserveProcesses(t.Context(), func(process.ProcessSighting) error { return nil })
	if !errors.Is(err, core.ErrProcessUnsupported) {
		t.Fatalf("ObserveProcesses(posix host) error = %v, want errors.Is %v", err, core.ErrProcessUnsupported)
	}
}

// TestProcessSightingValidatesItsMembers holds the sighting's own admission
// on every host: both members validate or the sighting refuses.
func TestProcessSightingValidatesItsMembers(t *testing.T) {
	t.Parallel()

	var zero process.ProcessSighting
	if err := zero.Validate(); !errors.Is(err, core.ErrProcessContract) {
		t.Fatalf("zero ProcessSighting.Validate() error = %v, want errors.Is %v", err, core.ErrProcessContract)
	}
	image, err := core.ParsePathComponent("bug")
	if err != nil {
		t.Fatalf("core.ParsePathComponent(bug) error = %v, want nil", err)
	}
	valid := process.ProcessSighting{Identity: process.ProcessIdentity(1), Image: image}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid ProcessSighting.Validate() error = %v, want nil", err)
	}
	missingImage := process.ProcessSighting{Identity: process.ProcessIdentity(1)}
	if err := missingImage.Validate(); !errors.Is(err, core.ErrProcessContract) {
		t.Fatalf("imageless ProcessSighting.Validate() error = %v, want errors.Is %v", err, core.ErrProcessContract)
	}
}

// TestObserveProcessesHoldsItsContractGates refuses a terminal context and a
// nil visitor before any platform ask.
func TestObserveProcessesHoldsItsContractGates(t *testing.T) {
	t.Parallel()

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := process.ObserveProcesses(cancelled, func(process.ProcessSighting) error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("ObserveProcesses(cancelled context) error = %v, want errors.Is %v", err, context.Canceled)
	}
	if err := process.ObserveProcesses(context.Background(), nil); !errors.Is(err, core.ErrProcessContract) {
		t.Fatalf("ObserveProcesses(nil visitor) error = %v, want errors.Is %v", err, core.ErrProcessContract)
	}
}
