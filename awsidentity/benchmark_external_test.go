package awsidentity_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/awsidentity"
)

func BenchmarkParseAudience(b *testing.B) {
	const value = "sts.amazonaws.com"
	var wantErr error
	b.ReportAllocs()
	var last awsidentity.Audience
	for b.Loop() {
		got, err := awsidentity.ParseAudience(value)
		if !errors.Is(err, wantErr) {
			b.Fatalf("awsidentity.ParseAudience() error = %v, want %v", err, wantErr)
		}
		last = got
	}
	if last.String() != value {
		b.Fatalf("awsidentity.ParseAudience() = %q, want %q", last, value)
	}
}
