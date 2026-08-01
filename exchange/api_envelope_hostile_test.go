package exchange_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// apiRunes builds one string of exactly count copies of fill. Rune-bounded
// contracts must be pressured with multi-byte fills, so the tables can prove the
// bound counts runes rather than bytes.
func apiRunes(count int, fill rune) string {
	return strings.Repeat(string(fill), count)
}

func apiTestEnvelopeBody(t *testing.T, message string) *transportDocument {
	t.Helper()

	document := transportDocument{Message: message}
	if err := document.Validate(); err != nil {
		t.Fatalf("transportDocument.Validate() error = %v, want nil", err)
	}
	return &document
}

func apiTestRequestID(t *testing.T, value string) exchange.APIRequestID {
	t.Helper()

	id, err := exchange.ParseAPIRequestID(value)
	if err != nil {
		t.Fatalf("ParseAPIRequestID(%q) error = %v, want nil", value, err)
	}
	return id
}

// TestAPICodeClosedDomainIsExhaustive walks the complete uint8 domain so no
// future member can be added without either a token or a deliberate test
// change. The closed set is small enough to exhaust rather than sample.
func TestAPICodeClosedDomainIsExhaustive(t *testing.T) {
	t.Parallel()

	admitted := map[exchange.APICode]string{
		exchange.APICodeNotFound:           exchange.APICodeTokenNotFound,
		exchange.APICodeInvalidInput:       exchange.APICodeTokenInvalidInput,
		exchange.APICodeConflict:           exchange.APICodeTokenConflict,
		exchange.APICodeUnauthorized:       exchange.APICodeTokenUnauthorized,
		exchange.APICodeForbidden:          exchange.APICodeTokenForbidden,
		exchange.APICodePayloadTooLarge:    exchange.APICodeTokenPayloadTooLarge,
		exchange.APICodeServiceUnavailable: exchange.APICodeTokenServiceUnavailable,
		exchange.APICodeInternal:           exchange.APICodeTokenInternal,
	}
	seen := make(map[string]exchange.APICode, len(admitted))
	for candidate := 0; candidate <= 255; candidate++ {
		code := exchange.APICode(candidate)
		wantToken, wantValid := admitted[code]
		if got := code.IsValid(); got != wantValid {
			t.Fatalf("APICode(%d).IsValid() = %t, want %t", candidate, got, wantValid)
		}
		if got := code.String(); got != wantToken {
			t.Fatalf("APICode(%d).String() = %q, want %q", candidate, got, wantToken)
		}
		gotErr := code.Validate()
		if wantValid && gotErr != nil {
			t.Fatalf("APICode(%d).Validate() error = %v, want nil", candidate, gotErr)
		}
		if !wantValid {
			if !errors.Is(gotErr, core.ErrExchangeContract) {
				t.Fatalf("APICode(%d).Validate() error = %v, want %v",
					candidate, gotErr, core.ErrExchangeContract)
			}
			continue
		}
		if prior, repeated := seen[wantToken]; repeated {
			t.Fatalf("APICode token %q = codes %d and %d, want one code",
				wantToken, prior, code)
		}
		seen[wantToken] = code
	}
	if len(seen) != len(admitted) {
		t.Fatalf("distinct APICode tokens = %d, want %d", len(seen), len(admitted))
	}
}

// TestParseAPICodeAdmitsExactTokensOnly pressures the token grammar. Every
// admitted token round trips; every near-miss spelling is refused rather than
// repaired, because a near miss is a producer defect.
func TestParseAPICodeAdmitsExactTokensOnly(t *testing.T) {
	t.Parallel()

	accepted := []struct {
		name  string
		token string
		want  exchange.APICode
	}{
		{name: "not found token", token: "not_found", want: exchange.APICodeNotFound},
		{name: "invalid input token", token: "invalid_input", want: exchange.APICodeInvalidInput},
		{name: "conflict token", token: "conflict", want: exchange.APICodeConflict},
		{name: "unauthorized token", token: "unauthorized", want: exchange.APICodeUnauthorized},
		{name: "forbidden token", token: "forbidden", want: exchange.APICodeForbidden},
		{name: "payload too large token", token: "payload_too_large", want: exchange.APICodePayloadTooLarge},
		{name: "service unavailable token", token: "service_unavailable", want: exchange.APICodeServiceUnavailable},
		{name: "internal token", token: "internal", want: exchange.APICodeInternal},
	}
	for _, tc := range accepted {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := exchange.ParseAPICode(tc.token)
			if gotErr != nil || got != tc.want {
				t.Fatalf("ParseAPICode(%q) = (%d, %v), want (%d, nil)",
					tc.token, got, gotErr, tc.want)
			}
			if round := got.String(); round != tc.token {
				t.Fatalf("ParseAPICode(%q).String() = %q, want %q", tc.token, round, tc.token)
			}
		})
	}

	rejected := []struct {
		name  string
		token string
	}{
		{name: "empty token", token: ""},
		{name: "uppercase spelling of an admitted token", token: "NOT_FOUND"},
		{name: "mixed case spelling of an admitted token", token: "Not_Found"},
		{name: "hyphen separator instead of underscore", token: "not-found"},
		{name: "space separator instead of underscore", token: "not found"},
		{name: "leading whitespace around an admitted token", token: " not_found"},
		{name: "trailing whitespace around an admitted token", token: "not_found "},
		{name: "leading underscore before an admitted token", token: "_not_found"},
		{name: "trailing underscore after an admitted token", token: "not_found_"},
		{name: "admitted token repeated", token: "not_foundnot_found"},
		{name: "prefix of an admitted token", token: "not_foun"},
		{name: "admitted token with a trailing null rune", token: "not_found\x00"},
		{name: "unknown future token", token: "quota_exhausted"},
		{name: "the zero member name rather than a token", token: "unknown"},
		{name: "numeric spelling of an admitted code", token: "1"},
		{name: "json null literal as a token", token: "null"},
		{name: "invalid utf-8 byte", token: "\xff"},
		{name: "replacement rune", token: "�"},
	}
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := exchange.ParseAPICode(tc.token)
			if !errors.Is(gotErr, core.ErrExchangeContract) {
				t.Fatalf("ParseAPICode(%q) error = %v, want %v",
					tc.token, gotErr, core.ErrExchangeContract)
			}
			if got != exchange.APICodeUnknown {
				t.Fatalf("ParseAPICode(%q) = %d, want %d",
					tc.token, got, exchange.APICodeUnknown)
			}
		})
	}
}

