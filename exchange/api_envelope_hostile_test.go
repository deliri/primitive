package exchange_test

import (
	"encoding"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

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

		envelope, envelopeErr := exchange.NewAPISuccessEnvelope(id, transportDocument{Message: "payload"})
		if envelopeErr != nil {
			t.Fatalf("NewAPISuccessEnvelope() error = %v, want nil", envelopeErr)
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

		envelope, envelopeErr := exchange.NewAPIFailureEnvelope[transportDocument](id, failure)
		if envelopeErr != nil {
			t.Fatalf("NewAPIFailureEnvelope() error = %v, want nil", envelopeErr)
		}
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

	t.Run("both arms present is refused at the only boundary that can express it", func(t *testing.T) {
		t.Parallel()

		// No constructor can build a two-armed envelope, so the wire is the
		// only place the state can arrive. Refusing it there is the whole rule.
		var envelope exchange.APIEnvelope[transportDocument]
		document := []byte(`{"data":{"message":"payload"},` +
			`"error":{"message":"Not found.","code":"not_found"},"request_id":"trace-41"}`)
		if gotErr := envelope.UnmarshalJSON(document); !errors.Is(gotErr, core.ErrExchangeContract) {
			t.Fatalf("both-arm UnmarshalJSON() error = %v, want %v",
				gotErr, core.ErrExchangeContract)
		}
		if _, gotErr := envelope.Outcome(); !errors.Is(gotErr, core.ErrExchangeContract) {
			t.Fatalf("refused decode left a readable envelope: Outcome() error = %v, want %v",
				gotErr, core.ErrExchangeContract)
		}
	})

	t.Run("neither arm present is refused", func(t *testing.T) {
		t.Parallel()

		var envelope exchange.APIEnvelope[transportDocument]
		if gotErr := envelope.UnmarshalJSON([]byte(`{"request_id":"trace-41"}`)); !errors.Is(
			gotErr, core.ErrExchangeContract) {
			t.Fatalf("armless UnmarshalJSON() error = %v, want %v",
				gotErr, core.ErrExchangeContract)
		}
		if gotErr := envelope.Validate(); !errors.Is(gotErr, core.ErrExchangeContract) {
			t.Fatalf("zero APIEnvelope.Validate() error = %v, want %v",
				gotErr, core.ErrExchangeContract)
		}
	})

	t.Run("an unset request identifier is refused on both arms", func(t *testing.T) {
		t.Parallel()

		var unset exchange.APIRequestID
		if _, gotErr := exchange.NewAPISuccessEnvelope(
			unset, transportDocument{Message: "payload"},
		); !errors.Is(gotErr, core.ErrExchangeContract) {
			t.Fatalf("identifier-less NewAPISuccessEnvelope() error = %v, want %v",
				gotErr, core.ErrExchangeContract)
		}
		if _, gotErr := exchange.NewAPIFailureEnvelope[transportDocument](
			unset, failure,
		); !errors.Is(gotErr, core.ErrExchangeContract) {
			t.Fatalf("identifier-less NewAPIFailureEnvelope() error = %v, want %v",
				gotErr, core.ErrExchangeContract)
		}
	})

	t.Run("an invalid payload is refused through the data arm", func(t *testing.T) {
		t.Parallel()

		_, gotErr := exchange.NewAPISuccessEnvelope(id, transportDocument{})
		if !errors.Is(gotErr, core.ErrExchangeContract) {
			t.Fatalf("invalid-payload NewAPISuccessEnvelope() error = %v, want %v",
				gotErr, core.ErrExchangeContract)
		}
		if !errors.Is(gotErr, errTransportDocumentContract) {
			t.Fatalf("invalid-payload NewAPISuccessEnvelope() error = %v, want it to reach %v",
				gotErr, errTransportDocumentContract)
		}
	})

	t.Run("an invalid failure payload is refused through the error arm", func(t *testing.T) {
		t.Parallel()

		invalid := exchange.APIErrorBody{Code: exchange.APICodeNotFound}
		if _, gotErr := exchange.NewAPIFailureEnvelope[transportDocument](
			id, invalid,
		); !errors.Is(gotErr, core.ErrExchangeContract) {
			t.Fatalf("invalid-failure NewAPIFailureEnvelope() error = %v, want %v",
				gotErr, core.ErrExchangeContract)
		}
	})

	t.Run("the no-body marker cannot become a success data arm", func(t *testing.T) {
		t.Parallel()

		envelope, gotErr := exchange.NewAPISuccessEnvelope(id, exchange.APINoBody{})
		if !errors.Is(gotErr, core.ErrExchangeContract) {
			t.Fatalf("NewAPISuccessEnvelope(APINoBody) error = %v, want %v",
				gotErr, core.ErrExchangeContract)
		}
		encoded, encodeErr := exchange.MarshalAPIEnvelope(envelope)
		if !errors.Is(encodeErr, core.ErrExchangeContract) || encoded != nil {
			t.Fatalf("MarshalAPIEnvelope(refused envelope) = (%q, %v), want (nil, %v)",
				encoded, encodeErr, core.ErrExchangeContract)
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

		envelope := apiSuccessEnvelope(t, id, "payload")
		got, gotErr := exchange.MarshalAPIEnvelope(envelope)
		if gotErr != nil {
			t.Fatalf("MarshalAPIEnvelope() error = %v, want nil", gotErr)
		}
		want := `{"data":{"message":"payload"},"request_id":"trace-41"}`
		if string(got) != want {
			t.Fatalf("MarshalAPIEnvelope() = %s, want %s", got, want)
		}
	})

	t.Run("positive the failure document carries the tip only when present", func(t *testing.T) {
		t.Parallel()

		withTip := exchange.APIErrorBody{
			Code: exchange.APICodeInvalidInput, Message: "Invalid input.", Tip: "Check id.",
		}
		envelope := apiFailureEnvelope(t, id, withTip)
		got, gotErr := exchange.MarshalAPIEnvelope(envelope)
		if gotErr != nil {
			t.Fatalf("MarshalAPIEnvelope() error = %v, want nil", gotErr)
		}
		want := `{"error":{"message":"Invalid input.","tip":"Check id.","code":"invalid_input"},` +
			`"request_id":"trace-41"}`
		if string(got) != want {
			t.Fatalf("MarshalAPIEnvelope() = %s, want %s", got, want)
		}

		withoutTip := exchange.APIErrorBody{Code: exchange.APICodeInvalidInput, Message: "Invalid input."}
		bare := apiFailureEnvelope(t, id, withoutTip)
		gotBare, gotBareErr := exchange.MarshalAPIEnvelope(bare)
		if gotBareErr != nil {
			t.Fatalf("MarshalAPIEnvelope() error = %v, want nil", gotBareErr)
		}
		wantBare := `{"error":{"message":"Invalid input.","code":"invalid_input"},"request_id":"trace-41"}`
		if string(gotBare) != wantBare {
			t.Fatalf("tipless MarshalAPIEnvelope() = %s, want %s", gotBare, wantBare)
		}
	})

	t.Run("positive operator text keeps one spelling with html escaping off", func(t *testing.T) {
		t.Parallel()

		body := exchange.APIErrorBody{Code: exchange.APICodeInvalidInput, Message: `a<b>c&d`}
		envelope := apiFailureEnvelope(t, id, body)
		got, gotErr := exchange.MarshalAPIEnvelope(envelope)
		if gotErr != nil {
			t.Fatalf("MarshalAPIEnvelope() error = %v, want nil", gotErr)
		}
		want := `{"error":{"message":"a<b>c&d","code":"invalid_input"},"request_id":"trace-41"}`
		if string(got) != want {
			t.Fatalf("MarshalAPIEnvelope() = %s, want %s", got, want)
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
				if _, outcomeErr := got.Outcome(); outcomeErr == nil {
					t.Fatalf("rejected DecodeStrictJSON(%s) left a readable envelope, want the zero envelope",
						tc.document)
				}
				if identifier := got.RequestID().String(); identifier != "" {
					t.Fatalf("rejected DecodeStrictJSON(%s) left request identifier %q, want the zero envelope",
						tc.document, identifier)
				}
			})
		}
	})

	t.Run("neutral the absent arm produces no member rather than a null", func(t *testing.T) {
		t.Parallel()

		failure := exchange.APIErrorBody{Code: exchange.APICodeInternal, Message: "Internal."}
		envelope := apiFailureEnvelope(t, id, failure)
		got, gotErr := exchange.MarshalAPIEnvelope(envelope)
		if gotErr != nil {
			t.Fatalf("MarshalAPIEnvelope() error = %v, want nil", gotErr)
		}
		if strings.Contains(string(got), `"data"`) {
			t.Fatalf("failure MarshalAPIEnvelope() = %s, want no data member", got)
		}
		if strings.Contains(string(got), "null") {
			t.Fatalf("failure MarshalAPIEnvelope() = %s, want no null member", got)
		}

		success := apiSuccessEnvelope(t, id, "payload")
		gotSuccess, gotSuccessErr := exchange.MarshalAPIEnvelope(success)
		if gotSuccessErr != nil {
			t.Fatalf("MarshalAPIEnvelope() error = %v, want nil", gotSuccessErr)
		}
		if strings.Contains(string(gotSuccess), `"error"`) {
			t.Fatalf("success MarshalAPIEnvelope() = %s, want no error member", gotSuccess)
		}
	})

	t.Run("neutral an absent payload envelope still seals its identifier and code", func(t *testing.T) {
		t.Parallel()

		failure := exchange.APIErrorBody{Code: exchange.APICodeServiceUnavailable, Message: "Unavailable."}
		envelope, envelopeErr := exchange.NewAPIFailureEnvelope[exchange.APINoBody](id, failure)
		if envelopeErr != nil {
			t.Fatalf("NewAPIFailureEnvelope[APINoBody]() error = %v, want nil", envelopeErr)
		}
		got, gotErr := exchange.MarshalAPIEnvelope(envelope)
		if gotErr != nil {
			t.Fatalf("MarshalAPIEnvelope[APINoBody]() error = %v, want nil", gotErr)
		}
		want := `{"error":{"message":"Unavailable.","code":"service_unavailable"},"request_id":"trace-41"}`
		if string(got) != want {
			t.Fatalf("MarshalAPIEnvelope[APINoBody]() = %s, want %s", got, want)
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
			name:     "success envelope",
			envelope: apiSuccessEnvelope(t, id, "payload"),
		},
		{
			name: "failure envelope without a tip",
			envelope: apiFailureEnvelope(t, id, exchange.APIErrorBody{
				Code: exchange.APICodeNotFound, Message: "Not found.",
			}),
		},
		{
			name: "failure envelope with a tip",
			envelope: apiFailureEnvelope(t, id, exchange.APIErrorBody{
				Code: exchange.APICodeConflict, Message: "Conflict.", Tip: "Retry with a fresh version.",
			}),
		},
		{
			name: "multi byte operator text",
			envelope: apiFailureEnvelope(t, apiTestRequestID(t, "追跡-41"), exchange.APIErrorBody{
				Code: exchange.APICodeForbidden, Message: "権限がありません", Tip: "🛰",
			}),
		},
		{
			name: "markup bearing operator text",
			envelope: apiFailureEnvelope(t, id, exchange.APIErrorBody{
				Code: exchange.APICodeInvalidInput, Message: `a<b>c&d"e`,
			}),
		},
		{
			name: "identifier at the rune bound",
			envelope: apiSuccessEnvelope(
				t, apiTestRequestID(t, apiRunes(exchange.APIRequestIDMaximumRunes, 'a')), "payload"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			encoded, gotErr := exchange.MarshalAPIEnvelope(tc.envelope)
			if gotErr != nil {
				t.Fatalf("MarshalAPIEnvelope() error = %v, want nil", gotErr)
			}
			decoded, decodeErr := core.DecodeStrictJSON[exchange.APIEnvelope[transportDocument]](
				encoded,
				core.DefaultStrictJSONLimits(),
			)
			if decodeErr != nil {
				t.Fatalf("DecodeStrictJSON(%s) error = %v, want nil", encoded, decodeErr)
			}
			if decoded.RequestID() != tc.envelope.RequestID() {
				t.Fatalf("decoded RequestID = %q, want %q",
					decoded.RequestID().String(), tc.envelope.RequestID().String())
			}
			requireAPIArmsMatch(t, decoded, tc.envelope)
			reencoded, reencodeErr := exchange.MarshalAPIEnvelope(decoded)
			if reencodeErr != nil {
				t.Fatalf("decoded MarshalAPIEnvelope() error = %v, want nil", reencodeErr)
			}
			if string(reencoded) != string(encoded) {
				t.Fatalf("re-encoded envelope = %s, want %s", reencoded, encoded)
			}
		})
	}
}

// apiRunes builds a fill string of exactly count runes, so a rune-counted
// bound is exercised at its real boundary rather than at a byte count that
// happens to agree for ASCII.
func apiRunes(count int, fill rune) string {
	return strings.Repeat(string(fill), count)
}

// apiSuccessEnvelope seals one data-arm envelope for a test that needs a valid
// one. Construction is itself a proof now, so the helper fails the test rather
// than letting an unchecked envelope reach an assertion about something else.
func apiSuccessEnvelope(
	t *testing.T,
	id exchange.APIRequestID,
	message string,
) exchange.APIEnvelope[transportDocument] {
	t.Helper()

	envelope, err := exchange.NewAPISuccessEnvelope(id, transportDocument{Message: message})
	if err != nil {
		t.Fatalf("NewAPISuccessEnvelope(%q) error = %v, want nil", message, err)
	}
	return envelope
}

// apiFailureEnvelope seals one error-arm envelope for the same reason.
func apiFailureEnvelope(
	t *testing.T,
	id exchange.APIRequestID,
	failure exchange.APIErrorBody,
) exchange.APIEnvelope[transportDocument] {
	t.Helper()

	envelope, err := exchange.NewAPIFailureEnvelope[transportDocument](id, failure)
	if err != nil {
		t.Fatalf("NewAPIFailureEnvelope(%+v) error = %v, want nil", failure, err)
	}
	return envelope
}

func requireAPIArmsMatch(
	t *testing.T,
	got exchange.APIEnvelope[transportDocument],
	want exchange.APIEnvelope[transportDocument],
) {
	t.Helper()

	gotOutcome, gotOutcomeErr := got.Outcome()
	wantOutcome, wantOutcomeErr := want.Outcome()
	if gotOutcomeErr != nil || wantOutcomeErr != nil {
		t.Fatalf("Outcome() errors = (%v, %v), want (nil, nil)", gotOutcomeErr, wantOutcomeErr)
	}
	if gotOutcome != wantOutcome {
		t.Fatalf("decoded outcome = %v, want %v", gotOutcome, wantOutcome)
	}
	if gotOutcome == exchange.APIOutcomeSuccess {
		gotPayload, gotErr := got.Payload()
		wantPayload, wantErr := want.Payload()
		if gotErr != nil || wantErr != nil {
			t.Fatalf("Payload() errors = (%v, %v), want (nil, nil)", gotErr, wantErr)
		}
		if gotPayload != wantPayload {
			t.Fatalf("decoded data = %+v, want %+v", gotPayload, wantPayload)
		}
		return
	}
	gotFailure, gotErr := got.Failure()
	wantFailure, wantErr := want.Failure()
	if gotErr != nil || wantErr != nil {
		t.Fatalf("Failure() errors = (%v, %v), want (nil, nil)", gotErr, wantErr)
	}
	if gotFailure != wantFailure {
		t.Fatalf("decoded error = %+v, want %+v", gotFailure, wantFailure)
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

	envelope, envelopeErr := exchange.NewAPISuccessEnvelope(
		apiTestRequestID(t, "trace-41"), apiRefusingBody{})
	if envelopeErr != nil {
		t.Fatalf("NewAPISuccessEnvelope() error = %v, want nil", envelopeErr)
	}
	if err := envelope.Validate(); err != nil {
		t.Fatalf("APIEnvelope.Validate() error = %v, want nil", err)
	}

	got, gotErr := exchange.MarshalAPIEnvelope(envelope)
	if !errors.Is(gotErr, core.ErrExchangeContract) {
		t.Fatalf("MarshalAPIEnvelope() error = %v, want %v", gotErr, core.ErrExchangeContract)
	}
	if !errors.Is(gotErr, core.ErrJSONContract) {
		t.Fatalf("MarshalAPIEnvelope() error = %v, want it to reach %v",
			gotErr, core.ErrJSONContract)
	}
	if !errors.Is(gotErr, errAPIRefusingBodyMarshal) {
		t.Fatalf("MarshalAPIEnvelope() error = %v, want it to reach %v",
			gotErr, errAPIRefusingBodyMarshal)
	}
	if got != nil {
		t.Fatalf("MarshalAPIEnvelope() bytes = %q, want nil", got)
	}
}

// TestAPIEnvelopeDecodeLeavesTheReceiverUntouchedOnRejection proves the decode
// boundary is transactional. A receiver already holding a good envelope must
// still hold exactly that envelope after a rejected document, so a hostile
// payload cannot half-overwrite a value a caller is still using.
func TestAPIEnvelopeDecodeLeavesTheReceiverUntouchedOnRejection(t *testing.T) {
	t.Parallel()

	id := apiTestRequestID(t, "trace-41")
	envelope := apiSuccessEnvelope(t, id, "payload")
	encoded, encodeErr := exchange.MarshalAPIEnvelope(envelope)
	if encodeErr != nil {
		t.Fatalf("MarshalAPIEnvelope() error = %v, want nil", encodeErr)
	}

	var receiver exchange.APIEnvelope[transportDocument]
	if err := receiver.UnmarshalJSON(encoded); err != nil {
		t.Fatalf("UnmarshalJSON(%s) error = %v, want nil", encoded, err)
	}

	hostile := []struct {
		name     string
		document string
	}{
		{name: "both arms", document: `{"data":{"message":"p"},` +
			`"error":{"message":"m","code":"internal"},"request_id":"t"}`},
		{name: "no arm", document: `{"request_id":"t"}`},
		{name: "invalid payload", document: `{"data":{"message":""},"request_id":"t"}`},
		{name: "unknown code", document: `{"error":{"message":"m","code":"teapot"},"request_id":"t"}`},
		{name: "not an object", document: `[]`},
	}
	for _, tc := range hostile {
		if err := receiver.UnmarshalJSON([]byte(tc.document)); err == nil {
			t.Fatalf("UnmarshalJSON(%s) error = nil, want a rejection", tc.document)
		}
		requireAPIArmsMatch(t, receiver, envelope)
		if receiver.RequestID() != id {
			t.Fatalf("after rejected %s the receiver RequestID = %q, want %q",
				tc.name, receiver.RequestID().String(), id.String())
		}
	}
}

// TestAPIEnvelopeEmitterRefusesAnUnprovenValue proves MarshalAPIEnvelope gates
// on Validate rather than trusting that every envelope reaching it came from a
// constructor. The zero value is the one unproven envelope Go itself can build,
// so it is what the emitter must refuse.
func TestAPIEnvelopeEmitterRefusesAnUnprovenValue(t *testing.T) {
	t.Parallel()

	var unset exchange.APIEnvelope[transportDocument]
	got, gotErr := exchange.MarshalAPIEnvelope(unset)
	if !errors.Is(gotErr, core.ErrExchangeContract) {
		t.Fatalf("MarshalAPIEnvelope(zero) error = %v, want %v", gotErr, core.ErrExchangeContract)
	}
	if got != nil {
		t.Fatalf("MarshalAPIEnvelope(zero) = %s, want no bytes", got)
	}
}

// TestAPIEnvelopeNilReceiverIsRefusedRatherThanPanicking proves the decode
// boundary reports a typed contract failure for a nil receiver instead of
// faulting, which is what every other decoding contract in this package does.
func TestAPIEnvelopeNilReceiverIsRefusedRatherThanPanicking(t *testing.T) {
	t.Parallel()

	var receiver *exchange.APIEnvelope[transportDocument]
	gotErr := receiver.UnmarshalJSON([]byte(`{"data":{"message":"p"},"request_id":"t"}`))
	if !errors.Is(gotErr, core.ErrExchangeContract) {
		t.Fatalf("nil receiver UnmarshalJSON() error = %v, want %v",
			gotErr, core.ErrExchangeContract)
	}
}

// TestAPIEnvelopeRefusesReflectedEncoding proves the envelope cannot be emitted
// by any path except MarshalAPIEnvelope. The arms are unexported, so a reflected
// encode would otherwise produce a document carrying only request_id and lose
// the payload silently. Failing loudly here is what makes the constrained
// emitter the single emission owner rather than merely the recommended one.
func TestAPIEnvelopeRefusesReflectedEncoding(t *testing.T) {
	t.Parallel()

	envelope := apiSuccessEnvelope(t, apiTestRequestID(t, "trace-41"), "payload")
	if _, implements := any(envelope).(json.Marshaler); implements {
		t.Fatalf("APIEnvelope implements json.Marshaler = %t, want false", implements)
	}
	if _, implements := any(envelope).(encoding.TextMarshaler); !implements {
		t.Fatalf("APIEnvelope implements encoding.TextMarshaler = %t, want true", implements)
	}

	got, gotErr := json.Marshal(envelope)
	if !errors.Is(gotErr, core.ErrExchangeContract) {
		t.Fatalf("json.Marshal(APIEnvelope) error = %v, want %v", gotErr, core.ErrExchangeContract)
	}
	if got != nil {
		t.Fatalf("json.Marshal(APIEnvelope) = %s, want no bytes", got)
	}

	// A value that merely contains an envelope must fail the same way rather
	// than emitting an object with the arms missing.
	container := struct {
		Envelope exchange.APIEnvelope[transportDocument] `json:"envelope"`
	}{Envelope: envelope}
	nested, nestedErr := json.Marshal(container)
	if !errors.Is(nestedErr, core.ErrExchangeContract) {
		t.Fatalf("json.Marshal(container) error = %v, want %v", nestedErr, core.ErrExchangeContract)
	}
	if nested != nil {
		t.Fatalf("json.Marshal(container) = %s, want no bytes", nested)
	}

	// The sanctioned emitter still works on the same value, so the refusal is
	// about the path taken and not about the envelope being unusable.
	encoded, encodeErr := exchange.MarshalAPIEnvelope(envelope)
	if encodeErr != nil {
		t.Fatalf("MarshalAPIEnvelope() error = %v, want nil", encodeErr)
	}
	want := `{"data":{"message":"payload"},"request_id":"trace-41"}`
	if string(encoded) != want {
		t.Fatalf("MarshalAPIEnvelope() = %s, want %s", encoded, want)
	}
}

// apiDecodeOnlyBody is a consumer-owned payload. It can be populated from a
// received document and validated, but deliberately owns no JSON emission
// method because a consumer is not permitted to reproduce the bearer it read.
type apiDecodeOnlyBody struct {
	Token string `json:"token"`
}

func (body apiDecodeOnlyBody) Validate() error {
	if body.Token == "" {
		return errors.New("decode-only test body requires a token")
	}
	return nil
}

// TestAPIEnvelopeAdmitsDecodeOnlyBodies proves the envelope's read contract is
// strictly weaker than its emission contract. The body and envelope remain
// Validatable without accidentally satisfying ValidatedJSONMarshaler.
func TestAPIEnvelopeAdmitsDecodeOnlyBodies(t *testing.T) {
	t.Parallel()

	document := []byte(`{"data":{"token":"bearer"},"request_id":"trace-41"}`)
	envelope, gotErr := core.DecodeStrictJSON[exchange.APIEnvelope[apiDecodeOnlyBody]](
		document,
		core.DefaultStrictJSONLimits(),
	)
	if gotErr != nil {
		t.Fatalf("DecodeStrictJSON(%s) error = %v, want nil", document, gotErr)
	}
	payload, payloadErr := envelope.Payload()
	if payloadErr != nil {
		t.Fatalf("APIEnvelope.Payload() error = %v, want nil", payloadErr)
	}
	if payload.Token != "bearer" {
		t.Fatalf("APIEnvelope.Payload().Token = %q, want bearer", payload.Token)
	}
	if _, implements := any(apiDecodeOnlyBody{}).(core.ValidatedJSONMarshaler); implements {
		t.Fatalf("apiDecodeOnlyBody implements core.ValidatedJSONMarshaler = %t, want false", implements)
	}
	if _, implements := any(envelope).(core.ValidatedJSONMarshaler); implements {
		t.Fatalf("APIEnvelope[apiDecodeOnlyBody] implements core.ValidatedJSONMarshaler = %t, want false",
			implements)
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
