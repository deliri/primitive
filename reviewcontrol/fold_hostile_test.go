package reviewcontrol

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/proofledger"
)

type foldFixture struct {
	packet      Packet
	observation Observation
	record      DecisionRecord
	genesis     proofledger.Head
	issued      proofledger.Envelope[EventPayload]
	observed    proofledger.Envelope[EventPayload]
	accepted    proofledger.Envelope[EventPayload]
}

func newFoldFixture(t testing.TB) foldFixture {
	t.Helper()
	packet := reviewPacket(t)
	observation := reviewObservation(t, packet)
	authority := verifiedHuman(t, AuthorityHuman)
	record, err := NewDecisionRecord(authority, reviewDecision(t, packet, observation, DecisionAccept))
	if err != nil {
		t.Fatalf("NewDecisionRecord() error = %v, want nil", err)
	}
	genesis, err := proofledger.NewGenesisHead(proofLedgerIdentity(t))
	if err != nil {
		t.Fatalf("NewGenesisHead() error = %v, want nil", err)
	}
	issued := reviewEvent(t, genesis, 0, EventPayload{Kind: EventReviewIssued, Review: &packet})
	observed := reviewEvent(t, issued.Head(), 1, EventPayload{Kind: EventObservationRecorded, Observation: &observation})
	accepted := reviewEvent(t, observed.Head(), 2, EventPayload{Kind: EventHumanAccepted, Decision: &record})
	return foldFixture{packet: packet, observation: observation, record: record, genesis: genesis, issued: issued, observed: observed, accepted: accepted}
}

func observeFold(events []proofledger.Envelope[EventPayload]) (Projection, error) {
	var fold Fold
	for index := range events {
		if err := fold.Observe(events[index]); err != nil {
			return Projection{}, err
		}
	}
	return fold.Projection()
}

