package reviewcontrol

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

const (
	reviewLeafDoorReviewIdentity uint8 = iota + 1
	reviewLeafDoorContractIdentity
	reviewLeafDoorObservationIdentity
	reviewLeafDoorFindingIdentity
	reviewLeafDoorReviewerIdentity
	reviewLeafDoorPrincipalIdentity
	reviewLeafDoorAuthorityIdentity
	reviewLeafDoorContractTitle
	reviewLeafDoorProblemStatement
	reviewLeafDoorCompletionStatement
	reviewLeafDoorFindingSummary
	reviewLeafDoorFindingDetail
	reviewLeafDoorDecisionReason
	reviewLeafDoorReviewerKind
	reviewLeafDoorVerdict
	reviewLeafDoorSeverity
	reviewLeafDoorCheckKind
	reviewLeafDoorProofKind
	reviewLeafDoorDecisionKind
	reviewLeafDoorEventKind
	reviewLeafDoorAuthorityKind
	reviewLeafDoorOperation
	reviewLeafDoorHumanAuthorityDomain
	reviewLeafDoorHumanAuthorityClaim
	reviewLeafDoorDecisionIntent
)

func addReviewLeafSeed[D core.ValidatedJSONMarshaler](f *testing.F, door uint8, value D) {
	f.Helper()
	encoded, err := value.MarshalJSON()
	if err != nil {
		f.Fatalf("review leaf door %d MarshalJSON(seed) error = %v, want nil", door, err)
	}
	f.Add(door, encoded)
}

func proveReviewLeafClosure[D interface {
	comparable
	core.ValidatedJSONMarshaler
}, P interface {
	*D
	json.Unmarshaler
}](t *testing.T, data []byte, admitted D) {
	t.Helper()
	want, err := admitted.MarshalJSON()
	if err != nil {
		t.Fatalf("review leaf admitted MarshalJSON() error = %v, want nil", err)
	}
	got := admitted
	gotErr := P(&got).UnmarshalJSON(data)
	if gotErr != nil {
		if !errors.Is(gotErr, core.ErrJSONContract) || got != admitted {
			t.Fatalf("review leaf UnmarshalJSON(rejected) = (%+v, %v), want preserved %+v and %v", got, gotErr, admitted, core.ErrJSONContract)
		}
		return
	}
	encoded, err := got.MarshalJSON()
	if err != nil {
		t.Fatalf("review leaf MarshalJSON(accepted) error = %v, want nil", err)
	}
	var roundTrip D
	if err := P(&roundTrip).UnmarshalJSON(encoded); err != nil {
		t.Fatalf("review leaf UnmarshalJSON(round trip) error = %v, want nil", err)
	}
	second, err := roundTrip.MarshalJSON()
	if err != nil || !bytes.Equal(second, encoded) {
		t.Fatalf("review leaf canonical closure = (%q, %v), want (%q, nil); admitted=%q", second, err, encoded, want)
	}
}

