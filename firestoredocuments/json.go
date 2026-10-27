package firestoredocuments

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"reflect"

	"cloud.google.com/go/firestore"
	"github.com/deliri/primitive/v2026/contextstate"
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
	if err := contextstate.Validate(ctx); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return JSONDocument[T]{}, errors.Join(core.ErrFirestoreDocumentsContract, err, context.Cause(ctx))
		}
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
	data, err := core.MarshalCanonicalJSONDocument(snapshot.Data())
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
