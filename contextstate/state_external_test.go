package contextstate_test

import (
	"encoding"
	json "encoding/json/v2"
	"errors"
	"fmt"
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
	type stateCase struct {
		name      string
		state     contextstate.State
		wantValid bool
	}
	cases := make([]stateCase, 0, int(math.MaxUint8)+1)
	for raw := range uint16(math.MaxUint8) + 1 {
		state := contextstate.State(raw)
		cases = append(cases, stateCase{name: fmt.Sprintf("backing value %d", raw), state: state, wantValid: slices.Contains(admitted[:], state)})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotErr := tc.state.Validate()
			if got := tc.state.IsValid(); got != tc.wantValid {
				t.Fatalf("State(%d).IsValid()=%t; want %t", tc.state, got, tc.wantValid)
			}
			if tc.wantValid {
				if gotErr != nil {
					t.Fatalf("State(%d).Validate()=%v; want nil", tc.state, gotErr)
				}
			} else if !errors.Is(gotErr, core.ErrContextStateContract) {
				t.Fatalf("State(%d).Validate()=%v; want %v", tc.state, gotErr, core.ErrContextStateContract)
			}
		})
	}
}

// TestStateStringIsClosedOverTheAdmittedDomain sweeps the whole underlying
// domain so a new enum member cannot silently inherit the unknown diagnostic
// from String's default arm.
func TestStateStringIsClosedOverTheAdmittedDomain(t *testing.T) {
	t.Parallel()
	type diagnosticCase struct {
		name      string
		state     contextstate.State
		wantKnown bool
	}
	cases := make([]diagnosticCase, 0, int(math.MaxUint8)+1)
	for raw := range uint16(math.MaxUint8) + 1 {
		state := contextstate.State(raw)
		known := state == contextstate.StateNone || state == contextstate.StateCancelled || state == contextstate.StateDeadlineExceeded
		cases = append(cases, diagnosticCase{name: fmt.Sprintf("backing value %d", raw), state: state, wantKnown: known})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.state.String()
			if tc.wantKnown {
				if got == "" || got == core.UnknownEnumDiagnostic {
					t.Fatalf("State(%d).String()=%q; want a known diagnostic", tc.state, got)
				}
			} else if got != core.UnknownEnumDiagnostic {
				t.Fatalf("State(%d).String()=%q; want %q", tc.state, got, core.UnknownEnumDiagnostic)
			}
		})
	}
	pairs := []struct {
		name        string
		left, right contextstate.State
	}{
		{name: "active differs from cancellation", left: contextstate.StateNone, right: contextstate.StateCancelled},
		{name: "active differs from expiry", left: contextstate.StateNone, right: contextstate.StateDeadlineExceeded},
		{name: "cancellation differs from expiry", left: contextstate.StateCancelled, right: contextstate.StateDeadlineExceeded},
	}
	for _, tc := range pairs {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			left, right := tc.left.String(), tc.right.String()
			if left == right {
				t.Fatalf("states %d and %d share diagnostic %q; want distinct observations", tc.left, tc.right, left)
			}
		})
	}
}

type wireInterfaceProbe struct {
	implements      func(any) bool
	name            string
	wantImplemented bool
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
		{name: "json.MarshalerTo", implements: func(value any) bool {
			_, ok := value.(json.MarshalerTo)
			return ok
		}},
		{name: "json.UnmarshalerFrom", implements: func(value any) bool {
			_, ok := value.(json.UnmarshalerFrom)
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
		{name: "encoding.TextAppender", implements: func(value any) bool {
			_, ok := value.(encoding.TextAppender)
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
		{name: "encoding.BinaryAppender", implements: func(value any) bool {
			_, ok := value.(encoding.BinaryAppender)
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
			t.Run(receiver.name+"/"+probe.name, func(t *testing.T) {
				t.Parallel()
				if got := probe.implements(receiver.value); got != probe.wantImplemented {
					t.Errorf("State %s receiver implements %s = %t; want %t", receiver.name, probe.name, got, probe.wantImplemented)
				}
			})
		}
	}
}
