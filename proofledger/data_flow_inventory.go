package proofledger

type protocolFact interface{ proofLedgerProtocolFact() }
type internalFlow interface{ proofLedgerInternalFlow() }

func (LedgerIdentity) proofLedgerProtocolFact()        {}
func (EventIdentity) proofLedgerProtocolFact()         {}
func (PageLimit) proofLedgerProtocolFact()             {}
func (Head) proofLedgerProtocolFact()                  {}
func (AppendIntent[P]) proofLedgerProtocolFact()       {}
func (Envelope[P]) proofLedgerProtocolFact()           {}
func (AppendReceipt) proofLedgerProtocolFact()         {}
func (AppendReceiptDocument) proofLedgerProtocolFact() {}
func (ResolveRequest) proofLedgerProtocolFact()        {}
func (PageRequest) proofLedgerProtocolFact()           {}
func (Page[P]) proofLedgerProtocolFact()               {}

func (Issue[P]) proofLedgerInternalFlow()                     {}
func (eventCommitment[P]) proofLedgerInternalFlow()           {}
func (Verifier[P]) proofLedgerInternalFlow()                  {}
func (AppendReceiptIssuance[P]) proofLedgerInternalFlow()     {}
func (AppendReceiptVerification[P]) proofLedgerInternalFlow() {}
func (VerifiedAppendReceipt) proofLedgerInternalFlow()        {}

var (
	_ protocolFact = LedgerIdentity{}
	_ internalFlow = Issue[AppendReceipt]{}
)
