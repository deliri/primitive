package googleidentity

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"golang.org/x/oauth2"
)

type browserClaimsFixture struct {
	Issuer   string  `json:"iss"`
	Audience string  `json:"aud"`
	Subject  string  `json:"sub"`
	IssuedAt int64   `json:"iat"`
	Expires  int64   `json:"exp"`
	Email    *string `json:"email,omitempty"`
	Verified *bool   `json:"email_verified,omitempty"`
	Nonce    *string `json:"nonce,omitempty"`
	Party    *string `json:"azp,omitempty"`
	Domain   *string `json:"hd,omitempty"`
}
type browserTokenFixture struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	IDToken     string `json:"id_token"`
	Expires     int64  `json:"expires_in"`
}

func browserString(value string) *string { return &value }
func browserBool(value bool) *bool       { return &value }
func browserClaimsSeed() browserClaimsFixture {
	return browserClaimsFixture{Issuer: googleCloudIdentityIssuer, Audience: "browser-client", Subject: "stable-google-subject", IssuedAt: 946684799, Expires: 946685100, Email: browserString("athlete@gmail.com"), Verified: browserBool(true), Nonce: browserString("owned-nonce")}
}
func signBrowserFixture(t testing.TB, claims browserClaimsFixture, foreign bool) string {
	t.Helper()
	keyPath := "testdata/verifier_rsa.pem"
	if foreign {
		keyPath = "testdata/verifier_foreign_rsa.pem"
	}
	key := verifierTestKey(t, keyPath)
	header, err := core.MarshalCanonicalJSONDocument(verifierTestHeader{Algorithm: verifierTestAlgorithm, KeyID: verifierTestKeyID})
	if err != nil {
		t.Fatalf("browser header encode = %v, want nil", err)
	}
	body, err := core.MarshalCanonicalJSONDocument(claims)
	if err != nil {
		t.Fatalf("browser claims encode = %v, want nil", err)
	}
	input := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	digest := sha256.Sum256([]byte(input))
	signature, err := rsa.SignPKCS1v15(nil, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("browser signature = %v, want nil", err)
	}
	return input + "." + base64.RawURLEncoding.EncodeToString(signature)
}
func browserRequestFixture(t testing.TB, client exchange.Client) BrowserAuthorizationCompleteRequest {
	t.Helper()
	id, e1 := core.ParseOAuthClientID("browser-client")
	secret, e2 := core.NewOAuthClientSecret([]byte("public-test-only-client-secret"))
	redirect, e3 := core.ParseOAuthRedirectURI("https://api.app.example/v1/auth/oauth/google/callback")
	code, e4 := core.ParseOAuthAuthorizationCode("public-test-code")
	verifier, e5 := core.ParseOAuthPKCEVerifier(strings.Repeat("v", 43))
	nonce, e6 := core.ParseOAuthNonce("owned-nonce")
	policy, e7 := DefaultPolicy()
	if err := errors.Join(e1, e2, e3, e4, e5, e6, e7); err != nil {
		t.Fatalf("browser fixture contracts = %v, want nil", err)
	}
	t.Cleanup(func() {
		if err := secret.Close(); err != nil {
			t.Errorf("browser secret Close = %v, want nil", err)
		}
	})
	identityVerifier, verifierErr := NewBrowserVerifier(context.Background(), BrowserVerifierConfiguration{Client: client, ClientID: id})
	if verifierErr != nil {
		t.Fatalf("browser verifier constructor = %v, want nil", verifierErr)
	}
	return BrowserAuthorizationCompleteRequest{IdentityVerifier: identityVerifier, Client: client, ClientID: id, ClientSecret: secret, RedirectURI: redirect, Code: code, Verifier: verifier, Nonce: nonce, Policy: policy.exchange()}
}
func browserLocalClient(t testing.TB, token string, status int, calls *atomic.Uint32) exchange.Client {
	t.Helper()
	key := verifierTestKey(t, "testdata/verifier_rsa.pem")
	jwk := verifierTestJWK{Algorithm: verifierTestAlgorithm, KeyID: verifierTestKeyID, KeyType: "RSA", Use: "sig", Exponent: base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()), Modulus: base64.RawURLEncoding.EncodeToString(key.N.Bytes())}
	keys, err := core.MarshalCanonicalJSONDocument(verifierTestKeySet{Keys: []verifierTestJWK{jwk}})
	if err != nil {
		t.Fatalf("browser JWK encode = %v, want nil", err)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/token" {
			if err := r.ParseForm(); err != nil {
				t.Errorf("token form parse = %v, want nil", err)
			}
			if r.Method != "POST" || r.Host != "oauth2.googleapis.com" || r.PostForm.Get("client_id") != "browser-client" || r.PostForm.Get("client_secret") != "public-test-only-client-secret" || r.PostForm.Get("code") != "public-test-code" || r.PostForm.Get("redirect_uri") != "https://api.app.example/v1/auth/oauth/google/callback" || r.PostForm.Get("code_verifier") != strings.Repeat("v", 43) || r.PostForm.Get("grant_type") != "authorization_code" {
				t.Errorf("token exchange binding = %s/%s, want exact typed request", r.Method, r.URL.Path)
			}
			w.WriteHeader(status)
			body, err := core.MarshalCanonicalJSONDocument(browserTokenFixture{AccessToken: "public-test-bearer", TokenType: "Bearer", IDToken: token, Expires: 300})
			if err != nil {
				t.Errorf("token fixture encode = %v, want nil", err)
				return
			}
			if _, err := w.Write(body); err != nil {
				t.Errorf("token fixture write = %v, want nil", err)
			}
			return
		}
		if r.Method != "GET" {
			t.Errorf("cert method = %s, want GET", r.Method)
		}
		w.Header().Set("Cache-Control", "public,max-age=3600")
		if _, err := w.Write(keys); err != nil {
			t.Errorf("cert fixture write = %v, want nil", err)
		}
	}))
	t.Cleanup(server.Close)
	transport, ok := server.Client().Transport.(*http.Transport)
	if !ok {
		t.Fatalf("browser local TLS transport = %T, want native transport", server.Client().Transport)
	}
	owned := transport.Clone()
	owned.Proxy = nil
	owned.TLSClientConfig.ServerName = server.Certificate().DNSNames[0]
	owned.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		var dialer net.Dialer
		return dialer.DialContext(ctx, network, server.Listener.Addr().String())
	}
	t.Cleanup(owned.CloseIdleConnections)
	client, err := exchange.NewClient(&http.Client{Transport: owned})
	if err != nil {
		t.Fatalf("browser exchange client = %v, want nil", err)
	}
	return client
}

