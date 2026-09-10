package googleidentity

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const googleTestToken = "eyJhbGciOiJSUzI1NiJ9.eyJhdWQiOiJ0ZXN0In0.signature"

func mustGooglePolicy(tb testing.TB) Policy {
	tb.Helper()
	got, err := DefaultPolicy()
	if err != nil {
		tb.Fatalf("DefaultPolicy() error = %v, want nil", err)
	}
	return got
}
func mustGoogleAudience(tb testing.TB) Audience {
	tb.Helper()
	got, err := ParseAudience("https://api.example.com/release")
	if err != nil {
		tb.Fatalf("ParseAudience() error = %v, want nil", err)
	}
	return got
}

func googleTestClient(tb testing.TB, handler http.Handler) Client {
	return googleTestClientWithProxy(tb, handler, nil)
}

func googleTestClientWithProxy(
	tb testing.TB,
	handler http.Handler,
	proxy func(*http.Request) (*url.URL, error),
) Client {
	tb.Helper()
	server := httptest.NewServer(handler)
	tb.Cleanup(server.Close)
	standard, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		tb.Fatalf("transport = %T, want *http.Transport", http.DefaultTransport)
	}
	transport := standard.Clone()
	tb.Cleanup(transport.CloseIdleConnections)
	transport.Proxy = proxy
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		var dialer net.Dialer
		return dialer.DialContext(ctx, network, server.Listener.Addr().String())
	}
	httpClient := &http.Client{Transport: transport}
	exchangeClient, err := exchange.NewClient(httpClient)
	if err != nil {
		tb.Fatalf("exchange.NewClient() error = %v, want nil", err)
	}
	got, err := NewClient(exchangeClient)
	if err != nil {
		tb.Fatalf("NewClient() error = %v, want nil", err)
	}
	return got
}

func TestGoogleAudienceHostileBoundaryTable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		wantErr error
		name    string
		value   string
	}{
		{name: "one ASCII byte reaches minimum", value: "a"}, {name: "service URL remains exact", value: "https://api.example.com"},
		{name: "custom audience remains exact", value: "release-broker"}, {name: "OAuth identifier remains exact", value: "123.apps.googleusercontent.com"},
		{name: "query delimiters remain data", value: "service?tenant=one&role=writer"}, {name: "space remains data", value: "service audience"},
		{name: "plus remains data", value: "service+audience"}, {name: "Unicode remains exact", value: "服务"},
		{name: "one below former maximum is admitted", value: strings.Repeat("a", googleFormerAudienceBytes-1)}, {name: "exact former maximum is admitted", value: strings.Repeat("a", googleFormerAudienceBytes)},
		{name: "empty value is refused", wantErr: core.ErrGoogleIdentityContract}, {name: "beyond former maximum is admitted", value: strings.Repeat("a", googleFormerAudienceBytes+1)},
		{name: "many former windows are admitted", value: strings.Repeat("a", 4*googleFormerAudienceBytes)},
		{name: "single invalid UTF8 byte is refused", value: string([]byte{0xff}), wantErr: core.ErrGoogleIdentityContract},
		{name: "truncated two byte UTF8 is refused", value: string([]byte{0xc2}), wantErr: core.ErrGoogleIdentityContract},
		{name: "truncated three byte UTF8 is refused", value: string([]byte{0xe2, 0x82}), wantErr: core.ErrGoogleIdentityContract},
		{name: "surrogate UTF8 is refused", value: string([]byte{0xed, 0xa0, 0x80}), wantErr: core.ErrGoogleIdentityContract},
		{name: "overlong UTF8 is refused", value: string([]byte{0xc0, 0xaf}), wantErr: core.ErrGoogleIdentityContract},
		{name: "invalid suffix at former extent is refused", value: strings.Repeat("a", googleFormerAudienceBytes-1) + string([]byte{0xff}), wantErr: core.ErrGoogleIdentityContract},
		{name: "large multibyte extent is admitted", value: strings.Repeat("界", googleFormerAudienceBytes/2)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseAudience(tc.value)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || got != (Audience{}) {
					t.Fatalf("ParseAudience() = (%v, %v), want zero and %v", got, err, tc.wantErr)
				}
				return
			}
			if err != nil || got.Validate() != nil || got.String() != tc.value {
				t.Fatalf("ParseAudience() = (%q, %v), want (%q, nil)", got.String(), err, tc.value)
			}
		})
	}
}

