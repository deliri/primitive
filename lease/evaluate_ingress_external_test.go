package lease_test

import (
	"errors"
	"math"
	"testing"
	"testing/synctest"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
	"github.com/deliri/primitive/v2026/temporal"
)

// TestEvaluateIngressPressure proves the pure evaluation ingress closes every
// supplied fact before any classification runs. Evaluate is the gate a consumer
// asks before creating paid work, so an unset proof, an unset high-water, an
// unusable observation, or a reversed observation pair must stop at ingress
// rather than produce a plausible assessment from partial state.
func TestEvaluateIngressPressure(t *testing.T) {
	t.Parallel()

	authority := fixtureAuthority(t, 61)
	subject := fixtureSubject(t, 63)
	decision := fixtureGrantDecision(t, subject, 1, 1_000, fixtureGrant())
	_, verified := fixtureVerified(t, authority, decision, subject)
	valid := lease.EvaluateRequest{
		Decision:         verified,
		DurableHighWater: fixtureInstant(1_000),
		StartedAt:        fixtureObservation(t, 2_500),
		ObservedAt:       fixtureObservation(t, 2_500),
	}

	cases := []struct {
		mutate   func(lease.EvaluateRequest) lease.EvaluateRequest
		wantErr  error
		wantAlso error
		name     string
	}{
		{
			name:   "complete ingress closes",
			mutate: func(r lease.EvaluateRequest) lease.EvaluateRequest { return r },
		},
		{
			name: "identical start and observation is a zero elapsed span",
			mutate: func(r lease.EvaluateRequest) lease.EvaluateRequest {
				r.ObservedAt = r.StartedAt
				return r
			},
		},
		{
			name: "unset proof carrier",
			mutate: func(r lease.EvaluateRequest) lease.EvaluateRequest {
				r.Decision = lease.Verified{}
				return r
			},
			wantErr:  core.ErrLeaseContract,
			wantAlso: core.ErrLeaseVerification,
		},
		{
			name: "unset durable high water",
			mutate: func(r lease.EvaluateRequest) lease.EvaluateRequest {
				r.DurableHighWater = temporal.Instant{}
				return r
			},
			wantErr: core.ErrLeaseContract,
		},
		{
			name: "unset start observation",
			mutate: func(r lease.EvaluateRequest) lease.EvaluateRequest {
				r.StartedAt = temporal.Observation{}
				return r
			},
			wantErr: core.ErrLeaseContract,
		},
		{
			name: "unset current observation",
			mutate: func(r lease.EvaluateRequest) lease.EvaluateRequest {
				r.ObservedAt = temporal.Observation{}
				return r
			},
			wantErr: core.ErrLeaseContract,
		},
		{
			name: "both observations unset",
			mutate: func(r lease.EvaluateRequest) lease.EvaluateRequest {
				r.StartedAt = temporal.Observation{}
				r.ObservedAt = temporal.Observation{}
				return r
			},
			wantErr: core.ErrLeaseContract,
		},
		{
			name: "current observation precedes the start by one nanosecond",
			mutate: func(r lease.EvaluateRequest) lease.EvaluateRequest {
				r.StartedAt = fixtureObservation(t, 2_501)
				r.ObservedAt = fixtureObservation(t, 2_500)
				return r
			},
			wantErr: core.ErrLeaseContract,
		},
		{
			name: "current observation precedes the start by a full day",
			mutate: func(r lease.EvaluateRequest) lease.EvaluateRequest {
				r.StartedAt = fixtureObservation(t, 24*int64(time.Hour))
				r.ObservedAt = fixtureObservation(t, 0)
				return r
			},
			wantErr: core.ErrLeaseContract,
		},
		{
			name: "elapsed span exceeds a representable duration",
			mutate: func(r lease.EvaluateRequest) lease.EvaluateRequest {
				r.StartedAt = fixtureObservation(t, math.MinInt64)
				r.ObservedAt = fixtureObservation(t, math.MaxInt64)
				return r
			},
			wantErr: core.ErrLeaseContract,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := tc.mutate(valid)
			if err := request.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("EvaluateRequest.Validate() error = %v, want %v", err, tc.wantErr)
			}
			got, err := lease.Evaluate(request)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("lease.Evaluate() error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantAlso != nil && !errors.Is(err, tc.wantAlso) {
				t.Fatalf("lease.Evaluate() error = %v, want nested %v", err, tc.wantAlso)
			}
			if tc.wantErr != nil {
				if got != (lease.Assessment{}) {
					t.Fatalf("lease.Evaluate() = %v, want zero", got)
				}
				return
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("Assessment.Validate() error = %v, want nil", err)
			}
		})
	}
}

