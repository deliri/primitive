package proofledger

import (
	"bytes"
	json "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const EventJSONMaximumBytes = 96 << 10

type CanonicalPayload interface {
	core.ValidatedJSONMarshaler
}

type Head struct {
	Ledger   LedgerIdentity    `json:"ledger"`
	Sequence Position          `json:"sequence"`
	Hash     core.SHA256Digest `json:"hash"`
}

type AppendIntent[P CanonicalPayload] struct {
	Payload      P                        `json:"payload"`
	ExpectedHead Head                     `json:"expected_head"`
	Request      controlwire.RequestNonce `json:"request"`
	Actor        core.Ed25519PublicKey    `json:"actor"`
	Ledger       LedgerIdentity           `json:"ledger"`
}

type Issue[P CanonicalPayload] struct {
	Intent     AppendIntent[P]
	Event      EventIdentity
	RecordedAt temporal.Instant
}

type Envelope[P CanonicalPayload] struct {
	Payload      P                        `json:"payload"`
	RecordedAt   temporal.Instant         `json:"recorded_at"`
	Sequence     Sequence                 `json:"sequence"`
	Request      controlwire.RequestNonce `json:"request"`
	PreviousHash core.SHA256Digest        `json:"previous_hash"`
	Hash         core.SHA256Digest        `json:"hash"`
	Actor        core.Ed25519PublicKey    `json:"actor"`
	Ledger       LedgerIdentity           `json:"ledger"`
	Event        EventIdentity            `json:"event"`
}

type eventCommitment[P CanonicalPayload] struct {
	Payload      P                        `json:"payload"`
	RecordedAt   temporal.Instant         `json:"recorded_at"`
	Sequence     Sequence                 `json:"sequence"`
	Request      controlwire.RequestNonce `json:"request"`
	PreviousHash core.SHA256Digest        `json:"previous_hash"`
	Actor        core.Ed25519PublicKey    `json:"actor"`
	Ledger       LedgerIdentity           `json:"ledger"`
	Event        EventIdentity            `json:"event"`
}

type envelopeWire[P CanonicalPayload] Envelope[P]

func proofLedgerJSONLimits(maximum uint64) (core.StrictJSONLimits, error) {
	encodedMaximum, err := core.NewByteCount(maximum)
	if err != nil {
		return core.StrictJSONLimits{}, jsonError(err)
	}
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = encodedMaximum
	if err := limits.Validate(); err != nil {
		return core.StrictJSONLimits{}, jsonError(err)
	}
	return limits, nil
}

func NewGenesisHead(ledger LedgerIdentity) (Head, error) {
	head := Head{Ledger: ledger, Hash: GenesisHash()}
	return head, head.Validate()
}

func (h Head) Validate() error {
	if err := errors.Join(h.Ledger.Validate(), h.Hash.Validate()); err != nil {
		return contractError(err)
	}
	if h.Sequence == 0 && h.Hash != GenesisHash() {
		return errors.Join(core.ErrProofLedgerPreviousHashMismatch, contractError())
	}
	if h.Sequence > 0 && h.Hash == GenesisHash() {
		return errors.Join(core.ErrProofLedgerPreviousHashMismatch, contractError())
	}
	return nil
}

func (i AppendIntent[P]) Validate() error {
	if err := errors.Join(i.Request.Validate(), i.Ledger.Validate(), i.ExpectedHead.Validate(), i.Actor.Validate(), i.Payload.Validate()); err != nil {
		return contractError(err)
	}
	if i.ExpectedHead.Ledger != i.Ledger {
		return errors.Join(core.ErrProofLedgerSequenceConflict, contractError())
	}
	return nil
}

func (i AppendIntent[P]) Digest() (core.SHA256Digest, error) {
	if err := i.Validate(); err != nil {
		return core.SHA256Digest{}, err
	}
	encoded, err := core.MarshalCanonicalJSONDocument(i)
	if err != nil {
		return core.SHA256Digest{}, contractError(err)
	}
	return core.SHA256Of(encoded), nil
}

// ValidateAppendHead binds an append intent to the provider's durable head.
func ValidateAppendHead[P CanonicalPayload](intent AppendIntent[P], current Head) error {
	if err := errors.Join(intent.Validate(), current.Validate()); err != nil {
		return contractError(err)
	}
	if intent.Ledger != current.Ledger || intent.ExpectedHead != current {
		return errors.Join(core.ErrProofLedgerSequenceConflict, contractError())
	}
	return nil
}

func NewEnvelope[P CanonicalPayload](issue Issue[P]) (Envelope[P], error) {
	if err := errors.Join(issue.Intent.Validate(), issue.Event.Validate(), issue.RecordedAt.Validate()); err != nil {
		return Envelope[P]{}, contractError(err)
	}
	sequence, err := issueSequence(issue.Intent.ExpectedHead.Sequence)
	if err != nil {
		return Envelope[P]{}, err
	}
	event := Envelope[P]{
		Ledger: issue.Intent.Ledger, Event: issue.Event, Request: issue.Intent.Request,
		Sequence: sequence, PreviousHash: issue.Intent.ExpectedHead.Hash,
		Actor: issue.Intent.Actor, RecordedAt: issue.RecordedAt, Payload: issue.Intent.Payload,
	}
	event.Hash, err = event.expectedHash()
	if err != nil {
		return Envelope[P]{}, err
	}
	return event, event.Validate()
}

func issueSequence(previous Position) (Sequence, error) {
	if previous == 0 {
		return NewSequence(1)
	}
	current, err := NewSequence(uint64(previous))
	if err != nil {
		return 0, errors.Join(core.ErrProofLedgerSequenceConflict, err)
	}
	return current.Next()
}

func (e Envelope[P]) Validate() error {
	if err := errors.Join(e.Ledger.Validate(), e.Event.Validate(), e.Request.Validate(), e.Sequence.Validate(),
		e.PreviousHash.Validate(), e.Hash.Validate(), e.Actor.Validate(), e.RecordedAt.Validate(), e.Payload.Validate()); err != nil {
		return contractError(err)
	}
	if e.Sequence == 1 && e.PreviousHash != GenesisHash() {
		return errors.Join(core.ErrProofLedgerPreviousHashMismatch, contractError())
	}
	if e.Sequence > 1 && e.PreviousHash == GenesisHash() {
		return errors.Join(core.ErrProofLedgerPreviousHashMismatch, contractError())
	}
	want, err := e.expectedHash()
	if err != nil {
		return err
	}
	if e.Hash != want {
		return errors.Join(core.ErrProofLedgerTampering, contractError())
	}
	return e.validateEncodedSize()
}

func (e Envelope[P]) validateEncodedSize() error {
	encoded, err := core.MarshalCanonicalJSONDocument(envelopeWire[P](e))
	if err != nil || len(encoded) > EventJSONMaximumBytes {
		return jsonError(err)
	}
	return nil
}

func (e Envelope[P]) expectedHash() (core.SHA256Digest, error) {
	commitment := eventCommitment[P]{
		Ledger: e.Ledger, Event: e.Event, Request: e.Request, Sequence: e.Sequence,
		PreviousHash: e.PreviousHash, Actor: e.Actor, RecordedAt: e.RecordedAt, Payload: e.Payload,
	}
	encoded, err := core.MarshalCanonicalJSONDocument(commitment)
	if err != nil {
		return core.SHA256Digest{}, contractError(err)
	}
	return core.SHA256Of(encoded), nil
}

func (e Envelope[P]) Head() Head {
	return Head{Ledger: e.Ledger, Sequence: Position(e.Sequence), Hash: e.Hash}
}

func (e Envelope[P]) MarshalJSON() ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(envelopeWire[P](e))
	if err != nil || len(encoded) > EventJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func DecodeEnvelope[P CanonicalPayload, PPtr interface {
	*P
	core.Validatable
	json.Unmarshaler
}](data []byte) (Envelope[P], error) {
	limits, err := proofLedgerJSONLimits(EventJSONMaximumBytes)
	if err != nil {
		return Envelope[P]{}, err
	}
	wire, err := core.DecodeStrictJSONStructure[envelopeWire[P]](data, limits)
	if err != nil {
		return Envelope[P]{}, jsonError(err)
	}
	candidate := Envelope[P](wire)
	if err := PPtr(&candidate.Payload).Validate(); err != nil {
		return Envelope[P]{}, jsonError(err)
	}
	if err := candidate.Validate(); err != nil {
		return Envelope[P]{}, jsonError(err)
	}
	canonical, err := candidate.MarshalJSON()
	if err != nil || !bytes.Equal(canonical, data) {
		return Envelope[P]{}, jsonError(err)
	}
	return candidate, nil
}
