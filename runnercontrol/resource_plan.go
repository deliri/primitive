package runnercontrol

import (
	"bytes"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runprotocol"
)

type MachineResourceCapacity struct {
	MemoryBytes    core.ByteCount `json:"memory_bytes"`
	FileMaximum    uint32         `json:"file_maximum"`
	CPUCount       uint16         `json:"cpu_count"`
	ProcessMaximum uint16         `json:"process_maximum"`
	WaveMaximum    uint16         `json:"wave_maximum"`
}

func (c MachineResourceCapacity) Validate() error {
	if err := c.MemoryBytes.Validate(); err != nil {
		return err
	}
	memory, err := c.MemoryBytes.Uint64()
	if err != nil || memory == 0 || c.CPUCount == 0 || c.ProcessMaximum == 0 || c.FileMaximum == 0 || c.WaveMaximum == 0 || c.WaveMaximum > SchedulingMemberMaximum {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	return nil
}

type ResourceReservation struct {
	Required   ResourceRequirement      `json:"required"`
	Experiment runprotocol.ExperimentID `json:"experiment_id"`
}

func (r ResourceReservation) Validate() error {
	return errors.Join(r.Experiment.Validate(), r.Required.Validate())
}

// PlanResourceWaves deterministically packs the admitted reservation order.
// It never reorders measurements to improve utilization, and exclusive work
// always owns a wave by itself.
func PlanResourceWaves(capacity MachineResourceCapacity, reservations []ResourceReservation) ([]ResourceWave, error) {
	if err := validateReservations(capacity, reservations); err != nil {
		return nil, err
	}
	waves := make([]ResourceWave, 0, len(reservations))
	current := waveAccumulator{}
	for _, reservation := range reservations {
		var err error
		waves, err = current.plan(waves, capacity, reservation)
		if err != nil {
			return nil, err
		}
	}
	var err error
	waves, err = appendAccumulatedWave(waves, current)
	if err != nil {
		return nil, err
	}
	for index := range waves {
		if err := waves[index].Validate(); err != nil {
			return nil, err
		}
	}
	return waves, nil
}

func (w *waveAccumulator) plan(waves []ResourceWave, capacity MachineResourceCapacity, reservation ResourceReservation) ([]ResourceWave, error) {
	if reservation.Required.Exclusive {
		flushed, err := appendAccumulatedWave(waves, *w)
		if err != nil {
			return nil, err
		}
		*w = waveAccumulator{}
		return append(flushed, oneReservationWave(reservation)), nil
	}
	if !w.canAdd(capacity, reservation.Required) {
		var err error
		waves, err = appendAccumulatedWave(waves, *w)
		if err != nil {
			return nil, err
		}
		*w = waveAccumulator{}
	}
	if err := w.add(reservation); err != nil {
		return nil, err
	}
	return waves, nil
}

func validateReservations(capacity MachineResourceCapacity, reservations []ResourceReservation) error {
	if err := capacity.Validate(); err != nil {
		return err
	}
	if len(reservations) == 0 || len(reservations) > SchedulingMemberMaximum {
		return core.ErrPrimitiveContract
	}
	for index := range reservations {
		if err := reservations[index].Validate(); err != nil || !reservationFits(capacity, reservations[index].Required) {
			return errors.Join(core.ErrPrimitiveContract, err)
		}
		for previous := range index {
			if reservations[previous].Experiment == reservations[index].Experiment {
				return core.ErrPrimitiveContract
			}
		}
	}
	return nil
}

func reservationFits(capacity MachineResourceCapacity, required ResourceRequirement) bool {
	requiredMemory, requiredErr := required.MemoryBytes.Uint64()
	capacityMemory, capacityErr := capacity.MemoryBytes.Uint64()
	if requiredErr != nil || capacityErr != nil {
		return false
	}
	return required.CPUCount <= capacity.CPUCount &&
		requiredMemory <= capacityMemory &&
		required.ProcessMaximum <= capacity.ProcessMaximum &&
		required.FileMaximum <= capacity.FileMaximum
}

type waveAccumulator struct {
	experiments []runprotocol.ExperimentID
	required    ResourceRequirement
}

func (w waveAccumulator) canAdd(capacity MachineResourceCapacity, next ResourceRequirement) bool {
	if len(w.experiments) == 0 {
		return true
	}
	if len(w.experiments) >= int(capacity.WaveMaximum) || next.Exclusive || !sameEgress(w.required.Egress, next.Egress) {
		return false
	}
	return aggregateFits(capacity, w.required, next)
}

func aggregateFits(capacity MachineResourceCapacity, current, next ResourceRequirement) bool {
	cpu := uint32(current.CPUCount) + uint32(next.CPUCount)
	processes := uint32(current.ProcessMaximum) + uint32(next.ProcessMaximum)
	files := uint64(current.FileMaximum) + uint64(next.FileMaximum)
	currentMemory, currentErr := current.MemoryBytes.Uint64()
	nextMemory, nextErr := next.MemoryBytes.Uint64()
	capacityMemory, capacityErr := capacity.MemoryBytes.Uint64()
	if currentErr != nil || nextErr != nil || capacityErr != nil {
		return false
	}
	memory := currentMemory + nextMemory
	if memory < currentMemory {
		return false
	}
	if cpu > uint32(capacity.CPUCount) || processes > uint32(capacity.ProcessMaximum) {
		return false
	}
	return files <= uint64(capacity.FileMaximum) && memory <= capacityMemory
}

func (w *waveAccumulator) add(reservation ResourceReservation) error {
	if len(w.experiments) == 0 {
		w.required = reservation.Required
		w.experiments = append(w.experiments, reservation.Experiment)
		return nil
	}
	currentMemory, currentErr := w.required.MemoryBytes.Uint64()
	nextMemory, nextErr := reservation.Required.MemoryBytes.Uint64()
	memory, memoryErr := core.NewByteCount(currentMemory + nextMemory)
	if err := errors.Join(currentErr, nextErr, memoryErr); err != nil {
		return err
	}
	w.required.CPUCount += reservation.Required.CPUCount
	w.required.MemoryBytes = memory
	w.required.ProcessMaximum += reservation.Required.ProcessMaximum
	w.required.FileMaximum += reservation.Required.FileMaximum
	w.experiments = append(w.experiments, reservation.Experiment)
	return nil
}

func appendAccumulatedWave(waves []ResourceWave, accumulated waveAccumulator) ([]ResourceWave, error) {
	if len(accumulated.experiments) == 0 {
		return waves, nil
	}
	waveWidth, err := core.CheckedUint16FromInt(len(accumulated.experiments))
	if err != nil {
		return nil, errors.Join(core.ErrPrimitiveContract, err)
	}
	return append(waves, ResourceWave{
		Experiments: append([]runprotocol.ExperimentID(nil), accumulated.experiments...),
		Required:    accumulated.required, WaveWidth: waveWidth,
	}), nil
}

func oneReservationWave(reservation ResourceReservation) ResourceWave {
	return ResourceWave{Experiments: []runprotocol.ExperimentID{reservation.Experiment}, Required: reservation.Required, WaveWidth: 1}
}

func sameEgress(left, right EgressPolicy) bool {
	leftBytes, leftErr := core.MarshalCanonicalJSONDocument(left)
	rightBytes, rightErr := core.MarshalCanonicalJSONDocument(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftBytes, rightBytes)
}

var (
	_ core.Validatable = MachineResourceCapacity{}
	_ core.Validatable = ResourceReservation{}
)