// TestEvaluateUsesRealGoMonotonicObservations integrates Lease with
// temporal.Observe instead of constructing wall-only observations. Synctest
// advances Go's clock deterministically, so the shipped observation path must
// carry the exact elapsed duration into Lease evaluation. Reprojecting only
// the ending wall reading then proves the same real monotonic span detects an
// excessive mid-run wall rollback.
func TestEvaluateUsesRealGoMonotonicObservations(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		startedAt, err := temporal.Observe()
		if err != nil {
			t.Fatalf("temporal.Observe() error = %v, want nil", err)
		}
		startWall, err := startedAt.Instant()
		if err != nil {
			t.Fatalf("Observation.Instant() error = %v, want nil", err)
		}
		sevenMinutes, err := temporal.DurationFromMinutes(7)
		if err != nil {
			t.Fatalf("temporal.DurationFromMinutes() error = %v, want nil", err)
		}
		oneMinute, err := temporal.DurationFromMinutes(1)
		if err != nil {
			t.Fatalf("temporal.DurationFromMinutes() error = %v, want nil", err)
		}
		thirtyMinutes, err := temporal.DurationFromMinutes(30)
		if err != nil {
			t.Fatalf("temporal.DurationFromMinutes() error = %v, want nil", err)
		}
		sixHundredMinutes, err := temporal.DurationFromMinutes(600)
		if err != nil {
			t.Fatalf("temporal.DurationFromMinutes() error = %v, want nil", err)
		}
		nineHundredMinutes, err := temporal.DurationFromMinutes(900)
		if err != nil {
			t.Fatalf("temporal.DurationFromMinutes() error = %v, want nil", err)
		}
		contactAfter, err := startWall.Add(thirtyMinutes)
		if err != nil {
			t.Fatalf("Instant.Add(contact) error = %v, want nil", err)
		}
		notAfter, err := startWall.Add(sixHundredMinutes)
		if err != nil {
			t.Fatalf("Instant.Add(normal) error = %v, want nil", err)
		}
		goodUntil, err := startWall.Add(nineHundredMinutes)
		if err != nil {
			t.Fatalf("Instant.Add(continuity) error = %v, want nil", err)
		}

		authority := fixtureAuthority(t, 75)
		subject := fixtureSubject(t, 76)
		headerNanoseconds, err := startWall.Nanoseconds()
		if err != nil {
			t.Fatalf("Instant.Nanoseconds() error = %v, want nil", err)
		}
		decision := fixtureGrantDecision(t, subject, 1, headerNanoseconds, lease.Grant{
			NotBefore:    startWall,
			ContactAfter: contactAfter,
			NotAfter:     notAfter,
			GoodUntil:    goodUntil,
		})
		_, verified := fixtureVerified(t, authority, decision, subject)

		timer := time.NewTimer(7 * time.Minute)
		<-timer.C
		observedAt, err := temporal.Observe()
		if err != nil {
			t.Fatalf("temporal.Observe() error = %v, want nil", err)
		}
		got, err := lease.Evaluate(lease.EvaluateRequest{
			StartedAt:        startedAt,
			ObservedAt:       observedAt,
			Decision:         verified,
			DurableHighWater: startWall,
		})
		if err != nil {
			t.Fatalf("lease.Evaluate() error = %v, want nil", err)
		}
		effective, err := got.EffectiveAt()
		if err != nil {
			t.Fatalf("Assessment.EffectiveAt() error = %v, want nil", err)
		}
		want, err := startWall.Add(sevenMinutes)
		if err != nil {
			t.Fatalf("Instant.Add(elapsed) error = %v, want nil", err)
		}
		comparison, err := effective.Compare(want)
		if err != nil || comparison != core.ComparisonEqual {
			t.Fatalf(
				"effective comparison = %v, error = %v, want equal and nil",
				comparison,
				err,
			)
		}
		if got.State() != lease.StateCurrent ||
			got.ContactState() != lease.ContactStateNotDue {
			t.Fatalf(
				"assessment = %v/%v, want %v/%v",
				got.State(),
				got.ContactState(),
				lease.StateCurrent,
				lease.ContactStateNotDue,
			)
		}

		rolledBackWall, err := startWall.Add(oneMinute)
		if err != nil {
			t.Fatalf("Instant.Add(rolled-back wall) error = %v, want nil", err)
		}
		rolledBackObservation, err := observedAt.WithWall(rolledBackWall)
		if err != nil {
			t.Fatalf("Observation.WithWall() error = %v, want nil", err)
		}
		rejected, err := lease.Evaluate(lease.EvaluateRequest{
			StartedAt:        startedAt,
			ObservedAt:       rolledBackObservation,
			Decision:         verified,
			DurableHighWater: startWall,
		})
		if !errors.Is(err, core.ErrLeaseClock) ||
			rejected != (lease.Assessment{}) {
			t.Fatalf(
				"wall-rollback lease.Evaluate() = (%v, %v), want zero/%v",
				rejected,
				err,
				core.ErrLeaseClock,
			)
		}
		var contradiction lease.ClockContradiction
		if !errors.As(err, &contradiction) ||
			contradiction.Observed != rolledBackWall ||
			contradiction.Trusted != want {
			t.Fatalf(
				"wall-rollback contradiction = %+v from %v, want observed/trusted %v/%v",
				contradiction,
				err,
				rolledBackWall,
				want,
			)
		}
	})
}