// TestAPICodeJSONRejectsHostileDocumentsWithoutMutation proves the decode
// boundary keeps a stable identity and never half-writes the receiver.
func TestAPICodeJSONRejectsHostileDocumentsWithoutMutation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		document string
	}{
		{name: "empty document", document: ""},
		{name: "json null", document: `null`},
		{name: "bare number instead of a token", document: `3`},
		{name: "boolean instead of a token", document: `true`},
		{name: "object instead of a token", document: `{"code":"internal"}`},
		{name: "array instead of a token", document: `["internal"]`},
		{name: "unterminated string", document: `"internal`},
		{name: "unknown token", document: `"teapot"`},
		{name: "empty token", document: `""`},
		{name: "token containing an escaped null rune", document: `"internal\u0000"`},
		{name: "token with surrounding whitespace inside the string", document: `" internal "`},
		{name: "uppercase token", document: `"INTERNAL"`},
		{name: "unpaired high surrogate escape", document: `"\ud800"`},
		{name: "unpaired low surrogate escape", document: `"\udc00"`},
		{name: "invalid utf-8 inside the string", document: "\"\xff\""},
		{name: "trailing data after a valid token", document: `"internal" "internal"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			code := exchange.APICodeConflict
			gotErr := code.UnmarshalJSON([]byte(tc.document))
			if !errors.Is(gotErr, core.ErrExchangeContract) {
				t.Fatalf("APICode.UnmarshalJSON(%q) error = %v, want %v",
					tc.document, gotErr, core.ErrExchangeContract)
			}
			if code != exchange.APICodeConflict {
				t.Fatalf("APICode after rejected UnmarshalJSON(%q) = %d, want %d",
					tc.document, code, exchange.APICodeConflict)
			}
		})
	}

	t.Run("nil receiver is refused rather than dereferenced", func(t *testing.T) {
		t.Parallel()

		var code *exchange.APICode
		if gotErr := code.UnmarshalJSON([]byte(`"internal"`)); !errors.Is(gotErr, core.ErrExchangeContract) {
			t.Fatalf("(*APICode)(nil).UnmarshalJSON() error = %v, want %v",
				gotErr, core.ErrExchangeContract)
		}
	})

	t.Run("out of domain code cannot become bytes", func(t *testing.T) {
		t.Parallel()

		encoded, gotErr := exchange.APICode(200).MarshalJSON()
		if !errors.Is(gotErr, core.ErrExchangeContract) {
			t.Fatalf("APICode(200).MarshalJSON() error = %v, want %v",
				gotErr, core.ErrExchangeContract)
		}
		if encoded != nil {
			t.Fatalf("APICode(200).MarshalJSON() bytes = %q, want nil", encoded)
		}
	})
}

// TestAPIRequestIDValidateHostileTable pressures both sides of the correlation
// identifier contract: presence, canonical UTF-8, control and replacement
// runes, surrounding whitespace, and the rune bound counted in runes rather
// than bytes.
func TestAPIRequestIDValidateHostileTable(t *testing.T) {
	t.Parallel()

	const bound = exchange.APIRequestIDMaximumRunes

	cases := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "single ascii rune is admitted", value: "a"},
		{name: "the missing token is itself a valid identifier", value: "missing"},
		{name: "uuid shaped identifier", value: "3f2504e0-4f89-11d3-9a0c-0305e82c3301"},
		{name: "identifier with interior spaces", value: "trace 41 shard 7"},
		{name: "identifier with interior underscores and dots", value: "trace_41.shard-7"},
		{name: "identifier with punctuation and symbols", value: "req#41/attempt=2?x&y"},
		{name: "multi byte identifier", value: "追跡識別子"},
		{name: "emoji identifier", value: "🛰🛰🛰"},
		{name: "identifier with a zero width joiner sequence", value: "a\u200db"},
		{name: "digits only identifier", value: "0000000000"},

		{name: "empty identifier is rejected", value: "", wantErr: true},
		{name: "single space is rejected", value: " ", wantErr: true},
		{name: "whitespace only identifier is rejected", value: "   ", wantErr: true},
		{name: "leading space is rejected", value: " trace", wantErr: true},
		{name: "trailing space is rejected", value: "trace ", wantErr: true},
		{name: "leading tab is rejected", value: "\ttrace", wantErr: true},
		{name: "trailing newline is rejected", value: "trace\n", wantErr: true},
		{name: "interior newline is rejected as a control rune", value: "trace\nid", wantErr: true},
		{name: "interior null is rejected as a control rune", value: "trace\x00id", wantErr: true},
		{name: "interior carriage return is rejected", value: "trace\rid", wantErr: true},
		{name: "delete control rune is rejected", value: "trace\x7fid", wantErr: true},
		{name: "unicode line separator is rejected as surrounding whitespace", value: "trace ", wantErr: true},
		{name: "non breaking space at the end is rejected", value: "trace ", wantErr: true},
		{name: "invalid utf-8 byte is rejected", value: "trace\xffid", wantErr: true},
		{name: "truncated multi byte sequence is rejected", value: "trace\xe6\x97", wantErr: true},
		{name: "literal replacement rune is rejected", value: "trace�id", wantErr: true},
		{name: "replacement rune alone is rejected", value: "�", wantErr: true},

		{name: "one rune below the bound is admitted", value: apiRunes(bound-1, 'a')},
		{name: "exactly at the bound is admitted", value: apiRunes(bound, 'a')},
		{name: "one rune above the bound is rejected", value: apiRunes(bound+1, 'a'), wantErr: true},
		{name: "bound counts runes not bytes at four bytes per rune", value: apiRunes(bound, '𝍖')},
		{name: "one four byte rune above the bound is rejected", value: apiRunes(bound+1, '𝍖'), wantErr: true},
		{name: "bound counts runes not bytes at three bytes per rune", value: apiRunes(bound, '追')},
		{name: "one three byte rune above the bound is rejected", value: apiRunes(bound+1, '追'), wantErr: true},
		{name: "one rune under a four byte bound is admitted", value: apiRunes(bound-1, '𝍖')},
		{name: "far above the bound is rejected", value: apiRunes(bound*4, 'a'), wantErr: true},
		{name: "at the bound with a trailing space is rejected", value: apiRunes(bound-1, 'a') + " ", wantErr: true},
		{name: "at the bound with an interior control rune is rejected", value: apiRunes(bound-1, 'a') + "\x01", wantErr: true},
		{name: "at the bound with a trailing replacement rune is rejected", value: apiRunes(bound-1, 'a') + "�", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := exchange.ParseAPIRequestID(tc.value)
			if tc.wantErr {
				if !errors.Is(gotErr, core.ErrExchangeContract) {
					t.Fatalf("ParseAPIRequestID(%q) error = %v, want %v",
						tc.value, gotErr, core.ErrExchangeContract)
				}
				if got.String() != "" {
					t.Fatalf("ParseAPIRequestID(%q) = %q, want the zero identifier",
						tc.value, got.String())
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("ParseAPIRequestID(%q) error = %v, want nil", tc.value, gotErr)
			}
			if got.String() != tc.value {
				t.Fatalf("ParseAPIRequestID(%q).String() = %q, want %q",
					tc.value, got.String(), tc.value)
			}
		})
	}
}

// TestNewAPIRequestIDDegradesRatherThanFails proves the documented repair
// contract: every input produces a valid identifier, and an input that cannot
// be repaired becomes exactly the missing token rather than an empty or
// half-normalized value.
func TestNewAPIRequestIDDegradesRatherThanFails(t *testing.T) {
	t.Parallel()

	const bound = exchange.APIRequestIDMaximumRunes

	cases := []struct {
		name  string
		value string
		want  string
	}{
		{name: "canonical identifier is preserved exactly", value: "trace-41", want: "trace-41"},
		{name: "surrounding spaces are trimmed", value: "  trace-41  ", want: "trace-41"},
		{name: "surrounding tabs are trimmed", value: "\ttrace-41\t", want: "trace-41"},
		{name: "interior control runes are dropped not replaced", value: "trace\x00-\x0141", want: "trace-41"},
		{name: "newline separated fragments join without a separator", value: "trace\n41", want: "trace41"},
		{name: "control runes that leave only whitespace degrade", value: "\x00 \x01 ", want: "missing"},
		{name: "empty input degrades", value: "", want: "missing"},
		{name: "whitespace only input degrades", value: "   ", want: "missing"},
		{name: "control runes only input degrades", value: "\x00\x01\x02", want: "missing"},
		{name: "invalid utf-8 input degrades", value: "trace\xff41", want: "missing"},
		{name: "replacement rune input degrades", value: "trace�41", want: "missing"},
		{name: "the missing token is preserved rather than re-degraded", value: "missing", want: "missing"},
		{name: "exactly at the bound is preserved", value: apiRunes(bound, 'a'), want: apiRunes(bound, 'a')},
		{name: "one above the bound is truncated to the bound", value: apiRunes(bound+1, 'a'), want: apiRunes(bound, 'a')},
		{name: "far above the bound is truncated to the bound", value: apiRunes(bound*4, 'a'), want: apiRunes(bound, 'a')},
		{name: "truncation counts runes not bytes", value: apiRunes(bound+8, '追'), want: apiRunes(bound, '追')},
		{
			name:  "truncation exposing trailing whitespace trims again",
			value: apiRunes(bound-1, 'a') + " tail",
			want:  apiRunes(bound-1, 'a'),
		},
		{
			name:  "leading whitespace is trimmed before the bound is applied",
			value: "  " + apiRunes(bound, 'a'),
			want:  apiRunes(bound, 'a'),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := exchange.NewAPIRequestID(tc.value)
			if got.String() != tc.want {
				t.Fatalf("NewAPIRequestID(%q).String() = %q, want %q",
					tc.value, got.String(), tc.want)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("NewAPIRequestID(%q).Validate() error = %v, want nil", tc.value, err)
			}
		})
	}
}

// TestAPIErrorBodyValidateHostileTable pressures the failure payload: the
// closed code, the required message, and the optional tip whose absence is
// admitted while a present but non-canonical tip is not.
func TestAPIErrorBodyValidateHostileTable(t *testing.T) {
	t.Parallel()

	const (
		messageBound = exchange.APIErrorMessageMaximumRunes
		tipBound     = exchange.APIErrorTipMaximumRunes
	)

	cases := []struct {
		name    string
		body    exchange.APIErrorBody
		wantErr bool
	}{
		{
			name: "message with an absent tip is admitted",
			body: exchange.APIErrorBody{Code: exchange.APICodeNotFound, Message: "Not found."},
		},
		{
			name: "message with a present tip is admitted",
			body: exchange.APIErrorBody{
				Code: exchange.APICodeInvalidInput, Message: "Invalid input.", Tip: "Check the id field.",
			},
		},
		{
			name: "every closed code is admitted with the same message",
			body: exchange.APIErrorBody{Code: exchange.APICodeInternal, Message: "Internal."},
		},
		{
			name: "single rune message is admitted",
			body: exchange.APIErrorBody{Code: exchange.APICodeConflict, Message: "x"},
		},
		{
			name: "multi byte message is admitted",
			body: exchange.APIErrorBody{Code: exchange.APICodeForbidden, Message: "権限がありません"},
		},
		{
			name: "message at the rune bound is admitted",
			body: exchange.APIErrorBody{Code: exchange.APICodeInternal, Message: apiRunes(messageBound, 'a')},
		},
		{
			name: "tip at the rune bound is admitted",
			body: exchange.APIErrorBody{
				Code: exchange.APICodeInternal, Message: "m", Tip: apiRunes(tipBound, 'a'),
			},
		},
		{
			name: "message one rune below the bound is admitted",
			body: exchange.APIErrorBody{Code: exchange.APICodeInternal, Message: apiRunes(messageBound-1, 'a')},
		},
		{
			name: "four byte message at the rune bound is admitted",
			body: exchange.APIErrorBody{Code: exchange.APICodeInternal, Message: apiRunes(messageBound, '𝍖')},
		},
		{
			name: "message containing markup is admitted as opaque operator text",
			body: exchange.APIErrorBody{Code: exchange.APICodeInvalidInput, Message: `<a href="x">&amp;</a>`},
		},

		{
			name:    "zero code is rejected",
			body:    exchange.APIErrorBody{Message: "Not found."},
			wantErr: true,
		},
		{
			name:    "out of domain code is rejected",
			body:    exchange.APIErrorBody{Code: exchange.APICode(200), Message: "Not found."},
			wantErr: true,
		},
		{
			name:    "empty message is rejected",
			body:    exchange.APIErrorBody{Code: exchange.APICodeNotFound},
			wantErr: true,
		},
		{
			name:    "whitespace only message is rejected",
			body:    exchange.APIErrorBody{Code: exchange.APICodeNotFound, Message: "   "},
			wantErr: true,
		},
		{
			name:    "message with a leading space is rejected",
			body:    exchange.APIErrorBody{Code: exchange.APICodeNotFound, Message: " Not found."},
			wantErr: true,
		},
		{
			name:    "message with a trailing newline is rejected",
			body:    exchange.APIErrorBody{Code: exchange.APICodeNotFound, Message: "Not found.\n"},
			wantErr: true,
		},
		{
			name:    "message with an interior control rune is rejected",
			body:    exchange.APIErrorBody{Code: exchange.APICodeNotFound, Message: "Not\x00found."},
			wantErr: true,
		},
		{
			name:    "message with invalid utf-8 is rejected",
			body:    exchange.APIErrorBody{Code: exchange.APICodeNotFound, Message: "Not\xfffound."},
			wantErr: true,
		},
		{
			name:    "message with a replacement rune is rejected",
			body:    exchange.APIErrorBody{Code: exchange.APICodeNotFound, Message: "Not�found."},
			wantErr: true,
		},
		{
			name: "tip with a leading space is rejected",
			body: exchange.APIErrorBody{
				Code: exchange.APICodeNotFound, Message: "Not found.", Tip: " Check the id.",
			},
			wantErr: true,
		},

		{
			name:    "message one rune above the bound is rejected",
			body:    exchange.APIErrorBody{Code: exchange.APICodeInternal, Message: apiRunes(messageBound+1, 'a')},
			wantErr: true,
		},
		{
			name: "tip one rune above the bound is rejected",
			body: exchange.APIErrorBody{
				Code: exchange.APICodeInternal, Message: "m", Tip: apiRunes(tipBound+1, 'a'),
			},
			wantErr: true,
		},
		{
			name:    "four byte message one rune above the bound is rejected",
			body:    exchange.APIErrorBody{Code: exchange.APICodeInternal, Message: apiRunes(messageBound+1, '𝍖')},
			wantErr: true,
		},
		{
			name: "four byte tip at the rune bound is admitted",
			body: exchange.APIErrorBody{
				Code: exchange.APICodeInternal, Message: "m", Tip: apiRunes(tipBound, '𝍖'),
			},
		},
		{
			name: "tip one rune below the bound is admitted",
			body: exchange.APIErrorBody{
				Code: exchange.APICodeInternal, Message: "m", Tip: apiRunes(tipBound-1, 'a'),
			},
		},
		{
			name: "a valid tip does not rescue an invalid message",
			body: exchange.APIErrorBody{
				Code: exchange.APICodeInternal, Message: "", Tip: "Check the id.",
			},
			wantErr: true,
		},
		{
			name: "a valid message does not rescue an invalid tip",
			body: exchange.APIErrorBody{
				Code: exchange.APICodeInternal, Message: "Internal.", Tip: "\x00",
			},
			wantErr: true,
		},
		{
			name: "a valid message and tip do not rescue an invalid code",
			body: exchange.APIErrorBody{
				Code: exchange.APICode(9), Message: "Internal.", Tip: "Check the id.",
			},
			wantErr: true,
		},
		{
			name:    "the completely zero body is rejected",
			body:    exchange.APIErrorBody{},
			wantErr: true,
		},
		{
			name: "tip whose only content is whitespace is rejected",
			body: exchange.APIErrorBody{
				Code: exchange.APICodeInternal, Message: "Internal.", Tip: " ",
			},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.body.Validate()
			if tc.wantErr {
				if !errors.Is(gotErr, core.ErrExchangeContract) {
					t.Fatalf("APIErrorBody.Validate() error = %v, want %v",
						gotErr, core.ErrExchangeContract)
				}
				encoded, marshalErr := tc.body.MarshalJSON()
				if !errors.Is(marshalErr, core.ErrExchangeContract) || encoded != nil {
					t.Fatalf("rejected APIErrorBody.MarshalJSON() = (%q, %v), want (nil, %v)",
						encoded, marshalErr, core.ErrExchangeContract)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("APIErrorBody.Validate() error = %v, want nil", gotErr)
			}
			if _, marshalErr := tc.body.MarshalJSON(); marshalErr != nil {
				t.Fatalf("APIErrorBody.MarshalJSON() error = %v, want nil", marshalErr)
			}
		})
	}
}

// TestAPIEnvelopeArmContractRejectsRatherThanResolvingPrecedence proves the
// central envelope invariant. Both arms present and neither arm present are
// both refused, so no reader ever has to guess which one wins.
func TestAPIEnvelopeArmContractRejectsRatherThanResolvingPrecedence(t *testing.T) {
	t.Parallel()

	id := apiTestRequestID(t, "trace-41")
	failure := exchange.APIErrorBody{Code: exchange.APICodeNotFound, Message: "Not found."}

	t.Run("data arm alone reads as success and yields its payload", func(t *testing.T) {
		t.Parallel()

		envelope := exchange.APIEnvelope[transportDocument]{
			Data: apiTestEnvelopeBody(t, "payload"), RequestID: id,
		}
		if err := envelope.Validate(); err != nil {
			t.Fatalf("APIEnvelope.Validate() error = %v, want nil", err)
		}
		got, gotErr := envelope.Outcome()
		if gotErr != nil || got != exchange.APIOutcomeSuccess {
			t.Fatalf("APIEnvelope.Outcome() = (%v, %v), want (%v, nil)",
				got, gotErr, exchange.APIOutcomeSuccess)
		}
		payload, payloadErr := envelope.Payload()
		if payloadErr != nil || payload.Message != "payload" {
			t.Fatalf("APIEnvelope.Payload() = (%+v, %v), want the data arm", payload, payloadErr)
		}
		if _, err := envelope.Failure(); !errors.Is(err, core.ErrExchangeContract) {
			t.Fatalf("APIEnvelope.Failure() error = %v, want %v", err, core.ErrExchangeContract)
		}
	})

	t.Run("error arm alone reads as failure and yields its failure body", func(t *testing.T) {
		t.Parallel()

		envelope := exchange.APIEnvelope[transportDocument]{Error: &failure, RequestID: id}
		if err := envelope.Validate(); err != nil {
			t.Fatalf("APIEnvelope.Validate() error = %v, want nil", err)
		}
		got, gotErr := envelope.Outcome()
		if gotErr != nil || got != exchange.APIOutcomeFailure {
			t.Fatalf("APIEnvelope.Outcome() = (%v, %v), want (%v, nil)",
				got, gotErr, exchange.APIOutcomeFailure)
		}
		body, bodyErr := envelope.Failure()
		if bodyErr != nil || body != failure {
			t.Fatalf("APIEnvelope.Failure() = (%+v, %v), want %+v", body, bodyErr, failure)
		}
		if _, err := envelope.Payload(); !errors.Is(err, core.ErrExchangeContract) {
			t.Fatalf("APIEnvelope.Payload() error = %v, want %v", err, core.ErrExchangeContract)
		}
	})

	t.Run("both arms present is refused rather than resolved", func(t *testing.T) {
		t.Parallel()

		envelope := exchange.APIEnvelope[transportDocument]{
			Data: apiTestEnvelopeBody(t, "payload"), Error: &failure, RequestID: id,
		}
		_, payloadErr := envelope.Payload()
		_, failureErr := envelope.Failure()
		_, outcomeErr := envelope.Outcome()
		for _, reading := range []struct {
			err  error
			name string
		}{
			{name: "Validate", err: envelope.Validate()},
			{name: "Payload", err: payloadErr},
			{name: "Failure", err: failureErr},
			{name: "Outcome", err: outcomeErr},
		} {
			if !errors.Is(reading.err, core.ErrExchangeContract) {
				t.Fatalf("both-arm APIEnvelope.%s() error = %v, want %v",
					reading.name, reading.err, core.ErrExchangeContract)
			}
		}
		encoded, marshalErr := envelope.MarshalJSON()
		if !errors.Is(marshalErr, core.ErrExchangeContract) || encoded != nil {
			t.Fatalf("both-arm APIEnvelope.MarshalJSON() = (%q, %v), want (nil, %v)",
				encoded, marshalErr, core.ErrExchangeContract)
		}
	})

	t.Run("neither arm present is refused", func(t *testing.T) {
		t.Parallel()

		envelope := exchange.APIEnvelope[transportDocument]{RequestID: id}
		if gotErr := envelope.Validate(); !errors.Is(gotErr, core.ErrExchangeContract) {
			t.Fatalf("empty APIEnvelope.Validate() error = %v, want %v",
				gotErr, core.ErrExchangeContract)
		}
	})

	t.Run("an unset request identifier is refused on both arms", func(t *testing.T) {
		t.Parallel()

		data := exchange.APIEnvelope[transportDocument]{Data: apiTestEnvelopeBody(t, "payload")}
		if gotErr := data.Validate(); !errors.Is(gotErr, core.ErrExchangeContract) {
			t.Fatalf("identifier-less data envelope Validate() error = %v, want %v",
				gotErr, core.ErrExchangeContract)
		}
		failureEnvelope := exchange.APIEnvelope[transportDocument]{Error: &failure}
		if gotErr := failureEnvelope.Validate(); !errors.Is(gotErr, core.ErrExchangeContract) {
			t.Fatalf("identifier-less failure envelope Validate() error = %v, want %v",
				gotErr, core.ErrExchangeContract)
		}
	})

	t.Run("an invalid payload is refused through the data arm", func(t *testing.T) {
		t.Parallel()

		invalid := transportDocument{}
		envelope := exchange.APIEnvelope[transportDocument]{Data: &invalid, RequestID: id}
		gotErr := envelope.Validate()
		if !errors.Is(gotErr, core.ErrExchangeContract) {
			t.Fatalf("invalid-payload APIEnvelope.Validate() error = %v, want %v",
				gotErr, core.ErrExchangeContract)
		}
		if !errors.Is(gotErr, errTransportDocumentContract) {
			t.Fatalf("invalid-payload APIEnvelope.Validate() error = %v, want it to reach %v",
				gotErr, errTransportDocumentContract)
		}
	})

	t.Run("an invalid failure payload is refused through the error arm", func(t *testing.T) {
		t.Parallel()

		invalid := exchange.APIErrorBody{Code: exchange.APICodeNotFound}
		envelope := exchange.APIEnvelope[transportDocument]{Error: &invalid, RequestID: id}
		if gotErr := envelope.Validate(); !errors.Is(gotErr, core.ErrExchangeContract) {
			t.Fatalf("invalid-failure APIEnvelope.Validate() error = %v, want %v",
				gotErr, core.ErrExchangeContract)
		}
	})

	t.Run("the no-body marker cannot become a success data arm", func(t *testing.T) {
		t.Parallel()

		noBody := exchange.APINoBody{}
		envelope := exchange.APIEnvelope[exchange.APINoBody]{Data: &noBody, RequestID: id}
		if gotErr := envelope.Validate(); !errors.Is(gotErr, core.ErrExchangeContract) {
			t.Fatalf("APIEnvelope[APINoBody].Validate() error = %v, want %v",
				gotErr, core.ErrExchangeContract)
		}
		encoded, gotErr := envelope.MarshalJSON()
		if !errors.Is(gotErr, core.ErrExchangeContract) || encoded != nil {
			t.Fatalf("APIEnvelope[APINoBody].MarshalJSON() = (%q, %v), want (nil, %v)",
				encoded, gotErr, core.ErrExchangeContract)
		}
	})
}

// TestAPIEnvelopeLayerTriad covers the envelope schema layer: the intended
// document is produced exactly, a hostile document fails with a stable
// identity, and the neutral case proves the absent arm produces no wire
// presence at all rather than a null that a client could mistake for a value.
func TestAPIEnvelopeLayerTriad(t *testing.T) {
	t.Parallel()

	id := apiTestRequestID(t, "trace-41")

	t.Run("positive the success document carries exactly the data and identifier members", func(t *testing.T) {
		t.Parallel()

		envelope := exchange.APIEnvelope[transportDocument]{
			Data: apiTestEnvelopeBody(t, "payload"), RequestID: id,
		}
		got, gotErr := envelope.MarshalJSON()
		if gotErr != nil {
			t.Fatalf("APIEnvelope.MarshalJSON() error = %v, want nil", gotErr)
		}
		want := `{"data":{"message":"payload"},"request_id":"trace-41"}`
		if string(got) != want {
			t.Fatalf("APIEnvelope.MarshalJSON() = %s, want %s", got, want)
		}
	})

	t.Run("positive the failure document carries the tip only when present", func(t *testing.T) {
		t.Parallel()

		withTip := exchange.APIErrorBody{
			Code: exchange.APICodeInvalidInput, Message: "Invalid input.", Tip: "Check id.",
		}
		envelope := exchange.APIEnvelope[transportDocument]{Error: &withTip, RequestID: id}
		got, gotErr := envelope.MarshalJSON()
		if gotErr != nil {
			t.Fatalf("APIEnvelope.MarshalJSON() error = %v, want nil", gotErr)
		}
		want := `{"error":{"message":"Invalid input.","tip":"Check id.","code":"invalid_input"},` +
			`"request_id":"trace-41"}`
		if string(got) != want {
			t.Fatalf("APIEnvelope.MarshalJSON() = %s, want %s", got, want)
		}

		withoutTip := exchange.APIErrorBody{Code: exchange.APICodeInvalidInput, Message: "Invalid input."}
		bare := exchange.APIEnvelope[transportDocument]{Error: &withoutTip, RequestID: id}
		gotBare, gotBareErr := bare.MarshalJSON()
		if gotBareErr != nil {
			t.Fatalf("APIEnvelope.MarshalJSON() error = %v, want nil", gotBareErr)
		}
		wantBare := `{"error":{"message":"Invalid input.","code":"invalid_input"},"request_id":"trace-41"}`
		if string(gotBare) != wantBare {
			t.Fatalf("tipless APIEnvelope.MarshalJSON() = %s, want %s", gotBare, wantBare)
		}
	})

	t.Run("positive operator text keeps one spelling with html escaping off", func(t *testing.T) {
		t.Parallel()

		body := exchange.APIErrorBody{Code: exchange.APICodeInvalidInput, Message: `a<b>c&d`}
		envelope := exchange.APIEnvelope[transportDocument]{Error: &body, RequestID: id}
		got, gotErr := envelope.MarshalJSON()
		if gotErr != nil {
			t.Fatalf("APIEnvelope.MarshalJSON() error = %v, want nil", gotErr)
		}
		want := `{"error":{"message":"a<b>c&d","code":"invalid_input"},"request_id":"trace-41"}`
		if string(got) != want {
			t.Fatalf("APIEnvelope.MarshalJSON() = %s, want %s", got, want)
		}
	})

	t.Run("negative a hostile document is refused with a stable identity", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name     string
			document string
		}{
			{name: "both arms present", document: `{"data":{"message":"p"},` +
				`"error":{"message":"m","code":"internal"},"request_id":"trace-41"}`},
			{name: "neither arm present", document: `{"request_id":"trace-41"}`},
			{name: "explicit null data with no error", document: `{"data":null,"request_id":"trace-41"}`},
			{name: "absent request identifier", document: `{"data":{"message":"p"}}`},
			{name: "empty request identifier", document: `{"data":{"message":"p"},"request_id":""}`},
			{name: "null request identifier", document: `{"data":{"message":"p"},"request_id":null}`},
			{name: "unknown member", document: `{"data":{"message":"p"},"request_id":"t","extra":1}`},
			{name: "duplicate request identifier member", document: `{"data":{"message":"p"},` +
				`"request_id":"a","request_id":"b"}`},
			{name: "case folded duplicate member", document: `{"data":{"message":"p"},` +
				`"request_id":"a","Request_ID":"b"}`},
			{name: "unknown failure code token", document: `{"error":{"message":"m","code":"teapot"},` +
				`"request_id":"t"}`},
			{name: "failure code as a number", document: `{"error":{"message":"m","code":8},"request_id":"t"}`},
			{name: "empty failure message", document: `{"error":{"message":"","code":"internal"},` +
				`"request_id":"t"}`},
			{name: "invalid payload through the data arm", document: `{"data":{"message":""},` +
				`"request_id":"trace-41"}`},
			{name: "json null document", document: `null`},
			{name: "array instead of an object", document: `[]`},
			{name: "trailing data after the document", document: `{"data":{"message":"p"},` +
				`"request_id":"t"}{}`},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				got, gotErr := core.DecodeStrictJSON[exchange.APIEnvelope[transportDocument]](
					[]byte(tc.document),
					core.DefaultStrictJSONLimits(),
				)
				if gotErr == nil {
					t.Fatalf("DecodeStrictJSON(%s) error = nil, want a rejection", tc.document)
				}
				if got.RequestID.String() != "" || got.Data != nil || got.Error != nil {
					t.Fatalf("rejected DecodeStrictJSON(%s) = %+v, want the zero envelope",
						tc.document, got)
				}
			})
		}
	})

	t.Run("neutral the absent arm produces no member rather than a null", func(t *testing.T) {
		t.Parallel()

		failure := exchange.APIErrorBody{Code: exchange.APICodeInternal, Message: "Internal."}
		envelope := exchange.APIEnvelope[transportDocument]{Error: &failure, RequestID: id}
		got, gotErr := envelope.MarshalJSON()
		if gotErr != nil {
			t.Fatalf("APIEnvelope.MarshalJSON() error = %v, want nil", gotErr)
		}
		if strings.Contains(string(got), `"data"`) {
			t.Fatalf("failure APIEnvelope.MarshalJSON() = %s, want no data member", got)
		}
		if strings.Contains(string(got), "null") {
			t.Fatalf("failure APIEnvelope.MarshalJSON() = %s, want no null member", got)
		}

		success := exchange.APIEnvelope[transportDocument]{
			Data: apiTestEnvelopeBody(t, "payload"), RequestID: id,
		}
		gotSuccess, gotSuccessErr := success.MarshalJSON()
		if gotSuccessErr != nil {
			t.Fatalf("APIEnvelope.MarshalJSON() error = %v, want nil", gotSuccessErr)
		}
		if strings.Contains(string(gotSuccess), `"error"`) {
			t.Fatalf("success APIEnvelope.MarshalJSON() = %s, want no error member", gotSuccess)
		}
	})

	t.Run("neutral an absent payload envelope still seals its identifier and code", func(t *testing.T) {
		t.Parallel()

		failure := exchange.APIErrorBody{Code: exchange.APICodeServiceUnavailable, Message: "Unavailable."}
		envelope := exchange.APIEnvelope[exchange.APINoBody]{Error: &failure, RequestID: id}
		got, gotErr := envelope.MarshalJSON()
		if gotErr != nil {
			t.Fatalf("APIEnvelope[APINoBody].MarshalJSON() error = %v, want nil", gotErr)
		}
		want := `{"error":{"message":"Unavailable.","code":"service_unavailable"},"request_id":"trace-41"}`
		if string(got) != want {
			t.Fatalf("APIEnvelope[APINoBody].MarshalJSON() = %s, want %s", got, want)
		}
	})
}

// TestAPIEnvelopeStrictJSONRoundTripIsStable proves the envelope survives
// Core's strict encoder, which requires an accepted document to decode into the
// same typed value and re-encode to identical bytes.
func TestAPIEnvelopeStrictJSONRoundTripIsStable(t *testing.T) {
	t.Parallel()

	id := apiTestRequestID(t, "trace-41")

	cases := []struct {
		name     string
		envelope exchange.APIEnvelope[transportDocument]
	}{
		{
			name: "success envelope",
			envelope: exchange.APIEnvelope[transportDocument]{
				Data: &transportDocument{Message: "payload"}, RequestID: id,
			},
		},
		{
			name: "failure envelope without a tip",
			envelope: exchange.APIEnvelope[transportDocument]{
				Error: &exchange.APIErrorBody{
					Code: exchange.APICodeNotFound, Message: "Not found.",
				},
				RequestID: id,
			},
		},
		{
			name: "failure envelope with a tip",
			envelope: exchange.APIEnvelope[transportDocument]{
				Error: &exchange.APIErrorBody{
					Code: exchange.APICodeConflict, Message: "Conflict.", Tip: "Retry with a fresh version.",
				},
				RequestID: id,
			},
		},
		{
			name: "multi byte operator text",
			envelope: exchange.APIEnvelope[transportDocument]{
				Error: &exchange.APIErrorBody{
					Code: exchange.APICodeForbidden, Message: "権限がありません", Tip: "🛰",
				},
				RequestID: apiTestRequestID(t, "追跡-41"),
			},
		},
		{
			name: "markup bearing operator text",
			envelope: exchange.APIEnvelope[transportDocument]{
				Error: &exchange.APIErrorBody{
					Code: exchange.APICodeInvalidInput, Message: `a<b>c&d"e`,
				},
				RequestID: id,
			},
		},
		{
			name: "identifier at the rune bound",
			envelope: exchange.APIEnvelope[transportDocument]{
				Data:      &transportDocument{Message: "payload"},
				RequestID: apiTestRequestID(t, apiRunes(exchange.APIRequestIDMaximumRunes, 'a')),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			encoded, gotErr := core.EncodeValidatedJSON(tc.envelope, core.DefaultStrictJSONLimits())
			if gotErr != nil {
				t.Fatalf("EncodeValidatedJSON() error = %v, want nil", gotErr)
			}
			decoded, decodeErr := core.DecodeStrictJSON[exchange.APIEnvelope[transportDocument]](
				encoded,
				core.DefaultStrictJSONLimits(),
			)
			if decodeErr != nil {
				t.Fatalf("DecodeStrictJSON(%s) error = %v, want nil", encoded, decodeErr)
			}
			if decoded.RequestID != tc.envelope.RequestID {
				t.Fatalf("decoded RequestID = %q, want %q",
					decoded.RequestID.String(), tc.envelope.RequestID.String())
			}
			requireAPIArmsMatch(t, decoded, tc.envelope)
			reencoded, reencodeErr := decoded.MarshalJSON()
			if reencodeErr != nil {
				t.Fatalf("decoded APIEnvelope.MarshalJSON() error = %v, want nil", reencodeErr)
			}
			if string(reencoded) != string(encoded) {
				t.Fatalf("re-encoded envelope = %s, want %s", reencoded, encoded)
			}
		})
	}
}

