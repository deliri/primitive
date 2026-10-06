package jsonio_test

import (
	"context"
	"encoding/json/jsontext"
	"errors"
	"io"
	"iter"
	"slices"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestJSONTokensExactGoProjection(t *testing.T) {
	t.Parallel()
	fixtures := []struct {
		Label  string
		Values []string
		Flag   bool
		Count  int64
	}{
		{}, {Label: "quoted\" and \\ escaped", Values: []string{"", "\x00", "é", "𝄞", "line\nend"}, Flag: true, Count: -123},
		{Values: []string{strings.Repeat("x", 65537)}, Count: 9223372036854775807},
	}
	for _, fixture := range fixtures {
		data, err := core.MarshalCanonicalJSONDocument(fixture)
		if err != nil {
			t.Fatal(err)
		}
		decoder := jsontext.NewDecoder(strings.NewReader(string(data)))
		seen := 0
		for got, err := range jsonio.Tokens(t.Context(), jsonio.TokenRequest{Source: iotest.OneByteReader(strings.NewReader(string(data))), NestingDepthMaximum: 2}) {
			if err != nil || got.Validate() != nil {
				t.Fatalf("Tokens() = (%+v,%v), want valid token", got, err)
			}
			want, err := decoder.ReadToken()
			if err != nil {
				t.Fatal(err)
			}
			if got.Kind != independentTokenKind(want.Kind()) {
				t.Fatalf("kind = %v, want Go kind %v", got.Kind, want.Kind())
			}
			if got.Kind == jsonio.TokenString || got.Kind == jsonio.TokenNumber {
				if got.Text != want.String() {
					t.Fatalf("token text = %q, want %q", got.Text, want.String())
				}
			} else if got.Text != "" {
				t.Fatalf("delimiter text = %q, want empty", got.Text)
			}
			seen++
		}
		if _, err := decoder.ReadToken(); err != io.EOF || seen == 0 {
			t.Fatalf("oracle EOF = %v after %d tokens, want complete nonempty projection", err, seen)
		}
	}
}

func FuzzJSONTokensGrammarAndProjectionAgainstGo(f *testing.F) {
	seed, err := core.MarshalCanonicalJSONDocument(struct{ Values []string }{Values: []string{"", "quoted\"", "é"}})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed, uint8(2))
	f.Add([]byte("[}"), uint8(1))
	f.Add([]byte{}, uint8(1))
	f.Fuzz(func(t *testing.T, data []byte, depth uint8) {
		maximum := uint16(depth%core.JSONNestingDepthMaximum) + 1
		decoder := jsontext.NewDecoder(strings.NewReader(string(data)))
		next, stop := iter.Pull2(jsonio.Tokens(t.Context(), jsonio.TokenRequest{
			Source: strings.NewReader(string(data)), NestingDepthMaximum: maximum,
		}))
		defer stop()
		for {
			want, wantErr := decoder.ReadToken()
			got, gotErr, ok := next()
			if wantErr == io.EOF {
				if ok {
					t.Fatalf("oracle EOF published (%+v,%v), want no token", got, gotErr)
				}
				return
			}
			if wantErr != nil || decoder.StackDepth() > int(maximum) {
				if !ok || got != (jsonio.Token{}) || !errors.Is(gotErr, core.ErrJSONContract) {
					t.Fatalf("oracle refusal = (%+v,%v,%v), want one zero typed refusal", got, gotErr, ok)
				}
				if _, _, ok := next(); ok {
					t.Fatal("token published after refusal")
				}
				return
			}
			if !ok || gotErr != nil || got.Validate() != nil || got.Kind != independentTokenKind(want.Kind()) {
				t.Fatalf("projection = (%+v,%v,%v), want exact Go kind %v", got, gotErr, ok, want.Kind())
			}
			if got.Kind == jsonio.TokenString || got.Kind == jsonio.TokenNumber {
				if got.Text != want.String() {
					t.Fatalf("projected text = %q, want %q", got.Text, want.String())
				}
			} else if got.Text != "" {
				t.Fatal("literal or delimiter carried text")
			}
		}
	})
}

