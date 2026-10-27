// Package firestoredocuments executes typed document operations through the
// official Firestore SDK. The caller owns account policy and transaction scope.
package firestoredocuments

import (
	"context"
	"errors"
	"reflect"

	"cloud.google.com/go/firestore"
	"github.com/deliri/primitive/v2026/core"
)

// GetRequest selects one native document, optionally within an existing SDK
// transaction. It does not perform a collection query or retain other documents.
type GetRequest struct {
	Reference   *firestore.DocumentRef
	Transaction *firestore.Transaction
}

func (r GetRequest) Validate() error {
	if r.Reference == nil || r.Reference.Parent == nil || r.Reference.ID == "" || r.Reference.Path == "" {
		return core.ErrFirestoreDocumentsContract
	}
	return nil
}

// Document contains one decoded and owner-validated record.
type Document[T core.Validatable] struct {
	Record T
}

func (d Document[T]) Validate() error {
	if reflect.TypeFor[T]().Kind() != reflect.Struct {
		return core.ErrFirestoreDocumentsContract
	}
	if err := d.Record.Validate(); err != nil {
		return errors.Join(core.ErrFirestoreDocumentsContract, err)
	}
	return nil
}

// Get reads exactly one document and releases no partial record on a read,
// native decoding or owning-validation failure. Firestore owns document extent.
func Get[T core.Validatable](ctx context.Context, request GetRequest) (Document[T], error) {
	if err := documentContext(ctx); err != nil {
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
		return Document[T]{}, errors.Join(core.ErrFirestoreDocumentsOperation, err, ctx.Err(), context.Cause(ctx))
	}
	var result Document[T]
	if err := snapshot.DataTo(&result.Record); err != nil {
		return Document[T]{}, errors.Join(core.ErrFirestoreDocumentsOperation, err)
	}
	if err := result.Validate(); err != nil {
		return Document[T]{}, err
	}
	return result, nil
}

// CreateRequest queues one owner-validated record in the caller's transaction.
// A successful Create is not a committed-write receipt.
type CreateRequest[T core.Validatable] struct {
	Reference   *firestore.DocumentRef
	Transaction *firestore.Transaction
	Record      T
}

func (r CreateRequest[T]) Validate() error {
	if (GetRequest{Reference: r.Reference}).Validate() != nil || r.Transaction == nil {
		return core.ErrFirestoreDocumentsContract
	}
	return (Document[T]{Record: r.Record}).Validate()
}

func Create[T core.Validatable](ctx context.Context, request CreateRequest[T]) error {
	if err := documentContext(ctx); err != nil {
		return errors.Join(core.ErrFirestoreDocumentsContract, err)
	}
	if err := request.Validate(); err != nil {
		return err
	}
	if err := request.Transaction.Create(request.Reference, request.Record); err != nil {
		return errors.Join(core.ErrFirestoreDocumentsOperation, err)
	}
	return nil
}

// TransactionRequest delegates retry and commit mechanics to the official SDK.
// Use must use its provided context and transaction and tolerate native retries.
type TransactionRequest struct {
	Client *firestore.Client
	Use    func(context.Context, *firestore.Transaction) error
}

func (r TransactionRequest) Validate() error {
	if r.Client == nil || r.Use == nil {
		return core.ErrFirestoreDocumentsContract
	}
	return nil
}

// TransactionResult is issued only after the native SDK confirms commitment.
// It is an execution fact, not an independent evidence acceptance receipt.
type TransactionResult struct {
	committed bool
}

func (r TransactionResult) Validate() error {
	if !r.committed {
		return core.ErrFirestoreDocumentsContract
	}
	return nil
}

func RunTransaction(ctx context.Context, request TransactionRequest) (TransactionResult, error) {
	if err := documentContext(ctx); err != nil {
		return TransactionResult{}, errors.Join(core.ErrFirestoreDocumentsContract, err)
	}
	if err := request.Validate(); err != nil {
		return TransactionResult{}, err
	}
	if err := request.Client.RunTransaction(ctx, request.Use); err != nil {
		return TransactionResult{}, errors.Join(core.ErrFirestoreDocumentsOperation, err, ctx.Err(), context.Cause(ctx))
	}
	return TransactionResult{committed: true}, nil
}
