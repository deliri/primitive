package hostfacts

import (
	"errors"
	"math"
	"math/bits"

	"github.com/deliri/primitive/v2026/core"
)

const percentDenominator = 100

// Percent is a Hostfacts-owned whole percentage in the closed range 1..100.
type Percent struct {
	value uint8
}

// NewPercent constructs a validated whole percentage.
func NewPercent(value uint8) (Percent, error) {
	percent := Percent{value: value}
	if err := percent.Validate(); err != nil {
		return Percent{}, err
	}
	return percent, nil
}

// Validate rejects zero and values above 100.
func (p Percent) Validate() error {
	if p.value == 0 || p.value > percentDenominator {
		return errors.Join(core.ErrHostFactsContract, errors.New("percentage must be between 1 and 100"))
	}
	return nil
}

// Uint8 returns the validated percentage.
func (p Percent) Uint8() (uint8, error) {
	return p.value, p.Validate()
}

// GoMemoryPressurePolicy owns the caller's Go-memory trigger percentage.
type GoMemoryPressurePolicy struct {
	TriggerPercent Percent
}

// Validate rejects an invalid percentage.
func (p GoMemoryPressurePolicy) Validate() error {
	return p.TriggerPercent.Validate()
}

// TriggerBytes projects the policy over a positive observed limit using exact
// upward rounding.
func (p GoMemoryPressurePolicy) TriggerBytes(limit core.ByteCount) (core.ByteCount, error) {
	if err := p.Validate(); err != nil {
		return core.ByteCount{}, err
	}
	value, err := limit.Uint64()
	if err != nil {
		return core.ByteCount{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	percent, _ := p.TriggerPercent.Uint8()
	high, low := bits.Mul64(value, uint64(percent))
	quotient, remainder := bits.Div64(high, low, percentDenominator)
	if remainder != 0 {
		quotient++
	}
	trigger, err := core.NewByteCount(quotient)
	if err != nil {
		return core.ByteCount{}, errors.Join(core.ErrHostFactsContract, err)
	}
	return trigger, nil
}

// GoMemoryAssessmentRequest carries one caller policy.
type GoMemoryAssessmentRequest struct {
	Policy GoMemoryPressurePolicy
}

// Validate rejects an invalid policy.
func (r GoMemoryAssessmentRequest) Validate() error {
	return r.Policy.Validate()
}

// GoMemorySnapshot is the exact Go runtime memory-limit accounting fact.
type GoMemorySnapshot struct {
	system       core.ByteCount
	heapReleased core.ByteLength
	managed      core.ByteLength
	limit        core.ByteCount
}

func newGoMemorySnapshot(system, heapReleased uint64, limit int64) (GoMemorySnapshot, error) {
	if system == 0 || heapReleased > system || limit <= 0 {
		return GoMemorySnapshot{}, errors.Join(
			core.ErrHostFactsObservation,
			errors.New("go memory snapshot is internally contradictory"),
		)
	}
	systemBytes, systemErr := core.NewByteCount(system)
	limitBytes, limitErr := core.NewByteCount(uint64(limit))
	if err := errors.Join(systemErr, limitErr); err != nil {
		return GoMemorySnapshot{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	releasedBytes, releasedErr := core.NewByteLength(heapReleased)
	managedBytes, managedErr := core.NewByteLength(system - heapReleased)
	if err := errors.Join(releasedErr, managedErr); err != nil {
		return GoMemorySnapshot{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	snapshot := GoMemorySnapshot{
		system:       systemBytes,
		heapReleased: releasedBytes,
		managed:      managedBytes,
		limit:        limitBytes,
	}
	return snapshot, snapshot.Validate()
}

// Validate closes every arithmetic relationship in the snapshot.
func (s GoMemorySnapshot) Validate() error {
	system, systemErr := s.system.Uint64()
	limit, limitErr := s.limit.Uint64()
	if errors.Join(systemErr, limitErr, s.heapReleased.Validate(), s.managed.Validate()) != nil ||
		s.heapReleased.Uint64() > system ||
		s.managed.Uint64() != system-s.heapReleased.Uint64() ||
		limit > math.MaxInt64 {
		return errors.Join(core.ErrHostFactsObservation, systemErr, limitErr)
	}
	return nil
}

// SystemBytes returns runtime.MemStats.Sys.
func (s GoMemorySnapshot) SystemBytes() core.ByteCount {
	return s.system
}

// HeapReleasedBytes returns runtime.MemStats.HeapReleased.
func (s GoMemorySnapshot) HeapReleasedBytes() core.ByteLength {
	return s.heapReleased
}

// ManagedBytes returns Sys minus HeapReleased.
func (s GoMemorySnapshot) ManagedBytes() core.ByteLength {
	return s.managed
}

// LimitBytes returns the current Go soft memory limit.
func (s GoMemorySnapshot) LimitBytes() core.ByteCount {
	return s.limit
}

// MemoryPressureState classifies Go-managed bytes against a projected trigger.
type MemoryPressureState uint8

const (
	MemoryPressureUnknown MemoryPressureState = iota
	MemoryPressureDisabled
	MemoryPressureHealthy
	MemoryPressureReached
	memoryPressureLimit
)

func memoryPressureStateLabels() [memoryPressureLimit]string {
	return [...]string{
		MemoryPressureDisabled: pressureDisabledLabel,
		MemoryPressureHealthy:  pressureHealthyLabel,
		MemoryPressureReached:  pressureReachedLabel,
	}
}

// Validate rejects states outside the closed domain.
func (s MemoryPressureState) Validate() error {
	if !s.IsValid() {
		return errors.Join(core.ErrHostFactsContract, errors.New("memory pressure state is outside the closed domain"))
	}
	return nil
}

// IsValid reports membership in the closed pressure domain.
func (s MemoryPressureState) IsValid() bool {
	return s > MemoryPressureUnknown && s < memoryPressureLimit &&
		memoryPressureStateLabels()[s] != ""
}

// OffWireEnum declares MemoryPressureState as a runtime assessment rather than
// a wire encoding.
func (MemoryPressureState) OffWireEnum() {}

// String returns the compiler-owned diagnostic label for s.
func (s MemoryPressureState) String() string {
	if !s.IsValid() {
		return unknownOperationText
	}
	return memoryPressureStateLabels()[s]
}

// GoMemoryAssessment is a validated snapshot, trigger, policy, and state.
type GoMemoryAssessment struct {
	snapshot GoMemorySnapshot
	trigger  core.ByteCount
	policy   GoMemoryPressurePolicy
	state    MemoryPressureState
}

// Validate closes the assessment shape and independently reprojects trigger.
func (a GoMemoryAssessment) Validate() error {
	if err := errors.Join(a.snapshot.Validate(), a.policy.Validate(), a.state.Validate(), a.trigger.Validate()); err != nil {
		return err
	}
	trigger, err := a.policy.TriggerBytes(a.snapshot.limit)
	if err != nil || trigger != a.trigger {
		return errors.Join(core.ErrHostFactsContract, err)
	}
	if a.state != classifyMemoryPressure(a.snapshot, a.trigger) {
		return errors.Join(core.ErrHostFactsContract, errors.New("memory assessment state contradicts snapshot and trigger"))
	}
	return nil
}

// Snapshot returns the observed Go runtime facts.
func (a GoMemoryAssessment) Snapshot() GoMemorySnapshot {
	return a.snapshot
}

// TriggerBytes returns the projected trigger.
func (a GoMemoryAssessment) TriggerBytes() core.ByteCount {
	return a.trigger
}

// Policy returns the caller policy.
func (a GoMemoryAssessment) Policy() GoMemoryPressurePolicy {
	return a.policy
}

// State returns the closed pressure state.
func (a GoMemoryAssessment) State() MemoryPressureState {
	return a.state
}

func assessGoMemorySnapshot(snapshot GoMemorySnapshot, policy GoMemoryPressurePolicy) (GoMemoryAssessment, error) {
	trigger, err := policy.TriggerBytes(snapshot.limit)
	if err != nil {
		return GoMemoryAssessment{}, err
	}
	state := classifyMemoryPressure(snapshot, trigger)
	assessment := GoMemoryAssessment{snapshot: snapshot, trigger: trigger, policy: policy, state: state}
	if err := assessment.Validate(); err != nil {
		return GoMemoryAssessment{}, err
	}
	if state == MemoryPressureReached {
		return assessment, core.ErrMemoryLimitReached
	}
	return assessment, nil
}

func classifyMemoryPressure(
	snapshot GoMemorySnapshot,
	trigger core.ByteCount,
) MemoryPressureState {
	limit, _ := snapshot.limit.Uint64()
	if limit == math.MaxInt64 {
		return MemoryPressureDisabled
	}
	if snapshot.managed.Uint64() >= mustByteCountValue(trigger) {
		return MemoryPressureReached
	}
	return MemoryPressureHealthy
}

func mustByteCountValue(value core.ByteCount) uint64 {
	result, _ := value.Uint64()
	return result
}
