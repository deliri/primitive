package temporal

import (
	"time"

	"github.com/deliri/primitive/v2026/core"
)

// Observation preserves one real time.Time, including its monotonic reading.
type Observation struct {
	value time.Time
}

// Observe captures the real standard-library clock once.
func Observe() (Observation, error) {
	return NewObservation(time.Now())
}

// NewObservation constructs an observation from a caller-supplied time.Time.
func NewObservation(value time.Time) (Observation, error) {
	if _, err := NewInstant(value); err != nil {
		return Observation{}, contractError("observation wall instant is invalid", err)
	}
	return Observation{value: value}, nil
}

// Validate rejects observations whose wall instant is not representable.
func (o Observation) Validate() error {
	if _, err := NewInstant(o.value); err != nil {
		return contractError("observation is invalid", err)
	}
	return nil
}

// Instant projects the observation's wall reading to exact Unix nanoseconds.
func (o Observation) Instant() (Instant, error) {
	if err := o.Validate(); err != nil {
		return Instant{}, err
	}
	return NewInstant(o.value)
}

// Since returns elapsed time using time.Time's monotonic reading when both
// observations carry one. It rejects time.Sub saturation.
func (o Observation) Since(earlier Observation) (Duration, error) {
	if err := o.Validate(); err != nil {
		return Duration{}, err
	}
	if err := earlier.Validate(); err != nil {
		return Duration{}, err
	}
	elapsed := o.value.Sub(earlier.value)
	if elapsed < 0 {
		return Duration{}, contractError("observation precedes the requested earlier observation")
	}
	if !earlier.value.Add(elapsed).Equal(o.value) {
		return Duration{}, overflowError("observation difference exceeded bounded duration")
	}
	return DurationFromNanoseconds(int64(elapsed))
}

var _ core.Validatable = Observation{}
