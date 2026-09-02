package runnercontrol

import (
	"context"
	json "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/standard"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	HeartbeatRequestMaximumBytes  = 64 * 1024
	HeartbeatResponseMaximumBytes = 64 * 1024
)

type HeartbeatState uint8

const (
	HeartbeatStateUnknown HeartbeatState = iota
	HeartbeatReady
	HeartbeatExecuting
	HeartbeatDraining
	heartbeatStateLimit
)

func (s HeartbeatState) Validate() error {
	if s <= HeartbeatStateUnknown || s >= heartbeatStateLimit {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (s HeartbeatState) IsValid() bool { return s.Validate() == nil }

func (s HeartbeatState) String() string {
	if !s.IsValid() {
		return invalidEnumString()
	}
	return []string{"", machineReadyStateText, machineExecutingStateText, machineDrainingStateText}[s]
}

func (s HeartbeatState) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(s.String())
}

func (s *HeartbeatState) UnmarshalJSON(data []byte) error {
	if s == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	switch value {
	case machineReadyStateText:
		*s = HeartbeatReady
	case machineExecutingStateText:
		*s = HeartbeatExecuting
	case machineDrainingStateText:
		*s = HeartbeatDraining
	default:
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	return nil
}

type DirectiveKind uint8

const (
	DirectiveUnknown DirectiveKind = iota
	DirectiveContinue
	DirectiveCancelMember
	DirectiveCancelUnit
	DirectiveRevokeLease
	DirectiveDrain
	directiveKindLimit
)

func (k DirectiveKind) Validate() error {
	if k <= DirectiveUnknown || k >= directiveKindLimit {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (k DirectiveKind) IsValid() bool { return k.Validate() == nil }

func (k DirectiveKind) String() string {
	if !k.IsValid() {
		return invalidEnumString()
	}
	return []string{"", heartbeatContinueActionText, heartbeatCancelMemberActionText, heartbeatCancelUnitActionText, heartbeatRevokeLeaseActionText, heartbeatDrainActionText}[k]
}

func (k DirectiveKind) MarshalJSON() ([]byte, error) {
	if err := k.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(k.String())
}

func (k *DirectiveKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	switch value {
	case heartbeatContinueActionText:
		*k = DirectiveContinue
	case heartbeatCancelMemberActionText:
		*k = DirectiveCancelMember
	case heartbeatCancelUnitActionText:
		*k = DirectiveCancelUnit
	case heartbeatRevokeLeaseActionText:
		*k = DirectiveRevokeLease
	case heartbeatDrainActionText:
		*k = DirectiveDrain
	default:
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	return nil
}

type Directive struct {
	Run  *standard.RunID `json:"run_id,omitempty"`
	Kind DirectiveKind   `json:"kind"`
}

func (d Directive) Validate() error {
	if err := d.Kind.Validate(); err != nil {
		return err
	}
	if d.Kind == DirectiveCancelMember {
		if d.Run == nil {
			return core.ErrPrimitiveContract
		}
		return d.Run.Validate()
	}
	if d.Run != nil {
		return core.ErrPrimitiveContract
	}
	return nil
}

type HeartbeatRequest struct {
	Scheduling    *SchedulingFence              `json:"scheduling_fence,omitempty"`
	Members       *MemberSet                    `json:"member_set,omitempty"`
	ActiveRuns    []standard.RunID              `json:"active_run_ids,omitempty"`
	Fence         MachineFence                  `json:"fence"`
	ObservedAt    temporal.Instant              `json:"observed_at"`
	SchemaVersion uint16                        `json:"schema_version"`
	Observation   standard.MachineObservationID `json:"observation_id"`
	State         HeartbeatState                `json:"state"`
}

func (r HeartbeatRequest) Validate() error {
	if r.SchemaVersion != SchemaVersion {
		return core.ErrPrimitiveContract
	}
	if err := errors.Join(r.Observation.Validate(), r.Fence.Validate(), r.State.Validate(), r.ObservedAt.Validate()); err != nil {
		return err
	}
	if r.State == HeartbeatExecuting {
		return r.validateExecuting()
	}
	if len(r.ActiveRuns) != 0 || r.Scheduling != nil || r.Members != nil {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (r HeartbeatRequest) validateExecuting() error {
	if len(r.ActiveRuns) == 0 || len(r.ActiveRuns) > SchedulingMemberMaximum || r.Scheduling == nil || r.Members == nil {
		return core.ErrPrimitiveContract
	}
	if err := errors.Join(r.Scheduling.Validate(), r.Members.Validate()); err != nil {
		return err
	}
	memberDigest, err := r.Members.Digest()
	if err != nil || r.Scheduling.Machine != r.Fence || memberDigest != r.Scheduling.MemberSetDigest || !activeRunsAreCanonicalMembers(r.ActiveRuns, *r.Members) {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	return nil
}

func activeRunsAreCanonicalMembers(active []standard.RunID, members MemberSet) bool {
	memberIndex := 0
	for index := range active {
		if active[index].Validate() != nil {
			return false
		}
		for memberIndex < len(members.Entries) && members.Entries[memberIndex] != active[index] {
			memberIndex++
		}
		if memberIndex == len(members.Entries) {
			return false
		}
		memberIndex++
	}
	return true
}

type HeartbeatResponse struct {
	Directive     Directive        `json:"directive"`
	Fence         MachineFence     `json:"fence"`
	NextAt        temporal.Instant `json:"next_at"`
	SchemaVersion uint16           `json:"schema_version"`
}

func (r HeartbeatResponse) Validate() error {
	if r.SchemaVersion != SchemaVersion {
		return core.ErrPrimitiveContract
	}
	return errors.Join(r.Fence.Validate(), r.Directive.Validate(), r.NextAt.Validate())
}

type heartbeatRequestWire HeartbeatRequest
type heartbeatResponseWire HeartbeatResponse

func (r HeartbeatRequest) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONDocument(heartbeatRequestWire(r))
}

func (r HeartbeatResponse) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONDocument(heartbeatResponseWire(r))
}

func (r *HeartbeatRequest) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[heartbeatRequestWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := HeartbeatRequest(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*r = candidate
	return nil
}

func (r *HeartbeatResponse) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[heartbeatResponseWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := HeartbeatResponse(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*r = candidate
	return nil
}

type HeartbeatRepository interface {
	Heartbeat(context.Context, HeartbeatRequest) (HeartbeatResponse, error)
}

type HeartbeatClient struct{ socket exchange.ClientSocket }

func NewHeartbeatClient(configuration exchange.ClientSocketConfiguration) (HeartbeatClient, error) {
	socket, err := exchange.NewClientSocket(configuration)
	if err != nil {
		return HeartbeatClient{}, err
	}
	return HeartbeatClient{socket: socket}, nil
}

func (c HeartbeatClient) Heartbeat(ctx context.Context, request HeartbeatRequest) (exchange.JSONResponse[HeartbeatResponse], error) {
	return exchange.SendSocketJSON[HeartbeatRequest, HeartbeatResponse](ctx, c.socket, request)
}

type HeartbeatServer struct {
	repository HeartbeatRepository
	socket     exchange.ServerSocket
}

func NewHeartbeatServer(contract exchange.JSONSocketContract, repository HeartbeatRepository) (HeartbeatServer, error) {
	if repository == nil {
		return HeartbeatServer{}, core.ErrPrimitiveContract
	}
	socket, err := exchange.NewServerSocket(contract)
	if err != nil {
		return HeartbeatServer{}, err
	}
	return HeartbeatServer{socket: socket, repository: repository}, nil
}

func (s HeartbeatServer) Serve(call exchange.SocketServerCall) error {
	if s.repository == nil {
		return core.ErrPrimitiveContract
	}
	ctx, err := call.Context()
	if err != nil {
		return err
	}
	received, err := exchange.ReceiveSocketJSON[HeartbeatRequest, *HeartbeatRequest](s.socket, call)
	if err != nil {
		return err
	}
	if err := RequireRunnerPeer(ctx, received.Body.Fence.Machine, received.Body.Fence.Generation); err != nil {
		return err
	}
	response, err := s.repository.Heartbeat(ctx, *received.Body)
	if err != nil {
		return err
	}
	if err := response.Validate(); err != nil {
		return err
	}
	if response.Fence != received.Body.Fence {
		return core.ErrPrimitiveContract
	}
	return exchange.WriteSocketJSON(s.socket, call, response)
}

func HeartbeatSocketContract(path exchange.SocketRoutePath) (exchange.JSONSocketContract, error) {
	requestLimit, requestErr := core.NewByteCount(HeartbeatRequestMaximumBytes)
	responseLimit, responseErr := core.NewByteCount(HeartbeatResponseMaximumBytes)
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
	_ core.Validatable            = HeartbeatStateUnknown
	_ core.Validatable            = DirectiveUnknown
	_ core.Validatable            = Directive{}
	_ core.ValidatedJSONMarshaler = HeartbeatRequest{}
	_ json.Unmarshaler            = (*HeartbeatRequest)(nil)
	_ core.ValidatedJSONMarshaler = HeartbeatResponse{}
	_ json.Unmarshaler            = (*HeartbeatResponse)(nil)
)
