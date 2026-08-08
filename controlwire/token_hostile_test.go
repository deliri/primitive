package controlwire_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
)

const (
	tokenHexWithLetters = "a3f1c8be5d2740196b8e0caf3d5b7128e94a6f03bc17d582ea409fb6c3d81e57"
	tokenHexAllDigits   = "2222222222222222222222222222222222222222222222222222222222222222"
	tokenHexAllZero     = "0000000000000000000000000000000000000000000000000000000000000000"
	// tokenHexMinimumEntropy is the smallest token Core's SecretMaterial will
	// hold: every byte zero but one. It proves the all-zero refusal is a floor
	// on the value rather than a rejection of low-looking tokens generally.
	tokenHexMinimumEntropy = "0000000000000000000000000000000000000000000000000000000000000001"
)

func mustToken(t *testing.T, token string) controlwire.RegistrationToken {
	t.Helper()

	got, err := controlwire.ParseRegistrationToken([]byte(token))
	if err != nil {
		t.Fatalf("ParseRegistrationToken(%q) error = %v, want nil", token, err)
	}
	return got
}

func TestParseRegistrationTokenEnforcesExactCanonicalHexadecimal(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		token    string
		wantOkay bool
	}{
		{name: "canonical lowercase hex with letters is accepted", token: tokenHexWithLetters, wantOkay: true},
		{name: "canonical all-digit hex is accepted", token: tokenHexAllDigits, wantOkay: true},
		{name: "all-f token is accepted", token: strings.Repeat("f", 64), wantOkay: true},
		{name: "single set bit is accepted as the entropy floor", token: tokenHexMinimumEntropy, wantOkay: true},
		{name: "all-zero token is refused because it is not secret material", token: tokenHexAllZero},
		{name: "empty token is refused", token: ""},
		{name: "one character short is refused", token: tokenHexWithLetters[:63]},
		{name: "one character long is refused", token: tokenHexWithLetters + "0"},
		{name: "half width is refused", token: tokenHexWithLetters[:32]},
		{name: "double width is refused", token: tokenHexWithLetters + tokenHexWithLetters},
		{name: "uppercase hex is refused as non-canonical", token: strings.ToUpper(tokenHexWithLetters)},
		{name: "single uppercase character is refused as non-canonical", token: "A3f1c8be5d2740196b8e0caf3d5b7128e94a6f03bc17d582ea409fb6c3d81e57"},
		{name: "0x prefix is refused", token: "0x" + tokenHexWithLetters[:62]},
		{name: "non-hex letter is refused", token: strings.Repeat("z", 64)},
		{name: "odd-length token is refused", token: tokenHexWithLetters[:61]},
		{name: "trailing space is refused", token: tokenHexWithLetters[:63] + " "},
		{name: "leading space is refused", token: " " + tokenHexWithLetters[:63]},
		{name: "trailing newline is refused", token: tokenHexWithLetters[:63] + "\n"},
		{name: "null byte is refused", token: tokenHexWithLetters[:63] + "\x00"},
		{name: "hyphenated token is refused", token: "a3f1c8be-5d27-4019-6b8e-0caf3d5b7128e94a6f03bc17d582ea409fb6c3d8"},
		{name: "fullwidth digit lookalike is refused", token: strings.Repeat("２", 64)},
		{name: "oversized token is refused", token: strings.Repeat("a", 4096)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := controlwire.ParseRegistrationToken([]byte(tc.token))
			if tc.wantOkay {
				if err != nil {
					t.Fatalf("ParseRegistrationToken(%q) error = %v, want nil", tc.token, err)
				}
				if err := got.Validate(); err != nil {
					t.Fatalf("ParseRegistrationToken(%q).Validate() error = %v, want nil", tc.token, err)
				}
				return
			}
			if !errors.Is(err, core.ErrControlWireToken) {
				t.Fatalf("ParseRegistrationToken(%q) error = %v, want %v", tc.token, err, core.ErrControlWireToken)
			}
			if !errors.Is(err, core.ErrControlWireContract) {
				t.Fatalf("ParseRegistrationToken(%q) error = %v, want %v", tc.token, err, core.ErrControlWireContract)
			}
			if err := got.Validate(); err == nil {
				t.Fatalf("ParseRegistrationToken(%q) returned a usable token on rejection", tc.token)
			}
		})
	}
}