func TestReviewFoldRejectsContradictoryAndOutOfOrderReviewFacts(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		build   func(testing.TB) []proofledger.Envelope[EventPayload]
		wantErr error
	}{
		{name: "observation before review issuance", build: func(t testing.TB) []proofledger.Envelope[EventPayload] {
			f := newFoldFixture(t)
			return []proofledger.Envelope[EventPayload]{reviewEvent(t, f.genesis, 0, EventPayload{Kind: EventObservationRecorded, Observation: &f.observation})}
		}, wantErr: core.ErrReviewControlSubjectMismatch},
		{name: "decision before review issuance", build: func(t testing.TB) []proofledger.Envelope[EventPayload] {
			f := newFoldFixture(t)
			return []proofledger.Envelope[EventPayload]{reviewEvent(t, f.genesis, 0, EventPayload{Kind: EventHumanAccepted, Decision: &f.record})}
		}, wantErr: core.ErrReviewControlSubjectMismatch},
		{name: "supersession before review issuance", build: func(t testing.TB) []proofledger.Envelope[EventPayload] {
			f := newFoldFixture(t)
			reason, _ := NewDecisionReason("No decision exists yet.")
			supersession := DecisionSuperseded{Previous: f.issued.Event, Reason: reason}
			return []proofledger.Envelope[EventPayload]{reviewEvent(t, f.genesis, 0, EventPayload{Kind: EventDecisionSuperseded, Supersession: &supersession})}
		}, wantErr: core.ErrReviewControlSubjectMismatch},
		{name: "second review issuance cannot replace first", build: func(t testing.TB) []proofledger.Envelope[EventPayload] {
			f := newFoldFixture(t)
			duplicate := reviewEvent(t, f.issued.Head(), 1, EventPayload{Kind: EventReviewIssued, Review: &f.packet})
			return []proofledger.Envelope[EventPayload]{f.issued, duplicate}
		}, wantErr: core.ErrReviewControlSubjectMismatch},
		{name: "observation for another review", build: func(t testing.TB) []proofledger.Envelope[EventPayload] {
			f := newFoldFixture(t)
			other, _ := NewReviewIdentity(reviewUUID(t, 7))
			changed := f.observation
			changed.Review = other
			event := reviewEvent(t, f.issued.Head(), 1, EventPayload{Kind: EventObservationRecorded, Observation: &changed})
			return []proofledger.Envelope[EventPayload]{f.issued, event}
		}, wantErr: core.ErrReviewControlSubjectMismatch},
		{name: "observation for another source subject", build: func(t testing.TB) []proofledger.Envelope[EventPayload] {
			f := newFoldFixture(t)
			changed := f.observation
			changed.Subject.Project.Project = core.Offering{Token: "other"}
			event := reviewEvent(t, f.issued.Head(), 1, EventPayload{Kind: EventObservationRecorded, Observation: &changed})
			return []proofledger.Envelope[EventPayload]{f.issued, event}
		}, wantErr: core.ErrReviewControlSubjectMismatch},
		{name: "observation for stale bytes retains stale-source identity", build: func(t testing.TB) []proofledger.Envelope[EventPayload] {
			f := newFoldFixture(t)
			changed := f.observation
			changed.Subject.SHA256 = core.SHA256Of([]byte("stale observation bytes"))
			event := reviewEvent(t, f.issued.Head(), 1, EventPayload{Kind: EventObservationRecorded, Observation: &changed})
			return []proofledger.Envelope[EventPayload]{f.issued, event}
		}, wantErr: core.ErrReviewControlStaleSource},
		{name: "acceptance without its observation in history", build: func(t testing.TB) []proofledger.Envelope[EventPayload] {
			f := newFoldFixture(t)
			decision := reviewEvent(t, f.issued.Head(), 1, EventPayload{Kind: EventHumanAccepted, Decision: &f.record})
			return []proofledger.Envelope[EventPayload]{f.issued, decision}
		}, wantErr: core.ErrReviewControlObservationMismatch},
		{name: "acceptance names another contract digest", build: func(t testing.TB) []proofledger.Envelope[EventPayload] {
			f := newFoldFixture(t)
			changed := f.record
			changed.Intent.Contract = core.SHA256Of([]byte("other contract"))
			decision := reviewEvent(t, f.observed.Head(), 2, EventPayload{Kind: EventHumanAccepted, Decision: &changed})
			return []proofledger.Envelope[EventPayload]{f.issued, f.observed, decision}
		}, wantErr: core.ErrReviewControlSubjectMismatch},
		{name: "acceptance names another review", build: func(t testing.TB) []proofledger.Envelope[EventPayload] {
			f := newFoldFixture(t)
			other, _ := NewReviewIdentity(reviewUUID(t, 7))
			changed := f.record
			changed.Intent.Review = other
			decision := reviewEvent(t, f.observed.Head(), 2, EventPayload{Kind: EventHumanAccepted, Decision: &changed})
			return []proofledger.Envelope[EventPayload]{f.issued, f.observed, decision}
		}, wantErr: core.ErrReviewControlSubjectMismatch},
		{name: "acceptance names another source subject", build: func(t testing.TB) []proofledger.Envelope[EventPayload] {
			f := newFoldFixture(t)
			changed := f.record
			changed.Intent.Subject.Project.Project = core.Offering{Token: "other"}
			decision := reviewEvent(t, f.observed.Head(), 2, EventPayload{Kind: EventHumanAccepted, Decision: &changed})
			return []proofledger.Envelope[EventPayload]{f.issued, f.observed, decision}
		}, wantErr: core.ErrReviewControlSubjectMismatch},
		{name: "acceptance for stale commit retains stale-source identity", build: func(t testing.TB) []proofledger.Envelope[EventPayload] {
			f := newFoldFixture(t)
			changed := f.record
			commit, _ := core.ParseBuildCommit("1123456789abcdef0123456789abcdef01234567")
			changed.Intent.Subject.Source.Commit = commit
			decision := reviewEvent(t, f.observed.Head(), 2, EventPayload{Kind: EventHumanAccepted, Decision: &changed})
			return []proofledger.Envelope[EventPayload]{f.issued, f.observed, decision}
		}, wantErr: core.ErrReviewControlStaleSource},
		{name: "acceptance names another observation", build: func(t testing.TB) []proofledger.Envelope[EventPayload] {
			f := newFoldFixture(t)
			other, _ := NewObservationIdentity(reviewUUID(t, 7))
			changed := f.record
			changed.Intent.Observation = &other
			decision := reviewEvent(t, f.observed.Head(), 2, EventPayload{Kind: EventHumanAccepted, Decision: &changed})
			return []proofledger.Envelope[EventPayload]{f.issued, f.observed, decision}
		}, wantErr: core.ErrReviewControlObservationMismatch},
		{name: "supersession names another decision event", build: func(t testing.TB) []proofledger.Envelope[EventPayload] {
			f := newFoldFixture(t)
			reason, _ := NewDecisionReason("Another decision cannot be superseded.")
			supersession := DecisionSuperseded{Previous: f.observed.Event, Reason: reason}
			event := reviewEvent(t, f.accepted.Head(), 3, EventPayload{Kind: EventDecisionSuperseded, Supersession: &supersession})
			return []proofledger.Envelope[EventPayload]{f.issued, f.observed, f.accepted, event}
		}, wantErr: core.ErrReviewControlObservationMismatch},
		{name: "same decision cannot be superseded twice", build: func(t testing.TB) []proofledger.Envelope[EventPayload] {
			f := newFoldFixture(t)
			reason, _ := NewDecisionReason("Decision is replaced once.")
			supersession := DecisionSuperseded{Previous: f.accepted.Event, Reason: reason}
			first := reviewEvent(t, f.accepted.Head(), 3, EventPayload{Kind: EventDecisionSuperseded, Supersession: &supersession})
			second := reviewEvent(t, first.Head(), 3, EventPayload{Kind: EventDecisionSuperseded, Supersession: &supersession})
			return []proofledger.Envelope[EventPayload]{f.issued, f.observed, f.accepted, first, second}
		}, wantErr: core.ErrReviewControlObservationMismatch},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := observeFold(tc.build(t))
			if !errors.Is(gotErr, tc.wantErr) || got != (Projection{}) {
				t.Fatalf("Fold hostile transition = (%+v, %v), want zero and %v", got, gotErr, tc.wantErr)
			}
		})
	}
}