func TestJSONTokensHostileGrammarAndDepth(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		depth      uint16
		refuse     bool
	}{
		{"empty stream", "", 2, false}, {"whitespace stream", " \n\t", 2, false},
		{"scalar string", `""`, 2, false}, {"scalar number", "1", 2, false},
		{"negative number", "-1", 2, false}, {"fraction", "1.25", 2, false},
		{"exponent", "1e10000", 2, false}, {"zero", "0", 2, false},
		{"true", "true", 2, false}, {"false", "false", 2, false}, {"null", "null", 2, false},
		{"empty object", "{}", 1, false}, {"empty array", "[]", 1, false},
		{"adjacent documents", "{}[]null", 2, false}, {"exact depth", "[[]]", 2, false},
		{"above depth", "[[[]]]", 2, true}, {"depth zero", "[]", 0, true},
		{"depth foreign", "[]", core.JSONNestingDepthMaximum + 1, true},
		{"missing close", "[1", 2, true}, {"missing object close", `{"a":1`, 2, true},
		{"missing value", `{"a":}`, 2, true}, {"missing colon", `{"a" 1}`, 2, true},
		{"duplicate field", `{"a":1,"a":2}`, 2, true}, {"escaped duplicate", `{"a":1,"\u0061":2}`, 2, true},
		{"unknown field is policy owned", `{"foreign":1}`, 2, false},
		{"trailing comma array", "[1,]", 2, true}, {"trailing comma object", `{"a":1,}`, 2, true},
		{"leading comma", "[,1]", 2, true}, {"double comma", "[1,,2]", 2, true},
		{"unterminated string", `"text`, 2, true}, {"invalid escape", `"\q"`, 2, true},
		{"invalid surrogate", `"\ud800"`, 2, true}, {"invalid UTF8", "\"\xff\"", 2, true},
		{"literal newline in string", "\"a\nb\"", 2, true}, {"NUL", "\x00", 2, true},
		{"leading zero in array", "[01]", 2, true}, {"positive sign", "+1", 2, true},
		{"missing fraction", "1.", 2, true}, {"missing exponent", "1e", 2, true},
		{"nan", "NaN", 2, true}, {"infinity", "Infinity", 2, true},
		{"literal truncation", "tru", 2, true}, {"literal suffix", "trueX", 2, true},
		{"single quote", "'x'", 2, true}, {"unquoted field", "{a:1}", 2, true},
		{"foreign delimiter", "(1)", 2, true}, {"unmatched close", "]", 2, true},
		{"garbage tail", "[]broken", 2, true}, {"incomplete tail", "{}[", 2, true},
		{"wrong close", "[}", 2, true}, {"object value without name", "{1}", 2, true},
		{"empty numeric sign", "-", 2, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			failed := false
			for token, err := range jsonio.Tokens(t.Context(), jsonio.TokenRequest{Source: strings.NewReader(tc.body), NestingDepthMaximum: tc.depth}) {
				if err != nil {
					if failed || !errors.Is(err, core.ErrJSONContract) || token != (jsonio.Token{}) {
						t.Fatalf("refusal = (%+v,%v), want one zero typed refusal", token, err)
					}
					failed = true
				} else if failed || token.Validate() != nil {
					t.Fatalf("published token = %+v after refusal=%v", token, failed)
				}
			}
			if failed != tc.refuse {
				t.Fatalf("refused = %v, want %v", failed, tc.refuse)
			}
		})
	}
}

func TestJSONTokensCancellationAndSourceIdentity(t *testing.T) {
	t.Parallel()
	ctx, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: t.Context()})
	if err != nil {
		t.Fatal(err)
	}
	defer cancel(nil)
	seen := 0
	for token, err := range jsonio.Tokens(ctx, jsonio.TokenRequest{Source: strings.NewReader("[1,2,3]"), NestingDepthMaximum: 1}) {
		seen++
		if seen == 1 {
			if token.Kind != jsonio.TokenArrayStart || err != nil {
				t.Fatal(token, err)
			}
			cancel(nil)
			continue
		}
		if token != (jsonio.Token{}) || !errors.Is(err, context.Canceled) || seen != 2 {
			t.Fatalf("canceled = (%+v,%v,%d), want zero cancellation once", token, err, seen)
		}
	}
	if seen != 2 {
		t.Fatalf("seen = %d, want prefix plus cancellation", seen)
	}
	for _, sourceErr := range []error{io.ErrUnexpectedEOF, io.ErrClosedPipe, errors.Join(io.EOF, io.ErrClosedPipe)} {
		seen := 0
		for token, err := range jsonio.Tokens(t.Context(), jsonio.TokenRequest{Source: iotest.ErrReader(sourceErr), NestingDepthMaximum: 1}) {
			seen++
			if token != (jsonio.Token{}) || !errors.Is(err, sourceErr) || !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("source refusal = (%+v,%v), want zero preserved %v", token, err, sourceErr)
			}
		}
		if seen != 1 {
			t.Fatalf("source refusals = %d, want one", seen)
		}
	}
}

