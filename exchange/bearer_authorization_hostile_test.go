package exchange_test

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestReceiveBearerAuthorizationHeaderBoundaryTable(t *testing.T) {
	t.Parallel()
	canonical := bearerAuthorizationWire(t, "token-123")
	maximumToken := strings.Repeat("Z", exchange.BearerAuthorizationTokenMaximumBytes)
	prefix := exchange.BearerAuthorizationScheme + " "
	cases := []struct {
		name      string
		inputs    [][]string
		wantToken string
		wantErr   error
	}{
		{name: "scheme case never changes opaque token bytes", inputs: [][]string{{canonical}, {strings.ToLower(exchange.BearerAuthorizationScheme) + " token-123"}, {strings.ToUpper(exchange.BearerAuthorizationScheme) + " token-123"}}, wantToken: "token-123"},
		{name: "absence and empty field cannot manufacture credentials", inputs: [][]string{nil, {}, {""}}, wantErr: core.ErrExchangeRequest},
		{name: "identical duplicate fields cannot evade exact cardinality", inputs: [][]string{{canonical, canonical}}, wantErr: core.ErrExchangeRequest},
		{name: "conflicting credentials cannot select the first field", inputs: [][]string{{canonical, bearerAuthorizationWire(t, "foreign")}, {bearerAuthorizationWire(t, "foreign"), canonical}}, wantErr: core.ErrExchangeRequest},
		{name: "comma combined fields cannot become one token", inputs: [][]string{{canonical + ", " + canonical}}, wantErr: core.ErrExchangeRequest},
		{name: "sibling scheme cannot become bearer authentication", inputs: [][]string{{"Basic token-123"}}, wantErr: core.ErrExchangeRequest},
		{name: "missing separator cannot be inferred", inputs: [][]string{{exchange.BearerAuthorizationScheme + "token-123"}}, wantErr: core.ErrExchangeRequest},
		{name: "tab cannot replace the scheme separator", inputs: [][]string{{exchange.BearerAuthorizationScheme + "\ttoken-123"}}, wantErr: core.ErrExchangeRequest},
		{name: "leading whitespace cannot be silently discarded", inputs: [][]string{{" " + canonical}}, wantErr: core.ErrExchangeRequest},
		{name: "additional separator cannot be token material", inputs: [][]string{{prefix + " token-123"}}, wantErr: core.ErrExchangeRequest},
		{name: "trailing whitespace cannot be silently discarded", inputs: [][]string{{canonical + " "}}, wantErr: core.ErrExchangeRequest},
		{name: "empty token cannot be manufactured by the scheme", inputs: [][]string{{prefix}}, wantErr: core.ErrExchangeRequest},
		{name: "orphan padding cannot authenticate", inputs: [][]string{{prefix + "="}, {prefix + "=="}}, wantErr: core.ErrExchangeRequest},
		{name: "material cannot resume after terminal padding", inputs: [][]string{{prefix + "A=A"}}, wantErr: core.ErrExchangeRequest},
		{name: "control bytes cannot frame another header", inputs: [][]string{{canonical + "\r\nX: injected"}, {canonical + "\x00"}}, wantErr: core.ErrExchangeRequest},
		{name: "minimum credential remains exact", inputs: [][]string{{bearerAuthorizationWire(t, "A")}}, wantToken: "A"},
		{name: "one below complete header ceiling is retained", inputs: [][]string{{bearerAuthorizationWire(t, maximumToken[:len(maximumToken)-1])}}, wantToken: maximumToken[:len(maximumToken)-1]},
		{name: "exact complete header ceiling is retained", inputs: [][]string{{bearerAuthorizationWire(t, maximumToken)}}, wantToken: maximumToken},
		{name: "one above complete header ceiling returns no token", inputs: [][]string{{prefix + maximumToken + "Z"}}, wantErr: core.ErrExchangeRequest},
		{name: "extreme complete header cannot reach token allocation", inputs: [][]string{{prefix + strings.Repeat(maximumToken, 2)}}, wantErr: core.ErrExchangeRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, fields := range tc.inputs {
				body := &bindingObservedBody{reader: strings.NewReader("untouched")}
				request := httptest.NewRequest(http.MethodGet, "/", body)
				name := exchange.StandardHeaderAuthorization.String()
				request.Header[name] = append([]string(nil), fields...)
				recorder := httptest.NewRecorder()
				recorder.Code = 0
				call, err := exchange.NewSocketServerCall(recorder, request)
				if err != nil {
					t.Fatalf("socket fixture = %v, want nil", err)
				}
				got, gotErr := exchange.ReceiveBearerAuthorization(call)
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("receive error = %v, want %v", gotErr, tc.wantErr)
				}
				if recorder.Code != 0 || recorder.Body.Len() != 0 || len(recorder.Header()) != 0 || body.reads != 0 || body.closes != 0 {
					t.Fatal("header parsing touched body custody or emitted a response")
				}
				if tc.wantErr != nil {
					if got.Token != nil || !errors.Is(gotErr, core.ErrExchangeContract) {
						t.Fatalf("refused credential = (%x,%v), want exact zero and typed contract identity", got.Token, gotErr)
					}
					continue
				}
				if err := got.Validate(); err != nil || string(got.Token) != tc.wantToken {
					t.Fatalf("received credential = (%x,%v), want exact declared token", got.Token, err)
				}
				clear(got.Token)
				if request.Header.Get(name) != fields[0] {
					t.Fatal("clearing receiver mutated borrowed request field")
				}
			}
		})
	}
}

