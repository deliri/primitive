// Package accesspermit authenticates one scoped, time-bounded authority answer.
// It knows neither the meaning of the offering nor what an admitted caller does.
// The authority supplies the decision and interval; the consumer owns policy.
package accesspermit

import (
	"crypto"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
	"github.com/deliri/primitive/v2026/receipt"
	"github.com/deliri/primitive/v2026/temporal"
)

// DocumentMaximumBytes bounds one signed access agreement, not an evidence stream.
const DocumentMaximumBytes = 8192

// Decision is explicit: an unset or future value never means refusal or grant.
type Decision uint8

const (
	DecisionUnknown Decision = iota
	DecisionAllow
	DecisionRefuse
)

const (
	DecisionAllowToken  = "allow"
	DecisionRefuseToken = "refuse"
)

func (d Decision) IsValid() bool { return d.Validate() == nil }

func (d Decision) String() string {
	if d == DecisionAllow {
		return DecisionAllowToken
	}
	if d == DecisionRefuse {
		return DecisionRefuseToken
	}
	return core.UnknownEnumDiagnostic
}

func (d Decision) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return core.MarshalCanonicalJSONString(d.String())
}

func (d *Decision) UnmarshalJSON(data []byte) error {
	if d == nil {
		return errors.Join(core.ErrAccessPermitContract, core.ErrJSONContract)
	}
	text, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(core.ErrAccessPermitContract, err)
	}
	var got Decision
	switch text {
	case DecisionAllowToken:
		got = DecisionAllow
	case DecisionRefuseToken:
		got = DecisionRefuse
	default:
		return errors.Join(core.ErrAccessPermitContract, core.ErrJSONContract)
	}
	*d = got
	return nil
}

func (d Decision) Validate() error {
	switch d {
	case DecisionAllow, DecisionRefuse:
		return nil
	default:
		return core.ErrAccessPermitContract
	}
}

// Binding pins both the authority namespace and the originating exchange.
type Binding struct {
	Family       controlwire.RouteFamily   `json:"family"`
	Subject      lease.Subject             `json:"subject"`
	Account      receipt.PrincipalIdentity `json:"account"`
	RequestNonce controlwire.RequestNonce  `json:"request_nonce"`
	Generation   lease.Generation          `json:"generation"`
}

func (b Binding) Validate() error {
	if b.Family != controlwire.RouteFamilyRegistrations && b.Family != controlwire.RouteFamilyCheckIns {
		return core.ErrAccessPermitContract
	}
	return errors.Join(b.Subject.Validate(), b.Account.Validate(), b.RequestNonce.Validate(), b.Generation.Validate())
}

// Window is independent of a lease's continuity interval. It is [start,end).
// Refusal retains exact timing for subsequent contact but never admits access.
type Window struct {
	NotBefore    temporal.Instant `json:"not_before"`
	RefreshAfter temporal.Instant `json:"refresh_after"`
	NotAfter     temporal.Instant `json:"not_after"`
	Decision     Decision         `json:"decision"`
}

func (w Window) Validate() error {
	if err := errors.Join(w.NotBefore.Validate(), w.RefreshAfter.Validate(), w.NotAfter.Validate(), w.Decision.Validate()); err != nil {
		return errors.Join(core.ErrAccessPermitContract, err)
	}
	start, _ := w.NotBefore.Nanoseconds()
	refresh, _ := w.RefreshAfter.Nanoseconds()
	end, _ := w.NotAfter.Nanoseconds()
	if start >= end || refresh < start || refresh > end {
		return core.ErrAccessPermitContract
	}
	return nil
}

type Terms struct {
	Binding  Binding `json:"binding"`
	Window   Window  `json:"window"`
	Revision uint16  `json:"revision"`
}

const RevisionV1 uint16 = 1

func (t Terms) Validate() error {
	if t.Revision != RevisionV1 {
		return core.ErrAccessPermitContract
	}
	if err := errors.Join(t.Binding.Validate(), t.Window.Validate()); err != nil {
		return errors.Join(core.ErrAccessPermitContract, err)
	}
	return nil
}

func (Terms) AttestationDomain() Domain { return DomainV1 }

func (t Terms) WriteCanonical(w io.Writer) error {
	if err := t.Validate(); err != nil {
		return err
	}
	data, err := core.MarshalCanonicalJSONDocument(t)
	if err != nil {
		return err
	}
	return write(w, data)
}

