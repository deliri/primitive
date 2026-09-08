package release_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/release"
)

// Each iteration decodes one fixed document and destroys the preceding owned
// result. The final live result is compared with the source outside timing.
func BenchmarkMaterialResponseDecodeOwnedLifetime(b *testing.B) {
	fixture, _ := materialResponseFixture(b)
	input, err := fixture.MarshalJSON()
	if err != nil {
		b.Fatalf("MaterialResponse.MarshalJSON(fixture) error = %v, want nil", err)
	}
	var got release.MaterialResponse
	b.ReportAllocs()
	b.SetBytes(int64(len(input)))
	for b.Loop() {
		if err := got.Destroy(); err != nil {
			b.Fatalf("MaterialResponse.Destroy(previous) error = %v, want nil", err)
		}
		if err := got.UnmarshalJSON(input); err != nil {
			b.Fatalf("MaterialResponse.UnmarshalJSON() error = %v, want nil", err)
		}
	}
	encoded, err := got.MarshalJSON()
	destroyErr := got.Destroy()
	if err != nil || destroyErr != nil || !bytes.Equal(encoded, input) || got != (release.MaterialResponse{}) {
		b.Fatalf("material decode lifetime = (%d bytes, %v, %v), want exact input and destroyed zero result", len(encoded), err, destroyErr)
	}
}

func BenchmarkReleaseSigningSeedDecodeOwnedLifetime(b *testing.B) {
	fixture, _ := materialResponseFixture(b)
	input, err := fixture.ReleaseSigningSeed.MarshalJSON()
	if err != nil {
		b.Fatalf("ReleaseSigningSeed.MarshalJSON(fixture) error = %v, want nil", err)
	}
	var got release.ReleaseSigningSeed
	b.ReportAllocs()
	b.SetBytes(int64(len(input)))
	for b.Loop() {
		if got != (release.ReleaseSigningSeed{}) {
			if err := got.Destroy(); err != nil {
				b.Fatalf("ReleaseSigningSeed.Destroy(previous) error = %v, want nil", err)
			}
		}
		got = release.ReleaseSigningSeed{}
		if err := got.UnmarshalJSON(input); err != nil {
			b.Fatalf("ReleaseSigningSeed.UnmarshalJSON() error = %v, want nil", err)
		}
	}
	encoded, err := got.MarshalJSON()
	destroyErr := got.Destroy()
	if err != nil || destroyErr != nil || !bytes.Equal(encoded, input) {
		b.Fatalf("seed decode lifetime = (%d bytes, %v, %v), want exact input and clean destruction", len(encoded), err, destroyErr)
	}
	if !errors.Is(got.Validate(), core.ErrReleaseContract) {
		b.Fatalf("destroyed seed Validate() = nil, want %v", core.ErrReleaseContract)
	}
}
