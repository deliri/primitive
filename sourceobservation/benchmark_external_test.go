package sourceobservation_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/sourceobservation"
)

func BenchmarkNewLanguage(b *testing.B) {
	const value = "go"
	var wantErr error
	b.ReportAllocs()
	var last sourceobservation.Language
	for b.Loop() {
		got, err := sourceobservation.NewLanguage(value)
		if !errors.Is(err, wantErr) {
			b.Fatalf("sourceobservation.NewLanguage() error = %v, want %v", err, wantErr)
		}
		last = got
	}
	if last.String() != value {
		b.Fatalf("sourceobservation.NewLanguage() = %q, want %q", last, value)
	}
}

func BenchmarkVerifyProject(b *testing.B) {
	project, resolver := packagedObservationFixture(b, true)
	b.ReportAllocs()
	var got sourceobservation.Summary
	var err error
	for b.Loop() {
		got, err = sourceobservation.VerifyProject(b.Context(), project, resolver)
		if err != nil {
			b.Fatal(err)
		}
	}
	if got.Files != 2 || got.Packages != 1 || got.Declarations != 3 {
		b.Fatalf("summary = %+v, want two files, one package and three declarations", got)
	}
}

func BenchmarkFileJSONRoundTrip(b *testing.B) {
	_, resolver := packagedObservationFixture(b, true)
	var file sourceobservation.File
	for _, candidate := range resolver.files {
		if len(candidate.Declarations) == 2 {
			file = candidate
		}
	}
	want, err := file.ObservationDigest()
	if err != nil || len(file.Declarations) != 2 {
		b.Fatalf("fixture digest = %v, %v; want a two-declaration file", want, err)
	}
	b.ReportAllocs()
	var got core.SHA256Digest
	for b.Loop() {
		encoded, err := file.MarshalJSON()
		if err != nil {
			b.Fatal(err)
		}
		var decoded sourceobservation.File
		if err := decoded.UnmarshalJSON(encoded); err != nil {
			b.Fatal(err)
		}
		got, err = decoded.ObservationDigest()
		if err != nil {
			b.Fatal(err)
		}
	}
	if got != want {
		b.Fatalf("round-trip digest = %v, want %v", got, want)
	}
}
