package wiring

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestRuntimeGraphDerivationLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive connected graph seals one exact manifest", func(t *testing.T) {
		t.Parallel()
		manifest, err := Derive(wiringRequest(wiringTestIdentityRoot,
			wiringDefinition(wiringTestIdentityRoot, wiringTestIdentityFirst),
			wiringDefinition(wiringTestIdentityFirst),
		))
		if err != nil || manifest.Validate() != nil || manifest.Root() != wiringTestIdentityRoot || manifest.Count() != 2 {
			t.Fatalf("Derive(connected) = (root %v, count %d, %v), want (%v, 2, nil)", manifest.Root(), manifest.Count(), err, wiringTestIdentityRoot)
		}
	})

	t.Run("negative missing dependency returns typed zero manifest", func(t *testing.T) {
		t.Parallel()
		manifest, err := Derive(wiringRequest(wiringTestIdentityRoot,
			wiringDefinition(wiringTestIdentityRoot, wiringTestIdentityFirst),
		))
		var typed *ContractError[wiringTestIdentity]
		if !errors.Is(err, core.ErrPrimitiveContract) || !errors.As(err, &typed) ||
			typed.Kind != ErrorKindMissingDependency || manifest.Count() != 0 || manifest.Root() != wiringTestIdentityUnknown {
			t.Fatalf("Derive(missing) = (root %v, count %d, error %v), want typed missing-dependency and zero manifest", manifest.Root(), manifest.Count(), err)
		}
	})

	t.Run("neutral absent graph invents no components or root", func(t *testing.T) {
		t.Parallel()
		manifest, err := Derive(Request[wiringTestIdentity]{})
		var typed *ContractError[wiringTestIdentity]
		if !errors.Is(err, core.ErrPrimitiveContract) || !errors.As(err, &typed) ||
			typed.Kind != ErrorKindRequest || manifest.Count() != 0 || manifest.Root() != wiringTestIdentityUnknown {
			t.Fatalf("Derive(zero) = (root %v, count %d, error %v), want typed request refusal and zero manifest", manifest.Root(), manifest.Count(), err)
		}
	})
}
