package lease_test

import (
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
)

// Reaffirmation may advance issuance while preserving every approved deadline.
// The contact deadline requests communication; it is not an authority expiry.
func TestGrantReaffirmationPreservesDeadlinesLayerTriad(t *testing.T) {
	t.Parallel()
	subject := fixtureSubject(t, 1)
	grant := fixtureGrant()
	for _, tc := range []struct {
		name    string
		issued  int64
		wantErr error
	}{
		{name: "one before contact retains future contact", issued: 2999},
		{name: "exact contact can be acknowledged", issued: 3000},
		{name: "one after contact does not invalidate authority", issued: 3001},
		{name: "one before normal expiry retains current authority", issued: 3999},
		{name: "exact normal expiry retains continuity", issued: 4000},
		{name: "one after normal expiry retains continuity", issued: 4001},
		{name: "one before hard expiry retains final continuity", issued: 4999},
		{name: "exact hard expiry records already expired authority", issued: 5000},
		{name: "one after hard expiry cannot issue a grant", issued: 5001, wantErr: core.ErrLeaseContract},
		{name: "maximum issuance cannot overflow hard expiry", issued: math.MaxInt64, wantErr: core.ErrLeaseContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			header := fixtureHeader(t, subject, 2, tc.issued)
			got, err := lease.NewGrantDecision(lease.GrantDecisionRequest{Header: header, Grant: grant})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("NewGrantDecision() error = %v, want %v", err, tc.wantErr)
			}
			if err != nil {
				if got != (lease.Decision{}) {
					t.Fatalf("rejected decision = %v, want zero", got)
				}
				return
			}
			gotGrant, err := got.Grant()
			if err != nil || gotGrant != grant {
				t.Fatalf("reaffirmed grant = %v/%v, want %v/nil", gotGrant, err, grant)
			}
			gotHeader, err := got.Header()
			if err != nil || gotHeader != header {
				t.Fatalf("reaffirmed header = %v/%v, want %v/nil", gotHeader, err, header)
			}
		})
	}
}
