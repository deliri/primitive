package lease_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
)

func TestAdvanceGenerationAndOutcomePressure(t *testing.T) {
	t.Parallel()

	authority := fixtureAuthority(t, 91)
	subject := fixtureSubject(t, 92)
	currentDecision := fixtureGrantDecision(t, subject, 5, 1_000, fixtureGrant())
	_, current := fixtureVerified(t, authority, currentDecision, subject)

	cases := []struct {
		wantErr   error
		candidate func(testing.TB) lease.Decision
		name      string
		wantState lease.AdvanceState
	}{
		{
			name: "identical replay retains current proof",
			candidate: func(testing.TB) lease.Decision {
				return currentDecision
			},
			wantState: lease.AdvanceStateUnchanged,
		},
		{
			name: "lower generation rolls back",
			candidate: func(tb testing.TB) lease.Decision {
				return fixtureGrantDecision(tb, subject, 4, 1_000, fixtureGrant())
			},
			wantErr: core.ErrLeaseRollback,
		},
		{
			name: "same generation divergent timeline conflicts",
			candidate: func(tb testing.TB) lease.Decision {
				grant := fixtureGrant()
				grant.ContactAfter = fixtureInstant(2_500)
				return fixtureGrantDecision(tb, subject, 5, 1_000, grant)
			},
			wantErr: core.ErrLeaseConflict,
		},
		{
			name: "higher grant advances",
			candidate: func(tb testing.TB) lease.Decision {
				grant := fixtureGrant()
				grant.NotBefore = fixtureInstant(2_100)
				grant.ContactAfter = fixtureInstant(3_100)
				grant.NotAfter = fixtureInstant(4_100)
				grant.GoodUntil = fixtureInstant(5_100)
				return fixtureGrantDecision(tb, subject, 6, 1_100, grant)
			},
			wantState: lease.AdvanceStateAdvanced,
		},
		{
			name: "higher refusal replaces favorable grant",
			candidate: func(tb testing.TB) lease.Decision {
				value, err := lease.NewRefusalDecision(lease.RefusalDecisionRequest{
					Header: fixtureHeader(tb, subject, 6, 5_000),
					Refusal: lease.Refusal{
						ContactAfter: fixtureInstant(6_000),
					},
				})
				if err != nil {
					tb.Fatalf("lease.NewRefusalDecision() error = %v, want nil", err)
				}
				return value
			},
			wantState: lease.AdvanceStateAdvanced,
		},
		{
			name: "higher revocation replaces favorable grant",
			candidate: func(tb testing.TB) lease.Decision {
				value, err := lease.NewRevocationDecision(lease.RevocationDecisionRequest{
					Header: fixtureHeader(tb, subject, 6, 5_000),
					Revocation: lease.Revocation{
						Reason: lease.RevocationReasonUnlawfulOrAbusiveUse,
					},
				})
				if err != nil {
					tb.Fatalf("lease.NewRevocationDecision() error = %v, want nil", err)
				}
				return value
			},
			wantState: lease.AdvanceStateAdvanced,
		},
		{
			name: "higher decision for another subject conflicts",
			candidate: func(tb testing.TB) lease.Decision {
				return fixtureGrantDecision(tb, fixtureSubject(tb, 93), 6, 1_100, fixtureGrant())
			},
			wantErr: core.ErrLeaseConflict,
		},
		{
			name: "higher decision with issued time regression rolls back",
			candidate: func(tb testing.TB) lease.Decision {
				return fixtureGrantDecision(tb, subject, 6, 999, fixtureGrant())
			},
			wantErr: core.ErrLeaseRollback,
		},
		{
			name: "higher grant with not before regression rolls back",
			candidate: func(tb testing.TB) lease.Decision {
				grant := fixtureGrant()
				grant.NotBefore = fixtureInstant(1_999)
				return fixtureGrantDecision(tb, subject, 6, 1_100, grant)
			},
			wantErr: core.ErrLeaseRollback,
		},
		{
			name: "higher grant with not after regression rolls back",
			candidate: func(tb testing.TB) lease.Decision {
				grant := fixtureGrant()
				grant.NotAfter = fixtureInstant(3_999)
				return fixtureGrantDecision(tb, subject, 6, 1_100, grant)
			},
			wantErr: core.ErrLeaseRollback,
		},
		{
			name: "higher grant with good until regression rolls back",
			candidate: func(tb testing.TB) lease.Decision {
				grant := fixtureGrant()
				grant.GoodUntil = fixtureInstant(4_999)
				return fixtureGrantDecision(tb, subject, 6, 1_100, grant)
			},
			wantErr: core.ErrLeaseRollback,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			candidateDecision := tc.candidate(t)
			candidateSubject, subjectErr := candidateDecision.Header()
			if subjectErr != nil {
				t.Fatalf("Decision.Header() error = %v, want nil", subjectErr)
			}
			_, candidate := fixtureVerified(
				t, authority, candidateDecision, candidateSubject.Subject,
			)
			got, err := lease.Advance(lease.AdvanceRequest{
				Current: current, Candidate: candidate,
			})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("lease.Advance() error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (lease.AdvanceResult{}) {
					t.Fatalf("lease.Advance() = %v, want zero", got)
				}
				return
			}
			if got.State() != tc.wantState {
				t.Fatalf("AdvanceResult.State() = %v, want %v", got.State(), tc.wantState)
			}
			selected, selectedErr := got.Verified()
			if selectedErr != nil {
				t.Fatalf("AdvanceResult.Verified() error = %v, want nil", selectedErr)
			}
			selectedDecision, selectedDecisionErr := selected.Decision()
			if selectedDecisionErr != nil {
				t.Fatalf("Verified.Decision() error = %v, want nil", selectedDecisionErr)
			}
			want := candidateDecision
			if tc.wantState == lease.AdvanceStateUnchanged {
				want = currentDecision
			}
			if selectedDecision != want {
				t.Fatalf("selected decision differs from wanted decision")
			}
		})
	}
}

