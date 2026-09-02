package reviewcontrol

import (
	"errors"

	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/projectstandards"
)

type DecisionIntent struct {
	Request     controlwire.RequestNonce `json:"request"`
	Review      ReviewIdentity           `json:"review"`
	Subject     Subject                  `json:"subject"`
	Observation *ObservationIdentity     `json:"observation,omitempty"`
	Contract    core.SHA256Digest        `json:"contract_digest"`
	Kind        DecisionKind             `json:"kind"`
	Reason      DecisionReason           `json:"reason"`
}

type DecisionValidation struct {
	Packet      Packet
	Observation *Observation
	Authority   VerifiedHumanAuthority
	Intent      DecisionIntent
}

func (d DecisionIntent) Validate() error {
	if err := errors.Join(d.Request.Validate(), d.Review.Validate(), d.Subject.Validate(), d.Contract.Validate(), d.Kind.Validate(), d.Reason.Validate()); err != nil {
		return contractError(err)
	}
	if d.Kind == DecisionAccept && d.Observation == nil {
		return errors.Join(core.ErrReviewControlObservationMismatch, contractError())
	}
	if d.Observation != nil {
		if err := d.Observation.Validate(); err != nil {
			return contractError(err)
		}
	}
	return validateEncodedDocument(decisionIntentWire(d), DecisionJSONMaximumBytes)
}

func ValidateDecision(request DecisionValidation) error {
	packet := request.Packet
	intent := request.Intent
	if err := errors.Join(packet.Validate(), request.Authority.Validate(), intent.Validate()); err != nil {
		return err
	}
	if intent.Review != packet.Identity {
		return errors.Join(core.ErrReviewControlSubjectMismatch, contractError())
	}
	if err := VerifySubject(packet.Subject, intent.Subject); err != nil {
		return err
	}
	digest, err := packet.ContractDigest()
	if err != nil {
		return err
	}
	if intent.Contract != digest {
		return errors.Join(core.ErrReviewControlSubjectMismatch, contractError())
	}
	if intent.Kind == DecisionRefuse {
		return nil
	}
	return validateAcceptedObservation(packet, request.Observation, intent)
}

func validateAcceptedObservation(packet Packet, observation *Observation, intent DecisionIntent) error {
	if observation == nil || intent.Observation == nil {
		return errors.Join(core.ErrReviewControlObservationMismatch, contractError())
	}
	if err := observation.Validate(); err != nil {
		return err
	}
	if observation.Identity != *intent.Observation || observation.Review != packet.Identity {
		return errors.Join(core.ErrReviewControlObservationMismatch, contractError())
	}
	if err := VerifySubject(packet.Subject, observation.Subject); err != nil {
		return err
	}
	if err := validateObservationLocations(packet, *observation); err != nil {
		return err
	}
	return validateRequiredEvidence(packet.Contract.RequiredProof, observation.Evidence)
}

func validateObservationLocations(packet Packet, observation Observation) error {
	for index := range observation.Findings {
		location := observation.Findings[index].Location
		if location != nil && !packetReferencesPath(packet, location.Path) {
			return errors.Join(core.ErrReviewControlSubjectMismatch, contractError())
		}
	}
	return nil
}

func packetReferencesPath(packet Packet, candidate projectstandards.SourcePath) bool {
	if candidate == packet.Subject.File {
		return true
	}
	for index := range packet.Context {
		if candidate == packet.Context[index].Path {
			return true
		}
	}
	for index := range packet.Contract.RelatedCode {
		if candidate == packet.Contract.RelatedCode[index].Path {
			return true
		}
	}
	return false
}

func validateRequiredEvidence(required []ProofRequirement, evidence []EvidenceReference) error {
	if len(evidence) != len(required) {
		return errors.Join(core.ErrReviewControlMissingEvidence, contractError())
	}
	for index := range required {
		if evidenceCountForRequirement(required[index].Rule, evidence) != 1 {
			return errors.Join(core.ErrReviewControlMissingEvidence, contractError())
		}
	}
	return nil
}

func evidenceCountForRequirement(requirement projectstandards.Identifier, evidence []EvidenceReference) int {
	count := 0
	for index := range evidence {
		if evidence[index].Requirement == requirement {
			count++
		}
	}
	return count
}

func (p Packet) ContractDigest() (core.SHA256Digest, error) {
	if err := p.Contract.Validate(); err != nil {
		return core.SHA256Digest{}, err
	}
	encoded, err := core.MarshalCanonicalJSONDocument(p.Contract)
	if err != nil {
		return core.SHA256Digest{}, contractError(err)
	}
	return core.SHA256Of(encoded), nil
}

type DecisionRecord struct {
	Intent    DecisionIntent     `json:"intent"`
	Authority AuthorityReference `json:"authority"`
}

func NewDecisionRecord(authority VerifiedHumanAuthority, intent DecisionIntent) (DecisionRecord, error) {
	reference, err := authority.Reference()
	if err != nil {
		return DecisionRecord{}, err
	}
	record := DecisionRecord{Intent: intent, Authority: reference}
	return record, record.Validate()
}

func (r DecisionRecord) Validate() error {
	return validateContract(r.Intent.Validate(), r.Authority.Validate())
}
