package garble_test

import (
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/garble"
)

type garbleOffWireEnum interface {
	comparable
	core.OffWireEnum
	IsValid() bool
	String() string
}

func TestGarbleOffWireEnumsExhaustClosedDomains(t *testing.T) {
	t.Parallel()

	cases := []struct {
		run  func(*testing.T)
		name string
	}{
		{name: "literal policies reject every unadmitted uint8 value", run: func(t *testing.T) {
			proveGarbleOffWireEnum(t, func(raw uint8) garble.LiteralPolicy { return garble.LiteralPolicy(raw) }, []garble.LiteralPolicy{
				garble.LiteralPolicyPreserve,
				garble.LiteralPolicyObfuscate,
			}, core.ErrGarbleContract)
		}},
		{name: "diagnostic policies reject every unadmitted uint8 value", run: func(t *testing.T) {
			proveGarbleOffWireEnum(t, func(raw uint8) garble.DiagnosticPolicy { return garble.DiagnosticPolicy(raw) }, []garble.DiagnosticPolicy{
				garble.DiagnosticPolicyPreserve,
				garble.DiagnosticPolicyStrip,
			}, core.ErrGarbleContract)
		}},
		{name: "argument kinds reject every unadmitted uint8 value", run: func(t *testing.T) {
			proveGarbleOffWireEnum(t, func(raw uint8) garble.ArgumentKind { return garble.ArgumentKind(raw) }, []garble.ArgumentKind{
				garble.ArgumentKindSeed,
				garble.ArgumentKindLiterals,
				garble.ArgumentKindTiny,
				garble.ArgumentKindBuild,
			}, core.ErrGarbleBuildIntent)
		}},
		{name: "derivation generations reject every unadmitted uint8 value", run: func(t *testing.T) {
			proveGarbleOffWireEnum(t, func(raw uint8) garble.DerivationGeneration { return garble.DerivationGeneration(raw) }, []garble.DerivationGeneration{
				garble.DerivationGenerationOne,
			}, core.ErrGarbleContract)
		}},
		{name: "tool identities reject every unadmitted uint8 value", run: func(t *testing.T) {
			proveGarbleOffWireEnum(t, func(raw uint8) garble.ToolIdentity { return garble.ToolIdentity(raw) }, []garble.ToolIdentity{
				garble.ToolIdentityPrimitive2026,
			}, core.ErrGarbleContract)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.run(t)
		})
	}
}

func proveGarbleOffWireEnum[T garbleOffWireEnum](
	t *testing.T,
	fromRaw func(uint8) T,
	admitted []T,
	wantErr error,
) {
	t.Helper()

	wantAdmitted := make(map[T]struct{}, len(admitted))
	labels := make(map[string]T, len(admitted))
	unknownLabel := fromRaw(0).String()
	if unknownLabel == "" {
		t.Fatalf("%T zero String() = empty, want one safe diagnostic", fromRaw(0))
	}
	for _, value := range admitted {
		wantAdmitted[value] = struct{}{}
	}
	for raw := uint16(0); raw <= math.MaxUint8; raw++ {
		value := fromRaw(uint8(raw))
		_, wantValid := wantAdmitted[value]
		gotErr := value.Validate()
		if value.IsValid() != wantValid || (gotErr == nil) != wantValid {
			t.Fatalf(
				"%T(%d) validity = IsValid:%t Validate:%v, want %t",
				value,
				raw,
				value.IsValid(),
				gotErr,
				wantValid,
			)
		}
		if !wantValid {
			if !errors.Is(gotErr, wantErr) || value.String() != unknownLabel {
				t.Fatalf(
					"%T(%d) rejection = error:%v label:%q, want %v/%q",
					value,
					raw,
					gotErr,
					value.String(),
					wantErr,
					unknownLabel,
				)
			}
			continue
		}
		label := value.String()
		if label == "" || label == unknownLabel {
			t.Fatalf("%T(%d).String() = %q, want an admitted diagnostic", value, raw, label)
		}
		if prior, exists := labels[label]; exists {
			t.Fatalf("%T values %v and %v share label %q, want unique labels", value, prior, value, label)
		}
		labels[label] = value
	}
	proveGarbleEnumStaysOffWire(t, admitted[0])
}

func proveGarbleEnumStaysOffWire[T garbleOffWireEnum](t *testing.T, value T) {
	t.Helper()

	if _, implemented := any(value).(json.Marshaler); implemented {
		t.Fatalf("%T implements json.Marshaler, want an off-wire enum", value)
	}
	if _, implemented := any(&value).(json.Unmarshaler); implemented {
		t.Fatalf("*%T implements json.Unmarshaler, want an off-wire enum", value)
	}
}