func TestBrowserOAuthLocalProviderLayerTriad(t *testing.T) {
	t.Parallel()
	rows := []struct {
		name      string
		mutate    func(*browserClaimsFixture)
		wantErr   error
		foreign   bool
		noToken   bool
		cancel    bool
		authority bool
		status    int
	}{
		{name: "positive/Gmail provider authority", authority: true},
		{name: "positive/canonical principal from issuer alias", mutate: func(c *browserClaimsFixture) { c.Issuer = "accounts.google.com" }, authority: true},
		{name: "positive/Workspace authority derives signed hosted domain", mutate: func(c *browserClaimsFixture) {
			c.Email = browserString("athlete@team.example")
			c.Domain = browserString("team.example")
		}, authority: true},
		{name: "neutral/third party verified email is not Google authority", mutate: func(c *browserClaimsFixture) { c.Email = browserString("athlete@elsewhere.example") }},
		{name: "neutral/unverified email still authenticates bound subject", mutate: func(c *browserClaimsFixture) { c.Verified = browserBool(false) }},
		{name: "neutral/absent email still authenticates bound subject", mutate: func(c *browserClaimsFixture) { c.Email = nil; c.Verified = nil }},
		{name: "positive/matching authorized party", mutate: func(c *browserClaimsFixture) { c.Party = browserString("browser-client") }, authority: true},
		{name: "negative/foreign audience cannot become principal", mutate: func(c *browserClaimsFixture) { c.Audience = "another-client" }, wantErr: core.ErrGoogleIdentityContract},
		{name: "negative/foreign issuer despite valid Google key", mutate: func(c *browserClaimsFixture) { c.Issuer = "https://issuer.example" }, wantErr: core.ErrGoogleIdentityContract},
		{name: "negative/foreign authorized party", mutate: func(c *browserClaimsFixture) { c.Party = browserString("another-client") }, wantErr: core.ErrGoogleIdentityContract},
		{name: "negative/foreign nonce", mutate: func(c *browserClaimsFixture) { c.Nonce = browserString("another-nonce") }, wantErr: core.ErrGoogleIdentityContract},
		{name: "negative/absent nonce", mutate: func(c *browserClaimsFixture) { c.Nonce = nil }, wantErr: core.ErrGoogleIdentityContract},
		{name: "negative/foreign signer", foreign: true, wantErr: core.ErrGoogleIdentityContract},
		{name: "negative/no ID token", noToken: true, wantErr: core.ErrGoogleIdentityContract},
		{name: "negative/expired token", mutate: func(c *browserClaimsFixture) { c.Expires = 946684799 }, wantErr: core.ErrGoogleIdentityContract},
		{name: "boundary/exact expiry is no live identity", mutate: func(c *browserClaimsFixture) { c.Expires = 946684800 }, wantErr: core.ErrGoogleIdentityContract},
		{name: "boundary/issued exactly now", mutate: func(c *browserClaimsFixture) { c.IssuedAt = 946684800 }, authority: true},
		{name: "boundary/issued one second in future", mutate: func(c *browserClaimsFixture) { c.IssuedAt = 946684801 }, wantErr: core.ErrGoogleIdentityContract},
		{name: "negative/reversed signed interval", mutate: func(c *browserClaimsFixture) { c.IssuedAt = c.Expires + 1 }, wantErr: core.ErrGoogleIdentityContract},
		{name: "negative/verified claim without email", mutate: func(c *browserClaimsFixture) { c.Email = nil }, wantErr: core.ErrGoogleIdentityContract},
		{name: "negative/malformed email cannot escape", mutate: func(c *browserClaimsFixture) { c.Email = browserString("not-an-address") }, wantErr: core.ErrGoogleIdentityContract},
		{name: "negative/malformed hosted domain", mutate: func(c *browserClaimsFixture) { c.Domain = browserString("https://team.example") }, wantErr: core.ErrGoogleIdentityContract},
		{name: "negative/provider refuses code", status: 401, wantErr: core.ErrGoogleIdentityContract},
		{name: "neutral/canceled caller starts no exchange", cancel: true, wantErr: context.Canceled},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				claims := browserClaimsSeed()
				if row.mutate != nil {
					row.mutate(&claims)
				}
				token := signBrowserFixture(t, claims, row.foreign)
				if row.noToken {
					token = ""
				}
				var calls atomic.Uint32
				status := row.status
				if status == 0 {
					status = 200
				}
				client := browserLocalClient(t, token, status, &calls)
				request := browserRequestFixture(t, client)
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				if row.cancel {
					cancel()
				}
				got, err := CompleteBrowserAuthorization(ctx, request)
				if !errors.Is(err, row.wantErr) {
					t.Fatalf("browser provider result = (%+v,%v), want error %v", got, err, row.wantErr)
				}
				if row.wantErr != nil {
					if got != (BrowserVerifiedIdentity{}) {
						t.Fatalf("refused browser identity = %+v, want zero", got)
					}
					if row.cancel && calls.Load() != 0 {
						t.Fatalf("canceled provider calls = %d, want0", calls.Load())
					}
					return
				}
				if got.Subject.String() != claims.Subject || got.Issuer.String() != googleCloudIdentityIssuer || got.AuthoritativeEmail() != row.authority || calls.Load() != 2 {
					t.Fatalf("browser identity = %+v, authority=%t calls=%d, want exact stable principal/authority and2actual exchanges", got, got.AuthoritativeEmail(), calls.Load())
				}
				if err := got.Validate(); err != nil {
					t.Fatalf("browser identity Validate = %v, want nil", err)
				}
			})
		})
	}
}