type tokenCancelReader struct {
	source io.Reader
	cancel context.CancelCauseFunc
}

func (r tokenCancelReader) Read(data []byte) (int, error) {
	n, err := r.source.Read(data)
	r.cancel(nil)
	return n, err
}

func TestJSONTokensCancellationDuringReadCannotBecomeEOF(t *testing.T) {
	t.Parallel()
	for _, body := range []string{"", "null"} {
		ctx, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: t.Context()})
		if err != nil {
			t.Fatal(err)
		}
		seen := 0
		for token, err := range jsonio.Tokens(ctx, jsonio.TokenRequest{
			Source: tokenCancelReader{source: strings.NewReader(body), cancel: cancel}, NestingDepthMaximum: 1,
		}) {
			seen++
			if token != (jsonio.Token{}) || !errors.Is(err, context.Canceled) {
				t.Fatalf("read cancellation = (%+v,%v), want zero cancellation", token, err)
			}
		}
		cancel(nil)
		if seen != 1 {
			t.Fatalf("read cancellation publications = %d, want one refusal", seen)
		}
	}
}

func TestJSONTokenOwnedValidation(t *testing.T) {
	t.Parallel()
	for _, token := range []jsonio.Token{{}, {Kind: jsonio.TokenKind(255)},
		{Kind: jsonio.TokenString, Text: "\xff"}, {Kind: jsonio.TokenNumber, Text: " 1"},
		{Kind: jsonio.TokenNumber, Text: "1 "}, {Kind: jsonio.TokenNumber, Text: "1 2"},
		{Kind: jsonio.TokenNumber, Text: "null"}, {Kind: jsonio.TokenNumber},
		{Kind: jsonio.TokenObjectStart, Text: "{"}, {Kind: jsonio.TokenTrue, Text: "true"}} {
		if err := token.Validate(); !errors.Is(err, core.ErrJSONContract) {
			t.Fatalf("Token.Validate(%+v) = %v, want typed refusal", token, err)
		}
	}
	var absent *strings.Reader
	for _, source := range []io.Reader{nil, absent} {
		seen := 0
		for token, err := range jsonio.Tokens(t.Context(), jsonio.TokenRequest{Source: source, NestingDepthMaximum: 1}) {
			seen++
			if token != (jsonio.Token{}) || !errors.Is(err, core.ErrJSONContract) {
				t.Fatal(token, err)
			}
		}
		if seen != 1 {
			t.Fatalf("absent source refusals = %d, want one", seen)
		}
	}
}

func FuzzJSONTokensExactDecodedStrings(f *testing.F) {
	f.Add("", uint8(0))
	f.Add("quoted\"\x00é", uint8(1))
	f.Fuzz(func(t *testing.T, value string, windows uint8) {
		data, err := core.MarshalCanonicalJSONString(value)
		if err != nil {
			return
		} // Invalid UTF-8 cannot seed a valid string contract.
		var source io.Reader = strings.NewReader(string(data))
		if windows%2 == 0 {
			source = iotest.OneByteReader(source)
		}
		var got []jsonio.Token
		for token, err := range jsonio.Tokens(t.Context(), jsonio.TokenRequest{Source: source, NestingDepthMaximum: 1}) {
			if err != nil || token.Validate() != nil {
				t.Fatalf("canonical string = (%+v,%v), want valid observation", token, err)
			}
			got = append(got, token)
		}
		if !slices.Equal(got, []jsonio.Token{{Kind: jsonio.TokenString, Text: value}}) {
			t.Fatalf("decoded tokens = %+v, want one exact string", got)
		}
	})
}

func independentTokenKind(kind jsontext.Kind) jsonio.TokenKind {
	for _, pair := range [...]struct {
		standard jsontext.Kind
		owned    jsonio.TokenKind
	}{
		{jsontext.KindBeginObject, jsonio.TokenObjectStart}, {jsontext.KindEndObject, jsonio.TokenObjectEnd},
		{jsontext.KindBeginArray, jsonio.TokenArrayStart}, {jsontext.KindEndArray, jsonio.TokenArrayEnd},
		{jsontext.KindString, jsonio.TokenString}, {jsontext.KindNumber, jsonio.TokenNumber},
		{jsontext.KindTrue, jsonio.TokenTrue}, {jsontext.KindFalse, jsonio.TokenFalse}, {jsontext.KindNull, jsonio.TokenNull},
	} {
		if kind == pair.standard {
			return pair.owned
		}
	}
	return jsonio.TokenUnknown
}