func bearerAuthorizationWire(t testing.TB, token string) string {
	t.Helper()
	header, err := exchange.NewBearerAuthorizationHeader(exchange.BearerAuthorization{Token: []byte(token)})
	if err != nil {
		t.Fatalf("typed bearer fixture = %v, want nil", err)
	}
	wire, err := header.Values[0].Value()
	if err != nil {
		t.Fatalf("bearer wire fixture = %v, want nil", err)
	}
	return wire
}

func TestBearerAuthorizationMatchesLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		left, right string
		wantMatch   bool
		wantErr     error
	}{
		{name: "minimum exact credentials match", left: "A", right: "A", wantMatch: true},
		{name: "maximum exact credentials remain comparable", left: strings.Repeat("A", exchange.BearerAuthorizationTokenMaximumBytes), right: strings.Repeat("A", exchange.BearerAuthorizationTokenMaximumBytes), wantMatch: true},
		{name: "first byte disagreement cannot authenticate", left: "Abcd", right: "Zbcd"},
		{name: "interior byte disagreement cannot authenticate", left: "Abcd", right: "AbZd"},
		{name: "last byte disagreement cannot authenticate", left: "Abcd", right: "AbcZ"},
		{name: "shorter candidate cannot authenticate by prefix", left: "Abc", right: "Abcd"},
		{name: "longer candidate cannot authenticate by prefix", left: "Abcd", right: "Abc"},
		{name: "padding remains opaque credential material", left: "A=", right: "A=="},
		{name: "invalid left operand cannot compare equal", left: "=", right: "A", wantErr: core.ErrExchangeContract},
		{name: "invalid right operand cannot compare equal", left: "A", right: "=", wantErr: core.ErrExchangeContract},
		{name: "identical malformed operands cannot authenticate", left: "A=A", right: "A=A", wantErr: core.ErrExchangeContract},
		{name: "two absent credentials cannot authenticate", wantErr: core.ErrExchangeContract},
		{name: "equal oversized credentials cannot bypass admission", left: strings.Repeat("A", exchange.BearerAuthorizationTokenMaximumBytes+1), right: strings.Repeat("A", exchange.BearerAuthorizationTokenMaximumBytes+1), wantErr: core.ErrExchangeContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			left, right := exchange.BearerAuthorization{Token: []byte(tc.left)}, exchange.BearerAuthorization{Token: []byte(tc.right)}
			got, gotErr := exchange.BearerAuthorizationMatches(left, right)
			if got != tc.wantMatch || !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("credential match = (%t, %v), want (%t, %v)", got, gotErr, tc.wantMatch, tc.wantErr)
			}
			if string(left.Token) != tc.left || string(right.Token) != tc.right {
				t.Fatal("comparison mutated caller-owned credentials")
			}
		})
	}
}

