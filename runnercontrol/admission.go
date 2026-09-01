package runnercontrol

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"net/http"
	"slices"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/projectstandards"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	AdmissionRequestMaximumBytes  = 256 * 1024
	AdmissionResponseMaximumBytes = 512 * 1024
	RunWorkerMaximum              = 256
	MachineSessionMaximumHours    = 10
	admissionIdempotencyNamespace = "runner-control-request:"
)

type RunLimits struct {
	Duration       temporal.Duration `json:"duration"`
	OutputBytes    core.ByteCount    `json:"output_bytes"`
	ArtifactBytes  core.ByteCount    `json:"artifact_bytes"`
	ArtifactCount  uint16            `json:"artifact_count"`
	WorkerMaximum  uint16            `json:"worker_maximum"`
	ProcessMaximum uint16            `json:"process_maximum"`
	FileMaximum    uint32            `json:"file_maximum"`
	QueueDepth     uint16            `json:"queue_depth"`
}

func (l RunLimits) Validate() error {
	if err := errors.Join(l.Duration.Validate(), l.OutputBytes.Validate(), l.ArtifactBytes.Validate()); err != nil {
		return err
	}
	maximum, err := temporal.DurationFromHours(MachineSessionMaximumHours)
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	if l.Duration.IsZero() || l.Duration.Nanoseconds() > maximum.Nanoseconds() {
		return core.ErrPrimitiveContract
	}
	return l.validateCounts()
}

func (l RunLimits) validateCounts() error {
	if l.ArtifactCount == 0 || l.ArtifactCount > projectstandards.ArtifactReferenceMaximum {
		return core.ErrPrimitiveContract
	}
	if l.WorkerMaximum == 0 || l.WorkerMaximum > RunWorkerMaximum {
		return core.ErrPrimitiveContract
	}
	if l.ProcessMaximum == 0 || l.FileMaximum == 0 || l.QueueDepth == 0 {
		return core.ErrPrimitiveContract
	}
	return nil
}

type RequestedRun struct {
	SchemaVersion uint16                           `json:"schema_version"`
	Request       projectstandards.RequestIdentity `json:"request_id"`
	Probe         projectstandards.RequestedProbe  `json:"requested_probe"`
	Limits        RunLimits                        `json:"limits"`
	EvidencePlan  core.SHA256Digest                `json:"evidence_plan_digest"`
	RequestedAt   temporal.Instant                 `json:"requested_at"`
}

func (r RequestedRun) Validate() error {
	if r.SchemaVersion != SchemaVersion {
		return core.ErrPrimitiveContract
	}
	return errors.Join(r.Request.Validate(), r.Probe.Validate(), r.Limits.Validate(), r.EvidencePlan.Validate(), r.RequestedAt.Validate())
}

func (r RequestedRun) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONDocument(requestedRunWire(r))
}