func FuzzReviewControlExternalLeafJSONDoorsSemanticClosure(f *testing.F) {
	packet := reviewPacket(f)
	observation := reviewObservation(f, packet)
	intent := reviewDecision(f, packet, observation, DecisionAccept)
	finding, _ := NewFindingIdentity(reviewUUID(f, 6))
	reviewer, _ := NewReviewerIdentity(reviewUUID(f, 3))
	principal, _ := NewPrincipalIdentity(reviewUUID(f, 4))
	authority, _ := NewAuthorityIdentity(reviewUUID(f, 5))
	title, _ := NewContractTitle("Exact source")
	problem, _ := NewProblemStatement("A stale decision could migrate.")
	completion, _ := NewCompletionStatement("Exact bytes remain bound.")
	summary, _ := NewFindingSummary("Stale source")
	detail, _ := NewFindingDetail("The digest differs from the reviewed source.")
	reason, _ := NewDecisionReason("Exact source was reviewed.")
	claim := HumanAuthorityClaim{Principal: principal, Authority: authority, Kind: AuthorityHuman}

	addReviewLeafSeed(f, reviewLeafDoorReviewIdentity, packet.Identity)
	addReviewLeafSeed(f, reviewLeafDoorContractIdentity, packet.Contract.Identity)
	addReviewLeafSeed(f, reviewLeafDoorObservationIdentity, observation.Identity)
	addReviewLeafSeed(f, reviewLeafDoorFindingIdentity, finding)
	addReviewLeafSeed(f, reviewLeafDoorReviewerIdentity, reviewer)
	addReviewLeafSeed(f, reviewLeafDoorPrincipalIdentity, principal)
	addReviewLeafSeed(f, reviewLeafDoorAuthorityIdentity, authority)
	addReviewLeafSeed(f, reviewLeafDoorContractTitle, title)
	addReviewLeafSeed(f, reviewLeafDoorProblemStatement, problem)
	addReviewLeafSeed(f, reviewLeafDoorCompletionStatement, completion)
	addReviewLeafSeed(f, reviewLeafDoorFindingSummary, summary)
	addReviewLeafSeed(f, reviewLeafDoorFindingDetail, detail)
	addReviewLeafSeed(f, reviewLeafDoorDecisionReason, reason)
	addReviewLeafSeed(f, reviewLeafDoorReviewerKind, ReviewerAgent)
	addReviewLeafSeed(f, reviewLeafDoorVerdict, VerdictPass)
	addReviewLeafSeed(f, reviewLeafDoorSeverity, SeverityBlocking)
	addReviewLeafSeed(f, reviewLeafDoorCheckKind, CheckManualInspection)
	addReviewLeafSeed(f, reviewLeafDoorProofKind, ProofMachineObservation)
	addReviewLeafSeed(f, reviewLeafDoorDecisionKind, DecisionAccept)
	addReviewLeafSeed(f, reviewLeafDoorEventKind, EventReviewIssued)
	addReviewLeafSeed(f, reviewLeafDoorAuthorityKind, AuthorityHuman)
	addReviewLeafSeed(f, reviewLeafDoorOperation, OperationIssueReview)
	addReviewLeafSeed(f, reviewLeafDoorHumanAuthorityDomain, HumanAuthoritySigningDomainV1)
	addReviewLeafSeed(f, reviewLeafDoorHumanAuthorityClaim, claim)
	addReviewLeafSeed(f, reviewLeafDoorDecisionIntent, intent)
	f.Add(reviewLeafDoorVerdict, []byte{})
	f.Add(reviewLeafDoorDecisionIntent, []byte(`{}`))

	f.Fuzz(func(t *testing.T, door uint8, data []byte) {
		switch door {
		case reviewLeafDoorReviewIdentity:
			proveReviewLeafClosure[ReviewIdentity, *ReviewIdentity](t, data, packet.Identity)
		case reviewLeafDoorContractIdentity:
			proveReviewLeafClosure[ContractIdentity, *ContractIdentity](t, data, packet.Contract.Identity)
		case reviewLeafDoorObservationIdentity:
			proveReviewLeafClosure[ObservationIdentity, *ObservationIdentity](t, data, observation.Identity)
		case reviewLeafDoorFindingIdentity:
			proveReviewLeafClosure[FindingIdentity, *FindingIdentity](t, data, finding)
		case reviewLeafDoorReviewerIdentity:
			proveReviewLeafClosure[ReviewerIdentity, *ReviewerIdentity](t, data, reviewer)
		case reviewLeafDoorPrincipalIdentity:
			proveReviewLeafClosure[PrincipalIdentity, *PrincipalIdentity](t, data, principal)
		case reviewLeafDoorAuthorityIdentity:
			proveReviewLeafClosure[AuthorityIdentity, *AuthorityIdentity](t, data, authority)
		case reviewLeafDoorContractTitle:
			proveReviewLeafClosure[ContractTitle, *ContractTitle](t, data, title)
		case reviewLeafDoorProblemStatement:
			proveReviewLeafClosure[ProblemStatement, *ProblemStatement](t, data, problem)
		case reviewLeafDoorCompletionStatement:
			proveReviewLeafClosure[CompletionStatement, *CompletionStatement](t, data, completion)
		case reviewLeafDoorFindingSummary:
			proveReviewLeafClosure[FindingSummary, *FindingSummary](t, data, summary)
		case reviewLeafDoorFindingDetail:
			proveReviewLeafClosure[FindingDetail, *FindingDetail](t, data, detail)
		case reviewLeafDoorDecisionReason:
			proveReviewLeafClosure[DecisionReason, *DecisionReason](t, data, reason)
		case reviewLeafDoorReviewerKind:
			proveReviewLeafClosure[ReviewerKind, *ReviewerKind](t, data, ReviewerAgent)
		case reviewLeafDoorVerdict:
			proveReviewLeafClosure[Verdict, *Verdict](t, data, VerdictPass)
		case reviewLeafDoorSeverity:
			proveReviewLeafClosure[FindingSeverity, *FindingSeverity](t, data, SeverityBlocking)
		case reviewLeafDoorCheckKind:
			proveReviewLeafClosure[CheckKind, *CheckKind](t, data, CheckManualInspection)
		case reviewLeafDoorProofKind:
			proveReviewLeafClosure[ProofKind, *ProofKind](t, data, ProofMachineObservation)
		case reviewLeafDoorDecisionKind:
			proveReviewLeafClosure[DecisionKind, *DecisionKind](t, data, DecisionAccept)
		case reviewLeafDoorEventKind:
			proveReviewLeafClosure[EventKind, *EventKind](t, data, EventReviewIssued)
		case reviewLeafDoorAuthorityKind:
			proveReviewLeafClosure[AuthorityKind, *AuthorityKind](t, data, AuthorityHuman)
		case reviewLeafDoorOperation:
			proveReviewLeafClosure[Operation, *Operation](t, data, OperationIssueReview)
		case reviewLeafDoorHumanAuthorityDomain:
			proveReviewLeafClosure[HumanAuthoritySigningDomain, *HumanAuthoritySigningDomain](t, data, HumanAuthoritySigningDomainV1)
		case reviewLeafDoorHumanAuthorityClaim:
			proveReviewLeafClosure[HumanAuthorityClaim, *HumanAuthorityClaim](t, data, claim)
		case reviewLeafDoorDecisionIntent:
			proveReviewLeafClosure[DecisionIntent, *DecisionIntent](t, data, intent)
		}
	})
}
