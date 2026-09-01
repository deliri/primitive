package payment

import (
	"crypto"
	"errors"
	"io"
	"strings"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/receipt"
	"github.com/deliri/primitive/v2026/temporal"
)

// Continuation is a tagged union: End carries no cursor; More requires one.
type Continuation struct {
	Cursor Cursor                        `json:"cursor"`
	State  core.CatalogContinuationState `json:"state"`
}

// End states that the authenticated catalog has no later page.
func End() Continuation { return Continuation{State: core.CatalogContinuationEnd} }

// More binds another-page state to one opaque authority cursor.
func More(cursor Cursor) (Continuation, error) {
	candidate := Continuation{Cursor: cursor, State: core.CatalogContinuationMore}
	return candidate, candidate.Validate()
}

// Validate enforces the exact tagged-union arm.
func (c Continuation) Validate() error {
	if err := c.State.Validate(); err != nil {
		return contractError(err)
	}
	switch c.State {
	case core.CatalogContinuationEnd:
		if c.Cursor != (Cursor{}) {
			return contractError(errors.New("payment end continuation carries a cursor"))
		}
	case core.CatalogContinuationMore:
		if err := c.Cursor.Validate(); err != nil {
			return contractError(err)
		}
	default:
		return contractError(errors.New("payment continuation escaped its domain"))
	}
	return nil
}

// MarshalJSON emits only the member owned by the selected tagged-union arm.
func (c Continuation) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, jsonError(err)
	}
	if c.State == core.CatalogContinuationEnd {
		return core.MarshalCanonicalJSONDocument(struct {
			State core.CatalogContinuationState `json:"state"`
		}{State: c.State})
	}
	return core.MarshalCanonicalJSONDocument(struct {
		Cursor Cursor                        `json:"cursor"`
		State  core.CatalogContinuationState `json:"state"`
	}{Cursor: c.Cursor, State: c.State})
}

// CatalogPayload is one bounded, newest-first authority-observed receipt page.
type CatalogPayload struct {
	Scope        receipt.Scope     `json:"scope"`
	Entries      []Document        `json:"entries"`
	Watermark    receipt.Watermark `json:"watermark"`
	ObservedAt   temporal.Instant  `json:"observed_at"`
	Continuation Continuation      `json:"continuation"`
	Request      QueryCommitment   `json:"query_commitment"`
}

// Validate closes page bounds, scope, monotonic watermark, ordering, and every
// embedded signed receipt structure.
func (p CatalogPayload) Validate() error {
	if err := errors.Join(
		p.Scope.Validate(), p.Request.Validate(), p.Watermark.Validate(), p.ObservedAt.Validate(),
		p.Continuation.Validate(),
	); err != nil {
		return contractError(err)
	}
	if p.Watermark.Scope != p.Scope {
		return verificationError(errors.New("payment catalog watermark scope differs"))
	}
	if p.Entries == nil || len(p.Entries) > core.CatalogPageMaximumEntries {
		return contractError(errors.New("payment catalog entry count is outside its bound"))
	}
	return validateCatalogEntries(p)
}

func validateCatalogEntries(payload CatalogPayload) error {
	prior := ""
	for _, entry := range payload.Entries {
		if err := entry.Validate(); err != nil {
			return contractError(err)
		}
		if entry.Payload.Scope != payload.Scope {
			return verificationError(errors.New("payment catalog entry scope differs"))
		}
		current := entry.Payload.Identity.String()
		if prior != "" && strings.Compare(prior, current) <= 0 {
			return verificationError(errors.New("payment catalog entries are not strictly newest first"))
		}
		prior = current
	}
	return validateCatalogContinuation(payload)
}

