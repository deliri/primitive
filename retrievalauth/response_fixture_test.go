package retrievalauth

import (
	"crypto/ed25519"
	"github.com/deliri/primitive/v2026/chit"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/receipt"
	"github.com/deliri/primitive/v2026/retrieval"
	"github.com/deliri/primitive/v2026/temporal"
	"hash/crc32"
	"testing"
)

type responseFixture struct {
	request     retrievalAuthFixture
	signer      ed25519.PrivateKey
	grant       retrieval.GrantProjection
	chit        chit.Verified
	member      chit.VerifiedManifestEntry
	header      controlplane.ResponseHeader
	expectation controlplane.ResponseExpectation
	client      controlplane.Client
	canonical   []byte
}

// The fixture drives real receipt, manifest, chit and grant issuers/verifiers.
// The signed URL is an inert local fixture; no object-store operation is made.
func newResponseFixture(t testing.TB, authority byte) responseFixture {
	t.Helper()
	req := newRetrievalAuthFixture(t, retrievalAuthFixtureRequest{AuthorityByte: authority})
	seed := retrievalAuthSeed(authority)
	signer := ed25519.NewKeyFromSeed(seed[:])
	now, err := req.certificate.Body.IssuedAt.Nanoseconds()
	if err != nil {
		t.Fatalf("IssuedAt.Nanoseconds error = %v, want nil", err)
	}
	scope := req.request.Payload.Scope
	content := []byte("retained retrieval evidence")
	extent, err := core.NewByteLength(uint64(len(content)))
	if err != nil {
		t.Fatalf("NewByteLength error = %v, want nil", err)
	}
	rid, err := receipt.NewReceiptID([receipt.ReceiptIDBytes]byte{0x23})
	if err != nil {
		t.Fatalf("NewReceiptID error = %v, want nil", err)
	}
	submission, err := receipt.NewSubmissionIdentity([receipt.LifecycleIdentityBytes]byte{0x24})
	if err != nil {
		t.Fatalf("NewSubmissionIdentity error = %v, want nil", err)
	}
	object, err := receipt.NewObjectIdentity([receipt.LifecycleIdentityBytes]byte{0x25})
	if err != nil {
		t.Fatalf("NewObjectIdentity error = %v, want nil", err)
	}
	evidence, err := receipt.IssueEvidence(receipt.IssueEvidenceRequest{Key: signer, Identity: rid, Principal: scope.Principal, Offering: scope.Offering, OccurredAt: temporal.InstantFromNanoseconds(now), Body: receipt.EvidenceBody{Extent: extent, SHA256: core.SHA256Of(content), CRC32C: core.NewCRC32C(crc32.Checksum(content, crc32.MakeTable(crc32.Castagnoli))), Submission: submission, Object: object}})
	if err != nil {
		t.Fatalf("IssueEvidence error = %v, want nil", err)
	}
	verified, err := receipt.VerifyEvidence(receipt.VerifyEvidenceRequest{Document: evidence, TrustedKeys: req.trusted, Expected: receipt.EvidenceExpectation{Principal: scope.Principal, Offering: scope.Offering, Body: evidence.Payload.Body}})
	if err != nil {
		t.Fatalf("VerifyEvidence error = %v, want nil", err)
	}
	name, err := chit.ParseEntryName("evidence/result.json")
	if err != nil {
		t.Fatalf("ParseEntryName error = %v, want nil", err)
	}
	sequence, err := chit.NewEntrySequence(1)
	if err != nil {
		t.Fatalf("NewEntrySequence error = %v, want nil", err)
	}
	addition := chit.ManifestAddition{Entry: chit.ManifestEntry{Name: name, ContentType: core.HTTPMediaTypeOctetStream(), Evidence: evidence, Sequence: sequence}, Evidence: verified}
	accumulator := chit.NewManifestAccumulator()
	if err := accumulator.Add(addition); err != nil {
		t.Fatalf("ManifestAccumulator.Add error = %v, want nil", err)
	}
	summary, err := accumulator.Seal()
	if err != nil {
		t.Fatalf("ManifestAccumulator.Seal error = %v, want nil", err)
	}
	membership, err := chit.NewManifestEntryVerifier(sequence)
	if err != nil {
		t.Fatalf("NewManifestEntryVerifier error = %v, want nil", err)
	}
	if err := membership.Add(addition); err != nil {
		t.Fatalf("ManifestEntryVerifier.Add error = %v, want nil", err)
	}
	member, err := membership.Seal(summary)
	if err != nil {
		t.Fatalf("ManifestEntryVerifier.Seal error = %v, want nil", err)
	}
	collection, err := chit.ParseCollectionID("00000000-0003-7000-8000-000000000003")
	if err != nil {
		t.Fatalf("ParseCollectionID error = %v, want nil", err)
	}
	version, err := chit.NewVersion(1)
	if err != nil {
		t.Fatalf("NewVersion error = %v, want nil", err)
	}
	partition, err := chit.NewPartition(core.SHA256Of([]byte("retrieval fixture partition")))
	if err != nil {
		t.Fatalf("NewPartition error = %v, want nil", err)
	}
	chitDocument, err := chit.Issue(chit.Issuance{Signer: signer, TrustedKeys: req.trusted, Payload: chit.Payload{Identity: req.request.Payload.Chit, Collection: collection, Partition: partition, Scope: scope, Manifest: summary, AcceptedAt: temporal.InstantFromNanoseconds(now - 1), RetainUntil: temporal.InstantFromNanoseconds(now + 2_000_000_000), Version: version}})
	if err != nil {
		t.Fatalf("chit.Issue error = %v, want nil", err)
	}
	verifiedChit, err := chit.Verify(chit.Verification{Document: chitDocument, Expected: chit.Expectation{Identity: req.request.Payload.Chit, Scope: scope}, TrustedKeys: req.trusted})
	if err != nil {
		t.Fatalf("chit.Verify error = %v, want nil", err)
	}
	signedURL, err := objectstore.ParseSignedURL("https://storage.googleapis.com/bucket/object?X-Goog-Signature=fixture&X-Goog-SignedHeaders=host")
	if err != nil {
		t.Fatalf("ParseSignedURL error = %v, want nil", err)
	}
	headers, err := objectstore.NewSignedHeaders(nil)
	if err != nil {
		t.Fatalf("NewSignedHeaders error = %v, want nil", err)
	}
	capability, err := objectstore.NewDownloadCapabilityProjection(objectstore.ProviderGoogleCloudStorage, objectstore.DownloadTarget{URL: signedURL, Headers: headers, ExpiresAt: temporal.InstantFromNanoseconds(now + 1_000_000_000)})
	if err != nil {
		t.Fatalf("NewDownloadCapabilityProjection error = %v, want nil", err)
	}
	commitment, err := capability.Commitment()
	if err != nil {
		t.Fatalf("Capability.Commitment error = %v, want nil", err)
	}
	requestCommitment, err := retrieval.CommitRequest(req.request.Payload)
	if err != nil {
		t.Fatalf("CommitRequest error = %v, want nil", err)
	}
	authorization, err := controlwire.NewAuthorityNonce([core.SHA256DigestBytes]byte{0x71})
	if err != nil {
		t.Fatalf("NewAuthorityNonce error = %v, want nil", err)
	}
	grant, err := retrieval.IssueGrant(retrieval.GrantIssuance{Signer: signer, Capability: capability, Request: req.request.Payload, Chit: verifiedChit, Entry: member, Payload: retrieval.GrantPayload{Entry: addition.Entry, Request: requestCommitment, Authorization: authorization, Capability: commitment, Manifest: summary.Digest, Chit: req.request.Payload.Chit, IssuedAt: temporal.InstantFromNanoseconds(now), ExpiresAt: temporal.InstantFromNanoseconds(now + 1_000_000_000), Continuation: core.CatalogContinuationEnd}})
	if err != nil {
		t.Fatalf("IssueGrant error = %v, want nil", err)
	}
	activation, err := controlwire.NewPolicyActivation(1)
	if err != nil {
		t.Fatalf("NewPolicyActivation error = %v, want nil", err)
	}
	cert := req.certificate.Body
	header := controlplane.ResponseHeader{ProviderTime: cert.IssuedAt, RequestNonce: req.request.Payload.Nonce, Account: cert.Account, Installation: cert.Subject.DeviceID, Revision: req.request.Payload.Revision, Family: controlwire.RouteFamilyRetrievals, Status: controlplane.ProductStatusActive, Offering: cert.Build.Offering(), Policy: controlwire.PolicyCursor{Revision: controlwire.PolicyRevisionID{1}, Activation: activation}}
	expectation := controlplane.ResponseExpectation{RequestNonce: header.RequestNonce, Account: header.Account, Installation: header.Installation, Revision: header.Revision, Family: header.Family, Offering: header.Offering}
	client, err := controlplane.NewClient(controlplane.ClientConfiguration{TrustedAuthorityKeys: req.trusted})
	if err != nil {
		t.Fatalf("NewClient error = %v, want nil", err)
	}
	projection, err := IssueResponse(ResponseIssuance{Signer: signer, Header: header, Body: grant, Server: retrievalAuthServer(t, req.trusted), Assessment: responseAssessment(t, header)})
	if err != nil {
		t.Fatalf("IssueResponse error = %v, want nil", err)
	}
	canonical, err := projection.MarshalJSON()
	if err != nil || len(canonical) == 0 {
		t.Fatalf("response projection = (%d bytes, %v), want nonempty and nil", len(canonical), err)
	}
	return responseFixture{request: req, signer: signer, grant: grant, chit: verifiedChit, member: member, header: header, expectation: expectation, client: client, canonical: canonical}
}

