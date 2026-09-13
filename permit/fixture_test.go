package permit

import (
	"crypto/ed25519"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplanetest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
	"github.com/deliri/primitive/v2026/temporal"
)

func permitFixture(t testing.TB) (VerifyRequest, ed25519.PrivateKey) {
	t.Helper()
	installation, err := controlplanetest.IssueInstallation(controlplanetest.InstallationRequest{Offering: core.Offering{Token: "witness"}, AuthoritySeed: [32]byte{17}, DeviceSeed: [32]byte{23}})
	if err != nil {
		t.Fatalf("IssueInstallation() = %v, want nil", err)
	}
	t.Cleanup(func() { clear(installation.AuthorityPrivate); clear(installation.DevicePrivate) })
	generation, err := lease.NewGeneration(1)
	if err != nil {
		t.Fatalf("NewGeneration() = %v, want nil", err)
	}
	retry, err := temporal.DurationFromMilliseconds(1000)
	if err != nil {
		t.Fatalf("DurationFromMilliseconds() = %v, want nil", err)
	}
	skills, err := NewActions(permitAction(t, "operation-a"))
	if err != nil {
		t.Fatalf("NewActions() = %v, want nil", err)
	}
	requestNonce, err := controlwire.NewRequestNonce([32]byte{31})
	if err != nil {
		t.Fatalf("NewRequestNonce() = %v, want nil", err)
	}
	terms := Terms{RequestNonce: requestNonce, Revision: RevisionV1, Subject: installation.Certificate.Body.Subject, Build: installation.Certificate.Body.Build, Generation: generation, Actions: skills,
		NotBefore: temporal.InstantFromNanoseconds(100), ExpiresAt: temporal.InstantFromNanoseconds(200), ContactAt: temporal.InstantFromNanoseconds(150), RetryAfter: retry}
	document, err := Sign(terms, installation.AuthorityPrivate)
	if err != nil {
		t.Fatalf("Sign() = %v, want nil", err)
	}
	trusted, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{Keys: []core.Ed25519PublicKey{installation.AuthorityPublic}})
	if err != nil {
		t.Fatalf("NewTrustedKeys() = %v, want nil", err)
	}
	return VerifyRequest{RequestNonce: requestNonce, Document: document, TrustedKeys: trusted, Subject: terms.Subject, Build: terms.Build, EffectiveAt: terms.NotBefore, MinimumGeneration: generation}, installation.AuthorityPrivate
}

func permitAction(t testing.TB, text string) Action {
	t.Helper()
	action, err := ParseAction(text)
	if err != nil {
		t.Fatalf("ParseAction(%q) = %v, want nil", text, err)
	}
	return action
}
