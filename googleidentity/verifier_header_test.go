package googleidentity

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/deliri/primitive/v2026/core"
)

func TestGoogleCloudVerifierHeaderBoundaryLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name             string
		mutate           func([]byte) []byte
		wantErr          error
		wantCertificates uint64
	}{
		{name: "canonical compiler owned header admits signed claims", wantCertificates: 1},
		{name: "absent header cannot produce identity", mutate: func([]byte) []byte { return nil }, wantErr: core.ErrGoogleIdentityContract},
		{name: "duplicate algorithm is refused even when values agree", mutate: func(b []byte) []byte { return append([]byte(`{"alg":"RS256",`), b[1:]...) }, wantErr: core.ErrGoogleIdentityContract},
		{name: "unknown critical header cannot be ignored", mutate: func(b []byte) []byte { return append([]byte(`{"crit":["unowned"],`), b[1:]...) }, wantErr: core.ErrGoogleIdentityContract},
		{name: "wrong type algorithm cannot reach SDK", mutate: func(b []byte) []byte { return bytes.Replace(b, []byte(`"RS256"`), []byte(`123`), 1) }, wantErr: core.ErrGoogleIdentityContract},
		{name: "header null cannot become default authority", mutate: func([]byte) []byte { return []byte(`null`) }, wantErr: core.ErrGoogleIdentityContract},
		{name: "truncated header cannot reach certificate acquisition", mutate: func(b []byte) []byte { return b[:len(b)-1] }, wantErr: core.ErrGoogleIdentityContract},
		{name: "second header document cannot be silently ignored", mutate: func(b []byte) []byte { return append(b, b...) }, wantErr: core.ErrGoogleIdentityContract},
		{name: "header one below byte ceiling preserves signature", mutate: padVerifierHeader(GoogleCloudIdentityHeaderMaximumBytes - 1), wantCertificates: 1},
		{name: "header at byte ceiling preserves signature", mutate: padVerifierHeader(GoogleCloudIdentityHeaderMaximumBytes), wantCertificates: 1},
		{name: "header one above byte ceiling cannot fetch authority", mutate: padVerifierHeader(GoogleCloudIdentityHeaderMaximumBytes + 1), wantErr: core.ErrGoogleIdentityContract},
		{name: "header extreme below token ceiling cannot fetch authority", mutate: padVerifierHeader(2 * GoogleCloudIdentityHeaderMaximumBytes), wantErr: core.ErrGoogleIdentityContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				p := newVerifierTestProvider(t, nil)
				header := googleCloudJWTHeader{Algorithm: googleCloudSigningAlgorithmRS256, KeyID: verifierTestKeyID}
				if err := header.Validate(); err != nil {
					t.Fatalf("header Validate() error = %v, want nil", err)
				}
				encoded, err := core.MarshalCanonicalJSONDocument(header)
				if err != nil {
					t.Fatalf("header encoding error = %v, want nil", err)
				}
				if tc.mutate != nil {
					before := bytes.Clone(encoded)
					encoded = tc.mutate(encoded)
					if bytes.Equal(before, encoded) {
						t.Fatal("header mutation byte equality = true, want false")
					}
				}
				claims := verifierClaims()
				body, err := core.MarshalCanonicalJSONDocument(claims)
				if err != nil {
					t.Fatalf("claims encoding error = %v, want nil", err)
				}
				bearer := p.signBytes(t, encoded, body, false)
				got, err := p.verifier(t, verifierTestAudience).Verify(t.Context(), bearer)
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("Verify() error = %v, want %v", err, tc.wantErr)
				}
				if calls := p.calls.Load(); calls != tc.wantCertificates {
					t.Fatalf("certificate requests = %d, want %d", calls, tc.wantCertificates)
				}
				want := GoogleCloudVerifiedIdentity{}
				if tc.wantErr == nil {
					want = claims.identity(t)
				}
				if got != want {
					t.Fatalf("Verify() identity = %+v, want %+v", got, want)
				}
			})
		})
	}
}

func padVerifierHeader(size int) func([]byte) []byte {
	return func(b []byte) []byte { return append(b, strings.Repeat(" ", size-len(b))...) }
}

func FuzzGoogleCloudSigningAlgorithmSemanticClosure(f *testing.F) {
	seed, err := googleCloudSigningAlgorithmRS256.MarshalJSON()
	if err != nil {
		f.Fatalf("algorithm MarshalJSON() error = %v, want nil", err)
	}
	f.Add(seed)
	f.Add([]byte(`null`))
	f.Add([]byte(`"ES256"`))
	f.Fuzz(func(t *testing.T, data []byte) {
		got := googleCloudSigningAlgorithmRS256
		err := got.UnmarshalJSON(data)
		if err != nil {
			if !errors.Is(err, core.ErrGoogleIdentityContract) || !errors.Is(err, core.ErrJSONContract) || got != googleCloudSigningAlgorithmRS256 {
				t.Fatalf("refused algorithm = (%v, %v), want unchanged and typed refusal", got, err)
			}
			if bytes.Equal(data, seed) {
				t.Fatal("canonical algorithm result = refused, want accepted")
			}
			return
		}
		text, decodeErr := core.DecodeJSONStringToken(data)
		if decodeErr != nil || text != googleCloudAlgorithmRS256Text || got != googleCloudSigningAlgorithmRS256 || got.Validate() != nil {
			t.Fatalf("admitted algorithm = (%v, %q, %v), want RS256 only", got, text, decodeErr)
		}
		encoded, err := got.MarshalJSON()
		if err != nil || !bytes.Equal(encoded, seed) {
			t.Fatalf("canonical algorithm = (%q, %v), want (%q, nil)", encoded, err, seed)
		}
	})
}