func TestBrowserOAuthMalformedES256IsARefusal(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		defer func() {
			if got := recover(); got != nil {
				t.Errorf("malformed provider signature panic = %v, want typed refusal", got)
			}
		}()
		header, err := core.MarshalCanonicalJSONDocument(verifierTestHeader{Algorithm: "ES256", KeyID: verifierTestKeyID})
		if err != nil {
			t.Fatal(err)
		}
		body, err := core.MarshalCanonicalJSONDocument(browserClaimsSeed())
		if err != nil {
			t.Fatal(err)
		}
		raw := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body) + "." + base64.RawURLEncoding.EncodeToString([]byte{1})
		var calls atomic.Uint32
		client := browserLocalClient(t, raw, 200, &calls)
		got, err := CompleteBrowserAuthorization(t.Context(), browserRequestFixture(t, client))
		if !errors.Is(err, core.ErrGoogleIdentityContract) || got != (BrowserVerifiedIdentity{}) || calls.Load() != 1 {
			t.Fatalf("malformed provider signature = (%+v,%v,%drequests), want zero, typed refusal and no JWKS fetch", got, err, calls.Load())
		}
	})
}
func TestBrowserOAuthTokenRefusalCannotReplayClientSecrets(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		var calls atomic.Uint32
		client := browserLocalClient(t, "", 401, &calls)
		got, err := CompleteBrowserAuthorization(t.Context(), browserRequestFixture(t, client))
		var sdk *oauth2.RetrieveError
		if !errors.As(err, &sdk) || !errors.Is(err, core.ErrGoogleIdentityContract) || got != (BrowserVerifiedIdentity{}) {
			t.Fatalf("SDK refusal = (%+v,%v), want zero and retained native OAuth2 cause", got, err)
		}
		if sdk.Response == nil || sdk.Response.StatusCode != 401 || sdk.Response.Request != nil || len(sdk.Body) != 0 || sdk.ErrorDescription != "" || sdk.ErrorURI != "" {
			t.Fatalf("SDK refusal retains sensitive request/body: request=%t bodybytes=%d, want only safe status/class", sdk.Response != nil && sdk.Response.Request != nil, len(sdk.Body))
		}
	})
}
