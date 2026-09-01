package retrieval

import (
	"bytes"
	"crypto/ed25519"
	json "encoding/json/v2"
	"errors"
	"hash/crc32"
	"io"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/chit"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/receipt"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	retrievalGrantIssuedAt  = int64(1_800_000_000_000_000_000)
	retrievalGrantObserved  = int64(1_850_000_000_000_000_000)
	retrievalGrantExpiresAt = int64(2_000_000_000_000_000_000)
)

type downloadCallFixture struct {
	private      ed25519.PrivateKey
	payload      []byte
	capability   objectstore.DownloadCapabilityProjection
	request      RequestPayload
	addition     chit.ManifestAddition
	membership   chit.VerifiedManifestEntry
	chit         chit.Verified
	grantPayload GrantPayload
	document     GrantDocument
	grant        VerifiedGrant
	trusted      attest.TrustedKeys
	policy       objectstore.Policy
}

type downloadCallFixtureRequest struct {
	Payload         []byte
	Selection       Selection
	Continuation    core.CatalogContinuationState
	EntrySequence   uint64
	ManifestEntries uint64
}

func TestVerifiedGrantDownloadCallLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive exact authenticated entry projects one blind download", func(t *testing.T) {
		t.Parallel()
		fixture := newDownloadCallFixture(t, downloadCallFixtureRequest{Payload: bytes.Repeat([]byte{0x5a}, 32<<10+1)})
		var destination bytes.Buffer
		observed := 0
		call, err := fixture.grant.DownloadCall(DownloadCallRequest{
			Destination: &destination, Policy: fixture.policy,
			Observer: func(objectstore.TransferProgress) error {
				observed++
				return nil
			},
		})
		if err != nil || call.Validate() != nil || call.Destination != &destination ||
			call.Integrity.Length.Uint64() != uint64(len(fixture.payload)) ||
			call.ContentType != core.HTTPMediaTypeOctetStream() || call.Observer == nil {
			t.Fatalf("VerifiedGrant.DownloadCall() = (%v, %v), want exact valid source-free projection", call, err)
		}
		if destination.Len() != 0 || observed != 0 {
			t.Fatalf("DownloadCall effects = (%d destination bytes, %d observations), want none", destination.Len(), observed)
		}
	})

	t.Run("negative zero verified grant cannot project a plausible call", func(t *testing.T) {
		t.Parallel()
		fixture := newDownloadCallFixture(t, downloadCallFixtureRequest{Payload: []byte{1}})
		call, err := (VerifiedGrant{}).DownloadCall(DownloadCallRequest{
			Destination: io.Discard, Policy: fixture.policy,
		})
		if !errors.Is(err, core.ErrRetrievalContract) || !downloadCallIsZero(call) {
			t.Fatalf("zero VerifiedGrant.DownloadCall() = (%v, %v), want zero and errors.Is %v",
				call, err, core.ErrRetrievalContract)
		}
	})

	t.Run("neutral projection never writes the destination", func(t *testing.T) {
		t.Parallel()
		fixture := newDownloadCallFixture(t, downloadCallFixtureRequest{Payload: []byte{2, 3, 4}})
		destination := bytes.NewBufferString("customer-owned")
		before := append([]byte(nil), destination.Bytes()...)
		_, err := fixture.grant.DownloadCall(DownloadCallRequest{
			Destination: destination, Policy: fixture.policy,
		})
		if err != nil || !bytes.Equal(destination.Bytes(), before) {
			t.Fatalf("DownloadCall destination = (%q, %v), want untouched %q", destination.Bytes(), err, before)
		}
	})
}

type downloadCallMutation uint8

const (
	downloadCallZeroRequest downloadCallMutation = iota
	downloadCallNilDestination
	downloadCallZeroPolicy
	downloadCallZeroOperationTimeout
	downloadCallZeroAttemptTimeout
	downloadCallZeroErrorLimit
	downloadCallAttemptExceedsOperation
	downloadCallZeroGrant
)

