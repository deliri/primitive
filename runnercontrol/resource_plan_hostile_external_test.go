package runnercontrol_test

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	primitiveid "github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/standard"
)

func TestPlanResourceWavesHostileBoundaries(t *testing.T) {
	t.Parallel()

	defaultCapacity := resourceCapacity(t, 4, 400, 40, 400, 4)
	defaultRequirement := resourceRequirement(t, 1, 50, 5, 50, false)
	cases := []struct {
		wantErr    error
		name       string
		why        string
		required   []runnercontrol.ResourceRequirement
		wantWidths []uint16
		capacity   runnercontrol.MachineResourceCapacity
		duplicate  bool
	}{
		{name: "one ordinary reservation owns one non-vacuous wave", capacity: defaultCapacity, required: repeatRequirement(defaultRequirement, 1), wantWidths: []uint16{1}, why: "the smallest admitted plan must remain executable"},
		{name: "two compatible reservations pack in admitted order", capacity: defaultCapacity, required: repeatRequirement(defaultRequirement, 2), wantWidths: []uint16{2}, why: "compatible work should use the rented machine"},
		{name: "cpu saturation starts a second wave", capacity: defaultCapacity, required: repeatRequirement(resourceRequirement(t, 3, 50, 5, 50, false), 2), wantWidths: []uint16{1, 1}, why: "CPU may not be overcommitted"},
		{name: "memory saturation starts a second wave", capacity: defaultCapacity, required: repeatRequirement(resourceRequirement(t, 1, 250, 5, 50, false), 2), wantWidths: []uint16{1, 1}, why: "memory may not be overcommitted"},
		{name: "process saturation starts a second wave", capacity: defaultCapacity, required: repeatRequirement(resourceRequirement(t, 1, 50, 25, 50, false), 2), wantWidths: []uint16{1, 1}, why: "process ceilings are evidence"},
		{name: "file saturation starts a second wave", capacity: defaultCapacity, required: repeatRequirement(resourceRequirement(t, 1, 50, 5, 250, false), 2), wantWidths: []uint16{1, 1}, why: "file ceilings are evidence"},
		{name: "exclusive first reservation owns its wave", capacity: defaultCapacity, required: []runnercontrol.ResourceRequirement{resourceRequirement(t, 1, 50, 5, 50, true), defaultRequirement}, wantWidths: []uint16{1, 1}, why: "exclusive measurements cannot overlap"},
		{name: "exclusive middle reservation splits both neighbors", capacity: defaultCapacity, required: []runnercontrol.ResourceRequirement{defaultRequirement, resourceRequirement(t, 1, 50, 5, 50, true), defaultRequirement}, wantWidths: []uint16{1, 1, 1}, why: "both sides of exclusivity must remain isolated"},
		{name: "wave width starts a second deterministic wave", capacity: defaultCapacity, required: repeatRequirement(defaultRequirement, 5), wantWidths: []uint16{4, 1}, why: "worker width is a hard ceiling"},
		{name: "three resource-balanced waves preserve reservation order", capacity: defaultCapacity, required: repeatRequirement(resourceRequirement(t, 2, 200, 20, 200, false), 5), wantWidths: []uint16{2, 2, 1}, why: "packing must not reorder experiment identity"},

		{name: "empty reservation set is refused", capacity: defaultCapacity, wantErr: core.ErrPrimitiveContract, why: "an empty plan would manufacture execution evidence"},
		{name: "duplicate experiment identity is refused", capacity: defaultCapacity, required: repeatRequirement(defaultRequirement, 2), duplicate: true, wantErr: core.ErrPrimitiveContract, why: "one experiment cannot be counted twice"},
		{name: "zero machine cpu is refused", capacity: resourceCapacity(t, 0, 400, 40, 400, 4), required: repeatRequirement(defaultRequirement, 1), wantErr: core.ErrPrimitiveContract, why: "zero CPU is not executable capacity"},
		{name: "zero machine memory is refused", capacity: resourceCapacity(t, 4, 0, 40, 400, 4), required: repeatRequirement(defaultRequirement, 1), wantErr: core.ErrPrimitiveContract, why: "zero memory is not executable capacity"},
		{name: "zero machine process ceiling is refused", capacity: resourceCapacity(t, 4, 400, 0, 400, 4), required: repeatRequirement(defaultRequirement, 1), wantErr: core.ErrPrimitiveContract, why: "a process-less machine cannot execute"},
		{name: "zero machine file ceiling is refused", capacity: resourceCapacity(t, 4, 400, 40, 0, 4), required: repeatRequirement(defaultRequirement, 1), wantErr: core.ErrPrimitiveContract, why: "a file-less process contract is impossible"},
		{name: "zero machine wave ceiling is refused", capacity: resourceCapacity(t, 4, 400, 40, 400, 0), required: repeatRequirement(defaultRequirement, 1), wantErr: core.ErrPrimitiveContract, why: "zero workers cannot account admitted work"},
		{name: "zero reservation cpu is refused", capacity: defaultCapacity, required: repeatRequirement(resourceRequirement(t, 0, 50, 5, 50, false), 1), wantErr: core.ErrPrimitiveContract, why: "every experiment reserves CPU"},
		{name: "zero reservation memory is refused", capacity: defaultCapacity, required: repeatRequirement(resourceRequirement(t, 1, 0, 5, 50, false), 1), wantErr: core.ErrPrimitiveContract, why: "every experiment reserves memory"},
		{name: "zero reservation process ceiling is refused", capacity: defaultCapacity, required: repeatRequirement(resourceRequirement(t, 1, 50, 0, 50, false), 1), wantErr: core.ErrPrimitiveContract, why: "every experiment owns a process tree"},

		{name: "cpu one below machine ceiling is admitted", capacity: defaultCapacity, required: repeatRequirement(resourceRequirement(t, 3, 50, 5, 50, false), 1), wantWidths: []uint16{1}, why: "CPU boundary minus one"},
		{name: "cpu exactly at machine ceiling is admitted", capacity: defaultCapacity, required: repeatRequirement(resourceRequirement(t, 4, 50, 5, 50, false), 1), wantWidths: []uint16{1}, why: "CPU boundary exact"},
		{name: "cpu one above machine ceiling is refused", capacity: defaultCapacity, required: repeatRequirement(resourceRequirement(t, 5, 50, 5, 50, false), 1), wantErr: core.ErrPrimitiveContract, why: "CPU boundary plus one"},
		{name: "cpu maximum carrier is refused without overflow", capacity: defaultCapacity, required: repeatRequirement(resourceRequirement(t, math.MaxUint16, 50, 5, 50, false), 1), wantErr: core.ErrPrimitiveContract, why: "CPU extreme"},
		{name: "memory one below machine ceiling is admitted", capacity: defaultCapacity, required: repeatRequirement(resourceRequirement(t, 1, 399, 5, 50, false), 1), wantWidths: []uint16{1}, why: "memory boundary minus one"},
		{name: "memory exactly at machine ceiling is admitted", capacity: defaultCapacity, required: repeatRequirement(resourceRequirement(t, 1, 400, 5, 50, false), 1), wantWidths: []uint16{1}, why: "memory boundary exact"},
		{name: "memory one above machine ceiling is refused", capacity: defaultCapacity, required: repeatRequirement(resourceRequirement(t, 1, 401, 5, 50, false), 1), wantErr: core.ErrPrimitiveContract, why: "memory boundary plus one"},
		{name: "memory maximum carrier is refused without overflow", capacity: defaultCapacity, required: repeatRequirement(resourceRequirement(t, 1, math.MaxUint64, 5, 50, false), 1), wantErr: core.ErrPrimitiveContract, why: "memory extreme"},
		{name: "process one below machine ceiling is admitted", capacity: defaultCapacity, required: repeatRequirement(resourceRequirement(t, 1, 50, 39, 50, false), 1), wantWidths: []uint16{1}, why: "process boundary minus one"},
		{name: "process exactly at machine ceiling is admitted", capacity: defaultCapacity, required: repeatRequirement(resourceRequirement(t, 1, 50, 40, 50, false), 1), wantWidths: []uint16{1}, why: "process boundary exact"},
		{name: "process one above machine ceiling is refused", capacity: defaultCapacity, required: repeatRequirement(resourceRequirement(t, 1, 50, 41, 50, false), 1), wantErr: core.ErrPrimitiveContract, why: "process boundary plus one"},
		{name: "process maximum carrier is refused without overflow", capacity: defaultCapacity, required: repeatRequirement(resourceRequirement(t, 1, 50, math.MaxUint16, 50, false), 1), wantErr: core.ErrPrimitiveContract, why: "process extreme"},
		{name: "file one below machine ceiling is admitted", capacity: defaultCapacity, required: repeatRequirement(resourceRequirement(t, 1, 50, 5, 399, false), 1), wantWidths: []uint16{1}, why: "file boundary minus one"},
		{name: "file exactly at machine ceiling is admitted", capacity: defaultCapacity, required: repeatRequirement(resourceRequirement(t, 1, 50, 5, 400, false), 1), wantWidths: []uint16{1}, why: "file boundary exact"},
		{name: "file one above machine ceiling is refused", capacity: defaultCapacity, required: repeatRequirement(resourceRequirement(t, 1, 50, 5, 401, false), 1), wantErr: core.ErrPrimitiveContract, why: "file boundary plus one"},
		{name: "file maximum carrier is refused without overflow", capacity: defaultCapacity, required: repeatRequirement(resourceRequirement(t, 1, 50, 5, math.MaxUint32, false), 1), wantErr: core.ErrPrimitiveContract, why: "file extreme"},
		{name: "width one below wave ceiling remains one wave", capacity: resourceCapacity(t, 256, 256, 256, 256, 4), required: repeatRequirement(resourceRequirement(t, 1, 1, 1, 1, false), 3), wantWidths: []uint16{3}, why: "wave-width boundary minus one"},
		{name: "width exactly at wave ceiling remains one wave", capacity: resourceCapacity(t, 256, 256, 256, 256, 4), required: repeatRequirement(resourceRequirement(t, 1, 1, 1, 1, false), 4), wantWidths: []uint16{4}, why: "wave-width boundary exact"},
		{name: "width one above wave ceiling starts another wave", capacity: resourceCapacity(t, 256, 256, 256, 256, 4), required: repeatRequirement(resourceRequirement(t, 1, 1, 1, 1, false), 5), wantWidths: []uint16{4, 1}, why: "wave-width boundary plus one"},
		{name: "maximum admitted member count closes into bounded waves", capacity: resourceCapacity(t, 256, 256, 256, 256, 4), required: repeatRequirement(resourceRequirement(t, 1, 1, 1, 1, false), runnercontrol.SchedulingMemberMaximum), wantWidths: repeatWidth(4, 64), why: "wave-width extreme"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reservations := resourceReservations(t, tc.required)
			if tc.duplicate && len(reservations) > 1 {
				reservations[1].Experiment = reservations[0].Experiment
			}
			got, gotErr := runnercontrol.PlanResourceWaves(tc.capacity, reservations)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got != nil {
					t.Fatalf("PlanResourceWaves(%s) = (%+v, %v), want nil and errors.Is %v; case exists because %s", tc.name, got, gotErr, tc.wantErr, tc.why)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("PlanResourceWaves(%s) error = %v, want nil; case exists because %s", tc.name, gotErr, tc.why)
			}
			gotWidths := make([]uint16, len(got))
			var gotExperiments []standard.ExperimentID
			for index := range got {
				gotWidths[index] = got[index].WaveWidth
				gotExperiments = append(gotExperiments, got[index].Experiments...)
			}
			if !slices.Equal(gotWidths, tc.wantWidths) {
				t.Fatalf("PlanResourceWaves(%s) wave widths = %v, want %v; case exists because %s", tc.name, gotWidths, tc.wantWidths, tc.why)
			}
			wantExperiments := make([]standard.ExperimentID, len(reservations))
			for index := range reservations {
				wantExperiments[index] = reservations[index].Experiment
			}
			if !slices.Equal(gotExperiments, wantExperiments) {
				t.Fatalf("PlanResourceWaves(%s) experiment order = %v, want %v", tc.name, gotExperiments, wantExperiments)
			}
		})
	}
}

