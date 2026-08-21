package chit

import (
	"crypto"
	json "encoding/json/v2"
	"errors"
	"io"
	"strings"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/receipt"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	custodyStateTokenStored               = "stored"
	custodyStateTokenRetrievalUnavailable = "retrieval-unavailable"
	custodyStateTokenDeleted              = "deleted"
	// CatalogCursorCommitmentDomain separates catalog positions from every
	// other SHA-256 use in Primitive.
	CatalogCursorCommitmentDomain = "primitive.chit.catalog-cursor.v1"
	// CatalogCursorFrameSeparator makes the domain/identity frame injective.
	CatalogCursorFrameSeparator byte = 0
)

// CustodyState is the authority-observed availability of one immutable chit.
type CustodyState uint8

const (
	CustodyStateUnknown CustodyState = iota
	CustodyStateStored
	CustodyStateRetrievalUnavailable
	CustodyStateDeleted
	custodyStateLimit
)

func custodyStateTokens() [custodyStateLimit]string {
	return [...]string{
		"",
		custodyStateTokenStored,
		custodyStateTokenRetrievalUnavailable,
		custodyStateTokenDeleted,
	}
}

func (s CustodyState) Validate() error {
	if s <= CustodyStateUnknown || s >= custodyStateLimit || custodyStateTokens()[s] == "" {
		return contractError(errors.New("custody state is invalid"))
	}
	return nil
}

// IsValid reports whether s is one published custody state.
func (s CustodyState) IsValid() bool { return s.Validate() == nil }

func (s CustodyState) String() string {
	if s >= custodyStateLimit {
		return ""
	}
	return custodyStateTokens()[s]
}

func (s CustodyState) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return core.MarshalCanonicalJSONString(s.String())
}

func (s *CustodyState) UnmarshalJSON(data []byte) error {
	if s == nil {
		return jsonError(errors.New("nil custody state receiver"))
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return jsonError(err)
	}
	for candidate := CustodyStateUnknown + 1; candidate < custodyStateLimit; candidate++ {
		if candidate.String() == value {
			*s = candidate
			return nil
		}
	}
	return jsonError(errors.New("custody state text is unsupported"))
}

// CatalogEntry binds one immutable chit to its current availability.
type CatalogEntry struct {
	Chit  Document     `json:"chit"`
	State CustodyState `json:"state"`
}

func (e CatalogEntry) ValidateAt(observed temporal.Instant) error {
	if err := errors.Join(e.Validate(), observed.Validate()); err != nil {
		return contractError(err)
	}
	if e.State == CustodyStateStored {
		return nil
	}
	order, err := observed.Compare(e.Chit.Payload.RetainUntil)
	if err != nil || order == core.ComparisonLess {
		return conflictError(errors.New("custody became unavailable before its retention promise"), err)
	}
	return nil
}

