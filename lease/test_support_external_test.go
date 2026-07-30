package lease_test

import (
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
	"github.com/deliri/primitive/v2026/temporal"
)

type authorityFixture struct {
	private ed25519.PrivateKey
	public  core.Ed25519PublicKey
	trusted attest.TrustedKeys
}

func fixtureAuthority(tb testing.TB, marker byte) authorityFixture {
	tb.Helper()

	seed := make([]byte, ed25519.SeedSize)
	for index := range seed {
		seed[index] = marker + byte(index)
	}
	private := ed25519.NewKeyFromSeed(seed)
	public, err := core.NewEd25519PublicKey(private.Public().(ed25519.PublicKey))
	if err != nil {
		tb.Fatalf("core.NewEd25519PublicKey() error = %v, want nil", err)
	}
	trusted, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
		Keys: []core.Ed25519PublicKey{public},
	})
	if err != nil {
		tb.Fatalf("attest.NewTrustedKeys() error = %v, want nil", err)
	}
	return authorityFixture{private: private, public: public, trusted: trusted}
}

func fixtureIdentifierBytes(marker byte) [lease.IdentifierBytes]byte {
	var value [lease.IdentifierBytes]byte
	for index := range value {
		value[index] = marker + byte(index)
	}
	return value
}

func fixtureSubject(tb testing.TB, marker byte) lease.Subject {
	tb.Helper()

	product, err := lease.NewProduct(fixtureIdentifierBytes(marker))
	if err != nil {
		tb.Fatalf("lease.NewProduct() error = %v, want nil", err)
	}
	entitlement, err := lease.NewEntitlementID(fixtureIdentifierBytes(marker + 32))
	if err != nil {
		tb.Fatalf("lease.NewEntitlementID() error = %v, want nil", err)
	}
	device, err := lease.NewDeviceID(fixtureIdentifierBytes(marker + 64))
	if err != nil {
		tb.Fatalf("lease.NewDeviceID() error = %v, want nil", err)
	}
	return lease.Subject{
		Product: product, EntitlementID: entitlement, DeviceID: device,
	}
}

func fixtureInstant(nanoseconds int64) temporal.Instant {
	return temporal.InstantFromNanoseconds(nanoseconds)
}

func fixtureObservation(tb testing.TB, nanoseconds int64) temporal.Observation {
	tb.Helper()

	value, err := temporal.NewObservation(time.Unix(0, nanoseconds).UTC())
	if err != nil {
		tb.Fatalf("temporal.NewObservation() error = %v, want nil", err)
	}
	return value
}

func fixtureHeader(
	tb testing.TB,
	subject lease.Subject,
	generation uint64,
	issuedAt int64,
) lease.Header {
	tb.Helper()

	value, err := lease.NewGeneration(generation)
	if err != nil {
		tb.Fatalf("lease.NewGeneration() error = %v, want nil", err)
	}
	return lease.Header{
		Revision: lease.RevisionV1, Subject: subject,
		Generation: value, IssuedAt: fixtureInstant(issuedAt),
	}
}

func fixtureGrantDecision(
	tb testing.TB,
	subject lease.Subject,
	generation uint64,
	issuedAt int64,
	grant lease.Grant,
) lease.Decision {
	tb.Helper()

	decision, err := lease.NewGrantDecision(lease.GrantDecisionRequest{
		Header: fixtureHeader(tb, subject, generation, issuedAt),
		Grant:  grant,
	})
	if err != nil {
		tb.Fatalf("lease.NewGrantDecision() error = %v, want nil", err)
	}
	return decision
}

func fixtureGrant() lease.Grant {
	return lease.Grant{
		NotBefore:    fixtureInstant(2_000),
		ContactAfter: fixtureInstant(3_000),
		NotAfter:     fixtureInstant(4_000),
		GoodUntil:    fixtureInstant(5_000),
	}
}

func fixtureVerified(
	tb testing.TB,
	authority authorityFixture,
	decision lease.Decision,
	expected lease.Subject,
) (lease.Document, lease.Verified) {
	tb.Helper()

	envelope, err := attest.Sign(attest.SignRequest[lease.Domain]{
		Body: decision, Key: authority.private,
	})
	if err != nil {
		tb.Fatalf("attest.Sign() error = %v, want nil", err)
	}
	document := lease.Document{Decision: decision, Attestation: envelope}
	verified, err := lease.Verify(lease.VerifyRequest{
		Document: document, TrustedKeys: authority.trusted,
		ExpectedSubject: expected,
	})
	if err != nil {
		tb.Fatalf("lease.Verify() error = %v, want nil", err)
	}
	return document, verified
}
