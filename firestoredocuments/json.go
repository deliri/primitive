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

// GetJSON reads a JSON-shaped stored document into the caller's actual record.
// Native Firestore values are projected inside the SDK; unknown JSON members
// remain errors rather than being discarded by Firestore's DataTo conversion.
func GetJSON[T core.Validatable](ctx context.Context, request GetRequest) (Document[T], error) {
	if err := contextstate.Validate(ctx); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return Document[T]{}, errors.Join(core.ErrFirestoreDocumentsContract, err, context.Cause(ctx))
		}
		return Document[T]{}, errors.Join(core.ErrFirestoreDocumentsContract, err)
	}
	if err := request.Validate(); err != nil {
		return Document[T]{}, err
	}
	if reflect.TypeFor[T]().Kind() != reflect.Struct {
		return Document[T]{}, core.ErrFirestoreDocumentsContract
	}
	var snapshot *firestore.DocumentSnapshot
	var err error
	if request.Transaction == nil {
		snapshot, err = request.Reference.Get(ctx)
	} else {
		snapshot, err = request.Transaction.Get(request.Reference)
	}
	if err != nil {
		return Document[T]{}, errors.Join(core.ErrFirestoreDocumentsOperation, err)
	}
	data, err := core.MarshalCanonicalJSONDocument(snapshot.Data())
	if err != nil {
		return Document[T]{}, errors.Join(core.ErrFirestoreDocumentsOperation, err)
	}
	var result Document[T]
	if err := json.Unmarshal(data, &result.Record, json.RejectUnknownMembers(true)); err != nil {
		return Document[T]{}, errors.Join(core.ErrFirestoreDocumentsOperation, err)
	}
	if err := result.Validate(); err != nil {
		return Document[T]{}, err
	}
	return result, nil
}
