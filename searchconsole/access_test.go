package searchconsole

import (
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/googleidentity"
)

const fixtureSigner = "search-reader@fixture-project.iam.gserviceaccount.com"
const fixtureSearchScope = "https://www.googleapis.com/auth/webmasters.readonly"

type assertionFixture struct {
	Issuer   string  `json:"iss"`
	Subject  *string `json:"sub,omitzero"`
	Audience string  `json:"aud"`
	Scope    string  `json:"scope"`
	Issued   int64   `json:"iat"`
	Expires  int64   `json:"exp"`
}

func TestSearchConsoleServiceAccountScopeBinding(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		changeClaims func(*assertionFixture)
		scope        string
		wantErr      error
		wantExchange int32
		wantRead     int32
	}{
		{name: "plain service account uses narrow search scope", scope: fixtureSearchScope, wantExchange: 1, wantRead: 1},
		{name: "IAM cannot introduce a delegated subject", scope: fixtureSearchScope, changeClaims: func(c *assertionFixture) { subject := "operator@example.invalid"; c.Subject = &subject }, wantErr: core.ErrGoogleDelegationBinding},
		{name: "IAM cannot introduce an explicitly empty subject", scope: fixtureSearchScope, changeClaims: func(c *assertionFixture) { subject := ""; c.Subject = &subject }, wantErr: core.ErrGoogleDelegationBinding},
		{name: "IAM cannot return a different signer", scope: fixtureSearchScope, changeClaims: func(c *assertionFixture) { c.Issuer = "other@fixture-project.iam.gserviceaccount.com" }, wantErr: core.ErrGoogleDelegationBinding},
		{name: "IAM cannot widen the assertion scope", scope: fixtureSearchScope, changeClaims: func(c *assertionFixture) { c.Scope = "https://www.googleapis.com/auth/webmasters" }, wantErr: core.ErrGoogleDelegationBinding},
		{name: "exchange may omit the unchanged scope", wantExchange: 1, wantRead: 1},
		{name: "exchanged bearer returned with a wider scope", scope: "https://www.googleapis.com/auth/webmasters", wantErr: core.ErrGoogleDelegationBinding, wantExchange: 1},
		{name: "exchanged bearer returned with extra Gmail scope", scope: fixtureSearchScope + " https://www.googleapis.com/auth/gmail.readonly", wantErr: core.ErrGoogleDelegationBinding, wantExchange: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var metadataCalls, iamCalls, tokenCalls, readCalls atomic.Int32
			metadata := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				metadataCalls.Add(1)
				if r.Method != http.MethodGet || r.Host != "metadata.google.internal" ||
					r.URL.Path != "/computeMetadata/v1/instance/service-accounts/default/token" || r.Header.Get("Metadata-Flavor") != "Google" {
					t.Errorf("metadata call = %s %s, want attached service-account token", r.Method, r.URL)
				}
				w.Header().Set("Metadata-Flavor", "Google")
				writeJSON(t, w, 200, []byte(`{"access_token":"runtime-fixture","token_type":"Bearer","expires_in":3600}`))
			}))
			t.Cleanup(metadata.Close)
			network := providerNetwork(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.Host {
				case "iamcredentials.googleapis.com":
					iamCalls.Add(1)
					if r.Method != http.MethodPost || r.URL.Path != "/v1/projects/-/serviceAccounts/"+fixtureSigner+":signJwt" ||
						r.Header.Get("Authorization") != "Bearer runtime-fixture" {
						t.Errorf("IAM request is %s %s, want authorized signJwt on the fixture reader", r.Method, r.URL)
					}
					var input struct {
						Payload string `json:"payload"`
					}
					var claims assertionFixture
					raw, err := io.ReadAll(io.LimitReader(r.Body, 16*1024))
					if err != nil || json.Unmarshal(raw, &input) != nil || json.Unmarshal([]byte(input.Payload), &claims) != nil {
						t.Errorf("IAM payload decode error = %v, want one typed claims document", err)
						w.WriteHeader(400)
						return
					}
					if claims.Issuer != fixtureSigner || claims.Scope != fixtureSearchScope || claims.Subject != nil || strings.Contains(input.Payload, `"sub"`) ||
						claims.Audience != "https://oauth2.googleapis.com/token" || claims.Issued <= 0 || claims.Expires-claims.Issued != 300 {
						t.Errorf("IAM claims = %+v, want five-minute exact read scope, reader issuer and no subject", claims)
					}
					if tc.changeClaims != nil {
						tc.changeClaims(&claims)
					}
					header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","kid":"fixture-key"}`))
					signed := fixtureJSON(t, claims)
					if claims.Subject != nil && !strings.Contains(string(signed), `"sub":`) {
						t.Error("the signed response fixture omitted its subject mutation")
					}
					payload := base64.RawURLEncoding.EncodeToString(signed)
					writeJSON(t, w, 200, fixtureJSON(t, struct {
						Key string `json:"keyId"`
						JWT string `json:"signedJwt"`
					}{Key: "fixture-key", JWT: header + "." + payload + ".Zml4dHVyZQ"}))
				case "oauth2.googleapis.com":
					tokenCalls.Add(1)
					if r.Method != http.MethodPost || r.URL.Path != "/token" || r.Header.Get("Authorization") != "" || r.ParseForm() != nil ||
						r.PostForm.Get("grant_type") != "urn:ietf:params:oauth:grant-type:jwt-bearer" || !strings.HasSuffix(r.PostForm.Get("assertion"), ".Zml4dHVyZQ") {
						t.Errorf("OAuth exchange target = %s %s, want form with the signed JWT", r.Method, r.URL)
					}
					writeJSON(t, w, 200, fixtureJSON(t, struct {
						Token  string `json:"access_token"`
						Type   string `json:"token_type"`
						Expiry int    `json:"expires_in"`
						Scope  string `json:"scope,omitempty"`
					}{Token: bearerFixture, Type: "Bearer", Expiry: 3600, Scope: tc.scope}))
				case "www.googleapis.com":
					readCalls.Add(1)
					if r.Header.Get("Authorization") != "Bearer "+bearerFixture {
						t.Error("Search Console token does not match OAuth fixture")
					}
					writeJSON(t, w, 200, fixtureJSON(t, siteWire{Property: propertyFixture, Permission: "siteFullUser"}))
				default:
					t.Errorf("unexpected fixture host %q", r.Host)
					w.WriteHeader(400)
				}
			}, metadata)
			signer, signErr := googleidentity.ParseServiceAccountEmail(fixtureSigner)
			policy, policyErr := googleidentity.DefaultPolicy()
			if err := errors.Join(signErr, policyErr); err != nil {
				t.Fatal(err)
			}
			source, err := NewServiceAccountSource(network, signer, policy)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = source.CloseIdleConnections() })
			c, err := NewClient(network, source)
			if err != nil {
				t.Fatal(err)
			}
			r := fixtureRequest(t)
			got, err := c.GetSite(t.Context(), SiteRequest{Property: r.Property, Policy: r.Policy})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("read error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil && got != (Site{}) || tc.wantErr == nil && got != (Site{Property: r.Property, Permission: PermissionFull}) {
				t.Fatalf("site = %+v, want exact full grant or zero on refusal", got)
			}
			if metadataCalls.Load() != 1 || iamCalls.Load() != 1 || tokenCalls.Load() != tc.wantExchange || readCalls.Load() != tc.wantRead {
				t.Fatalf("metadata/IAM/OAuth/Sites calls = %d/%d/%d/%d, want 1/1/%d/%d", metadataCalls.Load(), iamCalls.Load(), tokenCalls.Load(), readCalls.Load(), tc.wantExchange, tc.wantRead)
			}
		})
	}
}
