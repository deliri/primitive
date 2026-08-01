package release

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestReleaseEnumsExhaustCompleteByteDomains(t *testing.T) {
	t.Parallel()

	for value := range 256 {
		revision := Revision(value)
		wantRevision := revision == Revision2026V1
		if revision.IsValid() != wantRevision ||
			(revision.String() != unknownDiagnostic) != wantRevision {
			t.Fatalf("Revision(%d) = (%v, %q), want validity %v", value, revision.IsValid(), revision.String(), wantRevision)
		}

		domain := Domain(value)
		wantDomain := domain == DomainManifestV1 || domain == DomainLatestV1
		if domain.IsValid() != wantDomain ||
			(domain.String() != unknownDiagnostic) != wantDomain {
			t.Fatalf("Domain(%d) = (%v, %q), want validity %v", value, domain.IsValid(), domain.String(), wantDomain)
		}

		freshness := LatestFreshness(value)
		wantFreshness := freshness >= LatestFreshnessNotYetValid && freshness <= LatestFreshnessExpired
		if freshness.IsValid() != wantFreshness ||
			(freshness.String() != unknownDiagnostic) != wantFreshness {
			t.Fatalf("LatestFreshness(%d) = (%v, %q), want validity %v", value, freshness.IsValid(), freshness.String(), wantFreshness)
		}

		clock := LatestClockState(value)
		wantClock := clock == LatestClockObserved || clock == LatestClockCorrected
		if clock.IsValid() != wantClock ||
			(clock.String() != unknownDiagnostic) != wantClock {
			t.Fatalf("LatestClockState(%d) = (%v, %q), want validity %v", value, clock.IsValid(), clock.String(), wantClock)
		}

		advance := LatestAdvanceState(value)
		wantAdvance := advance == LatestAdvanceReplay || advance == LatestAdvanceAdvanced
		if advance.IsValid() != wantAdvance ||
			(advance.String() != unknownDiagnostic) != wantAdvance {
			t.Fatalf("LatestAdvanceState(%d) = (%v, %q), want validity %v", value, advance.IsValid(), advance.String(), wantAdvance)
		}

		cache := CachedLatestState(value)
		wantCache := cache == CachedLatestMissing || cache == CachedLatestPresent
		if cache.IsValid() != wantCache ||
			(cache.String() != unknownDiagnostic) != wantCache {
			t.Fatalf("CachedLatestState(%d) = (%v, %q), want validity %v", value, cache.IsValid(), cache.String(), wantCache)
		}

		selection := SelectionState(value)
		wantSelection := selection >= SelectionCurrent && selection <= SelectionReassessAt
		if selection.IsValid() != wantSelection ||
			(selection.String() != unknownDiagnostic) != wantSelection {
			t.Fatalf("SelectionState(%d) = (%v, %q), want validity %v", value, selection.IsValid(), selection.String(), wantSelection)
		}
	}
}

func TestGenerationJSONPressuresNumericBoundaries(t *testing.T) {
	t.Parallel()

	valid := []uint64{1, 2, math.MaxUint32, math.MaxUint64 - 1, math.MaxUint64}
	for _, value := range valid {
		t.Run(strconv.FormatUint(value, 10), func(t *testing.T) {
			t.Parallel()
			generation, err := NewGeneration(value)
			if err != nil {
				t.Fatalf("NewGeneration(%d) error = %v", value, err)
			}
			encoded, err := json.Marshal(generation)
			if err != nil {
				t.Fatalf("json.Marshal(Generation(%d)) error = %v", value, err)
			}
			var got Generation
			if err := json.Unmarshal(encoded, &got); err != nil {
				t.Fatalf("json.Unmarshal(Generation(%d)) error = %v", value, err)
			}
			if got != generation || got.Uint64() != value {
				t.Fatalf("Generation(%d) round trip = %v/%d, want %v/%d", value, got, got.Uint64(), generation, value)
			}
		})
	}

	hostile := []struct {
		data         string
		wantContract bool
	}{
		{data: `0`, wantContract: true},
		{data: `-1`, wantContract: true},
		{data: `1.0`, wantContract: true},
		{data: `01`},
		{data: `+1`},
		{data: `"1"`, wantContract: true},
		{data: `null`, wantContract: true},
		{data: `true`, wantContract: true},
		{data: `18446744073709551616`, wantContract: true},
		{data: ``},
		{data: `{}`, wantContract: true},
		{data: `[]`, wantContract: true},
	}
	for _, tc := range hostile {
		receiver := mustGeneration(t, 9)
		err := json.Unmarshal([]byte(tc.data), &receiver)
		if err == nil || tc.wantContract && !errors.Is(err, core.ErrJSONContract) {
			t.Fatalf("json.Unmarshal(Generation %q) error = %v, want rejection with contract=%v", tc.data, err, tc.wantContract)
		}
		if receiver != mustGeneration(t, 9) {
			t.Fatalf("json.Unmarshal(Generation %q) mutated receiver", tc.data)
		}
	}
	for _, data := range []string{` 1`, "1\n"} {
		var got Generation
		if err := json.Unmarshal([]byte(data), &got); err != nil || got.Uint64() != 1 {
			t.Fatalf("json.Unmarshal(Generation %q) = (%d, %v), want (1, nil)", data, got.Uint64(), err)
		}
	}

	var receiver *Generation
	if err := receiver.UnmarshalJSON([]byte(`1`)); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil Generation.UnmarshalJSON() error = %v, want %v", err, core.ErrJSONContract)
	}
	if _, err := json.Marshal(Generation{}); !errors.Is(err, core.ErrReleaseLatest) {
		t.Fatalf("json.Marshal(zero Generation) error = %v, want %v", err, core.ErrReleaseLatest)
	}
}

