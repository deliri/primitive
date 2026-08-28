package attest

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// attestExternalJSONDoorInventory is compiler-visible: changing either public
// JSON receiver breaks this test package before the AST ratchet can even run.
// Envelope uses the package's self-referential test domain so the generic
// method expression has the exact production signature.
type attestExternalJSONDoorInventory struct {
	Envelope  func(*Envelope[internalTestDomain], []byte) error
	Signature func(*Signature, []byte) error
}

var attestExternalJSONDoors = attestExternalJSONDoorInventory{
	Envelope:  (*Envelope[internalTestDomain]).UnmarshalJSON,
	Signature: (*Signature).UnmarshalJSON,
}

func FuzzSignatureExternalJSONDoor(f *testing.F) {
	canonical := signatureCanonicalFixture(f, 0x5a)
	survivor := signatureFixture(f, 0xa5)
	for _, seed := range [][]byte{
		canonical,
		nil,
		{},
		[]byte(`null`),
		[]byte(`""`),
		[]byte(`{}`),
		[]byte(`[]`),
		[]byte(`"00"`),
		append(bytes.Clone(canonical), 0),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		var gotFresh Signature
		gotFreshErr := attestExternalJSONDoors.Signature(&gotFresh, data)
		gotPopulated := survivor
		gotPopulatedErr := attestExternalJSONDoors.Signature(&gotPopulated, data)
		if gotFreshErr != nil || gotPopulatedErr != nil {
			if !errors.Is(gotFreshErr, core.ErrJSONContract) ||
				!errors.Is(gotFreshErr, core.ErrAttestContract) ||
				!errors.Is(gotPopulatedErr, core.ErrJSONContract) ||
				!errors.Is(gotPopulatedErr, core.ErrAttestContract) {
				t.Fatalf(
					"Signature.UnmarshalJSON(rejected) errors = (%v, %v), want %v and %v for both",
					gotFreshErr,
					gotPopulatedErr,
					core.ErrJSONContract,
					core.ErrAttestContract,
				)
			}
			if gotFresh != (Signature{}) {
				t.Fatalf("fresh Signature after rejection = %v, want zero", gotFresh)
			}
			if gotPopulated != survivor {
				t.Fatalf("populated Signature after rejection = %v, want preserved %v", gotPopulated, survivor)
			}
			return
		}
		if gotFresh != gotPopulated {
			t.Fatalf("Signature.UnmarshalJSON(accepted) receivers = (%v, %v), want equal", gotFresh, gotPopulated)
		}
		if gotErr := gotFresh.Validate(); gotErr != nil {
			t.Fatalf("Signature.UnmarshalJSON(accepted).Validate() error = %v, want nil", gotErr)
		}
		encoded, gotErr := gotFresh.MarshalJSON()
		if gotErr != nil {
			t.Fatalf("Signature.MarshalJSON() error = %v, want nil", gotErr)
		}
		var roundTrip Signature
		if gotErr := attestExternalJSONDoors.Signature(&roundTrip, encoded); gotErr != nil || roundTrip != gotFresh {
			t.Fatalf(
				"Signature canonical round trip = (%v, %v), want (%v, nil)",
				roundTrip,
				gotErr,
				gotFresh,
			)
		}
		second, gotErr := roundTrip.MarshalJSON()
		if gotErr != nil || !bytes.Equal(second, encoded) {
			t.Fatalf(
				"Signature second canonical projection = (%q, %v), want (%q, nil)",
				second,
				gotErr,
				encoded,
			)
		}
	})
}

func signatureFixture(t testing.TB, fill byte) Signature {
	t.Helper()

	value, gotErr := newSignature(bytes.Repeat([]byte{fill}, ed25519.SignatureSize))
	if gotErr != nil {
		t.Fatalf("newSignature() setup error = %v, want nil", gotErr)
	}
	return value
}

func signatureCanonicalFixture(t testing.TB, fill byte) []byte {
	t.Helper()

	encoded, gotErr := signatureFixture(t, fill).MarshalJSON()
	if gotErr != nil {
		t.Fatalf("Signature.MarshalJSON() setup error = %v, want nil", gotErr)
	}
	return encoded
}

func TestAttestExternalJSONDoorInventoryMatchesProduction(t *testing.T) {
	t.Parallel()

	got, gotErr := scanAttestExternalJSONReceivers(".")
	if gotErr != nil {
		t.Fatalf("scanAttestExternalJSONReceivers() error = %v, want nil", gotErr)
	}
	want := externalDoorInventoryFieldNames(attestExternalJSONDoors)
	if !slices.Equal(got, want) {
		t.Fatalf("Attest external JSON receivers = %q, want compiler inventory %q", got, want)
	}
}

func externalDoorInventoryFieldNames(inventory any) []string {
	typeOf := reflect.TypeOf(inventory)
	fields := make([]string, 0, typeOf.NumField())
	for field := range typeOf.Fields() {
		fields = append(fields, field.Name)
	}
	slices.Sort(fields)
	return fields
}