func validateCatalogContinuation(payload CatalogPayload) error {
	if payload.Continuation.State != core.CatalogContinuationMore {
		return nil
	}
	if len(payload.Entries) == 0 {
		return verificationError(errors.New("empty payment catalog claims continuation"))
	}
	last := payload.Entries[len(payload.Entries)-1]
	cursor, err := CursorFor(last.Payload.Identity)
	if err != nil {
		return err
	}
	if cursor != payload.Continuation.Cursor {
		return verificationError(errors.New("payment catalog continuation does not close the page tail"))
	}
	return nil
}

// AttestationDomain selects the payment-catalog namespace.
func (CatalogPayload) AttestationDomain() SigningDomain { return SigningDomainCatalogV1 }

// WriteCanonical writes the exact compact signed page.
func (p CatalogPayload) WriteCanonical(destination io.Writer) error {
	if destination == nil {
		return contractError(errors.New("payment catalog destination is nil"))
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

// MarshalJSON emits one bounded canonical payment page.
func (p CatalogPayload) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	type wire CatalogPayload
	encoded, err := core.MarshalCanonicalJSONDocument(wire(p))
	if err != nil || len(encoded) > core.JSONDocumentMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

// UnmarshalJSON strictly decodes and preserves the receiver on rejection.
func (p *CatalogPayload) UnmarshalJSON(data []byte) error {
	if p == nil {
		return jsonError(errors.New("nil payment catalog payload receiver"))
	}
	type wire CatalogPayload
	decoded, err := decodeStrict[wire](data, core.JSONDocumentMaximumBytes)
	if err != nil {
		return err
	}
	candidate := CatalogPayload(decoded)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*p = candidate
	return nil
}

// CatalogDocument carries one authority-signed receipt page.
type CatalogDocument struct {
	Payload     CatalogPayload                 `json:"payload"`
	Attestation attest.Envelope[SigningDomain] `json:"attestation"`
}

// Validate closes the page, envelope, and exact signing namespace.
func (d CatalogDocument) Validate() error {
	if err := errors.Join(d.Payload.Validate(), d.Attestation.Validate()); err != nil {
		return contractError(err)
	}
	if d.Attestation.Domain != SigningDomainCatalogV1 {
		return verificationError(errors.New("payment catalog signing domain differs"))
	}
	return nil
}

// MarshalJSON emits one bounded canonical signed page.
func (d CatalogDocument) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	type wire CatalogDocument
	encoded, err := core.MarshalCanonicalJSONDocument(wire(d))
	if err != nil || len(encoded) > core.JSONDocumentMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

// UnmarshalJSON strictly decodes and preserves the receiver on rejection.
func (d *CatalogDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("nil payment catalog document receiver"))
	}
	type wire CatalogDocument
	decoded, err := decodeStrict[wire](data, core.JSONDocumentMaximumBytes)
	if err != nil {
		return err
	}
	candidate := CatalogDocument(decoded)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

// CatalogIssuance carries exact authority signing inputs for one page.
type CatalogIssuance struct {
	Signer  crypto.Signer
	Payload CatalogPayload
}

// Validate closes the page and signer without issuing.
func (i CatalogIssuance) Validate() error {
	if err := (attest.SignRequest[SigningDomain]{Body: i.Payload, Signer: i.Signer}).Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// IssueCatalog signs one exact payment catalog page.
func IssueCatalog(issuance CatalogIssuance) (CatalogDocument, error) {
	if err := issuance.Validate(); err != nil {
		return CatalogDocument{}, err
	}
	envelope, err := attest.Sign(attest.SignRequest[SigningDomain]{
		Body: issuance.Payload, Signer: issuance.Signer,
	})
	if err != nil {
		return CatalogDocument{}, contractError(err)
	}
	document := CatalogDocument{Payload: issuance.Payload, Attestation: envelope}
	return document, document.Validate()
}

// CatalogVerification carries one untrusted page, its exact signed query, and
// caller-selected authority keys.
type CatalogVerification struct {
	Request     QueryPayload
	Document    CatalogDocument
	TrustedKeys attest.TrustedKeys
}

// VerifiedCatalog is sealed proof that one exact payment page authenticated
// and matched the query that selected it.
type VerifiedCatalog struct {
	state *verifiedCatalogState
}

type verifiedCatalogState struct {
	payload CatalogPayload
	proof   attest.Verified[SigningDomain]
}

func (v VerifiedCatalog) Validate() error {
	if v.state == nil {
		return verificationError(errors.New("verified payment catalog is unset"))
	}
	return errors.Join(v.state.payload.Validate(), v.state.proof.Validate())
}

// Payload returns an ownership-isolated copy of the authenticated page.
func (v VerifiedCatalog) Payload() (CatalogPayload, error) {
	if err := v.Validate(); err != nil {
		return CatalogPayload{}, verificationError(err)
	}
	return cloneCatalogPayload(v.state.payload), nil
}

// Validate closes the complete catalog verification input.
func (v CatalogVerification) Validate() error {
	if err := errors.Join(v.Document.Validate(), v.Request.Validate(), v.TrustedKeys.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

// VerifyCatalog authenticates one exact payment page and binds it to the
// selection, cursor, limit, account, build, nonce, and revision requested.
func VerifyCatalog(verification CatalogVerification) (VerifiedCatalog, error) {
	if err := verification.Validate(); err != nil {
		return VerifiedCatalog{}, err
	}
	proof, err := attest.Verify(attest.VerifyRequest[SigningDomain]{
		Body: verification.Document.Payload, Envelope: verification.Document.Attestation,
		TrustedKeys: verification.TrustedKeys,
	})
	if err != nil {
		return VerifiedCatalog{}, verificationError(err)
	}
	commitment, err := CommitQuery(verification.Request)
	if err != nil {
		return VerifiedCatalog{}, contractError(err)
	}
	payload := cloneCatalogPayload(verification.Document.Payload)
	if payload.Scope != verification.Request.Query.Scope || payload.Request != commitment {
		return VerifiedCatalog{}, verificationError(errors.New("payment catalog scope differs from expectation"))
	}
	if len(payload.Entries) > int(verification.Request.Query.Limit.Uint16()) {
		return VerifiedCatalog{}, verificationError(errors.New("payment catalog page exceeds the requested limit"))
	}
	if err := validateCatalogSelection(payload, verification.Request.Query.Selection); err != nil {
		return VerifiedCatalog{}, err
	}
	verified := VerifiedCatalog{state: &verifiedCatalogState{payload: payload, proof: proof}}
	return verified, verified.Validate()
}

func cloneCatalogPayload(payload CatalogPayload) CatalogPayload {
	if payload.Entries == nil {
		return payload
	}
	entries := make([]Document, len(payload.Entries))
	copy(entries, payload.Entries)
	payload.Entries = entries
	return payload
}

func validateCatalogSelection(payload CatalogPayload, selection Selection) error {
	if selection.Kind == core.CatalogSelectionAll {
		return nil
	}
	if payload.Continuation.State != core.CatalogContinuationEnd || len(payload.Entries) > 1 {
		return verificationError(errors.New("specific payment catalog response has an invalid extent"))
	}
	if len(payload.Entries) == 1 && payload.Entries[0].Payload.Identity != selection.Payment {
		return verificationError(errors.New("specific payment catalog response carries another payment"))
	}
	return nil
}

var (
	_ core.Validatable                    = Continuation{}
	_ core.Validatable                    = CatalogPayload{}
	_ core.Validatable                    = CatalogDocument{}
	_ core.Validatable                    = CatalogIssuance{}
	_ core.Validatable                    = CatalogVerification{}
	_ core.ValidatedJSONMarshaler         = CatalogPayload{}
	_ core.ValidatedJSONMarshaler         = CatalogDocument{}
	_ core.ValidatedJSONMarshaler         = Continuation{}
	_ attest.CanonicalBody[SigningDomain] = CatalogPayload{}
)
