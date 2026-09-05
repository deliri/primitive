package sourceobservation

import (
	"errors"
	"github.com/deliri/primitive/v2026/capabilities"
	"math"
)

// CallCoverage retains completeness separately from an empty effect list.
// The zero value is valid unobserved knowledge, never completed analysis.
type CallCoverage struct {
	State      CallCoverageState `json:"state"`
	Effects    uint64            `json:"effects"`
	Pure       uint64            `json:"pure"`
	Contextual uint64            `json:"contextual"`
	Unresolved uint64            `json:"unresolved"`
}

func (c CallCoverage) Validate() error {
	counts := []uint64{c.Effects, c.Pure, c.Contextual, c.Unresolved}
	total := uint64(0)
	for _, count := range counts {
		if count > math.MaxUint64-total {
			return conflictError(errors.New(catalogCallCoverageCountOverflows))
		}
		total += count
	}
	if err := c.State.Validate(); err != nil {
		return err
	}
	if c.State == CallCoverageUnobserved && total != 0 {
		return conflictError(errors.New("unobserved call coverage carries counts"))
	}
	return nil
}

// Add admits one real call classification into a compiler-observed census.
func (c *CallCoverage) Add(fact capabilities.Classification) error {
	if c == nil {
		return contractError(errors.New("call coverage receiver is nil"))
	}
	if err := errors.Join(c.Validate(), fact.Validate()); err != nil {
		return err
	}
	if c.State == CallCoverageUnobserved {
		return conflictError(errors.New("call coverage was not opened by the compiler"))
	}
	candidate := *c
	counter, err := candidate.counter(fact.Disposition)
	if err != nil {
		return err
	}
	if *counter == math.MaxUint64 {
		return conflictError(errors.New(catalogCallCoverageCountOverflows))
	}
	*counter++
	if err := candidate.Validate(); err != nil {
		return err
	}
	*c = candidate
	return nil
}

func (CallCoverage) sourceObservationProtocolFact() {}

func (c *CallCoverage) counter(disposition capabilities.StandardSymbolDisposition) (*uint64, error) {
	switch disposition {
	case capabilities.StandardSymbolEffect:
		return &c.Effects, nil
	case capabilities.StandardSymbolPure:
		return &c.Pure, nil
	case capabilities.StandardSymbolContextual:
		return &c.Contextual, nil
	case capabilities.StandardSymbolUnresolved:
		return &c.Unresolved, nil
	default:
		return nil, contractError(errors.New("call coverage disposition is not admitted"))
	}
}

// Complete reports classification closure within the observed call scope only.
// It does not prove absence of runtime effects such as initializers or callbacks.
func (c CallCoverage) Complete() bool {
	return c.Validate() == nil && c.State == CallCoverageObserved && c.Contextual == 0 && c.Unresolved == 0
}

// CallCoverageState distinguishes absent, partial and complete compiler census.
const callCoverageStateErrorText = "call coverage state is unknown"

type CallCoverageState uint8

const (
	CallCoverageUnobserved CallCoverageState = iota
	CallCoveragePartial
	CallCoverageObserved
	callCoverageStateLimit
)

func (s CallCoverageState) Validate() error {
	if s >= callCoverageStateLimit {
		return contractError(errors.New(callCoverageStateErrorText))
	}
	return nil
}
func (s CallCoverageState) IsValid() bool { return s.Validate() == nil }
func (s CallCoverageState) String() string {
	if !s.IsValid() {
		return ""
	}
	return [...]string{"unobserved", "partial", "observed"}[s]
}
func (s CallCoverageState) MarshalJSON() ([]byte, error) {
	return marshalSourceEnum(s)
}
func (s *CallCoverageState) UnmarshalJSON(data []byte) error {
	return unmarshalSourceEnum(data, s, [...]CallCoverageState{CallCoverageUnobserved, callCoverageStateLimit})
}
