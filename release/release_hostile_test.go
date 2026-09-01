package release

import (
	"crypto/sha256"
	json "encoding/json/v2"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestArtifactSetRejectsEveryBrokenSlotContract(t *testing.T) {
	t.Parallel()
	fixture := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 1)

	for index := range TargetCount {
		target, ok := Targets().At(index)
		if !ok {
			t.Fatalf("Targets().At(%d) ok = false, want true", index)
		}
		t.Run(target.String()+"-slot", func(t *testing.T) {
			t.Parallel()
			duplicate := fixture.artifacts
			duplicate[index] = fixture.artifacts[(index+1)%TargetCount]
			_, err := NewArtifactSet(ArtifactSetRequest{Artifacts: duplicate})
			if !errors.Is(err, core.ErrReleaseManifest) {
				t.Fatalf("NewArtifactSet(duplicate slot %d) error = %v, want %v", index, err, core.ErrReleaseManifest)
			}

			divergent := fixture.artifacts
			build := fixture.builds[index]
			changed, err := core.NewBuildIdentity(core.BuildIdentityRequest{
				Offering: build.Offering(),
				Version:  core.NewReleaseVersion(2026, 7, 31),
				Commit:   build.Commit(),
				Platform: build.Platform(),
			})
			if err != nil {
				t.Fatalf("core.NewBuildIdentity(divergent slot %d) error = %v", index, err)
			}
			sum := sha256.Sum256([]byte{byte(index + 50)})
			divergent[index], err = NewArtifact(ArtifactRequest{
				Build: changed, Extent: mustByteCount(t, 1),
				SHA256: core.NewSHA256Digest(sum), CRC32C: core.NewCRC32C(1),
			})
			if err != nil {
				t.Fatalf("NewArtifact(divergent slot %d) error = %v", index, err)
			}
			_, err = NewArtifactSet(ArtifactSetRequest{Artifacts: divergent})
			if !errors.Is(err, core.ErrReleaseManifest) {
				t.Fatalf("NewArtifactSet(divergent slot %d) error = %v, want %v", index, err, core.ErrReleaseManifest)
			}
		})
	}
}

func TestManifestTotalExtentChecksOverflowAndSignedProjection(t *testing.T) {
	t.Parallel()
	fixture := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 1)
	got := fixture.verified.TotalExtent()
	want, err := core.NewByteCount(1 + 2 + 3 + 4)
	if err != nil {
		t.Fatalf("NewByteCount(total) error = %v", err)
	}
	if got != want {
		t.Fatalf("VerifiedManifest.TotalExtent() = %v, want %v", got, want)
	}

	artifacts := fixture.artifacts
	for index := range artifacts {
		artifacts[index], err = NewArtifact(ArtifactRequest{
			Build:  artifacts[index].Build(),
			Extent: mustByteCount(t, math.MaxUint64),
			SHA256: artifacts[index].Integrity().SHA256(),
			CRC32C: artifacts[index].Integrity().CRC32C(),
		})
		if err != nil {
			t.Fatalf("NewArtifact(max extent %d) error = %v", index, err)
		}
	}
	_, err = NewArtifactSet(ArtifactSetRequest{Artifacts: artifacts})
	if !errors.Is(err, core.ErrNumericOverflow) || !errors.Is(err, core.ErrReleaseManifest) {
		t.Fatalf("NewArtifactSet(overflow) error = %v, want %v and %v", err, core.ErrNumericOverflow, core.ErrReleaseManifest)
	}
}

func TestSignedDocumentsRejectTamperingAndLooseJSON(t *testing.T) {
	t.Parallel()
	fixture := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 1)

	tampered := fixture.manifest
	tampered.Fact = ManifestFact{}
	_, err := VerifyManifest(VerifyManifestRequest{
		Document: tampered, TrustedKeys: fixture.manifestTrust,
		ExpectedOffering: releaseOffering(t, 2),
	})
	if !errors.Is(err, core.ErrReleaseVerification) {
		t.Fatalf("VerifyManifest(tampered fact) error = %v, want %v", err, core.ErrReleaseVerification)
	}

	encoded, err := json.Marshal(fixture.latest)
	if err != nil {
		t.Fatalf("json.Marshal(LatestDocument) error = %v", err)
	}
	var roundTrip LatestDocument
	if err := json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatalf("json.Unmarshal(LatestDocument) error = %v", err)
	}
	if roundTrip != fixture.latest {
		t.Fatalf("LatestDocument round trip differs")
	}

	var receiver LatestDocument
	err = json.Unmarshal([]byte(`{"fact":{},"attestation":{},"future":true}`), &receiver)
	if !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("json.Unmarshal(unknown field) error = %v, want %v", err, core.ErrJSONContract)
	}
	if receiver != (LatestDocument{}) {
		t.Fatalf("json.Unmarshal(unknown field) mutated receiver")
	}
}

