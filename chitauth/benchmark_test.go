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