func TestVerifiedGrantDownloadCallHostileIngressMatrix(t *testing.T) {
	t.Parallel()

	mutations := []downloadCallMutation{
		downloadCallZeroRequest, downloadCallNilDestination, downloadCallZeroPolicy,
		downloadCallZeroOperationTimeout, downloadCallZeroAttemptTimeout,
		downloadCallZeroErrorLimit, downloadCallAttemptExceedsOperation, downloadCallZeroGrant,
	}
	for _, mutation := range mutations {
		t.Run(downloadCallMutationName(mutation), func(t *testing.T) {
			t.Parallel()
			fixture := newDownloadCallFixture(t, downloadCallFixtureRequest{Payload: []byte{9, 8, 7}})
			request := DownloadCallRequest{Destination: io.Discard, Policy: fixture.policy}
			grant := fixture.grant
			switch mutation {
			case downloadCallZeroRequest:
				request = DownloadCallRequest{}
			case downloadCallNilDestination:
				request.Destination = nil
			case downloadCallZeroPolicy:
				request.Policy = objectstore.Policy{}
			case downloadCallZeroOperationTimeout:
				request.Policy.OperationTimeout = temporal.Duration{}
			case downloadCallZeroAttemptTimeout:
				request.Policy.AttemptTimeout = temporal.Duration{}
			case downloadCallZeroErrorLimit:
				request.Policy.ErrorBodyLimit = core.ByteCount{}
			case downloadCallAttemptExceedsOperation:
				request.Policy.OperationTimeout = mustRetrievalDuration(t, 1)
				request.Policy.AttemptTimeout = mustRetrievalDuration(t, 2)
			case downloadCallZeroGrant:
				grant = VerifiedGrant{}
			default:
				t.Fatalf("download call mutation = %d, want published mutation", mutation)
			}
			call, err := grant.DownloadCall(request)
			if !errors.Is(err, core.ErrRetrievalContract) || !downloadCallIsZero(call) {
				t.Fatalf("DownloadCall(%s) = (%v, %v), want zero and errors.Is %v",
					downloadCallMutationName(mutation), call, err, core.ErrRetrievalContract)
			}
		})
	}
}

func downloadCallMutationName(mutation downloadCallMutation) string {
	return [...]string{
		"zero request", "nil destination", "zero policy", "zero operation timeout",
		"zero attempt timeout", "zero error limit", "attempt exceeds operation", "zero grant",
	}[mutation]
}

func downloadCallIsZero(call objectstore.DownloadCapabilityRequest) bool {
	return call.Destination == nil && call.Capability.IsZero() && call.ContentType.IsZero()
}

