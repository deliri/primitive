package googleidentity_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/googleidentity"
)

func BenchmarkParseAudience(b *testing.B) {
	const value = "https://iamcredentials.googleapis.com/"
	var wantErr error
	b.ReportAllocs()
	var last googleidentity.Audience
	for b.Loop() {
		got, err := googleidentity.ParseAudience(value)
		if !errors.Is(err, wantErr) {
			b.Fatalf("googleidentity.ParseAudience() error = %v, want %v", err, wantErr)
		}
		last = got
	}
	if last.String() != value {
		b.Fatalf("googleidentity.ParseAudience() = %q, want %q", last, value)
	}
}
