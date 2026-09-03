package sourceclaim

// The marker interfaces classify every production struct in this
// trust-boundary package. The architecture ratchet requires one role per
// struct so new carriers cannot enter the claim agreement invisibly.
type protocolFact interface{ sourceClaimProtocolFact() }
type sealedProjection interface{ sourceClaimSealedProjection() }
type internalFlow interface{ sourceClaimInternalFlow() }

func (ID) sourceClaimProtocolFact()                   {}
func (Text) sourceClaimProtocolFact()                 {}
func (Reference) sourceClaimProtocolFact()            {}
func (Boundary) sourceClaimProtocolFact()             {}
func (Narrative) sourceClaimProtocolFact()            {}
func (ExecutionRequirement) sourceClaimProtocolFact() {}
func (CompilerRequirement) sourceClaimProtocolFact()  {}
func (Requirement) sourceClaimProtocolFact()          {}
func (Claim) sourceClaimProtocolFact()                {}

func (Summary) sourceClaimSealedProjection()   {}
func (claimConsumer) sourceClaimInternalFlow() {}

var (
	_ protocolFact     = Claim{}
	_ sealedProjection = Summary{}
	_ internalFlow     = claimConsumer{}
)
