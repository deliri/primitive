package firestoredocuments

import (
	firestoreapi "cloud.google.com/go/firestore/apiv1"
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
func (r QueryRequest[T]) validatedQueryParent(bound *firestore.Client) (string, error) {
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
	if bound != nil && client.Pointer() != reflect.ValueOf(bound).Pointer() {
		return "", core.ErrFirestoreDocumentsContract
	}
	// The pinned client owns its native RPC capability in c. A zero Client
	// has the query pointer above but no capability; a closed initialized
	// client retains this slot and reports its ordinary native transport error.
	rpcClient := client.Elem().FieldByName("c")
	if !rpcClient.IsValid() || rpcClient.Type() != reflect.TypeFor[*firestoreapi.Client]() || rpcClient.IsNil() {
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
	_, err := r.validatedQueryParent(nil)
	return err
}

// ValidateClient binds the exact executing client and its native project/database.
// Caller policy still owns tenant/role authorization and query selection.
func (r QueryRequest[T]) ValidateClient(client *firestore.Client) error {
	if client == nil {
		return core.ErrFirestoreDocumentsContract
	}
	parent, err := r.validatedQueryParent(client)
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
	return query(ctx, request, func(_ context.Context, snapshot *firestore.DocumentSnapshot) (Document[T], error) {
		var document Document[T]
		if err := snapshot.DataTo(&document.Record); err != nil {
			return Document[T]{}, errors.Join(core.ErrFirestoreDocumentsOperation, err)
		}
		return document, nil
	})
}

// QueryIntegerJSON streams exactly like Query but decodes each row through the
// same canonical integer JSON projection as GetIntegerJSON, so JSON-shaped
// documents (unsigned members, text identities, nested records) are read by
// query with the point read's admission: unknown members and native doubles
// refuse the row and the query, and nothing is visited for that row.
func QueryIntegerJSON[T core.Validatable](ctx context.Context, request QueryRequest[T]) error {
	return query(ctx, request, func(ctx context.Context, snapshot *firestore.DocumentSnapshot) (Document[T], error) {
		projected, err := projectJSON[T](ctx, snapshot, true)
		if err != nil {
			return Document[T]{}, err
		}
		return Document[T]{Record: projected.Record}, nil
	})
}

func query[T core.Validatable](ctx context.Context, request QueryRequest[T], decode func(context.Context, *firestore.DocumentSnapshot) (Document[T], error)) error {
	if err := documentContext(ctx); err != nil {
		return errors.Join(core.ErrFirestoreDocumentsContract, err)
	}
	parent, err := request.validatedQueryParent(nil)
	if err != nil {
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
		// The SDK derives Ref from the returned row name without binding it.
		// Admit only a native document reference inside this query's parent.
		if (GetRequest{Reference: snapshot.Ref}).Validate() != nil || !strings.HasPrefix(snapshot.Ref.Path, parent+"/") {
			return errors.Join(core.ErrFirestoreDocumentsOperation, documentContext(ctx))
		}
		document, err := decode(ctx, snapshot)
		if err != nil {
			return errors.Join(err, documentContext(ctx))
		}
		document.Reference = snapshot.Ref
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
