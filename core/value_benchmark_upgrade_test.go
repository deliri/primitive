package core

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"math"
	"net/url"
	"strings"
	"testing"
)

func BenchmarkReleaseVersionCompare(b *testing.B) {
	left := NewReleaseVersion(math.MaxUint32, math.MaxUint32, math.MaxUint32-1)
	right := NewReleaseVersion(math.MaxUint32, math.MaxUint32, math.MaxUint32)
	b.ReportAllocs()
	var got Comparison
	var err error
	for b.Loop() {
		got, err = left.Compare(right)
		if err != nil {
			b.Fatal(err)
		}
	}
	if got != ComparisonLess {
		b.Fatalf("comparison=%v; want %v", got, ComparisonLess)
	}
}

func BenchmarkHTTPEndpointProjection(b *testing.B) {
	b.ReportAllocs()
	cases := []struct{ name, path string }{
		{name: "ASCII", path: strings.Repeat("a", 1024)},
		{name: "Escaped", path: strings.Repeat(" ", 1024)},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			source := "https://example.invalid/" + tc.path
			parsed, err := url.Parse(source)
			if err != nil {
				b.Fatal(err)
			}
			want := parsed.String()
			b.ReportAllocs()
			var got HTTPEndpoint
			for b.Loop() {
				got, err = ParseHTTPEndpoint(source)
				if err != nil {
					b.Fatal(err)
				}
			}
			if got.String() != want {
				b.Fatalf("endpoint=%q; want %q", got.String(), want)
			}
		})
	}
}

func BenchmarkByteCountRejectOversize(b *testing.B) {
	wire := bytes.Repeat([]byte{'0'}, JSONDocumentMaximumBytes)
	original, err := NewByteCount(7)
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(wire)))
	b.ReportAllocs()
	got := original
	for b.Loop() {
		err = got.UnmarshalJSON(wire)
		if !errors.Is(err, ErrJSONContract) {
			b.Fatalf("oversize integer error=%v; want %v", err, ErrJSONContract)
		}
	}
	if got != original {
		b.Fatalf("refused receiver=%v; want unchanged %v", got, original)
	}
}

type benchmarkOwnedProjection struct {
	Value string `json:"value"`
}

func (benchmarkOwnedProjection) Validate() error { return nil }

type benchmarkOwnedProjectionWire benchmarkOwnedProjection

func (v benchmarkOwnedProjection) MarshalJSON() ([]byte, error) {
	return json.Marshal(benchmarkOwnedProjectionWire(v))
}
func (v benchmarkOwnedProjection) ValidateJSONProjection(wire []byte, limits StrictJSONLimits) error {
	type record struct {
		Value string `json:"value"`
	}
	got, err := DecodeStrictJSONStructure[record](wire, limits)
	if err != nil {
		return err
	}
	if got.Value != v.Value {
		return ErrJSONContract
	}
	return nil
}

func BenchmarkEncodeValidatedJSONProjection(b *testing.B) {
	value := benchmarkOwnedProjection{Value: strings.Repeat("é<&", 64)}
	// A distinct wire struct avoids recursing through the projection's method.
	want, err := json.Marshal(struct {
		Value string `json:"value"`
	}{Value: value.Value})
	if err != nil {
		b.Fatal(err)
	}
	limits := DefaultStrictJSONLimits()
	var got []byte
	b.ReportAllocs()
	for b.Loop() {
		got, err = EncodeValidatedJSON(value, limits)
		if err != nil {
			b.Fatal(err)
		}
	}
	if !bytes.Equal(got, want) {
		b.Fatalf("projection=%q; want %q", got, want)
	}
}
