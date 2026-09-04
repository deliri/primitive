package runnercontrol

import (
	"bytes"
	"context"
	"crypto"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/runprotocol"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	ExperimentManifestMaximumEntries     = 256
	RunnerCompletionPayloadMaximumBytes  = 900 * 1024
	RunnerCompletionDocumentMaximumBytes = 1 << 20
	RunnerCompletionReceiptMaximumBytes  = 16 * 1024
)

type SourceGrantIdentity struct {
	Digest core.SHA256Digest `json:"digest"`
}

func (i SourceGrantIdentity) Validate() error { return i.Digest.Validate() }

type AttemptedSource struct {
	Repository runprotocol.RepositoryIdentity `json:"repository"`
	Commit     core.BuildCommit               `json:"commit"`
}

func (s AttemptedSource) Validate() error {
	return errors.Join(s.Repository.Validate(), s.Commit.Validate())
}

type ExperimentManifestEntry struct {
	Probe                  runprotocol.ProbeIdentity `json:"probe"`
	CompletionBytes        core.ByteLength           `json:"completion_bytes"`
	CompletionDigest       core.SHA256Digest         `json:"completion_digest"`
	ArtifactManifestDigest core.SHA256Digest         `json:"artifact_manifest_digest"`
	Experiment             runprotocol.ExperimentID  `json:"experiment_id"`
}

func (e ExperimentManifestEntry) Validate() error {
	if err := errors.Join(e.Experiment.Validate(), e.Probe.Validate(), e.CompletionDigest.Validate(), e.CompletionBytes.Validate(), e.ArtifactManifestDigest.Validate()); err != nil {
		return err
	}
	if e.Probe.Role != runprotocol.ProbeRoleExperiment {
		return core.ErrPrimitiveContract
	}
	return nil
}

type ExperimentManifest struct {
	Entries []ExperimentManifestEntry `json:"entries"`
}

func (m ExperimentManifest) Validate() error {
	if len(m.Entries) > ExperimentManifestMaximumEntries {
		return core.ErrPrimitiveContract
	}
	for index := range m.Entries {
		if err := m.Entries[index].Validate(); err != nil {
			return err
		}
		for previous := range index {
			if m.Entries[previous].Experiment == m.Entries[index].Experiment {
				return core.ErrPrimitiveContract
			}
		}
	}
	return nil
}

type PreSourceRunnerCompletion struct {
	Attempted AttemptedSource    `json:"attempted_source"`
	Failure   core.ErrorIdentity `json:"failure"`
}

func (c PreSourceRunnerCompletion) Validate() error {
	return errors.Join(c.Attempted.Validate(), c.Failure.Validate())
}

type SelectionRunnerCompletion struct {
	Selection       *runprotocol.SelectionObservation `json:"selection,omitempty"`
	Failure         *core.ErrorIdentity               `json:"failure,omitempty"`
	ExperimentFacts *ExperimentManifest               `json:"experiment_manifest,omitempty"`
	Source          runprotocol.SourceCoordinate      `json:"source"`
	Probe           runprotocol.ProbeIdentity         `json:"probe"`
}

func (c SelectionRunnerCompletion) Validate() error {
	if err := errors.Join(c.Source.Validate(), c.Probe.Validate()); err != nil {
		return err
	}
	if c.Probe.Role != runprotocol.ProbeRoleSelection || c.Probe.Source != c.Source {
		return core.ErrPrimitiveContract
	}
	if (c.Selection == nil) == (c.Failure == nil) {
		return core.ErrPrimitiveContract
	}
	if c.Failure != nil {
		if c.ExperimentFacts != nil {
			return core.ErrPrimitiveContract
		}
		return c.Failure.Validate()
	}
	return c.validateSelection()
}

func (c SelectionRunnerCompletion) validateSelection() error {
	if err := c.Selection.Validate(); err != nil {
		return err
	}
	if c.Selection.Executed == 0 {
		if c.ExperimentFacts != nil {
			return core.ErrPrimitiveContract
		}
		return nil
	}
	if c.ExperimentFacts == nil || len(c.ExperimentFacts.Entries) != int(c.Selection.Executed) {
		return core.ErrPrimitiveContract
	}
	if err := c.ExperimentFacts.Validate(); err != nil {
		return err
	}
	return c.validateExperimentFacts()
}

func (c SelectionRunnerCompletion) validateExperimentFacts() error {
	for index := range c.ExperimentFacts.Entries {
		if !experimentMatchesSelection(c.ExperimentFacts.Entries[index].Probe, c.Probe, c.Selection.ExpansionIdentity) {
			return core.ErrPrimitiveContract
		}
	}
	return nil
}

