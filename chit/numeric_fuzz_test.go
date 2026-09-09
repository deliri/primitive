package chit

import (
	"bytes"
	"errors"
	"math"
	"strconv"
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
		want, parseErr := strconv.ParseUint(string(data), 10, 64)
		valid := parseErr == nil && want > 0 && string(data) == strconv.FormatUint(want, 10)
		if (gotErr == nil) != valid || (valid && got.Uint64() != want) {
			t.Fatalf("numeric admission = %d, %v; want %d, valid %t", got.Uint64(), gotErr, want, valid)
		}
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
		want, parseErr := strconv.ParseUint(string(data), 10, 64)
		valid := parseErr == nil && want > 0 && string(data) == strconv.FormatUint(want, 10)
		if (gotErr == nil) != valid || (valid && got.Uint64() != want) {
			t.Fatalf("numeric admission = %d, %v; want %d, valid %t", got.Uint64(), gotErr, want, valid)
		}
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

func FuzzObjectCountJSONSemanticClosure(f *testing.F) {
	seed, err := NewObjectCount(math.MaxUint64)
	if err != nil {
		f.Fatal(err)
	}
	canonical, err := seed.MarshalJSON()
	if err != nil {
		f.Fatal(err)
	}
	for _, data := range [][]byte{canonical, []byte("1"), []byte("0"), nil, []byte("01"), []byte("1e0"), []byte("1.0"), []byte("null")} {
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		want, parseErr := strconv.ParseUint(string(data), 10, 64)
		valid := parseErr == nil && want > 0 && string(data) == strconv.FormatUint(want, 10)
		got := seed
		err := got.UnmarshalJSON(data)
		if (err == nil) != valid || (valid && got.Uint64() != want) {
			t.Fatalf("ObjectCount admission = %v, %v; want %d, valid %t", got, err, want, valid)
		}
		if err != nil {
			if !errors.Is(err, core.ErrJSONContract) || got != seed {
				t.Fatalf("ObjectCount refusal = %v, %v; want preserved %v and typed JSON error", got, err, seed)
			}
			return
		}
		encoded, encodeErr := got.MarshalJSON()
		if encodeErr != nil || !bytes.Equal(encoded, data) {
			t.Fatalf("ObjectCount accepted canonical projection = %q, %v; want %q, nil", encoded, encodeErr, data)
		}
	})
}
