package exchange

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzNewBasicAuthorizationHeaderCustody(f *testing.F) {
	for _, seed := range []BasicAuthorizationRequest{
		{Identity: "i", Secret: []byte("s")},
		{Identity: "é界", Secret: []byte("🔌:s")},
		{Identity: BasicAuthorizationIdentity(strings.Repeat("i", BasicAuthorizationIdentityMaximumBytes)), Secret: bytes.Repeat([]byte{'s'}, BasicAuthorizationSecretMaximumBytes)},
	} {
		if err := seed.Validate(); err != nil {
			f.Fatal(err)
		}
		// Project validated fields through their real nominal boundaries. The Basic
		// header constructor under test is reached only inside the callback.
		value, err := NewHeaderValue(string(seed.Secret))
		if err != nil {
			f.Fatal(err)
		}
		wire, err := value.Value()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(seed.Identity.String(), []byte(wire))
	}
	f.Add("", []byte("s"))
	f.Add("i", []byte{})
	f.Add("i:d", []byte("s"))
	f.Add("i", []byte{'s', 0})
	f.Add("i\u0085", []byte("s"))
	f.Add("i", []byte{0xff})
	f.Add(strings.Repeat("i", BasicAuthorizationIdentityMaximumBytes+1), []byte("s"))
	f.Add("i", bytes.Repeat([]byte{'s'}, BasicAuthorizationSecretMaximumBytes+1))
	f.Fuzz(func(t *testing.T, identity string, secret []byte) {
		if len(identity) > BasicAuthorizationIdentityMaximumBytes+1 || len(secret) > BasicAuthorizationSecretMaximumBytes+1 {
			return
		}
		wantAccepted := len(identity) > 0 && len(identity) <= BasicAuthorizationIdentityMaximumBytes && utf8.ValidString(identity) && !strings.ContainsRune(identity, ':') && strings.IndexFunc(identity, unicode.IsControl) < 0 && len(secret) > 0 && len(secret) <= BasicAuthorizationSecretMaximumBytes && utf8.Valid(secret) && strings.IndexFunc(string(secret), unicode.IsControl) < 0
		owned := bytes.Clone(secret)
		request := BasicAuthorizationRequest{Identity: BasicAuthorizationIdentity(identity), Secret: owned}
		got, gotErr := NewBasicAuthorizationHeader(request)
		if !bytes.Equal(owned, secret) || request.Identity.String() != identity {
			t.Fatalf("secret/identity preserved=%t/%t, want true/true", bytes.Equal(owned, secret), request.Identity.String() == identity)
		}
		if !wantAccepted {
			if !errors.Is(gotErr, core.ErrExchangeContract) || got.Name != (core.HTTPHeaderName{}) || got.Values != nil {
				t.Fatalf("header refusal = (%v,%v), want zero and typed refusal", got, gotErr)
			}
			return
		}
		if gotErr != nil || got.Validate() != nil || got.Name.String() != StandardHeaderAuthorization.String() || len(got.Values) != 1 {
			t.Fatalf("header admission = (%v,%v), want exact validated Authorization field", got, gotErr)
		}
		if fmt.Sprintf("%v", request) != core.RedactedValueText || fmt.Sprintf("%v", got.Values[0]) != core.RedactedValueText {
			t.Fatalf("request/header redacted=%t/%t, want true/true", fmt.Sprintf("%v", request) == core.RedactedValueText, fmt.Sprintf("%v", got.Values[0]) == core.RedactedValueText)
		}
		standard, err := http.NewRequest(http.MethodGet, "https://example.invalid", nil)
		if err != nil {
			t.Fatal(err)
		}
		standard.SetBasicAuth(identity, string(secret))
		want := standard.Header.Get(got.Name.String())
		clear(owned)
		wire, err := got.Values[0].Value()
		if err != nil || wire != want || len(wire) > BasicAuthorizationHeaderMaximumBytes {
			t.Fatalf("owned Basic wire = (%q,%v), want exact Go %q within ceiling", wire, err, want)
		}
		standard.Header.Set(got.Name.String(), wire)
		gotIdentity, gotSecret, ok := standard.BasicAuth()
		if !ok || gotIdentity != identity || gotSecret != string(secret) {
			t.Fatalf("Go BasicAuth accepted/identity/secret match=%t/%t/%t, want true/true/true", ok, gotIdentity == identity, gotSecret == string(secret))
		}
	})
}