func TestLatestLifetimeAndFreshnessBoundaries(t *testing.T) {
	t.Parallel()
	fixture := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 1)
	fact := fixture.verifiedLatest.Fact()

	cases := []struct {
		wantErr   error
		name      string
		at        int64
		want      LatestFreshness
		wantClock LatestClockState
	}{
		{name: "one nanosecond before tolerance is corrected to signed issue", at: 2_000 - int64(ReleaseClockRollbackToleranceNanoseconds) + 1, want: LatestFreshnessCurrent, wantClock: LatestClockCorrected},
		{name: "exact tolerance is corrected to signed issue", at: 2_000 - int64(ReleaseClockRollbackToleranceNanoseconds), want: LatestFreshnessCurrent, wantClock: LatestClockCorrected},
		{name: "one nanosecond beyond tolerance is rejected", at: 2_000 - int64(ReleaseClockRollbackToleranceNanoseconds) - 1, wantErr: core.ErrReleaseLatest},
		{name: "exact valid from is current", at: 2_000, want: LatestFreshnessCurrent, wantClock: LatestClockObserved},
		{name: "exact valid until is expired", at: fact.ValidUntilNanoseconds(t), want: LatestFreshnessExpired, wantClock: LatestClockObserved},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := AssessLatest(AssessLatestRequest{
				Latest: fixture.verifiedLatest, Time: latestTimeEvidenceAt(t, tc.at),
			})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("AssessLatest(%d) error = %v, want %v", tc.at, err, tc.wantErr)
			}
			if err == nil && (got.Freshness() != tc.want || got.ClockState() != tc.wantClock) {
				t.Fatalf("AssessLatest(%d) = (%v, %v), want (%v, %v)", tc.at, got.Freshness(), got.ClockState(), tc.want, tc.wantClock)
			}
		})
	}
}

func TestLatestFreshnessCannotRewindBehindDurableOrElapsedTime(t *testing.T) {
	t.Parallel()

	fixture := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 1)
	fact := fixture.verifiedLatest.Fact()

	t.Run("wall after issuance cannot rewind durable high-water beyond tolerance", func(t *testing.T) {
		t.Parallel()

		timeEvidence := latestTimeEvidenceAt(t, 3_000)
		timeEvidence.DurableHighWater = fact.ValidUntil()
		_, gotErr := AssessLatest(AssessLatestRequest{
			Latest: fixture.verifiedLatest, Time: timeEvidence,
		})
		if !errors.Is(gotErr, core.ErrReleaseLatest) {
			t.Fatalf("AssessLatest(rewound wall) error = %v, want %v", gotErr, core.ErrReleaseLatest)
		}
	})

	t.Run("wall after issuance is corrected to an expired durable high-water within tolerance", func(t *testing.T) {
		t.Parallel()

		observedAt := fact.ValidUntilNanoseconds(t) - int64(ReleaseClockRollbackToleranceNanoseconds)
		timeEvidence := latestTimeEvidenceAt(t, observedAt)
		timeEvidence.DurableHighWater = fact.ValidUntil()
		got, gotErr := AssessLatest(AssessLatestRequest{
			Latest: fixture.verifiedLatest, Time: timeEvidence,
		})
		if gotErr != nil || got.Freshness() != LatestFreshnessExpired ||
			got.ClockState() != LatestClockCorrected || got.EffectiveAt() != fact.ValidUntil() {
			t.Fatalf("AssessLatest(correctable wall) = (%v, %v, %v, %v), want expired, corrected, %v, nil",
				got.Freshness(), got.ClockState(), got.EffectiveAt(), gotErr, fact.ValidUntil())
		}
	})

	t.Run("elapsed progress advances a frozen wall observation", func(t *testing.T) {
		t.Parallel()

		started, err := temporal.NewObservation(time.Unix(0, 0).UTC())
		if err != nil {
			t.Fatalf("temporal.NewObservation(started) error = %v, want nil", err)
		}
		observed, err := temporal.NewObservation(time.Unix(0, 1_000).UTC())
		if err != nil {
			t.Fatalf("temporal.NewObservation(observed) error = %v, want nil", err)
		}
		wall := temporal.InstantFromNanoseconds(3_000)
		started, err = started.WithWall(wall)
		if err != nil {
			t.Fatalf("started.WithWall() error = %v, want nil", err)
		}
		observed, err = observed.WithWall(wall)
		if err != nil {
			t.Fatalf("observed.WithWall() error = %v, want nil", err)
		}
		got, gotErr := AssessLatest(AssessLatestRequest{
			Latest: fixture.verifiedLatest,
			Time: LatestTimeEvidence{
				StartedAt: started, ObservedAt: observed,
				DurableHighWater: wall,
			},
		})
		wantEffective := temporal.InstantFromNanoseconds(4_000)
		if gotErr != nil || got.EffectiveAt() != wantEffective ||
			got.ClockState() != LatestClockCorrected {
			t.Fatalf(
				"AssessLatest(frozen wall) = (%v, %v, %v), want (%v, %v, nil)",
				got.EffectiveAt(),
				got.ClockState(),
				gotErr,
				wantEffective,
				LatestClockCorrected,
			)
		}
	})
}

func (f LatestFact) ValidUntilNanoseconds(t *testing.T) int64 {
	t.Helper()
	got, err := f.ValidUntil().Nanoseconds()
	if err != nil {
		t.Fatalf("LatestFact.ValidUntil().Nanoseconds() error = %v", err)
	}
	return got
}
