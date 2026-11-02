package firestoredocuments

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// CreateIntegerJSON queues a create using the actual record's JSON projection.
// The returned extent counts emitted JSON bytes; only RunTransaction confirms
// commit. Returning an error from the transaction callback aborts queued writes.
func CreateIntegerJSON[T core.Validatable](ctx context.Context, request CreateRequest[T]) (core.ByteCount, error) {
	if err := documentContext(ctx); err != nil {
		return core.ByteCount{}, errors.Join(core.ErrFirestoreDocumentsContract, err)
	}
	if err := request.Validate(); err != nil {
		return core.ByteCount{}, err
	}
	data, err := core.MarshalCanonicalJSONDocument(request.Record)
	if err != nil {
		return core.ByteCount{}, errors.Join(core.ErrFirestoreDocumentsOperation, err)
	}
	if err := request.Transaction.Create(request.Reference, request.Record); err != nil {
		return core.ByteCount{}, errors.Join(core.ErrFirestoreDocumentsOperation, err)
	}
	return core.NewByteCount(uint64(len(data)))
}

// SetIntegerJSON queues a full replacement through the actual record's JSON
// projection. Its emitted extent is not a commit receipt.
func SetIntegerJSON[T core.Validatable](ctx context.Context, request SetRequest[T]) (core.ByteCount, error) {
	if err := documentContext(ctx); err != nil {
		return core.ByteCount{}, errors.Join(core.ErrFirestoreDocumentsContract, err)
	}
	if err := request.Validate(); err != nil {
		return core.ByteCount{}, err
	}
	data, err := core.MarshalCanonicalJSONDocument(request.Record)
	if err != nil {
		return core.ByteCount{}, errors.Join(core.ErrFirestoreDocumentsOperation, err)
	}
	if err := request.Transaction.Set(request.Reference, request.Record); err != nil {
		return core.ByteCount{}, errors.Join(core.ErrFirestoreDocumentsOperation, err)
	}
	return core.NewByteCount(uint64(len(data)))
}
