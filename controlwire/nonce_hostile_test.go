package controlwire_test

import (
	json "encoding/json/v2"
	"errors"
	"strings"
	"testing"

	"encoding/json/jsontext"

	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
)

const (
	// nonceHexWithLetters is a canonical nonce whose text contains hexadecimal
	// letters. Case rules are only provable against a value that has a case, so
	// an all-digit fixture would make every uppercase assertion vacuous.
	nonceHexWithLetters = "4527438986c634a958a42640c82dea45faaab23f36b9f62e6c5606814e2cebf7"
	nonceHexAllDigits   = "2121212121212121212121212121212121212121212121212121212121212121"
	nonceHexAllZero     = "0000000000000000000000000000000000000000000000000000000000000000"
	nonceHexAllF        = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
)

func TestNewRequestNonceRefusesTheUnpredictableFloor(t *testing.T) {
	t.Parallel()

	cases := []struct {
		build    func() [core.SHA256DigestBytes]byte
		name     string
		wantOkay bool
	}{
		{
			name:     "all-zero nonce is refused because it is not unpredictable",
			build:    func() [core.SHA256DigestBytes]byte { return [core.SHA256DigestBytes]byte{} },
			wantOkay: false,
		},
		{
			name: "single set bit in the last byte is accepted",
			build: func() [core.SHA256DigestBytes]byte {
				var value [core.SHA256DigestBytes]byte
				value[core.SHA256DigestBytes-1] = 1
				return value
			},
			wantOkay: true,
		},
		{
			name: "single set bit in the first byte is accepted",
			build: func() [core.SHA256DigestBytes]byte {
				var value [core.SHA256DigestBytes]byte
				value[0] = 1
				return value
			},
			wantOkay: true,
		},
		{
			name: "every byte set is accepted",
			build: func() [core.SHA256DigestBytes]byte {
				var value [core.SHA256DigestBytes]byte
				for index := range value {
					value[index] = 0xff
				}
				return value
			},
			wantOkay: true,
		},
		{
			name: "all bytes zero except a middle byte is accepted",
			build: func() [core.SHA256DigestBytes]byte {
				var value [core.SHA256DigestBytes]byte
				value[core.SHA256DigestBytes/2] = 0x80
				return value
			},
			wantOkay: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := controlwire.NewRequestNonce(tc.build())
			if tc.wantOkay {
				if err != nil {
					t.Fatalf("NewRequestNonce() error = %v, want nil", err)
				}
				if err := got.Validate(); err != nil {
					t.Fatalf("NewRequestNonce().Validate() error = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, core.ErrControlWireNonce) {
				t.Fatalf("NewRequestNonce() error = %v, want %v", err, core.ErrControlWireNonce)
			}
			if got.String() != "" {
				t.Fatalf("NewRequestNonce() = %q, want the zero nonce", got.String())
			}
		})
	}
}

func TestParseRequestNonceEnforcesExactCanonicalHexadecimal(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		token    string
		wantOkay bool
	}{
		{name: "canonical lowercase hex with letters is accepted", token: nonceHexWithLetters, wantOkay: true},
		{name: "canonical all-digit hex is accepted", token: nonceHexAllDigits, wantOkay: true},
		{name: "all-f nonce is accepted", token: nonceHexAllF, wantOkay: true},
		{name: "minimum nonzero nonce is accepted", token: strings.Repeat("0", 63) + "1", wantOkay: true},
		{name: "leading-nibble-only nonce is accepted", token: "1" + strings.Repeat("0", 63), wantOkay: true},
		{name: "all-zero nonce is refused", token: nonceHexAllZero},
		{name: "empty token is refused", token: ""},
		{name: "one character short is refused", token: nonceHexWithLetters[:63]},
		{name: "one character long is refused", token: nonceHexWithLetters + "0"},
		{name: "half width is refused", token: nonceHexWithLetters[:32]},
		{name: "double width is refused", token: nonceHexWithLetters + nonceHexWithLetters},
		{name: "uppercase hex is refused as non-canonical", token: strings.ToUpper(nonceHexWithLetters)},
		{name: "single uppercase character is refused as non-canonical", token: "4527438986C634a958a42640c82dea45faaab23f36b9f62e6c5606814e2cebf7"},
		{name: "0x prefix is refused", token: "0x" + nonceHexWithLetters[:62]},
		{name: "non-hex letter is refused", token: strings.Repeat("g", 64)},
		{name: "trailing space is refused", token: nonceHexWithLetters[:63] + " "},
		{name: "leading space is refused", token: " " + nonceHexWithLetters[:63]},
		{name: "interior space is refused", token: nonceHexWithLetters[:32] + " " + nonceHexWithLetters[33:]},
		{name: "trailing newline is refused", token: nonceHexWithLetters[:63] + "\n"},
		{name: "null byte is refused", token: nonceHexWithLetters[:63] + "\x00"},
		{name: "hyphenated uuid shape is refused", token: "45274389-86c6-34a9-58a4-2640c82dea45faaab23f36b9f62e6c5606814e2c"},
		{name: "base64-shaped token is refused", token: strings.Repeat("QUJD", 16)},
		{name: "fullwidth digit lookalike is refused", token: strings.Repeat("２", 64)},
		{name: "oversized token is refused", token: strings.Repeat("a", 4096)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := controlwire.ParseRequestNonce(tc.token)
			if tc.wantOkay {
				if err != nil {
					t.Fatalf("ParseRequestNonce(%q) error = %v, want nil", tc.token, err)
				}
				if got.String() != tc.token {
					t.Fatalf("ParseRequestNonce(%q).String() = %q, want %q", tc.token, got.String(), tc.token)
				}
				return
			}
			if !errors.Is(err, core.ErrControlWireNonce) {
				t.Fatalf("ParseRequestNonce(%q) error = %v, want %v", tc.token, err, core.ErrControlWireNonce)
			}
			if !errors.Is(err, core.ErrControlWireContract) {
				t.Fatalf("ParseRequestNonce(%q) error = %v, want %v", tc.token, err, core.ErrControlWireContract)
			}
			if got.String() != "" {
				t.Fatalf("ParseRequestNonce(%q) = %q, want the zero nonce", tc.token, got.String())
			}
		})
	}
}