type DirectExperimentRunnerCompletion struct {
	Source     runprotocol.SourceCoordinate `json:"source"`
	Probe      runprotocol.ProbeIdentity    `json:"probe"`
	Experiment ExperimentManifestEntry      `json:"experiment"`
}

func (c DirectExperimentRunnerCompletion) Validate() error {
	if err := errors.Join(c.Source.Validate(), c.Probe.Validate(), c.Experiment.Validate()); err != nil {
		return err
	}
	if c.Probe.Role != runprotocol.ProbeRoleExperiment || c.Probe.Source != c.Source {
		return core.ErrPrimitiveContract
	}
	return equalProbeIdentity(c.Probe, c.Experiment.Probe)
}

func experimentMatchesSelection(child, selection runprotocol.ProbeIdentity, expansion core.SHA256Digest) bool {
	if !experimentMatchesSelectionScalars(child, selection) {
		return false
	}
	if child.Parent.Kind != selection.Kind || child.Parent.ExpansionDigest != expansion {
		return false
	}
	left, leftErr := core.MarshalCanonicalJSONDocument(child.Parent.Target)
	right, rightErr := core.MarshalCanonicalJSONDocument(selection.Target)
	return leftErr == nil && rightErr == nil && bytes.Equal(left, right)
}

func experimentMatchesSelectionScalars(child, selection runprotocol.ProbeIdentity) bool {
	if child.Parent == nil {
		return false
	}
	return child.Origin == selection.Origin && child.Subject == selection.Subject && child.Source == selection.Source && child.Profile == selection.Profile && child.Environment == selection.Environment
}

func equalProbeIdentity(left, right runprotocol.ProbeIdentity) error {
	leftBytes, leftErr := core.MarshalCanonicalJSONDocument(left)
	rightBytes, rightErr := core.MarshalCanonicalJSONDocument(right)
	if leftErr != nil || rightErr != nil || !bytes.Equal(leftBytes, rightBytes) {
		return errors.Join(core.ErrPrimitiveContract, leftErr, rightErr)
	}
	return nil
}

type RunnerCompletionPayload struct {
	DirectExperiment       *DirectExperimentRunnerCompletion `json:"direct_experiment,omitempty"`
	Selection              *SelectionRunnerCompletion        `json:"selection,omitempty"`
	PreSource              *PreSourceRunnerCompletion        `json:"pre_source,omitempty"`
	Members                MemberSet                         `json:"member_set"`
	Fence                  SchedulingFence                   `json:"fence"`
	CompletedAt            temporal.Instant                  `json:"completed_at"`
	BeganAt                temporal.Instant                  `json:"began_at"`
	SchemaVersion          uint16                            `json:"schema_version"`
	ArtifactManifestDigest core.SHA256Digest                 `json:"artifact_manifest_digest"`
	SourceGrant            SourceGrantIdentity               `json:"source_grant"`
	AdmittedIntentDigest   core.SHA256Digest                 `json:"admitted_intent_digest"`
	Run                    runprotocol.RunID                 `json:"run_id"`
	Terminal               runprotocol.TerminalState         `json:"terminal"`
}

