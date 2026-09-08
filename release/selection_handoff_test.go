package release

import (
	"errors"
	"fmt"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

type selectionHandoffClass uint8

const (
	selectionHandoffUnknown selectionHandoffClass = iota
	selectionHandoffContradiction
	selectionHandoffRefusal
	selectionHandoffNeutral
	selectionHandoffBoundary
)

// Exhaust the admitted classification domain: three freshness states, three
// version orders, two clock states and every target slot. These are 72 distinct
// producer/selection combinations, not a quota of invalid spellings. Adjacent
// signed-time boundaries and typed producer refusals are exercised separately.
func TestAssessmentSelectionProducerClassifierExhaustiveMatrix(t *testing.T) {
	t.Parallel()
	installed := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 1)
	for _, version := range []struct {
		order   core.Comparison
		version core.ReleaseVersion
		state   SelectionState
		primary selectionHandoffClass
		wantErr error
	}{
		{order: core.ComparisonGreater, version: core.NewReleaseVersion(2026, 7, 29), primary: selectionHandoffContradiction, wantErr: core.ErrReleaseRollback},
		{order: core.ComparisonEqual, version: installed.builds[0].Version(), state: SelectionCurrent, primary: selectionHandoffNeutral},
		{order: core.ComparisonLess, version: core.NewReleaseVersion(2026, 7, 31), state: SelectionAvailable, primary: selectionHandoffBoundary},
	} {
		candidate := newReleaseFixture(t, version.version, 2)
		latest := issueVerifiedLatest(t, candidate, candidate.verified, 2, 1_000, 2_000, 10_000)
		cached, err := NewCachedLatest(latest)
		if err != nil {
			t.Fatalf("NewCachedLatest fixture error = %v, want nil", err)
		}
		for freshness := LatestFreshnessUnknown + 1; freshness < latestFreshnessLimit; freshness++ {
			var effective int64
			state, primary, wantErr := version.state, version.primary, version.wantErr
			switch freshness {
			case LatestFreshnessNotYetValid:
				effective, state, primary, wantErr = 1_999, SelectionReassessAt, selectionHandoffBoundary, nil
			case LatestFreshnessCurrent:
				effective = 2_000
			case LatestFreshnessExpired:
				effective, state, primary, wantErr = 10_000, SelectionRefreshRequired, selectionHandoffBoundary, nil
			default:
				t.Fatalf("freshness %v has no matrix fixture, want all admitted states covered", freshness)
			}
			for clock := LatestClockUnknown + 1; clock < latestClockLimit; clock++ {
				for target := range TargetCount {
					name := fmt.Sprintf("%s/%s/%s/%s/class-%d", version.order, freshness, clock, installed.builds[target].Platform(), primary)
					t.Run(name, func(t *testing.T) {
						t.Parallel()
						evidence := latestTimeEvidenceAt(t, effective)
						switch clock {
						case LatestClockObserved:
						case LatestClockCorrected:
							evidence = latestTimeEvidenceAt(t, effective-1)
							evidence.DurableHighWater = temporal.InstantFromNanoseconds(effective)
						default:
							t.Fatalf("clock state %v has no matrix fixture, want all admitted states covered", clock)
						}
						producer, err := AssessLatest(AssessLatestRequest{Latest: latest, Time: evidence})
						boundary, hasBoundary := producer.Boundary()
						wantBoundary := latest.Fact().ValidUntil()
						if freshness == LatestFreshnessNotYetValid {
							wantBoundary = latest.Fact().ValidFrom()
						}
						if freshness == LatestFreshnessExpired {
							wantBoundary = temporal.Instant{}
						}
						if err != nil || producer.Validate() != nil || producer.Freshness() != freshness || producer.ClockState() != clock || producer.EffectiveAt() != temporal.InstantFromNanoseconds(effective) || producer.ValidUntil() != latest.Fact().ValidUntil() || boundary != wantBoundary || hasBoundary != (freshness != LatestFreshnessExpired) {
							t.Fatalf("producer = (%v, %v), want freshness %v clock %v effective %d and exact signed boundary %v", producer, err, freshness, clock, effective, wantBoundary)
						}
						request := EvaluateInstalledRequest{Installed: installed.builds[target], Evaluate: EvaluateRequest{InstalledManifest: installed.verified, Latest: cached, Time: evidence}}
						before := request
						got, err := EvaluateInstalled(request)
						if !errors.Is(err, wantErr) || request != before {
							t.Fatalf("selection error/input = (%v, unchanged %t), want (%v, unchanged true)", err, request == before, wantErr)
						}
						if wantErr != nil {
							if primary != selectionHandoffContradiction || got != (Selection{}) || errors.Is(err, core.ErrReleaseConflict) || errors.Is(err, core.ErrReleaseLatest) {
								t.Fatalf("rollback classification = (%v, %v, class %d), want zero and exclusive rollback contradiction", got, err, primary)
							}
							return
						}
						proveSelectionHandoff(t, selectionHandoffProof{Selection: got, State: state, Installed: installed, Candidate: candidate, Latest: latest, Assessment: producer, Time: evidence, Target: target})
						replayed, replayErr := EvaluateInstalled(request)
						if replayErr != nil || replayed != got {
							t.Fatalf("replayed selection = (%v, %v), want identical immutable selection %v", replayed, replayErr, got)
						}
					})
				}
			}
		}
	}
}