func newDownloadCallFixture(t testing.TB, fixtureRequest downloadCallFixtureRequest) downloadCallFixture {
	t.Helper()

	if fixtureRequest.Selection == (Selection{}) {
		fixtureRequest.Selection = StartAll()
	}
	if fixtureRequest.Continuation == core.CatalogContinuationUnknown {
		fixtureRequest.Continuation = core.CatalogContinuationEnd
	}
	if fixtureRequest.EntrySequence == 0 {
		fixtureRequest.EntrySequence = 1
	}
	if fixtureRequest.ManifestEntries == 0 {
		fixtureRequest.ManifestEntries = fixtureRequest.EntrySequence
	}
	private, trusted := retrievalAuthority(t, 0x61)
	request := newRetrievalRequestFixture(t, retrievalRequestFixtureRequest{Selection: fixtureRequest.Selection}).payload
	manifest := chit.NewManifestAccumulator()
	entrySequence, err := chit.NewEntrySequence(fixtureRequest.EntrySequence)
	if err != nil {
		t.Fatalf("chit.NewEntrySequence(%d) error = %v, want nil", fixtureRequest.EntrySequence, err)
	}
	entryVerifier, err := chit.NewManifestEntryVerifier(entrySequence)
	if err != nil {
		t.Fatalf("chit.NewManifestEntryVerifier() error = %v, want nil", err)
	}
	var addition chit.ManifestAddition
	var scope receipt.Scope
	for sequence := uint64(1); sequence <= fixtureRequest.ManifestEntries; sequence++ {
		payload := []byte{byte(sequence)}
		if sequence == fixtureRequest.EntrySequence {
			payload = fixtureRequest.Payload
		}
		candidate, candidateScope := retrievalManifestAddition(t, retrievalEvidenceRequest{
			Private: private, Trusted: trusted, Payload: payload, Sequence: sequence,
		})
		if sequence == 1 {
			scope = candidateScope
		}
		if candidateScope != scope {
			t.Fatalf("retrieval manifest scope at sequence %d = %v, want %v", sequence, candidateScope, scope)
		}
		if err := manifest.Add(candidate); err != nil {
			t.Fatalf("ManifestAccumulator.Add(sequence %d) error = %v, want nil", sequence, err)
		}
		if err := entryVerifier.Add(candidate); err != nil {
			t.Fatalf("ManifestEntryVerifier.Add(sequence %d) error = %v, want nil", sequence, err)
		}
		if sequence == fixtureRequest.EntrySequence {
			addition = candidate
		}
	}
	if err := addition.Validate(); err != nil {
		t.Fatalf("retrieval target entry sequence %d is absent from %d manifest entries: %v",
			fixtureRequest.EntrySequence, fixtureRequest.ManifestEntries, err)
	}
	summary, err := manifest.Seal()
	if err != nil {
		t.Fatalf("ManifestAccumulator.Seal() error = %v, want nil", err)
	}
	membership, err := entryVerifier.Seal(summary)
	if err != nil {
		t.Fatalf("ManifestEntryVerifier.Seal() error = %v, want nil", err)
	}
	verifiedChit := retrievalVerifiedChit(t, retrievalChitRequest{
		Private: private, Trusted: trusted, Request: request,
		Scope: scope, Summary: summary,
	})
	capability := retrievalDownloadCapability(t, retrievalGrantExpiresAt)
	commitment, err := capability.Commitment()
	if err != nil {
		t.Fatalf("DownloadCapabilityProjection.Commitment() error = %v, want nil", err)
	}
	requestCommitment, err := CommitRequest(request)
	if err != nil {
		t.Fatalf("CommitRequest() error = %v, want nil", err)
	}
	nonceBytes := [core.SHA256DigestBytes]byte{0x71}
	nonce, err := controlwire.NewAuthorityNonce(nonceBytes)
	if err != nil {
		t.Fatalf("controlwire.NewAuthorityNonce() error = %v, want nil", err)
	}
	grantPayload := GrantPayload{
		Entry: addition.Entry, Request: requestCommitment, Authorization: nonce,
		Capability: commitment, Manifest: summary.Digest, Chit: request.Chit,
		IssuedAt:     temporal.InstantFromNanoseconds(retrievalGrantIssuedAt),
		ExpiresAt:    temporal.InstantFromNanoseconds(retrievalGrantExpiresAt),
		Continuation: fixtureRequest.Continuation,
	}
	projection, err := IssueGrant(GrantIssuance{
		Signer: private, Capability: capability, Payload: grantPayload,
		Entry: membership, Chit: verifiedChit, Request: request,
	})
	if err != nil {
		t.Fatalf("IssueGrant() error = %v, want nil", err)
	}
	encoded, err := projection.MarshalJSON()
	if err != nil {
		t.Fatalf("GrantProjection.MarshalJSON() error = %v, want nil", err)
	}
	var document GrantDocument
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatalf("json.Unmarshal(GrantDocument) error = %v, want nil", err)
	}
	grant, err := VerifyGrant(GrantExpectation{
		Document: document, Request: request, Chit: verifiedChit, Entry: membership,
		ObservedAt: temporal.InstantFromNanoseconds(retrievalGrantObserved), TrustedKeys: trusted,
	})
	if err != nil {
		t.Fatalf("VerifyGrant() error = %v, want nil", err)
	}
	return downloadCallFixture{
		private: private, grant: grant, payload: fixtureRequest.Payload, policy: retrievalPolicy(t), document: document,
		request: request, chit: verifiedChit, trusted: trusted, capability: capability,
		addition: addition, membership: membership, grantPayload: grantPayload,
	}
}

