package wiring

import (
	"errors"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestRuntimeGraphDerivationLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive connected graph seals one exact manifest", func(t *testing.T) {
		t.Parallel()
		wantComponents := []Definition[wiringTestIdentity]{
			wiringDefinition(wiringTestIdentityRoot, wiringTestIdentityFirst),
			wiringDefinition(wiringTestIdentityFirst),
		}
		manifest, gotErr := Derive(wiringRequest(wiringTestIdentityRoot, wantComponents...))
		if gotErr != nil {
			t.Fatalf("Derive(connected) error = %v, want nil", gotErr)
		}
		if gotErr := manifest.Validate(); gotErr != nil {
			t.Fatalf("Manifest.Validate(connected) error = %v, want nil", gotErr)
		}
		if gotRoot := manifest.Root(); gotRoot != wiringTestIdentityRoot {
			t.Fatalf("Manifest.Root(connected) = %v, want %v", gotRoot, wiringTestIdentityRoot)
		}
		if gotCount := manifest.Count(); gotCount != len(wantComponents) {
			t.Fatalf("Manifest.Count(connected) = %d, want %d", gotCount, len(wantComponents))
		}
		gotComponents := slices.Collect(manifest.Components())
		if !slices.EqualFunc(gotComponents, wantComponents, equalWiringDefinition) {
			t.Fatalf("Manifest.Components(connected) = %v, want %v", gotComponents, wantComponents)
		}
	})

	t.Run("negative missing dependency returns typed zero manifest", func(t *testing.T) {
		t.Parallel()
		manifest, gotErr := Derive(wiringRequest(wiringTestIdentityRoot,
			wiringDefinition(wiringTestIdentityRoot, wiringTestIdentityFirst),
		))
		var typed *ContractError[wiringTestIdentity]
		if !errors.Is(gotErr, core.ErrPrimitiveContract) {
			t.Fatalf("Derive(missing dependency) error = %v, want %v", gotErr, core.ErrPrimitiveContract)
		}
		if !errors.As(gotErr, &typed) {
			t.Fatalf("Derive(missing dependency) error = %T, want *ContractError", gotErr)
		}
		if typed.Kind != ErrorKindMissingDependency {
			t.Fatalf("ContractError.Kind = %v, want %v", typed.Kind, ErrorKindMissingDependency)
		}
		if gotCount := manifest.Count(); gotCount != 0 {
			t.Fatalf("refused Manifest.Count() = %d, want 0", gotCount)
		}
		if gotRoot := manifest.Root(); gotRoot != wiringTestIdentityUnknown {
			t.Fatalf("refused Manifest.Root() = %v, want %v", gotRoot, wiringTestIdentityUnknown)
		}
	})

	t.Run("neutral absent graph invents no components or root", func(t *testing.T) {
		t.Parallel()
		manifest, gotErr := Derive(Request[wiringTestIdentity]{})
		var typed *ContractError[wiringTestIdentity]
		if !errors.Is(gotErr, core.ErrPrimitiveContract) {
			t.Fatalf("Derive(zero request) error = %v, want %v", gotErr, core.ErrPrimitiveContract)
		}
		if !errors.As(gotErr, &typed) {
			t.Fatalf("Derive(zero request) error = %T, want *ContractError", gotErr)
		}
		if typed.Kind != ErrorKindRequest {
			t.Fatalf("ContractError.Kind = %v, want %v", typed.Kind, ErrorKindRequest)
		}
		if gotCount := manifest.Count(); gotCount != 0 {
			t.Fatalf("neutral Manifest.Count() = %d, want 0", gotCount)
		}
		if gotRoot := manifest.Root(); gotRoot != wiringTestIdentityUnknown {
			t.Fatalf("neutral Manifest.Root() = %v, want %v", gotRoot, wiringTestIdentityUnknown)
		}
	})
}