func TestLatestTimelineConstructionPressuresEveryBoundary(t *testing.T) {
	t.Parallel()
	fixture := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 1)

	cases := []struct {
		wantErr    error
		name       string
		issuedAt   int64
		validFrom  int64
		validUntil int64
	}{
		{name: "one nanosecond lifetime is accepted", issuedAt: 1, validFrom: 1, validUntil: 2},
		{name: "issue may precede validity", issuedAt: 1, validFrom: 2, validUntil: 3},
		{name: "exact maximum lifetime is accepted", issuedAt: 1, validFrom: 1, validUntil: 1 + ReleaseLatestMaximumLifetimeNanoseconds},
		{name: "one below maximum lifetime is accepted", issuedAt: 1, validFrom: 1, validUntil: ReleaseLatestMaximumLifetimeNanoseconds},
		{name: "issue one nanosecond after validity is rejected", issuedAt: 2, validFrom: 1, validUntil: 3, wantErr: core.ErrReleaseLatest},
		{name: "empty validity is rejected", issuedAt: 1, validFrom: 1, validUntil: 1, wantErr: core.ErrReleaseLatest},
		{name: "reversed validity is rejected", issuedAt: 1, validFrom: 2, validUntil: 1, wantErr: core.ErrReleaseLatest},
		{name: "one above maximum lifetime is rejected", issuedAt: 1, validFrom: 1, validUntil: 2 + ReleaseLatestMaximumLifetimeNanoseconds, wantErr: core.ErrReleaseLatest},
	}
	for index, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := IssueLatest(IssueLatestRequest{
				Manifest: fixture.verified, Generation: mustGeneration(t, uint64(index+1)),
				IssuedAt:   temporal.InstantFromNanoseconds(tc.issuedAt),
				ValidFrom:  temporal.InstantFromNanoseconds(tc.validFrom),
				ValidUntil: temporal.InstantFromNanoseconds(tc.validUntil),
				Key:        fixture.latestKey,
			})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("IssueLatest(%s) error = %v, want %v", tc.name, err, tc.wantErr)
			}
		})
	}
}

func TestAdvanceLatestRejectsEveryMonotonicRegression(t *testing.T) {
	t.Parallel()
	fixture := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 1)
	retained := issueVerifiedLatest(t, fixture, fixture.verified, 10, 1_000, 2_000, 10_000)

	cases := []struct {
		wantErr    error
		name       string
		generation uint64
		issuedAt   int64
		validFrom  int64
		validUntil int64
	}{
		{name: "lower generation rolls back", generation: 9, issuedAt: 1_000, validFrom: 2_000, validUntil: 10_000, wantErr: core.ErrReleaseRollback},
		{name: "higher generation cannot roll back issue", generation: 11, issuedAt: 999, validFrom: 2_000, validUntil: 10_000, wantErr: core.ErrReleaseRollback},
		{name: "higher generation cannot roll back valid from", generation: 11, issuedAt: 1_000, validFrom: 1_999, validUntil: 10_000, wantErr: core.ErrReleaseRollback},
		{name: "higher generation cannot roll back valid until", generation: 11, issuedAt: 1_000, validFrom: 2_000, validUntil: 9_999, wantErr: core.ErrReleaseRollback},
		{name: "higher generation may extend exact manifest", generation: 11, issuedAt: 1_001, validFrom: 2_001, validUntil: 10_001},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			proposed := issueVerifiedLatest(t, fixture, fixture.verified, tc.generation, tc.issuedAt, tc.validFrom, tc.validUntil)
			got, err := AdvanceLatest(AdvanceLatestRequest{Retained: retained, Proposed: proposed})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("AdvanceLatest(%s) error = %v, want %v", tc.name, err, tc.wantErr)
			}
			if err == nil && got.State() != LatestAdvanceAdvanced {
				t.Fatalf("AdvanceLatest(%s) state = %v, want %v", tc.name, got.State(), LatestAdvanceAdvanced)
			}
		})
	}

	olderVersion := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 29), 1)
	proposed := issueVerifiedLatest(t, olderVersion, olderVersion.verified, 11, 1_001, 2_001, 10_001)
	_, err := AdvanceLatest(AdvanceLatestRequest{Retained: retained, Proposed: proposed})
	if !errors.Is(err, core.ErrReleaseRollback) {
		t.Fatalf("AdvanceLatest(version rollback) error = %v, want %v", err, core.ErrReleaseRollback)
	}
}

