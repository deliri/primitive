package submission

import (
	"bytes"
	"github.com/deliri/primitive/v2026/objectstore"
	"testing"

	"github.com/deliri/primitive/v2026/attest"

	"github.com/deliri/primitive/v2026/temporal"
)

const signingDomainBenchmarkBatch = 16

func BenchmarkParseSigningDomainBatch(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		for range signingDomainBenchmarkBatch {
			got, err := ParseSigningDomain(SigningDomainRequestV1Token)
			if err != nil || got != SigningDomainRequestV1 {
				b.Fatalf("domain=%v error=%v", got, err)
			}
		}
	}
	b.ReportMetric(signingDomainBenchmarkBatch, "parses/op")
}

func BenchmarkSubmission(b *testing.B) {
	b.ReportAllocs()
	fixture := newCompletionFixture(b, submissionOffering(b, 2), []byte("submission benchmark transfer"), 0x10)
	wantProjection, err := IssueCompletion(CompletionIssuance{Signer: fixture.deviceSigner, Transfer: fixture.transfer, Request: fixture.request, Grant: fixture.grant, Nonce: fixture.nonce})
	if err != nil {
		b.Fatal(err)
	}
	document := receiveCompletionProjection(b, wantProjection)
	grant := newGrantFixture(b, grantFixtureRequest{})
	_, grantSigner := testSigningKey(b, 0x41)
	requestDocument, err := IssueRequest(RequestIssuance{Payload: fixture.request, Signer: fixture.deviceSigner})
	if err != nil {
		b.Fatal(err)
	}
	encoded, err := document.MarshalJSON()
	if err != nil {
		b.Fatal(err)
	}
	commitment, err := CommitRequest(fixture.request)
	if err != nil {
		b.Fatal(err)
	}
	decision := VerifiedDecision{kind: DecisionUpload, grant: &fixture.grant}
	source := bytes.NewReader(nil)
	policy := completionObjectstorePolicy(b)
	cases := []struct {
		run  func(*testing.B)
		name string
	}{
		{name: "RequestCommitment", run: func(b *testing.B) {
			got, err := CommitRequest(fixture.request)
			if err != nil || got != commitment {
				b.Fatalf("commitment=%v error=%v", got, err)
			}
		}},
		{name: "IssueRequest", run: func(b *testing.B) {
			got, err := IssueRequest(RequestIssuance{Payload: fixture.request, Signer: fixture.deviceSigner})
			if err != nil || got != requestDocument {
				b.Fatalf("request differs=%t error=%v", got != requestDocument, err)
			}
		}},
		{name: "VerifyRequest", run: func(b *testing.B) {
			got, err := VerifyRequest(RequestVerification{Document: requestDocument, TrustedKeys: fixture.deviceKeys})
			if err != nil || got.document != requestDocument || got.proof == (attest.Verified[SigningDomain]{}) {
				b.Fatalf("request proof error=%v", err)
			}
		}},
		{name: "IssueGrant", run: func(b *testing.B) {
			got, err := IssueGrant(GrantIssuance{Payload: grant.payload, Capability: grant.projection.Capability, Signer: grantSigner})
			if err != nil || got.Payload != grant.payload || got.Attestation != grant.document.Attestation || got.Capability.IsZero() {
				b.Fatalf("grant error=%v", err)
			}
		}},
		{name: "VerifyGrant", run: func(b *testing.B) {
			got, err := VerifyGrant(GrantExpectation{Request: grant.request, Document: grant.document, TrustedKeys: grant.trusted, ObservedAt: temporal.InstantFromNanoseconds(testGrantIssuedAt)})
			if err != nil || got.document.Payload != grant.payload || got.document.Attestation != grant.document.Attestation {
				b.Fatalf("grant proof error=%v", err)
			}
		}},
		{name: "IssueCompletion", run: func(b *testing.B) {
			got, err := IssueCompletion(CompletionIssuance{Signer: fixture.deviceSigner, Transfer: fixture.transfer, Request: fixture.request, Grant: fixture.grant, Nonce: fixture.nonce})
			if err != nil || got != wantProjection {
				b.Fatalf("completion error=%v", err)
			}
		}},
		{name: "VerifyCompletion", run: func(b *testing.B) {
			got, err := VerifyCompletion(CompletionExpectation{Provider: objectstore.ProviderGoogleCloudStorage, Document: document, Request: fixture.request, Grant: fixture.grantRecord, GrantKeys: fixture.grantKeys, CompletionKeys: fixture.deviceKeys, Nonce: fixture.nonce})
			if err != nil || got.document != document {
				b.Fatalf("completion proof error=%v", err)
			}
		}},
		{name: "DecodeCompletion", run: func(b *testing.B) {
			var got CompletionDocument
			err := got.UnmarshalJSON(encoded)
			if err != nil || got != document {
				b.Fatalf("decoded differs=%t error=%v", got != document, err)
			}
		}},
		{name: "UploadCall", run: func(b *testing.B) {
			got, err := decision.UploadCall(UploadCallRequest{Source: source, Request: fixture.request, Policy: policy})
			if err != nil || got.Source != source || got.Policy != policy || got.Integrity != fixture.request.Declaration.Integrity() || got.ContentType != fixture.request.Declaration.ContentType || got.Capability.IsZero() {
				b.Fatalf("upload projection error=%v", err)
			}
		}},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				tc.run(b)
			}
		})
	}
}
