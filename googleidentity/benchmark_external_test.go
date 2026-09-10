package googleidentity_test

import (
	"github.com/deliri/primitive/v2026/googleidentity"
	"strings"
	"testing"
)

const audienceBenchmarkBatch = 64

func BenchmarkAudienceParseBatch(b *testing.B) {
	text := strings.Repeat("é", 256)
	want, err := googleidentity.ParseAudience(text)
	if err != nil || want.String() != text {
		b.Fatalf("fixture error=%v, want exact audience", err)
	}
	b.ReportAllocs()
	for b.Loop() {
		for range audienceBenchmarkBatch {
			got, err := googleidentity.ParseAudience(text)
			if err != nil || got != want {
				b.Fatalf("audience got=%v error=%v, want %v", got, err, want)
			}
		}
	}
	b.ReportMetric(audienceBenchmarkBatch, "parses/op")
}
func BenchmarkAudienceStringBatch(b *testing.B) {
	text := strings.Repeat("é", 256)
	value, err := googleidentity.ParseAudience(text)
	if err != nil || value.String() != text {
		b.Fatalf("fixture error=%v, want exact audience", err)
	}
	b.ReportAllocs()
	for b.Loop() {
		for range audienceBenchmarkBatch {
			got := value.String()
			if got != text {
				b.Fatalf("audience text got=%q, want %q", got, text)
			}
		}
	}
	b.ReportMetric(audienceBenchmarkBatch, "strings/op")
}
func BenchmarkCommandTokenDisclosure(b *testing.B) {
	text := strings.Repeat("a", 4096)
	input := []byte(text + "\r\n")
	want := "Bearer " + text
	b.ReportAllocs()
	for b.Loop() {
		token, err := googleidentity.ParseGoogleCloudCommandOutput(input)
		if err != nil {
			b.Fatal(err)
		}
		got, err := token.BearerValue()
		if err != nil || got != want {
			b.Fatalf("disclosure equal=%t error=%v, want exact token", got == want, err)
		}
	}
}
