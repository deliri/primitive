package cloudbilling

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

type authorizedUserFixture struct {
	Type         string `json:"type"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RefreshToken string `json:"refresh_token"`
}

func TestAuthorizedUserFileRefreshAndAuthenticatedPage(t *testing.T) {
	t.Parallel()
	var tokenCalls, catalogCalls atomic.Int64
	body := encodedFixture(t, fixturePage())
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			tokenCalls.Add(1)
			if err := r.ParseForm(); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			if r.Method != "POST" || r.Host != "oauth2.googleapis.com" || r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != "fixture-refresh" || r.Form.Get("client_id") != "fixture-client" || r.Form.Get("client_secret") != "fixture-secret" {
				t.Error("authorized-user refresh did not use the actual SDK-owned endpoint and bound fixture credentials")
				w.WriteHeader(400)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(tokenFixture{AccessToken: "file-fixture-access", TokenType: "Bearer", ExpiresIn: 3600}); err != nil {
				t.Error(err)
			}
		case "/v1/services/service/skus":
			catalogCalls.Add(1)
			if r.Header.Get("Authorization") != "Bearer file-fixture-access" || r.Header.Get(quotaProjectHeader) != "test-project" {
				t.Error("bound-file request omitted its SDK bearer/quota facts")
				w.WriteHeader(403)
				return
			}
			_, _ = w.Write(body)
		default:
			w.WriteHeader(404)
		}
	}))
	defer provider.Close()
	base := provider.Client().Transport.(*http.Transport).Clone()
	defer base.CloseIdleConnections()
	// The fixture verifies its own real TLS certificate. Only the official
	// token host and this explicit provider endpoint can reach the loopback.
	base.TLSClientConfig = base.TLSClientConfig.Clone()
	base.TLSClientConfig.ServerName = "example.com"
	localAddress := provider.Listener.Addr().String()
	base.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != "oauth2.googleapis.com:443" && address != localAddress {
			return nil, core.ErrCloudBillingContract
		}
		var dialer net.Dialer
		return dialer.DialContext(ctx, network, localAddress)
	}
	data, err := core.MarshalCanonicalJSONDocument(authorizedUserFixture{Type: "authorized_user", ClientID: "fixture-client", ClientSecret: "fixture-secret", RefreshToken: "fixture-refresh"})
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	path := filepath.Join(directory, "authorized-user.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := core.ParseAbsolutePath(path)
	if err != nil {
		t.Fatal(err)
	}
	quota, err := ParseQuotaProject("test-project")
	if err != nil {
		t.Fatal(err)
	}
	timeout, err := temporal.DurationFromSeconds(5)
	if err != nil {
		t.Fatal(err)
	}
	// This is the same constructor body NewADCClient calls. Only the native
	// transport dependency differs, so the SDK's fixed OAuth URL stays intact.
	client, err := newADCClientWithTransport(t.Context(), ADCClientRequest{QuotaProject: quota, CredentialTimeout: timeout, Credentials: CredentialSource{Kind: CredentialAuthorizedUserFile, File: file}}, base)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	}()
	server, err := client.Server(fixtureOptions(t, provider.URL+"/v1/services/service/skus"))
	if err != nil {
		t.Fatal(err)
	}
	var proof bytes.Buffer
	got, err := server.ListSKUs(t.Context(), ListSKUsRequest{Proof: &proof}, exchange.StreamPolicy{Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject}})
	if err != nil {
		t.Fatal(err)
	}
	if tokenCalls.Load() != 1 || catalogCalls.Load() != 1 || got.Source.SHA256 != core.SHA256Of(body) || !bytes.Equal(proof.Bytes(), append(append([]byte{}, body...), '\n')) {
		t.Fatalf("file SDK path token/catalog calls=(%d,%d), want(1,1) and exact source receipt", tokenCalls.Load(), catalogCalls.Load())
	}
}

func TestCredentialSourceClosedPolicyAndCancellation(t *testing.T) {
	t.Parallel()
	path, err := core.ParseAbsolutePath(filepath.Join(t.TempDir(), "must-not-open.json"))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		source  CredentialSource
		wantErr bool
	}{
		{name: "ordinary explicit ADC", source: CredentialSource{Kind: CredentialApplicationDefault}},
		{name: "bound authorized user file", source: CredentialSource{Kind: CredentialAuthorizedUserFile, File: path}},
		{name: "zero kind refuses ambient fallback", wantErr: true},
		{name: "unknown kind refuses discovery", source: CredentialSource{Kind: CredentialSourceKind(255)}, wantErr: true},
		{name: "file kind requires a bound absolute path", source: CredentialSource{Kind: CredentialAuthorizedUserFile}, wantErr: true},
		{name: "ADC plus explicit file is contradictory", source: CredentialSource{Kind: CredentialApplicationDefault, File: path}, wantErr: true},
	}
	quota, err := ParseQuotaProject("test-project")
	if err != nil {
		t.Fatal(err)
	}
	timeout, err := temporal.DurationFromSeconds(5)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.source.Validate()
			if tc.wantErr {
				if !errors.Is(err, core.ErrCloudBillingContract) {
					t.Fatalf("source.Validate()=%v,want contract refusal", err)
				}
				client, err := NewADCClient(t.Context(), ADCClientRequest{QuotaProject: quota, CredentialTimeout: timeout, Credentials: tc.source})
				if client != nil || !errors.Is(err, core.ErrCloudBillingContract) || errors.Is(err, os.ErrNotExist) {
					t.Fatalf("invalid source setup=(%v,%v),want no credential read and contract refusal", client, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			client, err := NewADCClient(ctx, ADCClientRequest{QuotaProject: quota, CredentialTimeout: timeout, Credentials: tc.source})
			if client != nil || !errors.Is(err, context.Canceled) || errors.Is(err, os.ErrNotExist) {
				t.Fatalf("canceled source setup=(%v,%v),want cancellation before file/discovery", client, err)
			}
		})
	}
}

func TestCloudBillingErrorIdentityRetainsOwningContract(t *testing.T) {
	t.Parallel()
	for _, leaf := range []core.ErrorIdentity{core.ErrCloudBillingAuthentication, core.ErrCloudBillingResponse} {
		if !errors.Is(leaf, core.ErrCloudBillingContract) || !errors.Is(leaf, core.ErrPrimitiveContract) || errors.Is(leaf, core.ErrGoogleIdentityContract) {
			t.Fatalf("error ancestry for %v is not the CloudBilling→Primitive contract", leaf)
		}
	}
}
