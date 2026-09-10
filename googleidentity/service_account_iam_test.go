package googleidentity

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"cloud.google.com/go/auth/credentials"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type googleIAMFixtureIntent struct {
	Audience     Audience `json:"audience"`
	IncludeEmail bool     `json:"includeEmail"`
	Delegates    []string `json:"delegates,omitempty"`
}
type googleIAMFixtureReceipt struct {
	Token string `json:"token"`
}

func TestGoogleServiceAccountIAMBoundaryLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		redirect bool
		mutate   func([]byte) []byte
		wantErr  error
	}{
		{name: "sdk_authenticates_exact_iam_intent"},
		{name: "large_iam_response_whitespace", mutate: func(b []byte) []byte { return append([]byte(strings.Repeat(" ", 128<<10)), b...) }},
		{name: "iam_redirect_cannot_escape_exchange", redirect: true, wantErr: core.ErrExchangeRedirect},
		{name: "duplicate_iam_token_cannot_replace_receipt", mutate: func(b []byte) []byte { return append(b[:len(b)-1], []byte(",\"token\":\"other\"}")...) }, wantErr: core.ErrJSONContract},
		{name: "absent_iam_token_cannot_invent_identity", mutate: func([]byte) []byte { return []byte("{}") }, wantErr: core.ErrGoogleIdentityContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			synctest.Test(t, func(t *testing.T) {
				signer := &verifierTestProvider{key: verifierTestKey(t, "testdata/verifier_rsa.pem")}
				signed := signer.sign(t, verifierTestHeader{Algorithm: verifierTestAlgorithm, KeyID: verifierTestKeyID}, verifierClaims(), false)
				response, err := core.MarshalCanonicalJSONDocument(googleIAMFixtureReceipt{Token: strings.TrimPrefix(signed, bearerPrefix)})
				if err != nil {
					t.Fatal(err)
				}
				if tc.mutate != nil {
					response = tc.mutate(response)
				}
				request := serviceAccountFixtureRequest(t)
				var calls atomic.Uint64
				server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls.Add(1)
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Errorf("read request: %v", err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					intent, err := core.DecodeStrictJSONStructure[googleIAMFixtureIntent](body, core.ExtensibleJSONLimits())
					if err != nil || intent.Audience != request.Audience || intent.IncludeEmail || len(intent.Delegates) != 0 || r.Method != http.MethodPost || r.Host != "iamcredentials."+googleTestUniverse || r.URL.Path != "/v1/projects/-/serviceAccounts/"+serviceAccountFixtureEmail+":generateIdToken" {
						t.Errorf("IAM request intent=%v method=%v host=%v path=%v error=%v, want exact typed intent", intent, r.Method, r.Host, r.URL.Path, err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					header, claims, err := decodeServiceAccountAssertion(strings.TrimPrefix(r.Header.Get("Authorization"), bearerPrefix), &signer.key.PublicKey)
					if err != nil || header.KeyID != serviceAccountFixtureKeyID || header.Algorithm != googleCloudSigningAlgorithmRS256 || claims.Issuer != serviceAccountFixtureEmail || claims.Subject != serviceAccountFixtureEmail || claims.Scope != googleIAMCredentialScope || claims.Audience != "" || claims.Expires <= claims.IssuedAt {
						t.Errorf("IAM authorization header=%v claims=%v error=%v, want exact SDK signed service-account credential", header, claims, err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					if tc.redirect {
						w.Header().Set("Location", "/unexpected")
						w.WriteHeader(http.StatusFound)
						return
					}
					if _, err := w.Write(response); err != nil {
						t.Errorf("provider write: %v", err)
					}
				}))
				defer server.Close()
				transport, ok := server.Client().Transport.(*http.Transport)
				if !ok {
					t.Fatalf("transport = %T, want *http.Transport", server.Client().Transport)
				}
				transport = transport.Clone()
				transport.TLSClientConfig.ServerName = server.Certificate().DNSNames[0]
				transport.Proxy = nil
				transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
					var dialer net.Dialer
					return dialer.DialContext(ctx, network, server.Listener.Addr().String())
				}
				defer transport.CloseIdleConnections()
				observed := &googleObservedTransport{base: transport}
				key, err := verifierTestKeys.ReadFile("testdata/verifier_rsa.pem")
				if err != nil {
					t.Fatal(err)
				}
				document, err := core.MarshalCanonicalJSONDocument(serviceAccountDocument{Type: credentials.ServiceAccount, ClientEmail: serviceAccountFixtureEmail, PrivateKey: string(key), PrivateKeyID: serviceAccountFixtureKeyID, UniverseDomain: googleTestUniverse})
				if err != nil {
					t.Fatal(err)
				}
				source := serviceAccountSourceWithBytes(t, dir, document)
				source.client, err = exchange.NewClient(&http.Client{Transport: observed})
				if err != nil {
					t.Fatal(err)
				}
				got, err := source.Acquire(t.Context(), request)
				if !errors.Is(err, tc.wantErr) || calls.Load() != 1 || observed.calls.Load() != 1 {
					t.Fatalf("error=%v calls=%d exchange=%d, want %v and one call", err, calls.Load(), observed.calls.Load(), tc.wantErr)
				}
				if tc.wantErr != nil {
					if got != (Token{}) || !errors.Is(err, core.ErrGoogleIdentityContract) {
						t.Fatalf("refused token=%v error=%v, want zero typed refusal", got, err)
					}
					return
				}
				value, err := got.BearerValue()
				if err != nil || value != signed {
					t.Fatalf("returned token matches=%t error=%v, want exact receipt", value == signed, err)
				}
			})
		})
	}
}
