package release

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

type AssessLatestRequest struct {
	Latest VerifiedLatest
	Time   LatestTimeEvidence
}

// LatestTimeEvidence supplies real Temporal observations and the consumer's
// durably committed high-water for one freshness decision.
type LatestTimeEvidence struct {
	StartedAt        temporal.Observation
	ObservedAt       temporal.Observation
	DurableHighWater temporal.Instant
}

// Validate closes the wall, monotonic-elapsed, and durable clock facts.
func (e LatestTimeEvidence) Validate() error {
	if err := e.StartedAt.Validate(); err != nil {
		return latestError(err)
	}
	if err := e.ObservedAt.Validate(); err != nil {
		return latestError(err)
	}
	if err := e.DurableHighWater.Validate(); err != nil {
		return latestError(err)
	}
	if _, err := e.ObservedAt.Since(e.StartedAt); err != nil {
		return latestError(err)
	}
	return nil
}

// Validate proves the authenticated Latest value and observation instant.
func (r AssessLatestRequest) Validate() error {
	if err := r.Latest.Validate(); err != nil {
		return latestError(err)
	}
	if err := r.Time.Validate(); err != nil {
		return latestError(err)
	}
	return nil
}

// LatestAssessment is a closed, value-copy freshness projection.
type LatestAssessment struct {
	effective   temporal.Instant
	validFrom   temporal.Instant
	validUntil  temporal.Instant
	boundary    temporal.Instant
	freshness   LatestFreshness
	clockState  LatestClockState
	hasBoundary bool
	valid       bool
}

func AssessLatest(request AssessLatestRequest) (LatestAssessment, error) {
	if err := request.Validate(); err != nil {
		return LatestAssessment{}, err
	}
	fact := request.Latest.Fact()
	effective, clock, err := effectiveObservation(request.Time, fact.IssuedAt())
	if err != nil {
		return LatestAssessment{}, err
	}
	freshness, boundary, hasBoundary, err := classifyFreshness(effective, fact)
	if err != nil {
		return LatestAssessment{}, err
	}
	result := LatestAssessment{
		freshness: freshness, clockState: clock, effective: effective,
		validFrom: fact.ValidFrom(), validUntil: fact.ValidUntil(),
		boundary: boundary, hasBoundary: hasBoundary, valid: true,
	}
	if err := result.Validate(); err != nil {
		return LatestAssessment{}, err
	}
	return result, nil
}

func effectiveObservation(
	evidence LatestTimeEvidence,
	issued temporal.Instant,
) (temporal.Instant, LatestClockState, error) {
	trusted, err := trustedLatestInstant(evidence, issued)
	if err != nil {
		return temporal.Instant{}, LatestClockUnknown, err
	}
	observed, err := evidence.ObservedAt.Instant()
	if err != nil {
		return temporal.Instant{}, LatestClockUnknown, latestError(err)
	}
	return reconcileLatestObservation(observed, trusted)
}

func trustedLatestInstant(evidence LatestTimeEvidence, issued temporal.Instant) (temporal.Instant, error) {
	floor, err := maximumLatestInstant(issued, evidence.DurableHighWater)
	if err != nil {
		return temporal.Instant{}, err
	}
	elapsed, err := evidence.ObservedAt.Since(evidence.StartedAt)
	if err != nil {
		return temporal.Instant{}, latestError(err)
	}
	started, err := evidence.StartedAt.Instant()
	if err != nil {
		return temporal.Instant{}, latestError(err)
	}
	advanced, err := started.Add(elapsed)
	if err != nil {
		return temporal.Instant{}, latestError(err)
	}
	return maximumLatestInstant(floor, advanced)
}

func reconcileLatestObservation(
	observed temporal.Instant,
	trusted temporal.Instant,
) (temporal.Instant, LatestClockState, error) {
	order, err := observed.Compare(trusted)
	if err != nil {
		return temporal.Instant{}, LatestClockUnknown, latestError(err)
	}
	if order != core.ComparisonLess {
		return observed, LatestClockObserved, nil
	}
	delta, err := trusted.Since(observed)
	if err != nil || delta.Nanoseconds() > ReleaseClockRollbackToleranceNanoseconds {
		return temporal.Instant{}, LatestClockUnknown,
			latestError(errors.New("observation trails trusted time beyond tolerance"), err)
	}
	return trusted, LatestClockCorrected, nil
}

func maximumLatestInstant(
	left temporal.Instant,
	right temporal.Instant,
) (temporal.Instant, error) {
	comparison, err := left.Compare(right)
	if err != nil {
		return temporal.Instant{}, latestError(err)
	}
	if comparison == core.ComparisonLess {
		return right, nil
	}
	return left, nil
}

func classifyFreshness(
	effective temporal.Instant,
	fact LatestFact,
) (LatestFreshness, temporal.Instant, bool, error) {
	fromOrder, err := effective.Compare(fact.ValidFrom())
	if err != nil {
		return LatestFreshnessUnknown, temporal.Instant{}, false, latestError(err)
	}
	if fromOrder == core.ComparisonLess {
		return LatestFreshnessNotYetValid, fact.ValidFrom(), true, nil
	}
	untilOrder, err := effective.Compare(fact.ValidUntil())
	if err != nil {
		return LatestFreshnessUnknown, temporal.Instant{}, false, latestError(err)
	}
	if untilOrder != core.ComparisonLess {
		return LatestFreshnessExpired, temporal.Instant{}, false, nil
	}
	return LatestFreshnessCurrent, fact.ValidUntil(), true, nil
}

func (a LatestAssessment) Validate() error {
	if !a.valid {
		return latestError(errors.New("latest assessment is unset"))
	}
	for _, err := range []error{
		a.freshness.Validate(), a.clockState.Validate(), a.effective.Validate(),
		a.validFrom.Validate(), a.validUntil.Validate(),
	} {
		if err != nil {
			return latestError(err)
		}
	}
	if a.hasBoundary {
		if err := a.boundary.Validate(); err != nil {
			return latestError(err)
		}
	} else if a.boundary != (temporal.Instant{}) {
		return latestError(errors.New("assessment carries an unannounced boundary"))
	}
	return nil
}

func (a LatestAssessment) Freshness() LatestFreshness    { return a.freshness }
func (a LatestAssessment) ClockState() LatestClockState  { return a.clockState }
func (a LatestAssessment) EffectiveAt() temporal.Instant { return a.effective }
func (a LatestAssessment) ValidUntil() temporal.Instant  { return a.validUntil }
func (a LatestAssessment) Boundary() (temporal.Instant, bool) {
	return a.boundary, a.valid && a.hasBoundary
}
