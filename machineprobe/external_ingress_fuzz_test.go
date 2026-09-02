package machineprobe_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/machineprobe"
)

// FuzzFailureKindJSONSemanticClosure ratchets the external JSON door for the
// machine-probe failure classification. The valid seeds come from the owning
// production marshaler; hostile bytes must retain both typed error identities
// and leave an already-populated receiver unchanged.
func FuzzFailureKindJSONSemanticClosure(f *testing.F) {
	for _, value := range []machineprobe.FailureKind{
		machineprobe.FailureExit,
		machineprobe.FailureOutput,
	} {
		encoded, err := value.MarshalJSON()
		if err != nil {
			f.Fatalf("FailureKind.MarshalJSON(seed) error = %v, want nil", err)
		}
		f.Add(encoded)
	}
	f.Add([]byte{})
	f.Add([]byte(`"future"`))
	f.Add([]byte(`null`))

	f.Fuzz(func(t *testing.T, data []byte) {
		original := machineprobe.FailureExit
		got := original
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) ||
				!errors.Is(gotErr, core.ErrStandardContract) ||
				got != original {
				t.Fatalf("FailureKind.UnmarshalJSON(rejected) = (%v, %v), want (%v, joined %v and %v)",
					got, gotErr, original, core.ErrJSONContract, core.ErrStandardContract)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("FailureKind.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		canonical, err := got.MarshalJSON()
		if err != nil {
			t.Fatalf("FailureKind.MarshalJSON(accepted) error = %v, want nil", err)
		}
		var roundTrip machineprobe.FailureKind
		if err := roundTrip.UnmarshalJSON(canonical); err != nil || roundTrip != got {
			t.Fatalf("FailureKind canonical round trip = (%v, %v), want (%v, nil)", roundTrip, err, got)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(second, canonical) {
			t.Fatalf("FailureKind second canonical projection = (%q, %v), want (%q, nil)", second, err, canonical)
		}
	})
}
