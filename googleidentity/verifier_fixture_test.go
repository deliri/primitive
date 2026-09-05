package googleidentity

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"embed"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

// These are public, test-only RSA keys. They have never represented a Google
// account. The local provider supplies their public keys to the real verifier.
//
//go:embed testdata/verifier_rsa.pem testdata/verifier_foreign_rsa.pem
var verifierTestKeys embed.FS

const (
	verifierTestAlgorithm = googleCloudAlgorithmRS256Text
	verifierTestKeyID     = "local-certificate-authority"
	verifierTestAudience  = "https://receiver.example"
	verifierTestIssued    = int64(946684800)
	verifierTestExpires   = int64(4102444800)
)

type verifierTestHeader struct {
	Algorithm string `json:"alg"`
	KeyID     string `json:"kid"`
}

type verifierTestClaims struct {
	Issuer        string `json:"iss"`
	Audience      string `json:"aud"`
	Subject       string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	IssuedAt      int64  `json:"iat"`
	Expires       int64  `json:"exp"`
}

type verifierTestJWK struct {
	Algorithm string `json:"alg"`
	KeyID     string `json:"kid"`
	KeyType   string `json:"kty"`
	Use       string `json:"use"`
	Exponent  string `json:"e"`
	Modulus   string `json:"n"`
}

type verifierTestKeySet struct {
	Keys []verifierTestJWK `json:"keys"`
}

type verifierTestProvider struct {
	client  exchange.Client
	key     *rsa.PrivateKey
	foreign *rsa.PrivateKey
	calls   atomic.Uint64
}

func newVerifierTestProvider(t testing.TB, response func(http.ResponseWriter, *http.Request, []byte)) *verifierTestProvider {
	t.Helper()
	p := &verifierTestProvider{
		key:     verifierTestKey(t, "testdata/verifier_rsa.pem"),
		foreign: verifierTestKey(t, "testdata/verifier_foreign_rsa.pem"),
	}
	key := verifierTestJWK{
		Algorithm: verifierTestAlgorithm, KeyID: verifierTestKeyID, KeyType: "RSA", Use: "sig",
		Exponent: base64.RawURLEncoding.EncodeToString(big.NewInt(int64(p.key.E)).Bytes()),
		Modulus:  base64.RawURLEncoding.EncodeToString(p.key.N.Bytes()),
	}
	body, err := core.MarshalCanonicalJSONDocument(verifierTestKeySet{Keys: []verifierTestJWK{key}})
	if err != nil {
		t.Fatalf("marshal certificate set error = %v, want nil", err)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		p.calls.Add(1)
		if request.Method != http.MethodGet {
			t.Errorf("certificate method = %s, want GET", request.Method)
		}
		if response != nil {
			response(writer, request, body)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("Cache-Control", "max-age=3600")
		if _, err := writer.Write(body); err != nil {
			t.Errorf("certificate Write() error = %v, want nil", err)
		}
	}))
	t.Cleanup(server.Close)
	transport := server.Client().Transport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.TLSClientConfig.ServerName = server.Certificate().DNSNames[0]
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		var dialer net.Dialer
		return dialer.DialContext(ctx, network, server.Listener.Addr().String())
	}
	t.Cleanup(transport.CloseIdleConnections)
	p.client, err = exchange.NewClient(&http.Client{Transport: transport})
	if err != nil {
		t.Fatalf("exchange.NewClient(local certificate transport) error = %v, want nil", err)
	}
	return p
}

func verifierTestKey(t testing.TB, path string) *rsa.PrivateKey {
	t.Helper()
	data, err := verifierTestKeys.ReadFile(path)
	if err != nil {
		t.Fatalf("read test-only key error = %v, want nil", err)
	}
	block, rest := pem.Decode(data)
	if block == nil || len(rest) != 0 {
		t.Fatal("test-only key PEM = incomplete, want one complete block")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("parse test-only key error = %v, want nil", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		t.Fatal("test-only key algorithm = non-RSA, want RSA")
	}
	return key
}

func verifierClaims() verifierTestClaims {
	return verifierTestClaims{
		Issuer: googleCloudIdentityIssuer, Audience: verifierTestAudience,
		Subject: "principal-01", Email: "runner@example.iam.gserviceaccount.com", EmailVerified: true,
		IssuedAt: verifierTestIssued, Expires: verifierTestExpires,
	}
}

func (c verifierTestClaims) identity(t testing.TB) GoogleCloudVerifiedIdentity {
	t.Helper()
	identity := GoogleCloudVerifiedIdentity{
		Issuer: c.Issuer, Audience: c.Audience, Subject: c.Subject, Email: c.Email, EmailVerified: c.EmailVerified,
		IssuedAt: temporal.InstantFromNanoseconds(c.IssuedAt * 1_000_000_000),
		Expires:  temporal.InstantFromNanoseconds(c.Expires * 1_000_000_000),
	}
	if err := identity.Validate(); err != nil {
		t.Fatalf("want identity Validate() error = %v, want nil", err)
	}
	return identity
}

func (p *verifierTestProvider) verifier(t testing.TB, audience string) GoogleCloudVerifier {
	t.Helper()
	parsed, err := ParseAudience(audience)
	if err != nil {
		t.Fatalf("ParseAudience() error = %v, want nil", err)
	}
	v, err := NewGoogleCloudVerifier(t.Context(), GoogleCloudVerifierConfiguration{Audience: parsed, Client: p.client})
	if err != nil {
		t.Fatalf("NewGoogleCloudVerifier() error = %v, want nil", err)
	}
	return v
}

func (p *verifierTestProvider) sign(t testing.TB, header verifierTestHeader, claims verifierTestClaims, foreign bool) string {
	t.Helper()
	h, err := core.MarshalCanonicalJSONDocument(header)
	if header.Algorithm == googleCloudAlgorithmRS256Text {
		typed := googleCloudJWTHeader{Algorithm: googleCloudSigningAlgorithmRS256, KeyID: header.KeyID}
		if typed.Validate() == nil {
			h, err = core.MarshalCanonicalJSONDocument(typed)
		}
	}
	if err != nil {
		t.Fatalf("marshal signed header error = %v, want nil", err)
	}
	b, err := core.MarshalCanonicalJSONDocument(claims)
	if err != nil {
		t.Fatalf("marshal signed claims error = %v, want nil", err)
	}
	return p.signBytes(t, h, b, foreign)
}

func (p *verifierTestProvider) signBytes(t testing.TB, h, b []byte, foreign bool) string {
	t.Helper()
	content := base64.RawURLEncoding.EncodeToString(h) + "." + base64.RawURLEncoding.EncodeToString(b)
	digest := sha256.Sum256([]byte(content))
	key := p.key
	if foreign {
		key = p.foreign
	}
	signature, err := rsa.SignPKCS1v15(nil, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("sign test-only token error = %v, want nil", err)
	}
	token, err := ParseGoogleCloudCommandOutput([]byte(content + "." + base64.RawURLEncoding.EncodeToString(signature)))
	if err != nil {
		t.Fatalf("ParseGoogleCloudCommandOutput(signed seed) error = %v, want nil", err)
	}
	bearer, err := token.BearerValue()
	if err != nil {
		t.Fatalf("signed seed BearerValue() error = %v, want nil", err)
	}
	return bearer
}
