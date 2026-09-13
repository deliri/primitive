package permit

import (
	"crypto"
	"errors"
	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
	"github.com/deliri/primitive/v2026/temporal"
	"io"
)

const unknownText = "unknown"
const revisionV1Text = "1"

// Domain separates permit signatures from other signed agreements.
// Offering identity is part of the signed subject and build, never inferred.
type Domain uint8

const DomainV1 Domain = 1
const domainText = "primitive-action-permit-v1"

func (d Domain) Validate() error {
	if d != DomainV1 {
		return core.ErrPermitContract
	}
	return nil
}
func (Domain) OffWireEnum()    {}
func (d Domain) IsValid() bool { return d == DomainV1 }
func (d Domain) String() string {
	if !d.IsValid() {
		return unknownText
	}
	return domainText
}
func (d Domain) MarshalText() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return []byte(domainText), nil
}
func (Domain) ParseCanonicalText(text []byte) (Domain, error) {
	if string(text) != domainText {
		return 0, core.ErrPermitContract
	}
	return DomainV1, nil
}

// Terms are selected by the server. Validity is [NotBefore, ExpiresAt).
// ContactAt and RetryAfter direct transport, never extend validity.
type Terms struct {
	RequestNonce controlwire.RequestNonce `json:"request_nonce"`
	Revision     Revision                 `json:"revision"`
	Subject      lease.Subject            `json:"subject"`
	Build        core.BuildIdentity       `json:"build"`
	Generation   lease.Generation         `json:"generation"`
	Actions      Actions                  `json:"actions"`
	NotBefore    temporal.Instant         `json:"not_before"`
	ExpiresAt    temporal.Instant         `json:"expires_at"`
	ContactAt    temporal.Instant         `json:"contact_at"`
	RetryAfter   temporal.Duration        `json:"retry_after"`
}

func (t Terms) Validate() error {
	if err := t.RequestNonce.Validate(); err != nil {
		return errors.Join(core.ErrPermitContract, err)
	}
	if err := t.Revision.Validate(); err != nil {
		return errors.Join(core.ErrPermitContract, err)
	}
	if err := errors.Join(t.Subject.Validate(), t.Build.Validate(), t.Generation.Validate(), t.Actions.Validate(), t.NotBefore.Validate(), t.ExpiresAt.Validate(), t.ContactAt.Validate(), t.RetryAfter.Validate()); err != nil {
		return errors.Join(core.ErrPermitContract, err)
	}
	if t.Build.Offering() != t.Subject.Offering || t.RetryAfter.IsZero() {
		return core.ErrPermitContract
	}
	order, err := t.NotBefore.Compare(t.ExpiresAt)
	if err != nil || order != core.ComparisonLess {
		return core.ErrPermitContract
	}
	order, err = t.ContactAt.Compare(t.NotBefore)
	if err != nil || order == core.ComparisonLess {
		return core.ErrPermitContract
	}
	return nil
}

func (Terms) AttestationDomain() Domain { return DomainV1 }
func (t Terms) WriteCanonical(w io.Writer) error {
	if err := t.Validate(); err != nil {
		return err
	}
	encoded, err := core.MarshalCanonicalJSONDocument(t)
	if err != nil {
		return errors.Join(core.ErrPermitContract, err)
	}
	return writeCanonicalBytes(w, encoded)
}

// Revision is part of the signed terms; unknown revisions fail closed.
type Revision uint8

const (
	RevisionUnknown Revision = iota
	RevisionV1
)

