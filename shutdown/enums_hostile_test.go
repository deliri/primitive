package shutdown

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type enumContract[T ~uint8] struct {
	first    T
	limit    T
	valid    func(T) bool
	validate func(T) error
	label    func(T) string
	name     string
}

func TestClosedEnumsExhaustUnderlyingByteDomain(t *testing.T) {
	t.Parallel()

	exhaustEnum(t, enumContract[Phase]{
		name: "phase", first: PhaseStopAdmission, limit: phaseLimit,
		valid: Phase.IsValid, validate: Phase.Validate, label: Phase.String,
	})
	exhaustEnum(t, enumContract[StepOutcome]{
		name: "step outcome", first: StepOutcomeCompleted, limit: stepOutcomeLimit,
		valid: StepOutcome.IsValid, validate: StepOutcome.Validate,
		label: StepOutcome.String,
	})
	exhaustEnum(t, enumContract[SignalKind]{
		name: "signal kind", first: SignalKindInterrupt, limit: signalKindLimit,
		valid: SignalKind.IsValid, validate: SignalKind.Validate,
		label: SignalKind.String,
	})
	exhaustEnum(t, enumContract[SignalSet]{
		name: "signal set", first: SignalSetInteractive, limit: signalSetLimit,
		valid: SignalSet.IsValid, validate: SignalSet.Validate,
		label: SignalSet.String,
	})
	exhaustEnum(t, enumContract[SecondSignalAction]{
		name: "second signal action", first: SecondSignalRelease,
		limit: secondSignalActionLimit, valid: SecondSignalAction.IsValid,
		validate: SecondSignalAction.Validate, label: SecondSignalAction.String,
	})
	exhaustEnum(t, enumContract[GraceExpiryAction]{
		name: "grace expiry action", first: GraceExpiryDisabled,
		limit: graceExpiryActionLimit, valid: GraceExpiryAction.IsValid,
		validate: GraceExpiryAction.Validate, label: GraceExpiryAction.String,
	})
	exhaustEnum(t, enumContract[EscalationReason]{
		name: "escalation reason", first: EscalationSecondSignal,
		limit: escalationReasonLimit, valid: EscalationReason.IsValid,
		validate: EscalationReason.Validate, label: EscalationReason.String,
	})
}

func exhaustEnum[T ~uint8](t *testing.T, contract enumContract[T]) {
	t.Helper()

	labels := make(map[string]T)
	for raw := 0; raw <= 255; raw++ {
		value := T(raw)
		wantValid := value >= contract.first && value < contract.limit
		gotValid := contract.valid(value)
		gotErr := contract.validate(value)
		gotLabel := contract.label(value)
		if gotValid != wantValid || (gotErr == nil) != wantValid {
			t.Fatalf("%s(%d) = valid:%t error:%v, want valid:%t",
				contract.name, raw, gotValid, gotErr, wantValid)
		}
		if !wantValid {
			if !errors.Is(gotErr, core.ErrShutdownContract) ||
				gotLabel != unknownLabel {
				t.Fatalf("%s(%d) = label:%q error:%v, want unknown/contract",
					contract.name, raw, gotLabel, gotErr)
			}
			continue
		}
		if gotLabel == "" || gotLabel == unknownLabel {
			t.Fatalf("%s(%d) label = %q, want admitted diagnostic", contract.name, raw, gotLabel)
		}
		if prior, exists := labels[gotLabel]; exists {
			t.Fatalf("%s(%d) label %q duplicates %d", contract.name, raw, gotLabel, prior)
		}
		labels[gotLabel] = value
	}
}
