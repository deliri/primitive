package github_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/github"
)

func BenchmarkParseRepository(b *testing.B) {
	const value = "offGridSoft/blink-kernel"
	var wantErr error
	b.ReportAllocs()
	var last github.Repository
	for b.Loop() {
		got, err := github.ParseRepository(value)
		if !errors.Is(err, wantErr) {
			b.Fatalf("github.ParseRepository() error = %v, want %v", err, wantErr)
		}
		last = got
	}
	if last.String() != value {
		b.Fatalf("github.ParseRepository() = %q, want %q", last, value)
	}
}
