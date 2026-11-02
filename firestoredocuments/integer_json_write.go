package firestoredocuments

import (
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"strconv"

	"github.com/deliri/primitive/v2026/core"
)

// CreateIntegerJSON queues a create using the actual record's JSON projection.
// All JSON numbers must be signed int64 integer tokens; fractions, exponents and
// overflow refuse before queuing. The returned count is emitted JSON extent,
// not a committed-write receipt. The transaction callback can still abort.
func CreateIntegerJSON[T core.Validatable](ctx context.Context, request CreateRequest[T]) (core.ByteCount, error) {
	if err := documentContext(ctx); err != nil {
		return core.ByteCount{}, errors.Join(core.ErrFirestoreDocumentsContract, err)
	}
	if err := request.Validate(); err != nil {
		return core.ByteCount{}, err
	}
	value, extent, err := integerJSONWriteValue(request.Record)
	if contextErr := documentContext(ctx); contextErr != nil {
		return core.ByteCount{}, errors.Join(core.ErrFirestoreDocumentsContract, contextErr, err)
	}
	if err != nil {
		return core.ByteCount{}, errors.Join(core.ErrFirestoreDocumentsOperation, err)
	}
	if err := request.Transaction.Create(request.Reference, value); err != nil {
		return core.ByteCount{}, errors.Join(core.ErrFirestoreDocumentsOperation, err)
	}
	return extent, nil
}

// SetIntegerJSON queues a full replacement through the actual record's JSON
// projection, with the same integer-token contract as CreateIntegerJSON. Its
// emitted extent is not a commit receipt; omitted old fields are removed.
func SetIntegerJSON[T core.Validatable](ctx context.Context, request SetRequest[T]) (core.ByteCount, error) {
	if err := documentContext(ctx); err != nil {
		return core.ByteCount{}, errors.Join(core.ErrFirestoreDocumentsContract, err)
	}
	if err := request.Validate(); err != nil {
		return core.ByteCount{}, err
	}
	value, extent, err := integerJSONWriteValue(request.Record)
	if contextErr := documentContext(ctx); contextErr != nil {
		return core.ByteCount{}, errors.Join(core.ErrFirestoreDocumentsContract, contextErr, err)
	}
	if err != nil {
		return core.ByteCount{}, errors.Join(core.ErrFirestoreDocumentsOperation, err)
	}
	if err := request.Transaction.Set(request.Reference, value); err != nil {
		return core.ByteCount{}, errors.Join(core.ErrFirestoreDocumentsOperation, err)
	}
	return extent, nil
}

// JSON's object/array representation is kept solely at the official SDK edge.
// Marshal the caller's real validated record once; do not reflect it through
// Firestore tags or round numeric instants through a floating-point value.
func integerJSONWriteValue[T core.Validatable](record T) (map[string]any, core.ByteCount, error) {
	data, err := core.MarshalCanonicalJSONDocument(record)
	if err != nil {
		return nil, core.ByteCount{}, err
	}
	var value map[string]any
	// The native decoder still owns maps, arrays, strings, booleans and null.
	// Only number tokens change their native destination from float64 to int64.
	numbers := json.UnmarshalFromFunc(func(decoder *jsontext.Decoder, target *any) error {
		if decoder.PeekKind() != jsontext.KindNumber {
			return errors.ErrUnsupported
		}
		token, readErr := decoder.ReadValue()
		if readErr != nil {
			return readErr
		}
		integer, parseErr := strconv.ParseInt(string(token), 10, 64)
		if parseErr != nil {
			return parseErr
		}
		*target = integer
		return nil
	})
	if err := json.Unmarshal(data, &value, json.WithUnmarshalers(numbers)); err != nil {
		return nil, core.ByteCount{}, errors.Join(core.ErrJSONContract, err)
	}
	if value == nil {
		return nil, core.ByteCount{}, core.ErrJSONContract
	}
	extent, err := core.NewByteCount(uint64(len(data)))
	if err != nil {
		return nil, core.ByteCount{}, err
	}
	return value, extent, nil
}

// IntegerJSONExtent validates the actual record and its signed-integer JSON
// projection without queuing a write. It returns the exact emitted JSON extent,
// allowing an owner to enforce its document budget before replacing a receiver.
// Fractions, exponents, overflow and non-object projections refuse identically
// to the write doors; no Firestore reference or transaction is consulted.
func IntegerJSONExtent[T core.Validatable](record T) (core.ByteCount, error) {
	if err := (Document[T]{Record: record}).Validate(); err != nil {
		return core.ByteCount{}, err
	}
	_, extent, err := integerJSONWriteValue(record)
	if err != nil {
		return core.ByteCount{}, errors.Join(core.ErrFirestoreDocumentsOperation, err)
	}
	return extent, nil
}
