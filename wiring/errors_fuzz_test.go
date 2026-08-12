package wiring

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzErrorKindJSONSemanticOracle(f *testing.F) {
	for kind := ErrorKindUnknown + 1; kind < errorKindLimit; kind++ {
		canonical, err := kind.MarshalJSON()
		if err != nil {
			f.Fatalf("ErrorKind(%d).MarshalJSON() seed error = %v, want nil", kind, err)
		}
		f.Add(canonical)
	}
	f.Add([]byte{})
	f.Add([]byte(`""`))
	f.Add([]byte(`null`))
	f.Add([]byte(`"primitive-door"`))

	f.Fuzz(func(t *testing.T, data []byte) {
		before := ErrorKindCycle
		got := before
		err := got.UnmarshalJSON(data)
		if err != nil {
			if got != before {
				t.Fatalf("ErrorKind.UnmarshalJSON(rejected input) mutated receiver from %v to %v", before, got)
			}
			if !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("ErrorKind.UnmarshalJSON(rejected input) error = %v, want errors.Is(..., %v)", err, core.ErrJSONContract)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("ErrorKind.UnmarshalJSON(accepted input) produced invalid %v: %v", got, err)
		}
		canonical, marshalErr := got.MarshalJSON()
		if marshalErr != nil {
			t.Fatalf("accepted ErrorKind.MarshalJSON() error = %v, want nil", marshalErr)
		}
		if !bytes.Equal(canonical, data) {
			t.Fatalf("ErrorKind.UnmarshalJSON accepted noncanonical bytes")
		}
		second := ErrorKindUnknown
		if roundTripErr := second.UnmarshalJSON(canonical); roundTripErr != nil || second != got {
			t.Fatalf("ErrorKind canonical round trip = (%v, %v), want (%v, nil)", second, roundTripErr, got)
		}
	})
}
