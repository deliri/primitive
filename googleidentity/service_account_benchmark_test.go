package googleidentity

import (
	"github.com/deliri/primitive/v2026/core"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func BenchmarkServiceAccountAcquisition(b *testing.B) {
	dir := b.TempDir()
	signer := &verifierTestProvider{key: verifierTestKey(b, "testdata/verifier_rsa.pem")}
	signed := signer.sign(b, verifierTestHeader{Algorithm: verifierTestAlgorithm, KeyID: verifierTestKeyID}, verifierClaims(), false)
	data, err := core.MarshalCanonicalJSONDocument(serviceAccountFixtureResponse{IDToken: strings.TrimPrefix(signed, bearerPrefix)})
	if err != nil {
		b.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(data); err != nil {
			b.Errorf("token response write error=%v, want nil", err)
		}
	}))
	defer server.Close()
	source := serviceAccountFixtureSource(b, dir, server.URL)
	request := serviceAccountFixtureRequest(b)
	b.ReportAllocs()
	for b.Loop() {
		got, err := source.Acquire(b.Context(), request)
		if err != nil {
			b.Fatal(err)
		}
		value, err := got.BearerValue()
		if err != nil || value != signed {
			b.Fatalf("token matches=%t error=%v, want exact provider bytes", value == signed, err)
		}
	}
}
