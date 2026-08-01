package lease_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestEvaluateGrantBoundaryPressure(t *testing.T) {
	t.Parallel()

	authority := fixtureAuthority(t, 61)
	subject := fixtureSubject(t, 62)
	decision := fixtureGrantDecision(t, subject, 1, 1_000, fixtureGrant())
	_, verified := fixtureVerified(t, authority, decision, subject)

	cases := []struct {
		name        string
		at          int64
		wantState   lease.State
		wantContact lease.ContactState
	}{
		{name: "one before not before", at: 1_999, wantState: lease.StateNotYetValid, wantContact: lease.ContactStateNotDue},
		{name: "at not before", at: 2_000, wantState: lease.StateCurrent, wantContact: lease.ContactStateNotDue},
		{name: "one after not before", at: 2_001, wantState: lease.StateCurrent, wantContact: lease.ContactStateNotDue},
		{name: "one before contact", at: 2_999, wantState: lease.StateCurrent, wantContact: lease.ContactStateNotDue},
		{name: "at contact", at: 3_000, wantState: lease.StateCurrent, wantContact: lease.ContactStateDue},
		{name: "one after contact", at: 3_001, wantState: lease.StateCurrent, wantContact: lease.ContactStateDue},
		{name: "one before not after", at: 3_999, wantState: lease.StateCurrent, wantContact: lease.ContactStateDue},
		{name: "at not after", at: 4_000, wantState: lease.StateContinuity, wantContact: lease.ContactStateDue},
		{name: "one after not after", at: 4_001, wantState: lease.StateContinuity, wantContact: lease.ContactStateDue},
		{name: "one before good until", at: 4_999, wantState: lease.StateContinuity, wantContact: lease.ContactStateDue},
		{name: "at good until", at: 5_000, wantState: lease.StateExpired, wantContact: lease.ContactStateDue},
		{name: "one after good until", at: 5_001, wantState: lease.StateExpired, wantContact: lease.ContactStateDue},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			observation := fixtureObservation(t, tc.at)
			got, err := lease.Evaluate(lease.EvaluateRequest{
				Decision: verified, DurableHighWater: fixtureInstant(1_000),
				StartedAt: observation, ObservedAt: observation,
			})
			if err != nil {
				t.Fatalf("lease.Evaluate() error = %v, want nil", err)
			}
			if got.State() != tc.wantState {
				t.Fatalf("Assessment.State() = %v, want %v", got.State(), tc.wantState)
			}
			if got.ContactState() != tc.wantContact {
				t.Fatalf("Assessment.ContactState() = %v, want %v", got.ContactState(), tc.wantContact)
			}
			gotDecision, decisionErr := got.Decision()
			if decisionErr != nil || gotDecision != decision {
				t.Fatalf("Assessment.Decision() differs from authentic decision or returned error %v", decisionErr)
			}
			effective, effectiveErr := got.EffectiveAt()
			if effectiveErr != nil {
				t.Fatalf("Assessment.EffectiveAt() error = %v, want nil", effectiveErr)
			}
			if comparison, compareErr := effective.Compare(fixtureInstant(tc.at)); compareErr != nil ||
				comparison != core.ComparisonEqual {
				t.Fatalf("Assessment.EffectiveAt() comparison = %v, error = %v, want equal and nil", comparison, compareErr)
			}
		})
	}
}

func TestEvaluateOutcomeLayerTriad(t *testing.T) {
	t.Parallel()

	authority := fixtureAuthority(t, 71)
	subject := fixtureSubject(t, 72)
	cases := []struct {
		decision    func(testing.TB) lease.Decision
		name        string
		at          int64
		wantState   lease.State
		wantContact lease.ContactState
	}{
		{
			name: "positive grant creates current work",
			decision: func(tb testing.TB) lease.Decision {
				return fixtureGrantDecision(tb, subject, 1, 1_000, fixtureGrant())
			},
			at: 2_500, wantState: lease.StateCurrent,
			wantContact: lease.ContactStateNotDue,
		},
		{
			name: "negative refusal stops work but preserves contact cadence",
			decision: func(tb testing.TB) lease.Decision {
				value, err := lease.NewRefusalDecision(lease.RefusalDecisionRequest{
					Header: fixtureHeader(tb, subject, 2, 5_000),
					Refusal: lease.Refusal{
						ContactAfter: fixtureInstant(6_000),
					},
				})
				if err != nil {
					tb.Fatalf("lease.NewRefusalDecision() error = %v, want nil", err)
				}
				return value
			},
			at: 5_500, wantState: lease.StateRefused,
			wantContact: lease.ContactStateNotDue,
		},
		{
			name: "neutral revocation stops work and contact",
			decision: func(tb testing.TB) lease.Decision {
				value, err := lease.NewRevocationDecision(lease.RevocationDecisionRequest{
					Header: fixtureHeader(tb, subject, 3, 7_000),
					Revocation: lease.Revocation{
						Reason: lease.RevocationReasonSecurityOrPlatformRisk,
					},
				})
				if err != nil {
					tb.Fatalf("lease.NewRevocationDecision() error = %v, want nil", err)
				}
				return value
			},
			at: 8_000, wantState: lease.StateRevoked,
			wantContact: lease.ContactStateProhibited,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			decision := tc.decision(t)
			_, verified := fixtureVerified(t, authority, decision, subject)
			observation := fixtureObservation(t, tc.at)
			got, err := lease.Evaluate(lease.EvaluateRequest{
				Decision:         verified,
				DurableHighWater: fixtureInstant(decisionIssuedAt(t, decision)),
				StartedAt:        observation, ObservedAt: observation,
			})
			if err != nil {
				t.Fatalf("lease.Evaluate() error = %v, want nil", err)
			}
			if got.State() != tc.wantState || got.ContactState() != tc.wantContact {
				t.Fatalf("assessment = %v/%v, want %v/%v", got.State(), got.ContactState(), tc.wantState, tc.wantContact)
			}
		})
	}
}

