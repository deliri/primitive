package googleidentity

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"cloud.google.com/go/auth/credentials"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/filestore"
)

type serviceAccountFixtureDocument struct {
	Type         credentials.CredType `json:"type"`
	ClientEmail  string               `json:"client_email"`
	PrivateKey   string               `json:"private_key"`
	PrivateKeyID string               `json:"private_key_id"`
	TokenURI     string               `json:"token_uri"`
}

type serviceAccountFixtureResponse struct {
	IDToken string `json:"id_token"`
}

// The local OAuth provider exercises acquisition only. Its opaque token is
// deliberately not a Google-signed identity and grants no production authority.
func TestServiceAccountSDKBoundaryRefusesUnownedCredentialKinds(t *testing.T) {
	t.Parallel()
	client, err := exchange.NewStandardClient()
	if err != nil {
		t.Fatal(err)
	}
	audience := serviceAccountFixtureRequest(t).Audience
	cases := []struct {
		name string
		data []byte
	}{
		{"empty credential cannot detect ADC", nil},
		{"null credential cannot detect ADC", []byte("null")},
		{"wrong JSON root is not a service account", []byte("[]")},
		{"missing key cannot issue assertion", []byte(`{"type":"service_account","client_email":"worker@example.invalid"}`)},
	}
	for _, kind := range []credentials.CredType{credentials.AuthorizedUser, credentials.ExternalAccount, credentials.ImpersonatedServiceAccount} {
		data, err := core.MarshalCanonicalJSONDocument(serviceAccountFixtureDocument{Type: kind})
		if err != nil {
			t.Fatal(err)
		}
		cases = append(cases, struct {
			name string
			data []byte
		}{"foreign credential kind " + string(kind), data})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := acquireServiceAccountDocument(t.Context(), client, audience, tc.data)
			if !errors.Is(err, core.ErrGoogleIdentityContract) || got != (Token{}) {
				t.Fatalf("acquireServiceAccountDocument() = (%v,%v), want zero and typed refusal", got, err)
			}
		})
	}
}

func serviceAccountFixtureRequest(t testing.TB) IdentityTokenRequest {
	t.Helper()
	audience, err := ParseAudience("https://receiver.example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	policy, err := DefaultPolicy()
	if err != nil {
		t.Fatal(err)
	}
	return IdentityTokenRequest{Audience: audience, Policy: policy}
}

func serviceAccountFixtureSource(t testing.TB, dir, endpoint string) ServiceAccountSource {
	t.Helper()
	key, err := verifierTestKeys.ReadFile("testdata/verifier_rsa.pem")
	if err != nil {
		t.Fatal(err)
	}
	document := serviceAccountFixtureDocument{Type: credentials.ServiceAccount, ClientEmail: serviceAccountFixtureEmail, PrivateKey: string(key), PrivateKeyID: serviceAccountFixtureKeyID, TokenURI: endpoint}
	encoded, err := core.MarshalCanonicalJSONDocument(document)
	if err != nil {
		t.Fatal(err)
	}
	return serviceAccountSourceWithBytes(t, dir, encoded)
}

func serviceAccountSourceWithBytes(t testing.TB, dir string, encoded []byte) ServiceAccountSource {
	t.Helper()
	path, err := core.ParseAbsolutePath(filepath.Join(dir, "credential.json"))
	if err != nil {
		t.Fatal(err)
	}
	location, err := filestore.OpenParent(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := location.Root.Close(); err != nil {
			t.Errorf("Close(fixture) = %v, want nil", err)
		}
	}()
	temporary, err := core.ParseRelativePath("credential.tmp")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := filestore.Write(t.Context(), filestore.WriteRequest{Source: bytes.NewReader(encoded), Location: location, Temporary: temporary, Mode: 0600, Install: filestore.InstallCreate}); err != nil {
		t.Fatal(err)
	}
	client, err := exchange.NewStandardClient()
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewServiceAccountSource(client, path)
	if err != nil {
		t.Fatal(err)
	}
	return source
}

// Finite regression/fuzz fixture crossing the former credential quota.
const serviceAccountCredentialFixtureBytes = 64 << 10

func TestServiceAccountFileExtentLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		size int
	}{
		{"one below former credential quota", serviceAccountCredentialFixtureBytes - 1},
		{"exact former credential quota", serviceAccountCredentialFixtureBytes},
		{"one above former credential quota", serviceAccountCredentialFixtureBytes + 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			size := tc.size
			t.Parallel()
			dir := t.TempDir()
			source := serviceAccountSourceWithBytes(t, dir, bytes.Repeat([]byte{' '}, size))
			got, err := readServiceAccountCredential(t.Context(), source.path)
			if err != nil || !bytes.Equal(got, bytes.Repeat([]byte{' '}, size)) {
				t.Fatalf("credential read = (%d,%v), want %d exact fixture bytes and nil", len(got), err, size)
			}
		})
	}
}

func FuzzServiceAccountCredentialAcquisition(f *testing.F) {
	signer := &verifierTestProvider{key: verifierTestKey(f, "testdata/verifier_rsa.pem")}
	signed := signer.sign(f, verifierTestHeader{Algorithm: verifierTestAlgorithm, KeyID: verifierTestKeyID}, verifierClaims(), false)
	response, err := core.MarshalCanonicalJSONDocument(serviceAccountFixtureResponse{IDToken: strings.TrimPrefix(signed, "Bearer ")})
	if err != nil {
		f.Fatal(err)
	}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(response); err != nil {
			f.Errorf("provider response = %v, want nil", err)
		}
	}))
	defer provider.Close()
	key, err := verifierTestKeys.ReadFile("testdata/verifier_rsa.pem")
	if err != nil {
		f.Fatal(err)
	}
	seed, err := core.MarshalCanonicalJSONDocument(serviceAccountFixtureDocument{Type: credentials.ServiceAccount, ClientEmail: serviceAccountFixtureEmail, PrivateKey: string(key), PrivateKeyID: serviceAccountFixtureKeyID, TokenURI: provider.URL})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Add([]byte{})
	f.Add([]byte("null"))
	f.Add([]byte("{broken"))
	f.Fuzz(func(t *testing.T, data []byte) {
		dir := t.TempDir()
		if len(data) > serviceAccountCredentialFixtureBytes+1 {
			data = data[:serviceAccountCredentialFixtureBytes+1]
		}
		source := serviceAccountSourceWithBytes(t, dir, data)
		// All mutated SDK endpoints terminate at this local provider. The adapter
		// under test owns this real transport; fuzzing never contacts a live service.
		transport := &http.Transport{DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, network, provider.Listener.Addr().String())
		}}
		defer transport.CloseIdleConnections()
		client, err := exchange.NewClient(&http.Client{Transport: transport})
		if err != nil {
			t.Fatal(err)
		}
		source.client = client
		got, err := source.Acquire(t.Context(), serviceAccountFixtureRequest(t))
		if err != nil {
			if bytes.Equal(data, seed) || !errors.Is(err, core.ErrGoogleIdentityContract) || got != (Token{}) {
				t.Fatalf("Acquire(rejected) = (%v,%v), want zero and typed refusal", got, err)
			}
			return
		}
		value, err := got.BearerValue()
		if err != nil || value != signed {
			t.Fatalf("Acquire(accepted) = (%v,%v), want exact local provider token", got, err)
		}
		if got.Validate() != nil {
			t.Fatalf("accepted token validation = %v, want nil", got.Validate())
		}
	})
}
