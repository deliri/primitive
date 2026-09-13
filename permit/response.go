package permit

import (
	"errors"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
)

// ResponseMaximumBytes bounds the complete enrollment/accounting plus permission
// projection. Each permission document retains its separate 8,192-byte bound.
const ResponseMaximumBytes = 64 << 10

func responseLimits() core.StrictJSONLimits {
	maximum, err := core.NewByteCount(ResponseMaximumBytes)
	if err != nil {
		return core.StrictJSONLimits{}
	}
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = maximum
	return limits
}

// RegistrationResponse binds the mechanical enrollment and the API-selected
// permission. Structural validation authenticates neither signed component.
type RegistrationResponse struct {
	Registration controlplane.RegistrationDocument `json:"registration"`
	Permission   Document                          `json:"permission"`
}

func (r RegistrationResponse) Validate() error {
	if err := errors.Join(r.Registration.Validate(), r.Permission.Validate()); err != nil {
		return errors.Join(core.ErrPermitContract, err)
	}
	p := r.Registration.Payload
	if p.Certificate == nil || r.Permission.Terms.Subject != p.Certificate.Body.Subject || r.Permission.Terms.Build != p.Certificate.Body.Build {
		return core.ErrPermitBinding
	}
	h, err := p.Lease.Decision.Header()
	if err != nil {
		return errors.Join(core.ErrPermitContract, err)
	}
	return r.Permission.Terms.validateResponseBinding(p.Header, h.Generation)
}

func (r RegistrationResponse) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	type wire RegistrationResponse
	return core.MarshalCanonicalJSONDocument(wire(r))
}

func (r *RegistrationResponse) UnmarshalJSON(data []byte) error {
	if r == nil {
		return core.ErrPermitContract
	}
	type wire RegistrationResponse
	decoded, err := core.DecodeStrictJSONStructure[wire](data, responseLimits())
	if err != nil {
		return errors.Join(core.ErrPermitContract, err)
	}
	got := RegistrationResponse(decoded)
	if err := got.Validate(); err != nil {
		return err
	}
	*r = got
	return nil
}

type CheckInResponse struct {
	CheckIn    controlplane.CheckInResponseDocument `json:"check_in"`
	Permission Document                             `json:"permission"`
}

func (r CheckInResponse) Validate() error {
	if err := errors.Join(r.CheckIn.Validate(), r.Permission.Validate()); err != nil {
		return errors.Join(core.ErrPermitContract, err)
	}
	p := r.CheckIn.Payload
	h, err := p.Lease.Decision.Header()
	if err != nil {
		return errors.Join(core.ErrPermitContract, err)
	}
	if r.Permission.Terms.Subject != h.Subject {
		return core.ErrPermitBinding
	}
	return r.Permission.Terms.validateResponseBinding(p.Header, h.Generation)
}

func (r CheckInResponse) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	type wire CheckInResponse
	return core.MarshalCanonicalJSONDocument(wire(r))
}

func (r *CheckInResponse) UnmarshalJSON(data []byte) error {
	if r == nil {
		return core.ErrPermitContract
	}
	type wire CheckInResponse
	decoded, err := core.DecodeStrictJSONStructure[wire](data, responseLimits())
	if err != nil {
		return errors.Join(core.ErrPermitContract, err)
	}
	got := CheckInResponse(decoded)
	if err := got.Validate(); err != nil {
		return err
	}
	*r = got
	return nil
}

func (t Terms) validateResponseBinding(header controlplane.ResponseHeader, generation lease.Generation) error {
	if t.Subject.DeviceID != header.Installation || t.Subject.Offering != header.Offering || t.RequestNonce != header.RequestNonce || t.Generation != generation || t.NotBefore != header.ProviderTime {
		return core.ErrPermitBinding
	}
	return nil
}
