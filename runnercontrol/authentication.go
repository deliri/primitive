package runnercontrol

import (
	"context"
	json "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/standard"
)

type PeerCredentialKind uint8

const (
	PeerCredentialUnknown PeerCredentialKind = iota
	PeerCredentialMutualTLS
	PeerCredentialGoogleCloud
	peerCredentialLimit
)

func (k PeerCredentialKind) Validate() error {
	if !k.IsValid() {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (k PeerCredentialKind) IsValid() bool {
	return k > PeerCredentialUnknown && k < peerCredentialLimit
}

func (k PeerCredentialKind) String() string {
	switch k {
	case PeerCredentialMutualTLS:
		return authenticationMutualTLSText
	case PeerCredentialGoogleCloud:
		return authenticationGoogleCloudText
	default:
		var text string
		handleInvalidPeerCredentialKindString(&text)
		return text
	}
}

func handleInvalidPeerCredentialKindString(target *string) { *target = "invalid_peer_credential_kind" }

func (k PeerCredentialKind) MarshalJSON() ([]byte, error) {
	if err := k.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(k.String())
}

func (k *PeerCredentialKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	switch value {
	case authenticationMutualTLSText:
		*k = PeerCredentialMutualTLS
	case authenticationGoogleCloudText:
		*k = PeerCredentialGoogleCloud
	default:
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	return nil
}

type PeerCredential struct {
	Kind     PeerCredentialKind `json:"kind"`
	Identity core.SHA256Digest  `json:"identity"`
}

func (c PeerCredential) Validate() error {
	return errors.Join(c.Kind.Validate(), c.Identity.Validate())
}

func NewPeerCredential(kind PeerCredentialKind, identity core.SHA256Digest) (PeerCredential, error) {
	credential := PeerCredential{Kind: kind, Identity: identity}
	return credential, credential.Validate()
}

type PeerRole uint8

const (
	PeerRoleUnknown PeerRole = iota
	PeerRoleOrigin
	PeerRoleRunner
	PeerRoleControl
	peerRoleLimit
)

func (r PeerRole) Validate() error {
	if !r.IsValid() {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (r PeerRole) IsValid() bool {
	return r > PeerRoleUnknown && r < peerRoleLimit
}

func (r PeerRole) String() string {
	switch r {
	case PeerRoleOrigin:
		return authenticationOriginRoleText
	case PeerRoleRunner:
		return authenticationRunnerRoleText
	case PeerRoleControl:
		return authenticationControlRoleText
	default:
		var text string
		handleInvalidPeerRoleString(&text)
		return text
	}
}

func handleInvalidPeerRoleString(target *string) { *target = "invalid_peer_role" }

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
	case authenticationOriginRoleText:
		*r = PeerRoleOrigin
	case authenticationRunnerRoleText:
		*r = PeerRoleRunner
	case authenticationControlRoleText:
		*r = PeerRoleControl
	default:
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	return nil
}

type AuthenticatedPeer struct {
	Origin     *standard.OriginIdentity      `json:"origin,omitempty"`
	Machine    *standard.MachineID           `json:"machine_id,omitempty"`
	Generation *standard.MachineGenerationID `json:"machine_generation_id,omitempty"`
	Credential PeerCredential                `json:"credential"`
	Role       PeerRole                      `json:"role"`
}

func (p AuthenticatedPeer) Validate() error {
	if err := errors.Join(p.Role.Validate(), p.Credential.Validate()); err != nil {
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
	ResolvePeer(context.Context, PeerCredential, PeerRole) (AuthenticatedPeer, error)
}

type RequestAuthenticator interface {
	Authenticate(exchange.SocketServerCall, PeerRole) (AuthenticatedPeer, error)
}

type MutualTLSAuthenticator struct{ repository PeerIdentityRepository }

type authenticatedPeerContextKey struct{}

func BindAuthenticatedPeer(call exchange.SocketServerCall, peer AuthenticatedPeer) (exchange.SocketServerCall, error) {
	if err := peer.Validate(); err != nil {
		return exchange.SocketServerCall{}, err
	}
	ctx, err := call.Context()
	if err != nil {
		return exchange.SocketServerCall{}, err
	}
	return call.WithContext(context.WithValue(ctx, authenticatedPeerContextKey{}, peer))
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

func RequireRunnerPeer(ctx context.Context, machine standard.MachineID, generation standard.MachineGenerationID) error {
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

func (a MutualTLSAuthenticator) Authenticate(call exchange.SocketServerCall, role PeerRole) (AuthenticatedPeer, error) {
	if err := validateAuthenticationRequest(call, a.repository, role); err != nil {
		return AuthenticatedPeer{}, err
	}
	digest, err := call.VerifiedClientCertificateDigest()
	if err != nil {
		return AuthenticatedPeer{}, errors.Join(core.ErrPrimitiveContract, err)
	}
	credential, err := NewPeerCredential(PeerCredentialMutualTLS, digest)
	if err != nil {
		return AuthenticatedPeer{}, err
	}
	ctx, err := call.Context()
	if err != nil {
		return AuthenticatedPeer{}, err
	}
	peer, err := a.repository.ResolvePeer(ctx, credential, role)
	if err != nil {
		return AuthenticatedPeer{}, err
	}
	if err := peer.Validate(); err != nil {
		return AuthenticatedPeer{}, errors.Join(core.ErrPrimitiveContract, err)
	}
	if peer.Role != role || peer.Credential != credential {
		return AuthenticatedPeer{}, core.ErrPrimitiveContract
	}
	return peer, nil
}

func validateAuthenticationRequest(call exchange.SocketServerCall, repository PeerIdentityRepository, role PeerRole) error {
	if repository == nil {
		return core.ErrPrimitiveContract
	}
	if err := call.Validate(); err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	if err := role.Validate(); err != nil {
		return core.ErrPrimitiveContract
	}
	if _, err := call.VerifiedClientCertificateDigest(); err != nil {
		return core.ErrPrimitiveContract
	}
	return nil
}

var (
	_ core.Validatable     = PeerRoleUnknown
	_ core.Validatable     = PeerCredentialUnknown
	_ json.Unmarshaler     = (*PeerCredentialKind)(nil)
	_ core.Validatable     = PeerCredential{}
	_ json.Unmarshaler     = (*PeerRole)(nil)
	_ core.Validatable     = AuthenticatedPeer{}
	_ RequestAuthenticator = MutualTLSAuthenticator{}
)
