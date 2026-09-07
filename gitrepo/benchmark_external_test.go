package gitrepo_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/gitrepo"
)

func BenchmarkDefaultConfiguration(b *testing.B) {
	var wantErr error
	b.ReportAllocs()
	var last gitrepo.Configuration
	for b.Loop() {
		got, err := gitrepo.DefaultConfiguration()
		if !errors.Is(err, wantErr) {
			b.Fatalf("gitrepo.DefaultConfiguration() error = %v, want %v", err, wantErr)
		}
		last = got
	}
	if err := last.Validate(); err != nil {
		b.Fatalf("gitrepo.DefaultConfiguration().Validate() error = %v, want nil", err)
	}
}

func BenchmarkWorktreeSelectionString(b *testing.B) {
	value := gitrepo.WorktreeSelectionTrackedAndUnignored
	b.ReportAllocs()
	var last string
	for b.Loop() {
		last = value.String()
	}
	if last != "tracked-and-repository-unignored" {
		b.Fatalf("gitrepo.WorktreeSelection.String() = %q, want tracked-and-repository-unignored", last)
	}
}
