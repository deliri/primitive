package firestoredocuments

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"reflect"

	"cloud.google.com/go/firestore"
	"github.com/deliri/primitive/v2026/core"
)

// JSONDocument retains the exact SDK-projected source extent alongside the
// decoded caller record. Re-encoding Record can omit source fields, so it
// cannot establish the size of the original native JSON projection.
type JSONDocument[T core.Validatable] struct {
	Record T
	Bytes  core.ByteCount
}

func (d JSONDocument[T]) Validate() error {
	if err := d.Bytes.Validate(); err != nil {
		return errors.Join(core.ErrFirestoreDocumentsContract, err)
	}
	return (Document[T]{Record: d.Record}).Validate()
}

func (JSONDocument[T]) firestoreDocumentsSealedProjection() {}

// GetJSON reads a JSON-shaped stored document into the caller's actual record.
// Native Firestore values are projected inside the SDK; unknown JSON members
// remain errors rather than being discarded by Firestore's DataTo conversion.
func GetJSON[T core.Validatable](ctx context.Context, request GetRequest) (JSONDocument[T], error) {
	return getJSON[T](ctx, request, false)
}

// GetIntegerJSON preserves the native Firestore integer kind before the JSON
// projection. Every native floating-point value, including an integral DOUBLE,
// refuses with no decoded result. It performs the same one document read as
// GetJSON and leaves the stored document unchanged.
func GetIntegerJSON[T core.Validatable](ctx context.Context, request GetRequest) (JSONDocument[T], error) {
	return getJSON[T](ctx, request, true)
}

func getJSON[T core.Validatable](ctx context.Context, request GetRequest, integerOnly bool) (JSONDocument[T], error) {
	if err := documentContext(ctx); err != nil {
		return JSONDocument[T]{}, errors.Join(core.ErrFirestoreDocumentsContract, err)
	}
	if err := request.Validate(); err != nil {
		return JSONDocument[T]{}, err
	}
	if reflect.TypeFor[T]().Kind() != reflect.Struct {
		return JSONDocument[T]{}, core.ErrFirestoreDocumentsContract
	}
	var snapshot *firestore.DocumentSnapshot
	var err error
	if request.Transaction == nil {
		snapshot, err = request.Reference.Get(ctx)
	} else {
		snapshot, err = request.Transaction.Get(request.Reference)
	}
	if err != nil {
		return JSONDocument[T]{}, errors.Join(core.ErrFirestoreDocumentsOperation, err, ctx.Err(), context.Cause(ctx))
	}
	return projectJSON[T](ctx, snapshot, integerOnly)
}

// projectJSON turns one native snapshot into the caller's record through the
// canonical JSON projection. Get and query reads share it exactly, so a query
// row admits precisely what a point read admits.
func projectJSON[T core.Validatable](ctx context.Context, snapshot *firestore.DocumentSnapshot, integerOnly bool) (JSONDocument[T], error) {
	native := snapshot.Data()
	if integerOnly {
		if err := validateIntegerJSONValue(ctx, native); err != nil {
			return JSONDocument[T]{}, errors.Join(core.ErrFirestoreDocumentsOperation, err)
		}
	}
	data, err := core.MarshalCanonicalJSONDocument(native)
	if err != nil {
		return JSONDocument[T]{}, errors.Join(core.ErrFirestoreDocumentsOperation, err)
	}
	length, err := core.NewByteCount(uint64(len(data)))
	if err != nil {
		return JSONDocument[T]{}, errors.Join(core.ErrFirestoreDocumentsOperation, err)
	}
	result := JSONDocument[T]{Bytes: length}
	if err := json.Unmarshal(data, &result.Record, json.RejectUnknownMembers(true)); err != nil {
		return JSONDocument[T]{}, errors.Join(core.ErrFirestoreDocumentsOperation, err)
	}
	if err := result.Validate(); err != nil {
		return JSONDocument[T]{}, err
	}
	return result, nil
}

// This borrows only the official SDK's materialized Firestore map/array shape.
// Firestore bounds persisted field nesting; no second document population or
// new acceptance quota is built here. Other native kinds retain GetJSON's
// canonical projection and the caller's own typed decoder/validator.
func validateIntegerJSONValue(ctx context.Context, value any) error {
	if err := documentContext(ctx); err != nil {
		return err
	}
	switch native := value.(type) {
	case float32, float64:
		return core.ErrJSONContract
	case map[string]any:
		for _, child := range native {
			if err := validateIntegerJSONValue(ctx, child); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range native {
			if err := validateIntegerJSONValue(ctx, child); err != nil {
				return err
			}
		}
	}
	return nil
}