func TestGoogleCommandOutputHostileBoundaryTable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		wantErr error
		name    string
		value   string
	}{
		{name: "one token byte reaches minimum", value: "a"}, {name: "three lexical segments remain opaque", value: "a.b.c"}, {name: "standard base64 alphabet is admitted", value: "abc+/"}, {name: "URL safe alphabet is admitted", value: "abc-_"}, {name: "tilde is admitted", value: "abc~"}, {name: "single padding is admitted", value: "abc="}, {name: "double padding is admitted", value: "abc=="}, {name: "line feed is trimmed", value: "abc\n"}, {name: "carriage return line feed is trimmed", value: "abc\r\n"}, {name: "exact former maximum is admitted", value: strings.Repeat("a", googleFormerTokenBytes)},
		{name: "empty output is refused", wantErr: core.ErrGoogleIdentityContract}, {name: "leading padding is refused", value: "=abc", wantErr: core.ErrGoogleIdentityContract}, {name: "interior padding is refused", value: "ab=c", wantErr: core.ErrGoogleIdentityContract}, {name: "space is refused", value: "ab c", wantErr: core.ErrGoogleIdentityContract}, {name: "tab is refused", value: "ab\tc", wantErr: core.ErrGoogleIdentityContract}, {name: "interior newline is refused", value: "ab\nc", wantErr: core.ErrGoogleIdentityContract}, {name: "bare carriage return is refused", value: "abc\r", wantErr: core.ErrGoogleIdentityContract}, {name: "double line feed is refused", value: "abc\n\n", wantErr: core.ErrGoogleIdentityContract}, {name: "leading line feed is refused", value: "\nabc", wantErr: core.ErrGoogleIdentityContract}, {name: "trailing space is refused", value: "abc ", wantErr: core.ErrGoogleIdentityContract},
		{name: "comma is refused", value: "ab,c", wantErr: core.ErrGoogleIdentityContract}, {name: "non ASCII is refused", value: "ab界c", wantErr: core.ErrGoogleIdentityContract}, {name: "token beyond former maximum is admitted", value: strings.Repeat("a", googleFormerTokenBytes+1)}, {name: "output beyond former framing extent is admitted", value: strings.Repeat("a", (googleFormerTokenBytes+2)+1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseGoogleCloudCommandOutput([]byte(tc.value))
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || got != (Token{}) {
					t.Fatalf("ParseGoogleCloudCommandOutput() = (%v, %v), want zero and %v", got, err, tc.wantErr)
				}
				return
			}
			bearer, bearerErr := got.BearerValue()
			want := "Bearer " + strings.TrimSuffix(strings.TrimSuffix(tc.value, "\n"), "\r")
			if err != nil || bearerErr != nil || bearer != want || fmt.Sprint(got) != core.RedactedValueText {
				t.Fatalf("token closure = (%q, %v, %v), want (%q, nil, nil)", bearer, err, bearerErr, want)
			}
		})
	}
}

