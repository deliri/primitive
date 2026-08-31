package runnercontrol

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"net/http"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/projectstandards"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	RunStateRequestMaximumBytes  = 64 * 1024
	RunStateResponseMaximumBytes = core.JSONDocumentMaximumBytes
)

type RunControlState uint8

const (
	RunControlStateUnknown RunControlState = iota
	RunControlQueued
	RunControlStarting
	RunControlReady
	RunControlExecuting
	RunControlCancellationRequested
	RunControlCleaning
	RunControlDelivering
	RunControlDelivered
	RunControlInfrastructureFailed
	runControlStateLimit
)

func (s RunControlState) Validate() error {
	if s <= RunControlStateUnknown || s >= runControlStateLimit {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (s RunControlState) String() string {
	switch s {
	case RunControlQueued:
		return "queued"
	case RunControlStarting:
		return "starting"
	case RunControlReady:
		return "ready"
	case RunControlExecuting:
		return "executing"
	case RunControlCancellationRequested:
		return "cancellation-requested"
	case RunControlCleaning:
		return "cleaning"
	case RunControlDelivering:
		return "delivering"
	case RunControlDelivered:
		return "delivered"
	case RunControlInfrastructureFailed:
		return "infrastructure-failed"
	default:
		return ""
	}
}

func (s RunControlState) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(s.String())
}

func (s *RunControlState) UnmarshalJSON(data []byte) error {
	if s == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	for candidate := RunControlStateUnknown + 1; candidate < runControlStateLimit; candidate++ {
		if candidate.String() == value {
			*s = candidate
			return nil
		}
	}
	return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
}

type RunStateRequest struct {
	SchemaVersion uint16                 `json:"schema_version"`
	Run           projectstandards.RunID `json:"run_id"`
	RequestedAt   temporal.Instant       `json:"requested_at"`
}

func (r RunStateRequest) Validate() error {
	if r.SchemaVersion != SchemaVersion {
		return core.ErrPrimitiveContract
	}
	return errors.Join(r.Run.Validate(), r.RequestedAt.Validate())
}

type RunStateResponse struct {
	SchemaVersion uint16                 `json:"schema_version"`
	Run           projectstandards.RunID `json:"run_id"`
	State         RunControlState        `json:"state"`
	UpdatedAt     temporal.Instant       `json:"updated_at"`
	Observation   *ObservationEnvelope   `json:"observation,omitempty"`
}

func (r RunStateResponse) Validate() error {
	if r.SchemaVersion != SchemaVersion {
		return core.ErrPrimitiveContract
	}
	if err := errors.Join(r.Run.Validate(), r.State.Validate(), r.UpdatedAt.Validate()); err != nil {
		return err
	}
	if r.State == RunControlDelivered {
		if r.Observation == nil {
			return errors.Join(core.ErrPrimitiveContract, errors.New("delivered run state has no immutable observation"))
		}
		if err := r.Observation.Validate(); err != nil || r.Observation.Payload.Run != r.Run {
			return errors.Join(core.ErrPrimitiveContract, err, errors.New("delivered observation does not bind the requested run"))
		}
		return nil
	}
	if r.Observation != nil {
		return errors.Join(core.ErrPrimitiveContract, errors.New("non-delivered run state carries a final observation"))
	}
	return nil
}

type CancellationIdentity struct {
	Digest core.SHA256Digest `json:"digest"`
}

func (i CancellationIdentity) Validate() error { return i.Digest.Validate() }

type CancellationRequest struct {
	SchemaVersion uint16                 `json:"schema_version"`
	Identity      CancellationIdentity   `json:"cancellation_id"`
	Run           projectstandards.RunID `json:"run_id"`
	RequestedAt   temporal.Instant       `json:"requested_at"`
}

func (r CancellationRequest) Validate() error {
	if r.SchemaVersion != SchemaVersion {
		return core.ErrPrimitiveContract
	}
	return errors.Join(r.Identity.Validate(), r.Run.Validate(), r.RequestedAt.Validate())
}

func (r CancellationRequest) IdempotencyKey() (exchange.IdempotencyKey, error) {
	if err := r.Validate(); err != nil {
		return exchange.IdempotencyKey{}, err
	}
	hex, err := r.Identity.Digest.Hex()
	if err != nil {
		return exchange.IdempotencyKey{}, err
	}
	return exchange.ParseIdempotencyKey("anvil-cancellation:" + hex)
}

type CancellationResponse struct {
	SchemaVersion uint16                 `json:"schema_version"`
	Identity      CancellationIdentity   `json:"cancellation_id"`
	Run           projectstandards.RunID `json:"run_id"`
	State         RunControlState        `json:"state"`
	RecordedAt    temporal.Instant       `json:"recorded_at"`
}

func (r CancellationResponse) Validate() error {
	if r.SchemaVersion != SchemaVersion {
		return core.ErrPrimitiveContract
	}
	if err := errors.Join(r.Identity.Validate(), r.Run.Validate(), r.State.Validate(), r.RecordedAt.Validate()); err != nil {
		return err
	}
	if r.State != RunControlCancellationRequested && r.State != RunControlCleaning && r.State != RunControlDelivering && r.State != RunControlDelivered && r.State != RunControlInfrastructureFailed {
		return errors.Join(core.ErrPrimitiveContract, errors.New("cancellation response does not expose cancellation or a terminally later state"))
	}
	return nil
}

type runStateRequestWire RunStateRequest
type runStateResponseWire RunStateResponse
type cancellationRequestWire CancellationRequest
type cancellationResponseWire CancellationResponse

func (r RunStateRequest) MarshalJSON() ([]byte, error) {
	return marshalRunState(r, runStateRequestWire(r))
}
func (r RunStateResponse) MarshalJSON() ([]byte, error) {
	return marshalRunState(r, runStateResponseWire(r))
}
func (r CancellationRequest) MarshalJSON() ([]byte, error) {
	return marshalRunState(r, cancellationRequestWire(r))
}
func (r CancellationResponse) MarshalJSON() ([]byte, error) {
	return marshalRunState(r, cancellationResponseWire(r))
}
func marshalRunState[T core.Validatable, W any](value T, wire W) ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONDocument(wire)
}

