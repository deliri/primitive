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
