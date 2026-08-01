package lease_test

import (
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
)

func TestDecisionLayerTriad(t *testing.T) {
	t.Parallel()

	subject := fixtureSubject(t, 1)
	cases := []struct {
		wantErr error
		build   func(testing.TB) (lease.Decision, error)
		name    string
		want    lease.Outcome
	}{
		{
			name: "positive exact grant closes",
			build: func(tb testing.TB) (lease.Decision, error) {
				return lease.NewGrantDecision(lease.GrantDecisionRequest{
					Header: fixtureHeader(tb, subject, 1, 1_000),
					Grant:  fixtureGrant(),
				})
			},
			want: lease.OutcomeGrant,
		},
		{
			name: "negative empty continuity interval is refused",
			build: func(tb testing.TB) (lease.Decision, error) {
				grant := fixtureGrant()
				grant.GoodUntil = grant.NotAfter
				return lease.NewGrantDecision(lease.GrantDecisionRequest{
					Header: fixtureHeader(tb, subject, 1, 1_000),
					Grant:  grant,
				})
			},
			wantErr: core.ErrLeaseContract,
		},
		{
			name: "negative next contact before issuance is refused",
			build: func(tb testing.TB) (lease.Decision, error) {
				grant := fixtureGrant()
				return lease.NewGrantDecision(lease.GrantDecisionRequest{
					Header: fixtureHeader(tb, subject, 1, 3_001),
					Grant:  grant,
				})
			},
			wantErr: core.ErrLeaseContract,
		},
		{
			name: "neutral recoverable refusal closes without a grant",
			build: func(tb testing.TB) (lease.Decision, error) {
				return lease.NewRefusalDecision(lease.RefusalDecisionRequest{
					Header: fixtureHeader(tb, subject, 2, 5_000),
					Refusal: lease.Refusal{
						ContactAfter: fixtureInstant(6_000),
					},
				})
			},
			want: lease.OutcomeRefusal,
		},
		{
			name: "neutral for-cause revocation closes without contact",
			build: func(tb testing.TB) (lease.Decision, error) {
				return lease.NewRevocationDecision(lease.RevocationDecisionRequest{
					Header: fixtureHeader(tb, subject, 3, 7_000),
					Revocation: lease.Revocation{
						Reason: lease.RevocationReasonLicenceBreach,
					},
				})
			},
			want: lease.OutcomeRevocation,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := tc.build(t)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("decision constructor error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (lease.Decision{}) {
					t.Fatalf("rejected decision = %v, want zero", got)
				}
				return
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("Decision.Validate() error = %v, want nil", err)
			}
			if got.Outcome() != tc.want {
				t.Fatalf("Decision.Outcome() = %v, want %v", got.Outcome(), tc.want)
			}
		})
	}
}

func TestGrantTimelineBoundaryPressure(t *testing.T) {
	t.Parallel()

	valid := fixtureGrant()
	cases := []struct {
		wantErr error
		mutate  func(lease.Grant) lease.Grant
		name    string
	}{
		{name: "minimum ordinary fixture", mutate: func(g lease.Grant) lease.Grant { return g }},
		{name: "contact equals not before", mutate: func(g lease.Grant) lease.Grant {
			g.ContactAfter = g.NotBefore
			return g
		}},
		{name: "contact equals not after", mutate: func(g lease.Grant) lease.Grant {
			g.ContactAfter = g.NotAfter
			return g
		}},
		{name: "one nanosecond continuity", mutate: func(g lease.Grant) lease.Grant {
			g.GoodUntil = fixtureInstant(4_001)
			return g
		}},
		{name: "maximum good until", mutate: func(g lease.Grant) lease.Grant {
			g.GoodUntil = fixtureInstant(math.MaxInt64)
			return g
		}},
		{name: "contact one before not before", mutate: func(g lease.Grant) lease.Grant {
			g.ContactAfter = fixtureInstant(1_999)
			return g
		}, wantErr: core.ErrLeaseContract},
		{name: "not before equals not after", mutate: func(g lease.Grant) lease.Grant {
			g.NotBefore = g.NotAfter
			return g
		}, wantErr: core.ErrLeaseContract},
		{name: "not before one after not after", mutate: func(g lease.Grant) lease.Grant {
			g.NotBefore = fixtureInstant(4_001)
			return g
		}, wantErr: core.ErrLeaseContract},
		{name: "contact one after not after", mutate: func(g lease.Grant) lease.Grant {
			g.ContactAfter = fixtureInstant(4_001)
			return g
		}, wantErr: core.ErrLeaseContract},
		{name: "not after equals good until", mutate: func(g lease.Grant) lease.Grant {
			g.GoodUntil = g.NotAfter
			return g
		}, wantErr: core.ErrLeaseContract},
		{name: "not after one beyond good until", mutate: func(g lease.Grant) lease.Grant {
			g.GoodUntil = fixtureInstant(3_999)
			return g
		}, wantErr: core.ErrLeaseContract},
		{name: "unset not before", mutate: func(g lease.Grant) lease.Grant {
			g.NotBefore = lease.Grant{}.NotBefore
			return g
		}, wantErr: core.ErrLeaseContract},
		{name: "unset contact", mutate: func(g lease.Grant) lease.Grant {
			g.ContactAfter = lease.Grant{}.ContactAfter
			return g
		}, wantErr: core.ErrLeaseContract},
		{name: "unset not after", mutate: func(g lease.Grant) lease.Grant {
			g.NotAfter = lease.Grant{}.NotAfter
			return g
		}, wantErr: core.ErrLeaseContract},
		{name: "unset good until", mutate: func(g lease.Grant) lease.Grant {
			g.GoodUntil = lease.Grant{}.GoodUntil
			return g
		}, wantErr: core.ErrLeaseContract},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := tc.mutate(valid)
			err := got.Validate()
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Grant.Validate() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestDecisionOutcomeJSONRoundTrips(t *testing.T) {
	t.Parallel()

	subject := fixtureSubject(t, 4)
	grant := fixtureGrantDecision(t, subject, 1, 1_000, fixtureGrant())
	refusal, err := lease.NewRefusalDecision(lease.RefusalDecisionRequest{
		Header: fixtureHeader(t, subject, 2, 5_000),
		Refusal: lease.Refusal{
			ContactAfter: fixtureInstant(6_000),
		},
	})
	if err != nil {
		t.Fatalf("lease.NewRefusalDecision() error = %v, want nil", err)
	}
	revocation, err := lease.NewRevocationDecision(lease.RevocationDecisionRequest{
		Header: fixtureHeader(t, subject, 3, 7_000),
		Revocation: lease.Revocation{
			Reason: lease.RevocationReasonInsolvency,
		},
	})
	if err != nil {
		t.Fatalf("lease.NewRevocationDecision() error = %v, want nil", err)
	}
	cases := []struct {
		name     string
		decision lease.Decision
	}{
		{name: "grant tagged union", decision: grant},
		{name: "refusal tagged union", decision: refusal},
		{name: "revocation tagged union", decision: revocation},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			encoded, err := json.Marshal(tc.decision)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v, want nil", err)
			}
			var got lease.Decision
			if err := got.UnmarshalJSON(encoded); err != nil {
				t.Fatalf("Decision.UnmarshalJSON() error = %v, want nil", err)
			}
			if got != tc.decision {
				t.Fatalf("Decision JSON round trip changed typed decision")
			}
		})
	}
}