func TestLayerTriadGoogleIdentityAcquireRealHTTP(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr   error
		name      string
		body      string
		flavor    string
		status    int
		wantCalls uint64
		cancel    bool
	}{
		{name: "provider fact with response flavor produces redacted bearer", body: googleTestToken, flavor: googleMetadataHeaderValue, status: http.StatusOK, wantCalls: 1},
		{name: "missing response flavor refuses an otherwise valid bearer", body: googleTestToken, status: http.StatusOK, wantErr: core.ErrGoogleIdentityContract, wantCalls: 1},
		{name: "foreign response flavor refuses an otherwise valid bearer", body: googleTestToken, flavor: "Foreign", status: http.StatusOK, wantErr: core.ErrGoogleIdentityContract, wantCalls: 1},
		{name: "provider token crosses many former windows", body: strings.Repeat("a", 4*googleFormerTokenBytes), flavor: googleMetadataHeaderValue, status: http.StatusOK, wantCalls: 1},
		{name: "empty provider fact is refused", flavor: googleMetadataHeaderValue, status: http.StatusOK, wantErr: core.ErrGoogleIdentityContract, wantCalls: 1},
		{name: "cancelled intent performs no effect", body: googleTestToken, flavor: googleMetadataHeaderValue, status: http.StatusOK, cancel: true, wantErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Uint64
			client := googleTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.URL.Path != googleMetadataIdentityPath || r.Header.Get(googleMetadataHeaderName) != googleMetadataHeaderValue {
					t.Errorf("metadata request = %s %q, want %s and %q", r.URL.Path, r.Header.Get(googleMetadataHeaderName), googleMetadataIdentityPath, googleMetadataHeaderValue)
				}
				if tc.flavor != "" {
					w.Header().Set(googleMetadataHeaderName, tc.flavor)
				}
				w.WriteHeader(tc.status)
				if _, err := w.Write([]byte(tc.body)); err != nil {
					t.Errorf("provider write: %v", err)
				}
			}))
			ctx := t.Context()
			if tc.cancel {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			got, err := AcquireGoogleCloud(ctx, client, IdentityTokenRequest{Audience: mustGoogleAudience(t), Policy: mustGooglePolicy(t)})
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || got != (Token{}) {
					t.Fatalf("AcquireGoogleCloud() = (%v, %v), want zero and %v", got, err, tc.wantErr)
				}
			} else {
				bearer, bearerErr := got.BearerValue()
				if err != nil || bearerErr != nil || bearer != bearerPrefix+tc.body {
					t.Fatalf("AcquireGoogleCloud() bearer = (%q, %v, %v), want valid", bearer, err, bearerErr)
				}
			}
			if calls.Load() != tc.wantCalls {
				t.Fatalf("metadata calls = %d, want %d", calls.Load(), tc.wantCalls)
			}
		})
	}
}

func TestGoogleMetadataAcquisitionBypassesConfiguredProxy(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		proxy     bool
		cancel    bool
		wantCalls uint64
		wantErr   error
	}{
		{name: "configured_proxy_cannot_observe_metadata", proxy: true, wantCalls: 1},
		{name: "absent_proxy_keeps_direct_metadata", wantCalls: 1},
		{name: "cancelled_intent_reaches_neither_proxy_nor_metadata", proxy: true, cancel: true, wantErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls, proxyCalls atomic.Uint64
			var proxy func(*http.Request) (*url.URL, error)
			if tc.proxy {
				proxy = func(*http.Request) (*url.URL, error) { proxyCalls.Add(1); return nil, core.ErrGoogleIdentityContract }
			}
			client := googleTestClientWithProxy(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.Header().Set(googleMetadataHeaderName, googleMetadataHeaderValue)
				if _, err := io.WriteString(w, googleTestToken); err != nil {
					t.Errorf("provider write: %v", err)
				}
			}), proxy)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancel {
				cancel()
			}
			got, err := client.Acquire(ctx, IdentityTokenRequest{Audience: mustGoogleAudience(t), Policy: mustGooglePolicy(t)})
			if !errors.Is(err, tc.wantErr) || calls.Load() != tc.wantCalls || proxyCalls.Load() != 0 {
				t.Fatalf("error=%v metadata=%d proxy=%d, want %v, %d, 0", err, calls.Load(), proxyCalls.Load(), tc.wantErr, tc.wantCalls)
			}
			if tc.wantErr != nil {
				if got != (Token{}) {
					t.Fatalf("refused token=%v, want zero", got)
				}
				return
			}
			bearer, err := got.BearerValue()
			if err != nil || bearer != bearerPrefix+googleTestToken {
				t.Fatalf("bearer matches=%t error=%v, want exact metadata token", bearer == bearerPrefix+googleTestToken, err)
			}
		})
	}
}