// TestParseRegistrationTokenDoesNotRetainTheCallerSlice proves the parser owns
// its bytes. A parser that aliased the caller's buffer would let a caller wipe
// its own copy and silently mutate the stored secret.
func TestParseRegistrationTokenDoesNotRetainTheCallerSlice(t *testing.T) {
	t.Parallel()

	source := []byte(tokenHexWithLetters)
	token, err := controlwire.ParseRegistrationToken(source)
	if err != nil {
		t.Fatalf("ParseRegistrationToken() error = %v, want nil", err)
	}
	want, err := token.Verifier()
	if err != nil {
		t.Fatalf("Verifier() error = %v, want nil", err)
	}
	clear(source)
	got, err := token.Verifier()
	if err != nil {
		t.Fatalf("Verifier() after caller wipe error = %v, want nil", err)
	}
	if !got.Equal(want) {
		t.Fatalf("Verifier() after caller wipe = %q, want %q", got.String(), want.String())
	}
}

// TestRegistrationTokenFormatRedactsEveryVerb proves no formatting path can
// print an unspent enrolment secret. A single unredacted verb is a disclosure
// bug, so every common verb is exercised rather than a sample.
func TestRegistrationTokenFormatRedactsEveryVerb(t *testing.T) {
	t.Parallel()

	token := mustToken(t, tokenHexWithLetters)
	for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%q", "%x", "%X", "%d", "%08s", "%-20v"} {
		got := fmt.Sprintf(verb, token)
		if got != core.RedactedValueText {
			t.Errorf("fmt.Sprintf(%q, token) = %q, want %q", verb, got, core.RedactedValueText)
		}
	}
	// fmt resolves %T and %p before consulting Formatter, so no type can redact
	// them. They must still disclose nothing: %T names the type, and %p falls
	// through to reflection printing, which reaches the secret only if
	// SecretMaterial ever stops holding its bytes behind a pointer. That is the
	// bug this asserts against.
	for _, verb := range []string{"%T", "%p"} {
		if got := fmt.Sprintf(verb, token); strings.Contains(got, tokenHexWithLetters) {
			t.Errorf("fmt.Sprintf(%q, token) = %q, disclosed the token", verb, got)
		}
	}
	for _, digit := range []string{"a3f1", "c8be", "5d27", "e94a", "1e57"} {
		if got := fmt.Sprintf("%p", token); strings.Contains(got, digit) {
			t.Errorf("fmt.Sprintf(%%p, token) = %q, leaked token fragment %q", got, digit)
		}
	}
	// A token nested inside a wrapped error must not disclose either. The
	// rendered text is searched for the secret, never for an error contract:
	// this is a redaction proof over the operator-facing rendering, and the
	// rejection contract stays with the typed identities above.
	wrapped := fmt.Errorf("registration failed for %v: %w", token, core.ErrControlWireToken)
	if rendered := fmt.Sprint(wrapped); strings.Contains(rendered, tokenHexWithLetters) {
		t.Fatalf("wrapped error rendering = %q, want it to omit the token text", rendered)
	}
}

// TestRegistrationTokenVerifierIsTheSHA256OfTheSecret pins the one-way
// derivation against an independently computed digest, so a future change to
// the derivation cannot silently invalidate every persisted verifier.
func TestRegistrationTokenVerifierIsTheSHA256OfTheSecret(t *testing.T) {
	t.Parallel()

	for _, text := range []string{tokenHexWithLetters, tokenHexAllDigits, tokenHexMinimumEntropy} {
		raw, err := hex.DecodeString(text)
		if err != nil {
			t.Fatalf("hex.DecodeString(%q) error = %v, want nil", text, err)
		}
		digest := sha256.Sum256(raw)
		want := hex.EncodeToString(digest[:])

		verifier, err := mustToken(t, text).Verifier()
		if err != nil {
			t.Fatalf("Verifier() error = %v, want nil", err)
		}
		if got := verifier.String(); got != want {
			t.Fatalf("Verifier().String() = %q, want %q", got, want)
		}
		if got := verifier.String(); got == text {
			t.Fatalf("Verifier().String() returned the secret itself")
		}
	}
}

