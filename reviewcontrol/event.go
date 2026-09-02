package reviewcontrol

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/proofledger"
)

type DecisionSuperseded struct {
	Previous proofledger.EventIdentity `json:"previous"`
	Reason   DecisionReason            `json:"reason"`
}

func (s DecisionSuperseded) Validate() error {
	return validateContract(s.Previous.Validate(), s.Reason.Validate())
}

type EventPayload struct {
	Kind         EventKind           `json:"kind"`
	Review       *Packet             `json:"review,omitempty"`
	Observation  *Observation        `json:"observation,omitempty"`
	Decision     *DecisionRecord     `json:"decision,omitempty"`
	Supersession *DecisionSuperseded `json:"supersession,omitempty"`
}

func (p EventPayload) Validate() error {
	if err := p.Kind.Validate(); err != nil {
		return err
	}
	if p.armCount() != 1 {
		return errors.Join(core.ErrReviewControlUnsupportedEventKind, contractError())
	}
	if err := p.validateArm(); err != nil {
		return err
	}
	return validateEncodedDocument(eventPayloadWire(p), EventPayloadJSONMaximumBytes)
}

func (p EventPayload) validateArm() error {
	switch p.Kind {
	case EventReviewIssued:
		if p.Review == nil {
			return errors.Join(core.ErrReviewControlUnsupportedEventKind, contractError())
		}
		return p.Review.Validate()
	case EventObservationRecorded:
		if p.Observation == nil {
			return errors.Join(core.ErrReviewControlUnsupportedEventKind, contractError())
		}
		return p.Observation.Validate()
	case EventHumanAccepted, EventHumanRefused:
		return p.validateDecision()
	case EventDecisionSuperseded:
		if p.Supersession == nil {
			return errors.Join(core.ErrReviewControlUnsupportedEventKind, contractError())
		}
		return p.Supersession.Validate()
	default:
		return errors.Join(core.ErrReviewControlUnsupportedEventKind, contractError())
	}
}

func (p EventPayload) armCount() int {
	count := 0
	if p.Review != nil {
		count++
	}
	if p.Observation != nil {
		count++
	}
	if p.Decision != nil {
		count++
	}
	if p.Supersession != nil {
		count++
	}
	return count
}

func (p EventPayload) validateDecision() error {
	if p.Decision == nil {
		return errors.Join(core.ErrReviewControlUnsupportedEventKind, contractError())
	}
	if err := p.Decision.Validate(); err != nil {
		return err
	}
	want := DecisionAccept
	if p.Kind == EventHumanRefused {
		want = DecisionRefuse
	}
	if p.Decision.Intent.Kind != want {
		return errors.Join(core.ErrReviewControlUnsupportedEventKind, contractError())
	}
	return nil
}

type Projection struct {
	Review         ReviewIdentity             `json:"review"`
	Subject        Subject                    `json:"subject"`
	LatestVerdict  *Verdict                   `json:"latest_verdict,omitempty"`
	LatestDecision *DecisionKind              `json:"latest_decision,omitempty"`
	Observation    *ObservationIdentity       `json:"observation,omitempty"`
	DecisionEvent  *proofledger.EventIdentity `json:"decision_event,omitempty"`
	Current        bool                       `json:"current"`
}

func (p Projection) Validate() error {
	if err := errors.Join(p.Review.Validate(), p.Subject.Validate()); err != nil {
		return contractError(err)
	}
	if err := p.validateEnums(); err != nil {
		return err
	}
	return p.validateReferences()
}

