package controlplane

import (
	"encoding/json/jsontext"
	"errors"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
)

// AccessRegistrationRequest is first contact authenticated by a reusable access
// token. It is not a one-use RegistrationRequest and cannot spend its grant.
// The product decides account scope, payment, revocation and machine limits.
type AccessRegistrationRequest struct {
	Token        controlwire.AccessToken  `json:"access_token"`
	Build        core.BuildIdentity       `json:"build"`
	RequestNonce controlwire.RequestNonce `json:"request_nonce"`
	DeviceKey    core.Ed25519PublicKey    `json:"device_public_key"`
	Installation lease.DeviceID           `json:"installation"`
	Revision     controlwire.Revision     `json:"revision"`
}

// The explicit wire boundary delays secret allocation until strict structure
// admission succeeds. A generic decoder can discard a partial result on an
// unknown member; allocating secret custody during that pass would lose its
// destruction handle. Raw JSON is confined to this bounded wire boundary.
type accessRegistrationRequestWire struct {
	Token        jsontext.Value           `json:"access_token"`
	Build        core.BuildIdentity       `json:"build"`
	RequestNonce controlwire.RequestNonce `json:"request_nonce"`
	DeviceKey    core.Ed25519PublicKey    `json:"device_public_key"`
	Installation lease.DeviceID           `json:"installation"`
	Revision     controlwire.Revision     `json:"revision"`
}

const AccessRegistrationRequestJSONMaximumBytes = 16 << 10

func (r AccessRegistrationRequest) Validate() error {
	if err := errors.Join(r.Token.Validate(), r.identity().Validate()); err != nil {
		return registrationError(err)
	}
	return nil
}

func (r AccessRegistrationRequest) identity() RegistrationIdentity {
	return RegistrationIdentity{Build: r.Build, RequestNonce: r.RequestNonce, DeviceKey: r.DeviceKey, Installation: r.Installation, Revision: r.Revision}
}

func (r AccessRegistrationRequest) Identity() (RegistrationIdentity, error) {
	if err := r.Validate(); err != nil {
		return RegistrationIdentity{}, err
	}
	return r.identity(), nil
}

func (r AccessRegistrationRequest) ControlRoute() (controlwire.RouteContract, error) {
	return controlwire.NewRouteContract(r.Build.Offering(), controlwire.RouteFamilyRegistrations)
}

func (r AccessRegistrationRequest) ControlRevision() controlwire.Revision  { return r.Revision }
func (r AccessRegistrationRequest) ControlNonce() controlwire.RequestNonce { return r.RequestNonce }

func (r AccessRegistrationRequest) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonError(err)
	}
	object := attest.BeginCanonicalObject(nil)
	object.Value(protocolMemberBuild, r.Build)
	object.Value(protocolMemberRevision, r.Revision)
	object.Value(protocolMemberRequestNonce, r.RequestNonce)
	object.Value(protocolMemberDevicePublicKey, r.DeviceKey)
	object.Value("access_token", r.Token)
	object.Value(protocolMemberInstallation, r.Installation)
	encoded, err := object.End()
	if err != nil || len(encoded) > AccessRegistrationRequestJSONMaximumBytes {
		clear(encoded)
		return nil, jsonError(registrationError(err))
	}
	return encoded, nil
}

// UnmarshalJSON preserves the receiver on refusal and destroys any decoded
// candidate secret on failure. The caller owns successful token custody.
func (r *AccessRegistrationRequest) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError(registrationError())
	}
	limits, err := documentJSONLimits(AccessRegistrationRequestJSONMaximumBytes)
	if err != nil {
		return jsonError(registrationError(err))
	}
	wire, err := core.DecodeStrictJSONStructure[accessRegistrationRequestWire](data, limits)
	defer clear(wire.Token)
	if err != nil {
		return jsonError(registrationError(err))
	}
	candidate := AccessRegistrationRequest{Build: wire.Build, RequestNonce: wire.RequestNonce, DeviceKey: wire.DeviceKey, Installation: wire.Installation, Revision: wire.Revision}
	if err := candidate.Token.UnmarshalJSON(wire.Token); err != nil {
		return jsonError(registrationError(err))
	}
	if err := candidate.Validate(); err != nil {
		_ = candidate.Token.Destroy()
		return jsonError(registrationError(err))
	}
	*r = candidate
	return nil
}

// AccessRegistrationVerification is a transaction-local mechanical agreement.
// ExpectedVerifier comes from the product's current durable key record.
// PriorReplay belongs to this installation, never to the reusable key: another
// installation may use the same key with its own empty replay slot.
type AccessRegistrationVerification struct {
	PriorReplay      *controlwire.ReplayIdentity
	Request          AccessRegistrationRequest
	ExpectedVerifier controlwire.AccessTokenVerifier
}

func (v AccessRegistrationVerification) Validate() error {
	if err := errors.Join(v.Request.Validate(), v.ExpectedVerifier.Validate()); err != nil {
		return registrationError(err)
	}
	if v.PriorReplay != nil {
		if err := v.PriorReplay.Validate(); err != nil {
			return registrationError(err)
		}
	}
	matches, err := v.ExpectedVerifier.Matches(v.Request.Token)
	if err != nil || !matches {
		return registrationError(errors.Join(core.ErrControlWireToken, err))
	}
	return nil
}

// VerifyAccessRegistration authenticates the request and seals its exact
// identity/replay facts. It neither consumes nor revokes the reusable key.
// Request token custody remains with the caller across transaction retries;
// the caller must destroy it after the outer operation completes.
func (s Authority) VerifyAccessRegistration(v AccessRegistrationVerification) (VerifiedRegistrationAuthority, error) {
	if err := errors.Join(s.Validate(), v.Validate()); err != nil {
		return VerifiedRegistrationAuthority{}, registrationError(err)
	}
	replay, err := controlwire.CommitReplayIdentity(v.Request)
	if err != nil {
		return VerifiedRegistrationAuthority{}, registrationError(err)
	}
	disposition := controlwire.ReplayDispositionFresh
	if v.PriorReplay != nil {
		if !v.PriorReplay.Equal(replay) {
			return VerifiedRegistrationAuthority{}, registrationError(core.ErrControlWireReplayConflict)
		}
		disposition = controlwire.ReplayDispositionExact
	}
	verified := VerifiedRegistrationAuthority{identity: v.Request.identity(), replay: replay, disposition: disposition}
	return verified, verified.Validate()
}

var (
	_ core.Validatable              = AccessRegistrationRequest{}
	_ core.Validatable              = AccessRegistrationVerification{}
	_ controlwire.RoutedJSONRequest = AccessRegistrationRequest{}
)
