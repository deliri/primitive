package retrieval_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/retrieval"
)

func BenchmarkParseSigningDomain(b *testing.B) {
	const value = retrieval.SigningDomainRequestV1Token
	var wantErr error
	b.ReportAllocs()
	var last retrieval.SigningDomain
	for b.Loop() {
		got, err := retrieval.SigningDomainUnknown.ParseCanonicalText([]byte(value))
		if !errors.Is(err, wantErr) {
			b.Fatalf("retrieval.ParseCanonicalText() error = %v, want %v", err, wantErr)
		}
		last = got
	}
	if last.String() != value {
		b.Fatalf("retrieval.ParseCanonicalText() = %q, want %q", last, value)
	}
}
