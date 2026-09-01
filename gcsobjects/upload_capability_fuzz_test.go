package gcsobjects

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzGCSServiceAccountSemanticClosure(f *testing.F) {
	valid, err := ParseGCSServiceAccount("media-signer@example-project.iam.gserviceaccount.com")
	if err != nil {
		f.Fatalf("ParseGCSServiceAccount(seed) error = %v, want nil", err)
	}
	f.Add(valid.String())
	f.Add("")
	f.Add("signer@example.com")
	f.Add("Signer <signer@example-project.iam.gserviceaccount.com>")

	f.Fuzz(func(t *testing.T, value string) {
		got, gotErr := ParseGCSServiceAccount(value)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrObjectStoreContract) || got != (GCSServiceAccount{}) {
				t.Fatalf("ParseGCSServiceAccount(rejected %q) = (%v, %v), want zero and typed contract error", value, got, gotErr)
			}
			return
		}
		if got.Validate() != nil || got.String() != value {
			t.Fatalf("ParseGCSServiceAccount(accepted %q) = %v, want exact validated value", value, got)
		}
		roundTrip, roundTripErr := ParseGCSServiceAccount(got.String())
		if roundTripErr != nil || roundTrip != got {
			t.Fatalf("ParseGCSServiceAccount(round trip %q) = (%v, %v), want (%v, nil)", got.String(), roundTrip, roundTripErr, got)
		}
	})
}
