package retrievalauth

import (
	"errors"
	"testing"
)

func BenchmarkAssemble(b *testing.B) {
	fixture := newRetrievalAuthFixture(b, retrievalAuthFixtureRequest{})
	assembly := RequestAssembly{
		Request:     fixture.request,
		Certificate: fixture.certificate,
	}
	var wantErr error
	b.ReportAllocs()
	var last RequestDocument
	for b.Loop() {
		got, err := Assemble(assembly)
		if !errors.Is(err, wantErr) {
			b.Fatalf("retrievalauth.Assemble() error = %v, want %v", err, wantErr)
		}
		last = got
	}
	if err := last.Validate(); err != nil {
		b.Fatalf("retrievalauth.Assemble().Validate() error = %v, want nil", err)
	}
}
