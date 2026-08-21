package controlwire_test

import (
	"fmt"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func controlwireExternalOfferingFixture(t testing.TB, marker byte) core.Offering {
	t.Helper()
	offering := core.Offering{Token: fmt.Sprintf("controlwire-external-%02x", marker)}
	if err := offering.Validate(); err != nil {
		t.Fatalf("Offering.Validate() error = %v, want nil", err)
	}
	return offering
}
