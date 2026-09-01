//go:build !linux

package runworkspace_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runworkspace"
)

func TestLinuxResidueSourceIsUnavailableOutsideLinux(t *testing.T) {
	t.Parallel()

	configuration := runworkspace.LinuxResidueConfiguration{}
	if gotErr := configuration.Validate(); !errors.Is(gotErr, core.ErrPrimitiveContract) {
		t.Fatalf("LinuxResidueConfiguration.Validate() error = %v, want errors.Is(..., %v)", gotErr, core.ErrPrimitiveContract)
	}
	source, constructionErr := runworkspace.NewLinuxResidueSource(configuration)
	validationErr := source.Validate()
	got, observationErr := source.ObserveResidue(t.Context())
	if !errors.Is(constructionErr, core.ErrPrimitiveContract) ||
		!errors.Is(validationErr, core.ErrPrimitiveContract) ||
		!errors.Is(observationErr, core.ErrPrimitiveContract) ||
		got != (runworkspace.Residue{}) {
		t.Fatalf("Linux residue outside Linux = construction %v/validation %v/observation (%+v, %v), want typed unavailability and zero residue",
			constructionErr, validationErr, got, observationErr)
	}
}