func TestAdvanceLatestTreatsSignerRotationAsDocumentIdentity(t *testing.T) {
	t.Parallel()
	fixture := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 5)

	rotatedLatestKey := deterministicKey(61)
	latestTrust := trustedKeys(t, fixture.latestKey, rotatedLatestKey)
	rotatedLatestDocument, err := IssueLatest(IssueLatestRequest{
		Manifest: fixture.verified, Generation: mustGeneration(t, 5),
		IssuedAt:   temporal.InstantFromNanoseconds(2_000),
		ValidFrom:  temporal.InstantFromNanoseconds(2_000),
		ValidUntil: temporal.InstantFromNanoseconds(2_000 + ReleaseLatestMaximumLifetimeNanoseconds),
		Key:        rotatedLatestKey,
	})
	if err != nil {
		t.Fatalf("IssueLatest(rotated signer) error = %v", err)
	}
	rotatedLatest, err := VerifyLatest(VerifyLatestRequest{
		Document: rotatedLatestDocument, LatestKeys: latestTrust,
		ManifestKeys: fixture.manifestTrust, ExpectedOffering: core.OfferingWitness,
	})
	if err != nil {
		t.Fatalf("VerifyLatest(rotated signer) error = %v", err)
	}
	_, err = AdvanceLatest(AdvanceLatestRequest{
		Retained: fixture.verifiedLatest, Proposed: rotatedLatest,
	})
	if !errors.Is(err, core.ErrReleaseConflict) {
		t.Fatalf("AdvanceLatest(equal generation re-signed fact) error = %v, want %v", err, core.ErrReleaseConflict)
	}

	rotatedManifestKey := deterministicKey(73)
	manifestTrust := trustedKeys(t, fixture.manifestKey, rotatedManifestKey)
	rotatedManifestDocument, err := IssueManifest(IssueManifestRequest{
		Fact: fixture.manifest.Fact, Key: rotatedManifestKey,
	})
	if err != nil {
		t.Fatalf("IssueManifest(rotated signer) error = %v", err)
	}
	rotatedManifest, err := VerifyManifest(VerifyManifestRequest{
		Document: rotatedManifestDocument, TrustedKeys: manifestTrust,
		ExpectedOffering: core.OfferingWitness,
	})
	if err != nil {
		t.Fatalf("VerifyManifest(rotated signer) error = %v", err)
	}
	proposedDocument, err := IssueLatest(IssueLatestRequest{
		Manifest: rotatedManifest, Generation: mustGeneration(t, 6),
		IssuedAt:   temporal.InstantFromNanoseconds(2_001),
		ValidFrom:  temporal.InstantFromNanoseconds(2_001),
		ValidUntil: temporal.InstantFromNanoseconds(2_001 + ReleaseLatestMaximumLifetimeNanoseconds),
		Key:        fixture.latestKey,
	})
	if err != nil {
		t.Fatalf("IssueLatest(rotated manifest) error = %v", err)
	}
	proposed, err := VerifyLatest(VerifyLatestRequest{
		Document: proposedDocument, LatestKeys: fixture.latestTrust,
		ManifestKeys: manifestTrust, ExpectedOffering: core.OfferingWitness,
	})
	if err != nil {
		t.Fatalf("VerifyLatest(rotated manifest) error = %v", err)
	}
	_, err = AdvanceLatest(AdvanceLatestRequest{
		Retained: fixture.verifiedLatest, Proposed: proposed,
	})
	if !errors.Is(err, core.ErrReleaseConflict) {
		t.Fatalf("AdvanceLatest(equal version re-signed manifest) error = %v, want %v", err, core.ErrReleaseConflict)
	}
}

func TestReleaseVerificationSeparatesEveryAuthority(t *testing.T) {
	t.Parallel()
	fixture := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 1)
	untrusted := deterministicKey(91)
	untrustedKeys := trustedKey(t, untrusted)

	_, err := VerifyManifest(VerifyManifestRequest{
		Document: fixture.manifest, TrustedKeys: untrustedKeys,
		ExpectedOffering: core.OfferingWitness,
	})
	if !errors.Is(err, core.ErrReleaseVerification) {
		t.Fatalf("VerifyManifest(untrusted key) error = %v, want %v", err, core.ErrReleaseVerification)
	}
	_, err = VerifyManifest(VerifyManifestRequest{
		Document: fixture.manifest, TrustedKeys: fixture.manifestTrust,
		ExpectedOffering: core.OfferingBug,
	})
	if !errors.Is(err, core.ErrReleaseVerification) {
		t.Fatalf("VerifyManifest(wrong offering) error = %v, want %v", err, core.ErrReleaseVerification)
	}
	_, err = VerifyLatest(VerifyLatestRequest{
		Document: fixture.latest, LatestKeys: untrustedKeys,
		ManifestKeys: fixture.manifestTrust, ExpectedOffering: core.OfferingWitness,
	})
	if !errors.Is(err, core.ErrReleaseVerification) {
		t.Fatalf("VerifyLatest(untrusted latest key) error = %v, want %v", err, core.ErrReleaseVerification)
	}
	_, err = VerifyLatest(VerifyLatestRequest{
		Document: fixture.latest, LatestKeys: fixture.latestTrust,
		ManifestKeys: untrustedKeys, ExpectedOffering: core.OfferingWitness,
	})
	if !errors.Is(err, core.ErrReleaseVerification) {
		t.Fatalf("VerifyLatest(untrusted manifest key) error = %v, want %v", err, core.ErrReleaseVerification)
	}
	_, err = VerifyLatest(VerifyLatestRequest{
		Document: fixture.latest, LatestKeys: fixture.latestTrust,
		ManifestKeys: fixture.manifestTrust, ExpectedOffering: core.OfferingBug,
	})
	if !errors.Is(err, core.ErrReleaseVerification) {
		t.Fatalf("VerifyLatest(wrong offering) error = %v, want %v", err, core.ErrReleaseVerification)
	}

	wrongDomain := fixture.latest
	wrongDomain.Attestation.Domain = DomainManifestV1
	if err := wrongDomain.Validate(); !errors.Is(err, core.ErrReleaseVerification) {
		t.Fatalf("LatestDocument(wrong domain).Validate() error = %v, want %v", err, core.ErrReleaseVerification)
	}
}