func (r *RunStateRequest) UnmarshalJSON(data []byte) error {
	return unmarshalRunState(data, r, func(w runStateRequestWire) RunStateRequest { return RunStateRequest(w) })
}
func (r *RunStateResponse) UnmarshalJSON(data []byte) error {
	return unmarshalRunState(data, r, func(w runStateResponseWire) RunStateResponse { return RunStateResponse(w) })
}
func (r *CancellationRequest) UnmarshalJSON(data []byte) error {
	return unmarshalRunState(data, r, func(w cancellationRequestWire) CancellationRequest { return CancellationRequest(w) })
}
func (r *CancellationResponse) UnmarshalJSON(data []byte) error {
	return unmarshalRunState(data, r, func(w cancellationResponseWire) CancellationResponse { return CancellationResponse(w) })
}

func unmarshalRunState[W any, T core.Validatable](data []byte, target *T, convert func(W) T) error {
	if target == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[W](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := convert(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*target = candidate
	return nil
}

type AuthenticatedRunStateRequest struct {
	Peer    AuthenticatedPeer
	Request RunStateRequest
}

func (r AuthenticatedRunStateRequest) Validate() error {
	if err := errors.Join(r.Peer.Validate(), r.Request.Validate()); err != nil {
		return err
	}
	if r.Peer.Role != PeerRoleOrigin || r.Peer.Origin == nil {
		return core.ErrPrimitiveContract
	}
	return nil
}

type AuthenticatedCancellationRequest struct {
	Peer    AuthenticatedPeer
	Request CancellationRequest
}

func (r AuthenticatedCancellationRequest) Validate() error {
	if err := errors.Join(r.Peer.Validate(), r.Request.Validate()); err != nil {
		return err
	}
	if r.Peer.Role != PeerRoleOrigin || r.Peer.Origin == nil {
		return core.ErrPrimitiveContract
	}
	return nil
}

type RunStateRepository interface {
	RunState(context.Context, AuthenticatedRunStateRequest) (RunStateResponse, error)
}

type CancellationRepository interface {
	Cancel(context.Context, AuthenticatedCancellationRequest) (CancellationResponse, error)
}

type RunStateClient struct{ socket exchange.ClientSocket }
type CancellationClient struct{ socket exchange.ClientSocket }
type RunStateServer struct {
	socket     exchange.ServerSocket
	repository RunStateRepository
}
type CancellationServer struct {
	socket     exchange.ServerSocket
	repository CancellationRepository
}

func NewRunStateClient(configuration exchange.ClientSocketConfiguration) (RunStateClient, error) {
	socket, err := exchange.NewClientSocket(configuration)
	return RunStateClient{socket: socket}, err
}

func NewCancellationClient(configuration exchange.ClientSocketConfiguration) (CancellationClient, error) {
	socket, err := exchange.NewClientSocket(configuration)
	return CancellationClient{socket: socket}, err
}

func (c RunStateClient) Fetch(ctx context.Context, request RunStateRequest) (exchange.JSONResponse[RunStateResponse], error) {
	return exchange.SendSocketJSON[RunStateRequest, RunStateResponse](ctx, c.socket, request)
}

func (c CancellationClient) Cancel(ctx context.Context, request CancellationRequest) (exchange.JSONResponse[CancellationResponse], error) {
	return exchange.SendReplayBoundSocketJSON[CancellationRequest, CancellationResponse](ctx, c.socket, request)
}

func NewRunStateServer(contract exchange.JSONSocketContract, repository RunStateRepository) (RunStateServer, error) {
	if repository == nil {
		return RunStateServer{}, core.ErrPrimitiveContract
	}
	socket, err := exchange.NewServerSocket(contract)
	return RunStateServer{socket: socket, repository: repository}, err
}

func NewCancellationServer(contract exchange.JSONSocketContract, repository CancellationRepository) (CancellationServer, error) {
	if repository == nil {
		return CancellationServer{}, core.ErrPrimitiveContract
	}
	socket, err := exchange.NewServerSocket(contract)
	return CancellationServer{socket: socket, repository: repository}, err
}

func (s RunStateServer) ServeAuthenticated(writer http.ResponseWriter, request *http.Request, peer AuthenticatedPeer) error {
	received, err := exchange.ReceiveSocketJSON[RunStateRequest, *RunStateRequest](s.socket, request)
	if err != nil {
		return err
	}
	authenticated := AuthenticatedRunStateRequest{Peer: peer, Request: *received.Body}
	if err := authenticated.Validate(); err != nil {
		return err
	}
	response, err := s.repository.RunState(request.Context(), authenticated)
	if err != nil {
		return err
	}
	if response.Run != received.Body.Run {
		return core.ErrPrimitiveContract
	}
	return exchange.WriteSocketJSON(s.socket, writer, response)
}

func (s CancellationServer) ServeAuthenticated(writer http.ResponseWriter, request *http.Request, peer AuthenticatedPeer) error {
	received, err := exchange.ReceiveReplayBoundSocketJSON[CancellationRequest, *CancellationRequest](s.socket, request)
	if err != nil {
		return err
	}
	authenticated := AuthenticatedCancellationRequest{Peer: peer, Request: *received.Body}
	if err := authenticated.Validate(); err != nil {
		return err
	}
	response, err := s.repository.Cancel(request.Context(), authenticated)
	if err != nil {
		return err
	}
	if response.Run != received.Body.Run || response.Identity != received.Body.Identity {
		return core.ErrPrimitiveContract
	}
	return exchange.WriteSocketJSON(s.socket, writer, response)
}

func RunStateSocketContract(path exchange.SocketRoutePath) (exchange.JSONSocketContract, error) {
	return runStateSocketContract(path, exchange.ReplaySingleAttempt)
}

func CancellationSocketContract(path exchange.SocketRoutePath) (exchange.JSONSocketContract, error) {
	return runStateSocketContract(path, exchange.ReplayIdempotencyKey)
}

func runStateSocketContract(path exchange.SocketRoutePath, replay exchange.ReplayMode) (exchange.JSONSocketContract, error) {
	requestLimit, requestErr := core.NewByteCount(RunStateRequestMaximumBytes)
	responseLimit, responseErr := core.NewByteCount(RunStateResponseMaximumBytes)
	if err := errors.Join(path.Validate(), requestErr, responseErr); err != nil {
		return exchange.JSONSocketContract{}, err
	}
	contract := exchange.JSONSocketContract{Path: path, Route: exchange.RouteSemantics{Method: exchange.MethodPost, Replay: replay}, RequestBodyLimit: requestLimit, ResponseBodyLimit: responseLimit, SuccessStatus: core.HTTPStatusOK()}
	return contract, contract.Validate()
}

var (
	_ core.Validatable            = RunControlStateUnknown
	_ json.Unmarshaler            = (*RunControlState)(nil)
	_ core.ValidatedJSONMarshaler = RunStateRequest{}
	_ json.Unmarshaler            = (*RunStateRequest)(nil)
	_ core.ValidatedJSONMarshaler = RunStateResponse{}
	_ json.Unmarshaler            = (*RunStateResponse)(nil)
	_ core.ValidatedJSONMarshaler = CancellationRequest{}
	_ json.Unmarshaler            = (*CancellationRequest)(nil)
	_ exchange.IdempotencyBound   = CancellationRequest{}
	_ core.ValidatedJSONMarshaler = CancellationResponse{}
	_ json.Unmarshaler            = (*CancellationResponse)(nil)
)
