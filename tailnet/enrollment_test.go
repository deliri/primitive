package tailnet

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/googleidentity"
	tailscale "tailscale.com/client/tailscale/v2"
)

const (
	fixtureAuthKey       = authKeyPrefix + "fixture-public-key"
	fixtureAccessToken   = "fixture-public-access"
	fixtureClientIDField = "client_id"
	fixtureJWTField      = "jwt"
)

type fixtureIdentityClaims struct {
	Exp int64 `json:"exp"`
}
type fixtureAccessResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// Local provider transport retains the SDK's request construction and real
// net/http framing, but routes all traffic to this test's owned HTTP server.
type localEnrollmentTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (p localEnrollmentTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	copy := request.Clone(request.Context())
	copy.URL.Scheme = p.target.Scheme
	copy.URL.Host = p.target.Host
	return p.base.RoundTrip(copy)
}

func fixtureIdentityToken(t testing.TB) googleidentity.Token {
	t.Helper()
	claims, err := core.MarshalCanonicalJSONDocument(fixtureIdentityClaims{Exp: 4102444800})
	if err != nil {
		t.Fatalf("claims encoding = %v, want nil", err)
	}
	// Acquisition admits opaque token68. SDK-only fixture: no claim of Google signature verification.
	token, err := googleidentity.ParseGoogleCloudCommandOutput([]byte("e30." + base64.RawURLEncoding.EncodeToString(claims) + ".cHVibGlj"))
	if err != nil {
		t.Fatalf("token acquisition = %v, want nil", err)
	}
	return token
}

func localEnrollmentClient(t testing.TB, handler http.Handler) exchange.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("local provider URL = %v, want nil", err)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	t.Cleanup(transport.CloseIdleConnections)
	client, err := exchange.NewClient(&http.Client{Transport: localEnrollmentTransport{target: target, base: transport}})
	if err != nil {
		t.Fatalf("NewClient(provider) = %v, want nil", err)
	}
	return client
}

func TestAuthKeySDKLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		tokenStatus  int
		tokenBody    []byte
		editKey      func(*tailscale.Key)
		keyStatus    int
		keyBody      []byte
		cancelBefore bool
		cancelToken  bool
		wantErr      error
		wantCause    error
		wantCalls    int32
	}{
		{name: "ephemeral single-use key preserves exact provider key", wantCalls: 2},
		{name: "credential refusal never requests a key", tokenStatus: http.StatusUnauthorized, wantErr: core.ErrTailnetEnrollment, wantCalls: 1},
		{name: "malformed access response cannot enroll", tokenBody: []byte("{broken"), wantErr: core.ErrTailnetEnrollment, wantCalls: 1},
		{name: "absent access token cannot create auth key", tokenBody: []byte("{}"), wantErr: core.ErrTailnetEnrollment, wantCalls: 1},
		{name: "oversized access response stops at Exchange ceiling", tokenBody: []byte(strings.Repeat(" ", enrollmentResponseMaximumBytes+1)), wantErr: core.ErrTailnetEnrollment, wantCause: core.ErrExchangeBodyLimit, wantCalls: 1},
		{name: "key refusal cannot expose partial credential", keyStatus: http.StatusForbidden, wantErr: core.ErrTailnetEnrollment, wantCalls: 2},
		{name: "malformed key response cannot enroll", keyBody: []byte("{broken"), wantErr: core.ErrTailnetEnrollment, wantCalls: 2},
		{name: "oversized key response stops at Exchange ceiling", keyBody: []byte(strings.Repeat(" ", enrollmentResponseMaximumBytes+1)), wantErr: core.ErrTailnetEnrollment, wantCause: core.ErrExchangeBodyLimit, wantCalls: 2},
		{name: "empty key cannot trigger ambient auth fallback", editKey: func(k *tailscale.Key) { k.Key = "" }, wantErr: core.ErrTailnetEnrollment, wantCalls: 2},
		{name: "key file directive cannot become filesystem access", editKey: func(k *tailscale.Key) { k.Key = "file:/secret" }, wantErr: core.ErrTailnetEnrollment, wantCalls: 2},
		{name: "key at byte ceiling remains exact", editKey: func(k *tailscale.Key) {
			k.Key = authKeyPrefix + strings.Repeat("a", authKeyMaximumBytes-len(authKeyPrefix))
		}, wantCalls: 2},
		{name: "key one below byte ceiling remains exact", editKey: func(k *tailscale.Key) {
			k.Key = authKeyPrefix + strings.Repeat("a", authKeyMaximumBytes-len(authKeyPrefix)-1)
		}, wantCalls: 2},
		{name: "key above byte ceiling cannot enroll", editKey: func(k *tailscale.Key) {
			k.Key = authKeyPrefix + strings.Repeat("a", authKeyMaximumBytes-len(authKeyPrefix)+1)
		}, wantErr: core.ErrTailnetEnrollment, wantCalls: 2},
		{name: "prefix alone is not key material", editKey: func(k *tailscale.Key) { k.Key = authKeyPrefix }, wantErr: core.ErrTailnetEnrollment, wantCalls: 2},
		{name: "control byte in credential refused", editKey: func(k *tailscale.Key) { k.Key = fixtureAuthKey + "\n" }, wantErr: core.ErrTailnetEnrollment, wantCalls: 2},
		{name: "invalid provider key refused", editKey: func(k *tailscale.Key) { k.Invalid = true }, wantErr: core.ErrTailnetEnrollment, wantCalls: 2},
		{name: "reusable key contradicts requested single use", editKey: func(k *tailscale.Key) { k.Capabilities.Devices.Create.Reusable = true }, wantErr: core.ErrTailnetEnrollment, wantCalls: 2},
		{name: "persistent key contradicts ephemeral request", editKey: func(k *tailscale.Key) { k.Capabilities.Devices.Create.Ephemeral = false }, wantErr: core.ErrTailnetEnrollment, wantCalls: 2},
		{name: "preauthorized key cannot widen approval", editKey: func(k *tailscale.Key) { k.Capabilities.Devices.Create.Preauthorized = true }, wantErr: core.ErrTailnetEnrollment, wantCalls: 2},
		{name: "foreign tag cannot replace authored authority", editKey: func(k *tailscale.Key) { k.Capabilities.Devices.Create.Tags = []string{"tag:foreign"} }, wantErr: core.ErrTailnetEnrollment, wantCalls: 2},
		{name: "absent tag cannot remove authority binding", editKey: func(k *tailscale.Key) { k.Capabilities.Devices.Create.Tags = nil }, wantErr: core.ErrTailnetEnrollment, wantCalls: 2},
		{name: "extra tag cannot widen authority", editKey: func(k *tailscale.Key) {
			k.Capabilities.Devices.Create.Tags = append(k.Capabilities.Devices.Create.Tags, "tag:foreign")
		}, wantErr: core.ErrTailnetEnrollment, wantCalls: 2},
		{name: "pre-cancelled enrollment performs no HTTP", cancelBefore: true, wantErr: core.ErrTailnetEnrollment, wantCause: context.Canceled, wantCalls: 0},
		{name: "cancellation interrupts SDK background token request", cancelToken: true, wantErr: core.ErrTailnetEnrollment, wantCause: context.Canceled, wantCalls: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			configuration := fixtureConfiguration(t)
			token := fixtureIdentityToken(t)
			bearer, err := token.BearerValue()
			if err != nil {
				t.Fatalf("BearerValue() = %v, want nil", err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			key := tailscale.Key{Key: fixtureAuthKey}
			key.Capabilities.Devices.Create.Ephemeral = true
			key.Capabilities.Devices.Create.Tags = []string{configuration.Tag.String()}
			if tc.editKey != nil {
				tc.editKey(&key)
			}
			var calls atomic.Int32
			client := localEnrollmentClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != http.MethodPost {
					t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
				}
				status := http.StatusOK
				var data []byte
				var encodeErr error
				switch r.URL.Path {
				case tokenExchangePath:
					if err := r.ParseForm(); err != nil {
						t.Errorf("ParseForm() = %v, want nil", err)
					}
					if r.Form.Get(fixtureClientIDField) != configuration.ClientID.String() || r.Form.Get(fixtureJWTField) != strings.TrimPrefix(bearer, exchange.BearerAuthorizationScheme+" ") {
						t.Errorf("federation request binding = client=%q, tokenMatches=%t, want exact configured identity", r.Form.Get(fixtureClientIDField), r.Form.Get(fixtureJWTField) == strings.TrimPrefix(bearer, exchange.BearerAuthorizationScheme+" "))
					}
					if tc.cancelToken {
						cancel()
						<-r.Context().Done()
						return
					}
					data, encodeErr = core.MarshalCanonicalJSONDocument(fixtureAccessResponse{AccessToken: fixtureAccessToken, TokenType: exchange.BearerAuthorizationScheme, ExpiresIn: 60})
					if tc.tokenBody != nil {
						data = tc.tokenBody
					}
					if tc.tokenStatus != 0 {
						status = tc.tokenStatus
					}
				case createKeyPath:
					body, readErr := io.ReadAll(io.LimitReader(r.Body, enrollmentResponseMaximumBytes+1))
					if readErr != nil {
						t.Errorf("request read = %v, want nil", readErr)
					}
					request, err := core.DecodeStrictJSONStructure[tailscale.CreateKeyRequest](body, core.DefaultStrictJSONLimits())
					if err != nil || request.Capabilities.Devices.Create.Reusable || !request.Capabilities.Devices.Create.Ephemeral || request.Capabilities.Devices.Create.Preauthorized || len(request.Capabilities.Devices.Create.Tags) != 1 {
						t.Errorf("key request = (%+v,%v), want one ephemeral single-use tag", request, err)
					} else if request.Capabilities.Devices.Create.Tags[0] != configuration.Tag.String() {
						t.Errorf("key tag = %q, want %q", request.Capabilities.Devices.Create.Tags[0], configuration.Tag.String())
					}
					if r.Header.Get("Authorization") != exchange.BearerAuthorizationScheme+" "+fixtureAccessToken {
						t.Errorf("key authorization matches = %t, want true", r.Header.Get("Authorization") == exchange.BearerAuthorizationScheme+" "+fixtureAccessToken)
					}
					data, encodeErr = core.MarshalCanonicalJSONDocument(key)
					if tc.keyBody != nil {
						data = tc.keyBody
					}
					if tc.keyStatus != 0 {
						status = tc.keyStatus
					}
				default:
					t.Errorf("provider path = %q, want one of the two pinned SDK operations", r.URL.Path)
					status = http.StatusNotFound
				}
				if encodeErr != nil {
					t.Errorf("provider encoding = %v, want nil", encodeErr)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				if _, err := w.Write(data); err != nil && !tc.cancelToken {
					t.Errorf("provider Write() = %v, want nil", err)
				}
			}))
			if tc.cancelBefore {
				cancel()
			}
			got, err := acquireAuthKey(ctx, client, configuration, token)
			want := ""
			if tc.wantErr == nil {
				want = key.Key
			}
			if got != want || !errors.Is(err, tc.wantErr) || calls.Load() != tc.wantCalls {
				t.Fatalf("acquireAuthKey() = (keyMatches=%t,%v,%d calls), want exact key,%v,%d calls", got == want, err, calls.Load(), tc.wantErr, tc.wantCalls)
			}
			if tc.wantCause != nil && !errors.Is(err, tc.wantCause) {
				t.Fatalf("cause = %v, want %v", err, tc.wantCause)
			}
		})
	}
}