func TestSelectionAndPreparationExhaustActionBoundaries(t *testing.T) {
	t.Parallel()
	installed := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 1)
	candidate := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 31), 2)
	installedBuild := installed.builds[2]

	missing, err := evaluateWithInstalled(EvaluateRequest{
		InstalledManifest: installed.verified,
		Latest:            MissingCachedLatest(),
		Observation:       temporal.InstantFromNanoseconds(2_000),
	}, installedBuild)
	if err != nil || missing.State() != SelectionRefreshRequired {
		t.Fatalf("evaluateWithInstalled(missing) = (%v, %v), want (%v, nil)", missing.State(), err, SelectionRefreshRequired)
	}

	currentCache, err := NewCachedLatest(installed.verifiedLatest)
	if err != nil {
		t.Fatalf("NewCachedLatest(current) error = %v", err)
	}
	current, err := evaluateWithInstalled(EvaluateRequest{
		InstalledManifest: installed.verified, Latest: currentCache,
		Observation: temporal.InstantFromNanoseconds(2_000),
	}, installedBuild)
	if err != nil || current.State() != SelectionCurrent {
		t.Fatalf("evaluateWithInstalled(current) = (%v, %v), want (%v, nil)", current.State(), err, SelectionCurrent)
	}
	currentCapability, ok := current.Current()
	if !ok {
		t.Fatalf("Selection.Current() ok = false, state = %v", current.State())
	}
	currentSummary, err := currentCapability.Summary()
	if err != nil || currentSummary.Version != installedBuild.Version() ||
		currentSummary.ValidUntil != temporal.InstantFromNanoseconds(2_000+ReleaseLatestMaximumLifetimeNanoseconds) {
		t.Fatalf("CurrentRelease.Summary() = (%+v, %v), want installed version and signed validity", currentSummary, err)
	}

	futureLatest := issueVerifiedLatest(t, candidate, candidate.verified, 3, 1_000, 2_000, 10_000)
	futureCache, err := NewCachedLatest(futureLatest)
	if err != nil {
		t.Fatalf("NewCachedLatest(future) error = %v", err)
	}
	reassess, err := evaluateWithInstalled(EvaluateRequest{
		InstalledManifest: installed.verified, Latest: futureCache,
		Observation: temporal.InstantFromNanoseconds(1_500),
	}, installedBuild)
	if err != nil || reassess.State() != SelectionReassessAt {
		t.Fatalf("evaluateWithInstalled(future) = (%v, %v), want (%v, nil)", reassess.State(), err, SelectionReassessAt)
	}

	expired, err := evaluateWithInstalled(EvaluateRequest{
		InstalledManifest: installed.verified, Latest: futureCache,
		Observation: temporal.InstantFromNanoseconds(10_000),
	}, installedBuild)
	if err != nil || expired.State() != SelectionRefreshRequired {
		t.Fatalf("evaluateWithInstalled(expired) = (%v, %v), want (%v, nil)", expired.State(), err, SelectionRefreshRequired)
	}

	available, err := evaluateWithInstalled(EvaluateRequest{
		InstalledManifest: installed.verified, Latest: futureCache,
		Observation: temporal.InstantFromNanoseconds(2_000),
	}, installedBuild)
	if err != nil || available.State() != SelectionAvailable {
		t.Fatalf("evaluateWithInstalled(available) = (%v, %v), want (%v, nil)", available.State(), err, SelectionAvailable)
	}
	capability, ok := available.Available()
	if !ok {
		t.Fatalf("Selection.Available() ok = false, state = %v", available.State())
	}
	summary, err := capability.Summary()
	if err != nil {
		t.Fatalf("AvailableRelease.Summary() error = %v", err)
	}
	if summary.Installed != installedBuild ||
		summary.Candidate != candidate.builds[2] ||
		summary.Artifact != candidate.artifacts[2].Identity() ||
		summary.Integrity != candidate.artifacts[2].Integrity() {
		t.Fatalf("AvailableRelease.Summary() = %+v, want exact installed/candidate closure", summary)
	}
	tamperedSummary := summary
	tamperedSummary.Candidate = installedBuild
	if err := tamperedSummary.Validate(); !errors.Is(err, core.ErrReleaseConflict) {
		t.Fatalf("tampered AvailableSummary.Validate() error = %v, want %v", err, core.ErrReleaseConflict)
	}
	provePreparationState(t, capability, 1_999, SelectionReassessAt)
	provePreparationState(t, capability, 2_000, SelectionAvailable)
	provePreparationState(t, capability, 9_999, SelectionAvailable)
	provePreparationState(t, capability, 10_000, SelectionRefreshRequired)
	provePreparationState(t, capability, 10_001, SelectionRefreshRequired)

	newerInstalled := newReleaseFixture(t, core.NewReleaseVersion(2026, 8, 0), 1)
	_, err = evaluateWithInstalled(EvaluateRequest{
		InstalledManifest: newerInstalled.verified, Latest: futureCache,
		Observation: temporal.InstantFromNanoseconds(2_000),
	}, newerInstalled.builds[2])
	if !errors.Is(err, core.ErrReleaseRollback) {
		t.Fatalf("evaluateWithInstalled(installed newer) error = %v, want %v", err, core.ErrReleaseRollback)
	}
}

