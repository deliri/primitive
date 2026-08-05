package controlwire_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
)

// FuzzRevisionUnmarshalJSON pressures the revision decoder, the boundary where
// a peer's bytes first become a typed contract.
//
// The oracle is not "did not panic". Every accepted document must validate,
// re-encode to the canonical token, and decode again to the same value; every
// rejected document must carry the stable revision identity and leave the
// receiver untouched.
func FuzzRevisionUnmarshalJSON(f *testing.F) {
	for _, seed := range []string{
		`"2026.1"`, `"2026.2"`, `"v1"`, `""`, `null`, `2026`, `true`,
		`["2026.1"]`, `{"revision":"2026.1"}`, `"2026.1"`, `"\ud800"`,
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		// The typed identity is asserted against the production decoder itself.
		// Routing the assertion through json.Unmarshal would let the standard
		// library's syntax scanner answer first for a malformed document, and a
		// stdlib syntax error is not proof that this package refused the value.
		direct := controlwire.Revision2026V1
		if err := direct.UnmarshalJSON(data); err != nil {
			if !errors.Is(err, core.ErrControlWireRevision) {
				t.Fatalf("UnmarshalJSON(%q) error = %v, want %v", data, err, core.ErrControlWireRevision)
			}
			if direct != controlwire.Revision2026V1 {
				t.Fatalf("rejected document mutated receiver to %v, want %v", direct, controlwire.Revision2026V1)
			}
		}

		got := controlwire.Revision2026V1
		err := json.Unmarshal(data, &got)
		if err != nil {
			if got != controlwire.Revision2026V1 {
				t.Fatalf("rejected document mutated receiver to %v, want %v", got, controlwire.Revision2026V1)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("accepted revision failed Validate(): %v", err)
		}
		encoded, err := json.Marshal(got)
		if err != nil {
			t.Fatalf("json.Marshal(accepted revision) error = %v, want nil", err)
		}
		var round controlwire.Revision
		if err := json.Unmarshal(encoded, &round); err != nil {
			t.Fatalf("json.Unmarshal(%s) error = %v, want nil", encoded, err)
		}
		if round != got {
			t.Fatalf("round trip = %v, want %v", round, got)
		}
	})
}

// FuzzParseRequestNonce pressures the nonce parser. The oracle proves an
// accepted nonce is nonzero, renders exactly the text it was parsed from, and
// survives a JSON round trip unchanged.
func FuzzParseRequestNonce(f *testing.F) {
	for _, seed := range []string{
		nonceHexWithLetters, nonceHexAllDigits, nonceHexAllZero, nonceHexAllF,
		"", "0x00", nonceHexWithLetters[:63], nonceHexWithLetters + "0",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, token string) {
		got, err := controlwire.ParseRequestNonce(token)
		if err != nil {
			if !errors.Is(err, core.ErrControlWireNonce) {
				t.Fatalf("ParseRequestNonce(%q) error = %v, want %v", token, err, core.ErrControlWireNonce)
			}
			if got.String() != "" {
				t.Fatalf("rejected nonce rendered %q, want empty", got.String())
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("accepted nonce failed Validate(): %v", err)
		}
		if got.String() != token {
			t.Fatalf("ParseRequestNonce(%q).String() = %q, want the exact input", token, got.String())
		}
		encoded, err := json.Marshal(got)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v, want nil", err)
		}
		var round controlwire.RequestNonce
		if err := json.Unmarshal(encoded, &round); err != nil {
			t.Fatalf("json.Unmarshal(%s) error = %v, want nil", encoded, err)
		}
		if round.String() != got.String() {
			t.Fatalf("round trip = %q, want %q", round.String(), got.String())
		}
	})
}

