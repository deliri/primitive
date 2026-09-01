package runnercontrol

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"net/http"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/projectstandards"
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
		return "mutual_tls"
	case PeerCredentialGoogleCloud:
		return "google_cloud"
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
	case "mutual_tls":
		*k = PeerCredentialMutualTLS
	case "google_cloud":
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
		return "origin"
	case PeerRoleRunner:
		return "runner"
	case PeerRoleControl:
		return "control"
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
	Role       PeerRole                              `json:"role"`
	Credential PeerCredential                        `json:"credential"`
	Origin     *projectstandards.OriginIdentity      `json:"origin,omitempty"`
	Machine    *projectstandards.MachineID           `json:"machine_id,omitempty"`
	Generation *projectstandards.MachineGenerationID `json:"machine_generation_id,omitempty"`
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
	credential, err := NewPeerCredential(PeerCredentialMutualTLS, core.SHA256Of(certificate.Raw))
	if err != nil {
		return AuthenticatedPeer{}, err
	}
	peer, err := a.repository.ResolvePeer(request.Context(), credential, role)
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
	_ core.Validatable     = PeerCredentialUnknown
	_ json.Unmarshaler     = (*PeerCredentialKind)(nil)
	_ core.Validatable     = PeerCredential{}
	_ json.Unmarshaler     = (*PeerRole)(nil)
	_ core.Validatable     = AuthenticatedPeer{}
	_ RequestAuthenticator = MutualTLSAuthenticator{}
)
