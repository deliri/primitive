package runnercontrol

import (
	"bytes"
	"context"
	"crypto"
	json "encoding/json/v2"
	"errors"
	"io"
	"slices"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runprotocol"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	RetainedExperimentMaximum          = 256
	ExperimentDeliveryEntryMaximum     = 64
	ExperimentDeliveryPageMaximum      = 16
	ExperimentDeliveryPageMaximumBytes = 8 * core.JSONDocumentMaximumBytes
)

type CleanupOutcomeKind uint8

const (
	CleanupOutcomeUnknown CleanupOutcomeKind = iota
	CleanupSucceeded
	CleanupFailed
	CleanupNotApplicable
	cleanupOutcomeLimit
)

func (k CleanupOutcomeKind) Validate() error {
	if k <= CleanupOutcomeUnknown || k >= cleanupOutcomeLimit {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (k CleanupOutcomeKind) IsValid() bool { return k.Validate() == nil }

func (k CleanupOutcomeKind) String() string {
	if !k.IsValid() {
		return invalidEnumString()
	}
	return []string{"", "cleanup_succeeded", "cleanup_failed", evidenceNotApplicableText}[k]
}

func (k CleanupOutcomeKind) MarshalJSON() ([]byte, error) {
	if err := k.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(k.String())
}

func (k *CleanupOutcomeKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	for candidate := CleanupOutcomeUnknown + 1; candidate < cleanupOutcomeLimit; candidate++ {
		if candidate.String() == value {
			*k = candidate
			return nil
		}
	}
	return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
}

type CleanupOutcome struct {
	ReceiptDigest *core.SHA256Digest  `json:"receipt_digest,omitempty"`
	Failure       *core.ErrorIdentity `json:"failure,omitempty"`
	Kind          CleanupOutcomeKind  `json:"kind"`
}

func (o CleanupOutcome) Validate() error {
	if err := o.Kind.Validate(); err != nil {
		return err
	}
	return o.validateShape()
}

func (o CleanupOutcome) validateShape() error {
	switch o.Kind {
	case CleanupSucceeded:
		if o.ReceiptDigest == nil || o.Failure != nil {
			return core.ErrPrimitiveContract
		}
		return o.ReceiptDigest.Validate()
	case CleanupFailed:
		if o.ReceiptDigest != nil || o.Failure == nil {
			return core.ErrPrimitiveContract
		}
		return o.Failure.Validate()
	case CleanupNotApplicable:
		if o.ReceiptDigest != nil || o.Failure != nil {
			return core.ErrPrimitiveContract
		}
		return nil
	default:
		return core.ErrPrimitiveContract
	}
}
func (o CleanupOutcome) Digest() (core.SHA256Digest, error) {
	if err := o.Validate(); err != nil {
		return core.SHA256Digest{}, err
	}
	encoded, err := core.MarshalCanonicalJSONDocument(o)
	if err != nil {
		return core.SHA256Digest{}, err
	}
	return core.SHA256Of(encoded), nil
}

type EvidenceBodyKind uint8

const (
	EvidenceBodyUnknown EvidenceBodyKind = iota
	EvidenceCompletedRunner
	EvidenceInterruptedRunner
	EvidencePreRunnerInfrastructure
	evidenceBodyLimit
)

func (k EvidenceBodyKind) Validate() error {
	if k <= EvidenceBodyUnknown || k >= evidenceBodyLimit {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (k EvidenceBodyKind) IsValid() bool { return k.Validate() == nil }

func (k EvidenceBodyKind) String() string {
	if !k.IsValid() {
		return invalidEnumString()
	}
	return []string{"", "completed-runner", "interrupted-runner", "pre-runner-infrastructure"}[k]
}

func (k EvidenceBodyKind) MarshalJSON() ([]byte, error) {
	if err := k.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(k.String())
}

func (k *EvidenceBodyKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	for candidate := EvidenceBodyUnknown + 1; candidate < evidenceBodyLimit; candidate++ {
		if candidate.String() == value {
			*k = candidate
			return nil
		}
	}
	return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
}

type InterruptedRunnerEvidence struct {
	ArtifactManifestDigest *core.SHA256Digest             `json:"artifact_manifest_digest,omitempty"`
	Experiments            []ExperimentCompletionDocument `json:"retained_experiments"`
	Interruption           core.ErrorIdentity             `json:"interruption"`
}

func (e InterruptedRunnerEvidence) Validate() error {
	if err := e.Interruption.Validate(); err != nil {
		return err
	}
	if len(e.Experiments) > RetainedExperimentMaximum {
		return core.ErrPrimitiveContract
	}
	seen := make([]runprotocol.ExperimentID, 0, len(e.Experiments))
	for index := range e.Experiments {
		if err := e.Experiments[index].Validate(); err != nil {
			return err
		}
		id := e.Experiments[index].Payload.Observation.Experiment
		if slices.Contains(seen, id) {
			return core.ErrPrimitiveContract
		}
		seen = append(seen, id)
	}
	if e.ArtifactManifestDigest != nil {
		return e.ArtifactManifestDigest.Validate()
	}
	return nil
}

type PreRunnerEvidence struct {
	Stage   runprotocol.Identifier `json:"stage"`
	Failure core.ErrorIdentity  `json:"failure"`
}

func (e PreRunnerEvidence) Validate() error {
	return errors.Join(e.Stage.Validate(), e.Failure.Validate())
}

type ObservationEvidenceBody struct {
	Completed   *RunnerCompletionDocument  `json:"completed,omitempty"`
	Interrupted *InterruptedRunnerEvidence `json:"interrupted,omitempty"`
	PreRunner   *PreRunnerEvidence         `json:"pre_runner,omitempty"`
	Kind        EvidenceBodyKind           `json:"kind"`
}

func (b ObservationEvidenceBody) Validate() error {
	if err := b.Kind.Validate(); err != nil {
		return err
	}
	count := 0
	for _, present := range []bool{b.Completed != nil, b.Interrupted != nil, b.PreRunner != nil} {
		if present {
			count++
		}
	}
	if count != 1 {
		return core.ErrPrimitiveContract
	}
	return b.validateSelectedBody()
}

func (b ObservationEvidenceBody) validateSelectedBody() error {
	switch b.Kind {
	case EvidenceCompletedRunner:
		if b.Completed == nil {
			return core.ErrPrimitiveContract
		}
		return b.Completed.Validate()
	case EvidenceInterruptedRunner:
		if b.Interrupted == nil {
			return core.ErrPrimitiveContract
		}
		return b.Interrupted.Validate()
	case EvidencePreRunnerInfrastructure:
		if b.PreRunner == nil {
			return core.ErrPrimitiveContract
		}
		return b.PreRunner.Validate()
	default:
		return core.ErrPrimitiveContract
	}
}

type ObservationEnvelopePayload struct {
	Evidence                   ObservationEvidenceBody  `json:"evidence"`
	Cleanup                    CleanupOutcome           `json:"cleanup"`
	Audience                   runprotocol.Identifier      `json:"audience"`
	Destination                runprotocol.Identifier      `json:"destination"`
	Origin                     runprotocol.OriginIdentity  `json:"origin"`
	Members                    MemberSet                `json:"member_set"`
	Fence                      SchedulingFence          `json:"fence"`
	CapturedAt                 temporal.Instant         `json:"captured_at"`
	SchemaVersion              uint16                   `json:"schema_version"`
	CleanupDigest              core.SHA256Digest        `json:"cleanup_digest"`
	DeliveryGrant              core.SHA256Digest        `json:"delivery_grant"`
	AdmissionDigest            core.SHA256Digest        `json:"admission_digest"`
	ExperimentDeliveryManifest core.SHA256Digest        `json:"experiment_delivery_manifest"`
	Run                        runprotocol.RunID           `json:"run_id"`
	Request                    runprotocol.RequestIdentity `json:"request_id"`
	Terminal                   runprotocol.TerminalState   `json:"terminal"`
}

func (p ObservationEnvelopePayload) Validate() error {
	if p.SchemaVersion != SchemaVersion {
		return core.ErrPrimitiveContract
	}
	if err := p.validateOwnedValues(); err != nil {
		return err
	}
	members, err := p.Members.Digest()
	cleanup, cleanupErr := p.Cleanup.Digest()
	if err != nil || cleanupErr != nil {
		return errors.Join(core.ErrPrimitiveContract, err, cleanupErr)
	}
	if members != p.Fence.MemberSetDigest || !p.Members.Contains(p.Run) || cleanup != p.CleanupDigest {
		return core.ErrPrimitiveContract
	}
	if err := p.validateTerminalClosure(); err != nil {
		return err
	}
	if err := p.validateCompletedEvidence(members); err != nil {
		return err
	}
	return p.validateInterruptedEvidence()
}

func (p ObservationEnvelopePayload) validateOwnedValues() error {
	return errors.Join(p.Request.Validate(), p.Run.Validate(), p.AdmissionDigest.Validate(), p.Fence.Validate(), p.Members.Validate(), p.Cleanup.Validate(), p.CleanupDigest.Validate(), p.Origin.Validate(), p.DeliveryGrant.Validate(), p.Destination.Validate(), p.Audience.Validate(), p.Terminal.Validate(), p.CapturedAt.Validate(), p.ExperimentDeliveryManifest.Validate(), p.Evidence.Validate())
}

func (p ObservationEnvelopePayload) validateTerminalClosure() error {
	if p.Evidence.Kind != EvidenceCompletedRunner && p.Terminal == runprotocol.TerminalCompleted {
		return core.ErrPrimitiveContract
	}
	if p.Cleanup.Kind != CleanupSucceeded && p.Terminal == runprotocol.TerminalCompleted {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (p ObservationEnvelopePayload) validateCompletedEvidence(members core.SHA256Digest) error {
	if p.Evidence.Completed != nil && p.Evidence.Completed.Payload.Run != p.Run {
		return core.ErrPrimitiveContract
	}
	if p.Evidence.Completed != nil {
		completed := p.Evidence.Completed.Payload
		return p.validateCompletedPayload(completed, members)
	}
	return nil
}

func (p ObservationEnvelopePayload) validateCompletedPayload(completed RunnerCompletionPayload, members core.SHA256Digest) error {
	completedMembers, err := completed.Members.Digest()
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	if completed.Fence != p.Fence || completedMembers != members || completedMembers != p.Fence.MemberSetDigest {
		return core.ErrPrimitiveContract
	}
	if completed.DirectExperiment != nil && completed.DirectExperiment.Probe.Origin != p.Origin {
		return core.ErrPrimitiveContract
	}
	if completed.Selection != nil && completed.Selection.Probe.Origin != p.Origin {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (p ObservationEnvelopePayload) validateInterruptedEvidence() error {
	if p.Evidence.Interrupted != nil {
		for index := range p.Evidence.Interrupted.Experiments {
			experiment := p.Evidence.Interrupted.Experiments[index].Payload
			if experiment.Run != p.Run || experiment.Fence != p.Fence || experiment.Probe.Origin != p.Origin {
				return core.ErrPrimitiveContract
			}
		}
	}
	return nil
}
func (ObservationEnvelopePayload) AttestationDomain() EvidenceSigningDomain {
	return EvidenceSigningDomainObservationV1
}
func (p ObservationEnvelopePayload) WriteCanonical(destination io.Writer) error {
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

type observationEnvelopePayloadWire ObservationEnvelopePayload

func (p ObservationEnvelopePayload) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONDocument(observationEnvelopePayloadWire(p))
}
func (p *ObservationEnvelopePayload) UnmarshalJSON(data []byte) error {
	if p == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[observationEnvelopePayloadWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := ObservationEnvelopePayload(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*p = candidate
	return nil
}

type ObservationEnvelope struct {
	Payload     ObservationEnvelopePayload             `json:"payload"`
	Attestation attest.Envelope[EvidenceSigningDomain] `json:"attestation"`
}

func (e ObservationEnvelope) Validate() error {
	if err := errors.Join(e.Payload.Validate(), e.Attestation.Validate()); err != nil {
		return err
	}
	if e.Attestation.Domain != e.Payload.AttestationDomain() {
		return core.ErrPrimitiveContract
	}
	return nil
}

type observationEnvelopeWire ObservationEnvelope

func (e ObservationEnvelope) MarshalJSON() ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONDocument(observationEnvelopeWire(e))
}
func (e *ObservationEnvelope) UnmarshalJSON(data []byte) error {
	if e == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[observationEnvelopeWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := ObservationEnvelope(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*e = candidate
	return nil
}
func IssueObservationEnvelope(payload ObservationEnvelopePayload, signer crypto.Signer) (ObservationEnvelope, error) {
	envelope, err := attest.Sign(attest.SignRequest[EvidenceSigningDomain]{Body: payload, Signer: signer})
	if err != nil {
		return ObservationEnvelope{}, err
	}
	document := ObservationEnvelope{Payload: payload, Attestation: envelope}
	return document, document.Validate()
}
func VerifyObservationEnvelope(document ObservationEnvelope, trusted attest.TrustedKeys) error {
	if err := document.Validate(); err != nil {
		return err
	}
	proof, err := attest.Verify(attest.VerifyRequest[EvidenceSigningDomain]{Body: document.Payload, Envelope: document.Attestation, TrustedKeys: trusted})
	if err != nil {
		return err
	}
	return proof.Validate()
}

type ExperimentDeliveryEntry struct {
	Probe      runprotocol.ProbeIdentity `json:"probe"`
	Bytes      core.ByteLength        `json:"bytes"`
	Page       uint16                 `json:"page"`
	Position   uint16                 `json:"position"`
	Digest     core.SHA256Digest      `json:"digest"`
	Experiment runprotocol.ExperimentID  `json:"experiment_id"`
}

func (e ExperimentDeliveryEntry) Validate() error {
	if e.Page == 0 {
		return core.ErrPrimitiveContract
	}
	return errors.Join(e.Experiment.Validate(), e.Probe.Validate(), e.Digest.Validate(), e.Bytes.Validate())
}

type ExperimentDeliveryManifest struct {
	Entries       []ExperimentDeliveryEntry `json:"entries"`
	SchemaVersion uint16                    `json:"schema_version"`
	PageCount     uint16                    `json:"page_count"`
	Run           runprotocol.RunID            `json:"run_id"`
}

func (m ExperimentDeliveryManifest) Validate() error {
	if err := m.validateHeader(); err != nil {
		return err
	}
	if err := m.validateEntries(); err != nil {
		return err
	}
	return m.validateFinalPage()
}

func (m ExperimentDeliveryManifest) validateHeader() error {
	if m.SchemaVersion != SchemaVersion || len(m.Entries) > RetainedExperimentMaximum {
		return core.ErrPrimitiveContract
	}
	if m.PageCount > ExperimentDeliveryPageMaximum {
		return core.ErrPrimitiveContract
	}
	if err := m.Run.Validate(); err != nil {
		return err
	}
	if len(m.Entries) == 0 && m.PageCount != 0 {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (m ExperimentDeliveryManifest) validateEntries() error {
	expectedPage := uint16(1)
	expectedPosition := uint16(0)
	for index := range m.Entries {
		if err := m.Entries[index].Validate(); err != nil {
			return err
		}
		if m.Entries[index].Page > m.PageCount {
			return core.ErrPrimitiveContract
		}
		if beginsNextDeliveryPage(m.Entries[index], expectedPage, expectedPosition) {
			expectedPage++
			expectedPosition = 0
		}
		if m.Entries[index].Page != expectedPage || m.Entries[index].Position != expectedPosition || expectedPosition >= ExperimentDeliveryEntryMaximum {
			return core.ErrPrimitiveContract
		}
		expectedPosition++
		if deliveryEntryDuplicatesEarlier(m.Entries, index) {
			return core.ErrPrimitiveContract
		}
	}
	return nil
}

func beginsNextDeliveryPage(entry ExperimentDeliveryEntry, expectedPage, expectedPosition uint16) bool {
	return entry.Page == expectedPage+1 && entry.Position == 0 && expectedPosition > 0
}

func deliveryEntryDuplicatesEarlier(entries []ExperimentDeliveryEntry, index int) bool {
	for previous := range index {
		if entries[previous].Experiment == entries[index].Experiment {
			return true
		}
		if entries[previous].Page == entries[index].Page && entries[previous].Position == entries[index].Position {
			return true
		}
	}
	return false
}

func (m ExperimentDeliveryManifest) validateFinalPage() error {
	expectedPage := uint16(0)
	if len(m.Entries) > 0 {
		expectedPage = m.Entries[len(m.Entries)-1].Page
	}
	if len(m.Entries) > 0 && m.PageCount != expectedPage {
		return core.ErrPrimitiveContract
	}
	return nil
}
func (m ExperimentDeliveryManifest) Digest() (core.SHA256Digest, error) {
	if err := m.Validate(); err != nil {
		return core.SHA256Digest{}, err
	}
	encoded, err := core.MarshalCanonicalJSONDocument(m)
	if err != nil {
		return core.SHA256Digest{}, err
	}
	return core.SHA256Of(encoded), nil
}

type ExperimentDeliveryPage struct {
	Documents     []ExperimentCompletionDocument `json:"documents"`
	SchemaVersion uint16                         `json:"schema_version"`
	Page          uint16                         `json:"page"`
	Run           runprotocol.RunID                 `json:"run_id"`
}

func (p ExperimentDeliveryPage) Validate() error {
	if p.SchemaVersion != SchemaVersion || p.Page == 0 || len(p.Documents) == 0 || len(p.Documents) > ExperimentDeliveryEntryMaximum {
		return core.ErrPrimitiveContract
	}
	if err := p.Run.Validate(); err != nil {
		return err
	}
	for index := range p.Documents {
		if err := p.Documents[index].Validate(); err != nil || p.Documents[index].Payload.Run != p.Run {
			return errors.Join(core.ErrPrimitiveContract, err)
		}
	}
	return nil
}

type experimentDeliveryPageWire ExperimentDeliveryPage

func (p ExperimentDeliveryPage) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(experimentDeliveryPageWire(p))
	if err != nil || len(encoded) > ExperimentDeliveryPageMaximumBytes {
		return nil, errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract, err)
	}
	return encoded, nil
}

func (p *ExperimentDeliveryPage) UnmarshalJSON(data []byte) error {
	if p == nil || len(data) > ExperimentDeliveryPageMaximumBytes {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[experimentDeliveryPageWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := ExperimentDeliveryPage(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*p = candidate
	return nil
}

type ObservationDeliveryRepository interface {
	StoreObservationDelivery(context.Context, ObservationEnvelope, ExperimentDeliveryManifest) error
}

type ObservationDeliveryVerification struct {
	Pages       []ExperimentDeliveryPage
	Stage       ObservationDeliveryStage
	ControlKeys attest.TrustedKeys
	RunnerKeys  attest.TrustedKeys
}

func (v ObservationDeliveryVerification) Validate() error {
	return errors.Join(v.Stage.Validate(), v.ControlKeys.Validate(), v.RunnerKeys.Validate())
}

func VerifyObservationDelivery(verification ObservationDeliveryVerification) error {
	if err := verification.Validate(); err != nil {
		return err
	}
	envelope := verification.Stage.Envelope
	manifest := verification.Stage.Manifest
	if err := VerifyObservationEnvelope(envelope, verification.ControlKeys); err != nil {
		return err
	}
	if len(verification.Pages) != int(manifest.PageCount) {
		return core.ErrPrimitiveContract
	}
	digest, err := manifest.Digest()
	if err != nil || digest != envelope.Payload.ExperimentDeliveryManifest {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	documents, err := verifyDeliveryPages(manifest, verification.Pages, verification.RunnerKeys)
	if err != nil {
		return err
	}
	return (deliveryClosure{body: envelope.Payload.Evidence, manifest: manifest, documents: documents, runnerKeys: verification.RunnerKeys}).verify()
}

type deliveryClosure struct {
	body       ObservationEvidenceBody
	documents  []ExperimentCompletionDocument
	manifest   ExperimentDeliveryManifest
	runnerKeys attest.TrustedKeys
}

func verifyDeliveryPages(manifest ExperimentDeliveryManifest, pages []ExperimentDeliveryPage, runnerKeys attest.TrustedKeys) ([]ExperimentCompletionDocument, error) {
	documents := make([]ExperimentCompletionDocument, len(manifest.Entries))
	if err := validateDeliveryPages(manifest, pages); err != nil {
		return nil, err
	}
	for index := range manifest.Entries {
		document, err := verifyDeliveryEntry(manifest.Entries[index], pages, runnerKeys)
		if err != nil {
			return nil, err
		}
		documents[index] = document
	}
	if err := validateDeliveryPageCoverage(manifest, pages); err != nil {
		return nil, err
	}
	return documents, nil
}

func validateDeliveryPages(manifest ExperimentDeliveryManifest, pages []ExperimentDeliveryPage) error {
	for pageIndex := range pages {
		page := pages[pageIndex]
		if err := page.Validate(); err != nil {
			return errors.Join(core.ErrPrimitiveContract, err)
		}
		pageNumber, err := core.CheckedUint16FromInt(pageIndex + 1)
		if err != nil || page.Run != manifest.Run || page.Page != pageNumber {
			return core.ErrPrimitiveContract
		}
	}
	return nil
}

func verifyDeliveryEntry(entry ExperimentDeliveryEntry, pages []ExperimentDeliveryPage, runnerKeys attest.TrustedKeys) (ExperimentCompletionDocument, error) {
	page := pages[int(entry.Page)-1]
	if int(entry.Position) >= len(page.Documents) {
		return ExperimentCompletionDocument{}, core.ErrPrimitiveContract
	}
	document := page.Documents[entry.Position]
	if err := VerifyExperimentCompletion(document, runnerKeys); err != nil {
		return ExperimentCompletionDocument{}, err
	}
	record, err := NewExperimentCompletionRecord(document)
	if err != nil {
		return ExperimentCompletionDocument{}, errors.Join(core.ErrPrimitiveContract, err)
	}
	if document.Payload.Observation.Experiment != entry.Experiment || record.Digest != entry.Digest || record.Bytes != entry.Bytes {
		return ExperimentCompletionDocument{}, core.ErrPrimitiveContract
	}
	if err := equalProbeIdentity(document.Payload.Probe, entry.Probe); err != nil {
		return ExperimentCompletionDocument{}, core.ErrPrimitiveContract
	}
	return document, nil
}

func validateDeliveryPageCoverage(manifest ExperimentDeliveryManifest, pages []ExperimentDeliveryPage) error {
	for pageIndex := range pages {
		expected := 0
		for _, entry := range manifest.Entries {
			if entry.Page == pages[pageIndex].Page {
				expected++
			}
		}
		if expected != len(pages[pageIndex].Documents) {
			return core.ErrPrimitiveContract
		}
	}
	return nil
}

func (c deliveryClosure) verify() error {
	switch c.body.Kind {
	case EvidenceCompletedRunner:
		if err := VerifyRunnerCompletion(*c.body.Completed, c.runnerKeys); err != nil {
			return err
		}
		return verifyCompletedManifestClosure(*c.body.Completed, c.manifest)
	case EvidenceInterruptedRunner:
		return verifyInterruptedEvidenceClosure(c.body, c.documents)
	case EvidencePreRunnerInfrastructure:
		if len(c.manifest.Entries) != 0 || len(c.documents) != 0 {
			return core.ErrPrimitiveContract
		}
		return nil
	default:
		return core.ErrPrimitiveContract
	}
}

func verifyInterruptedEvidenceClosure(body ObservationEvidenceBody, documents []ExperimentCompletionDocument) error {
	if len(body.Interrupted.Experiments) != len(documents) {
		return core.ErrPrimitiveContract
	}
	for index := range documents {
		left, leftErr := documents[index].MarshalJSON()
		right, rightErr := body.Interrupted.Experiments[index].MarshalJSON()
		if leftErr != nil || rightErr != nil {
			return errors.Join(core.ErrPrimitiveContract, leftErr, rightErr)
		}
		if !bytes.Equal(left, right) {
			return core.ErrPrimitiveContract
		}
	}
	return nil
}

func verifyCompletedManifestClosure(document RunnerCompletionDocument, delivery ExperimentDeliveryManifest) error {
	var runnerManifest ExperimentManifest
	if document.Payload.Selection != nil && document.Payload.Selection.ExperimentFacts != nil {
		runnerManifest = *document.Payload.Selection.ExperimentFacts
	}
	if document.Payload.DirectExperiment != nil {
		runnerManifest = ExperimentManifest{Entries: []ExperimentManifestEntry{document.Payload.DirectExperiment.Experiment}}
	}
	if len(runnerManifest.Entries) != len(delivery.Entries) {
		return core.ErrPrimitiveContract
	}
	for index := range runnerManifest.Entries {
		left := runnerManifest.Entries[index]
		right := delivery.Entries[index]
		if left.Experiment != right.Experiment || left.CompletionDigest != right.Digest || left.CompletionBytes != right.Bytes || equalProbeIdentity(left.Probe, right.Probe) != nil {
			return core.ErrPrimitiveContract
		}
	}
	return nil
}

var (
	_ core.Validatable            = CleanupOutcome{}
	_ json.Unmarshaler            = (*CleanupOutcomeKind)(nil)
	_ core.Validatable            = InterruptedRunnerEvidence{}
	_ core.Validatable            = PreRunnerEvidence{}
	_ core.Validatable            = ObservationEvidenceBody{}
	_ json.Unmarshaler            = (*EvidenceBodyKind)(nil)
	_ core.ValidatedJSONMarshaler = ObservationEnvelopePayload{}
	_ json.Unmarshaler            = (*ObservationEnvelopePayload)(nil)
	_ core.ValidatedJSONMarshaler = ObservationEnvelope{}
	_ json.Unmarshaler            = (*ObservationEnvelope)(nil)
	_ core.Validatable            = ExperimentDeliveryEntry{}
	_ core.Validatable            = ExperimentDeliveryManifest{}
	_ core.ValidatedJSONMarshaler = ExperimentDeliveryPage{}
	_ json.Unmarshaler            = (*ExperimentDeliveryPage)(nil)
)
