package controlwire_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
)

// verifierHexAllZero is the digest SHA-256 cannot produce. It is the value a
// blank or truncated persisted record decodes to, which is why it is the one
// digest this type refuses.
const verifierHexAllZero = "0000000000000000000000000000000000000000000000000000000000000000"

// TestRegistrationTokenVerifierRefusesTheImpossibleDigest is the reason the
// all-zero check exists.
//
// SHA-256 has no known preimage for the all-zero digest, so no token can ever
// derive it. What does produce it is a blank, truncated, or default-initialised
// persisted record. Admitting it would let two such records compare Equal and
// recognise each other as the same enrolment, and it would let a store hand back
// a verifier it never wrote. RequestNonce already refuses its own impossible
// value; this is the same rule on the same width.
func TestRegistrationTokenVerifierRefusesTheImpossibleDigest(t *testing.T) {
	t.Parallel()

	t.Run("text ingress refuses it", func(t *testing.T) {
		t.Parallel()

		got, err := controlwire.ParseRegistrationTokenVerifier(verifierHexAllZero)
		if !errors.Is(err, core.ErrControlWireToken) {
			t.Fatalf("ParseRegistrationTokenVerifier(all zero) error = %v, want %v", err, core.ErrControlWireToken)
		}
		if !errors.Is(err, core.ErrControlWireContract) {
			t.Fatalf("ParseRegistrationTokenVerifier(all zero) error = %v, want %v", err, core.ErrControlWireContract)
		}
		if got.String() != "" {
			t.Fatalf("ParseRegistrationTokenVerifier(all zero) = %q, want the zero verifier", got.String())
		}
	})

	t.Run("json ingress refuses it", func(t *testing.T) {
		t.Parallel()

		var got controlwire.RegistrationTokenVerifier
		err := json.Unmarshal([]byte(`"`+verifierHexAllZero+`"`), &got)
		if !errors.Is(err, core.ErrControlWireToken) {
			t.Fatalf("json.Unmarshal(all zero verifier) error = %v, want %v", err, core.ErrControlWireToken)
		}
		if got.String() != "" {
			t.Fatalf("refused decode left %q, want the zero verifier", got.String())
		}
	})

	t.Run("two refused blank records remain neutral and do not recognise each other", func(t *testing.T) {
		t.Parallel()

		var first, second controlwire.RegistrationTokenVerifier
		document := []byte(`"` + verifierHexAllZero + `"`)
		firstErr := json.Unmarshal(document, &first)
		secondErr := json.Unmarshal(document, &second)
		for position, refusal := range []error{firstErr, secondErr} {
			if !errors.Is(refusal, core.ErrJSONContract) ||
				!errors.Is(refusal, core.ErrControlWireContract) ||
				!errors.Is(refusal, core.ErrControlWireToken) {
				t.Fatalf(
					"json.Unmarshal(all zero verifier) refusal %d = %v, want %v/%v/%v",
					position, refusal, core.ErrJSONContract, core.ErrControlWireContract, core.ErrControlWireToken,
				)
			}
		}
		if first.String() != "" || second.String() != "" {
			t.Fatalf("refused blank receivers = (%q, %q), want two zero verifiers", first.String(), second.String())
		}
		if got := first.Equal(second); got {
			t.Fatalf("refused blank verifiers %v.Equal(%v) = %t, want false", first, second, got)
		}
		if err := first.Validate(); !errors.Is(err, core.ErrControlWireToken) {
			t.Fatalf("all-zero verifier Validate() error = %v, want %v", err, core.ErrControlWireToken)
		}
	})
}

// TestParseRegistrationTokenVerifierEnforcesExactCanonicalHexadecimal pressures
// the text ingress a control plane uses when it reads a persisted verifier back.
// That path takes bytes from a store rather than from a peer, so it is exactly
// where a truncated column, a re-cased export, or a padded field arrives.
func TestParseRegistrationTokenVerifierEnforcesExactCanonicalHexadecimal(t *testing.T) {
	t.Parallel()

	valid, err := mustToken(t, tokenHexWithLetters).Verifier()
	if err != nil {
		t.Fatalf("Verifier() error = %v, want nil", err)
	}
	canonical := valid.String()
	// Case rules are only provable against a character that has a case. The
	// digest is data-dependent, so the position of its first letter is found
	// rather than assumed; uppercasing a digit would make the case vacuous.
	uppercasedLetter := upperFirstHexLetter(t, canonical)

	cases := []struct {
		name     string
		text     string
		wantOkay bool
	}{
		{name: "canonical verifier text is accepted", text: canonical, wantOkay: true},
		{name: "all-f digest is accepted", text: strings.Repeat("f", 64), wantOkay: true},
		{name: "single set nibble is accepted", text: strings.Repeat("0", 63) + "1", wantOkay: true},
		{name: "leading set nibble is accepted", text: "1" + strings.Repeat("0", 63), wantOkay: true},
		{name: "all-zero digest is refused as impossible", text: verifierHexAllZero},
		{name: "empty text is refused", text: ""},
		{name: "one character short is refused", text: canonical[:63]},
		{name: "one character long is refused", text: canonical + "0"},
		{name: "half width is refused", text: canonical[:32]},
		{name: "double width is refused", text: canonical + canonical},
		{name: "uppercase hex is refused as non-canonical", text: strings.ToUpper(canonical)},
		{name: "single uppercase letter is refused", text: uppercasedLetter},
		{name: "0x prefix is refused", text: "0x" + canonical[:62]},
		{name: "non-hex letter is refused", text: strings.Repeat("z", 64)},
		{name: "trailing space is refused", text: canonical[:63] + " "},
		{name: "leading space is refused", text: " " + canonical[:63]},
		{name: "interior space is refused", text: canonical[:32] + " " + canonical[33:]},
		{name: "trailing newline is refused", text: canonical[:63] + "\n"},
		{name: "null byte is refused", text: canonical[:63] + "\x00"},
		{name: "fullwidth digit lookalike is refused", text: strings.Repeat("２", 64)},
		{name: "oversized text is refused", text: strings.Repeat("a", 4096)},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, err := controlwire.ParseRegistrationTokenVerifier(testCase.text)
			if testCase.wantOkay {
				if err != nil {
					t.Fatalf("ParseRegistrationTokenVerifier(%q) error = %v, want nil", testCase.text, err)
				}
				if got.String() != testCase.text {
					t.Fatalf("ParseRegistrationTokenVerifier(%q).String() = %q, want the exact text", testCase.text, got.String())
				}
				return
			}
			if !errors.Is(err, core.ErrControlWireToken) {
				t.Fatalf("ParseRegistrationTokenVerifier(%q) error = %v, want %v", testCase.text, err, core.ErrControlWireToken)
			}
			if got.String() != "" {
				t.Fatalf("ParseRegistrationTokenVerifier(%q) = %q, want the zero verifier", testCase.text, got.String())
			}
		})
	}
}