func (p RunnerCompletionPayload) Validate() error {
	if p.SchemaVersion != SchemaVersion {
		return core.ErrPrimitiveContract
	}
	if err := errors.Join(p.Run.Validate(), p.AdmittedIntentDigest.Validate(), p.SourceGrant.Validate(), p.Fence.Validate(), p.Members.Validate(), p.Terminal.Validate(), p.BeganAt.Validate(), p.CompletedAt.Validate(), p.ArtifactManifestDigest.Validate()); err != nil {
		return err
	}
	if err := p.validateMembership(); err != nil {
		return err
	}
	comparison, err := p.BeganAt.Compare(p.CompletedAt)
	if err != nil || comparison == core.ComparisonGreater {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	if p.variantCount() != 1 {
		return core.ErrPrimitiveContract
	}
	return p.validateVariant()
}

func (p RunnerCompletionPayload) validateMembership() error {
	memberDigest, err := p.Members.Digest()
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	if memberDigest != p.Fence.MemberSetDigest || !p.Members.Contains(p.Run) {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (p RunnerCompletionPayload) validateVariant() error {
	if p.PreSource != nil {
		if p.Terminal == runprotocol.TerminalCompleted {
			return core.ErrPrimitiveContract
		}
		return p.PreSource.Validate()
	}
	if p.Selection != nil {
		return p.Selection.Validate()
	}
	return p.DirectExperiment.Validate()
}

func (p RunnerCompletionPayload) variantCount() int {
	count := 0
	for _, present := range [...]bool{p.PreSource != nil, p.Selection != nil, p.DirectExperiment != nil} {
		if present {
			count++
		}
	}
	return count
}

func (RunnerCompletionPayload) AttestationDomain() CompletionSigningDomain {
	return CompletionSigningDomainRunnerV1
}

func (p RunnerCompletionPayload) WriteCanonical(destination io.Writer) error {
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

type runnerCompletionPayloadWire RunnerCompletionPayload

func (p RunnerCompletionPayload) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(runnerCompletionPayloadWire(p))
	if err != nil || len(encoded) > RunnerCompletionPayloadMaximumBytes {
		return nil, errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract, err)
	}
	return encoded, nil
}

func (p *RunnerCompletionPayload) UnmarshalJSON(data []byte) error {
	if p == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[runnerCompletionPayloadWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := RunnerCompletionPayload(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*p = candidate
	return nil
}

type RunnerCompletionDocument struct {
	Payload     RunnerCompletionPayload                  `json:"payload"`
	Attestation attest.Envelope[CompletionSigningDomain] `json:"attestation"`
}

func (d RunnerCompletionDocument) Validate() error {
	if err := errors.Join(d.Payload.Validate(), d.Attestation.Validate()); err != nil {
		return err
	}
	if d.Attestation.Domain != d.Payload.AttestationDomain() {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (d RunnerCompletionDocument) IdempotencyKey() (exchange.IdempotencyKey, error) {
	encoded, err := d.MarshalJSON()
	if err != nil {
		return exchange.IdempotencyKey{}, err
	}
	digest := core.SHA256Of(encoded)
	hex, err := digest.Hex()
	if err != nil {
		return exchange.IdempotencyKey{}, err
	}
	return exchange.ParseIdempotencyKey("runner-completion:" + hex)
}

type runnerCompletionDocumentWire RunnerCompletionDocument

func (d RunnerCompletionDocument) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(runnerCompletionDocumentWire(d))
	if err != nil || len(encoded) > RunnerCompletionDocumentMaximumBytes {
		return nil, errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract, err)
	}
	return encoded, nil
}

func (d *RunnerCompletionDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[runnerCompletionDocumentWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := RunnerCompletionDocument(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*d = candidate
	return nil
}

func IssueRunnerCompletion(payload RunnerCompletionPayload, signer crypto.Signer) (RunnerCompletionDocument, error) {
	envelope, err := attest.Sign(attest.SignRequest[CompletionSigningDomain]{Body: payload, Signer: signer})
	if err != nil {
		return RunnerCompletionDocument{}, err
	}
	document := RunnerCompletionDocument{Payload: payload, Attestation: envelope}
	return document, document.Validate()
}

func VerifyRunnerCompletion(document RunnerCompletionDocument, trusted attest.TrustedKeys) error {
	if err := document.Validate(); err != nil {
		return err
	}
	proof, err := attest.Verify(attest.VerifyRequest[CompletionSigningDomain]{Body: document.Payload, Envelope: document.Attestation, TrustedKeys: trusted})
	if err != nil {
		return err
	}
	return proof.Validate()
}

type RunnerCompletionRecord struct {
	Canonical []byte
	Document  RunnerCompletionDocument
	Bytes     core.ByteLength
	Digest    core.SHA256Digest
}

func NewRunnerCompletionRecord(document RunnerCompletionDocument) (RunnerCompletionRecord, error) {
	canonical, err := document.MarshalJSON()
	if err != nil {
		return RunnerCompletionRecord{}, err
	}
	extent, err := core.NewByteLength(uint64(len(canonical)))
	if err != nil {
		return RunnerCompletionRecord{}, err
	}
	record := RunnerCompletionRecord{Document: document, Canonical: canonical, Digest: core.SHA256Of(canonical), Bytes: extent}
	return record, record.Validate()
}

func (r RunnerCompletionRecord) Validate() error {
	if err := errors.Join(r.Document.Validate(), r.Digest.Validate(), r.Bytes.Validate()); err != nil {
		return err
	}
	encoded, err := r.Document.MarshalJSON()
	if err != nil || !bytes.Equal(encoded, r.Canonical) || r.Bytes.Uint64() != uint64(len(r.Canonical)) || r.Digest != core.SHA256Of(r.Canonical) {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	return nil
}

type RunnerCompletionReceipt struct {
	SchemaVersion uint16            `json:"schema_version"`
	Run           runprotocol.RunID `json:"run_id"`
	Digest        core.SHA256Digest `json:"document_digest"`
	Bytes         core.ByteLength   `json:"document_bytes"`
}

func (r RunnerCompletionReceipt) Validate() error {
	if r.SchemaVersion != SchemaVersion {
		return core.ErrPrimitiveContract
	}
	return errors.Join(r.Run.Validate(), r.Digest.Validate(), r.Bytes.Validate())
}

type runnerCompletionReceiptWire RunnerCompletionReceipt

func (r RunnerCompletionReceipt) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONDocument(runnerCompletionReceiptWire(r))
}

func (r *RunnerCompletionReceipt) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[runnerCompletionReceiptWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := RunnerCompletionReceipt(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*r = candidate
	return nil
}

type RunnerCompletionRepository interface {
	StoreRunnerCompletion(context.Context, RunnerCompletionRecord) error
}

type RunnerCompletionClient struct{ socket exchange.ClientSocket }

func NewRunnerCompletionClient(configuration exchange.ClientSocketConfiguration) (RunnerCompletionClient, error) {
	socket, err := exchange.NewClientSocket(configuration)
	if err != nil {
		return RunnerCompletionClient{}, err
	}
	return RunnerCompletionClient{socket: socket}, nil
}

func (c RunnerCompletionClient) Submit(ctx context.Context, document RunnerCompletionDocument) (exchange.JSONResponse[RunnerCompletionReceipt], error) {
	return exchange.SendReplayBoundSocketJSON[RunnerCompletionDocument, RunnerCompletionReceipt](ctx, c.socket, document)
}

type RunnerCompletionServer struct {
	repository RunnerCompletionRepository
	socket     exchange.ServerSocket
	trusted    attest.TrustedKeys
}

func NewRunnerCompletionServer(contract exchange.JSONSocketContract, repository RunnerCompletionRepository, trusted attest.TrustedKeys) (RunnerCompletionServer, error) {
	if repository == nil || trusted.Validate() != nil {
		return RunnerCompletionServer{}, core.ErrPrimitiveContract
	}
	socket, err := exchange.NewServerSocket(contract)
	if err != nil {
		return RunnerCompletionServer{}, err
	}
	return RunnerCompletionServer{socket: socket, repository: repository, trusted: trusted}, nil
}

func (s RunnerCompletionServer) Serve(call exchange.SocketServerCall) error {
	if s.repository == nil {
		return core.ErrPrimitiveContract
	}
	ctx, err := call.Context()
	if err != nil {
		return err
	}
	received, err := exchange.ReceiveReplayBoundSocketJSON[RunnerCompletionDocument, *RunnerCompletionDocument](s.socket, call)
	if err != nil {
		return err
	}
	payload := received.Body.Payload
	machine := payload.Fence.Machine
	if err := RequireRunnerPeer(ctx, machine.Machine, machine.Generation); err != nil {
		return err
	}
	if err := VerifyRunnerCompletion(*received.Body, s.trusted); err != nil {
		return fmt.Errorf("verify runner completion for run %v: %w", received.Body.Payload.Run, err)
	}
	record, err := NewRunnerCompletionRecord(*received.Body)
	if err != nil {
		return err
	}
	if err := s.repository.StoreRunnerCompletion(ctx, record); err != nil {
		return err
	}
	receipt := RunnerCompletionReceipt{SchemaVersion: SchemaVersion, Run: record.Document.Payload.Run, Digest: record.Digest, Bytes: record.Bytes}
	return exchange.WriteSocketJSON(s.socket, call, receipt)
}

func RunnerCompletionSocketContract(path exchange.SocketRoutePath) (exchange.JSONSocketContract, error) {
	requestLimit, requestErr := core.NewByteCount(RunnerCompletionDocumentMaximumBytes)
	responseLimit, responseErr := core.NewByteCount(RunnerCompletionReceiptMaximumBytes)
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
	_ core.Validatable            = SourceGrantIdentity{}
	_ core.Validatable            = AttemptedSource{}
	_ core.Validatable            = ExperimentManifestEntry{}
	_ core.Validatable            = ExperimentManifest{}
	_ core.Validatable            = PreSourceRunnerCompletion{}
	_ core.Validatable            = SelectionRunnerCompletion{}
	_ core.Validatable            = DirectExperimentRunnerCompletion{}
	_ core.ValidatedJSONMarshaler = RunnerCompletionPayload{}
	_ json.Unmarshaler            = (*RunnerCompletionPayload)(nil)
	_ core.ValidatedJSONMarshaler = RunnerCompletionDocument{}
	_ exchange.IdempotencyBound   = RunnerCompletionDocument{}
	_ json.Unmarshaler            = (*RunnerCompletionDocument)(nil)
	_ core.Validatable            = RunnerCompletionRecord{}
	_ core.ValidatedJSONMarshaler = RunnerCompletionReceipt{}
	_ json.Unmarshaler            = (*RunnerCompletionReceipt)(nil)
)
