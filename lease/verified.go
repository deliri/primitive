package lease

import (
	"errors"

	"github.com/deliri/primitive/v2026/attest"
)

// VerifyRequest authenticates and subject-binds one untrusted document.
type VerifyRequest struct {
	ExpectedSubject Subject
	Document        Document
	TrustedKeys     attest.TrustedKeys
}

// Validate proves request structure without claiming signature trust.
func (r VerifyRequest) Validate() error {
	if err := r.Document.Validate(); err != nil {
		return contractError(err)
	}
	if err := r.TrustedKeys.Validate(); err != nil {
		return contractError(err)
	}
	if err := r.ExpectedSubject.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// Verified is an authentic decision bound to the caller's expected subject.
type Verified struct {
	subject  Subject
	decision Decision
	proof    attest.Verified[Domain]
	verified bool
}

// Verify authenticates one signed decision and binds its exact subject.
func Verify(request VerifyRequest) (Verified, error) {
	if err := request.Validate(); err != nil {
		return Verified{}, err
	}
	proof, err := attest.Verify(attest.VerifyRequest[Domain]{
		Body: request.Document.Decision, Envelope: request.Document.Attestation,
		TrustedKeys: request.TrustedKeys,
	})
	if err != nil {
		return Verified{}, verificationError(err)
	}
	header, err := request.Document.Decision.Header()
	if err != nil {
		return Verified{}, verificationError(err)
	}
	if header.Subject != request.ExpectedSubject {
		return Verified{}, verificationError(newScopeMismatch(
			request.ExpectedSubject,
			header.Subject,
		))
	}
	result := Verified{
		decision: request.Document.Decision, proof: proof,
		subject: request.ExpectedSubject, verified: true,
	}
	return result, result.Validate()
}

// Validate rejects zero or internally contradictory proof carriers.
func (v Verified) Validate() error {
	if !v.verified {
		return verificationError(errors.New("verified lease is unset"))
	}
	if err := v.decision.Validate(); err != nil {
		return verificationError(err)
	}
	if err := v.proof.Validate(); err != nil {
		return verificationError(err)
	}
	if err := v.subject.Validate(); err != nil {
		return verificationError(err)
	}
	header, err := v.decision.Header()
	if err != nil {
		return verificationError(err)
	}
	if header.Subject != v.subject {
		return verificationError(newScopeMismatch(v.subject, header.Subject))
	}
	envelope, err := v.proof.Envelope()
	if err != nil {
		return verificationError(err)
	}
	if envelope.Domain != v.decision.AttestationDomain() {
		return verificationError(errors.New("verified lease domain changed"))
	}
	return nil
}

// Decision returns an authentic value copy.
func (v Verified) Decision() (Decision, error) {
	if err := v.Validate(); err != nil {
		return Decision{}, err
	}
	return v.decision, nil
}

// Envelope returns the authentic detached envelope.
func (v Verified) Envelope() (attest.Envelope[Domain], error) {
	if err := v.Validate(); err != nil {
		return attest.Envelope[Domain]{}, err
	}
	return v.proof.Envelope()
}

// Subject returns the subject closed by Verify.
func (v Verified) Subject() (Subject, error) {
	if err := v.Validate(); err != nil {
		return Subject{}, err
	}
	return v.subject, nil
}
