package hostfacts

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// WorkloadMemoryLimitState classifies availability and limit presence.
type WorkloadMemoryLimitState uint8

const (
	WorkloadMemoryLimitUnknown WorkloadMemoryLimitState = iota
	WorkloadMemoryLimitUnsupported
	WorkloadMemoryLimitUnavailable
	WorkloadMemoryLimitUnlimited
	WorkloadMemoryLimitLimited
	workloadMemoryLimitStateLimit
)

// Validate rejects states outside the closed domain.
func (s WorkloadMemoryLimitState) Validate() error {
	if !s.IsValid() {
		return errors.Join(core.ErrHostFactsContract, errors.New("workload memory state is outside the closed domain"))
	}
	return nil
}

// IsValid reports membership in the closed state domain.
func (s WorkloadMemoryLimitState) IsValid() bool {
	return s > WorkloadMemoryLimitUnknown && s < workloadMemoryLimitStateLimit
}

// WorkloadMemoryLimitSource identifies the kernel interface used.
type WorkloadMemoryLimitSource uint8

const (
	WorkloadMemoryLimitSourceUnknown WorkloadMemoryLimitSource = iota
	WorkloadMemoryLimitSourceNone
	WorkloadMemoryLimitSourceCgroupV2
	WorkloadMemoryLimitSourceCgroupV1
	workloadMemoryLimitSourceLimit
)

// Validate rejects sources outside the closed domain.
func (s WorkloadMemoryLimitSource) Validate() error {
	if !s.IsValid() {
		return errors.Join(core.ErrHostFactsContract, errors.New("workload memory source is outside the closed domain"))
	}
	return nil
}

// IsValid reports membership in the closed source domain.
func (s WorkloadMemoryLimitSource) IsValid() bool {
	return s > WorkloadMemoryLimitSourceUnknown && s < workloadMemoryLimitSourceLimit
}

// WorkloadMemoryLimit is the effective cgroup hard limit for the current
// membership. A Limited result may carry zero bytes.
type WorkloadMemoryLimit struct {
	path   core.AbsolutePath
	limit  core.ByteLength
	state  WorkloadMemoryLimitState
	source WorkloadMemoryLimitSource
}

// Validate closes state, source, path, and value combinations.
func (l WorkloadMemoryLimit) Validate() error {
	if err := errors.Join(l.state.Validate(), l.source.Validate()); err != nil {
		return err
	}
	switch l.state {
	case WorkloadMemoryLimitUnsupported, WorkloadMemoryLimitUnavailable:
		return validateAbsentWorkloadMemoryLimit(l)
	case WorkloadMemoryLimitUnlimited:
		return validateUnlimitedWorkloadMemoryLimit(l)
	case WorkloadMemoryLimitLimited:
		return validateLimitedWorkloadMemoryLimit(l)
	default:
		return core.ErrHostFactsContract
	}
}

func validateAbsentWorkloadMemoryLimit(limit WorkloadMemoryLimit) error {
	if limit.source != WorkloadMemoryLimitSourceNone ||
		limit.path != (core.AbsolutePath{}) ||
		limit.limit.Uint64() != 0 {
		return core.ErrHostFactsContract
	}
	return nil
}

func validateUnlimitedWorkloadMemoryLimit(limit WorkloadMemoryLimit) error {
	if limit.source == WorkloadMemoryLimitSourceNone ||
		limit.path.Validate() != nil ||
		limit.limit.Uint64() != 0 {
		return core.ErrHostFactsContract
	}
	return validateWorkloadInterface(limit.path, limit.source)
}

func validateLimitedWorkloadMemoryLimit(limit WorkloadMemoryLimit) error {
	if limit.source == WorkloadMemoryLimitSourceNone || limit.path.Validate() != nil {
		return core.ErrHostFactsContract
	}
	return validateWorkloadInterface(limit.path, limit.source)
}

func validateWorkloadInterface(
	interfacePath core.AbsolutePath,
	source WorkloadMemoryLimitSource,
) error {
	base, err := interfacePath.Base()
	if err != nil {
		return errors.Join(core.ErrHostFactsContract, err)
	}
	want := cgroupV2LimitName
	if source == WorkloadMemoryLimitSourceCgroupV1 {
		want = cgroupV1LimitName
	}
	if base.String() != want {
		return errors.Join(core.ErrHostFactsContract, errors.New("workload memory source contradicts interface path"))
	}
	return nil
}

// State returns the availability and limit state.
func (l WorkloadMemoryLimit) State() WorkloadMemoryLimitState {
	return l.state
}

// Source returns the kernel interface source.
func (l WorkloadMemoryLimit) Source() WorkloadMemoryLimitSource {
	return l.source
}

// LimitBytes returns a present limit, including a valid zero-byte limit.
func (l WorkloadMemoryLimit) LimitBytes() (core.ByteLength, bool) {
	return l.limit, l.state == WorkloadMemoryLimitLimited
}

// InterfacePath returns the exact interface file that supplied the effective
// finite limit, or the closest present interface when every declaration is
// unlimited.
func (l WorkloadMemoryLimit) InterfacePath() (core.AbsolutePath, bool) {
	return l.path, l.state == WorkloadMemoryLimitLimited || l.state == WorkloadMemoryLimitUnlimited
}

func validateWorkloadMemoryLimit(result WorkloadMemoryLimit) (WorkloadMemoryLimit, error) {
	return result, result.Validate()
}

// unavailableWorkloadMemoryLimit is the fact for a host that supports the
// interface but exposes no memory ceiling to observe, either because the process
// has no memory-controlled cgroup membership or because no level of its
// membership declares a memory interface at all.
func unavailableWorkloadMemoryLimit() (WorkloadMemoryLimit, error) {
	return validateWorkloadMemoryLimit(WorkloadMemoryLimit{
		state:  WorkloadMemoryLimitUnavailable,
		source: WorkloadMemoryLimitSourceNone,
	})
}

func unsupportedWorkloadMemoryLimit() (WorkloadMemoryLimit, error) {
	return validateWorkloadMemoryLimit(WorkloadMemoryLimit{
		state:  WorkloadMemoryLimitUnsupported,
		source: WorkloadMemoryLimitSourceNone,
	})
}
