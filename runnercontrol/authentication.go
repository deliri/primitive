package runnercontrol

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"net/http"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/projectstandards"
)

type PeerRole uint8

const (
	PeerRoleUnknown PeerRole = iota
	PeerRoleOrigin
	PeerRoleRunner
	PeerRoleControl
	peerRoleLimit
)

func (r PeerRole) Validate() error {
	if r <= PeerRoleUnknown || r >= peerRoleLimit {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (r PeerRole) String() string {
	switch r {
	case PeerRoleOrigin:
		return "origin"
	case PeerRoleRunner:
		return "runner"
	case PeerRoleControl:
		return "control"
	default:
		return ""
	}
}

func (r PeerRole) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(r.String())
}

func (r *PeerRole) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	switch value {
	case "origin":
		*r = PeerRoleOrigin
	case "runner":
		*r = PeerRoleRunner
	case "control":
		*r = PeerRoleControl
	default:
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	return nil
}

type AuthenticatedPeer struct {
	Role        PeerRole                              `json:"role"`
	Certificate core.SHA256Digest                     `json:"certificate_digest"`
	Origin      *projectstandards.OriginIdentity      `json:"origin,omitempty"`
	Machine     *projectstandards.MachineID           `json:"machine_id,omitempty"`
	Generation  *projectstandards.MachineGenerationID `json:"machine_generation_id,omitempty"`
}

func (p AuthenticatedPeer) Validate() error {
	if err := errors.Join(p.Role.Validate(), p.Certificate.Validate()); err != nil {
		return err
	}
	return p.validateRoleShape()
}

func (p AuthenticatedPeer) validateRoleShape() error {
	switch p.Role {
	case PeerRoleOrigin:
		return p.validateOriginShape()
	case PeerRoleRunner:
		return p.validateRunnerShape()
	case PeerRoleControl:
		return p.validateControlShape()
	default:
		return core.ErrPrimitiveContract
	}
}

func (p AuthenticatedPeer) validateOriginShape() error {
	if p.Origin == nil || p.Machine != nil || p.Generation != nil {
		return core.ErrPrimitiveContract
	}
	return p.Origin.Validate()
}

func (p AuthenticatedPeer) validateRunnerShape() error {
	if p.Origin != nil || p.Machine == nil || p.Generation == nil {
		return core.ErrPrimitiveContract
	}
	return errors.Join(p.Machine.Validate(), p.Generation.Validate())
}

func (p AuthenticatedPeer) validateControlShape() error {
	if p.Origin != nil || p.Machine != nil || p.Generation != nil {
		return core.ErrPrimitiveContract
	}
	return nil
}

type PeerIdentityRepository interface {
	ResolvePeer(context.Context, core.SHA256Digest, PeerRole) (AuthenticatedPeer, error)
}

type RequestAuthenticator interface {
	Authenticate(*http.Request, PeerRole) (AuthenticatedPeer, error)
}

type MutualTLSAuthenticator struct{ repository PeerIdentityRepository }

type authenticatedPeerContextKey struct{}

func BindAuthenticatedPeer(request *http.Request, peer AuthenticatedPeer) (*http.Request, error) {
	if request == nil {
		return nil, core.ErrPrimitiveContract
	}
	if err := peer.Validate(); err != nil {
		return nil, err
	}
	return request.WithContext(context.WithValue(request.Context(), authenticatedPeerContextKey{}, peer)), nil
}

func AuthenticatedPeerFromContext(ctx context.Context) (AuthenticatedPeer, error) {
	if ctx == nil {
		return AuthenticatedPeer{}, core.ErrPrimitiveContract
	}
	peer, ok := ctx.Value(authenticatedPeerContextKey{}).(AuthenticatedPeer)
	if !ok {
		return AuthenticatedPeer{}, core.ErrPrimitiveContract
	}
	return peer, peer.Validate()
}

func RequireRunnerPeer(ctx context.Context, machine projectstandards.MachineID, generation projectstandards.MachineGenerationID) error {
	peer, err := AuthenticatedPeerFromContext(ctx)
	if err != nil {
		return err
	}
	if peer.Role != PeerRoleRunner || peer.Machine == nil || peer.Generation == nil || *peer.Machine != machine || *peer.Generation != generation {
		return core.ErrPrimitiveContract
	}
	return nil
}

func RequireControlPeer(ctx context.Context) error {
	peer, err := AuthenticatedPeerFromContext(ctx)
	if err != nil {
		return err
	}
	if peer.Role != PeerRoleControl || peer.Origin != nil || peer.Machine != nil || peer.Generation != nil {
		return core.ErrPrimitiveContract
	}
	return nil
}

func NewMutualTLSAuthenticator(repository PeerIdentityRepository) (MutualTLSAuthenticator, error) {
	if repository == nil {
		return MutualTLSAuthenticator{}, core.ErrPrimitiveContract
	}
	return MutualTLSAuthenticator{repository: repository}, nil
}

func (a MutualTLSAuthenticator) Authenticate(request *http.Request, role PeerRole) (AuthenticatedPeer, error) {
	if err := validateAuthenticationRequest(request, a.repository, role); err != nil {
		return AuthenticatedPeer{}, err
	}
	certificate := request.TLS.VerifiedChains[0][0]
	if certificate == nil || len(certificate.Raw) == 0 {
		return AuthenticatedPeer{}, core.ErrPrimitiveContract
	}
	digest := core.SHA256Of(certificate.Raw)
	peer, err := a.repository.ResolvePeer(request.Context(), digest, role)
	if err != nil {
		return AuthenticatedPeer{}, err
	}
	if err := peer.Validate(); err != nil {
		return AuthenticatedPeer{}, errors.Join(core.ErrPrimitiveContract, err)
	}
	if peer.Role != role || peer.Certificate != digest {
		return AuthenticatedPeer{}, core.ErrPrimitiveContract
	}
	return peer, nil
}

func validateAuthenticationRequest(request *http.Request, repository PeerIdentityRepository, role PeerRole) error {
	if request == nil || repository == nil {
		return core.ErrPrimitiveContract
	}
	if err := role.Validate(); err != nil {
		return core.ErrPrimitiveContract
	}
	if request.TLS == nil || len(request.TLS.VerifiedChains) == 0 {
		return core.ErrPrimitiveContract
	}
	if len(request.TLS.VerifiedChains[0]) == 0 {
		return core.ErrPrimitiveContract
	}
	return nil
}

var (
	_ core.Validatable     = PeerRoleUnknown
	_ json.Unmarshaler     = (*PeerRole)(nil)
	_ core.Validatable     = AuthenticatedPeer{}
	_ RequestAuthenticator = MutualTLSAuthenticator{}
)