func TestRegistrationTokenVerifierEqualSeparatesDistinctTokens(t *testing.T) {
	t.Parallel()

	first, err := mustToken(t, tokenHexWithLetters).Verifier()
	if err != nil {
		t.Fatalf("Verifier() error = %v, want nil", err)
	}
	same, err := mustToken(t, tokenHexWithLetters).Verifier()
	if err != nil {
		t.Fatalf("Verifier() error = %v, want nil", err)
	}
	other, err := mustToken(t, tokenHexAllDigits).Verifier()
	if err != nil {
		t.Fatalf("Verifier() error = %v, want nil", err)
	}
	var unset controlwire.RegistrationTokenVerifier

	if !first.Equal(same) {
		t.Error("Equal(same token) = false, want true")
	}
	if first.Equal(other) {
		t.Error("Equal(different token) = true, want false")
	}
	if first.Equal(unset) {
		t.Error("Equal(unset verifier) = true, want false")
	}
	if unset.Equal(unset) {
		t.Error("unset.Equal(unset) = true, want false")
	}
	if err := unset.Validate(); !errors.Is(err, core.ErrControlWireToken) {
		t.Fatalf("unset verifier Validate() error = %v, want %v", err, core.ErrControlWireToken)
	}
	if got := unset.String(); got != "" {
		t.Fatalf("unset verifier String() = %q, want empty", got)
	}
}

// TestDestroyedRegistrationTokenCannotBeUsed proves destruction is real: after
// the secret is released the token refuses to validate, marshal, or derive a
// verifier rather than serving stale bytes.
func TestDestroyedRegistrationTokenCannotBeUsed(t *testing.T) {
	t.Parallel()

	token := mustToken(t, tokenHexWithLetters)
	if err := token.Destroy(); err != nil {
		t.Fatalf("Destroy() error = %v, want nil", err)
	}
	if err := token.Validate(); !errors.Is(err, core.ErrControlWireToken) {
		t.Fatalf("destroyed token Validate() error = %v, want %v", err, core.ErrControlWireToken)
	}
	if _, err := token.Verifier(); !errors.Is(err, core.ErrControlWireToken) {
		t.Fatalf("destroyed token Verifier() error = %v, want %v", err, core.ErrControlWireToken)
	}
	encoded, err := json.Marshal(token)
	if !errors.Is(err, core.ErrControlWireToken) {
		t.Fatalf("json.Marshal(destroyed token) error = %v, want %v", err, core.ErrControlWireToken)
	}
	if encoded != nil {
		t.Fatalf("json.Marshal(destroyed token) = %s, want no bytes", encoded)
	}
}

func TestZeroRegistrationTokenIsRejectedEverywhere(t *testing.T) {
	t.Parallel()

	var token controlwire.RegistrationToken
	if err := token.Validate(); !errors.Is(err, core.ErrControlWireToken) {
		t.Fatalf("zero token Validate() error = %v, want %v", err, core.ErrControlWireToken)
	}
	if _, err := token.Verifier(); !errors.Is(err, core.ErrControlWireToken) {
		t.Fatalf("zero token Verifier() error = %v, want %v", err, core.ErrControlWireToken)
	}
	if _, err := json.Marshal(token); !errors.Is(err, core.ErrControlWireToken) {
		t.Fatalf("json.Marshal(zero token) error = %v, want %v", err, core.ErrControlWireToken)
	}
	if got := fmt.Sprintf("%v", token); got != core.RedactedValueText {
		t.Fatalf("fmt.Sprintf(zero token) = %q, want %q", got, core.RedactedValueText)
	}
}