// Validate closes the signed chit and current custody-state structure without
// inventing an observation instant for the temporal retention decision.
func (e CatalogEntry) Validate() error {
	if err := errors.Join(e.Chit.Validate(), e.State.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

// Cursor is the opaque closure of the last entry represented by a page.
type Cursor struct{ value core.SHA256Digest }

func newCursor(value core.SHA256Digest) (Cursor, error) {
	candidate := Cursor{value: value}
	if err := candidate.Validate(); err != nil {
		return Cursor{}, err
	}
	return candidate, nil
}

// CursorFor closes one exact Chit identity into the opaque position used after
// that entry. The identity is the catalog ordering key; custody-state changes
// therefore do not invalidate a customer's position.
func CursorFor(identity ChitID) (Cursor, error) {
	if err := identity.Validate(); err != nil {
		return Cursor{}, contractError(err)
	}
	writer := core.NewDigestWriter()
	if _, err := writer.Write([]byte(CatalogCursorCommitmentDomain)); err != nil {
		return Cursor{}, contractError(err)
	}
	if _, err := writer.Write([]byte{CatalogCursorFrameSeparator}); err != nil {
		return Cursor{}, contractError(err)
	}
	if _, err := writer.Write([]byte(identity.String())); err != nil {
		return Cursor{}, contractError(err)
	}
	digest, _, err := writer.Seal()
	if err != nil {
		return Cursor{}, contractError(err)
	}
	return newCursor(digest)
}

func (c Cursor) Validate() error {
	if err := c.value.Validate(); err != nil {
		return contractError(errors.New("catalog cursor is invalid"), err)
	}
	return nil
}

func (c Cursor) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return c.value.MarshalJSON()
}

func (c *Cursor) UnmarshalJSON(data []byte) error {
	if c == nil {
		return jsonError(errors.New("nil catalog cursor receiver"))
	}
	var value core.SHA256Digest
	if err := json.Unmarshal(data, &value); err != nil {
		return jsonError(err)
	}
	candidate, err := newCursor(value)
	if err != nil {
		return jsonError(err)
	}
	*c = candidate
	return nil
}

// Continuation is a tagged union: End carries no cursor; More requires one.
type Continuation struct {
	Cursor Cursor                        `json:"cursor"`
	State  core.CatalogContinuationState `json:"state"`
}

func End() Continuation { return Continuation{State: core.CatalogContinuationEnd} }

func More(cursor Cursor) (Continuation, error) {
	candidate := Continuation{State: core.CatalogContinuationMore, Cursor: cursor}
	return candidate, candidate.Validate()
}

func (c Continuation) Validate() error {
	if err := c.State.Validate(); err != nil {
		return contractError(err)
	}
	switch c.State {
	case core.CatalogContinuationEnd:
		if c.Cursor != (Cursor{}) {
			return contractError(errors.New("end continuation carries a cursor"))
		}
	case core.CatalogContinuationMore:
		if err := c.Cursor.Validate(); err != nil {
			return contractError(err)
		}
	default:
		return contractError(errors.New("continuation state escaped its domain"))
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

// CatalogPayload is one bounded authority-observed page. Entries are newest
// first by UUIDv7 text, making pagination deterministic across large histories.
type CatalogPayload struct {
	Scope        receipt.Scope     `json:"scope"`
	Entries      []CatalogEntry    `json:"entries"`
	Watermark    receipt.Watermark `json:"watermark"`
	ObservedAt   temporal.Instant  `json:"observed_at"`
	Continuation Continuation      `json:"continuation"`
	Request      QueryCommitment   `json:"query_commitment"`
}

func (p CatalogPayload) Validate() error {
	if err := errors.Join(
		p.Scope.Validate(), p.Request.Validate(), p.Watermark.Validate(),
		p.ObservedAt.Validate(), p.Continuation.Validate(),
	); err != nil {
		return contractError(err)
	}
	if p.Watermark.Scope != p.Scope {
		return conflictError(errors.New("catalog watermark scope differs"))
	}
	if p.Entries == nil || len(p.Entries) > core.CatalogPageMaximumEntries {
		return contractError(errors.New("catalog entry count is outside its bound"))
	}
	return validateCatalogEntries(p)
}

func validateCatalogEntries(payload CatalogPayload) error {
	prior := ""
	for _, entry := range payload.Entries {
		if err := entry.ValidateAt(payload.ObservedAt); err != nil {
			return err
		}
		if entry.Chit.Payload.Scope != payload.Scope {
			return conflictError(errors.New("catalog entry scope differs"))
		}
		current := entry.Chit.Payload.Identity.String()
		if prior != "" && strings.Compare(prior, current) <= 0 {
			return conflictError(errors.New("catalog entries are not strictly newest first"))
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
		return conflictError(errors.New("empty catalog page claims continuation"))
	}
	last := payload.Entries[len(payload.Entries)-1]
	cursor, err := CursorFor(last.Chit.Payload.Identity)
	if err != nil {
		return err
	}
	if cursor != payload.Continuation.Cursor {
		return conflictError(errors.New("catalog continuation does not close the page tail"))
	}
	return nil
}

func (CatalogPayload) AttestationDomain() SigningDomain { return SigningDomainCatalogV1 }

func (p CatalogPayload) WriteCanonical(destination io.Writer) error {
	if destination == nil {
		return contractError(errors.New("catalog canonical destination is nil"))
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

func (p *CatalogPayload) UnmarshalJSON(data []byte) error {
	if p == nil {
		return jsonError(errors.New("nil catalog payload receiver"))
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

type CatalogDocument struct {
	Payload     CatalogPayload                 `json:"payload"`
	Attestation attest.Envelope[SigningDomain] `json:"attestation"`
}

func (d CatalogDocument) Validate() error {
	if err := errors.Join(d.Payload.Validate(), d.Attestation.Validate()); err != nil {
		return contractError(err)
	}
	if d.Attestation.Domain != SigningDomainCatalogV1 {
		return verificationError(errors.New("catalog signing domain differs"))
	}
	return nil
}

type CatalogIssuance struct {
	Signer  crypto.Signer
	Payload CatalogPayload
}

func (i CatalogIssuance) Validate() error {
	if err := (attest.SignRequest[SigningDomain]{Body: i.Payload, Signer: i.Signer}).Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

func IssueCatalog(issuance CatalogIssuance) (CatalogDocument, error) {
	if err := issuance.Validate(); err != nil {
		return CatalogDocument{}, err
	}
	envelope, err := attest.Sign(attest.SignRequest[SigningDomain]{Body: issuance.Payload, Signer: issuance.Signer})
	if err != nil {
		return CatalogDocument{}, contractError(err)
	}
	document := CatalogDocument{Payload: issuance.Payload, Attestation: envelope}
	return document, document.Validate()
}

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

func (d *CatalogDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("nil catalog document receiver"))
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

type CatalogVerification struct {
	Request     QueryPayload
	Document    CatalogDocument
	TrustedKeys attest.TrustedKeys
}

func (v CatalogVerification) Validate() error {
	if err := errors.Join(v.Document.Validate(), v.Request.Validate(), v.TrustedKeys.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

func VerifyCatalog(verification CatalogVerification) (CatalogPayload, error) {
	if err := verification.Validate(); err != nil {
		return CatalogPayload{}, err
	}
	if _, err := attest.Verify(attest.VerifyRequest[SigningDomain]{
		Body: verification.Document.Payload, Envelope: verification.Document.Attestation,
		TrustedKeys: verification.TrustedKeys,
	}); err != nil {
		return CatalogPayload{}, verificationError(err)
	}
	commitment, err := CommitQuery(verification.Request)
	if err != nil {
		return CatalogPayload{}, contractError(err)
	}
	payload := verification.Document.Payload
	if payload.Scope != verification.Request.Query.Scope || payload.Request != commitment {
		return CatalogPayload{}, conflictError(errors.New("catalog scope differs from expectation"))
	}
	if len(payload.Entries) > int(verification.Request.Query.Limit.Uint16()) {
		return CatalogPayload{}, conflictError(errors.New("catalog page exceeds the requested limit"))
	}
	if err := validateCatalogSelection(payload, verification.Request.Query.Selection); err != nil {
		return CatalogPayload{}, err
	}
	return payload, nil
}

func validateCatalogSelection(payload CatalogPayload, selection Selection) error {
	if selection.Kind == core.CatalogSelectionAll {
		return nil
	}
	if payload.Continuation.State != core.CatalogContinuationEnd || len(payload.Entries) > 1 {
		return conflictError(errors.New("specific catalog response has an invalid extent"))
	}
	if len(payload.Entries) == 1 && payload.Entries[0].Chit.Payload.Identity != selection.Chit {
		return conflictError(errors.New("specific catalog response carries another chit"))
	}
	return nil
}

var (
	_ core.Validatable                    = CustodyStateUnknown
	_ core.Validatable                    = CatalogEntry{}
	_ core.Validatable                    = Cursor{}
	_ core.Validatable                    = Continuation{}
	_ core.Validatable                    = CatalogPayload{}
	_ core.Validatable                    = CatalogDocument{}
	_ core.Validatable                    = CatalogIssuance{}
	_ core.Validatable                    = CatalogVerification{}
	_ core.ValidatedJSONMarshaler         = CustodyState(0)
	_ core.ValidatedJSONMarshaler         = Cursor{}
	_ core.ValidatedJSONMarshaler         = Continuation{}
	_ core.ValidatedJSONMarshaler         = CatalogPayload{}
	_ core.ValidatedJSONMarshaler         = CatalogDocument{}
	_ attest.CanonicalBody[SigningDomain] = CatalogPayload{}
)
