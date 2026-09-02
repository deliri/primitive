package exchange_test

import (
	"bytes"
	"errors"
	"net/http"
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

func FuzzReceiveBasicAuthorizationSemanticClosure(f *testing.F) {
	identity, err := exchange.ParseBasicAuthorizationIdentity("agent-operator")
	if err != nil {
		f.Fatalf("ParseBasicAuthorizationIdentity(seed) error = %v, want nil", err)
	}
	header, err := exchange.NewBasicAuthorizationHeader(exchange.BasicAuthorizationRequest{
		Identity: identity,
		Secret:   []byte("secret"),
	})
	if err != nil {
		f.Fatalf("NewBasicAuthorizationHeader(seed) error = %v, want nil", err)
	}
	canonical, err := header.Values[0].Value()
	if err != nil {
		f.Fatalf("HeaderValue.Value(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add("")
	f.Add("Basic ")
	f.Add("Bearer token")
	f.Add("Basic !!!!")
	f.Add("Basic dXNlcg==")

	headerName, err := exchange.StandardHeaderAuthorization.Name()
	if err != nil {
		f.Fatalf("StandardHeaderAuthorization.Name() error = %v, want nil", err)
	}
	f.Fuzz(func(t *testing.T, value string) {
		request, err := http.NewRequest(http.MethodGet, "https://example.invalid", nil)
		if err != nil {
			t.Fatalf("http.NewRequest() error = %v, want nil", err)
		}
		request.Header.Set(headerName.String(), value)
		got, gotErr := exchange.ReceiveBasicAuthorization(socketServerCall(t, request))
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrExchangeRequest) || !errors.Is(gotErr, core.ErrExchangeContract) ||
				got.Identity != "" || got.Secret != nil {
				t.Fatalf("ReceiveBasicAuthorization(rejected) = (%v, %v), want zero and typed request contract rejection", got, gotErr)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("ReceiveBasicAuthorization(accepted).Validate() error = %v, want nil", err)
		}
		projected, err := exchange.NewBasicAuthorizationHeader(got)
		if err != nil {
			t.Fatalf("NewBasicAuthorizationHeader(accepted) error = %v, want nil", err)
		}
		projectedValue, err := projected.Values[0].Value()
		if err != nil {
			t.Fatalf("HeaderValue.Value(accepted) error = %v, want nil", err)
		}
		roundTripRequest, err := http.NewRequest(http.MethodGet, "https://example.invalid", nil)
		if err != nil {
			t.Fatalf("http.NewRequest(round trip) error = %v, want nil", err)
		}
		roundTripRequest.Header.Set(headerName.String(), projectedValue)
		roundTrip, err := exchange.ReceiveBasicAuthorization(socketServerCall(t, roundTripRequest))
		if err != nil || roundTrip.Identity != got.Identity || !bytes.Equal(roundTrip.Secret, got.Secret) {
			t.Fatalf("Basic authorization canonical round trip = (%v, %v), want (%v, nil)", roundTrip, err, got)
		}
	})
}