func (r Revision) Validate() error {
	if r != RevisionV1 {
		return core.ErrPermitRevision
	}
	return nil
}
func (r Revision) IsValid() bool { return r == RevisionV1 }
func (r Revision) String() string {
	if !r.IsValid() {
		return unknownText
	}
	return revisionV1Text
}
func (r Revision) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	return core.MarshalCanonicalJSONDocument(uint8(r))
}
func (r *Revision) UnmarshalJSON(data []byte) error {
	if r == nil {
		return core.ErrPermitRevision
	}
	limits, err := documentLimits()
	if err != nil {
		return err
	}
	limits.DocumentMaximumBytes, err = core.NewByteCount(16)
	if err != nil {
		return err
	}
	value, err := core.DecodeStrictJSONStructure[uint8](data, limits)
	if err != nil {
		return errors.Join(core.ErrPermitRevision, err)
	}
	got := Revision(value)
	if err := got.Validate(); err != nil {
		return err
	}
	*r = got
	return nil
}

// Document is the sole client/server wire shape for issued permission terms.
type Document struct {
	Terms       Terms                   `json:"terms"`
	Attestation attest.Envelope[Domain] `json:"attestation"`
}

func (d Document) Validate() error {
	return errors.Join(d.Terms.Validate(), d.Attestation.Validate())
}

const DocumentMaximumBytes = 8192

// Decode admits bounded structural documents; callers must still Verify them.
func Decode(reader io.Reader) (Document, error) {
	limits, err := documentLimits()
	if err != nil {
		return Document{}, err
	}
	document, err := core.DecodeStrictJSON[Document](reader, limits)
	if err != nil {
		return Document{}, errors.Join(core.ErrPermitContract, err)
	}
	return document, nil
}

func documentLimits() (core.StrictJSONLimits, error) {
	maximum, err := core.NewByteCount(DocumentMaximumBytes)
	if err != nil {
		return core.StrictJSONLimits{}, errors.Join(core.ErrPermitContract, err)
	}
	return core.StrictJSONLimits{
		DocumentMaximumBytes: maximum, NestingDepthMaximum: 8,
		ObjectFieldMaximum: 16, ArrayItemMaximum: ActionsMaximumCount,
	}, nil
}

func (d Document) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, errors.Join(core.ErrPermitContract, err)
	}
	type wire Document
	return core.MarshalCanonicalJSONDocument(wire(d))
}

func (d Document) Write(w io.Writer) error {
	limits, err := documentLimits()
	if err != nil {
		return err
	}
	encoded, err := core.EncodeValidatedJSON(d, limits)
	if err != nil {
		return errors.Join(core.ErrPermitContract, err)
	}
	return writeCanonicalBytes(w, encoded)
}

// Encoding is complete and bounded before any caller-owned writer is touched.
// The fixed typed document has no unbounded aggregate or caller-owned raw bytes.
func writeCanonicalBytes(w io.Writer, encoded []byte) error {
	if len(encoded) == 0 || len(encoded) > DocumentMaximumBytes {
		return core.ErrPermitContract
	}
	if core.WriterIsNil(w) {
		return errors.Join(core.ErrPermitWrite, io.ErrClosedPipe)
	}
	written, err := w.Write(encoded)
	if written != len(encoded) {
		return errors.Join(core.ErrPermitWrite, io.ErrShortWrite, err)
	}
	if err != nil {
		return errors.Join(core.ErrPermitWrite, err)
	}
	return nil
}

// Sign performs mechanics only. The API must authorize issuance before calling.
func Sign(terms Terms, signer crypto.Signer) (Document, error) {
	envelope, err := attest.Sign(attest.SignRequest[Domain]{Body: terms, Signer: signer})
	if err != nil {
		return Document{}, errors.Join(core.ErrPermitContract, err)
	}
	return Document{Terms: terms, Attestation: envelope}, nil
}

// VerifyRequest contains independently known installation/build and an effective
// time supplied by Primitive's clock verification, not an entitlement decision.
type VerifyRequest struct {
	RequestNonce      controlwire.RequestNonce
	Document          Document
	TrustedKeys       attest.TrustedKeys
	Subject           lease.Subject
	Build             core.BuildIdentity
	EffectiveAt       temporal.Instant
	MinimumGeneration lease.Generation
}

// Verified cannot be constructed by callers and retains the authenticated terms.
type Verified struct {
	terms Terms
	proof attest.Verified[Domain]
}