func TestReviewFoldPreservesNonpassingObservationWithoutChoosingProductPolicy(t *testing.T) {
	t.Parallel()
	fixture := newFoldFixture(t)
	observation := fixture.observation
	observation.Verdict = VerdictUnableToReview
	observed := reviewEvent(t, fixture.issued.Head(), 1, EventPayload{Kind: EventObservationRecorded, Observation: &observation})
	accepted := reviewEvent(t, observed.Head(), 2, EventPayload{Kind: EventHumanAccepted, Decision: &fixture.record})

	got, gotErr := observeFold([]proofledger.Envelope[EventPayload]{fixture.issued, observed, accepted})
	if gotErr != nil || !got.Current || got.LatestVerdict == nil || *got.LatestVerdict != VerdictUnableToReview || got.LatestDecision == nil || *got.LatestDecision != DecisionAccept {
		t.Fatalf("Fold(non-passing observation then human acceptance) = (%+v, %v), want preserved advisory verdict and current human decision", got, gotErr)
	}
}

func TestReviewFoldProjectionReturnsAnIndependentSnapshot(t *testing.T) {
	t.Parallel()
	fixture := newFoldFixture(t)
	var fold Fold
	for _, event := range []proofledger.Envelope[EventPayload]{fixture.issued, fixture.observed, fixture.accepted} {
		if err := fold.Observe(event); err != nil {
			t.Fatalf("Fold.Observe() error = %v, want nil", err)
		}
	}
	first, err := fold.Projection()
	if err != nil {
		t.Fatalf("Fold.Projection(first) error = %v, want nil", err)
	}
	*first.LatestVerdict = VerdictUnableToReview
	*first.LatestDecision = DecisionRefuse
	otherObservation, err := NewObservationIdentity(reviewUUID(t, 7))
	if err != nil {
		t.Fatalf("NewObservationIdentity(other) error = %v, want nil", err)
	}
	otherEvent := proofEventIdentity(t, 4)
	*first.Observation = otherObservation
	*first.DecisionEvent = otherEvent

	second, err := fold.Projection()
	if err != nil || second.LatestVerdict == nil || *second.LatestVerdict != fixture.observation.Verdict || second.LatestDecision == nil || *second.LatestDecision != DecisionAccept || second.Observation == nil || *second.Observation != fixture.observation.Identity || second.DecisionEvent == nil || *second.DecisionEvent != fixture.accepted.Event {
		t.Fatalf("Fold.Projection(after caller mutation) = (%+v, %v), want original fold-owned snapshot", second, err)
	}
}

func TestReviewFoldNeutralHistoryCreatesNoDecision(t *testing.T) {
	t.Parallel()
	fixture := newFoldFixture(t)
	cases := []struct {
		name            string
		events          []proofledger.Envelope[EventPayload]
		wantObservation bool
	}{
		{name: "issued review has no observation or decision", events: []proofledger.Envelope[EventPayload]{fixture.issued}},
		{name: "advisory observation has no human decision", events: []proofledger.Envelope[EventPayload]{fixture.issued, fixture.observed}, wantObservation: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := observeFold(tc.events)
			if gotErr != nil || got.Current || got.LatestDecision != nil || got.DecisionEvent != nil || (got.Observation != nil) != tc.wantObservation {
				t.Fatalf("Fold neutral history = (%+v, %v), want observation=%t and no decision", got, gotErr, tc.wantObservation)
			}
			response := ReadProjectionResponse{Projection: got}
			if _, gotErr := response.MarshalJSON(); gotErr != nil {
				t.Fatalf("ReadProjectionResponse.MarshalJSON(neutral) error = %v, want nil", gotErr)
			}
		})
	}
}