func (r *RequestedRun) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[requestedRunWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := RequestedRun(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*r = candidate
	return nil
}

func (r RequestedRun) IdempotencyKey() (exchange.IdempotencyKey, error) {
	if err := r.Validate(); err != nil {
		return exchange.IdempotencyKey{}, err
	}
	encoded, err := r.Request.MarshalJSON()
	if err != nil {
		return exchange.IdempotencyKey{}, err
	}
	value, err := core.DecodeJSONStringToken(encoded)
	if err != nil {
		return exchange.IdempotencyKey{}, err
	}
	return exchange.ParseIdempotencyKey(admissionIdempotencyNamespace + value)
}

type RepositoryGrant struct {
	Identity         core.SHA256Digest                   `json:"identity"`
	Origin           projectstandards.OriginIdentity     `json:"origin"`
	Subject          projectstandards.SubjectIdentity    `json:"subject"`
	Repository       projectstandards.RepositoryIdentity `json:"repository"`
	SourceAuthority  core.HTTPEndpoint                   `json:"source_authority"`
	CredentialIssuer core.HTTPEndpoint                   `json:"credential_issuer"`
	Enabled          bool                                `json:"enabled"`
	ExpiresAt        temporal.Instant                    `json:"expires_at"`
}

func (g RepositoryGrant) Validate() error {
	if err := errors.Join(g.Identity.Validate(), g.Origin.Validate(), g.Subject.Validate(), g.Repository.Validate(), g.SourceAuthority.Validate(), g.CredentialIssuer.Validate(), g.ExpiresAt.Validate()); err != nil {
		return err
	}
	if !g.Enabled || g.Subject.Repository != g.Repository || g.SourceAuthority.HTTPURL().Scheme != core.SchemeHTTPS || g.CredentialIssuer.HTTPURL().Scheme != core.SchemeHTTPS {
		return core.ErrPrimitiveContract
	}
	if g.Identity != repositoryGrantDigest(g) {
		return core.ErrPrimitiveContract
	}
	return nil
}

func NewRepositoryGrant(grant RepositoryGrant) (RepositoryGrant, error) {
	grant.Identity = repositoryGrantDigest(grant)
	return grant, grant.Validate()
}

func repositoryGrantDigest(g RepositoryGrant) core.SHA256Digest {
	type projection struct {
		Origin           projectstandards.OriginIdentity     `json:"origin"`
		Subject          projectstandards.SubjectIdentity    `json:"subject"`
		Repository       projectstandards.RepositoryIdentity `json:"repository"`
		SourceAuthority  core.HTTPEndpoint                   `json:"source_authority"`
		CredentialIssuer core.HTTPEndpoint                   `json:"credential_issuer"`
		Enabled          bool                                `json:"enabled"`
		ExpiresAt        temporal.Instant                    `json:"expires_at"`
	}
	encoded, err := core.MarshalCanonicalJSONDocument(projection{g.Origin, g.Subject, g.Repository, g.SourceAuthority, g.CredentialIssuer, g.Enabled, g.ExpiresAt})
	if err != nil {
		return core.SHA256Digest{}
	}
	return core.SHA256Of(encoded)
}

type OriginDeliveryGrant struct {
	Identity    core.SHA256Digest               `json:"identity"`
	Origin      projectstandards.OriginIdentity `json:"origin"`
	Endpoint    core.HTTPEndpoint               `json:"endpoint"`
	Credential  PeerCredential                  `json:"credential"`
	Audience    projectstandards.Identifier     `json:"audience"`
	Application projectstandards.Identifier     `json:"application"`
	Enabled     bool                            `json:"enabled"`
	ExpiresAt   temporal.Instant                `json:"expires_at"`
}

func (g OriginDeliveryGrant) Validate() error {
	if err := errors.Join(g.Identity.Validate(), g.Origin.Validate(), g.Endpoint.Validate(), g.Credential.Validate(), g.Audience.Validate(), g.Application.Validate(), g.ExpiresAt.Validate()); err != nil {
		return err
	}
	if !g.Enabled || g.Endpoint.HTTPURL().Scheme != core.SchemeHTTPS {
		return core.ErrPrimitiveContract
	}
	if g.Identity != originDeliveryGrantDigest(g) {
		return core.ErrPrimitiveContract
	}
	return nil
}

func NewOriginDeliveryGrant(grant OriginDeliveryGrant) (OriginDeliveryGrant, error) {
	grant.Identity = originDeliveryGrantDigest(grant)
	return grant, grant.Validate()
}

func originDeliveryGrantDigest(g OriginDeliveryGrant) core.SHA256Digest {
	type projection struct {
		Origin      projectstandards.OriginIdentity `json:"origin"`
		Endpoint    core.HTTPEndpoint               `json:"endpoint"`
		Credential  PeerCredential                  `json:"credential"`
		Audience    projectstandards.Identifier     `json:"audience"`
		Application projectstandards.Identifier     `json:"application"`
		Enabled     bool                            `json:"enabled"`
		ExpiresAt   temporal.Instant                `json:"expires_at"`
	}
	encoded, err := core.MarshalCanonicalJSONDocument(projection{g.Origin, g.Endpoint, g.Credential, g.Audience, g.Application, g.Enabled, g.ExpiresAt})
	if err != nil {
		return core.SHA256Digest{}
	}
	return core.SHA256Of(encoded)
}

type SourceGrant struct {
	Identity        SourceGrantIdentity               `json:"identity"`
	RepositoryGrant core.SHA256Digest                 `json:"repository_grant"`
	Source          projectstandards.SourceCoordinate `json:"source"`
	Authority       core.HTTPEndpoint                 `json:"authority"`
	Credential      projectstandards.Identifier       `json:"credential_custody"`
	ExpiresAt       temporal.Instant                  `json:"expires_at"`
}

func (g SourceGrant) Validate() error {
	if err := errors.Join(g.Identity.Validate(), g.RepositoryGrant.Validate(), g.Source.Validate(), g.Authority.Validate(), g.Credential.Validate(), g.ExpiresAt.Validate()); err != nil {
		return err
	}
	if g.Authority.HTTPURL().Scheme != core.SchemeHTTPS {
		return core.ErrPrimitiveContract
	}
	return nil
}

func NewSourceGrant(grant SourceGrant) (SourceGrant, error) {
	grant.Identity = SourceGrantIdentity{Digest: sourceGrantDigest(grant)}
	return grant, grant.Validate()
}

type AdmittedRun struct {
	SchemaVersion uint16                           `json:"schema_version"`
	Request       projectstandards.RequestIdentity `json:"request_id"`
	Run           projectstandards.RunID           `json:"run_id"`
	Requested     projectstandards.RequestedProbe  `json:"requested_probe"`
	Probe         projectstandards.ProbeIdentity   `json:"admitted_probe"`
	Limits        RunLimits                        `json:"limits"`
	EvidencePlan  core.SHA256Digest                `json:"evidence_plan_digest"`
	Repository    RepositoryGrant                  `json:"repository_grant"`
	Delivery      OriginDeliveryGrant              `json:"origin_delivery_grant"`
	Source        SourceGrant                      `json:"source_grant"`
	AdmittedAt    temporal.Instant                 `json:"admitted_at"`
}

func (r AdmittedRun) Validate() error {
	if r.SchemaVersion != SchemaVersion {
		return core.ErrPrimitiveContract
	}
	if err := errors.Join(r.Request.Validate(), r.Run.Validate(), r.Requested.Validate(), r.Probe.Validate(), r.Limits.Validate(), r.EvidencePlan.Validate(), r.Repository.Validate(), r.Delivery.Validate(), r.Source.Validate(), r.AdmittedAt.Validate()); err != nil {
		return err
	}
	if r.Repository.Origin != r.Probe.Origin || r.Repository.Subject != r.Probe.Subject || r.Repository.Repository != r.Probe.Source.Repository || r.Delivery.Origin != r.Probe.Origin || r.Source.RepositoryGrant != r.Repository.Identity || r.Source.Source != r.Probe.Source || r.Source.Identity.Digest != sourceGrantDigest(r.Source) {
		return core.ErrPrimitiveContract
	}
	return validateProbeDescent(r.Requested, r.Probe)
}

func sourceGrantDigest(grant SourceGrant) core.SHA256Digest {
	type identityInput struct {
		RepositoryGrant core.SHA256Digest                 `json:"repository_grant"`
		Source          projectstandards.SourceCoordinate `json:"source"`
		Authority       core.HTTPEndpoint                 `json:"authority"`
		Credential      projectstandards.Identifier       `json:"credential_custody"`
		ExpiresAt       temporal.Instant                  `json:"expires_at"`
	}
	encoded, err := core.MarshalCanonicalJSONDocument(identityInput{RepositoryGrant: grant.RepositoryGrant, Source: grant.Source, Authority: grant.Authority, Credential: grant.Credential, ExpiresAt: grant.ExpiresAt})
	if err != nil {
		return core.SHA256Digest{}
	}
	return core.SHA256Of(encoded)
}

func validateProbeDescent(requested projectstandards.RequestedProbe, admitted projectstandards.ProbeIdentity) error {
	if !probeDescentScalarsMatch(requested, admitted) {
		return core.ErrPrimitiveContract
	}
	left, leftErr := core.MarshalCanonicalJSONDocument(admitted.Target)
	right, rightErr := core.MarshalCanonicalJSONDocument(requested.Target)
	if leftErr != nil || rightErr != nil {
		return errors.Join(core.ErrPrimitiveContract, leftErr, rightErr)
	}
	if !bytes.Equal(left, right) {
		return core.ErrPrimitiveContract
	}
	if admitted.Role == projectstandards.ProbeRoleSelection {
		return nil
	}
	if slices.Contains(requested.Kinds, admitted.Kind) {
		return nil
	}
	return core.ErrPrimitiveContract
}

func probeDescentScalarsMatch(requested projectstandards.RequestedProbe, admitted projectstandards.ProbeIdentity) bool {
	return admitted.Origin == requested.Origin && admitted.Subject == requested.Subject && admitted.Source == requested.Source && admitted.Profile == requested.Profile && admitted.Environment.Satisfies(requested.Constraints)
}

type AdmissionResponse struct {
	SchemaVersion uint16                           `json:"schema_version"`
	Request       projectstandards.RequestIdentity `json:"request_id"`
	Admitted      *AdmittedRun                     `json:"admitted,omitempty"`
	Refusal       *projectstandards.RefusalReason  `json:"refusal,omitempty"`
}

func (r AdmissionResponse) Validate() error {
	if r.SchemaVersion != SchemaVersion {
		return core.ErrPrimitiveContract
	}
	if err := r.Request.Validate(); err != nil || (r.Admitted == nil) == (r.Refusal == nil) {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	if r.Admitted != nil {
		if err := r.Admitted.Validate(); err != nil || r.Admitted.Request != r.Request {
			return errors.Join(core.ErrPrimitiveContract, err)
		}
		return nil
	}
	return r.Refusal.Validate()
}

type requestedRunWire RequestedRun
type admittedRunWire AdmittedRun
type admissionResponseWire AdmissionResponse

func (r AdmittedRun) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONDocument(admittedRunWire(r))
}
func (r *AdmittedRun) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[admittedRunWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := AdmittedRun(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*r = candidate
	return nil
}
func (r AdmissionResponse) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONDocument(admissionResponseWire(r))
}
func (r *AdmissionResponse) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[admissionResponseWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := AdmissionResponse(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*r = candidate
	return nil
}

type AdmissionRepository interface {
	Admit(context.Context, AuthenticatedAdmissionRequest) (AdmissionResponse, error)
}

type AuthenticatedAdmissionRequest struct {
	Peer      AuthenticatedPeer
	Requested RequestedRun
}

func (r AuthenticatedAdmissionRequest) Validate() error {
	if err := errors.Join(r.Peer.Validate(), r.Requested.Validate()); err != nil {
		return err
	}
	if r.Peer.Role != PeerRoleOrigin || r.Peer.Origin == nil || *r.Peer.Origin != r.Requested.Probe.Origin {
		return core.ErrPrimitiveContract
	}
	return nil
}

type AdmissionClient struct{ socket exchange.ClientSocket }

func NewAdmissionClient(configuration exchange.ClientSocketConfiguration) (AdmissionClient, error) {
	socket, err := exchange.NewClientSocket(configuration)
	if err != nil {
		return AdmissionClient{}, err
	}
	return AdmissionClient{socket: socket}, nil
}

func (c AdmissionClient) Submit(ctx context.Context, request RequestedRun) (exchange.JSONResponse[AdmissionResponse], error) {
	return exchange.SendReplayBoundSocketJSON[RequestedRun, AdmissionResponse](ctx, c.socket, request)
}

type AdmissionServer struct {
	socket     exchange.ServerSocket
	repository AdmissionRepository
}

func NewAdmissionServer(contract exchange.JSONSocketContract, repository AdmissionRepository) (AdmissionServer, error) {
	if repository == nil {
		return AdmissionServer{}, core.ErrPrimitiveContract
	}
	socket, err := exchange.NewServerSocket(contract)
	if err != nil {
		return AdmissionServer{}, err
	}
	return AdmissionServer{socket: socket, repository: repository}, nil
}

func (s AdmissionServer) Serve(writer http.ResponseWriter, request *http.Request) error {
	return core.ErrPrimitiveContract
}

func (s AdmissionServer) ServeAuthenticated(writer http.ResponseWriter, request *http.Request, peer AuthenticatedPeer) error {
	if s.repository == nil {
		return core.ErrPrimitiveContract
	}
	received, err := exchange.ReceiveReplayBoundSocketJSON[RequestedRun, *RequestedRun](s.socket, request)
	if err != nil {
		return err
	}
	authenticated := AuthenticatedAdmissionRequest{Peer: peer, Requested: *received.Body}
	if err := authenticated.Validate(); err != nil {
		return err
	}
	response, err := s.repository.Admit(request.Context(), authenticated)
	if err != nil {
		return err
	}
	if response.Request != received.Body.Request {
		return core.ErrPrimitiveContract
	}
	return exchange.WriteSocketJSON(s.socket, writer, response)
}

func AdmissionSocketContract(path exchange.SocketRoutePath) (exchange.JSONSocketContract, error) {
	requestLimit, requestErr := core.NewByteCount(AdmissionRequestMaximumBytes)
	responseLimit, responseErr := core.NewByteCount(AdmissionResponseMaximumBytes)
	var accepted core.HTTPStatusCode
	statusErr := accepted.AdmitInt(http.StatusAccepted)
	if err := errors.Join(path.Validate(), requestErr, responseErr, statusErr); err != nil {
		return exchange.JSONSocketContract{}, err
	}
	contract := exchange.JSONSocketContract{Path: path, Route: exchange.RouteSemantics{Method: exchange.MethodPost, Replay: exchange.ReplayIdempotencyKey}, RequestBodyLimit: requestLimit, ResponseBodyLimit: responseLimit, SuccessStatus: accepted}
	return contract, contract.Validate()
}

var (
	_ core.Validatable            = RunLimits{}
	_ core.ValidatedJSONMarshaler = RequestedRun{}
	_ json.Unmarshaler            = (*RequestedRun)(nil)
	_ exchange.IdempotencyBound   = RequestedRun{}
	_ core.Validatable            = RepositoryGrant{}
	_ core.Validatable            = OriginDeliveryGrant{}
	_ core.Validatable            = SourceGrant{}
	_ core.ValidatedJSONMarshaler = AdmittedRun{}
	_ json.Unmarshaler            = (*AdmittedRun)(nil)
	_ core.ValidatedJSONMarshaler = AdmissionResponse{}
	_ json.Unmarshaler            = (*AdmissionResponse)(nil)
	_ core.Validatable            = AuthenticatedAdmissionRequest{}
)