func requireAPIArmsMatch(
	t *testing.T,
	got exchange.APIEnvelope[transportDocument],
	want exchange.APIEnvelope[transportDocument],
) {
	t.Helper()

	if (got.Data == nil) != (want.Data == nil) || (got.Error == nil) != (want.Error == nil) {
		t.Fatalf("decoded arms = (data present %t, error present %t), want (%t, %t)",
			got.Data != nil, got.Error != nil, want.Data != nil, want.Error != nil)
	}
	if got.Data != nil && *got.Data != *want.Data {
		t.Fatalf("decoded data = %+v, want %+v", *got.Data, *want.Data)
	}
	if got.Error != nil && *got.Error != *want.Error {
		t.Fatalf("decoded error = %+v, want %+v", *got.Error, *want.Error)
	}
}

// errAPIRefusingBodyMarshal is the identity a test payload returns from
// MarshalJSON. A payload that validates but cannot encode is the only way the
// canonical document owner in Core fails on the envelope path, so it is how the
// envelope's identity restatement is proved rather than assumed.
var errAPIRefusingBodyMarshal = errors.New("test payload refuses to marshal")

// apiRefusingBody passes every validation the envelope performs and then
// refuses to encode, which is the hostile shape a real payload takes when its
// own owner rejects the value after the envelope already admitted it.
type apiRefusingBody struct{}

