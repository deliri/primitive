package submission_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/submission"
)

func BenchmarkParseSigningDomain(b *testing.B) {
	const value = submission.SigningDomainRequestV1Token
	var wantErr error
	b.ReportAllocs()
	var last submission.SigningDomain
	for b.Loop() {
		got, err := submission.ParseSigningDomain(value)
		if !errors.Is(err, wantErr) {
			b.Fatalf("submission.ParseSigningDomain() error = %v, want %v", err, wantErr)
		}
		last = got
	}
	if last.String() != value {
		b.Fatalf("submission.ParseSigningDomain() = %q, want %q", last, value)
	}
}
