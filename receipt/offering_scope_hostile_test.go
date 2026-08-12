package receipt

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestOfferingIdentityForExhaustsClosedOfferingDomainAndKeepsNamespacesDistinct(t *testing.T) {
	t.Parallel()

	seen := make(map[OfferingIdentity]core.Offering)
	for raw := 0; raw <= 255; raw++ {
		offering := core.Offering(raw)
		got, gotErr := OfferingIdentityFor(offering)
		if !offering.IsValid() {
			if !errors.Is(gotErr, core.ErrLifecycleIdentityContract) || got != (OfferingIdentity{}) {
				t.Fatalf("OfferingIdentityFor(%d) = (%v, %v), want zero and errors.Is %v",
					raw, got, gotErr, core.ErrLifecycleIdentityContract)
			}
			continue
		}
		if gotErr != nil || got.Validate() != nil {
			t.Fatalf("OfferingIdentityFor(%v) = (%v, %v), want valid identity and nil", offering, got, gotErr)
		}
		again, err := OfferingIdentityFor(offering)
		if err != nil || again != got {
			t.Fatalf("OfferingIdentityFor(%v) second derivation = (%v, %v), want (%v, nil)", offering, again, err, got)
		}
		if previous, exists := seen[got]; exists {
			t.Fatalf("OfferingIdentityFor(%v) = %v, already assigned to %v", offering, got, previous)
		}
		seen[got] = offering
	}
	if got, want := len(seen), 3; got != want {
		t.Fatalf("distinct derived offering identities = %d, want %d", got, want)
	}
}

func TestScopeForBindsExactAccountAndDerivedOffering(t *testing.T) {
	t.Parallel()

	account, err := NewAccountIdentity([LifecycleIdentityBytes]byte{1})
	if err != nil {
		t.Fatalf("NewAccountIdentity() error = %v, want nil", err)
	}
	for _, offering := range []core.Offering{core.OfferingBug, core.OfferingWitness, core.OfferingPeachfuzz} {
		offering := offering
		t.Run(offering.String(), func(t *testing.T) {
			t.Parallel()

			got, gotErr := ScopeFor(account, offering)
			wantOffering, wantErr := OfferingIdentityFor(offering)
			want := Scope{Account: account, Offering: wantOffering}
			if gotErr != nil || wantErr != nil || got != want {
				t.Fatalf("ScopeFor(%v) = (%v, %v), want (%v, nil); identity error = %v",
					offering, got, gotErr, want, wantErr)
			}
		})
	}
	if got, err := ScopeFor(AccountIdentity{}, core.OfferingWitness); !errors.Is(err, core.ErrReceiptContract) || got != (Scope{}) {
		t.Fatalf("ScopeFor(zero account) = (%v, %v), want zero and errors.Is %v", got, err, core.ErrReceiptContract)
	}
	if got, err := ScopeFor(account, core.OfferingUnknown); !errors.Is(err, core.ErrLifecycleIdentityContract) || got != (Scope{}) {
		t.Fatalf("ScopeFor(unknown offering) = (%v, %v), want zero and errors.Is %v", got, err, core.ErrLifecycleIdentityContract)
	}
}
