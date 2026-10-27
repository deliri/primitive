package firestoredocuments

import (
	"context"
	"errors"

	"cloud.google.com/go/firestore"
	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// SetRequest replaces one document in the caller's native transaction. It does
// not merge fields, commit independently or authorize the document's owner.
type SetRequest[T core.Validatable] struct {
	Reference   *firestore.DocumentRef
	Transaction *firestore.Transaction
	Record      T
}

func (r SetRequest[T]) Validate() error {
	if (GetRequest{Reference: r.Reference}).Validate() != nil || r.Transaction == nil {
		return core.ErrFirestoreDocumentsContract
	}
	return (Document[T]{Record: r.Record}).Validate()
}

// Set queues a full replacement. Only successful RunTransaction confirms commit.
func Set[T core.Validatable](ctx context.Context, request SetRequest[T]) error {
	if err := contextstate.Validate(ctx); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return errors.Join(core.ErrFirestoreDocumentsContract, err, context.Cause(ctx))
		}
		return errors.Join(core.ErrFirestoreDocumentsContract, err)
	}
	if err := request.Validate(); err != nil {
		return err
	}
	if err := request.Transaction.Set(request.Reference, request.Record); err != nil {
		return errors.Join(core.ErrFirestoreDocumentsOperation, err)
	}
	return nil
}

func (SetRequest[T]) firestoreDocumentsProtocolFact() {}
