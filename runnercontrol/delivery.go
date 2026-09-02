package runnercontrol

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/standard"
)

const ObservationDeliveryReceiptMaximumBytes = 32 * 1024

type ObservationDeliveryIdentity struct {
	Observation core.SHA256Digest `json:"observation_digest"`
	Manifest    core.SHA256Digest `json:"manifest_digest"`
}

func (i ObservationDeliveryIdentity) Validate() error {
	return errors.Join(i.Observation.Validate(), i.Manifest.Validate())
}

type ObservationDeliveryStage struct {
	Manifest      ExperimentDeliveryManifest `json:"manifest"`
	Envelope      ObservationEnvelope        `json:"envelope"`
	SchemaVersion uint16                     `json:"schema_version"`
}

func (s ObservationDeliveryStage) Validate() error {
	if s.SchemaVersion != SchemaVersion {
		return core.ErrPrimitiveContract
	}
	if err := errors.Join(s.Envelope.Validate(), s.Manifest.Validate()); err != nil {
		return err
	}
	if s.Manifest.Run != s.Envelope.Payload.Run {
		return core.ErrPrimitiveContract
	}
	manifest, err := s.Manifest.Digest()
	if err != nil || manifest != s.Envelope.Payload.ExperimentDeliveryManifest {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	return nil
}

func (s ObservationDeliveryStage) Identity() (ObservationDeliveryIdentity, error) {
	if err := s.Validate(); err != nil {
		return ObservationDeliveryIdentity{}, err
	}
	envelope, err := s.Envelope.MarshalJSON()
	if err != nil {
		return ObservationDeliveryIdentity{}, err
	}
	manifest, err := s.Manifest.Digest()
	if err != nil {
		return ObservationDeliveryIdentity{}, err
	}
	identity := ObservationDeliveryIdentity{Observation: core.SHA256Of(envelope), Manifest: manifest}
	return identity, identity.Validate()
}

func (s ObservationDeliveryStage) IdempotencyKey() (exchange.IdempotencyKey, error) {
	identity, err := s.Identity()
	if err != nil {
		return exchange.IdempotencyKey{}, err
	}
	digest, err := identity.Observation.Hex()
	if err != nil {
		return exchange.IdempotencyKey{}, err
	}
	return exchange.ParseIdempotencyKey("observation-stage:" + digest)
}

type observationDeliveryStageWire ObservationDeliveryStage

func (s ObservationDeliveryStage) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONDocument(observationDeliveryStageWire(s))
}