// TestAdvanceIdentityPrecedesSequence pins that lease identity is decided
// before generation order. Two decisions naming different subjects are not two
// points on one sequence, so their generation relation must not decide the
// error class: a consumer that retries on rollback and stops on conflict would
// otherwise silently discard a decision belonging to another installation
// purely because its generation happened to be lower.
func TestAdvanceIdentityPrecedesSequence(t *testing.T) {
	t.Parallel()

	authority := fixtureAuthority(t, 111)
	subject := fixtureSubject(t, 112)
	other := fixtureSubject(t, 113)
	currentDecision := fixtureGrantDecision(t, subject, 5, 1_000, fixtureGrant())
	_, current := fixtureVerified(t, authority, currentDecision, subject)

	cases := []struct {
		name       string
		generation uint64
	}{
		{name: "other subject one generation below current", generation: 4},
		{name: "other subject far below current", generation: 1},
		{name: "other subject at current generation", generation: 5},
		{name: "other subject one generation above current", generation: 6},
		{name: "other subject far above current", generation: 5_000},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			candidateDecision := fixtureGrantDecision(
				t, other, tc.generation, 1_000, fixtureGrant(),
			)
			_, candidate := fixtureVerified(t, authority, candidateDecision, other)
			got, err := lease.Advance(lease.AdvanceRequest{
				Current: current, Candidate: candidate,
			})
			if !errors.Is(err, core.ErrLeaseConflict) {
				t.Fatalf("lease.Advance() error = %v, want %v", err, core.ErrLeaseConflict)
			}
			if errors.Is(err, core.ErrLeaseRollback) {
				t.Fatalf("lease.Advance() error = %v, want no rollback identity", err)
			}
			if got != (lease.AdvanceResult{}) {
				t.Fatalf("lease.Advance() = %v, want zero", got)
			}
		})
	}
}

// TestAdvanceRejectsZeroAndUnverifiedInputs proves Advance reaches no selection
// decision when either side is not an authentic proof carrier.
func TestAdvanceRejectsZeroAndUnverifiedInputs(t *testing.T) {
	t.Parallel()

	authority := fixtureAuthority(t, 114)
	subject := fixtureSubject(t, 115)
	decision := fixtureGrantDecision(t, subject, 5, 1_000, fixtureGrant())
	_, verified := fixtureVerified(t, authority, decision, subject)

	cases := []struct {
		name    string
		request lease.AdvanceRequest
	}{
		{name: "both sides unset", request: lease.AdvanceRequest{}},
		{name: "current unset", request: lease.AdvanceRequest{Candidate: verified}},
		{name: "candidate unset", request: lease.AdvanceRequest{Current: verified}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := lease.Advance(tc.request)
			if !errors.Is(err, core.ErrLeaseContract) ||
				!errors.Is(err, core.ErrLeaseVerification) {
				t.Fatalf("lease.Advance() error = %v, want contract and verification identities", err)
			}
			if got != (lease.AdvanceResult{}) {
				t.Fatalf("lease.Advance() = %v, want zero", got)
			}
			if got.State() != lease.AdvanceStateUnknown {
				t.Fatalf("AdvanceResult.State() = %v, want %v", got.State(), lease.AdvanceStateUnknown)
			}
			if _, verifiedErr := got.Verified(); !errors.Is(verifiedErr, core.ErrLeaseContract) {
				t.Fatalf("AdvanceResult.Verified() error = %v, want %v", verifiedErr, core.ErrLeaseContract)
			}
		})
	}
}

