package hostfacts

import (
	"errors"
	"math/bits"

	"github.com/deliri/primitive/v2026/core"
)

// DiskPressurePolicy is the caller-owned available-space floor. A zero floor
// disables pressure classification without disabling capacity observation.
type DiskPressurePolicy struct {
	FreeSpaceFloor core.ByteLength
}

// Validate accepts the complete unsigned byte-length domain.
func (DiskPressurePolicy) Validate() error {
	return nil
}

// DiskAssessmentRequest binds one exact directory to one caller policy.
type DiskAssessmentRequest struct {
	Directory core.AbsolutePath
	Policy    DiskPressurePolicy
}

// Validate rejects an unset or malformed directory and invalid policy.
func (r DiskAssessmentRequest) Validate() error {
	if err := r.Directory.Validate(); err != nil {
		return errors.Join(core.ErrHostFactsContract, err)
	}
	return r.Policy.Validate()
}

// DiskCapacity is caller-available and total capacity for one held directory.
type DiskCapacity struct {
	available core.ByteLength
	total     core.ByteCount
}

func newDiskCapacity(available, total uint64) (DiskCapacity, error) {
	totalBytes, err := core.NewByteCount(total)
	if err != nil || available > total {
		return DiskCapacity{}, errors.Join(
			core.ErrHostFactsObservation,
			err,
			errors.New("disk capacity is internally contradictory"),
		)
	}
	capacity := DiskCapacity{available: core.NewByteLength(available), total: totalBytes}
	return capacity, capacity.Validate()
}

// Validate rejects an unset total or caller-available bytes above total.
func (c DiskCapacity) Validate() error {
	total, err := c.total.Uint64()
	if err != nil || c.available.Uint64() > total {
		return errors.Join(core.ErrHostFactsObservation, err)
	}
	return nil
}

// AvailableBytes returns capacity available to the current caller.
func (c DiskCapacity) AvailableBytes() core.ByteLength {
	return c.available
}

// TotalBytes returns the filesystem's total byte capacity.
func (c DiskCapacity) TotalBytes() core.ByteCount {
	return c.total
}

// DiskPressureState classifies caller-available capacity against a floor.
type DiskPressureState uint8

const (
	DiskPressureUnknown DiskPressureState = iota
	DiskPressureDisabled
	DiskPressureHealthy
	DiskPressureReached
	diskPressureLimit
)

// Validate rejects states outside the closed domain.
func (s DiskPressureState) Validate() error {
	if !s.IsValid() {
		return errors.Join(core.ErrHostFactsContract, errors.New("disk pressure state is outside the closed domain"))
	}
	return nil
}

// IsValid reports membership in the closed pressure domain.
func (s DiskPressureState) IsValid() bool {
	return s > DiskPressureUnknown && s < diskPressureLimit
}

// DiskAssessment is the validated capacity, policy, and classification.
type DiskAssessment struct {
	capacity DiskCapacity
	policy   DiskPressurePolicy
	state    DiskPressureState
}

// Validate closes the assessment shape.
func (a DiskAssessment) Validate() error {
	if err := errors.Join(a.capacity.Validate(), a.policy.Validate(), a.state.Validate()); err != nil {
		return err
	}
	if a.state != classifyDiskPressure(a.capacity, a.policy) {
		return errors.Join(core.ErrHostFactsContract, errors.New("disk assessment state contradicts capacity and policy"))
	}
	return nil
}

// Capacity returns the observed capacity.
func (a DiskAssessment) Capacity() DiskCapacity {
	return a.capacity
}

// Policy returns the caller policy used for classification.
func (a DiskAssessment) Policy() DiskPressurePolicy {
	return a.policy
}

// State returns the closed pressure state.
func (a DiskAssessment) State() DiskPressureState {
	return a.state
}

func assessDiskCapacity(capacity DiskCapacity, policy DiskPressurePolicy) (DiskAssessment, error) {
	if err := errors.Join(capacity.Validate(), policy.Validate()); err != nil {
		return DiskAssessment{}, err
	}
	state := classifyDiskPressure(capacity, policy)
	assessment := DiskAssessment{capacity: capacity, policy: policy, state: state}
	if err := assessment.Validate(); err != nil {
		return DiskAssessment{}, err
	}
	if state == DiskPressureReached {
		return assessment, core.ErrDiskFloorReached
	}
	return assessment, nil
}

func classifyDiskPressure(
	capacity DiskCapacity,
	policy DiskPressurePolicy,
) DiskPressureState {
	state := DiskPressureHealthy
	floor := policy.FreeSpaceFloor.Uint64()
	if floor == 0 {
		state = DiskPressureDisabled
	} else if capacity.available.Uint64() <= floor {
		state = DiskPressureReached
	}
	return state
}

func blocksToBytes(blocks, blockBytes uint64) (uint64, error) {
	high, low := bits.Mul64(blocks, blockBytes)
	if high != 0 {
		return 0, core.ErrNumericOverflow
	}
	return low, nil
}
