package contextstate_test

import (
	"encoding"
	json "encoding/json/v2"
	"errors"
	"math"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// TestStateValidateExhaustsUnderlyingDomain is an external public-contract
// proof. State's uint8 input space is small enough to test every possible value.
func TestStateValidateExhaustsUnderlyingDomain(t *testing.T) {
	t.Parallel()

	admitted := [...]contextstate.State{
		contextstate.StateNone,
		contextstate.StateCancelled,
		contextstate.StateDeadlineExceeded,
	}
	for raw := uint16(0); raw <= math.MaxUint8; raw++ {
		state := contextstate.State(raw)
		gotErr := state.Validate()
		wantValid := slices.Contains(admitted[:], state)
		if gotValid := state.IsValid(); gotValid != wantValid {
			t.Fatalf("State(%d).IsValid() = %t, want %t", state, gotValid, wantValid)
		}
		if wantValid {
			if gotErr != nil {
				t.Fatalf("State(%d).Validate() error = %v, want nil", state, gotErr)
			}
			continue
		}
		if !errors.Is(gotErr, core.ErrContextStateContract) {
			t.Fatalf(
				"State(%d).Validate() error = %v, want %v",
				state,
				gotErr,
				core.ErrContextStateContract,
			)
		}
	}
}

// admittedStateDiagnosticCount is the number of admitted states that must own a
// distinct diagnostic. Enrolling a new state must fail this test until that new
// state is given its own projection.
const admittedStateDiagnosticCount = 3

// TestStateStringIsClosedOverTheAdmittedDomain sweeps the whole underlying
// domain so a new enum member cannot silently inherit the unknown diagnostic
// from String's default arm.
func TestStateStringIsClosedOverTheAdmittedDomain(t *testing.T) {
	t.Parallel()

	unknown := contextstate.State(0).String()
	if unknown == "" {
		t.Fatalf("zero State.String() = %q, want non-empty", unknown)
	}
	var admitted []string
	for raw := uint16(0); raw <= math.MaxUint8; raw++ {
		state := contextstate.State(raw)
		got := state.String()
		if got == "" {
			t.Fatalf("State(%d).String() is empty", raw)
		}
		if !state.IsValid() {
			if got != unknown {
				t.Fatalf(
					"State(%d).String() = %q, want unknown diagnostic %q",
					raw,
					got,
					unknown,
				)
			}
			continue
		}
		if got == unknown {
			t.Fatalf("admitted State(%d).String() = %q, want a distinct diagnostic", raw, got)
		}
		if slices.Contains(admitted, got) {
			t.Fatalf("admitted State(%d).String() = %q duplicates an earlier state", raw, got)
		}
		admitted = append(admitted, got)
	}
	if len(admitted) != admittedStateDiagnosticCount {
		t.Fatalf(
			"admitted diagnostics = %d (%q), want %d",
			len(admitted),
			admitted,
			admittedStateDiagnosticCount,
		)
	}
}

type wireInterfaceProbe struct {
	implements func(any) bool
	name       string
}

// TestStateImplementsNoWireFormat proves the no-wire decision by asserting the
// absence of every standard marshaling interface on both the value and the
// pointer receiver. A marker method could not prove this.
func TestStateImplementsNoWireFormat(t *testing.T) {
	t.Parallel()

	probes := []wireInterfaceProbe{
		{name: "json.Marshaler", implements: func(value any) bool {
			_, ok := value.(json.Marshaler)
			return ok
		}},
		{name: "json.Unmarshaler", implements: func(value any) bool {
			_, ok := value.(json.Unmarshaler)
			return ok
		}},
		{name: "encoding.TextMarshaler", implements: func(value any) bool {
			_, ok := value.(encoding.TextMarshaler)
			return ok
		}},
		{name: "encoding.TextUnmarshaler", implements: func(value any) bool {
			_, ok := value.(encoding.TextUnmarshaler)
			return ok
		}},
		{name: "encoding.BinaryMarshaler", implements: func(value any) bool {
			_, ok := value.(encoding.BinaryMarshaler)
			return ok
		}},
		{name: "encoding.BinaryUnmarshaler", implements: func(value any) bool {
			_, ok := value.(encoding.BinaryUnmarshaler)
			return ok
		}},
	}
	state := contextstate.StateCancelled
	receivers := []struct {
		value any
		name  string
	}{
		{name: "value", value: state},
		{name: "pointer", value: &state},
	}
	for _, receiver := range receivers {
		for _, probe := range probes {
			if probe.implements(receiver.value) {
				t.Errorf(
					"State %s receiver implements %s, want no wire format",
					receiver.name,
					probe.name,
				)
			}
		}
	}
}
