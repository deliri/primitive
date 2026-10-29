package firestoredocuments

import (
	"context"
	"errors"
	"reflect"

	"cloud.google.com/go/firestore"
	"github.com/deliri/primitive/v2026/core"
	"google.golang.org/api/iterator"
)

// QueryVisitDecision controls only this forward native iterator, not query policy.
type QueryVisitDecision uint8

const (
	QueryContinue QueryVisitDecision = iota + 1
	QueryStop
)

func (d QueryVisitDecision) Validate() error {
	switch d {
	case QueryContinue, QueryStop:
		return nil
	default:
		return core.ErrFirestoreDocumentsContract
	}
}

func (QueryVisitDecision) OffWireEnum() {}

// QueryRequest borrows a caller-created SDK query. The caller owns selection,
// authorization, ordering and cursor policy. Visit receives only decoded,
// owner-validated structs, one at a time; earlier visits remain provisional if a
// later read fails. A visitor refusal never becomes a successful stop.
type QueryRequest[T core.Validatable] struct {
	Query firestore.Query
	Visit func(context.Context, Document[T]) (QueryVisitDecision, error)
}

func (r QueryRequest[T]) Validate() error {
	if r.Visit == nil || reflect.TypeFor[T]().Kind() != reflect.Struct {
		return core.ErrFirestoreDocumentsContract
	}
	if _, err := r.Query.Serialize(); err != nil {
		return errors.Join(core.ErrFirestoreDocumentsContract, err)
	}
	return nil
}

// Query streams without accumulating results and always stops its owned SDK
// iterator. Exhaustion and an explicit visitor stop succeed only when no error
// or cancellation is observed at that boundary. This is not a cancellation
// linearization guarantee: cancellation can occur after the final observation.
func Query[T core.Validatable](ctx context.Context, request QueryRequest[T]) error {
	if err := documentContext(ctx); err != nil {
		return errors.Join(core.ErrFirestoreDocumentsContract, err)
	}
	if err := request.Validate(); err != nil {
		return err
	}
	documents := request.Query.Documents(ctx)
	defer documents.Stop()
	for {
		if err := documentContext(ctx); err != nil {
			return errors.Join(core.ErrFirestoreDocumentsOperation, err)
		}
		snapshot, err := documents.Next()
		if err == iterator.Done {
			if err := documentContext(ctx); err != nil {
				return errors.Join(core.ErrFirestoreDocumentsOperation, err)
			}
			return nil
		}
		if err != nil {
			return errors.Join(core.ErrFirestoreDocumentsOperation, err, documentContext(ctx))
		}
		var document Document[T]
		if err := snapshot.DataTo(&document.Record); err != nil {
			return errors.Join(core.ErrFirestoreDocumentsOperation, err, documentContext(ctx))
		}
		if err := document.Validate(); err != nil {
			return errors.Join(err, documentContext(ctx))
		}
		if err := documentContext(ctx); err != nil {
			return errors.Join(core.ErrFirestoreDocumentsOperation, err)
		}
		decision, visitErr := request.Visit(ctx, document)
		if err := errors.Join(decision.Validate(), visitErr, documentContext(ctx)); err != nil {
			return errors.Join(core.ErrFirestoreDocumentsOperation, err)
		}
		if decision == QueryStop {
			return nil
		}
	}
}
