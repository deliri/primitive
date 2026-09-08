package release

import (
	json "encoding/json/v2"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func BenchmarkVerifyLatest(b *testing.B) {
	fixture := newReleaseFixture(b, core.NewReleaseVersion(2026, 7, 30), 1)
	request := VerifyLatestRequest{
		Document: fixture.latest, LatestKeys: fixture.latestTrust,
		ManifestKeys:     fixture.manifestTrust,
		ExpectedOffering: releaseOffering(b, 2),
	}
	b.ReportAllocs()

	var got VerifiedLatest
	var err error
	for b.Loop() {
		got, err = VerifyLatest(request)
	}
	if err != nil || got.Document() != request.Document || got.Validate() != nil {
		b.Fatalf("VerifyLatest() = (%v, %v), want exact verified document and nil", got, err)
	}
}

func BenchmarkAssessLatest(b *testing.B) {
	fixture := newReleaseFixture(b, core.NewReleaseVersion(2026, 7, 30), 1)
	request := AssessLatestRequest{
		Latest: fixture.verifiedLatest,
		Time:   latestTimeEvidenceAt(b, 3_000),
	}
	var got LatestAssessment
	var err error
	allocations := testing.AllocsPerRun(100, func() {
		got, err = AssessLatest(request)
	})
	if err != nil {
		b.Fatalf("AssessLatest(allocation ratchet) error = %v, want nil", err)
	}
	if got.Freshness() != LatestFreshnessCurrent {
		b.Fatalf(
			"AssessLatest(allocation ratchet).Freshness() = %v, want %v",
			got.Freshness(),
			LatestFreshnessCurrent,
		)
	}
	if allocations != 0 {
		b.Fatalf("AssessLatest() allocations = %v, want 0", allocations)
	}
	b.ReportAllocs()

	for b.Loop() {
		got, err = AssessLatest(request)
	}
	if err != nil || got.Validate() != nil || got.Freshness() != LatestFreshnessCurrent ||
		got.EffectiveAt() != request.Time.DurableHighWater || got.ValidUntil() != request.Latest.Fact().ValidUntil() {
		b.Fatalf("AssessLatest() = (%v, %v), want exact current assessment and nil", got, err)
	}
}

func BenchmarkLatestDocumentJSON(b *testing.B) {
	fixture := newReleaseFixture(b, core.NewReleaseVersion(2026, 7, 30), 1)
	b.ReportAllocs()

	var got []byte
	var err error
	for b.Loop() {
		got, err = json.Marshal(fixture.latest)
	}
	if err != nil || len(got) == 0 || len(got) > documentExtentMaximum {
		b.Fatalf("json.Marshal(LatestDocument) = (%d bytes, %v), want bounded nonempty document and nil", len(got), err)
	}
	var decoded LatestDocument
	if err := decoded.UnmarshalJSON(got); err != nil || decoded != fixture.latest {
		b.Fatalf("LatestDocument.UnmarshalJSON(benchmark output) = (%v, %v), want exact fixture and nil", decoded, err)
	}
}

func BenchmarkBuildDependenciesUnmarshalSparse(b *testing.B) {
	encoded := mustDependencyDocument(b, 1)
	var wantErr error
	b.ReportAllocs()
	var last BuildDependencies
	for b.Loop() {
		var got BuildDependencies
		err := got.UnmarshalJSON(encoded)
		if !errors.Is(err, wantErr) {
			b.Fatalf("BuildDependencies.UnmarshalJSON(sparse) error = %v, want %v", err, wantErr)
		}
		last = got
	}
	if last.Count() != 1 {
		b.Fatalf("BuildDependencies.UnmarshalJSON(sparse).Count() = %d, want 1", last.Count())
	}
}

func BenchmarkBuildDependenciesUnmarshalMaximum(b *testing.B) {
	encoded := mustDependencyDocument(b, BuildDependencyMaximumCount)
	var wantErr error
	b.ReportAllocs()
	var last BuildDependencies
	for b.Loop() {
		var got BuildDependencies
		err := got.UnmarshalJSON(encoded)
		if !errors.Is(err, wantErr) {
			b.Fatalf("BuildDependencies.UnmarshalJSON(maximum) error = %v, want %v", err, wantErr)
		}
		last = got
	}
	if last.Count() != BuildDependencyMaximumCount {
		b.Fatalf("BuildDependencies.UnmarshalJSON(maximum).Count() = %d, want %d", last.Count(), BuildDependencyMaximumCount)
	}
}

func mustDependencyDocument(b *testing.B, count int) []byte {
	b.Helper()

	value, err := newBuildDependencies(
		mustModulePath(b, testMainModule),
		CurrentGoToolchain(),
		numberedModules(b, count),
	)
	if err != nil {
		b.Fatalf("newBuildDependencies(%d) error = %v, want nil", count, err)
	}
	encoded, err := value.MarshalJSON()
	if err != nil {
		b.Fatalf("BuildDependencies.MarshalJSON(%d) error = %v, want nil", count, err)
	}
	return encoded
}
