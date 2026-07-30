package lease_test

import (
	"runtime"
	"testing"

	"github.com/deliri/primitive/v2026/lease"
)

func BenchmarkEvaluate(b *testing.B) {
	authority := fixtureAuthority(b, 131)
	subject := fixtureSubject(b, 132)
	decision := fixtureGrantDecision(b, subject, 1, 1_000, fixtureGrant())
	_, verified := fixtureVerified(b, authority, decision, subject)
	observation := fixtureObservation(b, 2_500)
	request := lease.EvaluateRequest{
		Decision: verified, DurableHighWater: fixtureInstant(1_000),
		StartedAt: observation, ObservedAt: observation,
	}
	b.ReportAllocs()

	var result lease.Assessment
	var err error
	for b.Loop() {
		result, err = lease.Evaluate(request)
	}
	runtime.KeepAlive(result)
	runtime.KeepAlive(err)
}

func BenchmarkVerify(b *testing.B) {
	authority := fixtureAuthority(b, 141)
	subject := fixtureSubject(b, 142)
	decision := fixtureGrantDecision(b, subject, 1, 1_000, fixtureGrant())
	document, _ := fixtureVerified(b, authority, decision, subject)
	request := lease.VerifyRequest{
		Document: document, TrustedKeys: authority.trusted,
		ExpectedSubject: subject,
	}
	b.ReportAllocs()

	var result lease.Verified
	var err error
	for b.Loop() {
		result, err = lease.Verify(request)
	}
	runtime.KeepAlive(result)
	runtime.KeepAlive(err)
}

func BenchmarkDecisionCanonicalJSON(b *testing.B) {
	subject := fixtureSubject(b, 151)
	decision := fixtureGrantDecision(b, subject, 1, 1_000, fixtureGrant())
	b.ReportAllocs()

	var result []byte
	var err error
	for b.Loop() {
		result, err = decision.MarshalJSON()
	}
	runtime.KeepAlive(result)
	runtime.KeepAlive(err)
}
