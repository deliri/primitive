package payment

import (
	"crypto"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/currency"
	"github.com/deliri/primitive/v2026/receipt"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	// ReceiptPayloadJSONMaximumBytes bounds one canonical payment fact.
	ReceiptPayloadJSONMaximumBytes = 32 << 10
	// ReceiptDocumentJSONMaximumBytes bounds one signed payment receipt.
	ReceiptDocumentJSONMaximumBytes = 64 << 10
)

// ServicePeriod is the exact service interval paid for by one receipt.
type ServicePeriod struct {
	Start temporal.Instant `json:"start"`
	End   temporal.Instant `json:"end"`
}

// Validate requires two valid, strictly increasing bounds.
func (p ServicePeriod) Validate() error {
	if err := errors.Join(p.Start.Validate(), p.End.Validate()); err != nil {
		return contractError(err)
	}
	order, err := p.Start.Compare(p.End)
	if err != nil || order != core.ComparisonLess {
		return contractError(errors.New("payment service period is not strictly ordered"), err)
	}
	return nil
}

// Bounds returns the validated Temporal-owned interval projection.
func (p ServicePeriod) Bounds() (temporal.IntervalBounds, error) {
	if err := p.Validate(); err != nil {
		return temporal.IntervalBounds{}, err
	}
	return temporal.IntervalBounds{Start: p.Start, End: p.End}, nil
}

// Payload is the immutable authority statement for one settled payment.
type Payload struct {
	Scope    receipt.Scope    `json:"scope"`
	Service  ServicePeriod    `json:"service_period"`
	Amount   currency.Amount  `json:"amount"`
	PaidAt   temporal.Instant `json:"paid_at"`
	Identity PaymentID        `json:"payment_id"`
}

// Validate closes identity, tenant scope, exact positive amount, settlement
// time, and service period.
func (p Payload) Validate() error {
	if err := errors.Join(
		p.Identity.Validate(), p.Scope.Validate(), p.Amount.Validate(),
		p.PaidAt.Validate(), p.Service.Validate(),
	); err != nil {
		return contractError(err)
	}
	minorUnits, err := p.Amount.MinorUnits()
	if err != nil || minorUnits <= 0 {
		return contractError(errors.New("payment amount is not positive"), err)
	}
	return nil
}

// AttestationDomain selects the immutable payment-receipt namespace.
func (Payload) AttestationDomain() SigningDomain { return SigningDomainReceiptV1 }

// WriteCanonical writes the exact compact signed payload.
func (p Payload) WriteCanonical(destination io.Writer) error {
	if destination == nil {
		return contractError(errors.New("payment canonical destination is nil"))
	}
	encoded, err := p.MarshalJSON()
	if err != nil {
		return err
	}
	written, err := destination.Write(encoded)
	if err != nil {
		return contractError(err)
	}
	if written != len(encoded) {
		return contractError(io.ErrShortWrite)
	}
	return nil
}

// MarshalJSON emits one bounded canonical payload.
func (p Payload) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	type wire Payload
	encoded, err := core.MarshalCanonicalJSONDocument(wire(p))
	if err != nil || len(encoded) > ReceiptPayloadJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

// UnmarshalJSON strictly decodes and preserves the receiver on rejection.
func (p *Payload) UnmarshalJSON(data []byte) error {
	if p == nil {
		return jsonError(errors.New("nil payment payload receiver"))
	}
	type wire Payload
	decoded, err := decodeStrict[wire](data, ReceiptPayloadJSONMaximumBytes)
	if err != nil {
		return err
	}
	candidate := Payload(decoded)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*p = candidate
	return nil
}

// Document carries one authority-signed immutable payment receipt.
type Document struct {
	Payload     Payload                        `json:"payload"`
	Attestation attest.Envelope[SigningDomain] `json:"attestation"`
}

// Validate closes the payload, signature envelope, and exact domain binding.
func (d Document) Validate() error {
	if err := errors.Join(d.Payload.Validate(), d.Attestation.Validate()); err != nil {
		return contractError(err)
	}
	if d.Attestation.Domain != SigningDomainReceiptV1 {
		return verificationError(errors.New("payment receipt signing domain differs"))
	}
	return nil
}

