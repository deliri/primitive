package controlwire_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/controlwire"
)

func BenchmarkParseRequestNonce(b *testing.B) {
	nonce, err := controlwire.GenerateRequestNonce()
	if err != nil {
		b.Fatalf("GenerateRequestNonce() error = %v, want nil", err)
	}
	text := nonce.String()
	var wantErr error
	b.ReportAllocs()
	var last controlwire.RequestNonce
	for b.Loop() {
		parsed, err := controlwire.ParseRequestNonce(text)
		if !errors.Is(err, wantErr) {
			b.Fatalf("controlwire.ParseRequestNonce() error = %v, want %v", err, wantErr)
		}
		last = parsed
	}
	if last.String() != text {
		b.Fatalf("controlwire.ParseRequestNonce() = %q, want %q", last, text)
	}
}