func TestEvaluatePublicBoundaryRejectsAnUnstampedBinary(t *testing.T) {
	t.Parallel()
	fixture := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 1)
	latest, err := NewCachedLatest(fixture.verifiedLatest)
	if err != nil {
		t.Fatalf("NewCachedLatest() error = %v", err)
	}
	got, err := Evaluate(EvaluateRequest{
		InstalledManifest: fixture.verified,
		Latest:            latest,
		Observation:       temporal.InstantFromNanoseconds(3_000),
	})
	if !errors.Is(err, core.ErrReleaseConflict) {
		t.Fatalf("Evaluate(unstamped binary) error = %v, want %v", err, core.ErrReleaseConflict)
	}
	if got != (Selection{}) {
		t.Fatalf("Evaluate(unstamped binary) = %v, want zero selection", got)
	}
}

func TestPreparedReleaseExposesOnlyValidatedExactHandoffFacts(t *testing.T) {
	t.Parallel()
	installed := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 1)
	candidate := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 31), 2)
	cached, err := NewCachedLatest(candidate.verifiedLatest)
	if err != nil {
		t.Fatalf("NewCachedLatest() error = %v", err)
	}
	selection, err := evaluateWithInstalled(EvaluateRequest{
		InstalledManifest: installed.verified, Latest: cached,
		Observation: temporal.InstantFromNanoseconds(3_000),
	}, installed.builds[2])
	if err != nil {
		t.Fatalf("evaluateWithInstalled() error = %v", err)
	}
	available, ok := selection.Available()
	if !ok {
		t.Fatalf("Selection.Available() ok = false, state = %v", selection.State())
	}
	preparation, err := available.PrepareAt(temporal.InstantFromNanoseconds(4_000))
	if err != nil {
		t.Fatalf("AvailableRelease.PrepareAt() error = %v", err)
	}
	prepared, ok := preparation.Ready()
	if !ok {
		t.Fatalf("Preparation.Ready() ok = false")
	}
	artifact, err := prepared.Artifact()
	if err != nil || artifact != candidate.artifacts[2] {
		t.Fatalf("PreparedRelease.Artifact() = (%v, %v), want exact candidate", artifact, err)
	}
	candidateManifest, err := prepared.CandidateManifest()
	if err != nil || candidateManifest != candidate.verified {
		t.Fatalf("PreparedRelease.CandidateManifest() differs or error = %v", err)
	}
	installedManifest, err := prepared.InstalledManifest()
	if err != nil || installedManifest != installed.verified {
		t.Fatalf("PreparedRelease.InstalledManifest() differs or error = %v", err)
	}
	latest, err := prepared.Latest()
	if err != nil || latest != candidate.verifiedLatest {
		t.Fatalf("PreparedRelease.Latest() differs or error = %v", err)
	}
	observation, err := prepared.Observation()
	if err != nil || observation != temporal.InstantFromNanoseconds(4_000) {
		t.Fatalf("PreparedRelease.Observation() = (%v, %v), want exact preparation observation", observation, err)
	}
	assessment, err := prepared.Assessment()
	if err != nil || assessment.Freshness() != LatestFreshnessCurrent ||
		assessment.EffectiveAt() != observation {
		t.Fatalf("PreparedRelease.Assessment() = (%v, %v), want current at observation", assessment, err)
	}

	spliced := prepared
	spliced.candidateManifest = installed.verified
	if err := spliced.Validate(); !errors.Is(err, core.ErrReleaseVerification) {
		t.Fatalf("spliced PreparedRelease.Validate() error = %v, want %v", err, core.ErrReleaseVerification)
	}
}

