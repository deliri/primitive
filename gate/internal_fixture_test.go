package gate

import (
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	internalIssuedAtNanoseconds  int64 = 1_000
	internalNotBeforeNanoseconds int64 = 2_000
	internalGoodUntilNanoseconds int64 = 5_000
)

// fixtureInternalAssessment builds one authentic assessment at an exact
// instant. Gate's fail-closed arms guard against a Gate-owned capability that
// closes over the wrong authentic state, so proving them needs a real
// assessment and direct access to the unexported carrier field.
func fixtureInternalAssessment(tb testing.TB, atNanoseconds int64) lease.Assessment {
	tb.Helper()

	private := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
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
	subject := fixtureInternalSubject(tb)
	decision := fixtureInternalGrantDecision(tb, subject)
	envelope, err := attest.Sign(attest.SignRequest[lease.Domain]{
		Body: decision, Key: private,
	})
	if err != nil {
		tb.Fatalf("attest.Sign() error = %v, want nil", err)
	}
	verified, err := lease.Verify(lease.VerifyRequest{
		Document:    lease.Document{Decision: decision, Attestation: envelope},
		TrustedKeys: trusted, ExpectedSubject: subject,
	})
	if err != nil {
		tb.Fatalf("lease.Verify() error = %v, want nil", err)
	}
	observation, err := temporal.NewObservation(time.Unix(0, atNanoseconds).UTC())
	if err != nil {
		tb.Fatalf("temporal.NewObservation() error = %v, want nil", err)
	}
	assessment, err := lease.Evaluate(lease.EvaluateRequest{
		Decision:         verified,
		DurableHighWater: temporal.InstantFromNanoseconds(internalIssuedAtNanoseconds),
		StartedAt:        observation,
		ObservedAt:       observation,
	})
	if err != nil {
		tb.Fatalf("lease.Evaluate() error = %v, want nil", err)
	}
	return assessment
}

func fixtureInternalSubject(tb testing.TB) lease.Subject {
	tb.Helper()

	product, err := lease.NewProduct([lease.IdentifierBytes]byte{1})
	if err != nil {
		tb.Fatalf("lease.NewProduct() error = %v, want nil", err)
	}
	entitlement, err := lease.NewEntitlementID([lease.IdentifierBytes]byte{2})
	if err != nil {
		tb.Fatalf("lease.NewEntitlementID() error = %v, want nil", err)
	}
	device, err := lease.NewDeviceID([lease.IdentifierBytes]byte{3})
	if err != nil {
		tb.Fatalf("lease.NewDeviceID() error = %v, want nil", err)
	}
	return lease.Subject{
		Product: product, EntitlementID: entitlement, DeviceID: device,
	}
}

func fixtureInternalGrantDecision(tb testing.TB, subject lease.Subject) lease.Decision {
	tb.Helper()

	generation, err := lease.NewGeneration(1)
	if err != nil {
		tb.Fatalf("lease.NewGeneration() error = %v, want nil", err)
	}
	decision, err := lease.NewGrantDecision(lease.GrantDecisionRequest{
		Header: lease.Header{
			Revision: lease.RevisionV1, Subject: subject, Generation: generation,
			IssuedAt: temporal.InstantFromNanoseconds(internalIssuedAtNanoseconds),
		},
		Grant: lease.Grant{
			NotBefore:    temporal.InstantFromNanoseconds(internalNotBeforeNanoseconds),
			ContactAfter: temporal.InstantFromNanoseconds(3_000),
			NotAfter:     temporal.InstantFromNanoseconds(4_000),
			GoodUntil:    temporal.InstantFromNanoseconds(internalGoodUntilNanoseconds),
		},
	})
	if err != nil {
		tb.Fatalf("lease.NewGrantDecision() error = %v, want nil", err)
	}
	return decision
}