// Exhaust the byte alphabet at each grammar state. Spelling variants are
// vectors inside a state row, not 768 quota entries. The independent regexp
// recognizes the complete token, rather than calling Exchange's byte helper.
func TestBearerAuthorizationGrammarTransitionsTable(t *testing.T) {
	t.Parallel()
	grammar := regexp.MustCompile(`^[A-Za-z0-9._~+/-]+=*$`)
	cases := []struct{ name, prefix string }{
		{name: "initial byte cannot be padding or foreign alphabet"},
		{name: "material may continue or enter terminal padding", prefix: "A"},
		{name: "padding cannot return to token material", prefix: "A="},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for value := range 256 {
				input := append([]byte(tc.prefix), byte(value))
				var wantErr error
				if !grammar.Match(input) {
					wantErr = core.ErrExchangeContract
				}
				gotErr := (exchange.BearerAuthorization{Token: input}).Validate()
				if !errors.Is(gotErr, wantErr) {
					t.Fatalf("grammar state %q plus byte %#02x = %v, want %v", tc.prefix, value, gotErr, wantErr)
				}
			}
		})
	}
}

func TestBearerAuthorizationHeaderCustodyTable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		token   []byte
		wantErr error
	}{
		{name: "absent token produces no header", wantErr: core.ErrExchangeContract},
		{name: "minimum material is sufficient", token: []byte("A")},
		{name: "all grammar classes remain opaque and exact", token: []byte("AZaz09-._~+/==")},
		{name: "one below token ceiling retains final material", token: bytes.Repeat([]byte{'Z'}, exchange.BearerAuthorizationTokenMaximumBytes-1)},
		{name: "exact token ceiling remains bounded", token: bytes.Repeat([]byte{'Z'}, exchange.BearerAuthorizationTokenMaximumBytes)},
		{name: "one above token ceiling cannot allocate a header", token: bytes.Repeat([]byte{'Z'}, exchange.BearerAuthorizationTokenMaximumBytes+1), wantErr: core.ErrExchangeContract},
		{name: "extreme token extent cannot bypass the ceiling", token: bytes.Repeat([]byte{'Z'}, 2*exchange.BearerAuthorizationTokenMaximumBytes), wantErr: core.ErrExchangeContract},
		{name: "maximum trailing padding is opaque material", token: []byte("A" + strings.Repeat("=", exchange.BearerAuthorizationTokenMaximumBytes-1))},
		{name: "terminal padding cannot be followed by material", token: []byte("A=A"), wantErr: core.ErrExchangeContract},
		{name: "orphan padding cannot become authentication material", token: []byte("="), wantErr: core.ErrExchangeContract},
		{name: "embedded control cannot frame another HTTP field", token: []byte("A\r\nX: injected"), wantErr: core.ErrExchangeContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			token := bytes.Clone(tc.token)
			authorization := exchange.BearerAuthorization{Token: token}
			got, gotErr := exchange.NewBearerAuthorizationHeader(authorization)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("header construction = %v, want %v", gotErr, tc.wantErr)
			}
			if !bytes.Equal(token, tc.token) {
				t.Fatal("header construction mutated caller token")
			}
			for _, format := range []string{"%v", "%+v", "%#v", "%s", "%q", "%x", "%020s", "%.1s"} {
				if text := fmt.Sprintf(format, authorization); text != core.RedactedValueText {
					t.Fatalf("format %q = %q, want %q", format, text, core.RedactedValueText)
				}
			}
			if tc.wantErr != nil {
				if got.Name != (core.HTTPHeaderName{}) || got.Values != nil {
					t.Fatalf("refused header = %v, want zero", got)
				}
				return
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("header validation = %v, want nil", err)
			}
			if got.Name.String() != exchange.StandardHeaderAuthorization.String() || len(got.Values) != 1 {
				t.Fatalf("header name/count = %v/%d, want Authorization/1", got.Name, len(got.Values))
			}
			clear(token)
			wire, wireErr := got.Values[0].Value()
			wantWire := exchange.BearerAuthorizationScheme + " " + string(tc.token)
			if wireErr != nil || wire != wantWire {
				t.Fatalf("header after caller clear = (%q, %v), want (%q, nil)", wire, wireErr, wantWire)
			}
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.Header.Set(got.Name.String(), wire)
			parsed, parseErr := exchange.ReceiveBearerAuthorization(socketServerCall(t, request))
			if parseErr != nil || !bytes.Equal(parsed.Token, tc.token) {
				t.Fatalf("receive = (%x, %v), want (%x, nil)", parsed.Token, parseErr, tc.token)
			}
			clear(parsed.Token)
			if retained := request.Header.Get(got.Name.String()); retained != wantWire {
				t.Fatalf("request field after receiver clear = %q, want %q", retained, wantWire)
			}
		})
	}
}
