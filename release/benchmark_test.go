package release

import (
	json "encoding/json/v2"
	"runtime"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
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
	runtime.KeepAlive(got)
	runtime.KeepAlive(err)
}

func BenchmarkAssessLatest(b *testing.B) {
	fixture := newReleaseFixture(b, core.NewReleaseVersion(2026, 7, 30), 1)
	request := AssessLatestRequest{
		Latest:      fixture.verifiedLatest,
		Observation: temporal.InstantFromNanoseconds(3_000),
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
	runtime.KeepAlive(got)
	runtime.KeepAlive(err)
}

func BenchmarkLatestDocumentJSON(b *testing.B) {
	fixture := newReleaseFixture(b, core.NewReleaseVersion(2026, 7, 30), 1)
	b.ReportAllocs()

	var got []byte
	var err error
	for b.Loop() {
		got, err = json.Marshal(fixture.latest)
	}
	runtime.KeepAlive(got)
	runtime.KeepAlive(err)
}
