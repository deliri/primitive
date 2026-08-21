package controlplanetest

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
)

func fixtureSeed(value byte) [ed25519.SeedSize]byte {
	seed := [ed25519.SeedSize]byte{}
	for index := range seed {
		seed[index] = value
	}
	return seed
}

// TestIssueInstallationConstructsOpaqueOfferingsThroughTheRealIssuers proves
// the test-support boundary is product blind and returns fully validating,
// internally coherent certificate fixtures for consumer-owned identities.
func TestIssueInstallationConstructsOpaqueOfferingsThroughTheRealIssuers(t *testing.T) {
	t.Parallel()

	for value, offering := range []core.Offering{
		controlplaneTestOffering(t, 1),
		controlplaneTestOffering(t, 127),
		controlplaneTestOffering(t, 255),
	} {
		fixture, err := IssueInstallation(InstallationRequest{
			AuthoritySeed: fixtureSeed(byte(value) + 0x20),
			DeviceSeed:    fixtureSeed(byte(value) + 0x40),
			Offering:      offering,
		})
		if err != nil {
			t.Fatalf("IssueInstallation(%v) error = %v, want nil", offering, err)
		}
		if err := fixture.Validate(); err != nil {
			t.Fatalf("Installation(%v).Validate() error = %v, want nil", offering, err)
		}
		if fixture.Build.Offering() != offering || fixture.Certificate.Body.Build != fixture.Build {
			t.Fatalf("Installation(%v) build facts disagree", offering)
		}
	}
}

// TestIssueInstallationReturnsNeutralForEveryInvalidSeedRelation proves the
// fixture helper cannot silently collapse authority and device identities.
func TestIssueInstallationReturnsNeutralForEveryInvalidSeedRelation(t *testing.T) {
	t.Parallel()

	validAuthority := fixtureSeed(0x21)
	validDevice := fixtureSeed(0x31)
	cases := []struct {
		name    string
		request InstallationRequest
	}{
		{
			name: "zero authority seed",
			request: InstallationRequest{
				DeviceSeed: validDevice, Offering: controlplaneTestOffering(t, 10),
			},
		},
		{
			name: "zero device seed",
			request: InstallationRequest{
				AuthoritySeed: validAuthority, Offering: controlplaneTestOffering(t, 11),
			},
		},
		{
			name: "same key on both sides",
			request: InstallationRequest{
				AuthoritySeed: validAuthority, DeviceSeed: validAuthority,
				Offering: controlplaneTestOffering(t, 12),
			},
		},
		{
			name: "unset offering",
			request: InstallationRequest{
				AuthoritySeed: validAuthority, DeviceSeed: validDevice,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fixture, err := IssueInstallation(tc.request)
			neutral := len(fixture.AuthorityPrivate) == 0 && len(fixture.DevicePrivate) == 0 &&
				fixture.Certificate == (controlplane.InstallationCertificateDocument{}) &&
				fixture.AuthorityPublic == (core.Ed25519PublicKey{}) &&
				fixture.DevicePublic == (core.Ed25519PublicKey{}) &&
				fixture.Build == (core.BuildIdentity{})
			if !errors.Is(err, core.ErrPrimitiveContract) || !neutral {
				t.Fatalf("IssueInstallation(%s) neutral = %t, error = %v, want true and errors.Is %v",
					tc.name, neutral, err, core.ErrPrimitiveContract)
			}
		})
	}
}

func controlplaneTestOffering(t testing.TB, marker byte) core.Offering {
	t.Helper()
	offering := core.Offering{Token: fmt.Sprintf("controlplanetest-fixture-%02x", marker)}
	if err := offering.Validate(); err != nil {
		t.Fatalf("Offering.Validate() error = %v, want nil", err)
	}
	return offering
}
