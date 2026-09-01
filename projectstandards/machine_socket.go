package projectstandards

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"net/http"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type MachineQuery struct {
	SchemaVersion uint16    `json:"schema_version"`
	Machine       MachineID `json:"machine_id"`
}

func (q MachineQuery) Validate() error {
	if q.SchemaVersion != SchemaVersion {
		return contractError(errors.New("project standards machine query schema version is unsupported"))
	}
	return q.Machine.Validate()
}

type machineQueryWire MachineQuery

func (q MachineQuery) MarshalJSON() ([]byte, error) {
	if err := q.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(machineQueryWire(q))
	if err != nil {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (q *MachineQuery) UnmarshalJSON(data []byte) error {
	if q == nil {
		return jsonError(errors.New("nil project standards machine query receiver"))
	}
	wire, err := core.DecodeStrictJSONStructure[machineQueryWire](data, aboutJSONLimits())
	if err != nil {
		return jsonError(err)
	}
	candidate := MachineQuery(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*q = candidate
	return nil
}

type MachineResponse struct {
	SchemaVersion uint16         `json:"schema_version"`
	Query         MachineQuery   `json:"query"`
	Machine       CurrentMachine `json:"machine"`
}

func (r MachineResponse) Validate() error {
	if r.SchemaVersion != SchemaVersion {
		return contractError(errors.New("project standards machine response schema version is unsupported"))
	}
	if err := contractJoin(r.Query.Validate(), r.Machine.Validate()); err != nil {
		return err
	}
	configuration := r.Machine.Generation.Configuration
	if r.Query.Machine != configuration.Identity.ID {
		return conflictError(errors.New("project standards machine response differs from query identity"))
	}
	return nil
}

type machineResponseWire MachineResponse

func (r MachineResponse) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(machineResponseWire(r))
	if err != nil {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (r *MachineResponse) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError(errors.New("nil project standards machine response receiver"))
	}
	wire, err := core.DecodeStrictJSONStructure[machineResponseWire](data, aboutJSONLimits())
	if err != nil {
		return jsonError(err)
	}
	candidate := MachineResponse(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*r = candidate
	return nil
}

type MachineRepository interface {
	CurrentMachine(context.Context, MachineID) (CurrentMachine, error)
}

type MachineService struct{ repository MachineRepository }

func NewMachineService(repository MachineRepository) (MachineService, error) {
	if repository == nil {
		return MachineService{}, contractError(errors.New("project standards machine repository is nil"))
	}
	return MachineService{repository: repository}, nil
}

func (s MachineService) Resolve(ctx context.Context, query MachineQuery) (MachineResponse, error) {
	if s.repository == nil {
		return MachineResponse{}, contractError(errors.New("project standards machine service is unconstructed"))
	}
	if err := query.Validate(); err != nil {
		return MachineResponse{}, err
	}
	if err := contextTerminal(ctx); err != nil {
		return MachineResponse{}, err
	}
	machine, err := s.repository.CurrentMachine(ctx, query.Machine)
	if err != nil {
		return MachineResponse{}, err
	}
	response := MachineResponse{SchemaVersion: SchemaVersion, Query: query, Machine: machine}
	return response, response.Validate()
}

type MachineClient struct{ socket exchange.ClientSocket }

func NewMachineClient(configuration exchange.ClientSocketConfiguration) (MachineClient, error) {
	want, err := SocketContract(configuration.Contract.Path)
	if err != nil {
		return MachineClient{}, err
	}
	if !sameSocketContract(configuration.Contract, want) {
		return MachineClient{}, transportError(errors.New("project standards machine client socket contract differs from Project standards contract"))
	}
	socket, err := exchange.NewClientSocket(configuration)
	if err != nil {
		return MachineClient{}, transportError(err)
	}
	return MachineClient{socket: socket}, nil
}

func (c MachineClient) Fetch(ctx context.Context, query MachineQuery) (MachineFetchResult, error) {
	if err := c.socket.Validate(); err != nil {
		return MachineFetchResult{}, transportError(err)
	}
	response, err := exchange.SendSocketJSON[MachineQuery, MachineResponse](ctx, c.socket, query)
	if err != nil {
		return MachineFetchResult{}, transportError(err)
	}
	result := MachineFetchResult{Response: response.Body, Metadata: response.Metadata}
	return result, result.Validate()
}

type MachineFetchResult struct {
	Response MachineResponse           `json:"response"`
	Metadata exchange.ResponseMetadata `json:"metadata"`
}

func (r MachineFetchResult) Validate() error {
	return contractJoin(r.Response.Validate(), r.Metadata.Validate())
}

type MachineServer struct {
	socket  exchange.ServerSocket
	service MachineService
}

func NewMachineServer(path exchange.SocketRoutePath, repository MachineRepository) (MachineServer, error) {
	contract, err := SocketContract(path)
	if err != nil {
		return MachineServer{}, err
	}
	socket, err := exchange.NewServerSocket(contract)
	if err != nil {
		return MachineServer{}, transportError(err)
	}
	service, err := NewMachineService(repository)
	if err != nil {
		return MachineServer{}, err
	}
	return MachineServer{socket: socket, service: service}, nil
}

func (s MachineServer) Serve(writer http.ResponseWriter, request *http.Request) error {
	if s.service.repository == nil {
		return contractError(errors.New("project standards machine server is unconstructed"))
	}
	received, err := exchange.ReceiveSocketJSON[MachineQuery, *MachineQuery](s.socket, request)
	if err != nil {
		return transportError(err)
	}
	response, err := s.service.Resolve(request.Context(), *received.Body)
	if err != nil {
		return err
	}
	if err := exchange.WriteSocketJSON(s.socket, writer, response); err != nil {
		return transportError(err)
	}
	return nil
}

var (
	_ json.Marshaler   = MachineQuery{}
	_ json.Unmarshaler = (*MachineQuery)(nil)
	_ json.Marshaler   = MachineResponse{}
	_ json.Unmarshaler = (*MachineResponse)(nil)
)