func TestZeroValueCapabilitiesRefuseEveryAccessor(t *testing.T) {
	t.Parallel()

	if err := (CachedLatest{}).Validate(); !errors.Is(err, core.ErrReleaseContract) {
		t.Fatalf("CachedLatest{}.Validate() error = %v, want %v", err, core.ErrReleaseContract)
	}
	if err := (LatestAdvance{}).Validate(); !errors.Is(err, core.ErrReleaseContract) {
		t.Fatalf("LatestAdvance{}.Validate() error = %v, want %v", err, core.ErrReleaseContract)
	}
	if err := (LatestAssessment{}).Validate(); !errors.Is(err, core.ErrReleaseLatest) {
		t.Fatalf("LatestAssessment{}.Validate() error = %v, want %v", err, core.ErrReleaseLatest)
	}
	if err := (Selection{}).Validate(); !errors.Is(err, core.ErrReleaseContract) {
		t.Fatalf("Selection{}.Validate() error = %v, want %v", err, core.ErrReleaseContract)
	}
	if _, ok := (Selection{}).Current(); ok {
		t.Fatalf("Selection{}.Current() ok = true, want false")
	}
	if _, ok := (Selection{}).Available(); ok {
		t.Fatalf("Selection{}.Available() ok = true, want false")
	}
	if _, ok := (Selection{}).Refresh(); ok {
		t.Fatalf("Selection{}.Refresh() ok = true, want false")
	}
	if _, ok := (Selection{}).Reassess(); ok {
		t.Fatalf("Selection{}.Reassess() ok = true, want false")
	}
	if _, err := (CurrentRelease{}).Summary(); !errors.Is(err, core.ErrReleaseContract) {
		t.Fatalf("CurrentRelease{}.Summary() error = %v, want %v", err, core.ErrReleaseContract)
	}
	if _, err := (AvailableRelease{}).Summary(); !errors.Is(err, core.ErrReleaseVerification) {
		t.Fatalf("AvailableRelease{}.Summary() error = %v, want %v", err, core.ErrReleaseVerification)
	}
	if _, err := (PreparedRelease{}).Artifact(); !errors.Is(err, core.ErrReleaseVerification) {
		t.Fatalf("PreparedRelease{}.Artifact() error = %v, want %v", err, core.ErrReleaseVerification)
	}
	if _, err := (PreparedRelease{}).CandidateManifest(); !errors.Is(err, core.ErrReleaseVerification) {
		t.Fatalf("PreparedRelease{}.CandidateManifest() error = %v, want %v", err, core.ErrReleaseVerification)
	}
	if _, err := (PreparedRelease{}).InstalledManifest(); !errors.Is(err, core.ErrReleaseVerification) {
		t.Fatalf("PreparedRelease{}.InstalledManifest() error = %v, want %v", err, core.ErrReleaseVerification)
	}
	if _, err := (PreparedRelease{}).Latest(); !errors.Is(err, core.ErrReleaseVerification) {
		t.Fatalf("PreparedRelease{}.Latest() error = %v, want %v", err, core.ErrReleaseVerification)
	}
	if _, err := (PreparedRelease{}).Observation(); !errors.Is(err, core.ErrReleaseVerification) {
		t.Fatalf("PreparedRelease{}.Observation() error = %v, want %v", err, core.ErrReleaseVerification)
	}
	if _, err := (PreparedRelease{}).Assessment(); !errors.Is(err, core.ErrReleaseVerification) {
		t.Fatalf("PreparedRelease{}.Assessment() error = %v, want %v", err, core.ErrReleaseVerification)
	}
	if err := (Preparation{}).Validate(); !errors.Is(err, core.ErrReleaseContract) {
		t.Fatalf("Preparation{}.Validate() error = %v, want %v", err, core.ErrReleaseContract)
	}
	if _, ok := (Preparation{}).Ready(); ok {
		t.Fatalf("Preparation{}.Ready() ok = true, want false")
	}
	if _, ok := (Preparation{}).Refresh(); ok {
		t.Fatalf("Preparation{}.Refresh() ok = true, want false")
	}
	if _, ok := (Preparation{}).Reassess(); ok {
		t.Fatalf("Preparation{}.Reassess() ok = true, want false")
	}
}

func provePreparationState(t *testing.T, available AvailableRelease, at int64, want SelectionState) {
	t.Helper()
	preparation, err := available.PrepareAt(temporal.InstantFromNanoseconds(at))
	if err != nil {
		t.Fatalf("AvailableRelease.PrepareAt(%d) error = %v", at, err)
	}
	switch want {
	case SelectionAvailable:
		_, ok := preparation.Ready()
		if !ok {
			t.Fatalf("AvailableRelease.PrepareAt(%d).Ready() ok = false", at)
		}
	case SelectionRefreshRequired:
		_, ok := preparation.Refresh()
		if !ok {
			t.Fatalf("AvailableRelease.PrepareAt(%d).Refresh() ok = false", at)
		}
	case SelectionReassessAt:
		directive, ok := preparation.Reassess()
		if !ok {
			t.Fatalf("AvailableRelease.PrepareAt(%d).Reassess() ok = false", at)
		}
		if directive.At != temporal.InstantFromNanoseconds(2_000) {
			t.Fatalf("AvailableRelease.PrepareAt(%d) reassess = %v, want valid-from", at, directive.At)
		}
	}
}

