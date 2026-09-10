package googleidentity

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/deliri/primitive/v2026/core"
)

// The local metadata provider reproduces the observed GCE distinction:
// standard omits the account claims; full includes them. Acquisition and
// signature verification are the real production implementations.
func TestGoogleMetadataAcquisitionToVerifierLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name             string
		mutate           func(*verifierTestClaims)
		foreign          bool
		cancel           bool
		wantErr          error
		wantCertificates uint64
	}{
		{name: "attached account survives metadata acquisition and verification", wantCertificates: 1},
		{name: "foreign signature cannot become an acquired identity", foreign: true, wantErr: core.ErrGoogleIdentityContract, wantCertificates: 1},
		{name: "unverified account cannot become an acquired identity", mutate: func(c *verifierTestClaims) { c.EmailVerified = false }, wantErr: core.ErrGoogleIdentityContract, wantCertificates: 1},
		{name: "foreign audience cannot borrow the metadata request audience", mutate: func(c *verifierTestClaims) { c.Audience += "/foreign" }, wantErr: core.ErrGoogleIdentityContract},
		{name: "cancelled acquisition produces no token or certificate request", cancel: true, wantErr: context.Canceled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				provider := newVerifierTestProvider(t, nil)
				claims := verifierClaims()
				if tc.mutate != nil {
					tc.mutate(&claims)
					if claims == verifierClaims() {
						t.Fatal("claim mutation equality = true, want false")
					}
				}
				header := verifierTestHeader{Algorithm: verifierTestAlgorithm, KeyID: verifierTestKeyID}
				full := provider.sign(t, header, claims, tc.foreign)
				// Exact standard-format shape observed on the real VM: no email fields.
				standardClaims := struct {
					Issuer   string `json:"iss"`
					Audience string `json:"aud"`
					Subject  string `json:"sub"`
					IssuedAt int64  `json:"iat"`
					Expires  int64  `json:"exp"`
				}{claims.Issuer, claims.Audience, claims.Subject, claims.IssuedAt, claims.Expires}
				h, err := core.MarshalCanonicalJSONDocument(header)
				if err != nil {
					t.Fatalf("marshal header error = %v, want nil", err)
				}
				b, err := core.MarshalCanonicalJSONDocument(standardClaims)
				if err != nil {
					t.Fatalf("marshal standard claims error = %v, want nil", err)
				}
				standard := provider.signBytes(t, h, b, tc.foreign)
				var metadataCalls atomic.Uint64
				client := googleTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					metadataCalls.Add(1)
					if r.URL.Query().Get(googleAudienceQueryName) != verifierTestAudience {
						t.Errorf("audience = %q, want %q", r.URL.Query().Get(googleAudienceQueryName), verifierTestAudience)
					}
					bearer := standard
					if r.URL.Query().Get(googleFormatQueryName) == googleFormatFullValue {
						bearer = full
					}
					w.Header().Set(googleMetadataHeaderName, googleMetadataHeaderValue)
					if _, err := io.WriteString(w, strings.TrimPrefix(bearer, bearerPrefix)); err != nil {
						t.Errorf("provider write: %v", err)
					}
				}))
				audience, err := ParseAudience(verifierTestAudience)
				if err != nil {
					t.Fatalf("ParseAudience() error = %v, want nil", err)
				}
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				if tc.cancel {
					cancel()
				}
				token, err := AcquireGoogleCloud(ctx, client, IdentityTokenRequest{Audience: audience, Policy: mustGooglePolicy(t)})
				if tc.cancel {
					if !errors.Is(err, tc.wantErr) || token != (Token{}) || metadataCalls.Load() != 0 || provider.calls.Load() != 0 {
						t.Fatalf("cancelled acquisition = (%v,%v,%d metadata,%d certificates), want zero token, %v, no effects", token, err, metadataCalls.Load(), provider.calls.Load(), tc.wantErr)
					}
					return
				}
				if err != nil {
					t.Fatalf("AcquireGoogleCloud() error = %v, want nil", err)
				}
				bearer, err := token.BearerValue()
				if err != nil {
					t.Fatalf("BearerValue() error = %v, want nil", err)
				}
				if bearer != full {
					t.Fatal("acquired signed document equality = false, want exact full-format source facts before verification")
				}
				got, gotErr := provider.verifier(t, verifierTestAudience).Verify(t.Context(), bearer)
				if calls := provider.calls.Load(); calls != tc.wantCertificates {
					t.Fatalf("certificate requests = %d, want %d", calls, tc.wantCertificates)
				}
				if !errors.Is(gotErr, tc.wantErr) || metadataCalls.Load() != 1 {
					t.Fatalf("acquired verification = (%v,%v,%d calls), want error %v and 1 metadata call", got, gotErr, metadataCalls.Load(), tc.wantErr)
				}
				if tc.wantErr != nil {
					if got != (GoogleCloudVerifiedIdentity{}) {
						t.Fatalf("refused identity = %+v, want zero", got)
					}
					return
				}
				want := claims.identity(t)
				if got != want || provider.calls.Load() != 1 {
					t.Fatalf("verified identity = (%+v,%d certificates), want (%+v,1 certificate)", got, provider.calls.Load(), want)
				}
			})
		})
	}
}
