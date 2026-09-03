package sourceproof

// Source proof carries typed evidence references and seals per-claim results
// and their lossless accounting separately.
type protocolFact interface{ sourceProofProtocolFact() }
type sealedProjection interface{ sourceProofSealedProjection() }
type internalFlow interface{ sourceProofInternalFlow() }

func (EvidenceReference) sourceProofProtocolFact() {}
func (RequirementResult) sourceProofProtocolFact() {}

func (Result) sourceProofSealedProjection()    {}
func (Summary) sourceProofSealedProjection()   {}
func (claimVerifier) sourceProofInternalFlow() {}

var (
	_ protocolFact     = EvidenceReference{}
	_ sealedProjection = Result{}
	_ internalFlow     = claimVerifier{}
)
