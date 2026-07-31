package upgrade

import (
	"encoding/json"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestClosedDomainsExhaustEveryBackingValue(t *testing.T) {
	t.Parallel()

	for raw := 0; raw <= 255; raw++ {
		slot := Slot(raw)
		wantSlot := slot == SlotA || slot == SlotB
		if slot.IsValid() != wantSlot ||
			(slot.Validate() == nil) != wantSlot ||
			(slot.String() != "") != wantSlot {
			t.Fatalf("Slot(%d) validity = %t/%t, want %t",
				raw, slot.IsValid(), slot.Validate() == nil, wantSlot)
		}

		outcome := TrialOutcome(raw)
		wantOutcome := outcome == TrialPassed || outcome == TrialFailed
		if outcome.IsValid() != wantOutcome ||
			(outcome.Validate() == nil) != wantOutcome ||
			(outcome.String() != "") != wantOutcome {
			t.Fatalf("TrialOutcome(%d) validity = %t/%t, want %t",
				raw, outcome.IsValid(), outcome.Validate() == nil, wantOutcome)
		}

		phase := FailurePhase(raw)
		wantPhase := phase > FailurePhaseUnknown && phase < failurePhaseLimit
		if phase.IsValid() != wantPhase ||
			(phase.Validate() == nil) != wantPhase ||
			(phase.String() != "") != wantPhase {
			t.Fatalf("FailurePhase(%d) validity = %t/%t, want %t",
				raw, phase.IsValid(), phase.Validate() == nil, wantPhase)
		}
	}
}

func TestSlotJSONRejectsUnknownAndPreservesReceiver(t *testing.T) {
	t.Parallel()

	for _, slot := range []Slot{SlotA, SlotB} {
		encoded, err := json.Marshal(slot)
		if err != nil {
			t.Fatalf("json.Marshal(%v) error = %v, want nil", slot, err)
		}
		var decoded Slot
		if err := json.Unmarshal(encoded, &decoded); err != nil || decoded != slot {
			t.Fatalf("Slot JSON round trip = (%v, %v), want (%v, nil)", decoded, err, slot)
		}
	}

	receiver := SlotA
	if err := json.Unmarshal([]byte(`"future"`), &receiver); err == nil ||
		receiver != SlotA {
		t.Fatalf("unknown Slot decode = (%v, %v), want preserved receiver and error", receiver, err)
	}
	if err := (*Slot)(nil).UnmarshalJSON(nil); err == nil {
		t.Fatalf("nil Slot receiver error = %v, want non-nil", err)
	}
	if _, err := json.Marshal(Slot(255)); !isUpgradeContract(err) {
		t.Fatalf("json.Marshal(Slot(255)) error = %v, want %v", err, core.ErrUpgradeContract)
	}
}