// TestZeroRequestNonceIsRejectedEverywhere proves the unset value cannot leak
// through any exported path. A zero nonce that renders, marshals, or projects
// would let a caller send a request identity it never obtained.
func TestZeroRequestNonceIsRejectedEverywhere(t *testing.T) {
	t.Parallel()

	var nonce controlwire.RequestNonce
	if err := nonce.Validate(); !errors.Is(err, core.ErrControlWireNonce) {
		t.Fatalf("zero RequestNonce.Validate() error = %v, want %v", err, core.ErrControlWireNonce)
	}
	if got := nonce.String(); got != "" {
		t.Fatalf("zero RequestNonce.String() = %q, want empty", got)
	}
	if _, err := json.Marshal(nonce); !errors.Is(err, core.ErrControlWireNonce) {
		t.Fatalf("json.Marshal(zero RequestNonce) error = %v, want %v", err, core.ErrControlWireNonce)
	}
	if _, err := nonce.IdempotencyKey(); !errors.Is(err, core.ErrControlWireNonce) {
		t.Fatalf("zero RequestNonce.IdempotencyKey() error = %v, want %v", err, core.ErrControlWireNonce)
	}
}

// TestGenerateRequestNonceProducesDistinctValidNonces proves the generator
// reaches the real entropy substrate. A generator that returned a fixed or zero
// value would pass a validity check alone, so distinctness is asserted too.
func TestGenerateRequestNonceProducesDistinctValidNonces(t *testing.T) {
	t.Parallel()

	const draws = 64
	seen := make(map[string]struct{}, draws)
	for draw := 1; draw <= draws; draw++ {
		nonce, err := controlwire.GenerateRequestNonce()
		if err != nil {
			t.Fatalf("GenerateRequestNonce() draw %d error = %v, want nil", draw, err)
		}
		if err := nonce.Validate(); err != nil {
			t.Fatalf("GenerateRequestNonce() draw %d produced an invalid nonce: %v", draw, err)
		}
		text := nonce.String()
		if len(text) != 2*core.SHA256DigestBytes {
			t.Fatalf("GenerateRequestNonce() draw %d text width = %d, want %d", draw, len(text), 2*core.SHA256DigestBytes)
		}
		if _, repeated := seen[text]; repeated {
			t.Fatalf("GenerateRequestNonce() draw %d repeated a prior nonce", draw)
		}
		seen[text] = struct{}{}
		if _, err := controlwire.ParseRequestNonce(text); err != nil {
			t.Fatalf("GenerateRequestNonce() draw %d produced unparseable text %q: %v", draw, text, err)
		}
	}
}

