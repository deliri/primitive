package core

import (
	"errors"
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

func TestStrictJSONContainerKindExhaustsBackingDomain(t *testing.T) {
	t.Parallel()

	for raw := 0; raw <= 255; raw++ {
		kind := strictJSONContainerKind(raw)
		admitted := kind == strictJSONContainerObject || kind == strictJSONContainerArray
		if kind.IsValid() != admitted || (kind.Validate() == nil) != admitted {
			t.Errorf("strictJSONContainerKind(%d) validity disagrees with admitted=%t", raw, admitted)
		}
	}
}
