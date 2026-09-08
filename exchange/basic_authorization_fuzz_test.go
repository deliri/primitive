package exchange_test

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"net/http"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

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
		if len(data) > core.JSONDocumentMaximumBytes+1 {
			return
		}
		var decoded *string
		var wantAccepted bool
		if len(data) <= core.JSONDocumentMaximumBytes {
			decodeErr := json.Unmarshal(data, &decoded)
			wantAccepted = decodeErr == nil && decoded != nil && basicIdentityInputAdmitted(*decoded)
		}
		got := seed
		before := got
		gotErr := got.UnmarshalJSON(data)
		if !wantAccepted {
			if !errors.Is(gotErr, core.ErrJSONContract) || !errors.Is(gotErr, core.ErrExchangeContract) || got != before {
				t.Fatalf("BasicAuthorizationIdentity.UnmarshalJSON(rejected) = (%v, %v), want preserved and %v", got, gotErr, core.ErrJSONContract)
			}
			return
		}
		if gotErr != nil || got.String() != *decoded {
			t.Fatalf("identity JSON input projection = (%q, %v), want (%q, nil)", got.String(), gotErr, *decoded)
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
	for _, size := range []int{1, exchange.BasicAuthorizationSecretMaximumBytes - 1, exchange.BasicAuthorizationSecretMaximumBytes} {
		credentials := exchange.BasicAuthorizationRequest{
			Identity: exchange.BasicAuthorizationIdentity(strings.Repeat("i", exchange.BasicAuthorizationIdentityMaximumBytes)),
			Secret:   bytes.Repeat([]byte{'s'}, size),
		}
		header, err := exchange.NewBasicAuthorizationHeader(credentials)
		if err != nil {
			f.Fatal(err)
		}
		value, err := header.Values[0].Value()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(value)
	}
	// Go can encode credentials that Primitive's nominal contract refuses.
	// These are malformed Primitive seeds, emitted without copying Basic wire
	// tokens or hand-encoding Base64 in the test.
	for _, credential := range []struct{ identity, secret string }{
		{"", "secret"}, {"identity", ""}, {"identity\u0085", "secret"},
		{"identity", "secret\u0085"}, {"\xff", "secret"}, {"identity", "\xff"},
		{"identity", strings.Repeat("s", exchange.BasicAuthorizationSecretMaximumBytes+1)},
	} {
		request, err := http.NewRequest(http.MethodGet, "https://example.invalid", nil)
		if err != nil {
			f.Fatal(err)
		}
		request.SetBasicAuth(credential.identity, credential.secret)
		f.Add(request.Header.Get(headerName.String()))
	}
	f.Fuzz(func(t *testing.T, value string) {
		if len(value) > exchange.BasicAuthorizationHeaderMaximumBytes+1 {
			return
		}
		request, err := http.NewRequest(http.MethodGet, "https://example.invalid", nil)
		if err != nil {
			t.Fatalf("http.NewRequest() error = %v, want nil", err)
		}
		request.Header.Set(headerName.String(), value)
		// Go owns Basic syntax and base64. The independent input predicate
		// adds only Primitive's nominal credential and HTTP field bounds.
		wantIdentity, wantSecret, standardAccepted := request.BasicAuth()
		wantAccepted := standardAccepted && len(value) <= exchange.BasicAuthorizationHeaderMaximumBytes &&
			strings.IndexFunc(value, func(r rune) bool { return r < ' ' && r != '\t' || r == 0x7f }) < 0 &&
			basicIdentityInputAdmitted(wantIdentity) && len(wantSecret) > 0 && len(wantSecret) <= exchange.BasicAuthorizationSecretMaximumBytes &&
			utf8.ValidString(wantSecret) && strings.IndexFunc(wantSecret, unicode.IsControl) < 0
		got, gotErr := exchange.ReceiveBasicAuthorization(socketServerCall(t, request))
		if !wantAccepted {
			if !errors.Is(gotErr, core.ErrExchangeRequest) || !errors.Is(gotErr, core.ErrExchangeContract) ||
				got.Identity != "" || got.Secret != nil {
				t.Fatalf("ReceiveBasicAuthorization(rejected) = (%v, %v), want zero and typed request contract rejection", got, gotErr)
			}
			return
		}
		if gotErr != nil || got.Identity.String() != wantIdentity || string(got.Secret) != wantSecret {
			t.Fatalf("ReceiveBasicAuthorization() identity/secret/error = (%q, %q, %v), want (%q, %q, nil)", got.Identity.String(), got.Secret, gotErr, wantIdentity, wantSecret)
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
		// A received secret belongs to this call. Changing it must not alter
		// the retained wire field or another receive's independently owned bytes.
		got.Secret[0] ^= 0xff
		if string(roundTrip.Secret) != wantSecret || request.Header.Get(headerName.String()) != value || got.Identity.String() != wantIdentity {
			t.Fatalf("secret/header/identity preserved=%t/%t/%t, want true/true/true", string(roundTrip.Secret) == wantSecret, request.Header.Get(headerName.String()) == value, got.Identity.String() == wantIdentity)
		}
		clear(got.Secret)
		clear(roundTrip.Secret)
	})
}

// Input-only oracle: no Exchange constructor or validator can decide its own
// acceptance. Unicode grammar is owned by Go; byte bounds are typed constants.
func basicIdentityInputAdmitted(value string) bool {
	return len(value) > 0 && len(value) <= exchange.BasicAuthorizationIdentityMaximumBytes &&
		utf8.ValidString(value) && !strings.ContainsRune(value, ':') && strings.IndexFunc(value, unicode.IsControl) < 0
}
