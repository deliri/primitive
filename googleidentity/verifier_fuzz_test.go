package googleidentity

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzGoogleCloudVerifierSignedSemanticClosure(f *testing.F) {
	provider := newVerifierTestProvider(f, nil)
	header := verifierTestHeader{Algorithm: verifierTestAlgorithm, KeyID: verifierTestKeyID}
	ordinary := verifierClaims()
	minimum := ordinary
	minimum.Subject = "s"
	maximum := ordinary
	maximum.Subject = strings.Repeat("s", GoogleCloudIdentityTextMaximumBytes)
	claims := []verifierTestClaims{minimum, ordinary, maximum}
	seeds := make([]string, len(claims))
	for index, claim := range claims {
		claim.identity(f) // Validate the actual typed source facts before signing.
		seeds[index] = provider.sign(f, header, claim, false)
		f.Add(seeds[index])
	}
	foreignAudience := ordinary
	foreignAudience.Audience += "/foreign"
	f.Add(provider.sign(f, header, foreignAudience, false))
	f.Add(provider.sign(f, header, ordinary, true))
	f.Add("")
	f.Add(googleCloudIdentityBearerPrefix + strings.Repeat("a", TokenMaximumBytes+1))
	f.Fuzz(func(t *testing.T, bearer string) {
		synctest.Test(t, func(t *testing.T) {
			local := newVerifierTestProvider(t, nil)
			// Exercise metadata acquisition on the same mutated bytes. Accepted
			// acquisition is opaque, so only the signature oracle below can grant
			// identity; acquisition must neither repair nor authenticate bytes.
			raw := strings.TrimPrefix(bearer, bearerPrefix)
			metadata := googleTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.Query().Get(googleFormatQueryName); got != googleFormatFullValue {
					t.Errorf("metadata format = %q, want %q", got, googleFormatFullValue)
				}
				w.Header().Set(googleMetadataHeaderName, googleMetadataHeaderValue)
				_, _ = io.WriteString(w, raw)
			}))
			audience, audienceErr := ParseAudience(verifierTestAudience)
			if audienceErr != nil {
				t.Fatalf("ParseAudience() error = %v, want nil", audienceErr)
			}
			token, acquireErr := AcquireGoogleCloud(t.Context(), metadata, IdentityTokenRequest{Audience: audience, Policy: mustGooglePolicy(t)})
			if acquireErr != nil {
				if !errors.Is(acquireErr, core.ErrGoogleIdentityContract) || token != (Token{}) || local.calls.Load() != 0 {
					t.Fatalf("metadata refusal = (%v,%v,%d certificates), want typed refusal, zero token and no certificate request", token, acquireErr, local.calls.Load())
				}
			} else {
				acquired, valueErr := token.BearerValue()
				if valueErr != nil || acquired != bearerPrefix+raw || token.Validate() != nil || len(raw) > TokenMaximumBytes {
					t.Fatalf("metadata acquisition = (unchanged %t,%v), want bounded unchanged token and nil", acquired == bearerPrefix+raw, valueErr)
				}
				// A missing Authorization prefix belongs to Verify's separate
				// ingress. Keep its original bytes for that rejection proof.
				if strings.HasPrefix(bearer, bearerPrefix) {
					bearer = acquired
				}
			}
			got, err := local.verifier(t, verifierTestAudience).Verify(t.Context(), bearer)
			if calls := local.calls.Load(); calls > 1 {
				t.Fatalf("certificate requests = %d, want at most one read", calls)
			}
			matched := -1
			for index, seed := range seeds {
				if sameSignedDocument(bearer, seed) {
					matched = index
					break
				}
			}
			if matched >= 0 && err != nil {
				t.Fatalf("Verify(genuinely signed admitted document) error = %v, want nil", err)
			}
			if matched >= 0 && acquireErr != nil {
				t.Fatalf("AcquireGoogleCloud(genuinely signed document) error = %v, want nil", acquireErr)
			}
			if err != nil {
				if !errors.Is(err, core.ErrGoogleIdentityContract) || got != (GoogleCloudVerifiedIdentity{}) {
					t.Fatalf("Verify(refused) = (%+v, %v), want zero and %v", got, err, core.ErrGoogleIdentityContract)
				}
				return
			}
			if len(bearer) > TokenMaximumBytes+len(googleCloudIdentityBearerPrefix) {
				t.Fatal("oversized bearer result = verified identity, want zero and typed refusal")
			}
			if matched < 0 {
				t.Fatal("accepted signature source = unknown, want trusted signed seed")
			}
			want := claims[matched].identity(t)
			if got != want || got.Validate() != nil {
				t.Fatalf("Verify(accepted) = %+v, want exact signed facts %+v", got, want)
			}
			first, err := core.MarshalCanonicalJSONDocument(got)
			if err != nil {
				t.Fatalf("canonical identity error = %v, want nil", err)
			}
			decoded, err := core.DecodeStrictJSONStructure[GoogleCloudVerifiedIdentity](first, core.DefaultStrictJSONLimits())
			if err != nil || decoded != got {
				t.Fatalf("canonical identity round trip = (%+v, %v), want exact facts", decoded, err)
			}
			second, err := core.MarshalCanonicalJSONDocument(decoded)
			if err != nil || !bytes.Equal(first, second) {
				t.Fatalf("canonical second encoding = (equal %t, %v), want true and nil", bytes.Equal(first, second), err)
			}
		})
	})
}

// JWT signatures may have equivalent accepted base64 spellings. The oracle
// pins the exact signed header/payload bytes and the decoded signature bytes,
// not the incidental spelling of the signature's unused trailing bits.
func sameSignedDocument(got, signed string) bool {
	gotEnd, signedEnd := strings.LastIndexByte(got, '.'), strings.LastIndexByte(signed, '.')
	if gotEnd < 0 || signedEnd < 0 || got[:gotEnd] != signed[:signedEnd] {
		return false
	}
	gotSignature, gotErr := base64.RawURLEncoding.DecodeString(got[gotEnd+1:])
	signedSignature, signedErr := base64.RawURLEncoding.DecodeString(signed[signedEnd+1:])
	return gotErr == nil && signedErr == nil && bytes.Equal(gotSignature, signedSignature)
}
