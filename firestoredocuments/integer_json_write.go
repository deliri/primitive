package firestoredocuments

import (
	"bytes"
	"context"
	json "encoding/json"
	"errors"

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
	value, extent, err := integerJSONWriteValue(ctx, request.Record)
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
	value, extent, err := integerJSONWriteValue(ctx, request.Record)
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
func integerJSONWriteValue[T core.Validatable](ctx context.Context, record T) (map[string]any, core.ByteCount, error) {
	data, err := core.MarshalCanonicalJSONDocument(record)
	if err != nil {
		return nil, core.ByteCount{}, err
	}
	var value map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, core.ByteCount{}, errors.Join(core.ErrJSONContract, err)
	}
	if value == nil {
		return nil, core.ByteCount{}, core.ErrJSONContract
	}
	if _, err := integerJSONNativeValue(ctx, value); err != nil {
		return nil, core.ByteCount{}, err
	}
	extent, err := core.NewByteCount(uint64(len(data)))
	if err != nil {
		return nil, core.ByteCount{}, err
	}
	return value, extent, nil
}

// The standard JSON decoder owns this finite native shape. Convert its number
// tokens in place; do not introduce a second document tree or acceptance quota.
func integerJSONNativeValue(ctx context.Context, value any) (any, error) {
	if err := documentContext(ctx); err != nil {
		return nil, err
	}
	switch native := value.(type) {
	case json.Number:
		integer, err := native.Int64()
		if err != nil {
			return nil, errors.Join(core.ErrJSONContract, err)
		}
		return integer, nil
	case map[string]any:
		for key, child := range native {
			converted, err := integerJSONNativeValue(ctx, child)
			if err != nil {
				return nil, err
			}
			native[key] = converted
		}
	case []any:
		for index, child := range native {
			converted, err := integerJSONNativeValue(ctx, child)
			if err != nil {
				return nil, err
			}
			native[index] = converted
		}
	case string, bool, nil:
	default:
		return nil, core.ErrJSONContract
	}
	return value, nil
}
