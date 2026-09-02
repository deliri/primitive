package proofledger

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
)

type PageRequest struct {
	Ledger LedgerIdentity `json:"ledger"`
	After  Head           `json:"after"`
	Limit  PageLimit      `json:"limit"`
}

type Page[P CanonicalPayload] struct {
	After  Head          `json:"after"`
	Limit  PageLimit     `json:"limit"`
	Events []Envelope[P] `json:"events"`
	Next   Head          `json:"next"`
	More   bool          `json:"more"`
}

func (r PageRequest) Validate() error {
	if err := errors.Join(r.Ledger.Validate(), r.After.Validate(), r.Limit.Validate()); err != nil {
		return contractError(err)
	}
	if r.Ledger != r.After.Ledger {
		return errors.Join(core.ErrProofLedgerSequenceConflict, contractError())
	}
	return nil
}

func (p Page[P]) Validate() error {
	if err := errors.Join(p.After.Validate(), p.Limit.Validate(), p.Next.Validate()); err != nil {
		return contractError(err)
	}
	limit, err := p.Limit.Uint16()
	if err != nil {
		return err
	}
	if len(p.Events) > int(limit) || p.After.Ledger != p.Next.Ledger || p.More && len(p.Events) != int(limit) {
		return errors.Join(core.ErrProofLedgerSequenceConflict, contractError())
	}
	verifier, err := NewVerifier[P](p.After)
	if err != nil {
		return err
	}
	for index := range p.Events {
		if err := verifier.Observe(p.Events[index]); err != nil {
			return err
		}
	}
	if len(p.Events) == 0 {
		if p.Next != p.After || p.More {
			return errors.Join(core.ErrProofLedgerSequenceConflict, contractError())
		}
		return p.validateEncodedSize()
	}
	if err := p.validateNonempty(verifier); err != nil {
		return err
	}
	return p.validateEncodedSize()
}

type pageWire[P CanonicalPayload] Page[P]

func (p Page[P]) validateEncodedSize() error {
	encoded, err := core.MarshalCanonicalJSONDocument(pageWire[P](p))
	if err != nil || len(encoded) > PageJSONMaximumBytes {
		return jsonError(err)
	}
	return nil
}

func (p Page[P]) validateNonempty(verifier Verifier[P]) error {
	if verifier.Head() != p.Next {
		return errors.Join(core.ErrProofLedgerSequenceConflict, contractError())
	}
	return nil
}

type Verifier[P CanonicalPayload] struct {
	head Head
}

func NewVerifier[P CanonicalPayload](after Head) (Verifier[P], error) {
	if err := after.Validate(); err != nil {
		return Verifier[P]{}, err
	}
	return Verifier[P]{head: after}, nil
}

func (v *Verifier[P]) Observe(event Envelope[P]) error {
	if v == nil {
		return contractError()
	}
	if err := event.Validate(); err != nil {
		return err
	}
	want, err := nextSequence(v.head.Sequence)
	if err != nil || event.Ledger != v.head.Ledger || event.Sequence != want {
		return errors.Join(core.ErrProofLedgerSequenceConflict, contractError(err))
	}
	if event.PreviousHash != v.head.Hash {
		return errors.Join(core.ErrProofLedgerPreviousHashMismatch, contractError())
	}
	v.head = event.Head()
	return nil
}

func (v Verifier[P]) Head() Head { return v.head }

func (v Verifier[P]) Finish(expected Head) error {
	if err := expected.Validate(); err != nil {
		return err
	}
	if v.head != expected {
		return errors.Join(core.ErrProofLedgerTruncated, contractError())
	}
	return nil
}

func nextSequence(previous Position) (Sequence, error) {
	if previous == Position(^uint64(0)) {
		return 0, core.ErrProofLedgerSequenceConflict
	}
	return Sequence(previous + 1), nil
}

type Appender[P CanonicalPayload] interface {
	Append(context.Context, AppendIntent[P]) (ReceiptDocument, error)
	Resolve(context.Context, ResolveRequest) (ReceiptDocument, error)
}

type ResolveRequest struct {
	Ledger  LedgerIdentity
	Request controlwire.RequestNonce
}

func (r ResolveRequest) Validate() error {
	if err := errors.Join(r.Ledger.Validate(), r.Request.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

type Reader[P CanonicalPayload] interface {
	ReadPage(context.Context, PageRequest) (Page[P], error)
	Open(context.Context, PageRequest) (Iterator[P], error)
}

type Iterator[P CanonicalPayload] interface {
	Next(context.Context) (Envelope[P], error)
	Close() error
}