func TestLayerTriadGoogleAccessTokenRealHTTP(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		change   func(*googleAccessTokenResponse)
		mutate   func([]byte) []byte
		noFlavor bool
		cancel   bool
		wantErr  error
	}{
		{name: "minimum_positive_lifetime_remains_exact", change: func(w *googleAccessTokenResponse) { w.ExpiresIn = 1 }},
		{name: "maximum_representable_lifetime_does_not_wrap", change: func(w *googleAccessTokenResponse) {
			w.ExpiresIn = googleAccessTokenLifetimeSeconds(GoogleCloudAccessTokenLifetimeMaximumSeconds)
		}},
		{name: "above_native_lifetime_representation_is_refused", change: func(w *googleAccessTokenResponse) {
			w.ExpiresIn = googleAccessTokenLifetimeSeconds(GoogleCloudAccessTokenLifetimeMaximumSeconds) + 1
		}, wantErr: core.ErrGoogleIdentityContract},
		{name: "zero_lifetime_cannot_invent_valid_token", change: func(w *googleAccessTokenResponse) { w.ExpiresIn = 0 }, wantErr: core.ErrGoogleIdentityContract},
		{name: "negative_lifetime_cannot_wrap_unsigned", mutate: func(b []byte) []byte { return bytes.Replace(b, []byte(":300"), []byte(":-1"), 1) }, wantErr: core.ErrGoogleIdentityContract},
		{name: "token_crosses_many_former_windows", change: func(w *googleAccessTokenResponse) { w.AccessToken = strings.Repeat("a", 4*googleFormerTokenBytes) }},
		{name: "large_token_still_refuses_invalid_suffix", change: func(w *googleAccessTokenResponse) {
			w.AccessToken = strings.Repeat("a", 4*googleFormerTokenBytes) + " "
		}, wantErr: core.ErrGoogleIdentityContract},
		{name: "missing_metadata_flavor_cannot_authenticate_source", noFlavor: true, wantErr: core.ErrGoogleIdentityContract},
		{name: "unknown_response_member_is_refused", mutate: func(b []byte) []byte { return append(b[:len(b)-1], []byte(",\"unknown\":true}")...) }, wantErr: core.ErrGoogleIdentityContract},
		{name: "cancelled_intent_performs_no_effect", cancel: true, wantErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			wire := googleAccessTokenResponse{AccessToken: googleTestToken, TokenType: googleAccessTokenTypeBearer, ExpiresIn: 300}
			if tc.change != nil {
				tc.change(&wire)
			}
			body, err := core.MarshalCanonicalJSONDocument(wire)
			if err != nil {
				t.Fatal(err)
			}
			if tc.mutate != nil {
				body = tc.mutate(body)
			}
			var calls atomic.Uint64
			client := googleTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != http.MethodGet || r.URL.Path != googleMetadataAccessTokenPath || r.Header.Get(googleMetadataHeaderName) != googleMetadataHeaderValue {
					t.Errorf("request=%v %v flavor=%v, want exact metadata intent", r.Method, r.URL.Path, r.Header.Get(googleMetadataHeaderName))
				}
				if !tc.noFlavor {
					w.Header().Set(googleMetadataHeaderName, googleMetadataHeaderValue)
				}
				if _, err := w.Write(body); err != nil {
					t.Errorf("provider write: %v", err)
				}
			}))
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancel {
				cancel()
			}
			got, err := AcquireGoogleCloudAccessToken(ctx, client, GoogleCloudAccessTokenRequest{Policy: mustGooglePolicy(t)})
			wantCalls := uint64(1)
			if tc.cancel {
				wantCalls = 0
			}
			if !errors.Is(err, tc.wantErr) || calls.Load() != wantCalls {
				t.Fatalf("error=%v calls=%d, want %v and %d", err, calls.Load(), tc.wantErr, wantCalls)
			}
			if tc.wantErr != nil {
				if got != (AccessToken{}) || !errors.Is(err, core.ErrGoogleIdentityContract) {
					t.Fatalf("refused token=%v error=%v, want zero typed refusal", got, err)
				}
				return
			}
			bearer, err := got.BearerValue()
			if err != nil || bearer != bearerPrefix+wire.AccessToken || got.Lifetime().Nanoseconds() != int64(wire.ExpiresIn)*int64(temporal.NanosecondsPerSecond) || got.Validate() != nil {
				t.Fatalf("token matches=%t lifetime=%v error=%v, want exact provider receipt", bearer == bearerPrefix+wire.AccessToken, got.Lifetime(), err)
			}
		})
	}
}

type googleExternalDoorInventory struct {
	Audience_UnmarshalText        func(*Audience, []byte) error
	Client_Acquire                func(Client, context.Context, IdentityTokenRequest) (Token, error)
	ServiceAccountSource_Acquire  func(ServiceAccountSource, context.Context, IdentityTokenRequest) (Token, error)
	GoogleCloudVerifier_Verify    func(GoogleCloudVerifier, context.Context, string) (GoogleCloudVerifiedIdentity, error)
	AcquireGoogleCloud            func(context.Context, Client, IdentityTokenRequest) (Token, error)
	AcquireGoogleCloudAccessToken func(context.Context, Client, GoogleCloudAccessTokenRequest) (AccessToken, error)
	NewGoogleCloudVerifier        func(context.Context, GoogleCloudVerifierConfiguration) (GoogleCloudVerifier, error)
	ParseAudience                 func(string) (Audience, error)
	ParseGoogleCloudCommandOutput func([]byte) (Token, error)
}

