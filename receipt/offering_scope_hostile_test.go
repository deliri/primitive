package receipt

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestScopeOfferingLayerTriad(t *testing.T) {
	t.Parallel()

	fixture := newReceiptFixture(t, 36)

	t.Run("positive opaque offering survives canonical scope closure", func(t *testing.T) {
		t.Parallel()
		got, gotErr := ScopeFor(fixture.account, fixture.offering)
		want := Scope{Account: fixture.account, Offering: fixture.offering}
		if gotErr != nil || got != want {
			t.Fatalf("ScopeFor(positive) = (%+v, %v), want (%+v, nil)", got, gotErr, want)
		}
		first, firstErr := json.Marshal(got)
		if firstErr != nil {
			t.Fatalf("json.Marshal(Scope) error = %v, want nil", firstErr)
		}
		var roundTrip Scope
		if roundTripErr := json.Unmarshal(first, &roundTrip); roundTripErr != nil || roundTrip != got {
			t.Fatalf("json.Unmarshal(Scope) = (%+v, %v), want (%+v, nil)", roundTrip, roundTripErr, got)
		}
		second, secondErr := json.Marshal(roundTrip)
		if secondErr != nil || !bytes.Equal(second, first) {
			t.Fatalf("second canonical Scope projection = (%q, %v), want (%q, nil)", second, secondErr, first)
		}
	})

	t.Run("negative invalid owner facts refuse without a partial scope", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			wantErr  error
			name     string
			offering core.Offering
			account  AccountIdentity
		}{
			{name: "unset account", offering: fixture.offering, wantErr: core.ErrLifecycleIdentityContract},
			{name: "unset offering", account: fixture.account, wantErr: core.ErrPrimitiveContract},
			{name: "noncanonical offering", account: fixture.account, offering: core.Offering{Token: "Receipt"}, wantErr: core.ErrPrimitiveContract},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				got, gotErr := ScopeFor(tc.account, tc.offering)
				if !errors.Is(gotErr, core.ErrReceiptContract) || !errors.Is(gotErr, tc.wantErr) || got != (Scope{}) {
					t.Fatalf("ScopeFor(%s) = (%+v, %v), want zero with %v and %v", tc.name, got, gotErr, core.ErrReceiptContract, tc.wantErr)
				}
			})
		}
	})

	t.Run("neutral Primitive preserves identity without interpreting product vocabulary", func(t *testing.T) {
		t.Parallel()
		other := offeringFixture(t, 39)
		first, firstErr := ScopeFor(fixture.account, fixture.offering)
		second, secondErr := ScopeFor(fixture.account, other)
		if firstErr != nil || secondErr != nil || first == second ||
			first.Offering != fixture.offering || second.Offering != other {
			t.Fatalf("opaque ScopeFor neutral pair = (%+v/%v, %+v/%v), want distinct exact offerings", first, firstErr, second, secondErr)
		}
	})
}
