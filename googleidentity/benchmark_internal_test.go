package googleidentity

import (
	"github.com/deliri/primitive/v2026/core"
	"io"
	"net/http"
	"testing"
)

func BenchmarkAccessResponseDecode(b *testing.B) {
	want := googleAccessTokenResponse{AccessToken: googleTestToken, TokenType: googleAccessTokenTypeBearer, ExpiresIn: 300}
	if err := want.Validate(); err != nil {
		b.Fatal(err)
	}
	input, err := core.MarshalCanonicalJSONDocument(want)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		got, err := decodeGoogleAccessTokenResponse(input)
		if err != nil || got != want {
			b.Fatalf("response equal=%t error=%v, want exact response", got == want, err)
		}
	}
}
func BenchmarkMetadataIdentityAcquisition(b *testing.B) {
	client := googleTestClient(b, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(googleMetadataHeaderName, googleMetadataHeaderValue)
		if _, err := io.WriteString(w, googleTestToken); err != nil {
			b.Errorf("metadata Write error=%v, want nil", err)
		}
	}))
	request := IdentityTokenRequest{Audience: mustGoogleAudience(b), Policy: mustGooglePolicy(b)}
	want := bearerPrefix + googleTestToken
	b.ReportAllocs()
	for b.Loop() {
		token, err := AcquireGoogleCloud(b.Context(), client, request)
		if err != nil {
			b.Fatal(err)
		}
		got, err := token.BearerValue()
		if err != nil || got != want {
			b.Fatalf("identity token equal=%t error=%v, want exact token", got == want, err)
		}
	}
}
func BenchmarkMetadataAccessAcquisition(b *testing.B) {
	wire := googleAccessTokenResponse{AccessToken: googleTestToken, TokenType: googleAccessTokenTypeBearer, ExpiresIn: 300}
	if err := wire.Validate(); err != nil {
		b.Fatal(err)
	}
	data, err := core.MarshalCanonicalJSONDocument(wire)
	if err != nil {
		b.Fatal(err)
	}
	client := googleTestClient(b, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(googleMetadataHeaderName, googleMetadataHeaderValue)
		if _, err := w.Write(data); err != nil {
			b.Errorf("metadata Write error=%v, want nil", err)
		}
	}))
	request := GoogleCloudAccessTokenRequest{Policy: mustGooglePolicy(b)}
	want, err := wire.token()
	if err != nil {
		b.Fatal(err)
	}
	wantText, err := want.BearerValue()
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		got, err := AcquireGoogleCloudAccessToken(b.Context(), client, request)
		if err != nil {
			b.Fatal(err)
		}
		text, err := got.BearerValue()
		if err != nil || text != wantText || got.Lifetime() != want.Lifetime() {
			b.Fatalf("access token equal=%t lifetime=%v error=%v, want exact provider facts", text == wantText, got.Lifetime(), err)
		}
	}
}
func BenchmarkVerifyCachedCertificate(b *testing.B) {
	provider := newVerifierTestProvider(b, nil)
	claims := verifierClaims()
	signed := provider.sign(b, verifierTestHeader{Algorithm: verifierTestAlgorithm, KeyID: verifierTestKeyID}, claims, false)
	verifier := provider.verifier(b, verifierTestAudience)
	want := claims.identity(b)
	got, err := verifier.Verify(b.Context(), signed)
	if err != nil || got != want || provider.calls.Load() != 1 {
		b.Fatalf("warm verifier got=%v error=%v calls=%d, want exact identity and one certificate fetch", got, err, provider.calls.Load())
	}
	b.ReportAllocs()
	for b.Loop() {
		got, err := verifier.Verify(b.Context(), signed)
		if err != nil || got != want {
			b.Fatalf("verified identity got=%v error=%v, want %v", got, err, want)
		}
	}
	if got := provider.calls.Load(); got != 1 {
		b.Fatalf("certificate fetches=%d, want warmed cache only", got)
	}
}
