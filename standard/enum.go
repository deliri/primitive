package standard

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

type DeliveryState uint8

const (
	DeliveryUnknown DeliveryState = iota
	DeliveryPlanned
	DeliveryInProgress
	DeliveryDelivered
	deliveryLimit
)

func deliveryLabels() []string {
	return []string{"", "delivery_planned", "delivery_in_progress", "delivery_delivered"}
}
func (s DeliveryState) Validate() error {
	return validateEnum(uint8(s), deliveryLabels(), deliveryStateInvalidDiagnostic)
}
func (s DeliveryState) IsValid() bool { return s.Validate() == nil }
func (s DeliveryState) String() string {
	if s.Validate() != nil {
		return ""
	}
	return deliveryLabels()[s]
}
func (s DeliveryState) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(s), deliveryLabels(), deliveryStateInvalidDiagnostic)
}
func (s *DeliveryState) UnmarshalJSON(data []byte) error {
	if s == nil {
		return jsonError(errors.New("nil standard delivery state receiver"))
	}
	value, err := unmarshalEnum(data, deliveryLabels(), deliveryStateInvalidDiagnostic)
	if err == nil {
		*s = DeliveryState(value)
	}
	return err
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
		return jsonError(errors.New("nil standard outcome receiver"))
	}
	value, err := unmarshalEnum(data, outcomeLabels(), outcomeInvalidDiagnostic)
	if err == nil {
		*o = Outcome(value)
	}
	return err
}

type ReportPlacement uint8

const (
	ReportPlacementUnknown ReportPlacement = iota
	ReportPlacementProject
	ReportPlacementPackage
	ReportPlacementInsights
	reportPlacementLimit
)

func reportPlacementLabels() []string {
	return []string{"", canonicalProjectText, "package", "insights"}
}
func (p ReportPlacement) Validate() error {
	return validateEnum(uint8(p), reportPlacementLabels(), reportPlacementInvalidDiagnostic)
}
func (p ReportPlacement) IsValid() bool  { return p.Validate() == nil }
func (p ReportPlacement) String() string { return enumString(uint8(p), reportPlacementLabels()) }
func (p ReportPlacement) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(p), reportPlacementLabels(), reportPlacementInvalidDiagnostic)
}
func (p *ReportPlacement) UnmarshalJSON(data []byte) error {
	if p == nil {
		return jsonError(errors.New("nil standard report placement receiver"))
	}
	value, err := unmarshalEnum(data, reportPlacementLabels(), reportPlacementInvalidDiagnostic)
	if err == nil {
		*p = ReportPlacement(value)
	}
	return err
}
