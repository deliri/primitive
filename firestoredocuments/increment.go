package firestoredocuments

import (
	"context"
	"errors"
	"unicode/utf8"

	"cloud.google.com/go/firestore"
	"github.com/deliri/primitive/v2026/core"
)

// IncrementRequest applies the official SDK integer transform at one field
// path, optionally in the caller's existing transaction. All int64 deltas,
// including zero, retain native semantics. Missing documents/fields and
// nonnumeric fields become Delta; native integer overflow saturates. Other
// fields outside the path remain intact. Native merge replaces a non-map
// ancestor with the map needed for the path. Application counter policy is
// the caller's concern.
type IncrementRequest struct {
	Reference   *firestore.DocumentRef
	Transaction *firestore.Transaction
	Field       firestore.FieldPath
	Delta       int64
}

func (r IncrementRequest) Validate() error {
	if err := (GetRequest{Reference: r.Reference}).Validate(); err != nil {
		return err
	}
	if len(r.Field) == 0 {
		return core.ErrFirestoreDocumentsContract
	}
	for _, component := range r.Field {
		if component == "" || !utf8.ValidString(component) {
			return core.ErrFirestoreDocumentsContract
		}
	}
	return nil
}

// Increment performs no read. With Transaction it only queues a native merge;
// nil means queued, not committed. The caller owns retries and commit/rollback.
// Without Transaction the SDK owns the single write. An error after submission
// does not establish that a remote write was rolled back.
func Increment(ctx context.Context, request IncrementRequest) error {
	if err := documentContext(ctx); err != nil {
		return errors.Join(core.ErrFirestoreDocumentsContract, err)
	}
	if err := request.Validate(); err != nil {
		return err
	}
	// This SDK-only map represents exactly one native transform. FieldPath
	// components are literal names, including dots; never reinterpret them.
	data := map[string]any{request.Field[len(request.Field)-1]: firestore.Increment(request.Delta)}
	for i := len(request.Field) - 2; i >= 0; i-- {
		data = map[string]any{request.Field[i]: data}
	}
	var err error
	if request.Transaction == nil {
		_, err = request.Reference.Set(ctx, data, firestore.MergeAll)
	} else {
		err = request.Transaction.Set(request.Reference, data, firestore.MergeAll)
	}
	if err := errors.Join(err, documentContext(ctx)); err != nil {
		return errors.Join(core.ErrFirestoreDocumentsOperation, err)
	}
	return nil
}
