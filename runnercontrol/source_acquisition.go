package runnercontrol

import (
	"context"
	json "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/standard"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	SourceAcquisitionRequestMaximumBytes  = 128 * 1024
	SourceAcquisitionResponseMaximumBytes = 512 * 1024
)

type SourceAcquisitionRequest struct {
	Members       MemberSet                 `json:"member_set"`
	Source        standard.SourceCoordinate `json:"source"`
	Fence         SchedulingFence           `json:"fence"`
	RequestedAt   temporal.Instant          `json:"requested_at"`
	SchemaVersion uint16                    `json:"schema_version"`
	Grant         SourceGrantIdentity       `json:"source_grant"`
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
	ContentType   core.HTTPMediaType             `json:"content_type"`
	Members       MemberSet                      `json:"member_set"`
	Grant         SourceGrant                    `json:"source_grant"`
	Capability    objectstore.DownloadCapability `json:"download_capability"`
	Document      SourceArchiveDocument          `json:"source_archive"`
	Fence         SchedulingFence                `json:"fence"`
	Integrity     objectstore.Integrity          `json:"integrity"`
	Policy        objectstore.Policy             `json:"policy"`
	SchemaVersion uint16                         `json:"schema_version"`
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
	ContentType   core.HTTPMediaType                       `json:"content_type"`
	Members       MemberSet                                `json:"member_set"`
	Grant         SourceGrant                              `json:"source_grant"`
	Capability    objectstore.DownloadCapabilityProjection `json:"download_capability"`
	Document      SourceArchiveDocument                    `json:"source_archive"`
	Fence         SchedulingFence                          `json:"fence"`
	Integrity     objectstore.Integrity                    `json:"integrity"`
	Policy        objectstore.Policy                       `json:"policy"`
	SchemaVersion uint16                                   `json:"schema_version"`
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
	want := standard.SourceCoordinate{Repository: manifest.Repository, Commit: manifest.Commit, Tree: manifest.Tree}
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

// ValidateJSONProjection proves that the exact issue-only bytes are admitted
// by the distinct receive-only SourceAcquisition contract. The embedded
// download capability intentionally cannot decode back into its issuer type.
func (p SourceAcquisitionProjection) ValidateJSONProjection(encoded []byte, limits core.StrictJSONLimits) error {
	return core.ValidateReceiveOnlyJSONProjection[
		SourceAcquisitionProjection,
		SourceAcquisition,
		*SourceAcquisition,
	](p, encoded, limits)
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
	repository SourceAcquisitionRepository
	socket     exchange.ServerSocket
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

func (s SourceAcquisitionServer) Serve(call exchange.SocketServerCall) error {
	ctx, err := call.Context()
	if err != nil {
		return err
	}
	received, err := exchange.ReceiveSocketJSON[SourceAcquisitionRequest, *SourceAcquisitionRequest](s.socket, call)
	if err != nil {
		return err
	}
	if err := RequireRunnerPeer(ctx, received.Body.Fence.Machine.Machine, received.Body.Fence.Machine.Generation); err != nil {
		return err
	}
	projection, err := s.repository.AcquireSource(ctx, *received.Body)
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
	_ core.ValidatedJSONMarshaler  = SourceAcquisitionRequest{}
	_ json.Unmarshaler             = (*SourceAcquisitionRequest)(nil)
	_ core.Validatable             = SourceAcquisition{}
	_ json.Unmarshaler             = (*SourceAcquisition)(nil)
	_ core.ValidatedJSONMarshaler  = SourceAcquisitionProjection{}
	_ core.ValidatedJSONProjection = SourceAcquisitionProjection{}
)
