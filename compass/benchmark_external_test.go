package compass_test

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/compass"
)

const compassParseBatch = 16

func BenchmarkCurrent(b *testing.B) {
	want, err := compass.Current()
	if err != nil || want.Validate() != nil {
		b.Fatalf("current fixture error=%v", err)
	}
	b.ReportAllocs()
	for b.Loop() {
		got, err := compass.Current()
		if err != nil || got != want {
			b.Fatalf("Current got=%v error=%v, want %v", got, err, want)
		}
	}
}
func BenchmarkDecodeConfiguration(b *testing.B) {
	b.ReportAllocs()
	want := compass.Configuration{Project: projectFixture(b, "Evidence Tool", "example.com/project/v2", "owner/project", 2026, 1, 3)}
	encoded := encodedConfiguration(b, want.Project)
	for _, tc := range []struct {
		name       string
		fragmented bool
	}{{"contiguous", false}, {"one_byte_reads", true}} {
		b.Run(tc.name, func(b *testing.B) {
			source := bytes.NewReader(encoded)
			var reader io.Reader = source
			if tc.fragmented {
				reader = iotest.OneByteReader(source)
			}
			b.ReportAllocs()
			for b.Loop() {
				source.Reset(encoded)
				got, err := compass.Decode[compass.Configuration](reader)
				if err != nil || got != want {
					b.Fatalf("Decode got=%v error=%v, want %v", got, err, want)
				}
			}
		})
	}
}
func BenchmarkParseProjectNameBatch(b *testing.B) {
	text := strings.Repeat("n", 128)
	want, err := compass.ParseProjectName(text)
	if err != nil || want.String() != text {
		b.Fatalf("name fixture error=%v", err)
	}
	b.ReportAllocs()
	for b.Loop() {
		for range compassParseBatch {
			got, err := compass.ParseProjectName(text)
			if err != nil || got != want {
				b.Fatalf("name differs=%t error=%v", got != want, err)
			}
		}
	}
	b.ReportMetric(compassParseBatch, "parses/op")
}
