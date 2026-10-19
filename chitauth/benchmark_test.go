package chitauth

import (
	"errors"
	"testing"
)

func BenchmarkAssemble(b *testing.B) {
	fixture := newQueryFixture(b, standardQueryFixtureRequest(b))
	assembly := RequestAssembly{
		Request:     fixture.document.Request,
		Certificate: fixture.document.Certificate,
	}
	var wantErr error
	b.ReportAllocs()
	var last RequestDocument
	for b.Loop() {
		got, err := Assemble(assembly)
		if !errors.Is(err, wantErr) {
			b.Fatalf("chitauth.Assemble() error = %v, want %v", err, wantErr)
		}
		last = got
	}
	if err := last.Validate(); err != nil {
		b.Fatalf("chitauth.Assemble().Validate() error = %v, want nil", err)
	}
}

var benchmarkVerifiedSink Verified

// Verify is the server's per-request admission path: certificate chain,
// device keys and query signature over a fixed fixture.
func BenchmarkVerify(b *testing.B) {
	fixture := newQueryFixture(b, standardQueryFixtureRequest(b))
	verification := Verification{Server: fixture.server, Document: fixture.document}
	if err := verification.Validate(); err != nil {
		b.Fatalf("chitauth.Verification.Validate() error = %v, want nil", err)
	}
	b.ReportAllocs()
	for b.Loop() {
		verified, err := Verify(verification)
		if err != nil {
			b.Fatalf("chitauth.Verify() error = %v, want nil", err)
		}
		benchmarkVerifiedSink = verified
	}
	if payload, err := benchmarkVerifiedSink.Payload(); err != nil || payload != fixture.payload {
		b.Fatalf("chitauth.Verify().Payload() = (%v, %v), want exact fixture payload", payload, err)
	}
}