// TestParseRegistrationTokenVerifierAgreesWithTheDerivedVerifier proves the two
// ways a verifier enters the process produce the same value. A control plane
// derives one from a presented token and reads the other from its store, then
// compares them; if those two paths disagreed, no enrolment would ever match.
func TestParseRegistrationTokenVerifierAgreesWithTheDerivedVerifier(t *testing.T) {
	t.Parallel()

	for _, text := range []string{tokenHexWithLetters, tokenHexAllDigits, tokenHexMinimumEntropy} {
		derived, err := mustToken(t, text).Verifier()
		if err != nil {
			t.Fatalf("Verifier() error = %v, want nil", err)
		}
		persisted, err := controlwire.ParseRegistrationTokenVerifier(derived.String())
		if err != nil {
			t.Fatalf("ParseRegistrationTokenVerifier(%q) error = %v, want nil", derived.String(), err)
		}
		if !persisted.Equal(derived) {
			t.Fatalf("persisted verifier = %q, want %q", persisted.String(), derived.String())
		}
		encoded, err := json.Marshal(persisted)
		if err != nil {
			t.Fatalf("json.Marshal(persisted) error = %v, want nil", err)
		}
		if want := `"` + derived.String() + `"`; string(encoded) != want {
			t.Fatalf("json.Marshal(persisted) = %s, want %s", encoded, want)
		}
	}
}

// TestDestroyingOneTokenHandleDisablesEveryCopy proves the release reaches every
// holder. RegistrationToken carries Core's shared secret handle, so a copy
// placed in a request struct is not a second secret. A caller that destroyed its
// own token and found the copy still able to marshal would be putting a spent
// credential back on the wire.
func TestDestroyingOneTokenHandleDisablesEveryCopy(t *testing.T) {
	t.Parallel()

	original := mustToken(t, tokenHexWithLetters)
	copied := original
	nested := struct{ Token controlwire.RegistrationToken }{Token: original}

	if err := original.Destroy(); err != nil {
		t.Fatalf("Destroy() error = %v, want nil", err)
	}
	for _, holder := range []struct {
		token controlwire.RegistrationToken
		name  string
	}{
		{name: "the destroyed handle", token: original},
		{name: "a copied handle", token: copied},
		{name: "a handle inside a struct", token: nested.Token},
	} {
		if err := holder.token.Validate(); !errors.Is(err, core.ErrControlWireToken) {
			t.Errorf("%s Validate() error = %v, want %v", holder.name, err, core.ErrControlWireToken)
		}
		if _, err := holder.token.Verifier(); !errors.Is(err, core.ErrControlWireToken) {
			t.Errorf("%s Verifier() error = %v, want %v", holder.name, err, core.ErrControlWireToken)
		}
		encoded, err := json.Marshal(holder.token)
		if !errors.Is(err, core.ErrControlWireToken) || encoded != nil {
			t.Errorf("json.Marshal(%s) = (%s, %v), want no bytes and %v",
				holder.name, encoded, err, core.ErrControlWireToken)
		}
	}
	if err := original.Destroy(); err != nil {
		t.Fatalf("repeated Destroy() error = %v, want nil", err)
	}
}

// upperFirstHexLetter uppercases the first a-f character in value. A digest is
// data-dependent, so a fixed offset could land on a digit and silently turn the
// non-canonical case into a canonical one.
func upperFirstHexLetter(t *testing.T, value string) string {
	t.Helper()

	for index := range len(value) {
		if value[index] >= 'a' && value[index] <= 'f' {
			return value[:index] + strings.ToUpper(value[index:index+1]) + value[index+1:]
		}
	}
	t.Fatalf("value %q contains no hexadecimal letter to uppercase", value)
	return ""
}