// TestAdvanceUnchangedRetainsTheOriginalProof proves an identical replay keeps
// the caller's already-trusted proof carrier rather than adopting the new
// envelope, so a replayed body signed a second time cannot swap the retained
// attestation.
func TestAdvanceUnchangedRetainsTheOriginalProof(t *testing.T) {
	t.Parallel()

	currentAuthority := fixtureAuthority(t, 116)
	candidateAuthority := fixtureAuthority(t, 117)
	trusted, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
		Keys: []core.Ed25519PublicKey{
			currentAuthority.public,
			candidateAuthority.public,
		},
	})
	if err != nil {
		t.Fatalf("attest.NewTrustedKeys() error = %v, want nil", err)
	}
	currentAuthority.trusted = trusted
	candidateAuthority.trusted = trusted

	subject := fixtureSubject(t, 117)
	decision := fixtureGrantDecision(t, subject, 5, 1_000, fixtureGrant())
	currentDocument, current := fixtureVerified(t, currentAuthority, decision, subject)
	candidateDocument, candidate := fixtureVerified(t, candidateAuthority, decision, subject)
	if currentDocument.Attestation == candidateDocument.Attestation {
		t.Fatalf("distinct signing keys produced equal envelopes; replay proof is vacuous")
	}

	got, err := lease.Advance(lease.AdvanceRequest{
		Current: current, Candidate: candidate,
	})
	if err != nil {
		t.Fatalf("lease.Advance() error = %v, want nil", err)
	}
	if got.State() != lease.AdvanceStateUnchanged {
		t.Fatalf("AdvanceResult.State() = %v, want %v", got.State(), lease.AdvanceStateUnchanged)
	}
	selected, selectedErr := got.Verified()
	if selectedErr != nil {
		t.Fatalf("AdvanceResult.Verified() error = %v, want nil", selectedErr)
	}
	if selected != current {
		t.Fatalf("AdvanceResult.Verified() adopted the candidate proof, want the current proof")
	}
	envelope, envelopeErr := selected.Envelope()
	if envelopeErr != nil || envelope != currentDocument.Attestation {
		t.Fatalf("Verified.Envelope() = (%v, %v), want the current attestation", envelope, envelopeErr)
	}
}

func TestAdvanceRefusalCanRestoreImmediately(t *testing.T) {
	t.Parallel()

	authority := fixtureAuthority(t, 101)
	subject := fixtureSubject(t, 102)
	refusal, err := lease.NewRefusalDecision(lease.RefusalDecisionRequest{
		Header: fixtureHeader(t, subject, 10, 10_000),
		Refusal: lease.Refusal{
			ContactAfter: fixtureInstant(10_001),
		},
	})
	if err != nil {
		t.Fatalf("lease.NewRefusalDecision() error = %v, want nil", err)
	}
	grant := lease.Grant{
		NotBefore:    fixtureInstant(10_002),
		ContactAfter: fixtureInstant(20_000),
		NotAfter:     fixtureInstant(30_000),
		GoodUntil:    fixtureInstant(40_000),
	}
	restored := fixtureGrantDecision(t, subject, 11, 10_002, grant)
	_, current := fixtureVerified(t, authority, refusal, subject)
	_, candidate := fixtureVerified(t, authority, restored, subject)

	got, err := lease.Advance(lease.AdvanceRequest{Current: current, Candidate: candidate})
	if err != nil {
		t.Fatalf("lease.Advance() error = %v, want nil", err)
	}
	if got.State() != lease.AdvanceStateAdvanced {
		t.Fatalf("AdvanceResult.State() = %v, want %v", got.State(), lease.AdvanceStateAdvanced)
	}
}
