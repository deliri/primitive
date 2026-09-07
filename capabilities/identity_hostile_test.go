package capabilities

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestIdentityExhaustsCompilerOwnedEffectDomain(t *testing.T) {
	t.Parallel()

	for raw := range 256 {
		effect := Effect(raw)
		identity, err := IdentityForEffect(effect)
		wantValid := effect >= EffectFilesystem && effect < effectLimit
		if wantValid {
			gotEffect, gotErr := identity.Effect()
			if err != nil || gotErr != nil || gotEffect != effect || identity.String() != effect.String() {
				t.Fatalf("IdentityForEffect(%d) = (%v, %v), Effect() = (%v, %v), want exact valid identity", raw, identity, err, gotEffect, gotErr)
			}
			continue
		}
		if !errors.Is(err, core.ErrCapabilitiesContract) || identity != (Identity{}) {
			t.Fatalf("IdentityForEffect(%d) = (%v, %v), want zero and errors.Is(..., %v)", raw, identity, err, core.ErrCapabilitiesContract)
		}
	}
}

func FuzzIdentityJSONSemanticClosure(f *testing.F) {
	for effect := range effectLimit {
		if effect < EffectFilesystem {
			continue
		}
		identity, err := IdentityForEffect(effect)
		if err != nil {
			f.Fatalf("IdentityForEffect(%v) error = %v, want nil", effect, err)
		}
		encoded, err := identity.MarshalJSON()
		if err != nil {
			f.Fatalf("Identity.MarshalJSON(%v) error = %v, want nil", effect, err)
		}
		f.Add(encoded)
	}
	f.Add([]byte{})
	f.Add([]byte(`null`))
	f.Add([]byte(`"future"`))

	f.Fuzz(func(t *testing.T, data []byte) {
		seed, err := IdentityForEffect(EffectFilesystem)
		if err != nil {
			t.Fatalf("IdentityForEffect(seed) error = %v, want nil", err)
		}
		got := seed
		gotErr := got.UnmarshalJSON(data)
		want, admitted := jsonEnumOracle(data, identityDomain())
		if (gotErr == nil) != admitted || (admitted && got != want) {
			t.Fatalf("identity source projection = (%v,%v), want %v admitted %t", got, gotErr, want, admitted)
		}
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrCapabilitiesContract) || got != seed {
				t.Fatalf("Identity.UnmarshalJSON(rejected) = (%v, %v), want preserved and errors.Is(..., %v)", got, gotErr, core.ErrCapabilitiesContract)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("Identity.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil {
			t.Fatalf("Identity.MarshalJSON(accepted) error = %v, want nil", err)
		}
		var roundTrip Identity
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
			t.Fatalf("Identity canonical round trip = (%v, %v), want (%v, nil)", roundTrip, err, got)
		}
	})
}