// TestRequestNonceIdempotencyKeyCarriesTheExactNonceText proves the projection
// does not reformat, truncate, or re-case the identity both ends must agree on.
func TestRequestNonceIdempotencyKeyCarriesTheExactNonceText(t *testing.T) {
	t.Parallel()

	nonce, err := controlwire.ParseRequestNonce(nonceHexWithLetters)
	if err != nil {
		t.Fatalf("ParseRequestNonce() error = %v, want nil", err)
	}
	key, err := nonce.IdempotencyKey()
	if err != nil {
		t.Fatalf("IdempotencyKey() error = %v, want nil", err)
	}
	if got, want := key.String(), nonceHexWithLetters; got != want {
		t.Fatalf("IdempotencyKey().String() = %q, want %q", got, want)
	}
	if err := key.Validate(); err != nil {
		t.Fatalf("IdempotencyKey().Validate() error = %v, want nil", err)
	}
}

func TestAuthorityNonceHostileNominalAndCanonicalBoundaries(t *testing.T) {
	t.Parallel()

	raw := [core.SHA256DigestBytes]byte{}
	for index := range raw {
		raw[index] = 0x5a
	}
	authority, err := controlwire.NewAuthorityNonce(raw)
	if err != nil {
		t.Fatalf("NewAuthorityNonce() error = %v, want nil", err)
	}
	request, err := controlwire.NewRequestNonce(raw)
	if err != nil {
		t.Fatalf("NewRequestNonce() error = %v, want nil", err)
	}
	if authority.String() != request.String() {
		t.Fatalf("same bytes project as authority %q and request %q, want identical text under distinct Go types",
			authority.String(), request.String())
	}
	parsed, err := controlwire.ParseAuthorityNonce(authority.String())
	if err != nil || parsed != authority {
		t.Fatalf("ParseAuthorityNonce() = (%v, %v), want exact nonce and nil", parsed, err)
	}
	encoded, err := json.Marshal(authority)
	if err != nil {
		t.Fatalf("json.Marshal(AuthorityNonce) error = %v, want nil", err)
	}
	var decoded controlwire.AuthorityNonce
	if err := json.Unmarshal(encoded, &decoded); err != nil || decoded != authority {
		t.Fatalf("json.Unmarshal(AuthorityNonce) = (%v, %v), want exact nonce and nil", decoded, err)
	}
}

func TestAuthorityNonceRefusesZeroAndEveryNonCanonicalTextWithoutMutation(t *testing.T) {
	t.Parallel()

	before, err := controlwire.ParseAuthorityNonce(nonceHexWithLetters)
	if err != nil {
		t.Fatalf("ParseAuthorityNonce(valid) error = %v, want nil", err)
	}
	cases := []struct {
		name  string
		token string
	}{
		{name: "all zero", token: nonceHexAllZero},
		{name: "empty", token: ""},
		{name: "one short", token: nonceHexWithLetters[:63]},
		{name: "one long", token: nonceHexWithLetters + "0"},
		{name: "uppercase", token: strings.ToUpper(nonceHexWithLetters)},
		{name: "non hexadecimal", token: strings.Repeat("g", 64)},
		{name: "leading space", token: " " + nonceHexWithLetters[:63]},
		{name: "trailing newline", token: nonceHexWithLetters[:63] + "\n"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got, gotErr := controlwire.ParseAuthorityNonce(testCase.token); !errors.Is(gotErr, core.ErrControlWireNonce) || got != (controlwire.AuthorityNonce{}) {
				t.Fatalf("ParseAuthorityNonce(%q) = (%v, %v), want zero and errors.Is %v",
					testCase.token, got, gotErr, core.ErrControlWireNonce)
			}
			encoded, marshalErr := core.MarshalCanonicalJSONString(testCase.token)
			if marshalErr != nil {
				t.Fatalf("core.MarshalCanonicalJSONString(%q) error = %v, want nil", testCase.token, marshalErr)
			}
			got := before
			gotErr := got.UnmarshalJSON(encoded)
			if !errors.Is(gotErr, core.ErrJSONContract) || got != before {
				t.Fatalf("AuthorityNonce.UnmarshalJSON(%q) = (%v, %v), want preserved %v and errors.Is %v",
					encoded, got, gotErr, before, core.ErrJSONContract)
			}
		})
	}
	if got, gotErr := controlwire.NewAuthorityNonce([core.SHA256DigestBytes]byte{}); !errors.Is(gotErr, core.ErrControlWireNonce) || got != (controlwire.AuthorityNonce{}) {
		t.Fatalf("NewAuthorityNonce(zero) = (%v, %v), want zero and errors.Is %v",
			got, gotErr, core.ErrControlWireNonce)
	}
}

