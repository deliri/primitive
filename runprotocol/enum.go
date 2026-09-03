package runprotocol

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

func validateEnum(value uint8, labels []string, diagnostic string) error {
	if int(value) >= len(labels) || labels[value] == "" {
		return contractError(errors.New(diagnostic))
	}
	return nil
}

func marshalEnum(value uint8, labels []string, diagnostic string) ([]byte, error) {
	if err := validateEnum(value, labels, diagnostic); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONString(labels[value])
	if err != nil {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func unmarshalEnum(data []byte, labels []string, diagnostic string) (uint8, error) {
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return 0, jsonError(err)
	}
	for index := 1; index < len(labels); index++ {
		if labels[index] == value {
			return uint8(index), nil
		}
	}
	return 0, jsonError(errors.New(diagnostic))
}

func enumString(value uint8, labels []string) string {
	if int(value) >= len(labels) {
		return ""
	}
	return labels[value]
}

type Outcome uint8

const (
	OutcomeUnknown Outcome = iota
	OutcomePassed
	OutcomeFailed
	OutcomeSkipped
	OutcomeUnavailable
	OutcomeTimedOut
	OutcomeCancelled
	OutcomeSetupFailed
	OutcomeInfrastructureFailed
	OutcomeNotRun
	OutcomeNonAccepting
	outcomeLimit
)

func outcomeLabels() []string {
	return []string{"", "outcome_passed", "outcome_failed", "outcome_skipped", "outcome_unavailable", "outcome_" + canonicalTimedOutText, "outcome_" + canonicalCancelledText, "outcome_setup_failed", "outcome_infrastructure_failed", "outcome_not_run", "outcome_non_accepting"}
}
func (o Outcome) Validate() error {
	return validateEnum(uint8(o), outcomeLabels(), outcomeInvalidDiagnostic)
}
func (o Outcome) IsValid() bool { return o.Validate() == nil }
func (o Outcome) String() string {
	if o.Validate() != nil {
		return ""
	}
	return outcomeLabels()[o]
}
func (o Outcome) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(o), outcomeLabels(), outcomeInvalidDiagnostic)
}
func (o *Outcome) UnmarshalJSON(data []byte) error {
	if o == nil {
		return jsonError(errors.New("nil run protocol outcome receiver"))
	}
	value, err := unmarshalEnum(data, outcomeLabels(), outcomeInvalidDiagnostic)
	if err == nil {
		*o = Outcome(value)
	}
	return err
}