func (apiRefusingBody) Validate() error { return nil }

func (apiRefusingBody) MarshalJSON() ([]byte, error) {
	return nil, errAPIRefusingBodyMarshal
}

// TestAPIEnvelopeRestatesCanonicalEncoderFailures proves the envelope keeps one
// stable Exchange identity when Core's canonical document owner fails, while
// leaving the originating error reachable through errors.Is.
func TestAPIEnvelopeRestatesCanonicalEncoderFailures(t *testing.T) {
	t.Parallel()

	envelope := exchange.APIEnvelope[apiRefusingBody]{
		Data:      &apiRefusingBody{},
		RequestID: apiTestRequestID(t, "trace-41"),
	}
	if err := envelope.Validate(); err != nil {
		t.Fatalf("APIEnvelope.Validate() error = %v, want nil", err)
	}

	got, gotErr := envelope.MarshalJSON()
	if !errors.Is(gotErr, core.ErrExchangeContract) {
		t.Fatalf("APIEnvelope.MarshalJSON() error = %v, want %v", gotErr, core.ErrExchangeContract)
	}
	if !errors.Is(gotErr, core.ErrJSONContract) {
		t.Fatalf("APIEnvelope.MarshalJSON() error = %v, want it to reach %v",
			gotErr, core.ErrJSONContract)
	}
	if !errors.Is(gotErr, errAPIRefusingBodyMarshal) {
		t.Fatalf("APIEnvelope.MarshalJSON() error = %v, want it to reach %v",
			gotErr, errAPIRefusingBodyMarshal)
	}
	if got != nil {
		t.Fatalf("APIEnvelope.MarshalJSON() bytes = %q, want nil", got)
	}
}

