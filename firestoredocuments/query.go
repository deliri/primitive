package firestoredocuments

import (
	"cloud.google.com/go/firestore/apiv1/firestorepb"
	"context"
	"errors"
	"google.golang.org/protobuf/proto"
	"reflect"
	"strings"

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

// validatedQueryParent observes native query structure without executing or
// reconstructing it. The original query keeps its in-process SDK options.
func (r QueryRequest[T]) validatedQueryParent() (string, error) {
	if r.Visit == nil || reflect.TypeFor[T]().Kind() != reflect.Struct {
		return "", core.ErrFirestoreDocumentsContract
	}
	// Firestore v1.28.0 exposes no client accessor. Serialize admits a query
	// deserialized on a zero receiver, but Documents then dereferences nil c.
	// Observe only that pinned native slot; a layout change fails closed.
	// Borrow the original query unchanged so native read options survive.
	client := reflect.ValueOf(r.Query).FieldByName("c")
	if !client.IsValid() || client.Type() != reflect.TypeFor[*firestore.Client]() || client.IsNil() {
		return "", core.ErrFirestoreDocumentsContract
	}
	encoded, err := r.Query.Serialize()
	if err != nil {
		return "", errors.Join(core.ErrFirestoreDocumentsContract, err)
	}
	var native firestorepb.RunQueryRequest
	if err := proto.Unmarshal(encoded, &native); err != nil {
		return "", errors.Join(core.ErrFirestoreDocumentsContract, err)
	}
	return native.Parent, nil
}

func (r QueryRequest[T]) Validate() error {
	_, err := r.validatedQueryParent()
	return err
}

// ValidateClient verifies native project/database resource membership only.
// Caller policy still owns tenant/role authorization and query selection.
func (r QueryRequest[T]) ValidateClient(client *firestore.Client) error {
	if client == nil {
		return core.ErrFirestoreDocumentsContract
	}
	parent, err := r.validatedQueryParent()
	if err != nil {
		return err
	}
	const collection = "primitive-query-binding"
	root := strings.TrimSuffix(client.Collection(collection).Path, "/"+collection)
	if parent != root && !strings.HasPrefix(parent, root+"/") {
		return core.ErrFirestoreDocumentsContract
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
		if visitErr != nil {
			return errors.Join(core.ErrFirestoreDocumentsOperation, visitErr, documentContext(ctx))
		}
		if err := errors.Join(decision.Validate(), documentContext(ctx)); err != nil {
			return errors.Join(core.ErrFirestoreDocumentsOperation, err)
		}
		if decision == QueryStop {
			return nil
		}
	}
}

func (d QueryVisitDecision) IsValid() bool { return d.Validate() == nil }
func (d QueryVisitDecision) String() string {
	switch d {
	case QueryContinue:
		return "continue"
	case QueryStop:
		return "stop"
	default:
		return "unknown"
	}
}