func TestDecisionVariantProjectionPressure(t *testing.T) {
	t.Parallel()

	subject := fixtureSubject(t, 5)
	grant := fixtureGrantDecision(t, subject, 1, 1_000, fixtureGrant())
	refusalPayload := lease.Refusal{
		ContactAfter: fixtureInstant(6_000),
	}
	refusal, err := lease.NewRefusalDecision(lease.RefusalDecisionRequest{
		Header: fixtureHeader(t, subject, 2, 5_000), Refusal: refusalPayload,
	})
	if err != nil {
		t.Fatalf("lease.NewRefusalDecision() error = %v, want nil", err)
	}
	revocationPayload := lease.Revocation{
		Reason: lease.RevocationReasonSecurityOrPlatformRisk,
	}
	revocation, err := lease.NewRevocationDecision(lease.RevocationDecisionRequest{
		Header: fixtureHeader(t, subject, 3, 7_000), Revocation: revocationPayload,
	})
	if err != nil {
		t.Fatalf("lease.NewRevocationDecision() error = %v, want nil", err)
	}
	cases := []struct {
		name     string
		decision lease.Decision
		outcome  lease.Outcome
	}{
		{name: "grant", decision: grant, outcome: lease.OutcomeGrant},
		{name: "refusal", decision: refusal, outcome: lease.OutcomeRefusal},
		{name: "revocation", decision: revocation, outcome: lease.OutcomeRevocation},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotGrant, grantErr := tc.decision.Grant()
			gotRefusal, refusalErr := tc.decision.Refusal()
			gotRevocation, revocationErr := tc.decision.Revocation()
			if tc.outcome == lease.OutcomeGrant {
				if grantErr != nil || gotGrant != fixtureGrant() {
					t.Fatalf("Decision.Grant() = (%v, %v), want fixture and nil", gotGrant, grantErr)
				}
			} else if !errors.Is(grantErr, core.ErrLeaseContract) ||
				gotGrant != (lease.Grant{}) {
				t.Fatalf("Decision.Grant() = (%v, %v), want zero and contract error", gotGrant, grantErr)
			}
			if tc.outcome == lease.OutcomeRefusal {
				if refusalErr != nil || gotRefusal != refusalPayload {
					t.Fatalf("Decision.Refusal() = (%v, %v), want payload and nil", gotRefusal, refusalErr)
				}
			} else if !errors.Is(refusalErr, core.ErrLeaseContract) ||
				gotRefusal != (lease.Refusal{}) {
				t.Fatalf("Decision.Refusal() = (%v, %v), want zero and contract error", gotRefusal, refusalErr)
			}
			if tc.outcome == lease.OutcomeRevocation {
				if revocationErr != nil || gotRevocation != revocationPayload {
					t.Fatalf("Decision.Revocation() = (%v, %v), want payload and nil", gotRevocation, revocationErr)
				}
			} else if !errors.Is(revocationErr, core.ErrLeaseContract) ||
				gotRevocation != (lease.Revocation{}) {
				t.Fatalf("Decision.Revocation() = (%v, %v), want zero and contract error", gotRevocation, revocationErr)
			}
		})
	}
}