// Terms returns a value copy of the authenticated instructions. Changing the
// returned value cannot change this proof or authorize a different permission.
func (v Verified) Terms() (Terms, error) {
	if err := errors.Join(v.proof.Validate(), v.terms.Validate()); err != nil {
		return Terms{}, errors.Join(core.ErrPermitAuthentication, err)
	}
	return v.terms, nil
}

// ContactAt exposes the authenticated instruction without inventing cadence.
func (v Verified) ContactAt() (temporal.Instant, error) {
	if err := errors.Join(v.proof.Validate(), v.terms.Validate()); err != nil {
		return temporal.Instant{}, errors.Join(core.ErrPermitAuthentication, err)
	}
	return v.terms.ContactAt, nil
}

func Verify(r VerifyRequest) (Verified, error) {
	if err := errors.Join(r.RequestNonce.Validate(), r.Subject.Validate(), r.Build.Validate(), r.EffectiveAt.Validate(), r.MinimumGeneration.Validate()); err != nil {
		return Verified{}, errors.Join(core.ErrPermitContract, err)
	}
	proof, err := attest.Verify(attest.VerifyRequest[Domain]{Body: r.Document.Terms, Envelope: r.Document.Attestation, TrustedKeys: r.TrustedKeys})
	if err != nil {
		return Verified{}, errors.Join(core.ErrPermitAuthentication, err)
	}
	t := r.Document.Terms
	if t.RequestNonce != r.RequestNonce || t.Subject != r.Subject || t.Build != r.Build {
		return Verified{}, core.ErrPermitBinding
	}
	generation, err := t.Generation.Uint64()
	if err != nil {
		return Verified{}, errors.Join(core.ErrPermitContract, err)
	}
	minimum, err := r.MinimumGeneration.Uint64()
	if err != nil {
		return Verified{}, errors.Join(core.ErrPermitContract, err)
	}
	if generation < minimum {
		return Verified{}, core.ErrPermitReplay
	}
	start, _ := r.EffectiveAt.Compare(t.NotBefore)
	end, _ := r.EffectiveAt.Compare(t.ExpiresAt)
	if start == core.ComparisonLess || end != core.ComparisonLess {
		return Verified{}, core.ErrPermitValidity
	}
	return Verified{terms: t, proof: proof}, nil
}

// Allows checks signed membership and validity at invocation. It never decides
// what a customer deserves or establishes that an uncontrolled clock is honest.
func (v Verified) Allows(action Action, now temporal.Instant) error {
	if err := v.proof.Validate(); err != nil {
		return errors.Join(core.ErrPermitAuthentication, err)
	}
	if err := errors.Join(v.terms.Validate(), action.Validate(), now.Validate()); err != nil {
		return errors.Join(core.ErrPermitContract, err)
	}
	start, err := now.Compare(v.terms.NotBefore)
	if err != nil {
		return errors.Join(core.ErrPermitContract, err)
	}
	end, err := now.Compare(v.terms.ExpiresAt)
	if err != nil {
		return errors.Join(core.ErrPermitContract, err)
	}
	if start == core.ComparisonLess || end != core.ComparisonLess {
		return core.ErrPermitValidity
	}
	allowed, err := v.terms.Actions.Contains(action)
	if err != nil {
		return err
	}
	if !allowed {
		return core.ErrPermitAction
	}
	return nil
}

// AllowsAll checks one nonempty requested set against the authenticated grant.
func (v Verified) AllowsAll(actions Actions, now temporal.Instant) error {
	if err := v.proof.Validate(); err != nil {
		return errors.Join(core.ErrPermitAuthentication, err)
	}
	if err := actions.Validate(); err != nil {
		return errors.Join(core.ErrPermitContract, err)
	}
	if actions.count == 0 {
		return core.ErrPermitAction
	}
	for _, action := range actions.values[:actions.count] {
		if err := v.Allows(action, now); err != nil {
			return err
		}
	}
	return nil
}
