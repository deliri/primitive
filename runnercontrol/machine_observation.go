package runnercontrol

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"net/http"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/projectstandards"
)

const (
	MachineObservationRequestMaximumBytes  = 512 * 1024
	MachineObservationResponseMaximumBytes = 16 * 1024
)

// MachineObservationSubmission binds the Primitive-produced machine sheet to
// the clean fixed-workspace proof that made this generation eligible to claim.
type MachineObservationSubmission struct {
	SchemaVersion uint16                              `json:"schema_version"`
	Observation   projectstandards.MachineObservation `json:"observation"`
	Clean         CleanMachineState                   `json:"clean_machine_state"`
}

func (s MachineObservationSubmission) Validate() error {
	if s.SchemaVersion != SchemaVersion {
		return core.ErrPrimitiveContract
	}
	if err := errors.Join(s.Observation.Validate(), s.Clean.Validate()); err != nil {
		return err
	}
	cleanBeforeObservation, err := s.Clean.Observation.ObservedAt.Compare(s.Observation.ObservedAt)
	if err != nil || cleanBeforeObservation == core.ComparisonGreater {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	return nil
}

type MachineObservationReceipt struct {
	SchemaVersion uint16                                `json:"schema_version"`
	ObservationID projectstandards.MachineObservationID `json:"observation_id"`
	CleanDigest   core.SHA256Digest                     `json:"clean_machine_state_digest"`
}

func (r MachineObservationReceipt) Validate() error {
	if r.SchemaVersion != SchemaVersion {
		return core.ErrPrimitiveContract
	}
	return errors.Join(r.ObservationID.Validate(), r.CleanDigest.Validate())
}

type machineObservationSubmissionWire MachineObservationSubmission
type machineObservationReceiptWire MachineObservationReceipt

func (s MachineObservationSubmission) MarshalJSON() ([]byte, error) {
	return marshalMachineObservation(s, machineObservationSubmissionWire(s))
}

func (r MachineObservationReceipt) MarshalJSON() ([]byte, error) {
	return marshalMachineObservation(r, machineObservationReceiptWire(r))
}

func marshalMachineObservation[T core.Validatable, W any](value T, wire W) ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(wire)
	if err != nil {
		return nil, errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract, err)
	}
	return encoded, nil
}

func (s *MachineObservationSubmission) UnmarshalJSON(data []byte) error {
	return unmarshalMachineObservation(data, s, func(wire machineObservationSubmissionWire) MachineObservationSubmission {
		return MachineObservationSubmission(wire)
	})
}

func (r *MachineObservationReceipt) UnmarshalJSON(data []byte) error {
	return unmarshalMachineObservation(data, r, func(wire machineObservationReceiptWire) MachineObservationReceipt {
		return MachineObservationReceipt(wire)
	})
}

func unmarshalMachineObservation[W any, T core.Validatable](data []byte, destination *T, project func(W) T) error {
	if destination == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[W](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract, err)
	}
	candidate := project(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*destination = candidate
	return nil
}

type MachineObservationRepository interface {
	RecordMachineObservation(context.Context, MachineObservationSubmission) error
}

type MachineObservationClient struct{ socket exchange.ClientSocket }

func NewMachineObservationClient(configuration exchange.ClientSocketConfiguration) (MachineObservationClient, error) {
	socket, err := exchange.NewClientSocket(configuration)
	if err != nil {
		return MachineObservationClient{}, err
	}
	return MachineObservationClient{socket: socket}, nil
}

func (c MachineObservationClient) Submit(ctx context.Context, submission MachineObservationSubmission) (exchange.JSONResponse[MachineObservationReceipt], error) {
	return exchange.SendSocketJSON[MachineObservationSubmission, MachineObservationReceipt](ctx, c.socket, submission)
}

type MachineObservationServer struct {
	socket     exchange.ServerSocket
	repository MachineObservationRepository
}

func NewMachineObservationServer(contract exchange.JSONSocketContract, repository MachineObservationRepository) (MachineObservationServer, error) {
	if repository == nil {
		return MachineObservationServer{}, core.ErrPrimitiveContract
	}
	socket, err := exchange.NewServerSocket(contract)
	if err != nil {
		return MachineObservationServer{}, err
	}
	return MachineObservationServer{socket: socket, repository: repository}, nil
}

func (s MachineObservationServer) Serve(writer http.ResponseWriter, request *http.Request) error {
	if s.repository == nil {
		return core.ErrPrimitiveContract
	}
	received, err := exchange.ReceiveSocketJSON[MachineObservationSubmission, *MachineObservationSubmission](s.socket, request)
	if err != nil {
		return err
	}
	if err := RequireRunnerPeer(request.Context(), received.Body.Observation.Configuration.Identity.ID, received.Body.Observation.GenerationID); err != nil {
		return err
	}
	if err := s.repository.RecordMachineObservation(request.Context(), *received.Body); err != nil {
		return err
	}
	cleanBytes, err := core.MarshalCanonicalJSONDocument(received.Body.Clean)
	if err != nil {
		return err
	}
	receipt := MachineObservationReceipt{
		SchemaVersion: SchemaVersion,
		ObservationID: received.Body.Observation.ID,
		CleanDigest:   core.SHA256Of(cleanBytes),
	}
	if err := receipt.Validate(); err != nil {
		return err
	}
	return exchange.WriteSocketJSON(s.socket, writer, receipt)
}

func MachineObservationSocketContract(path exchange.SocketRoutePath) (exchange.JSONSocketContract, error) {
	requestLimit, requestErr := core.NewByteCount(MachineObservationRequestMaximumBytes)
	responseLimit, responseErr := core.NewByteCount(MachineObservationResponseMaximumBytes)
	var status core.HTTPStatusCode
	statusErr := status.AdmitInt(http.StatusAccepted)
	if err := errors.Join(path.Validate(), requestErr, responseErr, statusErr); err != nil {
		return exchange.JSONSocketContract{}, err
	}
	contract := exchange.JSONSocketContract{
		Path: path, Route: exchange.RouteSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt},
		RequestBodyLimit: requestLimit, ResponseBodyLimit: responseLimit, SuccessStatus: status,
	}
	return contract, contract.Validate()
}

var (
	_ core.ValidatedJSONMarshaler = MachineObservationSubmission{}
	_ json.Unmarshaler            = (*MachineObservationSubmission)(nil)
	_ core.ValidatedJSONMarshaler = MachineObservationReceipt{}
	_ json.Unmarshaler            = (*MachineObservationReceipt)(nil)
)