var googleExternalDoors = googleExternalDoorInventory{Audience_UnmarshalText: (*Audience).UnmarshalText, Client_Acquire: Client.Acquire, ServiceAccountSource_Acquire: ServiceAccountSource.Acquire, GoogleCloudVerifier_Verify: GoogleCloudVerifier.Verify, AcquireGoogleCloud: AcquireGoogleCloud, AcquireGoogleCloudAccessToken: AcquireGoogleCloudAccessToken, NewGoogleCloudVerifier: NewGoogleCloudVerifier, ParseAudience: ParseAudience, ParseGoogleCloudCommandOutput: ParseGoogleCloudCommandOutput}

func TestGoogleExternalDoorInventoryMatchesProduction(t *testing.T) {
	t.Parallel()
	got, err := scanGoogleExternalDoors()
	if err != nil {
		t.Fatalf("scanGoogleExternalDoors() error = %v, want nil", err)
	}
	typeOf := reflect.TypeOf(googleExternalDoors)
	want := make([]string, 0, typeOf.NumField())
	for field := range typeOf.Fields() {
		want = append(want, field.Name)
	}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("Google external doors = %q, want %q", got, want)
	}
}
func scanGoogleExternalDoors() ([]string, error) {
	entries, err := fs.Glob(googleIdentitySource, "*.go")
	if err != nil {
		return nil, err
	}
	set := token.NewFileSet()
	var doors []string
	for _, entry := range entries {
		if strings.HasSuffix(entry, "_test.go") {
			continue
		}
		data, readErr := googleIdentitySource.ReadFile(entry)
		if readErr != nil {
			return nil, readErr
		}
		file, parseErr := parser.ParseFile(set, entry, data, parser.SkipObjectResolution)
		if parseErr != nil {
			return nil, parseErr
		}
		ast.Inspect(file, func(node ast.Node) bool {
			declaration, ok := node.(*ast.FuncDecl)
			if !ok {
				return true
			}
			name := declaration.Name.Name
			if declaration.Recv != nil {
				if name == "Verify" || name == "Acquire" || name == "UnmarshalText" {
					doors = append(doors, googleIdentityReceiverName(declaration.Recv.List[0].Type)+"_"+name)
				}
				return false
			}
			if strings.HasPrefix(name, "Acquire") || strings.HasPrefix(name, "Parse") || name == "NewGoogleCloudVerifier" {
				doors = append(doors, name)
			}
			return false
		})
	}
	slices.Sort(doors)
	return doors, nil
}

