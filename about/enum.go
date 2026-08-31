package about

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

func deliveryLabels() []string { return []string{"", "planned", "in_progress", "delivered"} }
func (s DeliveryState) Validate() error {
	return validateEnum(uint8(s), deliveryLabels(), "about delivery state is invalid")
}
func (s DeliveryState) IsValid() bool { return s.Validate() == nil }
func (s DeliveryState) String() string {
	if s.Validate() != nil {
		return ""
	}
	return deliveryLabels()[s]
}
func (s DeliveryState) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(s), deliveryLabels(), "about delivery state is invalid")
}
func (s *DeliveryState) UnmarshalJSON(data []byte) error {
	if s == nil {
		return jsonError(errors.New("nil about delivery state receiver"))
	}
	value, err := unmarshalEnum(data, deliveryLabels(), "about delivery state is invalid")
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
	return []string{"", "passed", "failed", "skipped", "unavailable", "timed_out", "cancelled", "setup_failed", "infrastructure_failed", "not_run", "non_accepting"}
}
func (o Outcome) Validate() error {
	return validateEnum(uint8(o), outcomeLabels(), "about outcome is invalid")
}
func (o Outcome) IsValid() bool { return o.Validate() == nil }
func (o Outcome) String() string {
	if o.Validate() != nil {
		return ""
	}
	return outcomeLabels()[o]
}
func (o Outcome) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(o), outcomeLabels(), "about outcome is invalid")
}
func (o *Outcome) UnmarshalJSON(data []byte) error {
	if o == nil {
		return jsonError(errors.New("nil about outcome receiver"))
	}
	value, err := unmarshalEnum(data, outcomeLabels(), "about outcome is invalid")
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

func reportPlacementLabels() []string { return []string{"", "project", "package", "insights"} }
func (p ReportPlacement) Validate() error {
	return validateEnum(uint8(p), reportPlacementLabels(), "about report placement is invalid")
}
func (p ReportPlacement) IsValid() bool  { return p.Validate() == nil }
func (p ReportPlacement) String() string { return enumString(uint8(p), reportPlacementLabels()) }
func (p ReportPlacement) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(p), reportPlacementLabels(), "about report placement is invalid")
}
func (p *ReportPlacement) UnmarshalJSON(data []byte) error {
	if p == nil {
		return jsonError(errors.New("nil about report placement receiver"))
	}
	value, err := unmarshalEnum(data, reportPlacementLabels(), "about report placement is invalid")
	if err == nil {
		*p = ReportPlacement(value)
	}
	return err
}
