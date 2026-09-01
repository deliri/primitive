package runnercontrol

import (
	"encoding"
	json "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

const ObservationEnvelopeSigningDomainToken = "primitive-runner-control-observation-2026-1"

type EvidenceSigningDomain uint8

const (
	EvidenceSigningDomainUnknown EvidenceSigningDomain = iota
	EvidenceSigningDomainObservationV1
	evidenceSigningDomainLimit
)

func (d EvidenceSigningDomain) Validate() error {
	if d != EvidenceSigningDomainObservationV1 {
		return core.ErrPrimitiveContract
	}
	return nil
}
func (d EvidenceSigningDomain) IsValid() bool { return d.Validate() == nil }
func (d EvidenceSigningDomain) String() string {
	if d == EvidenceSigningDomainObservationV1 {
		return ObservationEnvelopeSigningDomainToken
	}
	return invalidEnumString()
}
func (d EvidenceSigningDomain) MarshalText() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return []byte(d.String()), nil
}
func (EvidenceSigningDomain) ParseCanonicalText(text []byte) (EvidenceSigningDomain, error) {
	if len(text) > attest.SigningDomainMaximumBytes || string(text) != ObservationEnvelopeSigningDomainToken {
		return EvidenceSigningDomainUnknown, core.ErrPrimitiveContract
	}
	return EvidenceSigningDomainObservationV1, nil
}
func (d EvidenceSigningDomain) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(d.String())
}
func (d *EvidenceSigningDomain) UnmarshalJSON(data []byte) error {
	if d == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	parsed, err := EvidenceSigningDomainUnknown.ParseCanonicalText([]byte(value))
	if err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*d = parsed
	return nil
}

type evidenceSigningDomainWitness[D attest.SigningDomain[D]] [0]D

var (
	_ core.Validatable            = EvidenceSigningDomainUnknown
	_ core.ValidatedJSONMarshaler = EvidenceSigningDomain(0)
	_ encoding.TextMarshaler      = EvidenceSigningDomainUnknown
	_ json.Unmarshaler            = (*EvidenceSigningDomain)(nil)
	_                             = evidenceSigningDomainWitness[EvidenceSigningDomain]{}
)
