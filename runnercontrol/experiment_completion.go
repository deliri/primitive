package runnercontrol

import (
	"bytes"
	"context"
	"crypto"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/projectstandards"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	ExperimentCompletionPayloadMaximumBytes  = 512 * 1024
	ExperimentCompletionDocumentMaximumBytes = 768 * 1024
	ExperimentCompletionReceiptMaximumBytes  = 16 * 1024
)

type ExperimentCompletionPayload struct {
	SchemaVersion uint16                                 `json:"schema_version"`
	Run           projectstandards.RunID                 `json:"run_id"`
	Probe         projectstandards.ProbeIdentity         `json:"probe"`
	Fence         SchedulingFence                        `json:"fence"`
	Members       MemberSet                              `json:"member_set"`
	Observation   projectstandards.ExperimentObservation `json:"observation"`
	StartedAt     *temporal.Instant                      `json:"started_at,omitempty"`
	CompletedAt   temporal.Instant                       `json:"completed_at"`
	Process       *process.ResultObservation             `json:"process,omitempty"`
}

func (p ExperimentCompletionPayload) Validate() error {
	if p.SchemaVersion != SchemaVersion {
		return core.ErrPrimitiveContract
	}
	if err := errors.Join(p.Run.Validate(), p.Probe.Validate(), p.Fence.Validate(), p.Members.Validate(), p.Observation.Validate(), p.CompletedAt.Validate()); err != nil {
		return err
	}
	if err := p.validateMembership(); err != nil {
		return err
	}
	if err := p.validateProbeBinding(); err != nil {
		return err
	}
	return p.validateExecution()
}