// MarshalJSON emits one bounded canonical signed receipt.
func (d Document) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	type wire Document
	encoded, err := core.MarshalCanonicalJSONDocument(wire(d))
	if err != nil || len(encoded) > ReceiptDocumentJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

// UnmarshalJSON strictly decodes and preserves the receiver on rejection.
func (d *Document) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("nil payment document receiver"))
	}
	type wire Document
	decoded, err := decodeStrict[wire](data, ReceiptDocumentJSONMaximumBytes)
	if err != nil {
		return err
	}
	candidate := Document(decoded)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

// Issuance carries exact authority signing inputs.
type Issuance struct {
	Signer  crypto.Signer
	Payload Payload
}

// Validate closes the payload and signing capability without issuing.
func (i Issuance) Validate() error {
	if err := (attest.SignRequest[SigningDomain]{Body: i.Payload, Signer: i.Signer}).Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// Issue signs one exact payment receipt.
func Issue(issuance Issuance) (Document, error) {
	if err := issuance.Validate(); err != nil {
		return Document{}, err
	}
	envelope, err := attest.Sign(attest.SignRequest[SigningDomain]{
		Body: issuance.Payload, Signer: issuance.Signer,
	})
	if err != nil {
		return Document{}, contractError(err)
	}
	document := Document{Payload: issuance.Payload, Attestation: envelope}
	return document, document.Validate()
}

// Expectation prevents a valid receipt for another payment or tenant scope
// from satisfying a caller's request.
type Expectation struct {
	Scope    receipt.Scope
	Identity PaymentID
}

// Validate closes the exact expected identity and scope.
func (e Expectation) Validate() error {
	if err := errors.Join(e.Identity.Validate(), e.Scope.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

// Verification carries one untrusted receipt and caller-selected authority keys.
type Verification struct {
	Expected    Expectation
	Document    Document
	TrustedKeys attest.TrustedKeys
}

// Validate closes the complete verification input.
func (v Verification) Validate() error {
	if err := errors.Join(v.Document.Validate(), v.Expected.Validate(), v.TrustedKeys.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

// Verified is the sealed authentication result.
type Verified struct {
	document Document
	proof    attest.Verified[SigningDomain]
}

// Verify authenticates and binds one exact payment receipt.
func Verify(verification Verification) (Verified, error) {
	if err := verification.Validate(); err != nil {
		return Verified{}, err
	}
	proof, err := attest.Verify(attest.VerifyRequest[SigningDomain]{
		Body: verification.Document.Payload, Envelope: verification.Document.Attestation,
		TrustedKeys: verification.TrustedKeys,
	})
	if err != nil {
		return Verified{}, verificationError(err)
	}
	if verification.Document.Payload.Identity != verification.Expected.Identity ||
		verification.Document.Payload.Scope != verification.Expected.Scope {
		return Verified{}, verificationError(errors.New("payment receipt differs from expectation"))
	}
	verified := Verified{document: verification.Document, proof: proof}
	return verified, verified.Validate()
}

// Validate revalidates the sealed authentication proof.
func (v Verified) Validate() error {
	if err := errors.Join(v.document.Validate(), v.proof.Validate()); err != nil {
		return verificationError(err)
	}
	return nil
}

// Document returns the authenticated payment receipt.
func (v Verified) Document() (Document, error) {
	if err := v.Validate(); err != nil {
		return Document{}, err
	}
	return v.document, nil
}

func decodeStrict[T any](data []byte, maximum uint64) (T, error) {
	var zero T
	limit, err := core.NewByteCount(maximum)
	if err != nil {
		return zero, jsonError(err)
	}
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = limit
	decoded, err := core.DecodeStrictJSONStructure[T](data, limits)
	if err != nil {
		return zero, jsonError(err)
	}
	return decoded, nil
}

var (
	_ core.Validatable                    = ServicePeriod{}
	_ core.Validatable                    = Payload{}
	_ core.Validatable                    = Document{}
	_ core.Validatable                    = Issuance{}
	_ core.Validatable                    = Expectation{}
	_ core.Validatable                    = Verification{}
	_ core.Validatable                    = Verified{}
	_ core.ValidatedJSONMarshaler         = Payload{}
	_ core.ValidatedJSONMarshaler         = Document{}
	_ attest.CanonicalBody[SigningDomain] = Payload{}
)