func (s *ObservationDeliveryStage) UnmarshalJSON(data []byte) error {
	if s == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[observationDeliveryStageWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := ObservationDeliveryStage(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*s = candidate
	return nil
}

type ObservationDeliveryPageUpload struct {
	Page          ExperimentDeliveryPage      `json:"page"`
	SchemaVersion uint16                      `json:"schema_version"`
	Identity      ObservationDeliveryIdentity `json:"identity"`
}

func (u ObservationDeliveryPageUpload) Validate() error {
	if u.SchemaVersion != SchemaVersion {
		return core.ErrPrimitiveContract
	}
	return errors.Join(u.Identity.Validate(), u.Page.Validate())
}

func (u ObservationDeliveryPageUpload) IdempotencyKey() (exchange.IdempotencyKey, error) {
	if err := u.Validate(); err != nil {
		return exchange.IdempotencyKey{}, err
	}
	digest, err := u.Identity.Observation.Hex()
	if err != nil {
		return exchange.IdempotencyKey{}, err
	}
	return exchange.ParseIdempotencyKey(fmt.Sprintf("observation-page:%s:%d", digest, u.Page.Page))
}

type observationDeliveryPageUploadWire ObservationDeliveryPageUpload

func (u ObservationDeliveryPageUpload) MarshalJSON() ([]byte, error) {
	if err := u.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONDocument(observationDeliveryPageUploadWire(u))
}

func (u *ObservationDeliveryPageUpload) UnmarshalJSON(data []byte) error {
	if u == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[observationDeliveryPageUploadWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := ObservationDeliveryPageUpload(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*u = candidate
	return nil
}

type ObservationDeliveryCommit struct {
	SchemaVersion uint16                      `json:"schema_version"`
	Identity      ObservationDeliveryIdentity `json:"identity"`
	Run           standard.RunID              `json:"run_id"`
	PageCount     uint16                      `json:"page_count"`
}

func (c ObservationDeliveryCommit) Validate() error {
	if c.SchemaVersion != SchemaVersion || c.PageCount > ExperimentDeliveryPageMaximum {
		return core.ErrPrimitiveContract
	}
	return errors.Join(c.Identity.Validate(), c.Run.Validate())
}

func (c ObservationDeliveryCommit) IdempotencyKey() (exchange.IdempotencyKey, error) {
	if err := c.Validate(); err != nil {
		return exchange.IdempotencyKey{}, err
	}
	digest, err := c.Identity.Observation.Hex()
	if err != nil {
		return exchange.IdempotencyKey{}, err
	}
	return exchange.ParseIdempotencyKey("observation-commit:" + digest)
}

type observationDeliveryCommitWire ObservationDeliveryCommit

func (c ObservationDeliveryCommit) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONDocument(observationDeliveryCommitWire(c))
}

func (c *ObservationDeliveryCommit) UnmarshalJSON(data []byte) error {
	if c == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[observationDeliveryCommitWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := ObservationDeliveryCommit(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*c = candidate
	return nil
}

type ObservationDeliveryReceipt struct {
	SchemaVersion uint16                      `json:"schema_version"`
	Identity      ObservationDeliveryIdentity `json:"identity"`
	Run           standard.RunID              `json:"run_id"`
	PagesStored   uint16                      `json:"pages_stored"`
	Published     bool                        `json:"published"`
}

func (r ObservationDeliveryReceipt) Validate() error {
	if r.SchemaVersion != SchemaVersion || r.PagesStored > ExperimentDeliveryPageMaximum {
		return core.ErrPrimitiveContract
	}
	return errors.Join(r.Identity.Validate(), r.Run.Validate())
}

type observationDeliveryReceiptWire ObservationDeliveryReceipt

func (r ObservationDeliveryReceipt) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONDocument(observationDeliveryReceiptWire(r))
}

func (r *ObservationDeliveryReceipt) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[observationDeliveryReceiptWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := ObservationDeliveryReceipt(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*r = candidate
	return nil
}

type ObservationDeliveryStore interface {
	StageObservation(context.Context, ObservationDeliveryStage) (ObservationDeliveryReceipt, error)
	StageExperimentPage(context.Context, ObservationDeliveryPageUpload) (ObservationDeliveryReceipt, error)
	LoadStagedObservation(context.Context, ObservationDeliveryCommit) (ObservationDeliveryStage, []ExperimentDeliveryPage, error)
	PublishObservation(context.Context, ObservationDeliveryCommit) (ObservationDeliveryReceipt, error)
}

type ObservationDeliveryVerifier struct {
	Origin      standard.OriginIdentity
	Destination standard.Identifier
	Audience    standard.Identifier
	Grant       core.SHA256Digest
	ControlKeys attest.TrustedKeys
	RunnerKeys  attest.TrustedKeys
}

func (v ObservationDeliveryVerifier) Validate() error {
	return errors.Join(v.Origin.Validate(), v.Destination.Validate(), v.Audience.Validate(), v.Grant.Validate(), v.ControlKeys.Validate(), v.RunnerKeys.Validate())
}

func (v ObservationDeliveryVerifier) Verify(stage ObservationDeliveryStage, pages []ExperimentDeliveryPage) error {
	if err := v.Validate(); err != nil {
		return err
	}
	payload := stage.Envelope.Payload
	if payload.Origin != v.Origin || payload.Destination != v.Destination || payload.Audience != v.Audience || payload.DeliveryGrant != v.Grant {
		return core.ErrPrimitiveContract
	}
	return VerifyObservationDelivery(ObservationDeliveryVerification{Stage: stage, Pages: pages, ControlKeys: v.ControlKeys, RunnerKeys: v.RunnerKeys})
}

type ObservationDeliveryClient struct {
	stage  exchange.ClientSocket
	page   exchange.ClientSocket
	commit exchange.ClientSocket
}

type ObservationDeliveryClientConfiguration struct {
	Stage  exchange.ClientSocketConfiguration
	Page   exchange.ClientSocketConfiguration
	Commit exchange.ClientSocketConfiguration
}

func NewObservationDeliveryClient(configuration ObservationDeliveryClientConfiguration) (ObservationDeliveryClient, error) {
	stage, stageErr := exchange.NewClientSocket(configuration.Stage)
	page, pageErr := exchange.NewClientSocket(configuration.Page)
	commit, commitErr := exchange.NewClientSocket(configuration.Commit)
	if err := errors.Join(stageErr, pageErr, commitErr); err != nil {
		return ObservationDeliveryClient{}, err
	}
	return ObservationDeliveryClient{stage: stage, page: page, commit: commit}, nil
}

func (c ObservationDeliveryClient) Stage(ctx context.Context, request ObservationDeliveryStage) (exchange.JSONResponse[ObservationDeliveryReceipt], error) {
	return exchange.SendReplayBoundSocketJSON[ObservationDeliveryStage, ObservationDeliveryReceipt](ctx, c.stage, request)
}

func (c ObservationDeliveryClient) StorePage(ctx context.Context, request ObservationDeliveryPageUpload) (exchange.JSONResponse[ObservationDeliveryReceipt], error) {
	return exchange.SendReplayBoundSocketJSON[ObservationDeliveryPageUpload, ObservationDeliveryReceipt](ctx, c.page, request)
}

func (c ObservationDeliveryClient) Commit(ctx context.Context, request ObservationDeliveryCommit) (exchange.JSONResponse[ObservationDeliveryReceipt], error) {
	return exchange.SendReplayBoundSocketJSON[ObservationDeliveryCommit, ObservationDeliveryReceipt](ctx, c.commit, request)
}

type ObservationDeliveryServer struct {
	stage    exchange.ServerSocket
	page     exchange.ServerSocket
	commit   exchange.ServerSocket
	store    ObservationDeliveryStore
	verifier ObservationDeliveryVerifier
}

type ObservationDeliveryServerConfiguration struct {
	Stage    exchange.JSONSocketContract
	Page     exchange.JSONSocketContract
	Commit   exchange.JSONSocketContract
	Store    ObservationDeliveryStore
	Verifier ObservationDeliveryVerifier
}

func NewObservationDeliveryServer(configuration ObservationDeliveryServerConfiguration) (ObservationDeliveryServer, error) {
	if configuration.Store == nil {
		return ObservationDeliveryServer{}, core.ErrPrimitiveContract
	}
	stageSocket, stageErr := exchange.NewServerSocket(configuration.Stage)
	pageSocket, pageErr := exchange.NewServerSocket(configuration.Page)
	commitSocket, commitErr := exchange.NewServerSocket(configuration.Commit)
	if err := errors.Join(stageErr, pageErr, commitErr, configuration.Verifier.Validate()); err != nil {
		return ObservationDeliveryServer{}, err
	}
	return ObservationDeliveryServer{stage: stageSocket, page: pageSocket, commit: commitSocket, store: configuration.Store, verifier: configuration.Verifier}, nil
}

type deliveryReceiptWrite struct {
	call      exchange.SocketServerCall
	socket    exchange.ServerSocket
	receipt   ObservationDeliveryReceipt
	run       standard.RunID
	published bool
}

func (s ObservationDeliveryServer) ServeStage(call exchange.SocketServerCall) error {
	ctx, err := call.Context()
	if err != nil {
		return err
	}
	if err := RequireControlPeer(ctx); err != nil {
		return err
	}
	received, err := exchange.ReceiveReplayBoundSocketJSON[ObservationDeliveryStage, *ObservationDeliveryStage](s.stage, call)
	if err != nil {
		return err
	}
	receipt, err := s.store.StageObservation(ctx, *received.Body)
	if err != nil {
		return err
	}
	return writeDeliveryReceipt(deliveryReceiptWrite{socket: s.stage, call: call, receipt: receipt, run: received.Body.Envelope.Payload.Run})
}

func (s ObservationDeliveryServer) ServePage(call exchange.SocketServerCall) error {
	ctx, err := call.Context()
	if err != nil {
		return err
	}
	if err := RequireControlPeer(ctx); err != nil {
		return err
	}
	received, err := exchange.ReceiveReplayBoundSocketJSON[ObservationDeliveryPageUpload, *ObservationDeliveryPageUpload](s.page, call)
	if err != nil {
		return err
	}
	receipt, err := s.store.StageExperimentPage(ctx, *received.Body)
	if err != nil {
		return err
	}
	return writeDeliveryReceipt(deliveryReceiptWrite{socket: s.page, call: call, receipt: receipt, run: received.Body.Page.Run})
}

func (s ObservationDeliveryServer) ServeCommit(call exchange.SocketServerCall) error {
	ctx, err := call.Context()
	if err != nil {
		return err
	}
	if err := RequireControlPeer(ctx); err != nil {
		return err
	}
	received, err := exchange.ReceiveReplayBoundSocketJSON[ObservationDeliveryCommit, *ObservationDeliveryCommit](s.commit, call)
	if err != nil {
		return err
	}
	stage, pages, err := s.store.LoadStagedObservation(ctx, *received.Body)
	if err != nil {
		return err
	}
	if err := validateStagedObservation(*received.Body, stage, pages); err != nil {
		return err
	}
	if err := s.verifier.Verify(stage, pages); err != nil {
		return err
	}
	receipt, err := s.store.PublishObservation(ctx, *received.Body)
	if err != nil {
		return err
	}
	return writeDeliveryReceipt(deliveryReceiptWrite{socket: s.commit, call: call, receipt: receipt, run: received.Body.Run, published: true})
}

func validateStagedObservation(commit ObservationDeliveryCommit, stage ObservationDeliveryStage, pages []ExperimentDeliveryPage) error {
	if len(pages) != int(commit.PageCount) || stage.Envelope.Payload.Run != commit.Run {
		return core.ErrPrimitiveContract
	}
	identity, err := stage.Identity()
	if err != nil || identity != commit.Identity {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	return nil
}

func writeDeliveryReceipt(request deliveryReceiptWrite) error {
	if err := request.receipt.Validate(); err != nil || request.receipt.Run != request.run || request.receipt.Published != request.published {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	return exchange.WriteSocketJSON(request.socket, request.call, request.receipt)
}

func ObservationDeliverySocketContract(path exchange.SocketRoutePath, requestMaximum uint64) (exchange.JSONSocketContract, error) {
	requestLimit, requestErr := core.NewByteCount(requestMaximum)
	responseLimit, responseErr := core.NewByteCount(ObservationDeliveryReceiptMaximumBytes)
	if err := errors.Join(path.Validate(), requestErr, responseErr); err != nil {
		return exchange.JSONSocketContract{}, err
	}
	contract := exchange.JSONSocketContract{Path: path, Route: exchange.RouteSemantics{Method: exchange.MethodPost, Replay: exchange.ReplayIdempotencyKey}, RequestBodyLimit: requestLimit, ResponseBodyLimit: responseLimit, SuccessStatus: core.HTTPStatusOK()}
	return contract, contract.Validate()
}

var (
	_ core.Validatable            = ObservationDeliveryIdentity{}
	_ core.ValidatedJSONMarshaler = ObservationDeliveryStage{}
	_ json.Unmarshaler            = (*ObservationDeliveryStage)(nil)
	_ exchange.IdempotencyBound   = ObservationDeliveryStage{}
	_ core.ValidatedJSONMarshaler = ObservationDeliveryPageUpload{}
	_ json.Unmarshaler            = (*ObservationDeliveryPageUpload)(nil)
	_ exchange.IdempotencyBound   = ObservationDeliveryPageUpload{}
	_ core.ValidatedJSONMarshaler = ObservationDeliveryCommit{}
	_ json.Unmarshaler            = (*ObservationDeliveryCommit)(nil)
	_ exchange.IdempotencyBound   = ObservationDeliveryCommit{}
	_ core.ValidatedJSONMarshaler = ObservationDeliveryReceipt{}
	_ json.Unmarshaler            = (*ObservationDeliveryReceipt)(nil)
	_ core.Validatable            = ObservationDeliveryVerifier{}
)
