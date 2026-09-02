package proofledger

import (
	"crypto"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	AppendReceiptJSONMaximumBytes         = 4 << 10
	AppendReceiptDocumentJSONMaximumBytes = 1 << 16
)

// AppendReceipt is the producer-authenticated durable result of one exact
// ledger append. It has no independent receipt identity: the ledger, event,
// and idempotency request already name the operation it proves.
type AppendReceipt struct {
	RecordedAt   temporal.Instant         `json:"recorded_at"`
	Sequence     Sequence                 `json:"sequence"`
	Request      controlwire.RequestNonce `json:"request"`
	PreviousHash core.SHA256Digest        `json:"previous_hash"`
	Hash         core.SHA256Digest        `json:"hash"`
	Producer     core.Ed25519PublicKey    `json:"producer"`
	Ledger       LedgerIdentity           `json:"ledger"`
	Event        EventIdentity            `json:"event"`
}

func NewAppendReceipt[P CanonicalPayload](event Envelope[P], producer core.Ed25519PublicKey) (AppendReceipt, error) {
	if err := errors.Join(event.Validate(), producer.Validate()); err != nil {
		return AppendReceipt{}, contractError(err)
	}
	receipt := AppendReceipt{
		Ledger: event.Ledger, Event: event.Event, Request: event.Request,
		Sequence: event.Sequence, PreviousHash: event.PreviousHash, Hash: event.Hash,
		RecordedAt: event.RecordedAt, Producer: producer,
	}
	return receipt, receipt.Validate()
}

func (r AppendReceipt) Validate() error {
	if err := errors.Join(r.Ledger.Validate(), r.Event.Validate(), r.Request.Validate(), r.Sequence.Validate(),
		r.PreviousHash.Validate(), r.Hash.Validate(), r.RecordedAt.Validate(), r.Producer.Validate()); err != nil {
		return contractError(err)
	}
	if r.Sequence == 1 && r.PreviousHash != GenesisHash() {
		return errors.Join(core.ErrProofLedgerPreviousHashMismatch, contractError())
	}
	if r.Sequence > 1 && r.PreviousHash == GenesisHash() {
		return errors.Join(core.ErrProofLedgerPreviousHashMismatch, contractError())
	}
	return nil
}

func (AppendReceipt) AttestationDomain() AppendReceiptSigningDomain {
	return AppendReceiptSigningDomainV1
}

func (r AppendReceipt) WriteCanonical(destination io.Writer) error {
	if destination == nil {
		return contractError(errors.New("proof ledger receipt canonical destination is nil"))
	}
	encoded, err := r.MarshalJSON()
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

func VerifyAppendReceipt[P CanonicalPayload](event Envelope[P], receipt AppendReceipt) error {
	if err := errors.Join(event.Validate(), receipt.Validate()); err != nil {
		return err
	}
	if event.Ledger != receipt.Ledger || event.Event != receipt.Event || event.Request != receipt.Request ||
		event.Sequence != receipt.Sequence || event.PreviousHash != receipt.PreviousHash || event.Hash != receipt.Hash ||
		event.RecordedAt != receipt.RecordedAt {
		return errors.Join(core.ErrProofLedgerAppendReceiptMismatch, contractError())
	}
	return nil
}

type appendReceiptWire AppendReceipt

func (r AppendReceipt) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(appendReceiptWire(r))
	if err != nil || len(encoded) > AppendReceiptJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (r *AppendReceipt) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError()
	}
	limits, err := proofLedgerJSONLimits(AppendReceiptJSONMaximumBytes)
	if err != nil {
		return jsonError(err)
	}
	wire, err := core.DecodeStrictJSONStructure[appendReceiptWire](data, limits)
	if err != nil {
		return jsonError(err)
	}
	candidate := AppendReceipt(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*r = candidate
	return nil
}

type AppendReceiptDocument struct {
	Receipt     AppendReceipt                               `json:"receipt"`
	Attestation attest.Envelope[AppendReceiptSigningDomain] `json:"attestation"`
}

func (d AppendReceiptDocument) Validate() error {
	if err := errors.Join(d.Receipt.Validate(), d.Attestation.Validate()); err != nil {
		return contractError(err)
	}
	if d.Attestation.Domain != AppendReceiptSigningDomainV1 || d.Attestation.Signer != d.Receipt.Producer {
		return errors.Join(core.ErrProofLedgerAppendReceiptMismatch, contractError())
	}
	return nil
}

