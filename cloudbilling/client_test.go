package cloudbilling

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
	"github.com/deliri/primitive/v2026/testserial"
)

type serviceAccountFixture struct {
	Type        string `json:"type"`
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}
type tokenFixture struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

func TestADCDiscoveryAuthenticatedPageAndOwnedTransportClosure(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardProcessEnvironment, Scope: core.TestIsolationScopePackageProcess})
	var tokenCalls, catalogCalls atomic.Int64
	closed := make(chan struct{}, 2)
	body := encodedFixture(t, fixturePage())
	provider := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			tokenCalls.Add(1)
			if err := r.ParseForm(); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			if r.Method != "POST" || r.Form.Get("assertion") == "" || r.Form.Get("grant_type") != "urn:ietf:params:oauth:grant-type:jwt-bearer" {
				t.Errorf("token request method=%q/grant=%q, want actual SDK assertion exchange", r.Method, r.Form.Get("grant_type"))
				w.WriteHeader(400)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(tokenFixture{AccessToken: "fixture-access", TokenType: "Bearer", ExpiresIn: 3600}); err != nil {
				t.Error(err)
			}
		case "/v1/services/service/skus":
			catalogCalls.Add(1)
			if r.Header.Get("Authorization") != "Bearer fixture-access" || r.Header.Get(quotaProjectHeader) != "test-project" {
				t.Error("catalog request missing actual SDK bearer or selected quota project")
				w.WriteHeader(403)
				return
			}
			_, _ = w.Write(body)
		default:
			w.WriteHeader(404)
		}
	}))
	provider.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateClosed {
			select {
			case closed <- struct{}{}:
			default:
			}
		}
	}
	provider.Start()
	defer provider.Close()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	private := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	credentialBytes, err := core.MarshalCanonicalJSONDocument(serviceAccountFixture{Type: "service_account", ClientEmail: "fixture@test-project.iam.gserviceaccount.com", PrivateKey: string(private), TokenURI: provider.URL + "/token"})
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	credentialPath := filepath.Join(directory, "adc.json")
	if err := os.WriteFile(credentialPath, credentialBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", credentialPath)
	quota, err := ParseQuotaProject("test-project")
	if err != nil {
		t.Fatal(err)
	}
	timeout, err := temporal.DurationFromSeconds(5)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewADCClient(t.Context(), ADCClientRequest{QuotaProject: quota, CredentialTimeout: timeout})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if client.transport != nil {
			_ = client.Close()
		}
	})
	server, err := client.Server(fixtureOptions(t, provider.URL+"/v1/services/service/skus"))
	if err != nil {
		t.Fatal(err)
	}
	var proof bytes.Buffer
	_, err = server.ListSKUs(t.Context(), ListSKUsRequest{Proof: &proof}, exchange.StreamPolicy{Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(proof.Bytes(), append(append([]byte{}, body...), '\n')) {
		t.Fatal("authenticated SDK path did not retain the exact provider source")
	}
	if tokenCalls.Load() != 1 || catalogCalls.Load() != 1 {
		t.Fatalf("token/catalog requests=(%d,%d), want (1,1)", tokenCalls.Load(), catalogCalls.Load())
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("owned transport retained idle native connection after Close")
	}
	if _, err := client.Server(fixtureOptions(t, provider.URL+"/v1/services/service/skus")); !errors.Is(err, core.ErrCloudBillingContract) {
		t.Fatalf("closed client Server() error=%v, want contract refusal", err)
	}
}

func TestADCAdmissionRefusesBeforeDiscovery(t *testing.T) {
	t.Parallel()
	quota, err := ParseQuotaProject("test-project")
	if err != nil {
		t.Fatal(err)
	}
	timeout, err := temporal.DurationFromSeconds(5)
	if err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	for _, tc := range []struct {
		name    string
		ctx     context.Context
		request ADCClientRequest
		wantErr error
	}{
		{name: "cancellation before credentials", ctx: canceled, request: ADCClientRequest{QuotaProject: quota, CredentialTimeout: timeout}, wantErr: context.Canceled},
		{name: "nil context before credentials", request: ADCClientRequest{QuotaProject: quota, CredentialTimeout: timeout}, wantErr: core.ErrCloudBillingContract},
		{name: "zero quota before credentials", ctx: t.Context(), request: ADCClientRequest{CredentialTimeout: timeout}, wantErr: core.ErrCloudBillingContract},
		{name: "zero timeout before credentials", ctx: t.Context(), request: ADCClientRequest{QuotaProject: quota}, wantErr: core.ErrCloudBillingContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := NewADCClient(tc.ctx, tc.request)
			if got != nil || !errors.Is(err, tc.wantErr) {
				t.Fatalf("NewADCClient()=(%v,%v), want nil and %v", got, err, tc.wantErr)
			}
		})
	}
}

func TestADCDiscoveryFailurePreservesCause(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardProcessEnvironment, Scope: core.TestIsolationScopePackageProcess})
	path := filepath.Join(t.TempDir(), "missing-adc.json")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", path)
	quota, err := ParseQuotaProject("test-project")
	if err != nil {
		t.Fatal(err)
	}
	timeout, err := temporal.DurationFromSeconds(5)
	if err != nil {
		t.Fatal(err)
	}
	got, err := NewADCClient(t.Context(), ADCClientRequest{QuotaProject: quota, CredentialTimeout: timeout})
	if got != nil || !errors.Is(err, core.ErrCloudBillingAuthentication) || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("NewADCClient(missing ADC)=(%v,%v), want nil/authentication/missing-file cause", got, err)
	}
}
