package googleidentity

import (
	"errors"
	"math"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/deliri/primitive/v2026/core"
)

// Every row enters Verify with bytes signed by a real RSA key, or with an
// explicit mutation of those bytes. Certificate acquisition is real local TLS.
// No row constructs the verifier's output or substitutes its signature check.
func TestGoogleCloudVerifierSignedIngressHostile(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name             string
		claims           func(*verifierTestClaims)
		header           func(*verifierTestHeader)
		mutate           func(string) string
		foreign          bool
		wantErr          error
		wantCertificates uint64
	}{
		{name: "signed ordinary principal survives verification", wantCertificates: 1},
		{name: "different signed subject remains a distinct principal", claims: func(c *verifierTestClaims) { c.Subject = "principal-02" }, wantCertificates: 1},
		{name: "changed signed email preserves issuer subject identity", claims: func(c *verifierTestClaims) { c.Email = "renamed@example.iam.gserviceaccount.com" }, wantCertificates: 1},
		{name: "renewed signed issuance remains observable", claims: func(c *verifierTestClaims) { c.IssuedAt++ }, wantCertificates: 1},
		{name: "renewed signed expiry remains observable", claims: func(c *verifierTestClaims) { c.Expires++ }, wantCertificates: 1},
		{name: "Unicode subject preserves exact provider facts", claims: func(c *verifierTestClaims) { c.Subject = "主体" }, wantCertificates: 1},
		{name: "escaped subject is not recanonicalized into different identity", claims: func(c *verifierTestClaims) { c.Subject = "subject\"with\\escapes" }, wantCertificates: 1},
		{name: "epoch issuance remains a set instant", claims: func(c *verifierTestClaims) { c.IssuedAt = 0 }, wantCertificates: 1},
		{name: "minimum nonempty subject is admitted", claims: func(c *verifierTestClaims) { c.Subject = "s" }, wantCertificates: 1},
		{name: "minimum nonempty provider email fact is admitted", claims: func(c *verifierTestClaims) { c.Email = "e" }, wantCertificates: 1},
		{name: "independently signed foreign audience is refused before certificates", claims: func(c *verifierTestClaims) { c.Audience += "/foreign" }, wantErr: core.ErrGoogleIdentityContract},
		{name: "foreign signer cannot use a matching key identifier", foreign: true, wantErr: core.ErrGoogleIdentityContract, wantCertificates: 1},
		{name: "unknown certificate identifier cannot select the only key", header: func(h *verifierTestHeader) { h.KeyID += "-unknown" }, wantErr: core.ErrGoogleIdentityContract, wantCertificates: 1},
		{name: "signed foreign issuer cannot produce a Google principal", claims: func(c *verifierTestClaims) { c.Issuer = "https://foreign.example" }, wantErr: core.ErrGoogleIdentityContract, wantCertificates: 1},
		{name: "signed empty subject produces no identity", claims: func(c *verifierTestClaims) { c.Subject = "" }, wantErr: core.ErrGoogleIdentityContract, wantCertificates: 1},
		{name: "signed unverified email produces no identity", claims: func(c *verifierTestClaims) { c.EmailVerified = false }, wantErr: core.ErrGoogleIdentityContract, wantCertificates: 1},
		{name: "expired signed token is refused before certificates", claims: func(c *verifierTestClaims) { c.IssuedAt = verifierTestIssued - 2; c.Expires = verifierTestIssued - 1 }, wantErr: core.ErrGoogleIdentityContract},
		{name: "unsigned algorithm cannot bypass signature verification", header: func(h *verifierTestHeader) { h.Algorithm = "none" }, wantErr: core.ErrGoogleIdentityContract},
		{name: "truncated signature cannot preserve accepted claims", mutate: func(s string) string { return s[:strings.LastIndexByte(s, '.')+1] }, wantErr: core.ErrGoogleIdentityContract, wantCertificates: 1},
		{name: "subject one below text ceiling is admitted", claims: func(c *verifierTestClaims) { c.Subject = strings.Repeat("s", GoogleCloudIdentityTextMaximumBytes-1) }, wantCertificates: 1},
		{name: "subject at text ceiling is admitted", claims: func(c *verifierTestClaims) { c.Subject = strings.Repeat("s", GoogleCloudIdentityTextMaximumBytes) }, wantCertificates: 1},
		{name: "subject one above text ceiling is refused", claims: func(c *verifierTestClaims) { c.Subject = strings.Repeat("s", GoogleCloudIdentityTextMaximumBytes+1) }, wantErr: core.ErrGoogleIdentityContract, wantCertificates: 1},
		{name: "subject extreme below token ceiling is refused", claims: func(c *verifierTestClaims) { c.Subject = strings.Repeat("s", 8*GoogleCloudIdentityTextMaximumBytes) }, wantErr: core.ErrGoogleIdentityContract, wantCertificates: 1},
		{name: "email one below text ceiling is admitted", claims: func(c *verifierTestClaims) { c.Email = strings.Repeat("e", GoogleCloudIdentityTextMaximumBytes-1) }, wantCertificates: 1},
		{name: "email at text ceiling is admitted", claims: func(c *verifierTestClaims) { c.Email = strings.Repeat("e", GoogleCloudIdentityTextMaximumBytes) }, wantCertificates: 1},
		{name: "email one above text ceiling is refused", claims: func(c *verifierTestClaims) { c.Email = strings.Repeat("e", GoogleCloudIdentityTextMaximumBytes+1) }, wantErr: core.ErrGoogleIdentityContract, wantCertificates: 1},
		{name: "email extreme below token ceiling is refused", claims: func(c *verifierTestClaims) { c.Email = strings.Repeat("e", 8*GoogleCloudIdentityTextMaximumBytes) }, wantErr: core.ErrGoogleIdentityContract, wantCertificates: 1},
		{name: "expiry one below representable second ceiling is admitted", claims: func(c *verifierTestClaims) { c.Expires = math.MaxInt64/1_000_000_000 - 1 }, wantCertificates: 1},
		{name: "expiry at representable second ceiling is admitted", claims: func(c *verifierTestClaims) { c.Expires = math.MaxInt64 / 1_000_000_000 }, wantCertificates: 1},
		{name: "expiry one above representable second ceiling is refused", claims: func(c *verifierTestClaims) { c.Expires = math.MaxInt64/1_000_000_000 + 1 }, wantErr: core.ErrGoogleIdentityContract, wantCertificates: 1},
		{name: "expiry at signed integer maximum cannot overflow into identity", claims: func(c *verifierTestClaims) { c.Expires = math.MaxInt64 }, wantErr: core.ErrGoogleIdentityContract, wantCertificates: 1},
		{name: "absent bearer cannot fetch certificates or invent identity", mutate: func(string) string { return "" }, wantErr: core.ErrGoogleIdentityContract},
		{name: "extra JWT segment is not ignored", mutate: func(s string) string { return s + ".extra" }, wantErr: core.ErrGoogleIdentityContract},
		{name: "leading bearer whitespace is not repaired", mutate: func(s string) string { return " " + s }, wantErr: core.ErrGoogleIdentityContract},
		{name: "trailing bearer whitespace is not repaired", mutate: func(s string) string { return s + " " }, wantErr: core.ErrGoogleIdentityContract},
		{name: "invalid base64 signature retains typed refusal", mutate: func(s string) string { return s[:strings.LastIndexByte(s, '.')+1] + "~" }, wantErr: core.ErrGoogleIdentityContract},
		{name: "invalid provider text cannot be trimmed into authority", claims: func(c *verifierTestClaims) { c.Subject += " " }, wantErr: core.ErrGoogleIdentityContract, wantCertificates: 1},
		{name: "IAP algorithm cannot select another Google authority", header: func(h *verifierTestHeader) { h.Algorithm = "ES256" }, wantErr: core.ErrGoogleIdentityContract},
		{name: "blank key identifier cannot select an unnamed authority", header: func(h *verifierTestHeader) { h.KeyID = "" }, wantErr: core.ErrGoogleIdentityContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				provider := newVerifierTestProvider(t, nil)
				claims, header := verifierClaims(), verifierTestHeader{Algorithm: verifierTestAlgorithm, KeyID: verifierTestKeyID}
				if tc.claims != nil {
					tc.claims(&claims)
					if claims == verifierClaims() {
						t.Fatal("claims mutation equality = true, want false")
					}
				}
				if tc.header != nil {
					before := header
					tc.header(&header)
					if header == before {
						t.Fatal("header mutation equality = true, want false")
					}
				}
				bearer := provider.sign(t, header, claims, tc.foreign)
				if tc.mutate != nil {
					before := bearer
					bearer = tc.mutate(bearer)
					if bearer == before {
						t.Fatal("wire mutation equality = true, want false")
					}
				}
				got, err := provider.verifier(t, verifierTestAudience).Verify(t.Context(), bearer)
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("Verify() error = %v, want %v", err, tc.wantErr)
				}
				if calls := provider.calls.Load(); calls != tc.wantCertificates {
					t.Fatalf("certificate requests = %d, want %d", calls, tc.wantCertificates)
				}
				if tc.wantErr != nil {
					if got != (GoogleCloudVerifiedIdentity{}) {
						t.Fatalf("refused identity = %+v, want zero", got)
					}
					return
				}
				want := claims.identity(t)
				if got != want {
					t.Fatalf("verified identity = %+v, want %+v", got, want)
				}
				if err := got.Validate(); err != nil {
					t.Fatalf("verified identity Validate() error = %v, want nil", err)
				}
			})
		})
	}
}
