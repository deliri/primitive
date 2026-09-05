package googleidentity

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/deliri/primitive/v2026/core"
)

func TestGoogleCloudVerifierSignedMutationPairsPreserveAuthority(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name              string
		mutate            func(*verifierTestClaims)
		wantSamePrincipal bool
	}{
		{name: "email rename requires a fresh signature and preserves principal", mutate: func(c *verifierTestClaims) { c.Email = "renamed@example.iam.gserviceaccount.com" }, wantSamePrincipal: true},
		{name: "subject replacement requires a fresh signature and changes principal", mutate: func(c *verifierTestClaims) { c.Subject += "-other" }},
		{name: "expiry renewal requires a fresh signature and preserves principal", mutate: func(c *verifierTestClaims) { c.Expires++ }, wantSamePrincipal: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				p := newVerifierTestProvider(t, nil)
				v := p.verifier(t, verifierTestAudience)
				header := verifierTestHeader{Algorithm: verifierTestAlgorithm, KeyID: verifierTestKeyID}
				claims := verifierClaims()
				baseline := p.sign(t, header, claims, false)
				before, err := v.Verify(t.Context(), baseline)
				if err != nil || before != claims.identity(t) {
					t.Fatalf("baseline Verify() = (%+v, %v), want exact signed identity", before, err)
				}
				tc.mutate(&claims)
				if claims == verifierClaims() {
					t.Fatal("claim mutation equality = true, want false")
				}
				payload, err := core.MarshalCanonicalJSONDocument(claims)
				if err != nil {
					t.Fatalf("mutated claim encoding error = %v, want nil", err)
				}
				first, last := strings.IndexByte(baseline, '.'), strings.LastIndexByte(baseline, '.')
				forged := baseline[:first+1] + base64.RawURLEncoding.EncodeToString(payload) + baseline[last:]
				if forged == baseline {
					t.Fatal("signed bytes mutation equality = true, want false")
				}
				refused, err := v.Verify(t.Context(), forged)
				if !errors.Is(err, core.ErrGoogleIdentityContract) || refused != (GoogleCloudVerifiedIdentity{}) {
					t.Fatalf("forged Verify() = (%+v, %v), want zero and typed signature refusal", refused, err)
				}
				replayed, err := v.Verify(t.Context(), baseline)
				if err != nil || replayed != before {
					t.Fatalf("baseline replay = (%+v, %v), want (%+v, nil)", replayed, err, before)
				}
				after, err := v.Verify(t.Context(), p.sign(t, header, claims, false))
				if err != nil || after != claims.identity(t) {
					t.Fatalf("freshly signed mutation = (%+v, %v), want exact changed facts", after, err)
				}
				oldPrincipal, oldErr := before.PrincipalIdentity()
				newPrincipal, newErr := after.PrincipalIdentity()
				if oldErr != nil || newErr != nil || (oldPrincipal == newPrincipal) != tc.wantSamePrincipal {
					t.Fatalf("principal equality = (%t, %v, %v), want (%t, nil, nil)", oldPrincipal == newPrincipal, oldErr, newErr, tc.wantSamePrincipal)
				}
				if calls := p.calls.Load(); calls != 1 {
					t.Fatalf("cached authority reads = %d, want 1 across accepted refused and replayed tokens", calls)
				}
			})
		})
	}
}

func TestGoogleCloudSigningAlgorithmExhaustiveDomain(t *testing.T) {
	t.Parallel()
	for value := range 256 {
		got := googleCloudSigningAlgorithm(value)
		encoded, err := got.MarshalJSON()
		if got == googleCloudSigningAlgorithmRS256 {
			if err != nil || got.Validate() != nil {
				t.Fatalf("RS256 encoding = (%q, %v), want admitted", encoded, err)
			}
			continue
		}
		if !errors.Is(got.Validate(), core.ErrGoogleIdentityContract) || !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrGoogleIdentityContract) || len(encoded) != 0 {
			t.Fatalf("algorithm %d encoding = (%q, %v), want empty typed refusal", value, encoded, err)
		}
	}
	var receiver *googleCloudSigningAlgorithm
	if err := receiver.UnmarshalJSON(nil); !errors.Is(err, core.ErrGoogleIdentityContract) || !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil algorithm receiver error = %v, want typed refusal", err)
	}
}
