package core

import (
	"encoding/json"
	"errors"
	"math"
	"testing"
)

// TestStrictJSONContainerUnknownKindFailsClosed proves an unset parser
// discriminator cannot silently inherit array accounting. Although production
// pushes only named kinds today, the stack owner must reject its zero state at
// both admission and consumption.
func TestStrictJSONContainerUnknownKindFailsClosed(t *testing.T) {
	t.Parallel()

	limits := DefaultStrictJSONLimits()
	if _, err := pushStrictJSONContainer(nil, strictJSONContainerKind(0), limits); !errors.Is(err, ErrJSONContract) {
		t.Fatalf("pushStrictJSONContainer(unknown) error = %v, want %v", err, ErrJSONContract)
	}
	stack := []strictJSONContainer{{kind: strictJSONContainerKind(0)}}
	if _, err := completeStrictJSONValue(stack, limits); !errors.Is(err, ErrJSONContract) {
		t.Fatalf("completeStrictJSONValue(unknown) error = %v, want %v", err, ErrJSONContract)
	}
}

// TestStrictJSONContainerKindExhaustsBackingDomain sweeps the whole backing
// uint8 so a future discriminator value cannot inherit object or array
// accounting by construction. Every rejection must carry the JSON contract
// identity, and every admitted value must own a distinct label, because the
// parser selects container accounting from this discriminator alone.
func TestStrictJSONContainerKindExhaustsBackingDomain(t *testing.T) {
	t.Parallel()

	labels := strictJSONContainerKindLabels()
	admittedLabels := make(map[string]strictJSONContainerKind, len(labels))
	for raw := 0; raw <= math.MaxUint8; raw++ {
		kind := strictJSONContainerKind(raw)
		wantAdmitted := kind == strictJSONContainerObject || kind == strictJSONContainerArray
		gotErr := kind.Validate()
		if kind.IsValid() != wantAdmitted || (gotErr == nil) != wantAdmitted {
			t.Fatalf(
				"strictJSONContainerKind(%d) = IsValid:%t Validate:%v, want admitted=%t",
				raw, kind.IsValid(), gotErr, wantAdmitted,
			)
		}
		if !wantAdmitted {
			if !errors.Is(gotErr, ErrJSONContract) {
				t.Fatalf(
					"strictJSONContainerKind(%d).Validate() error = %v, want %v",
					raw, gotErr, ErrJSONContract,
				)
			}
			continue
		}
		label := labels[kind]
		if label == "" {
			t.Fatalf("strictJSONContainerKind(%d) label = %q, want an admitted diagnostic", raw, label)
		}
		if prior, duplicated := admittedLabels[label]; duplicated {
			t.Fatalf(
				"strictJSONContainerKind(%d) and (%d) share label %q, want unique labels",
				prior, kind, label,
			)
		}
		admittedLabels[label] = kind
	}
	if len(admittedLabels) != 2 {
		t.Fatalf("admitted strictJSONContainerKind labels = %d, want 2", len(admittedLabels))
	}
	proveStrictJSONContainerKindStaysOffWire(t, strictJSONContainerObject)
}

// proveStrictJSONContainerKindStaysOffWire proves the claim the discriminator's
// off-wire marker makes. The kind is a private parser stack fact: giving it a
// JSON encoding would invent a wire protocol out of an implementation detail
// and let a decoded document choose the parser's container accounting. Adding
// MarshalJSON or UnmarshalJSON to the kind turns this red.
func proveStrictJSONContainerKindStaysOffWire(t *testing.T, kind strictJSONContainerKind) {
	t.Helper()

	kind.OffWireEnum()
	if _, encodes := any(kind).(json.Marshaler); encodes {
		t.Fatalf("strictJSONContainerKind(%d) implements json.Marshaler, want an off-wire enum", kind)
	}
	if _, decodes := any(&kind).(json.Unmarshaler); decodes {
		t.Fatalf("*strictJSONContainerKind(%d) implements json.Unmarshaler, want an off-wire enum", kind)
	}
}
