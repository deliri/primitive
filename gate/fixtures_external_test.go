package gate_test

import (
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
	"github.com/deliri/primitive/v2026/temporal"
)

// Gate's ingress is an already-authentic lease.Assessment, which no caller can
// construct without a real Ed25519 signature over a real decision. These
// fixtures therefore sign and verify through Attest and Lease exactly as a CLI
// does, so every Gate proof below runs the real production path rather than a
// fabricated assessment. This is the one reason Gate declares test-only edges
// to Attest and Temporal in the Core architecture catalog.
const (
	fixtureIssuedAtNanoseconds     int64 = 1_000
	fixtureNotBeforeNanoseconds    int64 = 2_000
	fixtureContactAfterNanoseconds int64 = 3_000
	fixtureNotAfterNanoseconds     int64 = 4_000
	fixtureGoodUntilNanoseconds    int64 = 5_000
)

type authorityFixture struct {
	private ed25519.PrivateKey
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
	return authorityFixture{private: private, trusted: trusted}
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
		Generation: value,
		IssuedAt:   temporal.InstantFromNanoseconds(issuedAt),
	}
}

func fixtureGrantDecision(tb testing.TB, subject lease.Subject) lease.Decision {
	tb.Helper()

	decision, err := lease.NewGrantDecision(lease.GrantDecisionRequest{
		Header: fixtureHeader(tb, subject, 1, fixtureIssuedAtNanoseconds),
		Grant: lease.Grant{
			NotBefore:    temporal.InstantFromNanoseconds(fixtureNotBeforeNanoseconds),
			ContactAfter: temporal.InstantFromNanoseconds(fixtureContactAfterNanoseconds),
			NotAfter:     temporal.InstantFromNanoseconds(fixtureNotAfterNanoseconds),
			GoodUntil:    temporal.InstantFromNanoseconds(fixtureGoodUntilNanoseconds),
		},
	})
	if err != nil {
		tb.Fatalf("lease.NewGrantDecision() error = %v, want nil", err)
	}
	return decision
}

func fixtureRefusalDecision(tb testing.TB, subject lease.Subject) lease.Decision {
	tb.Helper()

	decision, err := lease.NewRefusalDecision(lease.RefusalDecisionRequest{
		Header: fixtureHeader(tb, subject, 2, fixtureIssuedAtNanoseconds),
		Refusal: lease.Refusal{
			ContactAfter: temporal.InstantFromNanoseconds(fixtureContactAfterNanoseconds),
		},
	})
	if err != nil {
		tb.Fatalf("lease.NewRefusalDecision() error = %v, want nil", err)
	}
	return decision
}

func fixtureRevocationDecision(tb testing.TB, subject lease.Subject) lease.Decision {
	tb.Helper()

	decision, err := lease.NewRevocationDecision(lease.RevocationDecisionRequest{
		Header: fixtureHeader(tb, subject, 3, fixtureIssuedAtNanoseconds),
		Revocation: lease.Revocation{
			Reason: lease.RevocationReasonSecurityOrPlatformRisk,
		},
	})
	if err != nil {
		tb.Fatalf("lease.NewRevocationDecision() error = %v, want nil", err)
	}
	return decision
}

func fixtureVerified(
	tb testing.TB,
	authority authorityFixture,
	decision lease.Decision,
	expected lease.Subject,
) lease.Verified {
	tb.Helper()

	envelope, err := attest.Sign(attest.SignRequest[lease.Domain]{
		Body: decision, Signer: authority.private,
	})
	if err != nil {
		tb.Fatalf("attest.Sign() error = %v, want nil", err)
	}
	verified, err := lease.Verify(lease.VerifyRequest{
		Document:    lease.Document{Decision: decision, Attestation: envelope},
		TrustedKeys: authority.trusted, ExpectedSubject: expected,
	})
	if err != nil {
		tb.Fatalf("lease.Verify() error = %v, want nil", err)
	}
	return verified
}

// fixtureAssessment evaluates one authentic decision at an exact instant and
// returns the real assessment a CLI would hand to Gate.
func fixtureAssessment(
	tb testing.TB,
	verified lease.Verified,
	atNanoseconds int64,
) lease.Assessment {
	tb.Helper()

	observation, err := temporal.NewObservation(time.Unix(0, atNanoseconds).UTC())
	if err != nil {
		tb.Fatalf("temporal.NewObservation() error = %v, want nil", err)
	}
	assessment, err := lease.Evaluate(lease.EvaluateRequest{
		Decision:         verified,
		DurableHighWater: temporal.InstantFromNanoseconds(fixtureIssuedAtNanoseconds),
		StartedAt:        observation,
		ObservedAt:       observation,
	})
	if err != nil {
		tb.Fatalf("lease.Evaluate() error = %v, want nil", err)
	}
	return assessment
}
