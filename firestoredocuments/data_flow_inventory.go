package firestoredocuments

import "github.com/deliri/primitive/v2026/core"

var _ core.OffWireEnum = TransactionReadWrite

type protocolFact interface{ firestoreDocumentsProtocolFact() }
type sealedProjection interface{ firestoreDocumentsSealedProjection() }

func (GetRequest) firestoreDocumentsProtocolFact()            {}
func (CreateRequest[T]) firestoreDocumentsProtocolFact()      {}
func (TransactionRequest) firestoreDocumentsProtocolFact()    {}
func (Document[T]) firestoreDocumentsSealedProjection()       {}
func (TransactionResult) firestoreDocumentsSealedProjection() {}
