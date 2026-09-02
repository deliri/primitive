package reviewcontrol

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

type ReviewerKind uint8
type Verdict uint8
type FindingSeverity uint8
type CheckKind uint8
type ProofKind uint8
type DecisionKind uint8
type EventKind uint8
type AuthorityKind uint8

const (
	ReviewerUnknown ReviewerKind = iota
	ReviewerHuman
	ReviewerAgent
	ReviewerTool
	reviewerKindLimit
)

const (
	AuthorityUnknown AuthorityKind = iota
	AuthorityHuman
	AuthorityAgent
	AuthorityTool
	authorityKindLimit
)

const (
	VerdictUnknown Verdict = iota
	VerdictPass
	VerdictChangesRequired
	VerdictUnableToReview
	verdictLimit
)

const (
	SeverityUnknown FindingSeverity = iota
	SeverityBlocking
	SeverityMajor
	SeverityMinor
	SeverityInformational
	severityLimit
)

const (
	CheckUnknown CheckKind = iota
	CheckCompile
	CheckTest
	CheckRace
	CheckFuzz
	CheckBenchmark
	CheckLint
	CheckManualInspection
	checkKindLimit
)

const (
	ProofUnknown ProofKind = iota
	ProofSourceInspection
	ProofMachineObservation
	ProofHumanInspection
	proofKindLimit
)

const (
	DecisionUnknown DecisionKind = iota
	DecisionAccept
	DecisionRefuse
	decisionKindLimit
)

const (
	EventUnknown EventKind = iota
	EventReviewIssued
	EventObservationRecorded
	EventHumanAccepted
	EventHumanRefused
	EventDecisionSuperseded
	eventKindLimit
)

func reviewerLabels() []string { return []string{"", "human", "agent", "tool"} }
func verdictLabels() []string  { return []string{"", "pass", "changes_required", "unable_to_review"} }
func severityLabels() []string { return []string{"", "blocking", "major", "minor", "informational"} }
func checkLabels() []string {
	return []string{"", "compile", "test", "race", "fuzz", "benchmark", "lint", "manual_inspection"}
}
func proofLabels() []string {
	return []string{"", "source_inspection", "machine_observation", "human_inspection"}
}
func decisionLabels() []string { return []string{"", "accept", "refuse"} }
func eventLabels() []string {
	return []string{"", "review_issued", "observation_recorded", "human_accepted", "human_refused", "decision_superseded"}
}
func authorityLabels() []string { return []string{"", "human", "agent", "tool"} }

func validateEnum(value uint8, labels []string) error {
	if int(value) >= len(labels) || value == 0 || labels[value] == "" {
		return contractError(errors.New("review control enum is outside its closed domain"))
	}
	return nil
}

func marshalEnum(value uint8, labels []string) ([]byte, error) {
	if err := validateEnum(value, labels); err != nil {
		return nil, jsonError(err)
	}
	return core.MarshalCanonicalJSONString(labels[value])
}

func unmarshalEnum(data []byte, labels []string) (uint8, error) {
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return 0, jsonError(err)
	}
	for index := 1; index < len(labels); index++ {
		if labels[index] == value {
			return uint8(index), nil
		}
	}
	return 0, jsonError(errors.New("review control enum text is not admitted"))
}

func (v ReviewerKind) Validate() error    { return validateEnum(uint8(v), reviewerLabels()) }
func (v Verdict) Validate() error         { return validateEnum(uint8(v), verdictLabels()) }
func (v FindingSeverity) Validate() error { return validateEnum(uint8(v), severityLabels()) }
func (v CheckKind) Validate() error       { return validateEnum(uint8(v), checkLabels()) }
func (v ProofKind) Validate() error       { return validateEnum(uint8(v), proofLabels()) }
func (v DecisionKind) Validate() error    { return validateEnum(uint8(v), decisionLabels()) }
func (v EventKind) Validate() error {
	if err := validateEnum(uint8(v), eventLabels()); err != nil {
		return errors.Join(core.ErrReviewControlUnsupportedEventKind, err)
	}
	return nil
}
func (v AuthorityKind) Validate() error { return validateEnum(uint8(v), authorityLabels()) }

func enumString(value uint8, labels []string) string {
	if validateEnum(value, labels) != nil {
		return ""
	}
	return labels[value]
}