func TestEvaluateClockContradictionPressure(t *testing.T) {
	t.Parallel()

	minute := int64(temporal.NanosecondsPerMinute)
	tolerance := lease.ClockRollbackToleranceNanoseconds
	authority := fixtureAuthority(t, 81)
	subject := fixtureSubject(t, 82)
	grant := lease.Grant{
		NotBefore:    fixtureInstant(10 * minute),
		ContactAfter: fixtureInstant(11 * minute),
		NotAfter:     fixtureInstant(12 * minute),
		GoodUntil:    fixtureInstant(13 * minute),
	}
	decision := fixtureGrantDecision(t, subject, 1, 10*minute, grant)
	_, verified := fixtureVerified(t, authority, decision, subject)

	cases := []struct {
		wantErr       error
		name          string
		durable       int64
		start         int64
		finish        int64
		wantEffective int64
		unsetDurable  bool
	}{
		{name: "exact signed floor", durable: 10 * minute, start: 10 * minute, finish: 10 * minute, wantEffective: 10 * minute},
		{name: "one nanosecond behind signed floor", durable: 10 * minute, start: 10*minute - 1, finish: 10*minute - 1, wantEffective: 10 * minute},
		{name: "exactly at rollback tolerance", durable: 10 * minute, start: 10*minute - tolerance, finish: 10*minute - tolerance, wantEffective: 10 * minute},
		{name: "one nanosecond beyond rollback tolerance", durable: 10 * minute, start: 10*minute - tolerance - 1, finish: 10*minute - tolerance - 1, wantErr: core.ErrLeaseClock},
		{name: "six minutes behind", durable: 10 * minute, start: 4 * minute, finish: 4 * minute, wantErr: core.ErrLeaseClock},
		{name: "durable floor ahead within tolerance", durable: 14 * minute, start: 9 * minute, finish: 9 * minute, wantEffective: 14 * minute},
		{name: "durable floor ahead beyond tolerance", durable: 14 * minute, start: 9*minute - 1, finish: 9*minute - 1, wantErr: core.ErrLeaseClock},
		{name: "monotonic elapsed advances trusted baseline", durable: 10 * minute, start: 10 * minute, finish: 11 * minute, wantEffective: 11 * minute},
		{name: "wall ahead advances high water", durable: 10 * minute, start: 10 * minute, finish: 15 * minute, wantEffective: 15 * minute},
		{name: "unset durable high water", start: 10 * minute, finish: 10 * minute, unsetDurable: true, wantErr: core.ErrLeaseContract},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			durable := fixtureInstant(tc.durable)
			if tc.unsetDurable {
				durable = temporal.Instant{}
			}
			got, err := lease.Evaluate(lease.EvaluateRequest{
				Decision: verified, DurableHighWater: durable,
				StartedAt:  fixtureObservation(t, tc.start),
				ObservedAt: fixtureObservation(t, tc.finish),
			})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("lease.Evaluate() error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (lease.Assessment{}) {
					t.Fatalf("lease.Evaluate() = %v, want zero", got)
				}
				if errors.Is(tc.wantErr, core.ErrLeaseClock) {
					var contradiction lease.ClockContradiction
					if !errors.As(err, &contradiction) {
						t.Fatalf("lease.Evaluate() error = %v, want ClockContradiction", err)
					}
				}
				return
			}
			effective, effectiveErr := got.EffectiveAt()
			if effectiveErr != nil {
				t.Fatalf("Assessment.EffectiveAt() error = %v, want nil", effectiveErr)
			}
			want := fixtureInstant(tc.wantEffective)
			comparison, compareErr := effective.Compare(want)
			if compareErr != nil || comparison != core.ComparisonEqual {
				t.Fatalf("effective comparison = %v, error = %v, want equal and nil", comparison, compareErr)
			}
		})
	}
}

func decisionIssuedAt(tb testing.TB, decision lease.Decision) int64 {
	tb.Helper()

	header, err := decision.Header()
	if err != nil {
		tb.Fatalf("Decision.Header() error = %v, want nil", err)
	}
	value, err := header.IssuedAt.Nanoseconds()
	if err != nil {
		tb.Fatalf("Instant.Nanoseconds() error = %v, want nil", err)
	}
	return value
}
