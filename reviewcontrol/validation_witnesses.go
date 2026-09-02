package reviewcontrol

import "github.com/deliri/primitive/v2026/core"

var (
	_ core.ValidatedJSONMarshaler = ReviewIdentity{}
	_ core.ValidatedJSONMarshaler = ContractIdentity{}
	_ core.ValidatedJSONMarshaler = ObservationIdentity{}
	_ core.ValidatedJSONMarshaler = FindingIdentity{}
	_ core.ValidatedJSONMarshaler = ReviewerIdentity{}
	_ core.ValidatedJSONMarshaler = PrincipalIdentity{}
	_ core.ValidatedJSONMarshaler = AuthorityIdentity{}
	_ core.ValidatedJSONMarshaler = ContractTitle{}
	_ core.ValidatedJSONMarshaler = ProblemStatement{}
	_ core.ValidatedJSONMarshaler = CompletionStatement{}
	_ core.ValidatedJSONMarshaler = FindingSummary{}
	_ core.ValidatedJSONMarshaler = FindingDetail{}
	_ core.ValidatedJSONMarshaler = DecisionReason{}
	_ core.ValidatedJSONMarshaler = ReviewerKind(0)
	_ core.ValidatedJSONMarshaler = Verdict(0)
	_ core.ValidatedJSONMarshaler = FindingSeverity(0)
	_ core.ValidatedJSONMarshaler = CheckKind(0)
	_ core.ValidatedJSONMarshaler = ProofKind(0)
	_ core.ValidatedJSONMarshaler = DecisionKind(0)
	_ core.ValidatedJSONMarshaler = EventKind(0)
	_ core.ValidatedJSONMarshaler = AuthorityKind(0)
	_ core.ValidatedJSONMarshaler = HumanAuthoritySigningDomain(0)
	_ core.ValidatedJSONMarshaler = Operation(0)
	_ core.ValidatedJSONMarshaler = HumanAuthorityClaim{}
	_ core.ValidatedJSONMarshaler = Packet{}
	_ core.ValidatedJSONMarshaler = Observation{}
	_ core.ValidatedJSONMarshaler = DecisionIntent{}
	_ core.ValidatedJSONMarshaler = EventPayload{}
	_ core.ValidatedJSONMarshaler = IssueReviewRequest{}
	_ core.ValidatedJSONMarshaler = IssueReviewResponse{}
	_ core.ValidatedJSONMarshaler = ReadReviewRequest{}
	_ core.ValidatedJSONMarshaler = ReadReviewResponse{}
	_ core.ValidatedJSONMarshaler = RecordObservationRequest{}
	_ core.ValidatedJSONMarshaler = RecordObservationResponse{}
	_ core.ValidatedJSONMarshaler = RecordDecisionRequest{}
	_ core.ValidatedJSONMarshaler = RecordDecisionResponse{}
	_ core.ValidatedJSONMarshaler = ReadEventsRequest{}
	_ core.ValidatedJSONMarshaler = ReadEventsResponse{}
	_ core.ValidatedJSONMarshaler = ReadProjectionRequest{}
	_ core.ValidatedJSONMarshaler = ReadProjectionResponse{}
)