type appendReceiptDocumentWire AppendReceiptDocument

func (d AppendReceiptDocument) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(appendReceiptDocumentWire(d))
	if err != nil || len(encoded) > AppendReceiptDocumentJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (d *AppendReceiptDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("nil proof ledger receipt document receiver"))
	}
	limits, err := receiptDocumentJSONLimits()
	if err != nil {
		return jsonError(err)
	}
	wire, err := core.DecodeStrictJSONStructure[appendReceiptDocumentWire](data, limits)
	if err != nil {
		return jsonError(err)
	}
	candidate := AppendReceiptDocument(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

func receiptDocumentJSONLimits() (core.StrictJSONLimits, error) {
	maximum, err := core.NewByteCount(AppendReceiptDocumentJSONMaximumBytes)
	if err != nil {
		return core.StrictJSONLimits{}, contractError(err)
	}
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = maximum
	return limits, limits.Validate()
}

type AppendReceiptIssuance[P CanonicalPayload] struct {
	Signer   crypto.Signer
	Event    Envelope[P]
	Producer core.Ed25519PublicKey
}

func IssueAppendReceipt[P CanonicalPayload](issuance AppendReceiptIssuance[P]) (AppendReceiptDocument, error) {
	if err := issuance.Event.Validate(); err != nil {
		return AppendReceiptDocument{}, err
	}
	if issuance.Signer == nil {
		return AppendReceiptDocument{}, contractError(errors.New("proof ledger append receipt signer is absent"))
	}
	if err := issuance.Producer.Validate(); err != nil {
		return AppendReceiptDocument{}, contractError(err)
	}
	receipt, err := NewAppendReceipt(issuance.Event, issuance.Producer)
	if err != nil {
		return AppendReceiptDocument{}, err
	}
	envelope, err := attest.Sign(attest.SignRequest[AppendReceiptSigningDomain]{Body: receipt, Signer: issuance.Signer})
	if err != nil {
		return AppendReceiptDocument{}, contractError(err)
	}
	document := AppendReceiptDocument{Receipt: receipt, Attestation: envelope}
	if err := document.Validate(); err != nil {
		return AppendReceiptDocument{}, err
	}
	return document, nil
}

type AppendReceiptVerification[P CanonicalPayload] struct {
	Event       Envelope[P]
	Document    AppendReceiptDocument
	TrustedKeys attest.TrustedKeys
}

type VerifiedAppendReceipt struct {
	document AppendReceiptDocument
	proof    attest.Verified[AppendReceiptSigningDomain]
}

func VerifyAppendReceiptDocument[P CanonicalPayload](verification AppendReceiptVerification[P]) (VerifiedAppendReceipt, error) {
	if err := errors.Join(verification.Event.Validate(), verification.Document.Validate(), verification.TrustedKeys.Validate()); err != nil {
		return VerifiedAppendReceipt{}, contractError(err)
	}
	if err := VerifyAppendReceipt(verification.Event, verification.Document.Receipt); err != nil {
		return VerifiedAppendReceipt{}, err
	}
	proof, err := attest.Verify(attest.VerifyRequest[AppendReceiptSigningDomain]{
		Body: verification.Document.Receipt, Envelope: verification.Document.Attestation, TrustedKeys: verification.TrustedKeys,
	})
	if err != nil {
		return VerifiedAppendReceipt{}, errors.Join(core.ErrProofLedgerAppendReceiptMismatch, contractError(err))
	}
	verified := VerifiedAppendReceipt{document: verification.Document, proof: proof}
	return verified, verified.Validate()
}

func (v VerifiedAppendReceipt) Validate() error {
	if err := errors.Join(v.document.Validate(), v.proof.Validate()); err != nil {
		return errors.Join(core.ErrProofLedgerAppendReceiptMismatch, contractError(err))
	}
	return nil
}

func (v VerifiedAppendReceipt) Document() (AppendReceiptDocument, error) {
	if err := v.Validate(); err != nil {
		return AppendReceiptDocument{}, err
	}
	return v.document, nil
}

var _ attest.CanonicalBody[AppendReceiptSigningDomain] = AppendReceipt{}
