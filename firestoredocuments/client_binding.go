package firestoredocuments

import (
	"cloud.google.com/go/firestore"
	"github.com/deliri/primitive/v2026/core"
)

// ClientBinding borrows a native client for request admission. The binding
// cannot close the client. Its owner must keep the client live while in use.
type ClientBinding struct{ client *firestore.Client }

func BindClient(client *firestore.Client) ClientBinding { return ClientBinding{client: client} }

func (b ClientBinding) Validate() error {
	if b.client == nil {
		return core.ErrFirestoreDocumentsContract
	}
	return nil
}

// ValidateReference checks database membership. The SDK does not expose the
// client that owns a borrowed transaction, which the caller must bind itself.
func (b ClientBinding) ValidateReference(reference *firestore.DocumentRef) error {
	if b.Validate() != nil || reference == nil || reference.Parent == nil {
		return core.ErrFirestoreDocumentsContract
	}
	top := reference.Parent
	for top.Parent != nil {
		if top.Parent.Parent == nil {
			return core.ErrFirestoreDocumentsContract
		}
		top = top.Parent.Parent
	}
	if top.ID == "" {
		return core.ErrFirestoreDocumentsContract
	}
	if bound := b.client.Collection(top.ID); bound == nil || bound.Path != top.Path {
		return core.ErrFirestoreDocumentsContract
	}
	return nil
}

// ValidateBinding checks which client owns the transaction request.
func (r TransactionRequest) ValidateBinding(binding ClientBinding) error {
	if binding.Validate() != nil || r.Client != binding.client {
		return core.ErrFirestoreDocumentsContract
	}
	return nil
}

func (r QueryRequest[T]) ValidateBinding(binding ClientBinding) error {
	return r.ValidateClient(binding.client)
}