type retrievalEvidenceRequest struct {
	Private  ed25519.PrivateKey
	Payload  []byte
	Trusted  attest.TrustedKeys
	Sequence uint64
}

func retrievalManifestAddition(t testing.TB, request retrievalEvidenceRequest) (chit.ManifestAddition, receipt.Scope) {
	t.Helper()
	scope := receipt.Scope{
		Account:  retrievalLifecycleIdentity(t, 0x21, receipt.NewAccountIdentity),
		Offering: retrievalOfferingIdentity(t, retrievalOffering(t, 2)),
	}
	extent, err := core.NewByteLength(uint64(len(request.Payload)))
	if err != nil {
		t.Fatalf("core.NewByteLength() error = %v, want nil", err)
	}
	receiptBytes := [receipt.ReceiptIDBytes]byte{0x23, byte(request.Sequence)}
	receiptID, err := receipt.NewReceiptID(receiptBytes)
	if err != nil {
		t.Fatalf("receipt.NewReceiptID() error = %v, want nil", err)
	}
	evidence, err := receipt.IssueEvidence(receipt.IssueEvidenceRequest{
		Key: request.Private, Identity: receiptID, Account: scope.Account, Offering: scope.Offering,
		OccurredAt: temporal.InstantFromNanoseconds(retrievalGrantIssuedAt),
		Body: receipt.EvidenceBody{
			Extent: extent, SHA256: core.SHA256Of(request.Payload),
			CRC32C:     core.NewCRC32C(crc32.Checksum(request.Payload, crc32.MakeTable(crc32.Castagnoli))),
			Submission: retrievalLifecycleIdentity(t, 0x24+byte(request.Sequence), receipt.NewSubmissionIdentity),
			Object:     retrievalLifecycleIdentity(t, 0x25+byte(request.Sequence), receipt.NewObjectIdentity),
		},
	})
	if err != nil {
		t.Fatalf("receipt.IssueEvidence() error = %v, want nil", err)
	}
	verified, err := receipt.VerifyEvidence(receipt.VerifyEvidenceRequest{
		Document: evidence, TrustedKeys: request.Trusted,
		Expected: receipt.EvidenceExpectation{Account: scope.Account, Offering: scope.Offering, Body: evidence.Payload.Body},
	})
	if err != nil {
		t.Fatalf("receipt.VerifyEvidence() error = %v, want nil", err)
	}
	name, err := chit.ParseEntryName("evidence/result-" + strconv.FormatUint(request.Sequence, 10) + ".json")
	if err != nil {
		t.Fatalf("chit.ParseEntryName() error = %v, want nil", err)
	}
	sequence, err := chit.NewEntrySequence(request.Sequence)
	if err != nil {
		t.Fatalf("chit.NewEntrySequence() error = %v, want nil", err)
	}
	entry := chit.ManifestEntry{
		Name: name, ContentType: core.HTTPMediaTypeOctetStream(), Evidence: evidence, Sequence: sequence,
	}
	return chit.ManifestAddition{Entry: entry, Evidence: verified}, scope
}

func retrievalOfferingIdentity(t testing.TB, offering core.Offering) core.Offering {
	t.Helper()
	if err := offering.Validate(); err != nil {
		t.Fatalf("Offering.Validate() error = %v, want nil", err)
	}
	return offering
}

type retrievalChitRequest struct {
	Scope   receipt.Scope
	Private ed25519.PrivateKey
	Request RequestPayload
	Trusted attest.TrustedKeys
	Summary chit.ManifestSummary
}

