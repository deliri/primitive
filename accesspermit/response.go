package accesspermit

import (
	"errors"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
)

const (
	RegistrationResponseMaximumBytes = controlplane.RegistrationDocumentJSONMaximumBytes + DocumentMaximumBytes + 64
	CheckInResponseMaximumBytes      = controlplane.CheckInResponseDocumentJSONMaximumBytes + DocumentMaximumBytes + 64
)

// RegistrationResponse joins installation mechanics and an independent access
// interval. Both sides consume this exact Primitive agreement, not local DTOs.
type RegistrationResponse struct {
	Registration controlplane.RegistrationDocument `json:"registration"`
	Permit       Document                          `json:"permit"`
}

func (r RegistrationResponse) Validate() error {
	if err := errors.Join(r.Registration.Validate(), r.Permit.Validate()); err != nil {
		return errors.Join(core.ErrAccessPermitContract, err)
	}
	return validateBinding(r.Permit.Terms, r.Registration.Payload.Header, r.Registration.Payload.Watermark)
}

type CheckInResponse struct {
	CheckIn controlplane.CheckInResponseDocument `json:"check_in"`
	Permit  Document                             `json:"permit"`
}

func (r CheckInResponse) Validate() error {
	if err := errors.Join(r.CheckIn.Validate(), r.Permit.Validate()); err != nil {
		return errors.Join(core.ErrAccessPermitContract, err)
	}
	return validateBinding(r.Permit.Terms, r.CheckIn.Payload.Header, r.CheckIn.Payload.Watermark)
}

func validateBinding(terms Terms, header controlplane.ResponseHeader, watermark controlplane.UsageWatermark) error {
	if terms.Binding != (Binding{Family: header.Family, Subject: watermark.Subject, Account: header.Account, RequestNonce: header.RequestNonce, Generation: watermark.Generation}) {
		return core.ErrAccessPermitBinding
	}
	if !header.Status.AdmitsGrant() && terms.Window.Decision == DecisionAllow {
		return core.ErrAccessPermitBinding
	}
	return nil
}

func (r RegistrationResponse) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	type wire RegistrationResponse
	return core.MarshalCanonicalJSONDocument(wire(r))
}

func (r CheckInResponse) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	type wire CheckInResponse
	return core.MarshalCanonicalJSONDocument(wire(r))
}

func responseLimits(maximum uint64) core.StrictJSONLimits {
	result := limits()
	result.DocumentMaximumBytes, _ = core.NewByteCount(maximum)
	return result
}

func (r *RegistrationResponse) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.Join(core.ErrJSONContract, core.ErrAccessPermitContract)
	}
	type wire RegistrationResponse
	got, err := core.DecodeStrictJSONStructure[wire](data, responseLimits(RegistrationResponseMaximumBytes))
	if err != nil {
		return errors.Join(core.ErrAccessPermitContract, err)
	}
	candidate := RegistrationResponse(got)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, core.ErrAccessPermitContract, err)
	}
	*r = candidate
	return nil
}

func (r *CheckInResponse) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.Join(core.ErrJSONContract, core.ErrAccessPermitContract)
	}
	type wire CheckInResponse
	got, err := core.DecodeStrictJSONStructure[wire](data, responseLimits(CheckInResponseMaximumBytes))
	if err != nil {
		return errors.Join(core.ErrAccessPermitContract, err)
	}
	candidate := CheckInResponse(got)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, core.ErrAccessPermitContract, err)
	}
	*r = candidate
	return nil
}

// ResponseBinding constructs the exact independently authenticated facts after
// the caller verifies its Primitive response. It does not authenticate a header.
func ResponseBinding(header controlplane.ResponseHeader, subject lease.Subject, generation lease.Generation) (Binding, error) {
	if err := errors.Join(header.Validate(), subject.Validate(), generation.Validate()); err != nil {
		return Binding{}, errors.Join(core.ErrAccessPermitContract, err)
	}
	if header.Offering != subject.Offering || header.Installation != subject.DeviceID {
		return Binding{}, core.ErrAccessPermitBinding
	}
	result := Binding{Family: header.Family, Subject: subject, Account: header.Account, RequestNonce: header.RequestNonce, Generation: generation}
	return result, result.Validate()
}

var (
	_ core.ValidatedJSONMarshaler = RegistrationResponse{}
	_ core.ValidatedJSONMarshaler = CheckInResponse{}
)