type selectionHandoffProof struct {
	Selection            Selection
	State                SelectionState
	Installed, Candidate releaseFixture
	Latest               VerifiedLatest
	Assessment           LatestAssessment
	Time                 LatestTimeEvidence
	Target               int
}

func proveSelectionHandoff(t *testing.T, proof selectionHandoffProof) {
	t.Helper()
	got := proof.Selection
	current, currentOK := got.Current()
	available, availableOK := got.Available()
	refresh, refreshOK := got.Refresh()
	reassess, reassessOK := got.Reassess()
	if got.Validate() != nil || got.State() != proof.State || currentOK != (proof.State == SelectionCurrent) || availableOK != (proof.State == SelectionAvailable) || refreshOK != (proof.State == SelectionRefreshRequired) || reassessOK != (proof.State == SelectionReassessAt) {
		t.Fatalf("selection arms = (%v, %t, %t, %t, %t), want exclusive %v", got.State(), currentOK, availableOK, refreshOK, reassessOK, proof.State)
	}
	if (!currentOK && current != (CurrentRelease{})) || (!availableOK && available != (AvailableRelease{})) || (!refreshOK && refresh != (RefreshDirective{})) || (!reassessOK && reassess != (ReassessDirective{})) {
		t.Fatalf("inactive selection arms = (%v, %v, %v, %v), want zero inactive evidence", current, available, refresh, reassess)
	}
	switch proof.State {
	case SelectionCurrent:
		summary, err := current.Summary()
		want := CurrentSummary{ValidUntil: proof.Latest.Fact().ValidUntil(), Version: proof.Installed.builds[proof.Target].Version(), Manifest: proof.Installed.verified.Identity(), Artifact: proof.Installed.artifacts[proof.Target].Identity()}
		if err != nil || summary != want {
			t.Fatalf("current summary = (%v, %v), want exact installed facts %v", summary, err, want)
		}
	case SelectionAvailable:
		candidateArtifact := proof.Candidate.artifacts[proof.Target]
		filename, filenameErr := candidateArtifact.Filename()
		summary, summaryErr := available.Summary()
		wantSummary := AvailableSummary{Installed: proof.Installed.builds[proof.Target], Candidate: candidateArtifact.Build(), Manifest: proof.Candidate.verified.Identity(), ManifestDocument: proof.Candidate.verified.DocumentDigest(), Artifact: candidateArtifact.Identity(), Filename: filename, Integrity: candidateArtifact.Integrity(), ValidUntil: proof.Latest.Fact().ValidUntil()}
		if filenameErr != nil || summaryErr != nil || summary != wantSummary {
			t.Fatalf("available summary = (%v, %v, %v), want exact candidate facts %v", summary, summaryErr, filenameErr, wantSummary)
		}
		summary.Candidate = proof.Installed.builds[proof.Target]
		if err := summary.Validate(); !errors.Is(err, core.ErrReleaseConflict) {
			t.Fatalf("substituted candidate summary error = %v, want %v", err, core.ErrReleaseConflict)
		}
		retained, err := available.Latest()
		if err != nil || retained != proof.Latest {
			t.Fatalf("available latest = (%v, %v), want exact producer latest %v", retained, err, proof.Latest)
		}
		prepared, err := available.Prepare(proof.Time)
		if err != nil {
			t.Fatalf("Prepare error = %v, want nil", err)
		}
		ready, ok := prepared.Ready()
		if !ok || prepared.Validate() != nil {
			t.Fatalf("preparation = (%v, ready %t), want exclusively ready", prepared, ok)
		}
		artifact, artifactErr := ready.Artifact()
		installed, installedErr := ready.InstalledManifest()
		candidate, candidateErr := ready.CandidateManifest()
		latest, latestErr := ready.Latest()
		timeEvidence, timeErr := ready.TimeEvidence()
		assessment, assessmentErr := ready.Assessment()
		if err := errors.Join(artifactErr, installedErr, candidateErr, latestErr, timeErr, assessmentErr); err != nil || artifact != proof.Candidate.artifacts[proof.Target] || installed != proof.Installed.verified || candidate != proof.Candidate.verified || latest != proof.Latest || timeEvidence != proof.Time || assessment != proof.Assessment {
			t.Fatalf("prepared facts = (%v, %v, %v, %v, %v, %v, %v), want exact input manifests/latest/time/assessment and target artifact %v", artifact, installed, candidate, latest, timeEvidence, assessment, err, proof.Candidate.artifacts[proof.Target])
		}
		if _, ok := prepared.Refresh(); ok {
			t.Fatal("ready preparation Refresh ok = true, want false")
		}
		if _, ok := prepared.Reassess(); ok {
			t.Fatal("ready preparation Reassess ok = true, want false")
		}
	case SelectionRefreshRequired:
		if refresh.Validate() != nil {
			t.Fatalf("refresh directive = %v, want valid without candidate authority", refresh)
		}
	case SelectionReassessAt:
		if reassess.At != proof.Latest.Fact().ValidFrom() {
			t.Fatalf("reassess boundary = %v, want signed valid-from %v", reassess.At, proof.Latest.Fact().ValidFrom())
		}
	default:
		t.Fatalf("selection state = %v, want one admitted matrix state", proof.State)
	}
}

