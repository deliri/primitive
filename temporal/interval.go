package temporal

import (
	"github.com/deliri/primitive/v2026/core"
)

const intervalStartInvalidReason = "interval start is invalid"

// IntervalRequest supplies two observations for one elapsed interval.
type IntervalRequest struct {
	Start  Observation
	Finish Observation
}

// Validate checks both observation boundaries and their ordering.
func (r IntervalRequest) Validate() error {
	if err := r.Start.Validate(); err != nil {
		return contractError(intervalStartInvalidReason, err)
	}
	if err := r.Finish.Validate(); err != nil {
		return contractError("interval finish is invalid", err)
	}
	if _, err := r.Finish.Since(r.Start); err != nil {
		return contractError("interval observations are invalid", err)
	}
	return nil
}

// IntervalBounds supplies exact persisted wall bounds.
type IntervalBounds struct {
	Start Instant
	End   Instant
}

// Validate checks both exact bounds and their ordering.
func (b IntervalBounds) Validate() error {
	if err := b.Start.Validate(); err != nil {
		return contractError("interval start bound is invalid", err)
	}
	if err := b.End.Validate(); err != nil {
		return contractError("interval end bound is invalid", err)
	}
	if _, err := b.End.Since(b.Start); err != nil {
		return contractError("interval bounds are invalid", err)
	}
	return nil
}

// Interval is a start, derived end, and exact nonnegative elapsed duration.
type Interval struct {
	start   Instant
	end     Instant
	elapsed Duration
}

// NewInterval constructs an interval from observations, preserving monotonic
// elapsed time and deriving the end from start plus elapsed.
func NewInterval(request IntervalRequest) (Interval, error) {
	if err := request.Validate(); err != nil {
		return Interval{}, err
	}
	start, err := request.Start.Instant()
	if err != nil {
		return Interval{}, err
	}
	elapsed, err := request.Finish.Since(request.Start)
	if err != nil {
		return Interval{}, err
	}
	end, err := start.Add(elapsed)
	if err != nil {
		return Interval{}, err
	}
	return Interval{start: start, end: end, elapsed: elapsed}, nil
}

// IntervalFromBounds constructs an interval from exact wall bounds.
func IntervalFromBounds(bounds IntervalBounds) (Interval, error) {
	if err := bounds.Validate(); err != nil {
		return Interval{}, err
	}
	elapsed, err := bounds.End.Since(bounds.Start)
	if err != nil {
		return Interval{}, err
	}
	return Interval{start: bounds.Start, end: bounds.End, elapsed: elapsed}, nil
}

// Validate checks the interval's exact arithmetic invariant.
func (i Interval) Validate() error {
	if err := i.start.Validate(); err != nil {
		return contractError(intervalStartInvalidReason, err)
	}
	if err := i.end.Validate(); err != nil {
		return contractError("interval end is invalid", err)
	}
	if err := i.elapsed.Validate(); err != nil {
		return contractError("interval elapsed duration is invalid", err)
	}
	end, err := i.start.Add(i.elapsed)
	if err != nil {
		return contractError("interval arithmetic is invalid", err)
	}
	comparison, err := end.Compare(i.end)
	if err != nil || comparison != core.ComparisonEqual {
		return contractError("interval end does not equal start plus elapsed", err)
	}
	return nil
}

// Start returns the exact start instant.
func (i Interval) Start() (Instant, error) {
	if err := i.Validate(); err != nil {
		return Instant{}, err
	}
	return i.start, nil
}

// End returns the exact derived end instant.
func (i Interval) End() (Instant, error) {
	if err := i.Validate(); err != nil {
		return Instant{}, err
	}
	return i.end, nil
}

// Elapsed returns the exact nonnegative duration.
func (i Interval) Elapsed() (Duration, error) {
	if err := i.Validate(); err != nil {
		return Duration{}, err
	}
	return i.elapsed, nil
}

// Bounds returns exact persisted wall bounds.
func (i Interval) Bounds() (IntervalBounds, error) {
	if err := i.Validate(); err != nil {
		return IntervalBounds{}, err
	}
	return IntervalBounds{Start: i.start, End: i.end}, nil
}

var (
	_ core.Validatable = IntervalRequest{}
	_ core.Validatable = IntervalBounds{}
	_ core.Validatable = Interval{}
)
