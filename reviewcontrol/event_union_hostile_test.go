package reviewcontrol

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestEventPayloadExhaustsKindAndArmPresenceDomain(t *testing.T) {
	t.Parallel()
	packet := reviewPacket(t)
	observation := reviewObservation(t, packet)
	authority := verifiedHuman(t, AuthorityHuman)
	acceptedIntent := reviewDecision(t, packet, observation, DecisionAccept)
	accepted, err := NewDecisionRecord(authority, acceptedIntent)
	if err != nil {
		t.Fatalf("NewDecisionRecord(accept) error = %v, want nil", err)
	}
	refusedIntent := reviewDecision(t, packet, observation, DecisionRefuse)
	refused, err := NewDecisionRecord(authority, refusedIntent)
	if err != nil {
		t.Fatalf("NewDecisionRecord(refuse) error = %v, want nil", err)
	}
	reason, err := NewDecisionReason("The exact prior decision is superseded.")
	if err != nil {
		t.Fatalf("NewDecisionReason() error = %v, want nil", err)
	}
	supersession := DecisionSuperseded{Previous: proofEventIdentity(t, 0), Reason: reason}

	for rawKind := 0; rawKind <= 255; rawKind++ {
		for mask := range 16 {
			payload := EventPayload{Kind: EventKind(rawKind)}
			if mask&1 != 0 {
				payload.Review = &packet
			}
			if mask&2 != 0 {
				payload.Observation = &observation
			}
			if mask&4 != 0 {
				if payload.Kind == EventHumanRefused {
					payload.Decision = &refused
				} else {
					payload.Decision = &accepted
				}
			}
			if mask&8 != 0 {
				payload.Supersession = &supersession
			}
			wantValid := eventPayloadCombinationIsValid(EventKind(rawKind), mask)
			gotErr := payload.Validate()
			if (gotErr == nil) != wantValid {
				t.Fatalf("EventPayload.Validate(kind=%d, arm_mask=%04b) error = %v, want valid=%t", rawKind, mask, gotErr, wantValid)
			}
			if !wantValid && !errors.Is(gotErr, core.ErrReviewControlUnsupportedEventKind) {
				t.Fatalf("EventPayload.Validate(kind=%d, arm_mask=%04b) error = %v, want %v", rawKind, mask, gotErr, core.ErrReviewControlUnsupportedEventKind)
			}
		}
	}
}

func eventPayloadCombinationIsValid(kind EventKind, mask int) bool {
	switch kind {
	case EventReviewIssued:
		return mask == 1
	case EventObservationRecorded:
		return mask == 2
	case EventHumanAccepted, EventHumanRefused:
		return mask == 4
	case EventDecisionSuperseded:
		return mask == 8
	default:
		return false
	}
}
