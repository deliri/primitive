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

const ReceiptDocumentJSONMaximumBytes = 1 << 16

type Receipt struct {
	Ledger       LedgerIdentity           `json:"ledger"`
	Event        EventIdentity            `json:"event"`
	Request      controlwire.RequestNonce `json:"request"`
	Sequence     Sequence                 `json:"sequence"`
	PreviousHash core.SHA256Digest        `json:"previous_hash"`
	Hash         core.SHA256Digest        `json:"hash"`
	RecordedAt   temporal.Instant         `json:"recorded_at"`
	Producer     core.Ed25519PublicKey    `json:"producer"`
}

func NewReceipt[P CanonicalPayload](event Envelope[P], producer core.Ed25519PublicKey) (Receipt, error) {
	if err := errors.Join(event.Validate(), producer.Validate()); err != nil {
		return Receipt{}, contractError(err)
	}
	receipt := Receipt{
		Ledger: event.Ledger, Event: event.Event, Request: event.Request,
		Sequence: event.Sequence, PreviousHash: event.PreviousHash, Hash: event.Hash,
		RecordedAt: event.RecordedAt, Producer: producer,
	}
	return receipt, receipt.Validate()
}

func (r Receipt) Validate() error {
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

func (Receipt) AttestationDomain() ReceiptSigningDomain { return ReceiptSigningDomainV1 }

func (r Receipt) WriteCanonical(destination io.Writer) error {
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

func VerifyReceipt[P CanonicalPayload](event Envelope[P], receipt Receipt) error {
	if err := errors.Join(event.Validate(), receipt.Validate()); err != nil {
		return err
	}
	if event.Ledger != receipt.Ledger || event.Event != receipt.Event || event.Request != receipt.Request ||
		event.Sequence != receipt.Sequence || event.PreviousHash != receipt.PreviousHash || event.Hash != receipt.Hash ||
		event.RecordedAt != receipt.RecordedAt {
		return errors.Join(core.ErrProofLedgerReceiptMismatch, contractError())
	}
	return nil
}

type receiptWire Receipt

func (r Receipt) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return core.MarshalCanonicalJSONDocument(receiptWire(r))
}

func (r *Receipt) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError()
	}
	wire, err := core.DecodeStrictJSONStructure[receiptWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return jsonError(err)
	}
	candidate := Receipt(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*r = candidate
	return nil
}

type ReceiptDocument struct {
	Receipt     Receipt                               `json:"receipt"`
	Attestation attest.Envelope[ReceiptSigningDomain] `json:"attestation"`
}

func (d ReceiptDocument) Validate() error {
	if err := errors.Join(d.Receipt.Validate(), d.Attestation.Validate()); err != nil {
		return contractError(err)
	}
	if d.Attestation.Domain != ReceiptSigningDomainV1 || d.Attestation.Signer != d.Receipt.Producer {
		return errors.Join(core.ErrProofLedgerReceiptMismatch, contractError())
	}
	return nil
}

type receiptDocumentWire ReceiptDocument

func (d ReceiptDocument) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(receiptDocumentWire(d))
	if err != nil || len(encoded) > ReceiptDocumentJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (d *ReceiptDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("nil proof ledger receipt document receiver"))
	}
	limits, err := receiptDocumentJSONLimits()
	if err != nil {
		return jsonError(err)
	}
	wire, err := core.DecodeStrictJSONStructure[receiptDocumentWire](data, limits)
	if err != nil {
		return jsonError(err)
	}
	candidate := ReceiptDocument(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

func receiptDocumentJSONLimits() (core.StrictJSONLimits, error) {
	maximum, err := core.NewByteCount(ReceiptDocumentJSONMaximumBytes)
	if err != nil {
		return core.StrictJSONLimits{}, contractError(err)
	}
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = maximum
	return limits, limits.Validate()
}

type ReceiptIssuance[P CanonicalPayload] struct {
	Event    Envelope[P]
	Producer core.Ed25519PublicKey
	Signer   crypto.Signer
}

func IssueReceipt[P CanonicalPayload](issuance ReceiptIssuance[P]) (ReceiptDocument, error) {
	if err := issuance.Event.Validate(); err != nil {
		return ReceiptDocument{}, err
	}
	if issuance.Signer == nil {
		return ReceiptDocument{}, contractError(errors.New("proof ledger receipt signer is absent"))
	}
	if err := issuance.Producer.Validate(); err != nil {
		return ReceiptDocument{}, contractError(err)
	}
	receipt, err := NewReceipt(issuance.Event, issuance.Producer)
	if err != nil {
		return ReceiptDocument{}, err
	}
	envelope, err := attest.Sign(attest.SignRequest[ReceiptSigningDomain]{Body: receipt, Signer: issuance.Signer})
	if err != nil {
		return ReceiptDocument{}, contractError(err)
	}
	document := ReceiptDocument{Receipt: receipt, Attestation: envelope}
	if err := document.Validate(); err != nil {
		return ReceiptDocument{}, err
	}
	return document, nil
}

type ReceiptVerification[P CanonicalPayload] struct {
	Event       Envelope[P]
	Document    ReceiptDocument
	TrustedKeys attest.TrustedKeys
}

type VerifiedReceipt struct {
	document ReceiptDocument
	proof    attest.Verified[ReceiptSigningDomain]
}

func VerifyReceiptDocument[P CanonicalPayload](verification ReceiptVerification[P]) (VerifiedReceipt, error) {
	if err := errors.Join(verification.Event.Validate(), verification.Document.Validate(), verification.TrustedKeys.Validate()); err != nil {
		return VerifiedReceipt{}, contractError(err)
	}
	if err := VerifyReceipt(verification.Event, verification.Document.Receipt); err != nil {
		return VerifiedReceipt{}, err
	}
	proof, err := attest.Verify(attest.VerifyRequest[ReceiptSigningDomain]{
		Body: verification.Document.Receipt, Envelope: verification.Document.Attestation, TrustedKeys: verification.TrustedKeys,
	})
	if err != nil {
		return VerifiedReceipt{}, errors.Join(core.ErrProofLedgerReceiptMismatch, contractError(err))
	}
	verified := VerifiedReceipt{document: verification.Document, proof: proof}
	return verified, verified.Validate()
}

func (v VerifiedReceipt) Validate() error {
	if err := errors.Join(v.document.Validate(), v.proof.Validate()); err != nil {
		return errors.Join(core.ErrProofLedgerReceiptMismatch, contractError(err))
	}
	return nil
}

func (v VerifiedReceipt) Document() (ReceiptDocument, error) {
	if err := v.Validate(); err != nil {
		return ReceiptDocument{}, err
	}
	return v.document, nil
}

var _ attest.CanonicalBody[ReceiptSigningDomain] = Receipt{}
