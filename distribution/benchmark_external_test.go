package distribution_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/distribution"
)

func BenchmarkParseSigningDomain(b *testing.B) {
	const value = distribution.SigningDomainPublicationRequestV1Token
	var wantErr error
	b.ReportAllocs()
	var last distribution.SigningDomain
	for b.Loop() {
		got, err := distribution.ParseSigningDomain(value)
		if !errors.Is(err, wantErr) {
			b.Fatalf("distribution.ParseSigningDomain() error = %v, want %v", err, wantErr)
		}
		last = got
	}
	if last.String() != value {
		b.Fatalf("distribution.ParseSigningDomain() = %q, want %q", last, value)
	}
}
