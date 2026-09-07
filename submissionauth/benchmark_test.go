package submissionauth

import (
	"errors"
	"testing"
)

func BenchmarkAssemble(b *testing.B) {
	fixture := newAuthFixture(b, authFixtureRequest{})
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
			b.Fatalf("submissionauth.Assemble() error = %v, want %v", err, wantErr)
		}
		last = got
	}
	if err := last.Validate(); err != nil {
		b.Fatalf("submissionauth.Assemble().Validate() error = %v, want nil", err)
	}
}