func retrievalVerifiedChit(t testing.TB, request retrievalChitRequest) chit.Verified {
	t.Helper()
	collection, err := chit.ParseCollectionID("00000000-0003-7000-8000-000000000003")
	if err != nil {
		t.Fatalf("chit.ParseCollectionID() error = %v, want nil", err)
	}
	version, err := chit.NewVersion(1)
	if err != nil {
		t.Fatalf("chit.NewVersion() error = %v, want nil", err)
	}
	document, err := chit.Issue(chit.Issuance{
		Signer: request.Private, TrustedKeys: request.Trusted,
		Payload: chit.Payload{
			Identity: request.Request.Chit, Collection: collection,
			Partition: retrievalPartition(t, 0x71), Scope: request.Scope,
			Manifest:    request.Summary,
			AcceptedAt:  temporal.InstantFromNanoseconds(retrievalGrantIssuedAt - 1),
			RetainUntil: temporal.InstantFromNanoseconds(retrievalGrantExpiresAt + 1), Version: version,
		},
	})
	if err != nil {
		t.Fatalf("chit.Issue() error = %v, want nil", err)
	}
	verified, err := chit.Verify(chit.Verification{
		Document:    document,
		Expected:    chit.Expectation{Identity: request.Request.Chit, Scope: request.Scope},
		TrustedKeys: request.Trusted,
	})
	if err != nil {
		t.Fatalf("chit.Verify() error = %v, want nil", err)
	}
	return verified
}

func retrievalPartition(t testing.TB, marker byte) chit.Partition {
	t.Helper()
	raw := [core.SHA256DigestBytes]byte{}
	for index := range raw {
		raw[index] = marker
	}
	partition, err := chit.NewPartition(core.NewSHA256Digest(raw))
	if err != nil {
		t.Fatalf("chit.NewPartition(marker %d) error = %v, want nil", marker, err)
	}
	return partition
}

func retrievalDownloadCapability(t testing.TB, expiresAt int64) objectstore.DownloadCapabilityProjection {
	t.Helper()
	signed, err := objectstore.ParseSignedURL(
		core.SchemeHTTPS + "://" + core.GoogleCloudStorageHost + "/bucket/object" +
			"?X-Goog-Signature=fixture&X-Goog-SignedHeaders=host",
	)
	if err != nil {
		t.Fatalf("objectstore.ParseSignedURL() error = %v, want nil", err)
	}
	headers, err := objectstore.NewSignedHeaders(nil)
	if err != nil {
		t.Fatalf("objectstore.NewSignedHeaders() error = %v, want nil", err)
	}
	projection, err := objectstore.NewDownloadCapabilityProjection(
		objectstore.ProviderGoogleCloudStorage,
		objectstore.DownloadTarget{
			URL: signed, Headers: headers,
			ExpiresAt: temporal.InstantFromNanoseconds(expiresAt),
		},
	)
	if err != nil {
		t.Fatalf("objectstore.NewDownloadCapabilityProjection() error = %v, want nil", err)
	}
	return projection
}

func retrievalAuthority(t testing.TB, marker byte) (ed25519.PrivateKey, attest.TrustedKeys) {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{marker}, ed25519.SeedSize))
	public, err := core.NewEd25519PublicKey(private.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatalf("core.NewEd25519PublicKey() error = %v, want nil", err)
	}
	trusted, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{Keys: []core.Ed25519PublicKey{public}})
	if err != nil {
		t.Fatalf("attest.NewTrustedKeys() error = %v, want nil", err)
	}
	return private, trusted
}

func retrievalLifecycleIdentity[T core.Validatable](
	t testing.TB,
	marker byte,
	constructor func([receipt.LifecycleIdentityBytes]byte) (T, error),
) T {
	t.Helper()
	value := [receipt.LifecycleIdentityBytes]byte{marker}
	identity, err := constructor(value)
	if err != nil {
		t.Fatalf("retrieval lifecycle constructor error = %v, want nil", err)
	}
	return identity
}

func retrievalPolicy(t testing.TB) objectstore.Policy {
	t.Helper()
	limit, err := core.NewByteCount(4096)
	if err != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
	return objectstore.Policy{
		OperationTimeout: mustRetrievalDuration(t, 10),
		AttemptTimeout:   mustRetrievalDuration(t, 5), ErrorBodyLimit: limit,
	}
}

func mustRetrievalDuration(t testing.TB, seconds uint64) temporal.Duration {
	t.Helper()
	duration, err := temporal.DurationFromSeconds(seconds)
	if err != nil {
		t.Fatalf("temporal.DurationFromSeconds(%d) error = %v, want nil", seconds, err)
	}
	return duration
}
