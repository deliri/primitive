package firestoredocuments

type protocolFact interface{ firestoreDocumentsProtocolFact() }
type sealedProjection interface{ firestoreDocumentsSealedProjection() }

func (GetRequest) firestoreDocumentsProtocolFact()            {}
func (CreateRequest[T]) firestoreDocumentsProtocolFact()      {}
func (TransactionRequest) firestoreDocumentsProtocolFact()    {}
func (Document[T]) firestoreDocumentsSealedProjection()       {}
func (TransactionResult) firestoreDocumentsSealedProjection() {}