func TestGenerateAuthorityNonceProducesDistinctValidatedValues(t *testing.T) {
	t.Parallel()

	const draws = 64
	seen := make(map[string]struct{}, draws)
	for draw := range draws {
		nonce, err := controlwire.GenerateAuthorityNonce()
		if err != nil || nonce.Validate() != nil {
			t.Fatalf("GenerateAuthorityNonce() draw %d = (%v, %v), want valid and nil", draw, nonce, err)
		}
		if _, exists := seen[nonce.String()]; exists {
			t.Fatalf("GenerateAuthorityNonce() draw %d repeated a prior value", draw)
		}
		seen[nonce.String()] = struct{}{}
	}
}

func TestRequestNonceJSONRejectionLeavesTheReceiverUnchanged(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		document   string
		wantOkay   bool
		wantSyntax bool
	}{
		{name: "canonical token is accepted", document: `"` + nonceHexWithLetters + `"`, wantOkay: true},
		{name: "json null is refused", document: `null`},
		{name: "number is refused", document: `1`},
		{name: "boolean is refused", document: `true`},
		{name: "empty string is refused", document: `""`},
		{name: "object is refused", document: `{"request_nonce":"` + nonceHexWithLetters + `"}`},
		{name: "array is refused", document: `["` + nonceHexWithLetters + `"]`},
		{name: "all-zero nonce is refused", document: `"` + nonceHexAllZero + `"`},
		{name: "uppercase hex is refused", document: `"` + strings.ToUpper(nonceHexWithLetters) + `"`},
		{name: "truncated token is refused", document: `"` + nonceHexWithLetters[:63] + `"`},
		{name: "overlong token is refused", document: `"` + nonceHexWithLetters + `0"`},
		{name: "unterminated string is refused", document: `"` + nonceHexWithLetters, wantSyntax: true},
		{name: "unpaired high surrogate is refused by JSON v2 syntax", document: `"\ud800"`, wantSyntax: true},
		{name: "empty document is refused", document: ``, wantSyntax: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			prior, err := controlwire.ParseRequestNonce(nonceHexAllDigits)
			if err != nil {
				t.Fatalf("ParseRequestNonce(prior) error = %v, want nil", err)
			}
			got := prior
			err = json.Unmarshal([]byte(tc.document), &got)
			if tc.wantOkay {
				if err != nil {
					t.Fatalf("json.Unmarshal(%s) error = %v, want nil", tc.document, err)
				}
				if got.String() != nonceHexWithLetters {
					t.Fatalf("json.Unmarshal(%s) = %q, want %q", tc.document, got.String(), nonceHexWithLetters)
				}
				return
			}
			var syntax *jsontext.SyntacticError
			if tc.wantSyntax {
				if !errors.As(err, &syntax) {
					t.Fatalf("json.Unmarshal(%s) error = %v, want errors.As *jsontext.SyntacticError", tc.document, err)
				}
			} else if !errors.Is(err, core.ErrControlWireNonce) || !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("json.Unmarshal(%s) error = %v, want errors.Is %v and %v", tc.document, err,
					core.ErrControlWireNonce, core.ErrJSONContract)
			}
			if got.String() != prior.String() {
				t.Fatalf("json.Unmarshal(%s) mutated receiver to %q, want %q", tc.document, got.String(), prior.String())
			}
		})
	}
}

func TestRequestNonceUnmarshalJSONOnNilReceiverIsRejected(t *testing.T) {
	t.Parallel()

	var nonce *controlwire.RequestNonce
	err := nonce.UnmarshalJSON([]byte(`"` + nonceHexWithLetters + `"`))
	if !errors.Is(err, core.ErrControlWireNonce) {
		t.Fatalf("(*RequestNonce)(nil).UnmarshalJSON() error = %v, want %v", err, core.ErrControlWireNonce)
	}
}

// TestRequestNonceRoundTripsWithoutMovingAByte proves marshal and unmarshal are
// inverses. Both ends of the control conversation encode this value, so a
// re-encode that differs by one byte is a protocol break.
func TestRequestNonceRoundTripsWithoutMovingAByte(t *testing.T) {
	t.Parallel()

	for _, token := range []string{nonceHexWithLetters, nonceHexAllDigits, nonceHexAllF} {
		nonce, err := controlwire.ParseRequestNonce(token)
		if err != nil {
			t.Fatalf("ParseRequestNonce(%q) error = %v, want nil", token, err)
		}
		encoded, err := json.Marshal(nonce)
		if err != nil {
			t.Fatalf("json.Marshal(%q) error = %v, want nil", token, err)
		}
		if want := `"` + token + `"`; string(encoded) != want {
			t.Fatalf("json.Marshal(%q) = %s, want %s", token, encoded, want)
		}
		var decoded controlwire.RequestNonce
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
