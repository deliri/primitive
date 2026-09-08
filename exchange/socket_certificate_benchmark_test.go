package exchange_test

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func BenchmarkSocketVerifiedClientCertificateDigest(b *testing.B) {
	fixture := socketCertificates(b)
	now, err := fixture.now.Time()
	if err != nil {
		b.Fatal(err)
	}
	chains, err := fixture.leaf.Verify(x509.VerifyOptions{Roots: fixture.roots, CurrentTime: now, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}})
	if err != nil || len(chains) == 0 || len(fixture.leaf.Raw) == 0 {
		b.Fatalf("Go verified fixture = (%v,%v), want nonempty verified certificate", chains, err)
	}
	request := httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/", nil)
	request.TLS = &tls.ConnectionState{VerifiedChains: chains}
	call, err := exchange.NewSocketServerCall(httptest.NewRecorder(), request)
	if err != nil {
		b.Fatal(err)
	}
	want := core.NewSHA256Digest(sha256.Sum256(fixture.leaf.Raw))
	b.SetBytes(int64(len(fixture.leaf.Raw)))
	b.ReportAllocs()
	for b.Loop() {
		got, err := call.VerifiedClientCertificateDigest()
		if err != nil || got != want {
			b.Fatalf("certificate identity = (%v,%v), want exact Go digest", got, err)
		}
	}
}
