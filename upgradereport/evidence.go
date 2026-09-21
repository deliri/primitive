package upgradereport

import (
	"crypto"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/submission"
	"github.com/deliri/primitive/v2026/submissionauth"
)

// EvidencePayload binds one attempt identity to the commitment of an exact
// device-signed submission. Eligibility and retention remain authority policy.
type EvidencePayload struct {
	Attempt    controlwire.RequestNonce     `json:"attempt"`
	Submission submission.RequestCommitment `json:"submission"`
}

func (p EvidencePayload) Validate() error {
	if err := errors.Join(p.Attempt.Validate(), p.Submission.Validate()); err != nil {
		return errors.Join(core.ErrReportContract, err)
	}
	return nil
}

func (EvidencePayload) AttestationDomain() Domain { return DomainEvidence }

func (p EvidencePayload) WriteCanonical(w io.Writer) error {
	if err := p.Validate(); err != nil {
		return err
	}
	b, err := core.MarshalCanonicalJSONDocument(p)
	if err != nil {
		return err
	}
	return writeCanonical(w, b)
}

// EvidenceRequest carries the existing Submission agreement and an authenticated
// binding to its owning attempt. No capability or bearer is stored in this request.
type EvidenceRequest struct {
	Submission  submissionauth.RequestDocument `json:"submission"`
	Attestation attest.Envelope[Domain]        `json:"attestation"`
	Payload     EvidencePayload                `json:"payload"`
}

func (r EvidenceRequest) Validate() error {
	if err := errors.Join(r.Payload.Validate(), r.Submission.Validate(), r.Attestation.Validate()); err != nil {
		return errors.Join(core.ErrReportContract, err)
	}
	commitment, err := submission.CommitRequest(r.Submission.Request.Payload)
	if err != nil {
		return errors.Join(core.ErrReportContract, err)
	}
	if r.Payload.Submission != commitment || r.Attestation.Domain != DomainEvidence || r.Attestation.Signer != r.Submission.Certificate.Body.DeviceKey {
		return core.ErrReportBinding
	}
	return nil
}

func SignEvidence(attempt controlwire.RequestNonce, request submissionauth.RequestDocument, signer crypto.Signer) (EvidenceRequest, error) {
	if err := request.Validate(); err != nil {
		return EvidenceRequest{}, err
	}
	commitment, err := submission.CommitRequest(request.Request.Payload)
	if err != nil {
		return EvidenceRequest{}, err
	}
	payload := EvidencePayload{Attempt: attempt, Submission: commitment}
	signature, err := attest.Sign(attest.SignRequest[Domain]{Body: payload, Signer: signer})
	if err != nil {
		return EvidenceRequest{}, errors.Join(core.ErrReportAuthentication, err)
	}
	r := EvidenceRequest{Payload: payload, Submission: request, Attestation: signature}
	if err := r.Validate(); err != nil {
		return EvidenceRequest{}, err
	}
	return r, nil
}

func (r EvidenceRequest) Verify(authority controlplane.Authority) error {
	if err := r.Validate(); err != nil {
		return err
	}
	if _, err := submissionauth.Verify(submissionauth.Verification{Document: r.Submission, Server: authority}); err != nil {
		return errors.Join(core.ErrReportAuthentication, err)
	}
	certificate, err := authority.VerifyInstallationCertificate(r.Submission.Certificate)
	if err != nil {
		return errors.Join(core.ErrReportAuthentication, err)
	}
	keys, err := certificate.DeviceKeys()
	if err != nil {
		return errors.Join(core.ErrReportAuthentication, err)
	}
	if _, err := attest.Verify(attest.VerifyRequest[Domain]{Body: r.Payload, Envelope: r.Attestation, TrustedKeys: keys}); err != nil {
		return errors.Join(core.ErrReportAuthentication, err)
	}
	return nil
}

func (r EvidenceRequest) ControlRoute() (controlwire.RouteContract, error) {
	if err := r.Validate(); err != nil {
		return controlwire.RouteContract{}, err
	}
	build := r.Submission.Request.Payload.Build
	return controlwire.NewRouteContract(build.Offering(), controlwire.RouteFamilyUpgradeEvidence)
}
func (r EvidenceRequest) ControlRevision() controlwire.Revision {
	return r.Submission.Request.Payload.Revision
}
func (r EvidenceRequest) ControlNonce() controlwire.RequestNonce { return r.Payload.Attempt }
func (r EvidenceRequest) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	type wire EvidenceRequest
	return core.MarshalCanonicalJSONDocument(wire(r))
}
func (r *EvidenceRequest) UnmarshalJSON(b []byte) error {
	if r == nil {
		return core.ErrReportContract
	}
	type wire EvidenceRequest
	v, err := core.DecodeStrictJSONStructure[wire](b, core.ExtensibleJSONLimits())
	if err != nil {
		return errors.Join(core.ErrReportContract, err)
	}
	got := EvidenceRequest(v)
	if err := got.Validate(); err != nil {
		return err
	}
	*r = got
	return nil
}