type Document struct {
	Terms     Terms                   `json:"terms"`
	Signature attest.Envelope[Domain] `json:"signature"`
}

func (d Document) Validate() error {
	if err := errors.Join(d.Terms.Validate(), d.Signature.Validate()); err != nil {
		return errors.Join(core.ErrAccessPermitContract, err)
	}
	if d.Signature.Domain != DomainV1 {
		return core.ErrAccessPermitContract
	}
	return nil
}

func limits() core.StrictJSONLimits {
	result := core.DefaultStrictJSONLimits()
	result.DocumentMaximumBytes, _ = core.NewByteCount(DocumentMaximumBytes)
	return result
}

func (d Document) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	type wire Document
	data, err := core.MarshalCanonicalJSONDocument(wire(d))
	if err != nil || len(data) > DocumentMaximumBytes {
		return nil, errors.Join(core.ErrAccessPermitContract, err)
	}
	return data, nil
}

func (d *Document) UnmarshalJSON(data []byte) error {
	if d == nil {
		return errors.Join(core.ErrAccessPermitContract, core.ErrJSONContract)
	}
	type wire Document
	got, err := core.DecodeStrictJSONStructure[wire](data, limits())
	if err != nil {
		return errors.Join(core.ErrAccessPermitContract, err)
	}
	candidate := Document(got)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrAccessPermitContract, core.ErrJSONContract, err)
	}
	*d = candidate
	return nil
}

func Decode(source io.Reader) (Document, error) {
	got, err := core.DecodeStrictJSON[Document](source, limits())
	if err != nil {
		return Document{}, errors.Join(core.ErrAccessPermitContract, err)
	}
	return got, nil
}

func Issue(terms Terms, signer crypto.Signer) (Document, error) {
	if err := terms.Validate(); err != nil {
		return Document{}, err
	}
	envelope, err := attest.Sign(attest.SignRequest[Domain]{Body: terms, Signer: signer})
	if err != nil {
		return Document{}, errors.Join(core.ErrAccessPermitContract, err)
	}
	document := Document{Terms: terms, Signature: envelope}
	return document, document.Validate()
}

// Verified can only be obtained by authenticating against independently pinned
// binding facts. Structural validity alone cannot create this proof.
type Verified struct {
	terms Terms
	proof attest.Verified[Domain]
}

func Verify(document Document, expected Binding, keys attest.TrustedKeys) (Verified, error) {
	if err := errors.Join(document.Validate(), expected.Validate()); err != nil {
		return Verified{}, errors.Join(core.ErrAccessPermitContract, err)
	}
	if document.Terms.Binding != expected {
		return Verified{}, core.ErrAccessPermitBinding
	}
	proof, err := attest.Verify(attest.VerifyRequest[Domain]{Body: document.Terms, Envelope: document.Signature, TrustedKeys: keys})
	if err != nil {
		return Verified{}, errors.Join(core.ErrAccessPermitDenied, err)
	}
	return Verified{terms: document.Terms, proof: proof}, nil
}

func (v Verified) Validate() error {
	if err := errors.Join(v.terms.Validate(), v.proof.Validate()); err != nil {
		return errors.Join(core.ErrAccessPermitDenied, err)
	}
	return nil
}

func (v Verified) Terms() (Terms, error) {
	if err := v.Validate(); err != nil {
		return Terms{}, err
	}
	return v.terms, nil
}

func (v Verified) Allows(now temporal.Instant) error {
	if err := errors.Join(v.Validate(), now.Validate()); err != nil {
		return errors.Join(core.ErrAccessPermitDenied, err)
	}
	at, _ := now.Nanoseconds()
	start, _ := v.terms.Window.NotBefore.Nanoseconds()
	end, _ := v.terms.Window.NotAfter.Nanoseconds()
	if v.terms.Window.Decision != DecisionAllow || at < start || at >= end {
		return core.ErrAccessPermitDenied
	}
	return nil
}

func write(w io.Writer, data []byte) error {
	if w == nil {
		return core.ErrAccessPermitContract
	}
	n, err := w.Write(data)
	if err != nil {
		return errors.Join(core.ErrAccessPermitContract, err)
	}
	if n != len(data) {
		return errors.Join(core.ErrAccessPermitContract, io.ErrShortWrite)
	}
	return nil
}

var (
	_ core.ValidatedJSONMarshaler = Decision(0)
	_ core.ValidatedJSONMarshaler = Document{}
)
