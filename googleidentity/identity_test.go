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
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
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
	transport := http.DefaultTransport.(*http.Transport).Clone()
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
		{name: "one below maximum is admitted", value: strings.Repeat("a", AudienceMaximumBytes-1)}, {name: "exact maximum is admitted", value: strings.Repeat("a", AudienceMaximumBytes)},
		{name: "empty value is refused", wantErr: core.ErrGoogleIdentityContract}, {name: "one above maximum is refused", value: strings.Repeat("a", AudienceMaximumBytes+1), wantErr: core.ErrGoogleIdentityContract},
		{name: "far above maximum is refused", value: strings.Repeat("a", 4*AudienceMaximumBytes), wantErr: core.ErrGoogleIdentityContract},
		{name: "single invalid UTF8 byte is refused", value: string([]byte{0xff}), wantErr: core.ErrGoogleIdentityContract},
		{name: "truncated two byte UTF8 is refused", value: string([]byte{0xc2}), wantErr: core.ErrGoogleIdentityContract},
		{name: "truncated three byte UTF8 is refused", value: string([]byte{0xe2, 0x82}), wantErr: core.ErrGoogleIdentityContract},
		{name: "surrogate UTF8 is refused", value: string([]byte{0xed, 0xa0, 0x80}), wantErr: core.ErrGoogleIdentityContract},
		{name: "overlong UTF8 is refused", value: string([]byte{0xc0, 0xaf}), wantErr: core.ErrGoogleIdentityContract},
		{name: "invalid maximum suffix is refused", value: strings.Repeat("a", AudienceMaximumBytes-1) + string([]byte{0xff}), wantErr: core.ErrGoogleIdentityContract},
		{name: "multibyte extent above bound is refused", value: strings.Repeat("界", AudienceMaximumBytes/2), wantErr: core.ErrGoogleIdentityContract},
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
		{name: "one token byte reaches minimum", value: "a"}, {name: "three lexical segments remain opaque", value: "a.b.c"}, {name: "standard base64 alphabet is admitted", value: "abc+/"}, {name: "URL safe alphabet is admitted", value: "abc-_"}, {name: "tilde is admitted", value: "abc~"}, {name: "single padding is admitted", value: "abc="}, {name: "double padding is admitted", value: "abc=="}, {name: "line feed is trimmed", value: "abc\n"}, {name: "carriage return line feed is trimmed", value: "abc\r\n"}, {name: "exact maximum is admitted", value: strings.Repeat("a", TokenMaximumBytes)},
		{name: "empty output is refused", wantErr: core.ErrGoogleIdentityContract}, {name: "leading padding is refused", value: "=abc", wantErr: core.ErrGoogleIdentityContract}, {name: "interior padding is refused", value: "ab=c", wantErr: core.ErrGoogleIdentityContract}, {name: "space is refused", value: "ab c", wantErr: core.ErrGoogleIdentityContract}, {name: "tab is refused", value: "ab\tc", wantErr: core.ErrGoogleIdentityContract}, {name: "interior newline is refused", value: "ab\nc", wantErr: core.ErrGoogleIdentityContract}, {name: "bare carriage return is refused", value: "abc\r", wantErr: core.ErrGoogleIdentityContract}, {name: "double line feed is refused", value: "abc\n\n", wantErr: core.ErrGoogleIdentityContract}, {name: "leading line feed is refused", value: "\nabc", wantErr: core.ErrGoogleIdentityContract}, {name: "trailing space is refused", value: "abc ", wantErr: core.ErrGoogleIdentityContract},
		{name: "comma is refused", value: "ab,c", wantErr: core.ErrGoogleIdentityContract}, {name: "non ASCII is refused", value: "ab界c", wantErr: core.ErrGoogleIdentityContract}, {name: "token one above maximum is refused", value: strings.Repeat("a", TokenMaximumBytes+1), wantErr: core.ErrGoogleIdentityContract}, {name: "output above framing maximum is refused", value: strings.Repeat("a", GoogleCloudCommandOutputMaximumBytes+1), wantErr: core.ErrGoogleIdentityContract},
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
				_, _ = w.Write([]byte(tc.body))
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
				if err != nil || bearerErr != nil || bearer != "Bearer "+googleTestToken {
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

	var calls atomic.Uint64
	var proxyCalls atomic.Uint64
	client := googleTestClientWithProxy(t, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		writer.Header().Set(googleMetadataHeaderName, googleMetadataHeaderValue)
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(googleTestToken))
	}), func(*http.Request) (*url.URL, error) {
		proxyCalls.Add(1)
		return nil, errors.New("configured proxy must not observe metadata acquisition")
	})
	got, gotErr := AcquireGoogleCloud(t.Context(), client, IdentityTokenRequest{Audience: mustGoogleAudience(t), Policy: mustGooglePolicy(t)})
	bearer, bearerErr := got.BearerValue()
	if gotErr != nil || bearerErr != nil || bearer != "Bearer "+googleTestToken || calls.Load() != 1 || proxyCalls.Load() != 0 {
		t.Fatalf("AcquireGoogleCloud(no proxy) = (bearer %q, errors %v/%v, metadata calls %d, proxy calls %d), want exact bearer, nil/nil, 1, 0", bearer, gotErr, bearerErr, calls.Load(), proxyCalls.Load())
	}
}