func (v ReviewerKind) IsValid() bool     { return v.Validate() == nil }
func (v Verdict) IsValid() bool          { return v.Validate() == nil }
func (v FindingSeverity) IsValid() bool  { return v.Validate() == nil }
func (v CheckKind) IsValid() bool        { return v.Validate() == nil }
func (v ProofKind) IsValid() bool        { return v.Validate() == nil }
func (v DecisionKind) IsValid() bool     { return v.Validate() == nil }
func (v EventKind) IsValid() bool        { return v.Validate() == nil }
func (v AuthorityKind) IsValid() bool    { return v.Validate() == nil }
func (v ReviewerKind) String() string    { return enumString(uint8(v), reviewerLabels()) }
func (v Verdict) String() string         { return enumString(uint8(v), verdictLabels()) }
func (v FindingSeverity) String() string { return enumString(uint8(v), severityLabels()) }
func (v CheckKind) String() string       { return enumString(uint8(v), checkLabels()) }
func (v ProofKind) String() string       { return enumString(uint8(v), proofLabels()) }
func (v DecisionKind) String() string    { return enumString(uint8(v), decisionLabels()) }
func (v EventKind) String() string       { return enumString(uint8(v), eventLabels()) }
func (v AuthorityKind) String() string   { return enumString(uint8(v), authorityLabels()) }

func (v ReviewerKind) MarshalJSON() ([]byte, error) { return marshalEnum(uint8(v), reviewerLabels()) }
func (v Verdict) MarshalJSON() ([]byte, error)      { return marshalEnum(uint8(v), verdictLabels()) }
func (v FindingSeverity) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(v), severityLabels())
}
func (v CheckKind) MarshalJSON() ([]byte, error)     { return marshalEnum(uint8(v), checkLabels()) }
func (v ProofKind) MarshalJSON() ([]byte, error)     { return marshalEnum(uint8(v), proofLabels()) }
func (v DecisionKind) MarshalJSON() ([]byte, error)  { return marshalEnum(uint8(v), decisionLabels()) }
func (v EventKind) MarshalJSON() ([]byte, error)     { return marshalEnum(uint8(v), eventLabels()) }
func (v AuthorityKind) MarshalJSON() ([]byte, error) { return marshalEnum(uint8(v), authorityLabels()) }

func (v *ReviewerKind) UnmarshalJSON(data []byte) error {
	if v == nil {
		return jsonError()
	}
	n, e := unmarshalEnum(data, reviewerLabels())
	if e == nil {
		*v = ReviewerKind(n)
	}
	return e
}
func (v *Verdict) UnmarshalJSON(data []byte) error {
	if v == nil {
		return jsonError()
	}
	n, e := unmarshalEnum(data, verdictLabels())
	if e == nil {
		*v = Verdict(n)
	}
	return e
}
func (v *FindingSeverity) UnmarshalJSON(data []byte) error {
	if v == nil {
		return jsonError()
	}
	n, e := unmarshalEnum(data, severityLabels())
	if e == nil {
		*v = FindingSeverity(n)
	}
	return e
}
func (v *CheckKind) UnmarshalJSON(data []byte) error {
	if v == nil {
		return jsonError()
	}
	n, e := unmarshalEnum(data, checkLabels())
	if e == nil {
		*v = CheckKind(n)
	}
	return e
}
func (v *ProofKind) UnmarshalJSON(data []byte) error {
	if v == nil {
		return jsonError()
	}
	n, e := unmarshalEnum(data, proofLabels())
	if e == nil {
		*v = ProofKind(n)
	}
	return e
}
func (v *DecisionKind) UnmarshalJSON(data []byte) error {
	if v == nil {
		return jsonError()
	}
	n, e := unmarshalEnum(data, decisionLabels())
	if e == nil {
		*v = DecisionKind(n)
	}
	return e
}
func (v *EventKind) UnmarshalJSON(data []byte) error {
	if v == nil {
		return jsonError()
	}
	n, e := unmarshalEnum(data, eventLabels())
	if e == nil {
		*v = EventKind(n)
	}
	return e
}
func (v *AuthorityKind) UnmarshalJSON(data []byte) error {
	if v == nil {
		return jsonError()
	}
	n, e := unmarshalEnum(data, authorityLabels())
	if e == nil {
		*v = AuthorityKind(n)
	}
	return e
}
