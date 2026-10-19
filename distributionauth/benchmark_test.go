package distributionauth

import (
	"errors"
	"testing"
)

func BenchmarkAssembleUpdate(b *testing.B) {
	fixture := newDistributionAuthFixture(b, distributionAuthFixtureRequest{})
	assembly := UpdateRequestAssembly{
		Request:     fixture.update.Request,
		Certificate: fixture.update.Certificate,
	}
	var wantErr error
	b.ReportAllocs()
	var last UpdateRequestDocument
	for b.Loop() {
		got, err := AssembleUpdate(assembly)
		if !errors.Is(err, wantErr) {
			b.Fatalf("distributionauth.AssembleUpdate() error = %v, want %v", err, wantErr)
		}
		last = got
	}
	if err := last.Validate(); err != nil {
		b.Fatalf("distributionauth.AssembleUpdate().Validate() error = %v, want nil", err)
	}
}

var benchmarkVerifiedUpdateSink VerifiedUpdate

// VerifyUpdate is the server's per-request admission path for update checks.
func BenchmarkVerifyUpdate(b *testing.B) {
	fixture := newDistributionAuthFixture(b, distributionAuthFixtureRequest{})
	verification := UpdateVerification{Document: fixture.update, Server: distributionAuthServer(b, fixture.trusted)}
	b.ReportAllocs()
	for b.Loop() {
		verified, err := VerifyUpdate(verification)
		if err != nil {
			b.Fatalf("distributionauth.VerifyUpdate() error = %v, want nil", err)
		}
		benchmarkVerifiedUpdateSink = verified
	}
	if payload, err := benchmarkVerifiedUpdateSink.Payload(); err != nil || payload != fixture.update.Request.Payload {
		b.Fatalf("distributionauth.VerifyUpdate().Payload() = (%+v, %v), want exact fixture payload", payload, err)
	}
}
