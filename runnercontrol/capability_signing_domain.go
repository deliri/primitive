package runnercontrol

import (
	"encoding"
	json "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

const (
	SchedulingCapabilitySigningDomainToken = "primitive-runner-scheduling-capability-2026-1"
	MemberCapabilitySigningDomainToken     = "primitive-runner-member-capability-2026-1"
	ExperimentCapabilitySigningDomainToken = "primitive-runner-experiment-capability-2026-1"
)

// CapabilitySigningDomain closes the three independently signed layers of one
// runner scheduling claim. A signature for one layer cannot authenticate any
// other layer.
type CapabilitySigningDomain uint8

const (
	CapabilitySigningDomainUnknown CapabilitySigningDomain = iota
	CapabilitySigningDomainSchedulingV1
	CapabilitySigningDomainMemberV1
	CapabilitySigningDomainExperimentV1
	capabilitySigningDomainLimit
)

func capabilitySigningDomainTokens() [capabilitySigningDomainLimit]string {
	return [...]string{
		CapabilitySigningDomainUnknown:      "",
		CapabilitySigningDomainSchedulingV1: SchedulingCapabilitySigningDomainToken,
		CapabilitySigningDomainMemberV1:     MemberCapabilitySigningDomainToken,
		CapabilitySigningDomainExperimentV1: ExperimentCapabilitySigningDomainToken,
	}
}

func (d CapabilitySigningDomain) Validate() error {
	if d <= CapabilitySigningDomainUnknown || d >= capabilitySigningDomainLimit || capabilitySigningDomainTokens()[d] == "" {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (d CapabilitySigningDomain) IsValid() bool {
	return d.Validate() == nil
}

func (d CapabilitySigningDomain) String() string {
	if d.Validate() != nil {
		return ""
	}
	return capabilitySigningDomainTokens()[d]
}

func (d CapabilitySigningDomain) MarshalText() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return []byte(d.String()), nil
}

func (CapabilitySigningDomain) ParseCanonicalText(text []byte) (CapabilitySigningDomain, error) {
	if len(text) > attest.SigningDomainMaximumBytes {
		return CapabilitySigningDomainUnknown, core.ErrPrimitiveContract
	}
	for candidate := CapabilitySigningDomainUnknown + 1; candidate < capabilitySigningDomainLimit; candidate++ {
		if candidate.String() == string(text) {
			return candidate, nil
		}
	}
	return CapabilitySigningDomainUnknown, core.ErrPrimitiveContract
}

func (d CapabilitySigningDomain) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(d.String())
}

func (d *CapabilitySigningDomain) UnmarshalJSON(data []byte) error {
	if d == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	parsed, err := CapabilitySigningDomainUnknown.ParseCanonicalText([]byte(value))
	if err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*d = parsed
	return nil
}

type capabilitySigningDomainWitness[D attest.SigningDomain[D]] [0]D

var (
	_ core.Validatable            = CapabilitySigningDomainUnknown
	_ core.ValidatedJSONMarshaler = CapabilitySigningDomain(0)
	_ encoding.TextMarshaler      = CapabilitySigningDomainUnknown
	_ json.Unmarshaler            = (*CapabilitySigningDomain)(nil)
	_                             = capabilitySigningDomainWitness[CapabilitySigningDomain]{}
)