func (p Projection) validateEnums() error {
	if p.LatestVerdict != nil {
		if err := p.LatestVerdict.Validate(); err != nil {
			return err
		}
	}
	if p.LatestDecision != nil {
		if err := p.LatestDecision.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (p Projection) validateReferences() error {
	if (p.LatestVerdict == nil) != (p.Observation == nil) || (p.LatestDecision == nil) != (p.DecisionEvent == nil) {
		return contractError()
	}
	if p.Observation != nil {
		if err := p.Observation.Validate(); err != nil {
			return err
		}
	}
	if p.DecisionEvent != nil {
		if err := p.DecisionEvent.Validate(); err != nil {
			return err
		}
	}
	if p.Current && p.LatestDecision == nil {
		return contractError()
	}
	return nil
}

type Fold struct {
	projection Projection
	contract   core.SHA256Digest
	issued     bool
}

func (f *Fold) Observe(event proofledger.Envelope[EventPayload]) error {
	if f == nil {
		return contractError()
	}
	if err := event.Validate(); err != nil {
		return err
	}
	payload := event.Payload
	if payload.Kind == EventReviewIssued {
		return f.issue(*payload.Review)
	}
	if !f.issued {
		return errors.Join(core.ErrReviewControlSubjectMismatch, contractError())
	}
	switch payload.Kind {
	case EventObservationRecorded:
		return f.observe(*payload.Observation)
	case EventHumanAccepted, EventHumanRefused:
		return f.decide(event.Event, *payload.Decision)
	case EventDecisionSuperseded:
		return f.supersede(*payload.Supersession)
	default:
		return errors.Join(core.ErrReviewControlUnsupportedEventKind, contractError())
	}
}

func (f *Fold) issue(packet Packet) error {
	if f.issued {
		return errors.Join(core.ErrReviewControlSubjectMismatch, contractError())
	}
	if err := packet.Validate(); err != nil {
		return err
	}
	digest, err := packet.ContractDigest()
	if err != nil {
		return err
	}
	f.projection = Projection{Review: packet.Identity, Subject: packet.Subject}
	f.contract = digest
	f.issued = true
	return nil
}

func (f *Fold) observe(observation Observation) error {
	if observation.Review != f.projection.Review {
		return errors.Join(core.ErrReviewControlSubjectMismatch, contractError())
	}
	if err := VerifySubject(f.projection.Subject, observation.Subject); err != nil {
		return err
	}
	verdict := observation.Verdict
	f.projection.LatestVerdict = &verdict
	identity := observation.Identity
	f.projection.Observation = &identity
	return nil
}

func (f *Fold) decide(event proofledger.EventIdentity, record DecisionRecord) error {
	if record.Intent.Review != f.projection.Review {
		return errors.Join(core.ErrReviewControlSubjectMismatch, contractError())
	}
	if err := VerifySubject(f.projection.Subject, record.Intent.Subject); err != nil {
		return err
	}
	if record.Intent.Contract != f.contract {
		return errors.Join(core.ErrReviewControlSubjectMismatch, contractError())
	}
	if record.Intent.Kind == DecisionAccept {
		if record.Intent.Observation == nil || f.projection.Observation == nil ||
			*record.Intent.Observation != *f.projection.Observation {
			return errors.Join(core.ErrReviewControlObservationMismatch, contractError())
		}
	}
	decision := record.Intent.Kind
	f.projection.LatestDecision = &decision
	f.projection.DecisionEvent = &event
	f.projection.Current = true
	return nil
}

func (f *Fold) supersede(supersession DecisionSuperseded) error {
	if !f.projection.Current || f.projection.DecisionEvent == nil || *f.projection.DecisionEvent != supersession.Previous {
		return errors.Join(core.ErrReviewControlObservationMismatch, contractError())
	}
	f.projection.Current = false
	return nil
}

func (f Fold) Projection() (Projection, error) {
	if !f.issued {
		return Projection{}, contractError()
	}
	if err := f.projection.Validate(); err != nil {
		return Projection{}, err
	}
	return copyProjection(f.projection), nil
}

func copyProjection(source Projection) Projection {
	result := source
	if source.LatestVerdict != nil {
		value := *source.LatestVerdict
		result.LatestVerdict = &value
	}
	if source.LatestDecision != nil {
		value := *source.LatestDecision
		result.LatestDecision = &value
	}
	if source.Observation != nil {
		value := *source.Observation
		result.Observation = &value
	}
	if source.DecisionEvent != nil {
		value := *source.DecisionEvent
		result.DecisionEvent = &value
	}
	return result
}
