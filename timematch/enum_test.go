package timematch

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type enumDoor struct {
	name      string
	limit     int
	text      func(uint8) string
	valid     func(uint8) bool
	marshal   func(uint8) ([]byte, error)
	unmarshal func([]byte) (uint8, error)
}

func door[E ~uint8, P interface {
	*E
	UnmarshalJSON([]byte) error
}](name string, limit int, text func(E) string, valid func(E) bool, marshal func(E) ([]byte, error)) enumDoor {
	return enumDoor{name: name, limit: limit,
		text:    func(v uint8) string { return text(E(v)) },
		valid:   func(v uint8) bool { return valid(E(v)) },
		marshal: func(v uint8) ([]byte, error) { return marshal(E(v)) },
		unmarshal: func(data []byte) (uint8, error) {
			var got E
			err := P(&got).UnmarshalJSON(data)
			return uint8(got), err
		},
	}
}

func enumDoors() []enumDoor {
	return []enumDoor{
		door[Provider]("Provider", 3, Provider.String, Provider.IsValid, Provider.MarshalJSON),
		door[Access]("Access", 2, Access.String, Access.IsValid, Access.MarshalJSON),
		door[Lifetime]("Lifetime", 2, Lifetime.String, Lifetime.IsValid, Lifetime.MarshalJSON),
		door[MicrosoftTenant]("MicrosoftTenant", 3, MicrosoftTenant.String, MicrosoftTenant.IsValid, MicrosoftTenant.MarshalJSON),
		door[AvailabilityState]("AvailabilityState", 2, AvailabilityState.String, AvailabilityState.IsValid, AvailabilityState.MarshalJSON),
		door[RevokeOutcome]("RevokeOutcome", 3, RevokeOutcome.String, RevokeOutcome.IsValid, RevokeOutcome.MarshalJSON),
		door[DeleteOutcome]("DeleteOutcome", 2, DeleteOutcome.String, DeleteOutcome.IsValid, DeleteOutcome.MarshalJSON),
		door[EventCreationAttempt]("EventCreationAttempt", 2, EventCreationAttempt.String, EventCreationAttempt.IsValid, EventCreationAttempt.MarshalJSON),
		door[InvitationMethod]("InvitationMethod", 2, InvitationMethod.String, InvitationMethod.IsValid, InvitationMethod.MarshalJSON),
	}
}

// Every uint8 value of every TimeMatch enum: valid values render, encode and
// decode to themselves; every other value is invalid, renders empty and
// refuses to encode.
func TestEnumsExhaustEveryUint8Value(t *testing.T) {
	t.Parallel()
	for _, enum := range enumDoors() {
		t.Run(enum.name, func(t *testing.T) {
			t.Parallel()
			seen := map[string]bool{}
			for raw := range 256 {
				value := uint8(raw)
				wantValid := raw >= 1 && raw <= enum.limit
				encoded, err := enum.marshal(value)
				if enum.valid(value) != wantValid || (enum.text(value) != "") != wantValid || (err == nil) != wantValid {
					t.Fatalf("%s(%d) valid=%t text=%q marshal err=%v, want valid %t", enum.name, raw, enum.valid(value), enum.text(value), err, wantValid)
				}
				if !wantValid {
					if !errors.Is(err, core.ErrJSONContract) {
						t.Fatalf("%s(%d).MarshalJSON() error = %v, want %v", enum.name, raw, err, core.ErrJSONContract)
					}
					continue
				}
				if seen[enum.text(value)] {
					t.Fatalf("%s(%d) text %q duplicates another value", enum.name, raw, enum.text(value))
				}
				seen[enum.text(value)] = true
				got, err := enum.unmarshal(encoded)
				if err != nil || got != value {
					t.Fatalf("%s round trip(%s) = (%d, %v), want (%d, nil)", enum.name, encoded, got, err, raw)
				}
			}
		})
	}
}

func FuzzEnumJSONSemanticClosure(f *testing.F) {
	for _, enum := range enumDoors() {
		for raw := 1; raw <= enum.limit; raw++ {
			encoded, err := enum.marshal(uint8(raw))
			if err != nil {
				f.Fatalf("%s(%d).MarshalJSON() error = %v, want nil", enum.name, raw, err)
			}
			f.Add(encoded)
		}
	}
	for _, raw := range []string{`""`, `"GOOGLE"`, `" google"`, `"google\u0000"`, `1`, `null`, `"manage-events"`, `"unknown_next"`} {
		f.Add([]byte(raw))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		for _, enum := range enumDoors() {
			got, err := enum.unmarshal(data)
			if err != nil {
				if !errors.Is(err, core.ErrTimeMatchContract) || got != 0 {
					t.Fatalf("%s.UnmarshalJSON(rejected) = (%d, %v), want zero and ErrTimeMatchContract", enum.name, got, err)
				}
				continue
			}
			if !enum.valid(got) {
				t.Fatalf("%s.UnmarshalJSON accepted invalid value %d", enum.name, got)
			}
			first, err := enum.marshal(got)
			if err != nil {
				t.Fatalf("%s.MarshalJSON(accepted) error = %v, want nil", enum.name, err)
			}
			again, err := enum.unmarshal(first)
			second, secondErr := enum.marshal(again)
			if err != nil || secondErr != nil || again != got || !bytes.Equal(first, second) {
				t.Fatalf("%s canonical closure = (%d, %q, %v, %v), want (%d, %q)", enum.name, again, second, err, secondErr, got, first)
			}
		}
	})
}
