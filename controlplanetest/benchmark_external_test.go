package controlplanetest_test

import (
	"crypto/ed25519"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/controlplanetest"
	"github.com/deliri/primitive/v2026/core"
)

func BenchmarkIssueInstallation(b *testing.B) {
	offering := core.Offering{Token: "controlplanetest-fixture-01"}
	if err := offering.Validate(); err != nil {
		b.Fatalf("Offering.Validate() error = %v, want nil", err)
	}
	request := controlplanetest.InstallationRequest{
		AuthoritySeed: controlplaneTestSeed(0x21),
		DeviceSeed:    controlplaneTestSeed(0x41),
		Offering:      offering,
	}
	var wantErr error
	b.ReportAllocs()
	var last controlplanetest.Installation
	for b.Loop() {
		got, err := controlplanetest.IssueInstallation(request)
		if !errors.Is(err, wantErr) {
			b.Fatalf("controlplanetest.IssueInstallation() error = %v, want %v", err, wantErr)
		}
		last = got
	}
	if err := last.Validate(); err != nil {
		b.Fatalf("Installation.Validate() error = %v, want nil", err)
	}
}

func controlplaneTestSeed(value byte) [ed25519.SeedSize]byte {
	var seed [ed25519.SeedSize]byte
	for index := range seed {
		seed[index] = value
	}
	return seed
}
