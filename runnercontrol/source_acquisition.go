package runnercontrol

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"net/http"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/projectstandards"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	SourceAcquisitionRequestMaximumBytes  = 128 * 1024
	SourceAcquisitionResponseMaximumBytes = 512 * 1024
)

type SourceAcquisitionRequest struct {
	SchemaVersion uint16                            `json:"schema_version"`
	Fence         SchedulingFence                   `json:"fence"`
	Members       MemberSet                         `json:"member_set"`
	Source        projectstandards.SourceCoordinate `json:"source"`
	Grant         SourceGrantIdentity               `json:"source_grant"`
	RequestedAt   temporal.Instant                  `json:"requested_at"`
}

func (r SourceAcquisitionRequest) Validate() error {
	if r.SchemaVersion != SchemaVersion {
		return core.ErrPrimitiveContract
	}
	if err := errors.Join(r.Fence.Validate(), r.Members.Validate(), r.Source.Validate(), r.Grant.Validate(), r.RequestedAt.Validate()); err != nil {
		return err
	}
	members, err := r.Members.Digest()
	if err != nil || members != r.Fence.MemberSetDigest {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	comparison, err := r.RequestedAt.Compare(r.Fence.Machine.ExpiresAt)
	if err != nil || comparison != core.ComparisonLess {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	return nil
}

type sourceAcquisitionRequestWire SourceAcquisitionRequest

func (r SourceAcquisitionRequest) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONDocument(sourceAcquisitionRequestWire(r))
}

func (r *SourceAcquisitionRequest) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[sourceAcquisitionRequestWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := SourceAcquisitionRequest(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*r = candidate
	return nil
}

type SourceAcquisition struct {
	SchemaVersion uint16                         `json:"schema_version"`
	Fence         SchedulingFence                `json:"fence"`
	Members       MemberSet                      `json:"member_set"`
	Grant         SourceGrant                    `json:"source_grant"`
	Document      SourceArchiveDocument          `json:"source_archive"`
	Capability    objectstore.DownloadCapability `json:"download_capability"`
	Integrity     objectstore.Integrity          `json:"integrity"`
	ContentType   core.HTTPMediaType             `json:"content_type"`
	Policy        objectstore.Policy             `json:"policy"`
}

func (a SourceAcquisition) Validate() error {
	if a.SchemaVersion != SchemaVersion {
		return core.ErrPrimitiveContract
	}
	if err := errors.Join(a.Fence.Validate(), a.Members.Validate(), a.Grant.Validate(), a.Document.Validate(), a.Capability.Validate(), a.Integrity.Validate(), a.ContentType.Validate(), a.Policy.Validate()); err != nil {
		return err
	}
	return validateSourceAcquisitionClosure(a)
}

type sourceAcquisitionWire SourceAcquisition

func (a *SourceAcquisition) UnmarshalJSON(data []byte) error {
	if a == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[sourceAcquisitionWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := SourceAcquisition(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*a = candidate
	return nil
}

type SourceAcquisitionProjection struct {
	SchemaVersion uint16                                   `json:"schema_version"`
	Fence         SchedulingFence                          `json:"fence"`
	Members       MemberSet                                `json:"member_set"`
	Grant         SourceGrant                              `json:"source_grant"`
	Document      SourceArchiveDocument                    `json:"source_archive"`
	Capability    objectstore.DownloadCapabilityProjection `json:"download_capability"`
	Integrity     objectstore.Integrity                    `json:"integrity"`
	ContentType   core.HTTPMediaType                       `json:"content_type"`
	Policy        objectstore.Policy                       `json:"policy"`
}

func (p SourceAcquisitionProjection) Validate() error {
	if p.SchemaVersion != SchemaVersion {
		return core.ErrPrimitiveContract
	}
	if err := errors.Join(p.Fence.Validate(), p.Members.Validate(), p.Grant.Validate(), p.Document.Validate(), p.Capability.Validate(), p.Integrity.Validate(), p.ContentType.Validate(), p.Policy.Validate()); err != nil {
		return err
	}
	return validateSourceProjectionClosure(p)
}

func validateSourceProjectionClosure(p SourceAcquisitionProjection) error {
	encoded, err := p.Capability.MarshalJSON()
	if err != nil {
		return err
	}
	var capability objectstore.DownloadCapability
	if err := capability.UnmarshalJSON(encoded); err != nil {
		return err
	}
	return validateSourceAcquisitionClosure(SourceAcquisition{Fence: p.Fence, Members: p.Members, Grant: p.Grant, Document: p.Document, Integrity: p.Integrity, Capability: capability})
}

func validateSourceAcquisitionClosure(acquisition SourceAcquisition) error {
	memberDigest, memberErr := acquisition.Members.Digest()
	manifest := acquisition.Document.Manifest
	want := projectstandards.SourceCoordinate{Repository: manifest.Repository, Commit: manifest.Commit, Tree: manifest.Tree}
	target, targetErr := acquisition.Capability.Target()
	targetGrantComparison, comparisonErr := target.ExpiresAt.Compare(acquisition.Grant.ExpiresAt)
	if memberErr != nil || targetErr != nil || comparisonErr != nil || memberDigest != acquisition.Fence.MemberSetDigest || acquisition.Grant.Source != want || acquisition.Integrity.SHA256 != manifest.ArchiveDigest || acquisition.Integrity.Length != manifest.ArchiveBytes || targetGrantComparison == core.ComparisonGreater {
		return errors.Join(core.ErrPrimitiveContract, memberErr, targetErr, comparisonErr)
	}
	return nil
}

type sourceAcquisitionProjectionWire SourceAcquisitionProjection

func (p SourceAcquisitionProjection) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONDocument(sourceAcquisitionProjectionWire(p))
}

type SourceAcquisitionRepository interface {
	AcquireSource(context.Context, SourceAcquisitionRequest) (SourceAcquisitionProjection, error)
}

type SourceAcquisitionClient struct{ socket exchange.ClientSocket }

func NewSourceAcquisitionClient(configuration exchange.ClientSocketConfiguration) (SourceAcquisitionClient, error) {
	socket, err := exchange.NewClientSocket(configuration)
	if err != nil {
		return SourceAcquisitionClient{}, err
	}
	return SourceAcquisitionClient{socket: socket}, nil
}

func (c SourceAcquisitionClient) Acquire(ctx context.Context, request SourceAcquisitionRequest) (exchange.JSONResponse[SourceAcquisition], error) {
	response, err := exchange.SendSocketJSON[SourceAcquisitionRequest, SourceAcquisition](ctx, c.socket, request)
	if err != nil {
		return exchange.JSONResponse[SourceAcquisition]{}, err
	}
	if response.Body.Fence != request.Fence || response.Body.Grant.Identity != request.Grant || response.Body.Grant.Source != request.Source {
		return exchange.JSONResponse[SourceAcquisition]{}, core.ErrPrimitiveContract
	}
	return response, nil
}

type SourceAcquisitionServer struct {
	socket     exchange.ServerSocket
	repository SourceAcquisitionRepository
}

func NewSourceAcquisitionServer(contract exchange.JSONSocketContract, repository SourceAcquisitionRepository) (SourceAcquisitionServer, error) {
	if repository == nil {
		return SourceAcquisitionServer{}, core.ErrPrimitiveContract
	}
	socket, err := exchange.NewServerSocket(contract)
	if err != nil {
		return SourceAcquisitionServer{}, err
	}
	return SourceAcquisitionServer{socket: socket, repository: repository}, nil
}

func (s SourceAcquisitionServer) Serve(writer http.ResponseWriter, request *http.Request) error {
	call, err := exchange.NewSocketServerCall(writer, request)
	if err != nil {
		return err
	}
	received, err := exchange.ReceiveSocketJSON[SourceAcquisitionRequest, *SourceAcquisitionRequest](s.socket, call)
	if err != nil {
		return err
	}
	if err := RequireRunnerPeer(request.Context(), received.Body.Fence.Machine.Machine, received.Body.Fence.Machine.Generation); err != nil {
		return err
	}
	projection, err := s.repository.AcquireSource(request.Context(), *received.Body)
	if err != nil {
		return err
	}
	if err := projection.Validate(); err != nil || projection.Fence != received.Body.Fence || projection.Grant.Identity != received.Body.Grant || projection.Grant.Source != received.Body.Source {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	return exchange.WriteSocketJSON(s.socket, call, projection)
}

func SourceAcquisitionSocketContract(path exchange.SocketRoutePath) (exchange.JSONSocketContract, error) {
	requestLimit, requestErr := core.NewByteCount(SourceAcquisitionRequestMaximumBytes)
	responseLimit, responseErr := core.NewByteCount(SourceAcquisitionResponseMaximumBytes)
	if err := errors.Join(path.Validate(), requestErr, responseErr); err != nil {
		return exchange.JSONSocketContract{}, err
	}
	contract := exchange.JSONSocketContract{Path: path, Route: exchange.RouteSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt}, RequestBodyLimit: requestLimit, ResponseBodyLimit: responseLimit, SuccessStatus: core.HTTPStatusOK()}
	return contract, contract.Validate()
}

var (
	_ core.ValidatedJSONMarshaler = SourceAcquisitionRequest{}
	_ json.Unmarshaler            = (*SourceAcquisitionRequest)(nil)
	_ core.Validatable            = SourceAcquisition{}
	_ json.Unmarshaler            = (*SourceAcquisition)(nil)
	_ core.ValidatedJSONMarshaler = SourceAcquisitionProjection{}
)