// TestAPIOutcomeClosedDomainIsExhaustive walks the complete uint8 domain so no
// future reading can be added without a diagnostic name and a deliberate test
// change, and pins that the reading stays off the wire.
func TestAPIOutcomeClosedDomainIsExhaustive(t *testing.T) {
	t.Parallel()

	admitted := map[exchange.APIOutcome]string{
		exchange.APIOutcomeSuccess: "success",
		exchange.APIOutcomeFailure: "failure",
	}
	for candidate := 0; candidate <= 255; candidate++ {
		outcome := exchange.APIOutcome(candidate)
		wantText, wantValid := admitted[outcome]
		if got := outcome.IsValid(); got != wantValid {
			t.Fatalf("APIOutcome(%d).IsValid() = %t, want %t", candidate, got, wantValid)
		}
		if got := outcome.String(); got != wantText {
			t.Fatalf("APIOutcome(%d).String() = %q, want %q", candidate, got, wantText)
		}
		gotErr := outcome.Validate()
		if wantValid && gotErr != nil {
			t.Fatalf("APIOutcome(%d).Validate() error = %v, want nil", candidate, gotErr)
		}
		if !wantValid && !errors.Is(gotErr, core.ErrExchangeContract) {
			t.Fatalf("APIOutcome(%d).Validate() error = %v, want %v",
				candidate, gotErr, core.ErrExchangeContract)
		}
	}

	reading := any(exchange.APIOutcomeSuccess)
	if _, ok := reading.(json.Marshaler); ok {
		t.Fatalf("APIOutcome implements json.Marshaler, want a reading that stays off the wire")
	}
	if _, ok := any(new(exchange.APIOutcome)).(json.Unmarshaler); ok {
		t.Fatalf("*APIOutcome implements json.Unmarshaler, want a reading that stays off the wire")
	}
	if _, ok := reading.(core.OffWireEnum); !ok {
		t.Fatalf("APIOutcome does not declare core.OffWireEnum, want the positive declaration")
	}
}
