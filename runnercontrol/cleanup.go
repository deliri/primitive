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
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	CleanupPayloadMaximumBytes  = 128 * 1024
	CleanupDocumentMaximumBytes = 256 * 1024
	CleanupReceiptMaximumBytes  = 16 * 1024
)

type MachineStateObservation struct {
	RootIdentity      core.SHA256Digest `json:"root_identity"`
	Entries           uint32            `json:"entries"`
	Processes         uint32            `json:"processes"`
	ControlGroups     uint32            `json:"control_groups"`
	Namespaces        uint32            `json:"namespaces"`
	Mounts            uint32            `json:"mounts"`
	Descriptors       uint32            `json:"descriptors"`
	Sockets           uint32            `json:"sockets"`
	CredentialCustody uint32            `json:"credential_custody"`
	SecretCustody     uint32            `json:"secret_custody"`
	ObservedAt        temporal.Instant  `json:"observed_at"`
}

func (s MachineStateObservation) Validate() error {
	return errors.Join(s.RootIdentity.Validate(), s.ObservedAt.Validate())
}

func (s MachineStateObservation) IsClean() bool {
	return s.Validate() == nil && s.Entries == 0 && s.Processes == 0 && s.ControlGroups == 0 && s.Namespaces == 0 && s.Mounts == 0 && s.Descriptors == 0 && s.Sockets == 0 && s.CredentialCustody == 0 && s.SecretCustody == 0
}

type CleanMachineState struct {
	Observation MachineStateObservation `json:"observation"`
}

func (s CleanMachineState) Validate() error {
	if !s.Observation.IsClean() {
		return core.ErrPrimitiveContract
	}
	return nil
}

type CleanupPayload struct {
	SchemaVersion uint16                  `json:"schema_version"`
	Fence         SchedulingFence         `json:"fence"`
	Members       MemberSet               `json:"member_set"`
	WorkspaceRoot core.SHA256Digest       `json:"workspace_root"`
	Before        MachineStateObservation `json:"before"`
	After         CleanMachineState       `json:"after"`
	CompletedAt   temporal.Instant        `json:"completed_at"`
}

func (p CleanupPayload) Validate() error {
	if p.SchemaVersion != SchemaVersion {
		return core.ErrPrimitiveContract
	}
	if err := errors.Join(p.Fence.Validate(), p.Members.Validate(), p.WorkspaceRoot.Validate(), p.Before.Validate(), p.After.Validate(), p.CompletedAt.Validate()); err != nil {
		return err
	}
	digest, err := p.Members.Digest()
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	if digest != p.Fence.MemberSetDigest || p.Before.RootIdentity != p.WorkspaceRoot || p.After.Observation.RootIdentity != p.WorkspaceRoot {
		return core.ErrPrimitiveContract
	}
	return p.validateTimeline()
}

func (p CleanupPayload) validateTimeline() error {
	beforeAfter, err := p.Before.ObservedAt.Compare(p.After.Observation.ObservedAt)
	afterComplete, completeErr := p.After.Observation.ObservedAt.Compare(p.CompletedAt)
	if err != nil || completeErr != nil || beforeAfter == core.ComparisonGreater || afterComplete == core.ComparisonGreater {
		return errors.Join(core.ErrPrimitiveContract, err, completeErr)
	}
	return nil
}

func (CleanupPayload) AttestationDomain() CompletionSigningDomain {
	return CompletionSigningDomainCleanupV1
}
func (p CleanupPayload) WriteCanonical(destination io.Writer) error {
	if destination == nil {
		return core.ErrPrimitiveContract
	}
	encoded, err := p.MarshalJSON()
	if err != nil {
		return err
	}
	written, err := destination.Write(encoded)
	if err != nil || written != len(encoded) {
		return errors.Join(core.ErrPrimitiveContract, err, io.ErrShortWrite)
	}
	return nil
}

type cleanupPayloadWire CleanupPayload