func TestReleaseJSONReceiversRejectMalformedAndRemainUnchanged(t *testing.T) {
	t.Parallel()
	fixture := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 1)
	latestBytes, err := json.Marshal(fixture.latest)
	if err != nil {
		t.Fatalf("json.Marshal(LatestDocument) error = %v", err)
	}
	manifestBytes, err := json.Marshal(fixture.manifest)
	if err != nil {
		t.Fatalf("json.Marshal(ManifestDocument) error = %v", err)
	}

	hostile := []struct {
		data         string
		wantContract bool
	}{
		{data: ``},
		{data: `null`, wantContract: true},
		{data: `{}`, wantContract: true},
		{data: `[]`, wantContract: true},
		{data: `true`, wantContract: true},
		{data: `{"fact":null,"attestation":null}`, wantContract: true},
		{data: `{"fact":{},"attestation":{}}`, wantContract: true},
		{data: `{"fact":{},"fact":{},"attestation":{}}`, wantContract: true},
		{data: `{"fact":{},"attestation":{},"future":true}`, wantContract: true},
		{data: `{"fact":[],"attestation":{}}`, wantContract: true},
		{data: `"` + strings.Repeat("x", documentExtentMaximum+1) + `"`, wantContract: true},
	}
	for _, tc := range hostile {
		latestReceiver := fixture.latest
		err := json.Unmarshal([]byte(tc.data), &latestReceiver)
		if err == nil || tc.wantContract && !errors.Is(err, core.ErrJSONContract) {
			t.Fatalf("json.Unmarshal(LatestDocument %q) error = %v, want rejection with contract=%v", boundedDiagnostic(tc.data), err, tc.wantContract)
		}
		if latestReceiver != fixture.latest {
			t.Fatalf("json.Unmarshal(LatestDocument %q) mutated receiver", boundedDiagnostic(tc.data))
		}

		manifestReceiver := fixture.manifest
		err = json.Unmarshal([]byte(tc.data), &manifestReceiver)
		if err == nil || tc.wantContract && !errors.Is(err, core.ErrJSONContract) {
			t.Fatalf("json.Unmarshal(ManifestDocument %q) error = %v, want rejection with contract=%v", boundedDiagnostic(tc.data), err, tc.wantContract)
		}
		if manifestReceiver != fixture.manifest {
			t.Fatalf("json.Unmarshal(ManifestDocument %q) mutated receiver", boundedDiagnostic(tc.data))
		}
	}

	var nilLatest *LatestDocument
	if err := nilLatest.UnmarshalJSON(latestBytes); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil LatestDocument.UnmarshalJSON() error = %v, want %v", err, core.ErrJSONContract)
	}
	var nilManifest *ManifestDocument
	if err := nilManifest.UnmarshalJSON(manifestBytes); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil ManifestDocument.UnmarshalJSON() error = %v, want %v", err, core.ErrJSONContract)
	}
}

func boundedDiagnostic(value string) string {
	if len(value) <= 80 {
		return value
	}
	return value[:80]
}

func TestCanonicalReleaseProjectionsRoundTripExactSignedFacts(t *testing.T) {
	t.Parallel()
	fixture := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 1)

	manifestFactBytes, err := json.Marshal(fixture.manifest.Fact)
	if err != nil {
		t.Fatalf("json.Marshal(ManifestFact) error = %v", err)
	}
	var manifestFact ManifestFact
	if err := json.Unmarshal(manifestFactBytes, &manifestFact); err != nil || manifestFact != fixture.manifest.Fact {
		t.Fatalf("ManifestFact round trip = (%v, %v), want exact signed fact", manifestFact, err)
	}
	if manifestFact.Revision() != Revision2026V1 ||
		manifestFact.Commit() != fixture.builds[0].Commit() ||
		manifestFact.CreatedAt() != temporal.InstantFromNanoseconds(1_000) {
		t.Fatalf("ManifestFact accessors differ from constructed facts")
	}

	latestFactBytes, err := json.Marshal(fixture.latest.Fact)
	if err != nil {
		t.Fatalf("json.Marshal(LatestFact) error = %v", err)
	}
	var latestFact LatestFact
	if err := json.Unmarshal(latestFactBytes, &latestFact); err != nil || latestFact != fixture.latest.Fact {
		t.Fatalf("LatestFact round trip = (%v, %v), want exact signed fact", latestFact, err)
	}
	if latestFact.Identity().String() == "" ||
		manifestFact.Identity().String() == "" ||
		fixture.artifacts[0].Identity().String() == "" {
		t.Fatalf("signed nominal identity rendered empty canonical text")
	}

	integrityBytes, err := json.Marshal(fixture.artifacts[0].Integrity())
	if err != nil {
		t.Fatalf("json.Marshal(ArtifactIntegrity) error = %v", err)
	}
	var integrity ArtifactIntegrity
	if err := json.Unmarshal(integrityBytes, &integrity); err != nil ||
		integrity != fixture.artifacts[0].Integrity() {
		t.Fatalf("ArtifactIntegrity round trip = (%v, %v), want exact integrity", integrity, err)
	}

	artifactBytes, err := json.Marshal(fixture.artifacts[0])
	if err != nil {
		t.Fatalf("json.Marshal(Artifact) error = %v", err)
	}
	var artifact Artifact
	if err := json.Unmarshal(artifactBytes, &artifact); err != nil || artifact != fixture.artifacts[0] {
		t.Fatalf("Artifact round trip = (%v, %v), want exact artifact", artifact, err)
	}

	setBytes, err := json.Marshal(fixture.artifactSet)
	if err != nil {
		t.Fatalf("json.Marshal(ArtifactSet) error = %v", err)
	}
	var set ArtifactSet
	if err := json.Unmarshal(setBytes, &set); err != nil || set != fixture.artifactSet {
		t.Fatalf("ArtifactSet round trip = (%v, %v), want exact set", set, err)
	}

	documentDigest, err := fixture.verified.DocumentDigest()
	if err != nil || documentDigest.String() == "" {
		t.Fatalf("VerifiedManifest.DocumentDigest() = (%q, %v), want canonical digest", documentDigest.String(), err)
	}
}