// FuzzParseRegistrationToken pressures the secret parser. The oracle proves an
// accepted token derives a stable verifier, never discloses itself through
// formatting, and re-encodes to the exact text it was parsed from.
func FuzzParseRegistrationToken(f *testing.F) {
	for _, seed := range []string{
		tokenHexWithLetters, tokenHexAllDigits, tokenHexAllZero,
		"", "0x00", tokenHexWithLetters[:63], tokenHexWithLetters + "0",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := controlwire.ParseRegistrationToken(data)
		if err != nil {
			if !errors.Is(err, core.ErrControlWireToken) {
				t.Fatalf("ParseRegistrationToken(%q) error = %v, want %v", data, err, core.ErrControlWireToken)
			}
			if validateErr := got.Validate(); validateErr == nil {
				t.Fatal("rejected token returned a usable value")
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("accepted token failed Validate(): %v", err)
		}
		verifier, err := got.Verifier()
		if err != nil {
			t.Fatalf("Verifier() error = %v, want nil", err)
		}
		if verifier.String() == string(data) {
			t.Fatal("verifier disclosed the token")
		}
		encoded, err := json.Marshal(got)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v, want nil", err)
		}
		if want := `"` + string(data) + `"`; string(encoded) != want {
			t.Fatalf("json.Marshal() = %s, want %s", encoded, want)
		}
		var round controlwire.RegistrationToken
		if err := json.Unmarshal(encoded, &round); err != nil {
			t.Fatalf("json.Unmarshal(%s) error = %v, want nil", encoded, err)
		}
		roundVerifier, err := round.Verifier()
		if err != nil {
			t.Fatalf("round trip Verifier() error = %v, want nil", err)
		}
		if !roundVerifier.Equal(verifier) {
			t.Fatalf("round trip verifier = %q, want %q", roundVerifier.String(), verifier.String())
		}
	})
}

// FuzzParseRegistrationTokenVerifier pressures the persisted-value ingress. A
// verifier arrives from a store rather than from a peer, so the hostile input
// here is a corrupted, truncated, or re-cased record rather than a crafted
// request. The oracle proves an accepted verifier is never the impossible
// all-zero digest, renders exactly the text it was parsed from, and recognises
// itself and nothing else.
func FuzzParseRegistrationTokenVerifier(f *testing.F) {
	derived, err := controlwire.ParseRegistrationToken([]byte(tokenHexWithLetters))
	if err != nil {
		f.Fatalf("ParseRegistrationToken() error = %v, want nil", err)
	}
	verifier, err := derived.Verifier()
	if err != nil {
		f.Fatalf("Verifier() error = %v, want nil", err)
	}
	for _, seed := range []string{
		verifier.String(), verifierHexAllZero, strings.Repeat("f", 64),
		"", "0x00", verifier.String()[:63], verifier.String() + "0",
		strings.ToUpper(verifier.String()),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, text string) {
		got, err := controlwire.ParseRegistrationTokenVerifier(text)
		if err != nil {
			if !errors.Is(err, core.ErrControlWireToken) {
				t.Fatalf("ParseRegistrationTokenVerifier(%q) error = %v, want %v", text, err, core.ErrControlWireToken)
			}
			if got.String() != "" {
				t.Fatalf("rejected verifier rendered %q, want empty", got.String())
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("accepted verifier failed Validate(): %v", err)
		}
		if text == verifierHexAllZero {
			t.Fatal("accepted the all-zero digest, which no token can derive")
		}
		if got.String() != text {
			t.Fatalf("ParseRegistrationTokenVerifier(%q).String() = %q, want the exact input", text, got.String())
		}
		if !got.Equal(got) {
			t.Fatalf("accepted verifier %q does not recognise itself", text)
		}
		encoded, err := json.Marshal(got)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v, want nil", err)
		}
		var round controlwire.RegistrationTokenVerifier
		if err := json.Unmarshal(encoded, &round); err != nil {
			t.Fatalf("json.Unmarshal(%s) error = %v, want nil", encoded, err)
		}
		if !round.Equal(got) {
			t.Fatalf("round trip verifier = %q, want %q", round.String(), got.String())
		}
	})
}
