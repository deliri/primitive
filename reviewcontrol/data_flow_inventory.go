package reviewcontrol

type protocolFact interface{ reviewControlProtocolFact() }
type sealedProjection interface{ reviewControlSealedProjection() }
type internalFlow interface{ reviewControlInternalFlow() }
type capabilityWrapper interface{ reviewControlCapabilityWrapper() }

func (ReviewIdentity) reviewControlProtocolFact()            {}
func (ContractIdentity) reviewControlProtocolFact()          {}
func (ObservationIdentity) reviewControlProtocolFact()       {}
func (FindingIdentity) reviewControlProtocolFact()           {}
func (ReviewerIdentity) reviewControlProtocolFact()          {}
func (PrincipalIdentity) reviewControlProtocolFact()         {}
func (AuthorityIdentity) reviewControlProtocolFact()         {}
func (ContractTitle) reviewControlProtocolFact()             {}
func (ProblemStatement) reviewControlProtocolFact()          {}
func (CompletionStatement) reviewControlProtocolFact()       {}
func (FindingSummary) reviewControlProtocolFact()            {}
func (FindingDetail) reviewControlProtocolFact()             {}
func (DecisionReason) reviewControlProtocolFact()            {}
func (Subject) reviewControlProtocolFact()                   {}
func (CheckRequirement) reviewControlProtocolFact()          {}
func (ProofRequirement) reviewControlProtocolFact()          {}
func (Contract) reviewControlProtocolFact()                  {}
func (ContextReference) reviewControlProtocolFact()          {}
func (Packet) reviewControlProtocolFact()                    {}
func (Reviewer) reviewControlProtocolFact()                  {}
func (SourceLocation) reviewControlProtocolFact()            {}
func (Finding) reviewControlProtocolFact()                   {}
func (EvidenceReference) reviewControlProtocolFact()         {}
func (Observation) reviewControlProtocolFact()               {}
func (HumanAuthorityClaim) reviewControlProtocolFact()       {}
func (AuthorityReference) reviewControlProtocolFact()        {}
func (DecisionIntent) reviewControlProtocolFact()            {}
func (DecisionRecord) reviewControlProtocolFact()            {}
func (DecisionSuperseded) reviewControlProtocolFact()        {}
func (EventPayload) reviewControlProtocolFact()              {}
func (IssueReviewRequest) reviewControlProtocolFact()        {}
func (IssueReviewResponse) reviewControlProtocolFact()       {}
func (ReadReviewRequest) reviewControlProtocolFact()         {}
func (ReadReviewResponse) reviewControlProtocolFact()        {}
func (RecordObservationRequest) reviewControlProtocolFact()  {}
func (RecordObservationResponse) reviewControlProtocolFact() {}
func (RecordDecisionRequest) reviewControlProtocolFact()     {}
func (RecordDecisionResponse) reviewControlProtocolFact()    {}
func (ReadEventsRequest) reviewControlProtocolFact()         {}
func (ReadEventsResponse) reviewControlProtocolFact()        {}
func (ReadProjectionRequest) reviewControlProtocolFact()     {}
func (ReadProjectionResponse) reviewControlProtocolFact()    {}

func (Projection) reviewControlSealedProjection() {}

func (uuidIdentity) reviewControlInternalFlow()                  {}
func (boundedText) reviewControlInternalFlow()                   {}
func (VerifiedHumanAuthority) reviewControlInternalFlow()        {}
func (DecisionValidation) reviewControlInternalFlow()            {}
func (Fold) reviewControlInternalFlow()                          {}
func (issueReviewRequestWire) reviewControlInternalFlow()        {}
func (issueReviewResponseWire) reviewControlInternalFlow()       {}
func (readReviewRequestWire) reviewControlInternalFlow()         {}
func (readReviewResponseWire) reviewControlInternalFlow()        {}
func (recordObservationRequestWire) reviewControlInternalFlow()  {}
func (recordObservationResponseWire) reviewControlInternalFlow() {}
func (recordDecisionRequestWire) reviewControlInternalFlow()     {}
func (recordDecisionResponseWire) reviewControlInternalFlow()    {}
func (readEventsRequestWire) reviewControlInternalFlow()         {}
func (readEventsResponseWire) reviewControlInternalFlow()        {}
func (readProjectionRequestWire) reviewControlInternalFlow()     {}
func (readProjectionResponseWire) reviewControlInternalFlow()    {}

func (SocketResult[Response]) reviewControlInternalFlow()          {}
func (ReadSocketClientConfiguration) reviewControlInternalFlow()   {}
func (IssuerSocketClient) reviewControlCapabilityWrapper()         {}
func (ObservationSocketClient) reviewControlCapabilityWrapper()    {}
func (DecisionSocketClient) reviewControlCapabilityWrapper()       {}
func (ReadSocketClient) reviewControlCapabilityWrapper()           {}
func (IssueSocketServer) reviewControlCapabilityWrapper()          {}
func (ReviewReadSocketServer) reviewControlCapabilityWrapper()     {}
func (ObservationSocketServer) reviewControlCapabilityWrapper()    {}
func (DecisionSocketServer) reviewControlCapabilityWrapper()       {}
func (EventReadSocketServer) reviewControlCapabilityWrapper()      {}
func (ProjectionReadSocketServer) reviewControlCapabilityWrapper() {}

var (
	_ protocolFact      = Packet{}
	_ sealedProjection  = Projection{}
	_ internalFlow      = Fold{}
	_ capabilityWrapper = IssuerSocketClient{}
)