func TestRegistrationTokenJSONRejectionLeavesTheReceiverUnchanged(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		document string
		wantOkay bool
	}{
		{name: "canonical token is accepted", document: `"` + tokenHexWithLetters + `"`, wantOkay: true},
		{name: "json null is refused", document: `null`},
		{name: "number is refused", document: `1`},
		{name: "boolean is refused", document: `true`},
		{name: "empty string is refused", document: `""`},
		{name: "object is refused", document: `{"registration_token":"` + tokenHexWithLetters + `"}`},
		{name: "array is refused", document: `["` + tokenHexWithLetters + `"]`},
		{name: "uppercase hex is refused", document: `"` + strings.ToUpper(tokenHexWithLetters) + `"`},
		{name: "truncated token is refused", document: `"` + tokenHexWithLetters[:63] + `"`},
		{name: "overlong token is refused", document: `"` + tokenHexWithLetters + `0"`},
		{name: "unterminated string is refused", document: `"` + tokenHexWithLetters},
		{name: "unpaired high surrogate is refused", document: `"\ud800"`},
		{name: "empty document is refused", document: ``},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			prior := mustToken(t, tokenHexAllDigits)
			priorVerifier, err := prior.Verifier()
			if err != nil {
				t.Fatalf("Verifier() error = %v, want nil", err)
			}
			got := prior
			err = json.Unmarshal([]byte(tc.document), &got)
			gotVerifier, verifierErr := got.Verifier()
			if tc.wantOkay {
				if err != nil {
					t.Fatalf("json.Unmarshal(%s) error = %v, want nil", tc.document, err)
				}
				if verifierErr != nil {
					t.Fatalf("Verifier() after accept error = %v, want nil", verifierErr)
				}
				want, err := mustToken(t, tokenHexWithLetters).Verifier()
				if err != nil {
					t.Fatalf("Verifier() error = %v, want nil", err)
				}
				if !gotVerifier.Equal(want) {
					t.Fatalf("json.Unmarshal(%s) verifier = %q, want %q", tc.document, gotVerifier.String(), want.String())
				}
				return
			}
			if err == nil {
				t.Fatalf("json.Unmarshal(%s) error = nil, want a rejection", tc.document)
			}
			if verifierErr != nil {
				t.Fatalf("Verifier() after rejection error = %v, want nil", verifierErr)
			}
			if !gotVerifier.Equal(priorVerifier) {
				t.Fatalf("json.Unmarshal(%s) mutated receiver to %q, want %q", tc.document, gotVerifier.String(), priorVerifier.String())
			}
		})
	}
}

func TestRegistrationTokenUnmarshalJSONOnNilReceiverIsRejected(t *testing.T) {
	t.Parallel()

	var token *controlwire.RegistrationToken
	if err := token.UnmarshalJSON([]byte(`"` + tokenHexWithLetters + `"`)); !errors.Is(err, core.ErrControlWireToken) {
		t.Fatalf("(*RegistrationToken)(nil).UnmarshalJSON() error = %v, want %v", err, core.ErrControlWireToken)
	}
	var verifier *controlwire.RegistrationTokenVerifier
	if err := verifier.UnmarshalJSON([]byte(`"` + tokenHexAllZero + `"`)); !errors.Is(err, core.ErrControlWireToken) {
		t.Fatalf("(*RegistrationTokenVerifier)(nil).UnmarshalJSON() error = %v, want %v", err, core.ErrControlWireToken)
	}
}

// TestRegistrationTokenRoundTripsWithoutMovingAByte proves the single
// deliberate crossing point encodes exactly what it parsed.
func TestRegistrationTokenRoundTripsWithoutMovingAByte(t *testing.T) {
	t.Parallel()

	for _, text := range []string{tokenHexWithLetters, tokenHexAllDigits, tokenHexMinimumEntropy} {
		encoded, err := json.Marshal(mustToken(t, text))
		if err != nil {
			t.Fatalf("json.Marshal(%q) error = %v, want nil", text, err)
		}
		if want := `"` + text + `"`; string(encoded) != want {
			t.Fatalf("json.Marshal(%q) = %s, want %s", text, encoded, want)
		}
		var decoded controlwire.RegistrationToken
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			t.Fatalf("json.Unmarshal(%s) error = %v, want nil", encoded, err)
		}
		reencoded, err := json.Marshal(decoded)
		if err != nil {
			t.Fatalf("re-marshal error = %v, want nil", err)
		}
		if string(reencoded) != string(encoded) {
			t.Fatalf("re-marshal = %s, want %s", reencoded, encoded)
		}
	}
}

func TestRegistrationTokenVerifierRoundTripsThroughJSON(t *testing.T) {
	t.Parallel()

	want, err := mustToken(t, tokenHexWithLetters).Verifier()
	if err != nil {
		t.Fatalf("Verifier() error = %v, want nil", err)
	}
	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("json.Marshal(verifier) error = %v, want nil", err)
	}
	if got, wantText := string(encoded), `"`+want.String()+`"`; got != wantText {
		t.Fatalf("json.Marshal(verifier) = %s, want %s", got, wantText)
	}
	var got controlwire.RegistrationTokenVerifier
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("json.Unmarshal(%s) error = %v, want nil", encoded, err)
	}
	if !got.Equal(want) {
		t.Fatalf("round trip verifier = %q, want %q", got.String(), want.String())
	}
}
