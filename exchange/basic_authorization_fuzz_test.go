package exchange_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func FuzzBasicAuthorizationIdentityJSONSemanticClosure(f *testing.F) {
	seed, err := exchange.ParseBasicAuthorizationIdentity("agent-operator")
	if err != nil {
		f.Fatalf("ParseBasicAuthorizationIdentity(seed) error = %v, want nil", err)
	}
	canonical, err := seed.MarshalJSON()
	if err != nil {
		f.Fatalf("BasicAuthorizationIdentity.MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte("null"))
	f.Add([]byte(`"identity:secret"`))
	f.Add([]byte(`"identity\n"`))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := seed
		before := got
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) || got != before {
				t.Fatalf("BasicAuthorizationIdentity.UnmarshalJSON(rejected) = (%v, %v), want preserved and %v", got, gotErr, core.ErrJSONContract)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("BasicAuthorizationIdentity.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil {
			t.Fatalf("BasicAuthorizationIdentity.MarshalJSON(accepted) error = %v, want nil", err)
		}
		var roundTrip exchange.BasicAuthorizationIdentity
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
			t.Fatalf("BasicAuthorizationIdentity canonical round trip = (%v, %v), want (%v, nil)", roundTrip, err, got)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("BasicAuthorizationIdentity second canonical projection = (%q, %v), want (%q, nil)", second, err, encoded)
		}
	})
}