func TestAssessmentPreparationProducerClassifierBoundaryAndRefusalMatrix(t *testing.T) {
	t.Parallel()
	installed := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 1)
	candidate := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 31), 2)
	latest := issueVerifiedLatest(t, candidate, candidate.verified, 2, 1_000, 2_000, 10_000)
	cached, err := NewCachedLatest(latest)
	if err != nil {
		t.Fatalf("NewCachedLatest error = %v, want nil", err)
	}
	selection, err := EvaluateInstalled(EvaluateInstalledRequest{Installed: installed.builds[0], Evaluate: EvaluateRequest{InstalledManifest: installed.verified, Latest: cached, Time: latestTimeEvidenceAt(t, 3_000)}})
	if err != nil {
		t.Fatalf("EvaluateInstalled fixture error = %v, want nil", err)
	}
	available, ok := selection.Available()
	if !ok {
		t.Fatal("fixture Available ok = false, want true")
	}
	for _, tc := range []struct {
		name      string
		primary   selectionHandoffClass
		at        int64
		mutate    func(*testing.T, *LatestTimeEvidence)
		freshness LatestFreshness
		wantErr   error
	}{
		{name: "one before valid-from withdraws readiness", primary: selectionHandoffBoundary, at: 1_999, freshness: LatestFreshnessNotYetValid},
		{name: "exact valid-from admits readiness", primary: selectionHandoffBoundary, at: 2_000, freshness: LatestFreshnessCurrent},
		{name: "one after valid-from stays ready", primary: selectionHandoffBoundary, at: 2_001, freshness: LatestFreshnessCurrent},
		{name: "same evidence retains identical authority", primary: selectionHandoffNeutral, at: 3_000, freshness: LatestFreshnessCurrent},
		{name: "one before valid-until remains ready", primary: selectionHandoffBoundary, at: 9_999, freshness: LatestFreshnessCurrent},
		{name: "exact valid-until withdraws readiness", primary: selectionHandoffBoundary, at: 10_000, freshness: LatestFreshnessExpired},
		{name: "one after valid-until cannot revive readiness", primary: selectionHandoffBoundary, at: 10_001, freshness: LatestFreshnessExpired},
		{name: "durable correction crosses expiration before preparation", primary: selectionHandoffBoundary, at: 9_999, mutate: func(_ *testing.T, e *LatestTimeEvidence) {
			e.DurableHighWater = temporal.InstantFromNanoseconds(10_000)
		}, freshness: LatestFreshnessExpired},
		{name: "missing started observation cannot become preparation", primary: selectionHandoffRefusal, at: 3_000, mutate: func(_ *testing.T, e *LatestTimeEvidence) { e.StartedAt = temporal.Observation{} }, wantErr: core.ErrReleaseLatest},
		{name: "missing final observation cannot become preparation", primary: selectionHandoffRefusal, at: 3_000, mutate: func(_ *testing.T, e *LatestTimeEvidence) { e.ObservedAt = temporal.Observation{} }, wantErr: core.ErrReleaseLatest},
		{name: "missing durable high-water cannot become preparation", primary: selectionHandoffRefusal, at: 3_000, mutate: func(_ *testing.T, e *LatestTimeEvidence) { e.DurableHighWater = temporal.Instant{} }, wantErr: core.ErrReleaseLatest},
		{name: "negative elapsed evidence cannot become preparation", primary: selectionHandoffRefusal, at: 3_000, mutate: func(t *testing.T, e *LatestTimeEvidence) { e.StartedAt = latestTimeEvidenceAt(t, 3_001).StartedAt }, wantErr: core.ErrReleaseLatest},
		{name: "rollback beyond tolerance cannot become a refresh success", primary: selectionHandoffRefusal, at: 3_000, mutate: func(_ *testing.T, e *LatestTimeEvidence) {
			e.DurableHighWater = temporal.InstantFromNanoseconds(3_001 + int64(ReleaseClockRollbackToleranceNanoseconds))
		}, wantErr: core.ErrReleaseLatest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			evidence := latestTimeEvidenceAt(t, tc.at)
			if tc.mutate != nil {
				tc.mutate(t, &evidence)
			}
			before := available
			producer, producerErr := AssessLatest(AssessLatestRequest{Latest: latest, Time: evidence})
			got, gotErr := available.Prepare(evidence)
			if !errors.Is(producerErr, tc.wantErr) || !errors.Is(gotErr, tc.wantErr) || available != before {
				t.Fatalf("producer/preparation = (%v, %v, unchanged %t), want (%v, %v, unchanged true)", producerErr, gotErr, available == before, tc.wantErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if tc.primary != selectionHandoffRefusal || producer != (LatestAssessment{}) || got != (Preparation{}) || errors.Is(gotErr, core.ErrReleaseRollback) || errors.Is(gotErr, core.ErrReleaseConflict) {
					t.Fatalf("refused handoff = (%v, %v, class %d), want zero facts and exclusive typed refusal", producer, got, tc.primary)
				}
				return
			}
			if producer.Validate() != nil || producer.Freshness() != tc.freshness || got.Validate() != nil {
				t.Fatalf("handoff = (%v, %v), want valid preparation from freshness %v", producer, got, tc.freshness)
			}
			ready, readyOK := got.Ready()
			refresh, refreshOK := got.Refresh()
			reassess, reassessOK := got.Reassess()
			if readyOK != (tc.freshness == LatestFreshnessCurrent) || refreshOK != (tc.freshness == LatestFreshnessExpired) || reassessOK != (tc.freshness == LatestFreshnessNotYetValid) {
				t.Fatalf("preparation arms = (%t, %t, %t), want exclusive action for freshness %v", readyOK, refreshOK, reassessOK, tc.freshness)
			}
			if (!readyOK && ready != (PreparedRelease{})) || (!refreshOK && refresh != (RefreshDirective{})) || (!reassessOK && reassess != (ReassessDirective{})) {
				t.Fatalf("inactive preparation arms = (%v, %v, %v), want zero inactive evidence", ready, refresh, reassess)
			}
			if reassessOK && reassess.At != latest.Fact().ValidFrom() {
				t.Fatalf("reassess At = %v, want signed boundary %v", reassess.At, latest.Fact().ValidFrom())
			}
			if readyOK {
				assessment, err := ready.Assessment()
				if err != nil || assessment != producer {
					t.Fatalf("ready assessment = (%v, %v), want exact producer %v", assessment, err, producer)
				}
				artifact, err := ready.Artifact()
				if err != nil || artifact != candidate.artifacts[0] {
					t.Fatalf("ready artifact = (%v, %v), want exact candidate %v", artifact, err, candidate.artifacts[0])
				}
			}
			replayed, err := available.Prepare(evidence)
			if err != nil || replayed != got {
				t.Fatalf("replayed preparation = (%v, %v), want unchanged %v", replayed, err, got)
			}
		})
	}
}