// TestEvaluateSurvivesUptimeBeyondTheRollbackTolerance pins the documented
// consumer loop: one process observes its start once, keeps observing, and
// feeds back the high-water it durably committed on the previous evaluation.
//
// Trusted progress must be anchored to the start observation's own wall
// reading. Anchoring it to the floor instead counts the interval between the
// start observation and the committed high-water twice, so the effective clock
// runs fast and, once uptime passes the rollback tolerance, the process rejects
// its own committed high-water as a clock contradiction.
func TestEvaluateSurvivesUptimeBeyondTheRollbackTolerance(t *testing.T) {
	t.Parallel()

	minute := int64(temporal.NanosecondsPerMinute)
	authority := fixtureAuthority(t, 71)
	subject := fixtureSubject(t, 73)
	grant := lease.Grant{
		NotBefore:    fixtureInstant(10 * minute),
		ContactAfter: fixtureInstant(30 * minute),
		NotAfter:     fixtureInstant(600 * minute),
		GoodUntil:    fixtureInstant(900 * minute),
	}
	decision := fixtureGrantDecision(t, subject, 1, 10*minute, grant)
	_, verified := fixtureVerified(t, authority, decision, subject)

	// One start observation for the whole process, exactly as a daemon holds it.
	startedAt := fixtureObservation(t, 10*minute)
	durable := fixtureInstant(10 * minute)

	for tick := 1; tick <= 12; tick++ {
		wall := 10*minute + int64(tick)*10*minute
		got, err := lease.Evaluate(lease.EvaluateRequest{
			Decision:         verified,
			DurableHighWater: durable,
			StartedAt:        startedAt,
			ObservedAt:       fixtureObservation(t, wall),
		})
		if err != nil {
			t.Fatalf("tick %d lease.Evaluate() error = %v, want nil", tick, err)
		}
		effective, effectiveErr := got.EffectiveAt()
		if effectiveErr != nil {
			t.Fatalf("tick %d Assessment.EffectiveAt() error = %v, want nil", tick, effectiveErr)
		}
		nanoseconds, nanosecondErr := effective.Nanoseconds()
		if nanosecondErr != nil {
			t.Fatalf("tick %d Instant.Nanoseconds() error = %v, want nil", tick, nanosecondErr)
		}
		if nanoseconds != wall {
			t.Fatalf("tick %d effective = %d, want the real wall reading %d", tick, nanoseconds, wall)
		}
		durable = effective
	}
	if durable != fixtureInstant(130*minute) {
		t.Fatalf("final committed high water = %v, want the final wall reading", durable)
	}
}