func (p ExperimentCompletionPayload) validateMembership() error {
	memberDigest, err := p.Members.Digest()
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	if memberDigest != p.Fence.MemberSetDigest || !p.Members.Contains(p.Run) {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (p ExperimentCompletionPayload) validateProbeBinding() error {
	if p.Probe.Role != projectstandards.ProbeRoleExperiment || p.Probe.Environment.MachineGeneration != p.Fence.Machine.Generation {
		return core.ErrPrimitiveContract
	}
	if p.Observation.EnvironmentFingerprint != p.Probe.Environment.EnvironmentFingerprint || p.Observation.MachineSheetDigest != p.Probe.Environment.MachineSheetDigest {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (p ExperimentCompletionPayload) validateExecution() error {
	if p.Observation.Started {
		return p.validateStartedExecution()
	}
	if p.StartedAt != nil || p.Process != nil {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (p ExperimentCompletionPayload) validateStartedExecution() error {
	if p.StartedAt == nil || p.Process == nil {
		return core.ErrPrimitiveContract
	}
	if err := errors.Join(p.StartedAt.Validate(), p.Process.Validate()); err != nil {
		return err
	}
	comparison, err := p.StartedAt.Compare(p.CompletedAt)
	if err != nil || comparison == core.ComparisonGreater {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	return nil
}

func (ExperimentCompletionPayload) AttestationDomain() CompletionSigningDomain {
	return CompletionSigningDomainExperimentV1
}

func (p ExperimentCompletionPayload) WriteCanonical(destination io.Writer) error {
	if destination == nil {
		return core.ErrPrimitiveContract
	}
	encoded, err := p.MarshalJSON()
	if err != nil {
		return err
	}
	written, err := destination.Write(encoded)
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	if written != len(encoded) {
		return errors.Join(core.ErrPrimitiveContract, io.ErrShortWrite)
	}
	return nil
}

type experimentCompletionPayloadWire ExperimentCompletionPayload

func (p ExperimentCompletionPayload) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(experimentCompletionPayloadWire(p))
	if err != nil || len(encoded) > ExperimentCompletionPayloadMaximumBytes {
		return nil, errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract, err)
	}
	return encoded, nil
}

func (p *ExperimentCompletionPayload) UnmarshalJSON(data []byte) error {
	if p == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[experimentCompletionPayloadWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := ExperimentCompletionPayload(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*p = candidate
	return nil
}

type ExperimentCompletionDocument struct {
	Payload     ExperimentCompletionPayload              `json:"payload"`
	Attestation attest.Envelope[CompletionSigningDomain] `json:"attestation"`
}

func (d ExperimentCompletionDocument) Validate() error {
	if err := errors.Join(d.Payload.Validate(), d.Attestation.Validate()); err != nil {
		return err
	}
	if d.Attestation.Domain != d.Payload.AttestationDomain() {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (d ExperimentCompletionDocument) IdempotencyKey() (exchange.IdempotencyKey, error) {
	encoded, err := d.MarshalJSON()
	if err != nil {
		return exchange.IdempotencyKey{}, err
	}
	digest := core.SHA256Of(encoded)
	hex, err := digest.Hex()
	if err != nil {
		return exchange.IdempotencyKey{}, err
	}
	return exchange.ParseIdempotencyKey("experiment-completion:" + hex)
}

type experimentCompletionDocumentWire ExperimentCompletionDocument

func (d ExperimentCompletionDocument) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(experimentCompletionDocumentWire(d))
	if err != nil || len(encoded) > ExperimentCompletionDocumentMaximumBytes {
		return nil, errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract, err)
	}
	return encoded, nil
}

func (d *ExperimentCompletionDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[experimentCompletionDocumentWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := ExperimentCompletionDocument(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*d = candidate
	return nil
}

func IssueExperimentCompletion(payload ExperimentCompletionPayload, signer crypto.Signer) (ExperimentCompletionDocument, error) {
	envelope, err := attest.Sign(attest.SignRequest[CompletionSigningDomain]{Body: payload, Signer: signer})
	if err != nil {
		return ExperimentCompletionDocument{}, err
	}
	document := ExperimentCompletionDocument{Payload: payload, Attestation: envelope}
	return document, document.Validate()
}

func VerifyExperimentCompletion(document ExperimentCompletionDocument, trusted attest.TrustedKeys) error {
	if err := document.Validate(); err != nil {
		return err
	}
	proof, err := attest.Verify(attest.VerifyRequest[CompletionSigningDomain]{Body: document.Payload, Envelope: document.Attestation, TrustedKeys: trusted})
	if err != nil {
		return err
	}
	return proof.Validate()
}

type ExperimentCompletionRecord struct {
	Document  ExperimentCompletionDocument
	Canonical []byte
	Digest    core.SHA256Digest
	Bytes     core.ByteLength
}

func NewExperimentCompletionRecord(document ExperimentCompletionDocument) (ExperimentCompletionRecord, error) {
	canonical, err := document.MarshalJSON()
	if err != nil {
		return ExperimentCompletionRecord{}, err
	}
	extent, err := core.NewByteLength(uint64(len(canonical)))
	if err != nil {
		return ExperimentCompletionRecord{}, err
	}
	record := ExperimentCompletionRecord{Document: document, Canonical: canonical, Digest: core.SHA256Of(canonical), Bytes: extent}
	return record, record.Validate()
}

func (r ExperimentCompletionRecord) Validate() error {
	if err := errors.Join(r.Document.Validate(), r.Digest.Validate(), r.Bytes.Validate()); err != nil {
		return err
	}
	encoded, err := r.Document.MarshalJSON()
	if err != nil || !bytes.Equal(encoded, r.Canonical) || r.Bytes.Uint64() != uint64(len(r.Canonical)) || r.Digest != core.SHA256Of(r.Canonical) {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	return nil
}

type ExperimentCompletionReceipt struct {
	SchemaVersion uint16                        `json:"schema_version"`
	Run           projectstandards.RunID        `json:"run_id"`
	Experiment    projectstandards.ExperimentID `json:"experiment_id"`
	Digest        core.SHA256Digest             `json:"document_digest"`
	Bytes         core.ByteLength               `json:"document_bytes"`
}

func (r ExperimentCompletionReceipt) Validate() error {
	if r.SchemaVersion != SchemaVersion {
		return core.ErrPrimitiveContract
	}
	return errors.Join(r.Run.Validate(), r.Experiment.Validate(), r.Digest.Validate(), r.Bytes.Validate())
}

type experimentCompletionReceiptWire ExperimentCompletionReceipt

func (r ExperimentCompletionReceipt) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONDocument(experimentCompletionReceiptWire(r))
}

func (r *ExperimentCompletionReceipt) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[experimentCompletionReceiptWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := ExperimentCompletionReceipt(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*r = candidate
	return nil
}

type ExperimentCompletionRepository interface {
	StoreExperimentCompletion(context.Context, ExperimentCompletionRecord) error
}

type ExperimentCompletionClient struct{ socket exchange.ClientSocket }

func NewExperimentCompletionClient(configuration exchange.ClientSocketConfiguration) (ExperimentCompletionClient, error) {
	socket, err := exchange.NewClientSocket(configuration)
	if err != nil {
		return ExperimentCompletionClient{}, err
	}
	return ExperimentCompletionClient{socket: socket}, nil
}

func (c ExperimentCompletionClient) Submit(ctx context.Context, document ExperimentCompletionDocument) (exchange.JSONResponse[ExperimentCompletionReceipt], error) {
	return exchange.SendReplayBoundSocketJSON[ExperimentCompletionDocument, ExperimentCompletionReceipt](ctx, c.socket, document)
}

type ExperimentCompletionServer struct {
	socket     exchange.ServerSocket
	repository ExperimentCompletionRepository
	trusted    attest.TrustedKeys
}

func NewExperimentCompletionServer(contract exchange.JSONSocketContract, repository ExperimentCompletionRepository, trusted attest.TrustedKeys) (ExperimentCompletionServer, error) {
	if repository == nil || trusted.Validate() != nil {
		return ExperimentCompletionServer{}, core.ErrPrimitiveContract
	}
	socket, err := exchange.NewServerSocket(contract)
	if err != nil {
		return ExperimentCompletionServer{}, err
	}
	return ExperimentCompletionServer{socket: socket, repository: repository, trusted: trusted}, nil
}

func (s ExperimentCompletionServer) Serve(writer http.ResponseWriter, request *http.Request) error {
	if s.repository == nil {
		return core.ErrPrimitiveContract
	}
	received, err := exchange.ReceiveReplayBoundSocketJSON[ExperimentCompletionDocument, *ExperimentCompletionDocument](s.socket, request)
	if err != nil {
		return err
	}
	if err := RequireRunnerPeer(request.Context(), received.Body.Payload.Fence.Machine.Machine, received.Body.Payload.Fence.Machine.Generation); err != nil {
		return err
	}
	if err := VerifyExperimentCompletion(*received.Body, s.trusted); err != nil {
		return fmt.Errorf("verify experiment completion for run %v experiment %v: %w", received.Body.Payload.Run, received.Body.Payload.Observation.Experiment, err)
	}
	record, err := NewExperimentCompletionRecord(*received.Body)
	if err != nil {
		return err
	}
	if err := s.repository.StoreExperimentCompletion(request.Context(), record); err != nil {
		return err
	}
	receipt := ExperimentCompletionReceipt{
		SchemaVersion: SchemaVersion, Run: record.Document.Payload.Run,
		Experiment: record.Document.Payload.Observation.Experiment,
		Digest:     record.Digest, Bytes: record.Bytes,
	}
	return exchange.WriteSocketJSON(s.socket, writer, receipt)
}

func ExperimentCompletionSocketContract(path exchange.SocketRoutePath) (exchange.JSONSocketContract, error) {
	requestLimit, requestErr := core.NewByteCount(ExperimentCompletionDocumentMaximumBytes)
	responseLimit, responseErr := core.NewByteCount(ExperimentCompletionReceiptMaximumBytes)
	if err := errors.Join(path.Validate(), requestErr, responseErr); err != nil {
		return exchange.JSONSocketContract{}, err
	}
	contract := exchange.JSONSocketContract{
		Path: path, Route: exchange.RouteSemantics{Method: exchange.MethodPost, Replay: exchange.ReplayIdempotencyKey},
		RequestBodyLimit: requestLimit, ResponseBodyLimit: responseLimit, SuccessStatus: core.HTTPStatusOK(),
	}
	return contract, contract.Validate()
}

var (
	_ core.ValidatedJSONMarshaler = ExperimentCompletionPayload{}
	_ json.Unmarshaler            = (*ExperimentCompletionPayload)(nil)
	_ core.ValidatedJSONMarshaler = ExperimentCompletionDocument{}
	_ exchange.IdempotencyBound   = ExperimentCompletionDocument{}
	_ json.Unmarshaler            = (*ExperimentCompletionDocument)(nil)
	_ core.Validatable            = ExperimentCompletionRecord{}
	_ core.ValidatedJSONMarshaler = ExperimentCompletionReceipt{}
	_ json.Unmarshaler            = (*ExperimentCompletionReceipt)(nil)
)