func TestCanonicalWritersRefuseNilAndShortDestinations(t *testing.T) {
	t.Parallel()
	fixture := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 1)

	if err := fixture.manifest.Fact.WriteCanonical(nil); !errors.Is(err, core.ErrReleaseManifest) {
		t.Fatalf("ManifestFact.WriteCanonical(nil) error = %v, want %v", err, core.ErrReleaseManifest)
	}
	if err := fixture.latest.Fact.WriteCanonical(nil); !errors.Is(err, core.ErrReleaseLatest) {
		t.Fatalf("LatestFact.WriteCanonical(nil) error = %v, want %v", err, core.ErrReleaseLatest)
	}

	if err := fixture.manifest.Fact.WriteCanonical(shortReleaseWriter{}); !errors.Is(err, core.ErrReleaseManifest) ||
		!errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("ManifestFact.WriteCanonical(short) error = %v, want %v and %v", err, core.ErrReleaseManifest, io.ErrShortWrite)
	}
	if err := fixture.latest.Fact.WriteCanonical(shortReleaseWriter{}); !errors.Is(err, core.ErrReleaseLatest) ||
		!errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("LatestFact.WriteCanonical(short) error = %v, want %v and %v", err, core.ErrReleaseLatest, io.ErrShortWrite)
	}

	var manifest bytes.Buffer
	if err := fixture.manifest.Fact.WriteCanonical(&manifest); err != nil {
		t.Fatalf("ManifestFact.WriteCanonical() error = %v", err)
	}
	wantManifest, _ := json.Marshal(fixture.manifest.Fact)
	if !bytes.Equal(manifest.Bytes(), wantManifest) {
		t.Fatalf("ManifestFact.WriteCanonical() bytes differ from canonical JSON")
	}
	var latest bytes.Buffer
	if err := fixture.latest.Fact.WriteCanonical(&latest); err != nil {
		t.Fatalf("LatestFact.WriteCanonical() error = %v", err)
	}
	wantLatest, _ := json.Marshal(fixture.latest.Fact)
	if !bytes.Equal(latest.Bytes(), wantLatest) {
		t.Fatalf("LatestFact.WriteCanonical() bytes differ from canonical JSON")
	}
}

type shortReleaseWriter struct{}

func (shortReleaseWriter) Write(value []byte) (int, error) {
	if len(value) == 0 {
		return 0, nil
	}
	return len(value) - 1, nil
}

func TestReleaseNominalJSONAndTargetBounds(t *testing.T) {
	t.Parallel()
	fixture := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 1)

	revisionBytes, err := json.Marshal(Revision2026V1)
	if err != nil {
		t.Fatalf("json.Marshal(Revision) error = %v", err)
	}
	var revision Revision
	if err := json.Unmarshal(revisionBytes, &revision); err != nil || revision != Revision2026V1 {
		t.Fatalf("Revision round trip = (%v, %v), want (%v, nil)", revision, err, Revision2026V1)
	}
	revision = Revision2026V1
	if err := json.Unmarshal([]byte(`"future"`), &revision); !errors.Is(err, core.ErrJSONContract) ||
		revision != Revision2026V1 {
		t.Fatalf("Revision future decode = (%v, %v), want preserved receiver and %v", revision, err, core.ErrJSONContract)
	}

	for _, domain := range []Domain{DomainManifestV1, DomainLatestV1} {
		text, err := domain.MarshalText()
		if err != nil {
			t.Fatalf("Domain(%d).MarshalText() error = %v", domain, err)
		}
		got, err := (DomainUnknown).ParseCanonicalText(text)
		if err != nil || got != domain {
			t.Fatalf("Domain.ParseCanonicalText(%q) = (%v, %v), want (%v, nil)", text, got, err, domain)
		}
	}
	if _, err := (DomainUnknown).ParseCanonicalText([]byte("future")); !errors.Is(err, core.ErrReleaseContract) {
		t.Fatalf("Domain.ParseCanonicalText(future) error = %v, want %v", err, core.ErrReleaseContract)
	}
	if _, err := (DomainUnknown).MarshalText(); !errors.Is(err, core.ErrReleaseContract) {
		t.Fatalf("DomainUnknown.MarshalText() error = %v, want %v", err, core.ErrReleaseContract)
	}

	targets := Targets()
	if _, ok := targets.At(-1); ok {
		t.Fatalf("Targets().At(-1) ok = true, want false")
	}
	if _, ok := targets.At(TargetCount); ok {
		t.Fatalf("Targets().At(TargetCount) ok = true, want false")
	}
	if _, ok := (TargetSet{}).At(0); ok {
		t.Fatalf("TargetSet{}.At(0) ok = true, want false")
	}
	if _, ok := fixture.artifactSet.At(-1); ok {
		t.Fatalf("ArtifactSet.At(-1) ok = true, want false")
	}
	if _, ok := fixture.artifactSet.At(TargetCount); ok {
		t.Fatalf("ArtifactSet.At(TargetCount) ok = true, want false")
	}
	outside, err := core.NewPlatform(core.OperatingSystemDarwin, core.CPUArchitectureAMD64)
	if err != nil {
		t.Fatalf("NewPlatform(outside) error = %v", err)
	}
	if _, ok := fixture.artifactSet.ForPlatform(outside); ok {
		t.Fatalf("ArtifactSet.ForPlatform(outside) ok = true, want false")
	}
	if _, ok := (ArtifactSet{}).ForPlatform(fixture.builds[0].Platform()); ok {
		t.Fatalf("ArtifactSet{}.ForPlatform() ok = true, want false")
	}
}
