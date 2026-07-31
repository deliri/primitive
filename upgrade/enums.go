package upgrade

import (
	"encoding/json"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// Slot is one of the two fixed installation namespaces.
type Slot uint8

const (
	SlotUnknown Slot = iota
	SlotA
	SlotB
	slotLimit
)

// slotLabels is also the on-wire token set and the slot directory name set. An
// invalid slot deliberately projects the empty string, which no path component
// admits, so a slot that escapes validation cannot name a directory.
var slotLabels = [...]string{
	SlotUnknown: "",
	SlotA:       "slot-a",
	SlotB:       "slot-b",
}

var (
	_ [int(slotLimit) - len(slotLabels)]struct{}
	_ [len(slotLabels) - int(slotLimit)]struct{}
)

func (s Slot) Validate() error {
	if s <= SlotUnknown || s >= slotLimit {
		return contractError(diagnosticSlot)
	}
	return nil
}

func (s Slot) IsValid() bool { return s.Validate() == nil }

func (s Slot) String() string {
	if !s.IsValid() {
		return slotLabels[SlotUnknown]
	}
	return slotLabels[s]
}

func (s Slot) other() (Slot, error) {
	switch s {
	case SlotA:
		return SlotB, nil
	case SlotB:
		return SlotA, nil
	default:
		return SlotUnknown, contractError(diagnosticSlot)
	}
}

func (s Slot) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return json.Marshal(s.String())
}

func (s *Slot) UnmarshalJSON(data []byte) error {
	if s == nil {
		return errors.Join(core.ErrJSONContract, contractError(diagnosticSlot))
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return contractError(errors.Join(core.ErrJSONContract, err))
	}
	var candidate Slot
	switch value {
	case SlotA.String():
		candidate = SlotA
	case SlotB.String():
		candidate = SlotB
	default:
		return contractError(errors.Join(core.ErrJSONContract, diagnosticSlot))
	}
	*s = candidate
	return nil
}

// TrialOutcome is the product-owned result of running the exact candidate.
type TrialOutcome uint8

const (
	TrialOutcomeUnknown TrialOutcome = iota
	TrialPassed
	TrialFailed
	trialOutcomeLimit
)

var trialOutcomeLabels = [...]string{
	TrialOutcomeUnknown: "",
	TrialPassed:         "passed",
	TrialFailed:         "failed",
}

var (
	_ [int(trialOutcomeLimit) - len(trialOutcomeLabels)]struct{}
	_ [len(trialOutcomeLabels) - int(trialOutcomeLimit)]struct{}
)

func (o TrialOutcome) Validate() error {
	if o <= TrialOutcomeUnknown || o >= trialOutcomeLimit {
		return contractError(diagnosticTrialOutcome)
	}
	return nil
}

func (o TrialOutcome) IsValid() bool { return o.Validate() == nil }
func (TrialOutcome) OffWireEnum()    {}

func (o TrialOutcome) String() string {
	if !o.IsValid() {
		return trialOutcomeLabels[TrialOutcomeUnknown]
	}
	return trialOutcomeLabels[o]
}

// FailurePhase identifies the exact Upgrade boundary that failed.
type FailurePhase uint8

const (
	FailurePhaseUnknown FailurePhase = iota
	FailurePhaseBootstrap
	FailurePhaseCapacity
	FailurePhaseDownload
	FailurePhaseVerification
	FailurePhaseTrial
	FailurePhasePromotion
	FailurePhasePersistence
	FailurePhaseCleanup
	failurePhaseLimit
)

var failurePhaseLabels = [...]string{
	FailurePhaseUnknown:      "",
	FailurePhaseBootstrap:    "bootstrap",
	FailurePhaseCapacity:     "capacity",
	FailurePhaseDownload:     "download",
	FailurePhaseVerification: "verification",
	FailurePhaseTrial:        "trial",
	FailurePhasePromotion:    "promotion",
	FailurePhasePersistence:  "persistence",
	FailurePhaseCleanup:      "cleanup",
}

var (
	_ [int(failurePhaseLimit) - len(failurePhaseLabels)]struct{}
	_ [len(failurePhaseLabels) - int(failurePhaseLimit)]struct{}
)

func (p FailurePhase) Validate() error {
	if p <= FailurePhaseUnknown || p >= failurePhaseLimit {
		return contractError(diagnosticFailurePhase)
	}
	return nil
}

func (p FailurePhase) IsValid() bool { return p.Validate() == nil }
func (FailurePhase) OffWireEnum()    {}

func (p FailurePhase) String() string {
	if !p.IsValid() {
		return failurePhaseLabels[FailurePhaseUnknown]
	}
	return failurePhaseLabels[p]
}

var (
	_ core.OffWireEnum = TrialOutcomeUnknown
	_ core.OffWireEnum = FailurePhaseUnknown
)