func TestLayerTriadGoogleAccessTokenRealHTTP(t *testing.T) {
	t.Parallel()
	valid, _ := json.Marshal(googleAccessTokenResponse{AccessToken: "abc", TokenType: googleAccessTokenTypeBearer, ExpiresIn: 300})
	for _, tc := range []struct {
		wantErr   error
		name      string
		flavor    string
		body      []byte
		wantCalls uint64
		cancel    bool
	}{
		{name: "provider receipt with response flavor produces bounded access bearer", body: valid, flavor: googleMetadataHeaderValue, wantCalls: 1},
		{name: "missing response flavor refuses an otherwise valid access bearer", body: valid, wantErr: core.ErrGoogleIdentityContract, wantCalls: 1},
		{name: "unknown response member is refused", body: []byte(`{"access_token":"abc","token_type":"Bearer","expires_in":300,"unknown":true}`), flavor: googleMetadataHeaderValue, wantErr: core.ErrGoogleIdentityContract, wantCalls: 1},
		{name: "cancelled intent performs no effect", body: valid, flavor: googleMetadataHeaderValue, cancel: true, wantErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Uint64
			client := googleTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if tc.flavor != "" {
					w.Header().Set(googleMetadataHeaderName, tc.flavor)
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(tc.body)
			}))
			ctx := t.Context()
			if tc.cancel {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			got, err := AcquireGoogleCloudAccessToken(ctx, client, GoogleCloudAccessTokenRequest{Policy: mustGooglePolicy(t)})
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || got != (AccessToken{}) {
					t.Fatalf("AcquireGoogleCloudAccessToken() = (%v, %v), want zero and %v", got, err, tc.wantErr)
				}
			} else {
				bearer, bearerErr := got.BearerValue()
				if err != nil || bearerErr != nil || bearer != "Bearer abc" || got.Lifetime().IsZero() {
					t.Fatalf("access token closure = (%q, %v, %v), want bounded token", bearer, err, bearerErr)
				}
			}
			if calls.Load() != tc.wantCalls {
				t.Fatalf("metadata calls = %d, want %d", calls.Load(), tc.wantCalls)
			}
		})
	}
}

type googleExternalDoorInventory struct {
	AcquireGoogleCloud            func(context.Context, Client, IdentityTokenRequest) (Token, error)
	AcquireGoogleCloudAccessToken func(context.Context, Client, GoogleCloudAccessTokenRequest) (AccessToken, error)
	NewGoogleCloudVerifier        func(context.Context, GoogleCloudVerifierConfiguration) (GoogleCloudVerifier, error)
	ParseAudience                 func(string) (Audience, error)
	ParseGoogleCloudCommandOutput func([]byte) (Token, error)
}

var googleExternalDoors = googleExternalDoorInventory{AcquireGoogleCloud: AcquireGoogleCloud, AcquireGoogleCloudAccessToken: AcquireGoogleCloudAccessToken, NewGoogleCloudVerifier: NewGoogleCloudVerifier, ParseAudience: ParseAudience, ParseGoogleCloudCommandOutput: ParseGoogleCloudCommandOutput}

func TestGoogleExternalDoorInventoryMatchesProduction(t *testing.T) {
	t.Parallel()
	got, err := scanGoogleExternalDoors(".")
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
func scanGoogleExternalDoors(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	set := token.NewFileSet()
	var doors []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(set, filepath.Join(root, entry.Name()), nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return nil, parseErr
		}
		ast.Inspect(file, func(node ast.Node) bool {
			declaration, ok := node.(*ast.FuncDecl)
			if !ok || declaration.Recv != nil {
				return true
			}
			name := declaration.Name.Name
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
	canonical, _ := json.Marshal(googleAccessTokenResponse{AccessToken: "abc", TokenType: googleAccessTokenTypeBearer, ExpiresIn: 300})
	for _, seed := range [][]byte{canonical, nil, {}, []byte(`{}`), []byte(`null`), []byte(`{"access_token":"a","access_token":"b","expires_in":1,"token_type":"Bearer"}`), bytes.Repeat([]byte{' '}, GoogleCloudAccessTokenResponseMaximumBytes+1)} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := decodeGoogleAccessTokenResponse(data)
		if err != nil {
			if !errors.Is(err, core.ErrGoogleIdentityContract) || got != (googleAccessTokenResponse{}) {
				t.Fatalf("decodeGoogleAccessTokenResponse(rejected) = (%+v, %v), want zero and %v", got, err, core.ErrGoogleIdentityContract)
			}
			return
		}
		if got.Validate() != nil {
			t.Fatalf("decodeGoogleAccessTokenResponse(accepted).Validate() error = %v, want nil", got.Validate())
		}
		encoded, marshalErr := json.Marshal(got)
		roundTrip, roundTripErr := decodeGoogleAccessTokenResponse(encoded)
		if marshalErr != nil || roundTripErr != nil || roundTrip != got {
			t.Fatalf("access response closure = (%+v, %v, %v), want (%+v, nil, nil)", roundTrip, marshalErr, roundTripErr, got)
		}
	})
}

func FuzzParseGoogleCloudCommandOutputSemanticClosure(f *testing.F) {
	for _, seed := range [][]byte{nil, {}, []byte("a"), []byte(googleTestToken), []byte(googleTestToken + "\n"), []byte("=bad"), bytes.Repeat([]byte{'a'}, TokenMaximumBytes+1)} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := googleExternalDoors.ParseGoogleCloudCommandOutput(data)
		if err != nil {
			if !errors.Is(err, core.ErrGoogleIdentityContract) || got != (Token{}) {
				t.Fatalf("ParseGoogleCloudCommandOutput(rejected) = (%v, %v), want zero and %v", got, err, core.ErrGoogleIdentityContract)
			}
			return
		}
		bearer, bearerErr := got.BearerValue()
		if got.Validate() != nil || bearerErr != nil || !strings.HasPrefix(bearer, "Bearer ") {
			t.Fatalf("ParseGoogleCloudCommandOutput(accepted) = (%v, %v), want validated bearer", got, bearerErr)
		}
	})
}
