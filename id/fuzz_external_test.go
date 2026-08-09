package id_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/id"
)

// FuzzParseUUIDv7 pressures the admission boundary with an oracle stronger
// than not panicking: every rejection carries the stable identity, and every
// admission renders back to exactly the admitted text, which is what
// canonical-only means.
func FuzzParseUUIDv7(f *testing.F) {
	f.Add("00000000-0001-7000-8000-000000000001")
	f.Add("ffffffff-ffff-7fff-bfff-ffffffffffff")
	f.Add("00000000-0000-4000-8000-000000000000")
	f.Add("00000000-0000-7000-C000-000000000000")
	f.Add("urn:uuid:00000000-0000-7000-8000-000")
	f.Add(strings.Repeat("0", 36))
	f.Add("")
	f.Fuzz(func(t *testing.T, value string) {
		got, err := id.ParseUUIDv7(value)
		if err != nil {
			if !errors.Is(err, core.ErrIDContract) {
				t.Fatalf("ParseUUIDv7(%q) error = %v, want errors.Is %v", value, err, core.ErrIDContract)
			}
			if !got.IsZero() {
				t.Fatalf("ParseUUIDv7(%q) rejected with value %v, want the zero value", value, got)
			}
			return
		}
		if validateErr := got.Validate(); validateErr != nil {
			t.Fatalf("ParseUUIDv7(%q) admitted an invalid value: %v", value, validateErr)
		}
		if got.String() != value {
			t.Fatalf("ParseUUIDv7(%q).String() = %q, want the admitted text itself", value, got.String())
		}
	})
}

// FuzzParseULID holds the same admission oracle over the Crockford door.
func FuzzParseULID(f *testing.F) {
	f.Add("01ARZ3NDEKTSV4RRFFQ69G5FAV")
	f.Add("7" + strings.Repeat("Z", 25))
	f.Add("8" + strings.Repeat("Z", 25))
	f.Add(strings.Repeat("0", 26))
	f.Add("01arz3ndektsv4rrffq69g5fav")
	f.Add("")
	f.Fuzz(func(t *testing.T, value string) {
		got, err := id.ParseULID(value)
		if err != nil {
			if !errors.Is(err, core.ErrIDContract) {
				t.Fatalf("ParseULID(%q) error = %v, want errors.Is %v", value, err, core.ErrIDContract)
			}
			if !got.IsZero() {
				t.Fatalf("ParseULID(%q) rejected with value %v, want the zero value", value, got)
			}
			return
		}
		if validateErr := got.Validate(); validateErr != nil {
			t.Fatalf("ParseULID(%q) admitted an invalid value: %v", value, validateErr)
		}
		if got.String() != value {
			t.Fatalf("ParseULID(%q).String() = %q, want the admitted text itself", value, got.String())
		}
	})
}
