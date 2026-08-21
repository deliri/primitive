package lease

import (
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func fixtureInternalHeader(tb testing.TB) Header {
	tb.Helper()

	offering := core.Offering{Token: "lease-internal-fixture"}
	if err := offering.Validate(); err != nil {
		tb.Fatalf("Offering.Validate() error = %v, want nil", err)
	}
	entitlement, err := NewEntitlementID([IdentifierBytes]byte{2})
	if err != nil {
		tb.Fatalf("NewEntitlementID() error = %v, want nil", err)
	}
	device, err := NewDeviceID([IdentifierBytes]byte{3})
	if err != nil {
		tb.Fatalf("NewDeviceID() error = %v, want nil", err)
	}
	generation, err := NewGeneration(1)
	if err != nil {
		tb.Fatalf("NewGeneration() error = %v, want nil", err)
	}
	return Header{
		Revision: RevisionV1,
		Subject: Subject{
			Offering: offering, EntitlementID: entitlement, DeviceID: device,
		},
		Generation: generation,
		IssuedAt:   fixtureInternalInstant(1_000),
	}
}

func fixtureInternalGrant() Grant {
	return Grant{
		NotBefore:    fixtureInternalInstant(2_000),
		ContactAfter: fixtureInternalInstant(3_000),
		NotAfter:     fixtureInternalInstant(4_000),
		GoodUntil:    fixtureInternalInstant(5_000),
	}
}

func fixtureInternalInstant(nanoseconds int64) temporal.Instant {
	return temporal.InstantFromNanoseconds(nanoseconds)
}