func (p CleanupPayload) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(cleanupPayloadWire(p))
	if err != nil || len(encoded) > CleanupPayloadMaximumBytes {
		return nil, errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract, err)
	}
	return encoded, nil
}
func (p *CleanupPayload) UnmarshalJSON(data []byte) error {
	if p == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[cleanupPayloadWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := CleanupPayload(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*p = candidate
	return nil
}

type CleanupDocument struct {
	Payload     CleanupPayload                           `json:"payload"`
	Attestation attest.Envelope[CompletionSigningDomain] `json:"attestation"`
}

func (d CleanupDocument) Validate() error {
	if err := errors.Join(d.Payload.Validate(), d.Attestation.Validate()); err != nil {
		return err
	}
	if d.Attestation.Domain != d.Payload.AttestationDomain() {
		return core.ErrPrimitiveContract
	}
	return nil
}
func (d CleanupDocument) IdempotencyKey() (exchange.IdempotencyKey, error) {
	encoded, err := d.MarshalJSON()
	if err != nil {
		return exchange.IdempotencyKey{}, err
	}
	hex, err := core.SHA256Of(encoded).Hex()
	if err != nil {
		return exchange.IdempotencyKey{}, err
	}
	return exchange.ParseIdempotencyKey("cleanup:" + hex)
}

type cleanupDocumentWire CleanupDocument

func (d CleanupDocument) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(cleanupDocumentWire(d))
	if err != nil || len(encoded) > CleanupDocumentMaximumBytes {
		return nil, errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract, err)
	}
	return encoded, nil
}
func (d *CleanupDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[cleanupDocumentWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := CleanupDocument(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*d = candidate
	return nil
}
func IssueCleanup(payload CleanupPayload, signer crypto.Signer) (CleanupDocument, error) {
	envelope, err := attest.Sign(attest.SignRequest[CompletionSigningDomain]{Body: payload, Signer: signer})
	if err != nil {
		return CleanupDocument{}, err
	}
	document := CleanupDocument{Payload: payload, Attestation: envelope}
	return document, document.Validate()
}
func VerifyCleanup(document CleanupDocument, trusted attest.TrustedKeys) error {
	if err := document.Validate(); err != nil {
		return err
	}
	proof, err := attest.Verify(attest.VerifyRequest[CompletionSigningDomain]{Body: document.Payload, Envelope: document.Attestation, TrustedKeys: trusted})
	if err != nil {
		return err
	}
	return proof.Validate()
}

type CleanupRecord struct {
	Document  CleanupDocument
	Canonical []byte
	Digest    core.SHA256Digest
	Bytes     core.ByteLength
}

func NewCleanupRecord(document CleanupDocument) (CleanupRecord, error) {
	canonical, err := document.MarshalJSON()
	if err != nil {
		return CleanupRecord{}, err
	}
	extent, err := core.NewByteLength(uint64(len(canonical)))
	if err != nil {
		return CleanupRecord{}, err
	}
	record := CleanupRecord{Document: document, Canonical: canonical, Digest: core.SHA256Of(canonical), Bytes: extent}
	return record, record.Validate()
}
func (r CleanupRecord) Validate() error {
	if err := errors.Join(r.Document.Validate(), r.Digest.Validate(), r.Bytes.Validate()); err != nil {
		return err
	}
	canonical, err := r.Document.MarshalJSON()
	if err != nil || !bytes.Equal(canonical, r.Canonical) || r.Digest != core.SHA256Of(r.Canonical) || r.Bytes.Uint64() != uint64(len(r.Canonical)) {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	return nil
}

type CleanupReceipt struct {
	SchemaVersion uint16            `json:"schema_version"`
	Fence         SchedulingFence   `json:"fence"`
	Digest        core.SHA256Digest `json:"document_digest"`
	Bytes         core.ByteLength   `json:"document_bytes"`
}

func (r CleanupReceipt) Validate() error {
	if r.SchemaVersion != SchemaVersion {
		return core.ErrPrimitiveContract
	}
	return errors.Join(r.Fence.Validate(), r.Digest.Validate(), r.Bytes.Validate())
}

type cleanupReceiptWire CleanupReceipt

func (r CleanupReceipt) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONDocument(cleanupReceiptWire(r))
}
func (r *CleanupReceipt) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[cleanupReceiptWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := CleanupReceipt(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*r = candidate
	return nil
}

type CleanupRepository interface {
	StoreCleanup(context.Context, CleanupRecord) error
}
type CleanupClient struct{ socket exchange.ClientSocket }

func NewCleanupClient(configuration exchange.ClientSocketConfiguration) (CleanupClient, error) {
	socket, err := exchange.NewClientSocket(configuration)
	if err != nil {
		return CleanupClient{}, err
	}
	return CleanupClient{socket: socket}, nil
}
func (c CleanupClient) Submit(ctx context.Context, document CleanupDocument) (exchange.JSONResponse[CleanupReceipt], error) {
	return exchange.SendReplayBoundSocketJSON[CleanupDocument, CleanupReceipt](ctx, c.socket, document)
}

type CleanupServer struct {
	socket     exchange.ServerSocket
	repository CleanupRepository
	trusted    attest.TrustedKeys
}

func NewCleanupServer(contract exchange.JSONSocketContract, repository CleanupRepository, trusted attest.TrustedKeys) (CleanupServer, error) {
	if repository == nil || trusted.Validate() != nil {
		return CleanupServer{}, core.ErrPrimitiveContract
	}
	socket, err := exchange.NewServerSocket(contract)
	if err != nil {
		return CleanupServer{}, err
	}
	return CleanupServer{socket: socket, repository: repository, trusted: trusted}, nil
}
func (s CleanupServer) Serve(writer http.ResponseWriter, request *http.Request) error {
	if s.repository == nil {
		return core.ErrPrimitiveContract
	}
	received, err := exchange.ReceiveReplayBoundSocketJSON[CleanupDocument, *CleanupDocument](s.socket, request)
	if err != nil {
		return err
	}
	payload := received.Body.Payload
	machine := payload.Fence.Machine
	if err := RequireRunnerPeer(request.Context(), machine.Machine, machine.Generation); err != nil {
		return err
	}
	if err := VerifyCleanup(*received.Body, s.trusted); err != nil {
		return fmt.Errorf("verify cleanup for machine %v generation %v: %w", machine.Machine, machine.Generation, err)
	}
	record, err := NewCleanupRecord(*received.Body)
	if err != nil {
		return err
	}
	if err := s.repository.StoreCleanup(request.Context(), record); err != nil {
		return err
	}
	receipt := CleanupReceipt{SchemaVersion: SchemaVersion, Fence: record.Document.Payload.Fence, Digest: record.Digest, Bytes: record.Bytes}
	return exchange.WriteSocketJSON(s.socket, writer, receipt)
}
func CleanupSocketContract(path exchange.SocketRoutePath) (exchange.JSONSocketContract, error) {
	requestLimit, requestErr := core.NewByteCount(CleanupDocumentMaximumBytes)
	responseLimit, responseErr := core.NewByteCount(CleanupReceiptMaximumBytes)
	if err := errors.Join(path.Validate(), requestErr, responseErr); err != nil {
		return exchange.JSONSocketContract{}, err
	}
	contract := exchange.JSONSocketContract{Path: path, Route: exchange.RouteSemantics{Method: exchange.MethodPost, Replay: exchange.ReplayIdempotencyKey}, RequestBodyLimit: requestLimit, ResponseBodyLimit: responseLimit, SuccessStatus: core.HTTPStatusOK()}
	return contract, contract.Validate()
}

var (
	_ core.Validatable            = MachineStateObservation{}
	_ core.Validatable            = CleanMachineState{}
	_ core.ValidatedJSONMarshaler = CleanupPayload{}
	_ json.Unmarshaler            = (*CleanupPayload)(nil)
	_ core.ValidatedJSONMarshaler = CleanupDocument{}
	_ json.Unmarshaler            = (*CleanupDocument)(nil)
	_ exchange.IdempotencyBound   = CleanupDocument{}
	_ core.Validatable            = CleanupRecord{}
	_ core.ValidatedJSONMarshaler = CleanupReceipt{}
	_ json.Unmarshaler            = (*CleanupReceipt)(nil)
)