func FuzzGoogleAccessTokenResponseSemanticClosure(f *testing.F) {
	wire := googleAccessTokenResponse{AccessToken: googleTestToken, TokenType: googleAccessTokenTypeBearer, ExpiresIn: 300}
	canonical, err := core.MarshalCanonicalJSONDocument(wire)
	if err != nil {
		f.Fatal(err)
	}
	grammar := regexp.MustCompile(googleTestBearerGrammar)
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte("null"))
	f.Add(append(bytes.Repeat([]byte(" "), 32<<10), canonical...))
	f.Fuzz(func(t *testing.T, data []byte) {
		var reference googleAccessTokenResponse
		decodeErr := json.Unmarshal(data, &reference, json.RejectUnknownMembers(true))
		wantOK := decodeErr == nil && reference.TokenType == googleAccessTokenTypeBearer && reference.ExpiresIn > 0 && uint64(reference.ExpiresIn) <= GoogleCloudAccessTokenLifetimeMaximumSeconds && grammar.MatchString(reference.AccessToken)
		got, err := decodeGoogleAccessTokenResponse(data)
		if (err == nil) != wantOK {
			t.Fatalf("access response admitted=%t error=%v, want %t", err == nil, err, wantOK)
		}
		if !wantOK {
			if !errors.Is(err, core.ErrGoogleIdentityContract) || got != (googleAccessTokenResponse{}) {
				t.Fatalf("rejected response=%v error=%v, want zero typed refusal", got, err)
			}
		} else {
			if got != reference || got.Validate() != nil {
				t.Fatalf("response=%v, want exact typed reference %v", got, reference)
			}
			first, err := core.MarshalCanonicalJSONDocument(got)
			if err != nil {
				t.Fatal(err)
			}
			round, err := decodeGoogleAccessTokenResponse(first)
			if err != nil || round != got {
				t.Fatalf("round trip=%v error=%v, want %v", round, err, got)
			}
			second, err := core.MarshalCanonicalJSONDocument(round)
			if err != nil || !bytes.Equal(first, second) {
				t.Fatalf("second canonical equal=%t error=%v, want true and nil", bytes.Equal(first, second), err)
			}
		}
		var calls atomic.Uint64
		client := googleTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			if r.URL.Path != googleMetadataAccessTokenPath || r.Method != http.MethodGet {
				t.Errorf("metadata request=%v %v, want GET access-token path", r.Method, r.URL.Path)
			}
			w.Header().Set(googleMetadataHeaderName, googleMetadataHeaderValue)
			if _, err := w.Write(data); err != nil {
				t.Errorf("response Write error=%v, want nil", err)
			}
		}))
		acquired, acquireErr := AcquireGoogleCloudAccessToken(t.Context(), client, GoogleCloudAccessTokenRequest{Policy: mustGooglePolicy(t)})
		if calls.Load() != 1 || (acquireErr == nil) != wantOK {
			t.Fatalf("acquisition calls=%d error=%v, want one call and admitted=%t", calls.Load(), acquireErr, wantOK)
		}
		if !wantOK {
			if acquired != (AccessToken{}) || !errors.Is(acquireErr, core.ErrGoogleIdentityContract) {
				t.Fatalf("rejected acquisition=%v error=%v, want zero typed refusal", acquired, acquireErr)
			}
			return
		}
		text, err := acquired.BearerValue()
		if err != nil || text != bearerPrefix+reference.AccessToken || acquired.Lifetime().Nanoseconds() != int64(reference.ExpiresIn)*int64(temporal.NanosecondsPerSecond) {
			t.Fatalf("acquired token matches=%t lifetime=%v error=%v, want exact provider facts", text == bearerPrefix+reference.AccessToken, acquired.Lifetime(), err)
		}
	})
}
func FuzzParseGoogleCloudCommandOutputSemanticClosure(f *testing.F) {
	seed, err := newToken(googleTestToken)
	if err != nil {
		f.Fatal(err)
	}
	bearer, err := seed.BearerValue()
	if err != nil {
		f.Fatal(err)
	}
	canonical := []byte(strings.TrimPrefix(bearer, bearerPrefix))
	f.Add(canonical)
	f.Add(append(bytes.Clone(canonical), '\r', '\n'))
	f.Add([]byte{})
	f.Add([]byte("=bad"))
	f.Add(bytes.Repeat([]byte("a"), googleFormerTokenBytes+1))
	grammar := regexp.MustCompile(googleTestBearerGrammar)
	f.Fuzz(func(t *testing.T, data []byte) {
		raw := string(data)
		if before, ok := strings.CutSuffix(raw, "\n"); ok {
			raw = before
			raw = strings.TrimSuffix(raw, "\r")
		}
		wantOK := grammar.MatchString(raw)
		got, err := ParseGoogleCloudCommandOutput(data)
		if (err == nil) != wantOK {
			t.Fatalf("command admitted=%t error=%v, want %t", err == nil, err, wantOK)
		}
		if !wantOK {
			if !errors.Is(err, core.ErrGoogleIdentityContract) || got != (Token{}) {
				t.Fatalf("rejected command=%v error=%v, want zero typed refusal", got, err)
			}
			return
		}
		value, err := got.BearerValue()
		if err != nil || value != bearerPrefix+raw || got.Validate() != nil || fmt.Sprint(got) != core.RedactedValueText {
			t.Fatalf("command disclosure matches=%t error=%v, want exact opaque bytes and redaction", value == bearerPrefix+raw, err)
		}
		canonicalToken, err := ParseGoogleCloudCommandOutput([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		second, err := canonicalToken.BearerValue()
		if err != nil || second != value {
			t.Fatalf("canonical disclosure matches=%t error=%v, want true and nil", second == value, err)
		}
	})
}