// TestEvaluateHighWaterNeverRegresses proves the returned high-water is
// monotone in every supplied fact. The consumer persists this instant and feeds
// it back as the next floor, so a single regression would permanently reopen an
// expired timeline on the next evaluation.
func TestEvaluateHighWaterNeverRegresses(t *testing.T) {
	t.Parallel()

	minute := int64(temporal.NanosecondsPerMinute)
	authority := fixtureAuthority(t, 65)
	subject := fixtureSubject(t, 67)
	grant := lease.Grant{
		NotBefore:    fixtureInstant(10 * minute),
		ContactAfter: fixtureInstant(11 * minute),
		NotAfter:     fixtureInstant(12 * minute),
		GoodUntil:    fixtureInstant(13 * minute),
	}
	decision := fixtureGrantDecision(t, subject, 1, 10*minute, grant)
	_, verified := fixtureVerified(t, authority, decision, subject)

	cases := []struct {
		name        string
		durable     int64
		start       int64
		finish      int64
		wantAtLeast int64
	}{
		{name: "issuance floor binds a slow wall", durable: 10 * minute, start: 10 * minute, finish: 10 * minute, wantAtLeast: 10 * minute},
		{name: "durable floor binds an equal wall", durable: 11 * minute, start: 11 * minute, finish: 11 * minute, wantAtLeast: 11 * minute},
		{name: "durable floor binds a lagging wall", durable: 12 * minute, start: 12*minute - 1, finish: 12*minute - 1, wantAtLeast: 12 * minute},
		{name: "monotonic elapsed binds a stalled wall", durable: 10 * minute, start: 10 * minute, finish: 12 * minute, wantAtLeast: 12 * minute},
		{name: "wall ahead of every floor binds", durable: 10 * minute, start: 10 * minute, finish: 60 * minute, wantAtLeast: 60 * minute},
		{name: "far future durable floor binds", durable: 1_000 * minute, start: 1_000 * minute, finish: 1_000 * minute, wantAtLeast: 1_000 * minute},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := lease.Evaluate(lease.EvaluateRequest{
				Decision:         verified,
				DurableHighWater: fixtureInstant(tc.durable),
				StartedAt:        fixtureObservation(t, tc.start),
				ObservedAt:       fixtureObservation(t, tc.finish),
			})
			if err != nil {
				t.Fatalf("lease.Evaluate() error = %v, want nil", err)
			}
			effective, effectiveErr := got.EffectiveAt()
			if effectiveErr != nil {
				t.Fatalf("Assessment.EffectiveAt() error = %v, want nil", effectiveErr)
			}
			nanoseconds, nanosecondErr := effective.Nanoseconds()
			if nanosecondErr != nil {
				t.Fatalf("Instant.Nanoseconds() error = %v, want nil", nanosecondErr)
			}
			if nanoseconds < tc.wantAtLeast {
				t.Fatalf("Assessment.EffectiveAt() = %d, want at least %d", nanoseconds, tc.wantAtLeast)
			}
			if nanoseconds < tc.durable {
				t.Fatalf("Assessment.EffectiveAt() = %d, regressed below the durable floor %d", nanoseconds, tc.durable)
			}
			if nanoseconds < 10*minute {
				t.Fatalf("Assessment.EffectiveAt() = %d, regressed below the signed issuance %d", nanoseconds, 10*minute)
			}

			// Feeding the returned high-water straight back must be a fixed
			// point: the consumer does exactly this on the next evaluation.
			replayed, replayErr := lease.Evaluate(lease.EvaluateRequest{
				Decision:         verified,
				DurableHighWater: effective,
				StartedAt:        fixtureObservation(t, tc.start),
				ObservedAt:       fixtureObservation(t, tc.finish),
			})
			if replayErr != nil {
				t.Fatalf("replayed lease.Evaluate() error = %v, want nil", replayErr)
			}
			replayedEffective, replayedErr := replayed.EffectiveAt()
			if replayedErr != nil {
				t.Fatalf("replayed Assessment.EffectiveAt() error = %v, want nil", replayedErr)
			}
			comparison, compareErr := replayedEffective.Compare(effective)
			if compareErr != nil || comparison != core.ComparisonEqual {
				t.Fatalf("replayed high water comparison = %v, error = %v, want equal and nil",
					comparison, compareErr)
			}
			if replayed.State() != got.State() || replayed.ContactState() != got.ContactState() {
				t.Fatalf("replayed assessment = %v/%v, want %v/%v",
					replayed.State(), replayed.ContactState(), got.State(), got.ContactState())
			}
		})
	}
}
