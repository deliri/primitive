package chit

import (
	"bytes"
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzVersionJSONSemanticClosure(f *testing.F) {
	canonical, err := NewVersion(math.MaxUint64)
	if err != nil {
		f.Fatalf("NewVersion(seed) error = %v, want nil", err)
	}
	seed, err := canonical.MarshalJSON()
	if err != nil {
		f.Fatalf("Version.MarshalJSON(seed) error = %v, want nil", err)
	}
	for _, data := range [][]byte{seed, []byte("1"), nil, []byte("0"), []byte("01"), []byte("-1"), []byte("1.0"), []byte("null")} {
		f.Add(data)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		before, setupErr := NewVersion(1)
		if setupErr != nil {
			t.Fatalf("NewVersion(receiver) error = %v, want nil", setupErr)
		}
		got := before
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) || got != before {
				t.Fatalf("Version.UnmarshalJSON(%q) = (%v, %v), want preserved %v and %v", data, got, gotErr, before, core.ErrJSONContract)
			}
			return
		}
		encoded, marshalErr := got.MarshalJSON()
		var roundTrip Version
		roundTripErr := roundTrip.UnmarshalJSON(encoded)
		second, secondErr := roundTrip.MarshalJSON()
		if got.Validate() != nil || got.Uint64() == 0 || marshalErr != nil || roundTripErr != nil || secondErr != nil ||
			roundTrip != got || !bytes.Equal(second, encoded) {
			t.Fatalf(
				"Version accepted closure = (%d, %v, %v, %q, %v, %v), want positive valid stable round trip",
				got.Uint64(), got.Validate(), roundTrip, second, marshalErr, errors.Join(roundTripErr, secondErr),
			)
		}
	})
}

func FuzzEntrySequenceJSONSemanticClosure(f *testing.F) {
	canonical, err := NewEntrySequence(math.MaxUint64)
	if err != nil {
		f.Fatalf("NewEntrySequence(seed) error = %v, want nil", err)
	}
	seed, err := canonical.MarshalJSON()
	if err != nil {
		f.Fatalf("EntrySequence.MarshalJSON(seed) error = %v, want nil", err)
	}
	for _, data := range [][]byte{seed, []byte("1"), nil, []byte("0"), []byte("01"), []byte("-1"), []byte("1.0"), []byte("null")} {
		f.Add(data)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		before, setupErr := NewEntrySequence(1)
		if setupErr != nil {
			t.Fatalf("NewEntrySequence(receiver) error = %v, want nil", setupErr)
		}
		got := before
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) || got != before {
				t.Fatalf("EntrySequence.UnmarshalJSON(%q) = (%v, %v), want preserved %v and %v", data, got, gotErr, before, core.ErrJSONContract)
			}
			return
		}
		encoded, marshalErr := got.MarshalJSON()
		var roundTrip EntrySequence
		roundTripErr := roundTrip.UnmarshalJSON(encoded)
		second, secondErr := roundTrip.MarshalJSON()
		if got.Validate() != nil || got.Uint64() == 0 || marshalErr != nil || roundTripErr != nil || secondErr != nil ||
			roundTrip != got || !bytes.Equal(second, encoded) {
			t.Fatalf(
				"EntrySequence accepted closure = (%d, %v, %v, %q, %v, %v), want positive valid stable round trip",
				got.Uint64(), got.Validate(), roundTrip, second, marshalErr, errors.Join(roundTripErr, secondErr),
			)
		}
	})
}
