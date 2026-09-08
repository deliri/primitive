package release

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// Exhaust all nine generation/version order combinations from real signed
// producer facts. The unchanged timeline isolates the ordering handoff; the
// separate timeline and signer-rotation tables attack those additional axes.
func TestAdvanceLatestProducerClassifierOrderMatrix(t *testing.T) {
	t.Parallel()
	retained := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 5)
	for _, tc := range []struct {
		name       string
		generation uint64
		version    core.ReleaseVersion
		state      LatestAdvanceState
		primary    selectionHandoffClass
		wantErr    error
	}{
		{name: "lower generation cannot borrow a lower version", generation: 4, version: core.NewReleaseVersion(2026, 7, 29), primary: selectionHandoffContradiction, wantErr: core.ErrReleaseRollback},
		{name: "lower generation cannot borrow the same version", generation: 4, version: core.NewReleaseVersion(2026, 7, 30), primary: selectionHandoffContradiction, wantErr: core.ErrReleaseRollback},
		{name: "lower generation cannot borrow a newer version", generation: 4, version: core.NewReleaseVersion(2026, 7, 31), primary: selectionHandoffContradiction, wantErr: core.ErrReleaseRollback},
		{name: "same generation with an older version is contradictory replay", generation: 5, version: core.NewReleaseVersion(2026, 7, 29), primary: selectionHandoffContradiction, wantErr: core.ErrReleaseConflict},
		{name: "exact signed replay contributes no new generation", generation: 5, version: core.NewReleaseVersion(2026, 7, 30), state: LatestAdvanceReplay, primary: selectionHandoffNeutral},
		{name: "same generation cannot conceal a newer version", generation: 5, version: core.NewReleaseVersion(2026, 7, 31), primary: selectionHandoffContradiction, wantErr: core.ErrReleaseConflict},
		{name: "higher generation cannot conceal version rollback", generation: 6, version: core.NewReleaseVersion(2026, 7, 29), primary: selectionHandoffContradiction, wantErr: core.ErrReleaseRollback},
		{name: "higher generation can renew the identical manifest", generation: 6, version: core.NewReleaseVersion(2026, 7, 30), state: LatestAdvanceAdvanced, primary: selectionHandoffBoundary},
		{name: "higher generation admits a genuinely newer release", generation: 6, version: core.NewReleaseVersion(2026, 7, 31), state: LatestAdvanceAdvanced, primary: selectionHandoffBoundary},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			proposed := newReleaseFixture(t, tc.version, tc.generation)
			if proposed.verifiedLatest.Validate() != nil || proposed.verifiedLatest.Fact().Generation().Uint64() != tc.generation || proposed.verifiedLatest.Manifest().Version() != tc.version {
				t.Fatalf("producer = %v, want authenticated generation %d and version %v", proposed.verifiedLatest, tc.generation, tc.version)
			}
			request := AdvanceLatestRequest{Retained: retained.verifiedLatest, Proposed: proposed.verifiedLatest}
			before := request
			got, err := AdvanceLatest(request)
			if !errors.Is(err, tc.wantErr) || request != before {
				t.Fatalf("advance error/input = (%v, unchanged %t), want (%v, unchanged true)", err, request == before, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (LatestAdvance{}) || tc.primary != selectionHandoffContradiction || errors.Is(err, core.ErrReleaseRollback) != (errors.Is(tc.wantErr, core.ErrReleaseRollback)) || errors.Is(err, core.ErrReleaseConflict) != (errors.Is(tc.wantErr, core.ErrReleaseConflict)) {
					t.Fatalf("advance contradiction = (%v, %v, class %d), want zero and exclusive %v", got, err, tc.primary, tc.wantErr)
				}
				return
			}
			if got.Validate() != nil || got.State() != tc.state {
				t.Fatalf("advance state = %v, want %v", got.State(), tc.state)
			}
			replayed, err := AdvanceLatest(request)
			if err != nil || replayed != got {
				t.Fatalf("repeated advance classification = (%v, %v), want unchanged %v", replayed, err, got)
			}
		})
	}
}

func TestAdvanceLatestRefusalAndStreamIdentityPrecedence(t *testing.T) {
	t.Parallel()
	retained := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 5)
	for _, tc := range []struct {
		name                                string
		generation                          uint64
		zeroRetained, zeroProposed, foreign bool
		primary                             selectionHandoffClass
		wantErr                             error
	}{
		{name: "missing retained proof cannot become first append", generation: 6, zeroRetained: true, primary: selectionHandoffRefusal, wantErr: core.ErrReleaseVerification},
		{name: "missing proposed proof cannot become neutral replay", generation: 6, zeroProposed: true, primary: selectionHandoffRefusal, wantErr: core.ErrReleaseVerification},
		{name: "foreign stream precedes lower-generation rollback", generation: 4, foreign: true, primary: selectionHandoffContradiction, wantErr: core.ErrReleaseConflict},
		{name: "foreign stream precedes equal-generation replay", generation: 5, foreign: true, primary: selectionHandoffContradiction, wantErr: core.ErrReleaseConflict},
		{name: "foreign stream cannot borrow a higher generation", generation: 6, foreign: true, primary: selectionHandoffContradiction, wantErr: core.ErrReleaseConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			offering := retained.builds[0].Offering()
			if tc.foreign {
				offering = releaseOffering(t, 1)
			}
			proposed := newReleaseFixtureForOffering(t, offering, core.NewReleaseVersion(2026, 7, 31), tc.generation)
			request := AdvanceLatestRequest{Retained: retained.verifiedLatest, Proposed: proposed.verifiedLatest}
			if tc.zeroRetained {
				request.Retained = VerifiedLatest{}
			}
			if tc.zeroProposed {
				request.Proposed = VerifiedLatest{}
			}
			got, err := AdvanceLatest(request)
			if !errors.Is(err, tc.wantErr) || errors.Is(err, core.ErrReleaseRollback) || got != (LatestAdvance{}) || (tc.primary == selectionHandoffRefusal) != (tc.zeroRetained || tc.zeroProposed) {
				t.Fatalf("advance refusal = (%v, %v, class %d), want zero and exclusive %v", got, err, tc.primary, tc.wantErr)
			}
		})
	}
}
