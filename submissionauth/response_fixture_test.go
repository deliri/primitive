package submissionauth

import (
	"github.com/deliri/primitive/v2026/chit"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"testing"
)

func authResponseHeader(t testing.TB, fixture authCompletionFixture, family controlwire.RouteFamily) controlplane.ResponseHeader {
	t.Helper()
	certificate := fixture.request.certificate.Body
	nonce := fixture.request.request.Payload.Nonce
	if family == controlwire.RouteFamilySubmissionCompletions {
		nonce = fixture.completionNonce
	}
	return controlplane.ResponseHeader{ProviderTime: certificate.IssuedAt, RequestNonce: nonce, Account: certificate.Account,
		Installation: certificate.Subject.DeviceID, Revision: certificate.Revision, Family: family, Status: controlplane.ProductStatusActive,
		Offering: certificate.Build.Offering(), Policy: blindSubmissionPolicyCursor(t)}
}
func authResponseExpectation(header controlplane.ResponseHeader) controlplane.ResponseExpectation {
	return controlplane.ResponseExpectation{RequestNonce: header.RequestNonce, Account: header.Account, Installation: header.Installation, Revision: header.Revision, Family: header.Family, Offering: header.Offering}
}
func authCompletionResponseBody(t testing.TB, fixture authCompletionFixture) chit.Document {
	t.Helper()
	verified, err := VerifyCompletion(CompletionVerification{Document: fixture.credentialed, Request: fixture.verifiedRequest, Grant: fixture.grant,
		GrantKeys: fixture.request.trusted, Server: submissionAuthServer(t, fixture.request.trusted), Nonce: fixture.completionNonce})
	if err != nil {
		t.Fatal(err)
	}
	reconciled, err := ReconcileCompletion(benchmarkReconciliation(t, fixture, verified))
	if err != nil {
		t.Fatal(err)
	}
	addition, err := reconciled.Addition()
	if err != nil {
		t.Fatal(err)
	}
	accumulator := chit.NewManifestAccumulator()
	if err := accumulator.Add(addition); err != nil {
		t.Fatal(err)
	}
	summary, err := accumulator.Seal()
	if err != nil {
		t.Fatal(err)
	}
	scope, err := fixture.request.certificate.Body.Scope()
	if err != nil {
		t.Fatal(err)
	}
	identity, err := chit.ParseChitID("00000000-0008-7000-8000-000000000008")
	if err != nil {
		t.Fatal(err)
	}
	version, err := chit.NewVersion(1)
	if err != nil {
		t.Fatal(err)
	}
	manifest := fixture.request.request.Payload.Manifest
	document, err := chit.Issue(chit.Issuance{Signer: fixture.request.authority, TrustedKeys: fixture.request.trusted, Payload: chit.Payload{
		Identity: identity, Collection: manifest.Collection, Partition: manifest.Partition, Scope: scope, Manifest: summary, Version: version,
		AcceptedAt: fixture.grant.Payload.IssuedAt, RetainUntil: fixture.grant.Payload.RetainUntil,
	}})
	if err != nil {
		t.Fatal(err)
	}
	return document
}
