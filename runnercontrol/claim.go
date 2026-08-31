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
	SchemaVersion             uint16 = 1
	ClaimRequestMaximumBytes         = 64 * 1024
	ClaimResponseMaximumBytes        = core.JSONDocumentMaximumBytes
)

type ClaimKind uint8

const (
	ClaimKindUnknown ClaimKind = iota
	ClaimWait
	ClaimExecute
	ClaimDrain
	claimKindLimit
)

func claimKindLabels() []string { return []string{"", "wait", "execute", "drain"} }

func (k ClaimKind) Validate() error {
	if k <= ClaimKindUnknown || k >= claimKindLimit {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (k ClaimKind) String() string {
	if k.Validate() != nil {
		return ""
	}
	return claimKindLabels()[k]
}

func (k ClaimKind) MarshalJSON() ([]byte, error) {
	if err := k.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(k.String())
}

func (k *ClaimKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	for candidate := ClaimWait; candidate < claimKindLimit; candidate++ {
		if candidate.String() == value {
			*k = candidate
			return nil
		}
	}
	return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
}

// MachineFence is the control-owned authority for one observed machine generation.
type MachineFence struct {
	Machine    projectstandards.MachineID           `json:"machine_id"`
	Generation projectstandards.MachineGenerationID `json:"generation_id"`
	Epoch      uint64                               `json:"epoch"`
	ExpiresAt  temporal.Instant                     `json:"expires_at"`
}

func (f MachineFence) Validate() error {
	if err := errors.Join(f.Machine.Validate(), f.Generation.Validate(), f.ExpiresAt.Validate()); err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	if f.Epoch == 0 {
		return core.ErrPrimitiveContract
	}
	return nil
}

type ClaimRequest struct {
	SchemaVersion uint16                                `json:"schema_version"`
	Machine       projectstandards.MachineID            `json:"machine_id"`
	Generation    projectstandards.MachineGenerationID  `json:"generation_id"`
	Observation   projectstandards.MachineObservationID `json:"observation_id"`
	RequestedAt   temporal.Instant                      `json:"requested_at"`
}

func (r ClaimRequest) Validate() error {
	if r.SchemaVersion != SchemaVersion {
		return core.ErrPrimitiveContract
	}
	return errors.Join(r.Machine.Validate(), r.Generation.Validate(), r.Observation.Validate(), r.RequestedAt.Validate())
}

type ClaimResponse struct {
	SchemaVersion uint16           `json:"schema_version"`
	Kind          ClaimKind        `json:"kind"`
	Fence         MachineFence     `json:"fence"`
	Scheduling    *SchedulingClaim `json:"scheduling,omitempty"`
}

func (r ClaimResponse) Validate() error {
	if r.SchemaVersion != SchemaVersion {
		return core.ErrPrimitiveContract
	}
	if err := errors.Join(r.Kind.Validate(), r.Fence.Validate()); err != nil {
		return err
	}
	if r.Kind == ClaimExecute {
		if r.Scheduling == nil {
			return core.ErrPrimitiveContract
		}
		if err := r.Scheduling.Validate(); err != nil {
			return err
		}
		if r.Scheduling.Capability.Fence.Machine != r.Fence {
			return core.ErrPrimitiveContract
		}
		return nil
	}
	if r.Scheduling != nil {
		return core.ErrPrimitiveContract
	}
	return nil
}

type claimRequestWire ClaimRequest
type claimResponseWire ClaimResponse

func (r ClaimRequest) MarshalJSON() ([]byte, error)  { return marshalClaim(r, claimRequestWire(r)) }
func (r ClaimResponse) MarshalJSON() ([]byte, error) { return marshalClaim(r, claimResponseWire(r)) }

func marshalClaim[T core.Validatable, W any](value T, wire W) ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(wire)
	if err != nil {
		return nil, errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract, err)
	}
	return encoded, nil
}

func (r *ClaimRequest) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[claimRequestWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := ClaimRequest(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*r = candidate
	return nil
}

func (r *ClaimResponse) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[claimResponseWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := ClaimResponse(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*r = candidate
	return nil
}

type ClaimRepository interface {
	Claim(context.Context, ClaimRequest) (ClaimResponse, error)
}

type ClaimClient struct{ socket exchange.ClientSocket }

func NewClaimClient(configuration exchange.ClientSocketConfiguration) (ClaimClient, error) {
	socket, err := exchange.NewClientSocket(configuration)
	if err != nil {
		return ClaimClient{}, err
	}
	return ClaimClient{socket: socket}, nil
}

func (c ClaimClient) Claim(ctx context.Context, request ClaimRequest) (exchange.JSONResponse[ClaimResponse], error) {
	return exchange.SendSocketJSON[ClaimRequest, ClaimResponse](ctx, c.socket, request)
}

type ClaimServer struct {
	socket     exchange.ServerSocket
	repository ClaimRepository
}

func NewClaimServer(contract exchange.JSONSocketContract, repository ClaimRepository) (ClaimServer, error) {
	if repository == nil {
		return ClaimServer{}, core.ErrPrimitiveContract
	}
	socket, err := exchange.NewServerSocket(contract)
	if err != nil {
		return ClaimServer{}, err
	}
	return ClaimServer{socket: socket, repository: repository}, nil
}

func (s ClaimServer) Serve(writer http.ResponseWriter, request *http.Request) error {
	if s.repository == nil {
		return core.ErrPrimitiveContract
	}
	received, err := exchange.ReceiveSocketJSON[ClaimRequest, *ClaimRequest](s.socket, request)
	if err != nil {
		return err
	}
	if err := RequireRunnerPeer(request.Context(), received.Body.Machine, received.Body.Generation); err != nil {
		return err
	}
	response, err := s.repository.Claim(request.Context(), *received.Body)
	if err != nil {
		return err
	}
	if err := response.Validate(); err != nil {
		return err
	}
	return exchange.WriteSocketJSON(s.socket, writer, response)
}

func ClaimSocketContract(path exchange.SocketRoutePath) (exchange.JSONSocketContract, error) {
	requestLimit, requestErr := core.NewByteCount(ClaimRequestMaximumBytes)
	responseLimit, responseErr := core.NewByteCount(ClaimResponseMaximumBytes)
	if err := errors.Join(path.Validate(), requestErr, responseErr); err != nil {
		return exchange.JSONSocketContract{}, err
	}
	contract := exchange.JSONSocketContract{
		Path: path, Route: exchange.RouteSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt},
		RequestBodyLimit: requestLimit, ResponseBodyLimit: responseLimit, SuccessStatus: core.HTTPStatusOK(),
	}
	return contract, contract.Validate()
}

var (
	_ core.Validatable            = ClaimKindUnknown
	_ core.Validatable            = MachineFence{}
	_ core.ValidatedJSONMarshaler = ClaimRequest{}
	_ json.Unmarshaler            = (*ClaimRequest)(nil)
	_ core.ValidatedJSONMarshaler = ClaimResponse{}
	_ json.Unmarshaler            = (*ClaimResponse)(nil)
)
