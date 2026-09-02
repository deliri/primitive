package submissionauth

import (
	"crypto/ed25519"
	"errors"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/chit"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/receipt"
	"github.com/deliri/primitive/v2026/submission"
)

// CompletionReconciliationRequest supplies one authenticated completion, the
// authority's independent observation of its exact provider generation, and
// authority-owned receipt identities and signing trust.
type CompletionReconciliationRequest struct {
	Key         ed25519.PrivateKey
	Observation objectstore.VerifiedProviderUpload
	Completion  VerifiedCompletion
	TrustedKeys attest.TrustedKeys
	Receipt     receipt.ReceiptID
	Submission  receipt.SubmissionIdentity
	Object      receipt.ObjectIdentity
}

// ReconciledCompletion is sealed proof that the authenticated request,
// provider observation, authority receipt, and chit entry all describe one
// exact object.
type ReconciledCompletion struct {
	scope       receipt.Scope
	request     submission.RequestPayload
	observation objectstore.VerifiedProviderUpload
	completion  submission.CompletionPayload
	addition    chit.ManifestAddition
	body        receipt.EvidenceBody
	receipt     receipt.ReceiptID
}

func (r CompletionReconciliationRequest) Validate() error {
	if err := errors.Join(
		r.Completion.Validate(), r.Observation.Validate(), r.TrustedKeys.Validate(),
		r.Receipt.Validate(), r.Submission.Validate(), r.Object.Validate(),
	); err != nil {
		return contractError(err)
	}
	request, completion, scope, err := completionAuthorityFacts(r.Completion)
	if err != nil {
		return err
	}
	observation, err := reconcileProviderObservation(request, completion, r.Observation)
	if err != nil {
		return err
	}
	occurredAt, err := observation.OccurredAt()
	if err != nil {
		return contractError(err)
	}
	body := reconciliationEvidenceBody(request, r.Submission, r.Object)
	if err := (receipt.IssueEvidenceRequest{
		Key: r.Key, Body: body, OccurredAt: occurredAt, Identity: r.Receipt,
		Principal: scope.Principal, Offering: scope.Offering,
	}).Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// ReconcileCompletion issues and immediately authenticates one authority
// receipt before releasing the exact ManifestAddition it may enter.
func ReconcileCompletion(request CompletionReconciliationRequest) (ReconciledCompletion, error) {
	if err := request.Validate(); err != nil {
		return ReconciledCompletion{}, err
	}
	original, completion, scope, err := completionAuthorityFacts(request.Completion)
	if err != nil {
		return ReconciledCompletion{}, err
	}
	observation, err := reconcileProviderObservation(original, completion, request.Observation)
	if err != nil {
		return ReconciledCompletion{}, err
	}
	occurredAt, err := observation.OccurredAt()
	if err != nil {
		return ReconciledCompletion{}, contractError(err)
	}
	body := reconciliationEvidenceBody(original, request.Submission, request.Object)
	document, err := receipt.IssueEvidence(receipt.IssueEvidenceRequest{
		Key: request.Key, Body: body, OccurredAt: occurredAt, Identity: request.Receipt,
		Principal: scope.Principal, Offering: scope.Offering,
	})
	if err != nil {
		return ReconciledCompletion{}, contractError(err)
	}
	evidence, err := receipt.VerifyEvidence(receipt.VerifyEvidenceRequest{
		Document: document, TrustedKeys: request.TrustedKeys,
		Expected: receipt.EvidenceExpectation{Principal: scope.Principal, Offering: scope.Offering, Body: body},
	})
	if err != nil {
		return ReconciledCompletion{}, contractError(err)
	}
	addition := chit.ManifestAddition{
		Entry: chit.ManifestEntry{
			Name: original.Manifest.Name, Sequence: original.Manifest.Sequence,
			ContentType: original.Declaration.ContentType, Evidence: document,
		},
		Evidence: evidence,
	}
	reconciled := ReconciledCompletion{
		observation: request.Observation, addition: addition,
		request: original, completion: completion, scope: scope,
		receipt: request.Receipt, body: body,
	}
	return reconciled, reconciled.Validate()
}

func completionAuthorityFacts(
	verified VerifiedCompletion,
) (submission.RequestPayload, submission.CompletionPayload, receipt.Scope, error) {
	if err := verified.Validate(); err != nil {
		return submission.RequestPayload{}, submission.CompletionPayload{}, receipt.Scope{}, err
	}
	requestDocument, err := verified.requestProof.Document()
	if err != nil {
		return submission.RequestPayload{}, submission.CompletionPayload{}, receipt.Scope{}, contractError(err)
	}
	completion, err := verified.completionProof.Payload()
	if err != nil {
		return submission.RequestPayload{}, submission.CompletionPayload{}, receipt.Scope{}, contractError(err)
	}
	certificate, err := verified.certificateProof.Body()
	if err != nil {
		return submission.RequestPayload{}, submission.CompletionPayload{}, receipt.Scope{}, contractError(err)
	}
	scope, err := certificate.Scope()
	if err != nil {
		return submission.RequestPayload{}, submission.CompletionPayload{}, receipt.Scope{}, contractError(err)
	}
	return requestDocument.Request.Payload, completion, scope, nil
}

func reconcileProviderObservation(
	request submission.RequestPayload,
	completion submission.CompletionPayload,
	observation objectstore.VerifiedProviderUpload,
) (objectstore.VerifiedProviderUpload, error) {
	if err := errors.Join(request.Validate(), completion.Validate(), observation.Validate()); err != nil {
		return objectstore.VerifiedProviderUpload{}, contractError(err)
	}
	evidence, err := observation.Evidence()
	if err != nil || evidence != completion.Evidence {
		return objectstore.VerifiedProviderUpload{}, bindingError(core.ErrObjectStoreIntegrity, err)
	}
	contentType, err := observation.ContentType()
	if err != nil {
		return objectstore.VerifiedProviderUpload{}, contractError(err)
	}
	if contentType != request.Declaration.ContentType {
		return objectstore.VerifiedProviderUpload{}, bindingError(core.ErrObjectStoreIntegrity)
	}
	return observation, nil
}

func reconciliationEvidenceBody(
	request submission.RequestPayload,
	submissionIdentity receipt.SubmissionIdentity,
	objectIdentity receipt.ObjectIdentity,
) receipt.EvidenceBody {
	return receipt.EvidenceBody{
		Submission: submissionIdentity, Object: objectIdentity,
		Extent: request.Declaration.Extent, SHA256: request.Declaration.SHA256,
		CRC32C: request.Declaration.CRC32C,
	}
}

func (r ReconciledCompletion) Validate() error {
	if err := errors.Join(
		r.observation.Validate(), r.addition.Validate(), r.request.Validate(),
		r.completion.Validate(), r.scope.Validate(), r.receipt.Validate(), r.body.Validate(),
	); err != nil {
		return contractError(err)
	}
	observation, err := reconcileProviderObservation(r.request, r.completion, r.observation)
	if err != nil {
		return err
	}
	document, err := r.addition.Evidence.Document()
	if err != nil {
		return contractError(err)
	}
	if err := r.validateManifestEntry(); err != nil {
		return err
	}
	return r.validateReceiptDocument(document, observation)
}

func (r ReconciledCompletion) validateManifestEntry() error {
	entry := r.addition.Entry
	if entry.Name != r.request.Manifest.Name || entry.Sequence != r.request.Manifest.Sequence ||
		entry.ContentType != r.request.Declaration.ContentType {
		return bindingError()
	}
	return nil
}

func (r ReconciledCompletion) validateReceiptDocument(
	document receipt.EvidenceDocument,
	observation objectstore.VerifiedProviderUpload,
) error {
	body := document.Payload.Body
	header := document.Payload.Header
	occurredAt, err := observation.OccurredAt()
	if err != nil {
		return contractError(err)
	}
	if header.Identity != r.receipt || body != r.body ||
		header.Principal != r.scope.Principal || header.Offering != r.scope.Offering ||
		header.OccurredAt != occurredAt || body.Extent != r.request.Declaration.Extent ||
		body.SHA256 != r.request.Declaration.SHA256 || body.CRC32C != r.request.Declaration.CRC32C {
		return bindingError()
	}
	return nil
}

// Manifest returns the authenticated upload grouping and position.
func (r ReconciledCompletion) Manifest() (submission.ManifestIntent, error) {
	if err := r.Validate(); err != nil {
		return submission.ManifestIntent{}, err
	}
	return r.request.Manifest, nil
}

// Addition returns the exact receipt-authenticated chit entry.
func (r ReconciledCompletion) Addition() (chit.ManifestAddition, error) {
	if err := r.Validate(); err != nil {
		return chit.ManifestAddition{}, err
	}
	return r.addition, nil
}

var (
	_ core.Validatable = CompletionReconciliationRequest{}
	_ core.Validatable = ReconciledCompletion{}
)