func resourceCapacity(t testing.TB, cpu uint16, memory uint64, processes uint16, files uint32, width uint16) runnercontrol.MachineResourceCapacity {
	t.Helper()
	if memory == 0 {
		return runnercontrol.MachineResourceCapacity{CPUCount: cpu, ProcessMaximum: processes, FileMaximum: files, WaveMaximum: width}
	}
	bytes, err := core.NewByteCount(memory)
	if err != nil {
		t.Fatalf("core.NewByteCount(capacity %d) setup error = %v, want nil", memory, err)
	}
	return runnercontrol.MachineResourceCapacity{CPUCount: cpu, MemoryBytes: bytes, ProcessMaximum: processes, FileMaximum: files, WaveMaximum: width}
}

func resourceRequirement(t testing.TB, cpu uint16, memory uint64, processes uint16, files uint32, exclusive bool) runnercontrol.ResourceRequirement {
	t.Helper()
	if memory == 0 {
		return runnercontrol.ResourceRequirement{
			CPUCount: cpu, ProcessMaximum: processes, FileMaximum: files, Exclusive: exclusive,
			Egress: runnercontrol.EgressPolicy{Mode: runnercontrol.EgressDenied, DNSPolicy: core.SHA256Of([]byte("deny-all-dns"))},
		}
	}
	bytes, err := core.NewByteCount(memory)
	if err != nil {
		t.Fatalf("core.NewByteCount(requirement %d) setup error = %v, want nil", memory, err)
	}
	return runnercontrol.ResourceRequirement{
		CPUCount: cpu, MemoryBytes: bytes, ProcessMaximum: processes, FileMaximum: files, Exclusive: exclusive,
		Egress: runnercontrol.EgressPolicy{Mode: runnercontrol.EgressDenied, DNSPolicy: core.SHA256Of([]byte("deny-all-dns"))},
	}
}

func repeatRequirement(required runnercontrol.ResourceRequirement, count int) []runnercontrol.ResourceRequirement {
	got := make([]runnercontrol.ResourceRequirement, count)
	for index := range got {
		got[index] = required
	}
	return got
}

func resourceReservations(t testing.TB, requirements []runnercontrol.ResourceRequirement) []runnercontrol.ResourceReservation {
	t.Helper()
	got := make([]runnercontrol.ResourceReservation, len(requirements))
	for index := range requirements {
		text := fmt.Sprintf("01890f2e-7b00-7000-8000-%012x", index+1)
		uuid, err := primitiveid.ParseUUIDv7(text)
		if err != nil {
			t.Fatalf("id.ParseUUIDv7(%q) setup error = %v, want nil", text, err)
		}
		experiment, err := standard.NewExperimentID(uuid)
		if err != nil {
			t.Fatalf("standard.NewExperimentID(%q) setup error = %v, want nil", text, err)
		}
		got[index] = runnercontrol.ResourceReservation{Experiment: experiment, Required: requirements[index]}
	}
	return got
}

func repeatWidth(width uint16, count int) []uint16 {
	got := make([]uint16, count)
	for index := range got {
		got[index] = width
	}
	return got
}