func TestSelectionIngressLayerTriadPreservesRefusalAndAbsence(t *testing.T) {
	t.Parallel()
	fixture := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 1)
	cached, err := NewCachedLatest(fixture.verifiedLatest)
	if err != nil {
		t.Fatalf("NewCachedLatest error = %v, want nil", err)
	}
	outside, err := core.NewBuildIdentity(core.BuildIdentityRequest{Offering: fixture.builds[0].Offering(), Version: fixture.builds[0].Version(), Commit: fixture.builds[0].Commit(), Platform: core.Platform{OperatingSystem: core.OperatingSystemDarwin, Architecture: core.CPUArchitectureAMD64}})
	if err != nil {
		t.Fatalf("outside-target identity error = %v, want valid Core identity", err)
	}
	for _, tc := range []struct {
		name    string
		primary selectionHandoffClass
		mutate  func(*EvaluateInstalledRequest)
		state   SelectionState
		wantErr error
	}{
		{name: "valid retained authority remains current", primary: selectionHandoffNeutral, state: SelectionCurrent},
		{name: "explicit absence requests refresh without artifact authority", primary: selectionHandoffNeutral, mutate: func(r *EvaluateInstalledRequest) { r.Evaluate.Latest = MissingCachedLatest() }, state: SelectionRefreshRequired},
		{name: "valid Core target outside the Release target set cannot borrow an artifact", primary: selectionHandoffRefusal, mutate: func(r *EvaluateInstalledRequest) { r.Installed = outside }, wantErr: core.ErrReleaseConflict},
		{name: "missing installed identity cannot become absence", primary: selectionHandoffRefusal, mutate: func(r *EvaluateInstalledRequest) { r.Installed = core.BuildIdentity{} }, wantErr: core.ErrReleaseConflict},
		{name: "missing installed proof cannot become absence", primary: selectionHandoffRefusal, mutate: func(r *EvaluateInstalledRequest) { r.Evaluate.InstalledManifest = VerifiedManifest{} }, wantErr: core.ErrReleaseVerification},
		{name: "unset cache cannot be silently repaired into explicit absence", primary: selectionHandoffRefusal, mutate: func(r *EvaluateInstalledRequest) { r.Evaluate.Latest = CachedLatest{} }, wantErr: core.ErrReleaseContract},
		{name: "missing time cannot reuse prior freshness", primary: selectionHandoffRefusal, mutate: func(r *EvaluateInstalledRequest) { r.Evaluate.Time = LatestTimeEvidence{} }, wantErr: core.ErrReleaseLatest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := EvaluateInstalledRequest{Installed: fixture.builds[0], Evaluate: EvaluateRequest{InstalledManifest: fixture.verified, Latest: cached, Time: latestTimeEvidenceAt(t, 3_000)}}
			if tc.mutate != nil {
				tc.mutate(&request)
			}
			before := request
			got, err := EvaluateInstalled(request)
			if !errors.Is(err, tc.wantErr) || request != before {
				t.Fatalf("selection ingress = (%v, unchanged %t), want (%v, unchanged true)", err, request == before, tc.wantErr)
			}
			if tc.wantErr != nil {
				if tc.primary != selectionHandoffRefusal || got != (Selection{}) {
					t.Fatalf("refused selection = (%v, class %d), want zero and refusal class", got, tc.primary)
				}
				return
			}
			proveSelectionHandoff(t, selectionHandoffProof{Selection: got, State: tc.state, Installed: fixture, Candidate: fixture, Latest: fixture.verifiedLatest, Target: 0})
		})
	}
}

