package version_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/compass"
	"github.com/deliri/primitive/v2026/version"
)

func BenchmarkFromProject(b *testing.B) {
	configuration, err := compass.Current()
	if err != nil {
		b.Fatalf("compass.Current() error = %v, want nil", err)
	}
	var wantErr error
	b.ReportAllocs()
	var last version.Release
	for b.Loop() {
		release, err := version.FromProject(configuration.Project)
		if !errors.Is(err, wantErr) {
			b.Fatalf("version.FromProject() error = %v, want %v", err, wantErr)
		}
		last = release
	}
	if last.Validate() != nil || last.String() == "" {
		b.Fatalf("version.FromProject() = %v, want a validated release", last)
	}
}

func BenchmarkParseTag(b *testing.B) {
	configuration, err := compass.Current()
	if err != nil {
		b.Fatalf("compass.Current() error = %v, want nil", err)
	}
	release, err := version.FromProject(configuration.Project)
	if err != nil {
		b.Fatalf("version.FromProject() error = %v, want nil", err)
	}
	text := release.Tag().String()
	var wantErr error
	b.ReportAllocs()
	var last version.Tag
	for b.Loop() {
		tag, err := version.ParseTag(text)
		if !errors.Is(err, wantErr) {
			b.Fatalf("version.ParseTag() error = %v, want %v", err, wantErr)
		}
		last = tag
	}
	if last.String() != text {
		b.Fatalf("version.ParseTag() = %q, want %q", last, text)
	}
}
