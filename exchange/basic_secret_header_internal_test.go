package exchange

import (
	"bytes"
	"encoding/base64"
	"errors"
	"go/ast"
	"go/token"
	"net/http"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestBasicSecretHeaderOwnsBytesAndNativeWire(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, identity, password string }{
		{"minimum identity and secret", "a", "b"}, {"colon remains in password", "user", "a:b:c"},
		{"UTF8 identity and password", "café", "密碼"}, {"space remains valid password", "user", "a b"},
		{"identity at native bound", strings.Repeat("i", 255), "s"}, {"password at native bound", "u", strings.Repeat("s", 1024)},
		{"base64 no padding", "a", "b"}, {"base64 single padding", "ab", "c"}, {"base64 double padding", "abc", "d"},
		{"maximum combined UTF8 byte extent", strings.Repeat("i", 255), strings.Repeat("é", 512)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := BasicAuthorizationRequest{Identity: BasicAuthorizationIdentity(tc.identity), Secret: []byte(tc.password)}
			h, err := NewBasicAuthorizationSecretHeader(request)
			if err != nil {
				t.Fatal(err)
			}
			defer clear(h.Value)
			if h.Name != coreHeaderAuthorization() || h.Scheme != SecretHeaderBasic {
				t.Fatalf("header name%v scheme%v want Authorization Basic", h.Name, h.Scheme)
			}
			native, err := http.NewRequest(http.MethodGet, "https://example.test", nil)
			if err != nil {
				t.Fatal(err)
			}
			if err = applySecretHeaders(native, []SecretHeader{h}); err != nil {
				t.Fatal(err)
			}
			user, password, ok := native.BasicAuth()
			if !ok || user != tc.identity || password != tc.password {
				t.Fatalf("native decoding matches%t/%t/%t want all true", ok, user == tc.identity, password == tc.password)
			}
			clear(request.Secret)
			if err = h.Validate(); err != nil {
				t.Fatalf("caller-secret cleanup invalidated owned token: %v", err)
			}
			clear(h.Value)
			if !bytes.Equal(h.Value, make([]byte, len(h.Value))) {
				t.Fatalf("zero bytes after cleanup = %d, want %d", bytes.Count(h.Value, []byte{0}), len(h.Value))
			}
		})
	}
}
func TestBasicSecretHeaderRefusesInvalidInputsWithoutPartialCustody(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, identity string
		password       []byte
	}{
		{"absent identity", "", []byte("s")}, {"identity delimiter", "a:b", []byte("s")},
		{"identity over bound", strings.Repeat("i", 256), []byte("s")}, {"identity invalid UTF8", string([]byte{255}), []byte("s")},
		{"identity control", "a\n", []byte("s")}, {"absent password", "u", nil},
		{"password over bound", "u", bytes.Repeat([]byte{'s'}, 1025)}, {"password invalid UTF8", "u", []byte{255}},
		{"password CRLF", "u", []byte("s\r\n")}, {"password Unicode control", "u", []byte("s\u0085")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h, err := NewBasicAuthorizationSecretHeader(BasicAuthorizationRequest{Identity: BasicAuthorizationIdentity(tc.identity), Secret: tc.password})
			if !errors.Is(err, core.ErrExchangeContract) || len(h.Value) != 0 || h.Name != (core.HTTPHeaderName{}) || h.Scheme != SecretHeaderUnknown {
				t.Fatalf("invalid request yielded populated%t error%v want zero and ExchangeContract", len(h.Value) != 0, err)
			}
		})
	}
}
func TestBasicSecretTokenNativeRepresentationBoundaries(t *testing.T) {
	t.Parallel()
	good := base64.StdEncoding.EncodeToString([]byte("u:s"))
	for _, tc := range []struct {
		name, encoded string
		wrongName     bool
		want          bool
	}{
		{name: "canonical token", encoded: good, want: true}, {name: "empty token"}, {name: "wrong header name", encoded: good, wrongName: true},
		{name: "embedded LF", encoded: good[:2] + "\n" + good[2:]}, {name: "embedded CR", encoded: good[:2] + "\r" + good[2:]},
		{name: "prefixed scheme is not token", encoded: "Basic " + good}, {name: "space is not base64", encoded: good + " "},
		{name: "URL alphabet", encoded: "____"}, {name: "no delimiter", encoded: base64.StdEncoding.EncodeToString([]byte("user"))},
		{name: "empty decoded identity", encoded: base64.StdEncoding.EncodeToString([]byte(":s"))},
		{name: "empty decoded password", encoded: base64.StdEncoding.EncodeToString([]byte("u:"))},
		{name: "noncanonical trailing bits", encoded: "dTpzYR=="}, {name: "excess padding", encoded: good + "="},
		{name: "encoded extent over maximum", encoded: strings.Repeat("A", basicAuthorizationBase64MaximumBytes+1)},
		{name: "literal still refuses scheme space", encoded: good, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			name := coreHeaderAuthorization()
			if tc.wrongName {
				name = core.HTTPHeaderContentType()
			}
			h := SecretHeader{Name: name, Value: []byte(tc.encoded), Scheme: SecretHeaderBasic}
			err := h.Validate()
			if tc.want {
				if err != nil {
					t.Fatal(err)
				}
			} else if !errors.Is(err, core.ErrExchangeContract) {
				t.Fatalf("encoded refusal%v want ExchangeContract", err)
			}
			if tc.name == "literal still refuses scheme space" {
				h.Scheme = SecretHeaderLiteral
				h.Value = []byte("Basic " + good)
				if !errors.Is(h.Validate(), core.ErrExchangeContract) {
					t.Fatalf("literal separator validation = %v, want ExchangeContract", h.Validate())
				}
			}
		})
	}
}
func TestBasicSecretScratchCleanupRegisteredBeforeUse(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	found := 0
	for _, source := range exchangeArchitectureSources(t, fset) {
		if source.name != "basic_authorization.go" {
			continue
		}
		if sourceBindsClear(source.syntax) {
			t.Fatalf("%s shadows native clear = true, want false", source.name)
		}
		for _, d := range source.syntax.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || (fn.Name.Name != "NewBasicAuthorizationSecretHeader" && fn.Name.Name != "validateBasicSecretToken") {
				continue
			}
			found++
			if !fixedStorageCleanupRegistered(fn.Body) {
				t.Fatalf("%s lacks immediate full scratch cleanup", fn.Name.Name)
			}
		}
	}
	if found != 2 {
		t.Fatalf("scratch owners found%d want2", found)
	}
}
func FuzzBasicSecretHeaderNativeSemanticClosure(f *testing.F) {
	h, err := NewBasicAuthorizationSecretHeader(BasicAuthorizationRequest{Identity: "u", Secret: []byte("s")})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(bytes.Clone(h.Value))
	clear(h.Value)
	f.Add([]byte{})
	f.Add([]byte("Basic token"))
	f.Fuzz(func(t *testing.T, data []byte) {
		h := SecretHeader{Name: coreHeaderAuthorization(), Value: data, Scheme: SecretHeaderBasic}
		err := h.Validate()
		decoded, decodeErr := base64.StdEncoding.Strict().DecodeString(string(data))
		defer clear(decoded)
		identity, password, delimiter := bytes.Cut(decoded, []byte{':'})
		want := decodeErr == nil && !bytes.ContainsAny(data, "\r\n") && delimiter && (BasicAuthorizationRequest{Identity: BasicAuthorizationIdentity(identity), Secret: password}).Validate() == nil
		if err != nil {
			if !errors.Is(err, core.ErrExchangeContract) || want {
				t.Fatalf("token refusal%v canonical-valid%t want ExchangeContract and false", err, want)
			}
			return
		}
		if !want {
			t.Fatalf("Basic token validation = %v, independent canonical admission = %t, want refusal or true", err, want)
		}
		owned, factoryErr := NewBasicAuthorizationSecretHeader(BasicAuthorizationRequest{Identity: BasicAuthorizationIdentity(identity), Secret: password})
		if factoryErr != nil || !bytes.Equal(owned.Value, data) {
			clear(owned.Value)
			t.Fatalf("canonical factory closure exact%t error%v want true,nil", bytes.Equal(owned.Value, data), factoryErr)
		}
		clear(owned.Value)
		native, err := http.NewRequest(http.MethodGet, "https://example.test", nil)
		if err != nil {
			t.Fatal(err)
		}
		if err = applySecretHeaders(native, []SecretHeader{h}); err != nil {
			t.Fatal(err)
		}
		user, secret, ok := native.BasicAuth()
		if !ok || user != string(identity) || secret != string(password) {
			t.Fatalf("native Basic facts match = %t/%t/%t, want true/true/true", ok, user == string(identity), secret == string(password))
		}
	})
}