func responseAssessment(t testing.TB, h controlplane.ResponseHeader) controlwire.ProtocolAssessment {
	t.Helper()
	support, err := controlwire.PublishedProtocolSupport()
	if err != nil {
		t.Fatalf("PublishedProtocolSupport error = %v, want nil", err)
	}
	assessment, err := controlwire.AssessProtocol(controlwire.ProtocolAssessmentRequest{Support: support, Capability: controlwire.ProtocolCapability{Revision: h.Revision, Family: h.Family}})
	if err != nil {
		t.Fatalf("AssessProtocol error = %v, want nil", err)
	}
	return assessment
}

func (f responseFixture) prove(t testing.TB, proof controlplane.VerifiedResponse[retrieval.GrantDocument, *retrieval.GrantDocument]) {
	t.Helper()
	header, err := proof.Header()
	if err != nil || header != f.header {
		t.Fatalf("response header = (%v, %v), want exact signed header", header, err)
	}
	body, err := proof.Body()
	if err != nil {
		t.Fatalf("response Body error = %v, want nil", err)
	}
	commitment, err := body.Capability.Commitment()
	if err != nil || body.Payload != f.grant.Payload || body.Attestation != f.grant.Attestation || commitment != f.grant.Payload.Capability {
		t.Fatalf("grant payload = %v, capability = %v, error = %v, want exact signed payload and capability", body.Payload, commitment, err)
	}
	grant, err := retrieval.VerifyGrant(retrieval.GrantExpectation{Document: body, Request: f.request.request.Payload, Chit: f.chit, Entry: f.member, ObservedAt: f.header.ProviderTime, TrustedKeys: f.request.trusted})
	if err != nil || grant.Validate() != nil {
		t.Fatalf("independent VerifyGrant = (%v, %v), want valid proof and nil", grant.Validate(), err)
	}
}
