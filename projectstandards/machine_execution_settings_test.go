package projectstandards

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestMachineObservationExecutionSettingsBindsObservedCapacity(t *testing.T) {
	t.Parallel()

	t.Run("validated observation yields its exact generation and logical CPU capacity", func(t *testing.T) {
		t.Parallel()
		observation := fixtureCurrentMachine(t).Observation
		got, gotErr := observation.ExecutionSettings()
		if gotErr != nil || got.Observation != observation.ID || got.Generation != observation.GenerationID || got.LogicalCPUCount != observation.Configuration.Compute.VCPU {
			t.Fatalf("MachineObservation.ExecutionSettings() = (%+v, %v), want exact observation/generation and %d logical CPUs", got, gotErr, observation.Configuration.Compute.VCPU)
		}
	})

	t.Run("invalid observation cannot issue execution settings", func(t *testing.T) {
		t.Parallel()
		observation := fixtureCurrentMachine(t).Observation
		observation.Configuration.Compute.VCPU = 0
		got, gotErr := observation.ExecutionSettings()
		if !errors.Is(gotErr, core.ErrProjectStandardsContract) || got != (MachineExecutionSettings{}) {
			t.Fatalf("MachineObservation.ExecutionSettings(zero CPUs) = (%+v, %v), want zero and errors.Is(..., %v)", got, gotErr, core.ErrProjectStandardsContract)
		}
	})

	t.Run("detached zero execution settings fail closed", func(t *testing.T) {
		t.Parallel()
		if gotErr := (MachineExecutionSettings{}).Validate(); !errors.Is(gotErr, core.ErrProjectStandardsContract) {
			t.Fatalf("MachineExecutionSettings{}.Validate() error = %v, want errors.Is(..., %v)", gotErr, core.ErrProjectStandardsContract)
		}
	})
}
