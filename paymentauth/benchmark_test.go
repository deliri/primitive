package paymentauth

import (
	"errors"
	"testing"
)

func BenchmarkAssemble(b *testing.B) {
	fixture := newPaymentQueryFixture(b, standardPaymentQueryFixtureRequest(b))
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
			b.Fatalf("paymentauth.Assemble() error = %v, want %v", err, wantErr)
		}
		last = got
	}
	if err := last.Validate(); err != nil {
		b.Fatalf("paymentauth.Assemble().Validate() error = %v, want nil", err)
	}
}
