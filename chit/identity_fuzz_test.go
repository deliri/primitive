package chit

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzParseChitIDSemanticClosure(f *testing.F) {
	canonical := mustChitID(f, 0x31, 1).String()
	for _, seed := range []string{
		canonical,
		"",
		canonical[:len(canonical)-1],
		canonical + "0",
		"00000000-0001-6000-8000-000000000001",
		"00000000-0001-7000-c000-000000000001",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, text string) {
		got, gotErr := ParseChitID(text)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrChitContract) || got != (ChitID{}) || got.String() != "" {
				t.Fatalf("ParseChitID(%q) = (%v, %v), want zero and %v", text, got, gotErr, core.ErrChitContract)
			}
			return
		}
		if got.Validate() != nil || got.String() != text {
			t.Fatalf("ParseChitID(%q) = %v validation %v, want exact admitted text and nil", text, got, got.Validate())
		}
		encoded, marshalErr := got.MarshalJSON()
		var roundTrip ChitID
		unmarshalErr := roundTrip.UnmarshalJSON(encoded)
		second, secondErr := roundTrip.MarshalJSON()
		if marshalErr != nil || unmarshalErr != nil || secondErr != nil || roundTrip != got || !bytes.Equal(second, encoded) {
			t.Fatalf(
				"ChitID canonical round trip = (%v, %q, %v, %v, %v), want (%v, %q, nil, nil, nil)",
				roundTrip, second, marshalErr, unmarshalErr, secondErr, got, encoded,
			)
		}
	})
}

func FuzzParseCollectionIDSemanticClosure(f *testing.F) {
	canonical, err := NewCollectionID(mustUUIDv7(f, 0x41, 2))
	if err != nil {
		f.Fatalf("NewCollectionID(seed) error = %v, want nil", err)
	}
	text := canonical.String()
	for _, seed := range []string{
		text,
		"",
		text[:len(text)-1],
		text + "0",
		"00000000-0001-6000-8000-000000000001",
		"00000000-0001-7000-c000-000000000001",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, text string) {
		got, gotErr := ParseCollectionID(text)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrChitContract) || got != (CollectionID{}) || got.String() != "" {
				t.Fatalf("ParseCollectionID(%q) = (%v, %v), want zero and %v", text, got, gotErr, core.ErrChitContract)
			}
			return
		}
		if got.Validate() != nil || got.String() != text {
			t.Fatalf("ParseCollectionID(%q) = %v validation %v, want exact admitted text and nil", text, got, got.Validate())
		}
		encoded, marshalErr := got.MarshalJSON()
		var roundTrip CollectionID
		unmarshalErr := roundTrip.UnmarshalJSON(encoded)
		second, secondErr := roundTrip.MarshalJSON()
		if marshalErr != nil || unmarshalErr != nil || secondErr != nil || roundTrip != got || !bytes.Equal(second, encoded) {
			t.Fatalf(
				"CollectionID canonical round trip = (%v, %q, %v, %v, %v), want (%v, %q, nil, nil, nil)",
				roundTrip, second, marshalErr, unmarshalErr, secondErr, got, encoded,
			)
		}
	})
}