func TestSelectionRefusesEqualVersionManifestContradictions(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		changedTarget int
	}{
		{name: "changed installed-target bytes cannot remain current", changedTarget: 0},
		{name: "unchanged local artifact cannot hide a changed remote target", changedTarget: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fixture := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 1)
			artifacts := fixture.artifacts
			old := artifacts[tc.changedTarget]
			changed, err := NewArtifact(ArtifactRequest{Build: old.Build(), Extent: old.Integrity().Extent(), SHA256: core.SHA256Of([]byte("changed artifact bytes")), CRC32C: old.Integrity().CRC32C()})
			if err != nil || changed == old {
				t.Fatalf("changed artifact = (%v, %v), want distinct valid facts from %v", changed, err, old)
			}
			artifacts[tc.changedTarget] = changed
			set, err := NewArtifactSet(ArtifactSetRequest{Artifacts: artifacts})
			if err != nil {
				t.Fatalf("NewArtifactSet error = %v, want nil", err)
			}
			original := fixture.manifest.Fact
			fact, err := NewManifestFact(ManifestFactRequest{Revision: original.Revision(), Offering: original.Offering(), Version: original.Version(), Commit: original.Commit(), CreatedAt: original.CreatedAt(), Artifacts: set, Provenance: original.Provenance(), Metadata: original.Metadata()})
			if err != nil {
				t.Fatalf("NewManifestFact error = %v, want nil", err)
			}
			document, err := IssueManifest(IssueManifestRequest{Fact: fact, Signer: fixture.manifestKey})
			if err != nil {
				t.Fatalf("IssueManifest error = %v, want nil", err)
			}
			manifest, err := VerifyManifest(VerifyManifestRequest{Document: document, TrustedKeys: fixture.manifestTrust, ExpectedOffering: original.Offering()})
			if err != nil || manifest.Identity() == fixture.verified.Identity() {
				t.Fatalf("conflicting producer = (%v, %v), want valid and different from %v", manifest.Identity(), err, fixture.verified.Identity())
			}
			latest := issueVerifiedLatest(t, fixture, manifest, 2, 1_000, 2_000, 10_000)
			cached, err := NewCachedLatest(latest)
			if err != nil {
				t.Fatalf("NewCachedLatest conflict fixture error = %v, want nil", err)
			}
			got, err := EvaluateInstalled(EvaluateInstalledRequest{Installed: fixture.builds[0], Evaluate: EvaluateRequest{InstalledManifest: fixture.verified, Latest: cached, Time: latestTimeEvidenceAt(t, 3_000)}})
			if !errors.Is(err, core.ErrReleaseConflict) || errors.Is(err, core.ErrReleaseRollback) || got != (Selection{}) {
				t.Fatalf("equal-version contradiction = (%v, %v), want zero and exclusive immutable conflict", got, err)
			}
		})
	}
}
