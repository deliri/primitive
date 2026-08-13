package distribution_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"sync"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/deploy"
	"github.com/deliri/primitive/v2026/distribution"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/release"
	"github.com/deliri/primitive/v2026/temporal"
)

type publicationExchangeFixture struct {
	callerKey       ed25519.PrivateKey
	authorityKey    ed25519.PrivateKey
	grantProjection distribution.PublicationGrantProjection
	grantDocument   distribution.PublicationGrantDocument
	request         distribution.PublicationRequestPayload
	requestDocument distribution.PublicationRequestDocument
	verifiedGrant   distribution.VerifiedPublicationGrant
	verifiedRequest distribution.VerifiedPublicationRequest
	release         releaseFixture
	authorityKeys   attest.TrustedKeys
	callerKeys      attest.TrustedKeys
	releaseKeys     attest.TrustedKeys
	grantPayload    distribution.PublicationGrantPayload
}

type publicationTransport struct {
	mu             sync.Mutex
	failAt         int
	requests       int
	generationBase int
}

func (t *publicationTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	t.mu.Lock()
	index := t.requests
	t.requests++
	t.mu.Unlock()
	if index == t.failAt {
		return nil, errors.New("injected publication transport refusal")
	}
	if _, err := io.Copy(io.Discard, request.Body); err != nil {
		return nil, err
	}
	if err := request.Body.Close(); err != nil {
		return nil, err
	}
	headers := make(http.Header)
	headers.Set("x-goog-generation", strconv.Itoa(t.generationBase+index+1))
	return &http.Response{
		StatusCode: http.StatusOK, Header: headers,
		Body: http.NoBody, ContentLength: 0, Request: request,
	}, nil
}

func (t *publicationTransport) count() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.requests
}

func newPublicationExchangeFixture(t testing.TB) publicationExchangeFixture {
	t.Helper()
	software := newReleaseFixture(t, core.NewReleaseVersion(2026, 0, 55), 2)
	callerKey := signingKey(71)
	authorityKey := signingKey(101)
	releaseKey := signingKey(19)
	request := distribution.PublicationRequestPayload{
		Manifest: software.manifest.Document(), Build: software.builds[0], Nonce: requestNonce(t, 41),
		Revision: controlwire.Revision2026V1,
	}
	requestDocument, err := distribution.IssuePublicationRequest(
		distribution.PublicationRequestIssuance{Signer: callerKey, Payload: request},
	)
	if err != nil {
		t.Fatalf("distribution.IssuePublicationRequest() error = %v, want nil", err)
	}
	requestWire, err := json.Marshal(requestDocument)
	if err != nil {
		t.Fatalf("json.Marshal(PublicationRequestDocument) error = %v, want nil", err)
	}
	var receivedRequest distribution.PublicationRequestDocument
	if err := json.Unmarshal(requestWire, &receivedRequest); err != nil {
		t.Fatalf("json.Unmarshal(PublicationRequestDocument) error = %v, want nil", err)
	}
	verifiedRequest, err := distribution.VerifyPublicationRequest(
		distribution.PublicationRequestVerification{
			Document: receivedRequest, RequestKeys: trustedKeys(t, callerKey),
			ManifestKeys: trustedKeys(t, releaseKey), ExpectedOffering: core.OfferingBug,
		},
	)
	if err != nil {
		t.Fatalf("distribution.VerifyPublicationRequest() error = %v, want nil", err)
	}
	requestCommitment, err := distribution.CommitRequest(request)
	if err != nil {
		t.Fatalf("distribution.CommitRequest(publication) error = %v, want nil", err)
	}
	if got := requestCommitment.Domain(); got != distribution.SigningDomainPublicationRequestV1 {
		t.Fatalf(
			"distribution.CommitRequest(publication).Domain() = %v, want %v",
			got,
			distribution.SigningDomainPublicationRequestV1,
		)
	}
	var projections [release.PublicationObjectCount]objectstore.UploadCapabilityProjection
	var commitments [release.PublicationObjectCount]objectstore.UploadCapabilityCommitment
	for index := range projections {
		projection, _ := uploadCapabilityProjection(t, index)
		commitment, commitmentErr := projection.Commitment()
		if commitmentErr != nil {
			t.Fatalf("UploadCapabilityProjection(%d).Commitment() error = %v, want nil", index, commitmentErr)
		}
		projections[index], commitments[index] = projection, commitment
	}
	grantPayload := distribution.PublicationGrantPayload{
		Request: requestCommitment, Authorization: authorityNonce(t, 51),
		Commitments: commitments, IssuedAt: temporal.InstantFromNanoseconds(2_500),
		ExpiresAt: temporal.InstantFromNanoseconds(3_500),
	}
	grantProjection, err := distribution.IssuePublicationGrant(
		distribution.PublicationGrantIssuance{
			Signer: authorityKey, Capabilities: projections, Payload: grantPayload,
		},
	)
	if err != nil {
		t.Fatalf("distribution.IssuePublicationGrant() error = %v, want nil", err)
	}
	grantWire, err := json.Marshal(grantProjection)
	if err != nil {
		t.Fatalf("json.Marshal(PublicationGrantProjection) error = %v, want nil", err)
	}
	var grantDocument distribution.PublicationGrantDocument
	if err := json.Unmarshal(grantWire, &grantDocument); err != nil {
		t.Fatalf("json.Unmarshal(PublicationGrantDocument) error = %v, want nil", err)
	}
	verifiedGrant, err := distribution.VerifyPublicationGrant(
		distribution.PublicationGrantExpectation{
			Request: request, Document: grantDocument,
			TrustedKeys: trustedKeys(t, authorityKey),
			ObservedAt:  temporal.InstantFromNanoseconds(3_000),
		},
	)
	if err != nil {
		t.Fatalf("distribution.VerifyPublicationGrant() error = %v, want nil", err)
	}
	return publicationExchangeFixture{
		release: software, request: request, requestDocument: receivedRequest,
		verifiedRequest: verifiedRequest,
		grantPayload:    grantPayload, grantProjection: grantProjection,
		grantDocument: grantDocument, verifiedGrant: verifiedGrant,
		callerKey: callerKey, authorityKey: authorityKey,
		authorityKeys: trustedKeys(t, authorityKey),
		callerKeys:    trustedKeys(t, callerKey), releaseKeys: trustedKeys(t, releaseKey),
	}
}

func completedPublicationDocument(
	t testing.TB,
	fixture publicationExchangeFixture,
	generationBase int,
) distribution.PublicationCompletionDocument {
	t.Helper()

	var sources [release.PublicationObjectCount]distribution.PublicationSource
	for index, payload := range fixture.release.payloads {
		sources[index] = distribution.PublicationSource{Reader: bytes.NewReader(payload)}
	}
	plan, err := distribution.PreparePublicationPlan(distribution.PublicationPlanRequest{
		Grant: fixture.verifiedGrant, Manifest: fixture.release.manifest,
		Sources: sources, Policy: objectstorePolicy(t),
	})
	if err != nil {
		t.Fatalf("distribution.PreparePublicationPlan() error = %v, want nil", err)
	}
	receipts, err := deploy.ReleaseGCS(
		context.Background(), objectstoreClient(t, &publicationTransport{
			failAt: -1, generationBase: generationBase,
		}), plan,
	)
	if err != nil {
		t.Fatalf("deploy.ReleaseGCS(completion fixture) error = %v, want nil", err)
	}
	projection, err := distribution.IssuePublicationCompletion(
		distribution.PublicationCompletionIssuance{
			Signer: fixture.callerKey, Request: fixture.verifiedRequest,
			Grant: fixture.verifiedGrant, Receipts: receipts,
		},
	)
	if err != nil {
		t.Fatalf("distribution.IssuePublicationCompletion() error = %v, want nil", err)
	}
	wire, err := json.Marshal(projection)
	if err != nil {
		t.Fatalf("json.Marshal(PublicationCompletionProjection) error = %v, want nil", err)
	}
	var document distribution.PublicationCompletionDocument
	if err := json.Unmarshal(wire, &document); err != nil {
		t.Fatalf("json.Unmarshal(PublicationCompletionDocument) error = %v, want nil", err)
	}
	documentWire, err := document.MarshalJSON()
	if err != nil {
		t.Fatalf("PublicationCompletionDocument.MarshalJSON() error = %v, want nil", err)
	}
	var documentRoundTrip distribution.PublicationCompletionDocument
	if err := documentRoundTrip.UnmarshalJSON(documentWire); err != nil || documentRoundTrip != document {
		t.Fatalf(
			"PublicationCompletionDocument canonical round trip = (%v, %v), want (%v, nil)",
			documentRoundTrip,
			err,
			document,
		)
	}
	secondDocumentWire, err := documentRoundTrip.MarshalJSON()
	if err != nil || !bytes.Equal(secondDocumentWire, documentWire) {
		t.Fatalf(
			"PublicationCompletionDocument second canonical projection = (%q, %v), want (%q, nil)",
			secondDocumentWire,
			err,
			documentWire,
		)
	}
	payloadWire, err := document.Payload.MarshalJSON()
	if err != nil {
		t.Fatalf("PublicationCompletionPayload.MarshalJSON() error = %v, want nil", err)
	}
	var payloadRoundTrip distribution.PublicationCompletionPayload
	if err := payloadRoundTrip.UnmarshalJSON(payloadWire); err != nil || payloadRoundTrip != document.Payload {
		t.Fatalf(
			"PublicationCompletionPayload canonical round trip = (%v, %v), want (%v, nil)",
			payloadRoundTrip,
			err,
			document.Payload,
		)
	}
	secondPayloadWire, err := payloadRoundTrip.MarshalJSON()
	if err != nil || !bytes.Equal(secondPayloadWire, payloadWire) {
		t.Fatalf(
			"PublicationCompletionPayload second canonical projection = (%q, %v), want (%q, nil)",
			secondPayloadWire,
			err,
			payloadWire,
		)
	}
	return document
}

func TestPublicationLayerTriadExecutesExactPlanAndReturnsURLFreeCompletion(t *testing.T) {
	t.Parallel()

	fixture := newPublicationExchangeFixture(t)
	var progressMu sync.Mutex
	var completed [release.PublicationObjectCount]uint64
	var totals [release.PublicationObjectCount]uint64
	var sources [release.PublicationObjectCount]distribution.PublicationSource
	for index, payload := range fixture.release.payloads {
		slot := index
		sources[index] = distribution.PublicationSource{
			Reader: bytes.NewReader(payload),
			Observer: func(progress objectstore.TransferProgress) error {
				if err := progress.Validate(); err != nil {
					return err
				}
				if progress.Direction() != objectstore.DirectionUpload {
					return core.ErrDistributionBinding
				}
				progressMu.Lock()
				completed[slot] = progress.Completed().Uint64()
				totals[slot] = progress.Total().Uint64()
				progressMu.Unlock()
				return nil
			},
		}
	}
	plan, err := distribution.PreparePublicationPlan(distribution.PublicationPlanRequest{
		Grant: fixture.verifiedGrant, Manifest: fixture.release.manifest,
		Sources: sources, Policy: objectstorePolicy(t),
	})
	if err != nil {
		t.Fatalf("distribution.PreparePublicationPlan() error = %v, want nil", err)
	}
	transport := &publicationTransport{failAt: -1}
	receipts, err := deploy.ReleaseGCS(
		context.Background(), objectstoreClient(t, transport), plan,
	)
	if err != nil {
		t.Fatalf("deploy.ReleaseGCS(distribution plan) error = %v, want nil", err)
	}
	if receipts.Count() != release.PublicationObjectCount || transport.count() != release.PublicationObjectCount {
		t.Fatalf("publication counts = receipts %d requests %d, want %d", receipts.Count(), transport.count(), release.PublicationObjectCount)
	}
	progressMu.Lock()
	for index := range completed {
		if completed[index] == 0 || completed[index] != totals[index] || totals[index] != uint64(len(fixture.release.payloads[index])) {
			progressMu.Unlock()
			t.Fatalf("publication progress[%d] = %d/%d, want %d/%d", index, completed[index], totals[index], len(fixture.release.payloads[index]), len(fixture.release.payloads[index]))
		}
	}
	progressMu.Unlock()
	completionProjection, err := distribution.IssuePublicationCompletion(
		distribution.PublicationCompletionIssuance{
			Signer: fixture.callerKey, Request: fixture.verifiedRequest,
			Grant: fixture.verifiedGrant, Receipts: receipts,
		},
	)
	if err != nil {
		t.Fatalf("distribution.IssuePublicationCompletion() error = %v, want nil", err)
	}
	completionWire, err := json.Marshal(completionProjection)
	if err != nil {
		t.Fatalf("json.Marshal(PublicationCompletionProjection) error = %v, want nil", err)
	}
	if bytes.Contains(completionWire, []byte(core.GoogleCloudStorageHost)) {
		t.Fatalf("publication completion wire disclosed provider bearer material: %q", completionWire)
	}
	var completionDocument distribution.PublicationCompletionDocument
	if err := json.Unmarshal(completionWire, &completionDocument); err != nil {
		t.Fatalf("json.Unmarshal(PublicationCompletionDocument) error = %v, want nil", err)
	}
	verified, err := distribution.VerifyPublicationCompletion(
		distribution.PublicationCompletionExpectation{
			Request: fixture.verifiedRequest, Grant: fixture.grantPayload,
			GrantAttestation: fixture.grantProjection.Attestation,
			Document:         completionDocument, GrantKeys: fixture.authorityKeys,
			CompletionKeys: fixture.callerKeys,
		},
	)
	if err != nil {
		t.Fatalf("distribution.VerifyPublicationCompletion() error = %v, want nil", err)
	}
	payload, err := verified.Payload()
	if err != nil || payload.Manifest != fixture.release.manifest.Identity() ||
		payload.ManifestDocument != fixture.release.manifest.DocumentDigest() {
		t.Fatalf("VerifiedPublicationCompletion.Payload() = (%v, %v), want exact manifest", payload, err)
	}
}

func TestPublicationLayerTriadRejectsPartialReceiptsAndCrossManifestPlan(t *testing.T) {
	t.Parallel()

	fixture := newPublicationExchangeFixture(t)
	var sources [release.PublicationObjectCount]distribution.PublicationSource
	for index, payload := range fixture.release.payloads {
		sources[index] = distribution.PublicationSource{Reader: bytes.NewReader(payload)}
	}
	plan, err := distribution.PreparePublicationPlan(distribution.PublicationPlanRequest{
		Grant: fixture.verifiedGrant, Manifest: fixture.release.manifest,
		Sources: sources, Policy: objectstorePolicy(t),
	})
	if err != nil {
		t.Fatalf("distribution.PreparePublicationPlan() error = %v, want nil", err)
	}
	transport := &publicationTransport{failAt: 3}
	receipts, uploadErr := deploy.ReleaseGCS(
		context.Background(), objectstoreClient(t, transport), plan,
	)
	if !errors.Is(uploadErr, core.ErrDeployContract) || !errors.Is(uploadErr, core.ErrObjectStoreContract) ||
		!errors.Is(uploadErr, core.ErrExchangeTransport) || receipts.Count() != 3 || transport.count() != 4 {
		t.Fatalf("deploy.ReleaseGCS(partial) = receipts %d requests %d error %v, want 3, 4, errors.Is %v, %v, and %v",
			receipts.Count(), transport.count(), uploadErr, core.ErrDeployContract,
			core.ErrObjectStoreContract, core.ErrExchangeTransport)
	}
	_, err = distribution.IssuePublicationCompletion(
		distribution.PublicationCompletionIssuance{
			Signer: fixture.callerKey, Request: fixture.verifiedRequest,
			Grant: fixture.verifiedGrant, Receipts: receipts,
		},
	)
	if !errors.Is(err, core.ErrDistributionContract) {
		t.Fatalf("distribution.IssuePublicationCompletion(partial) error = %v, want %v", err, core.ErrDistributionContract)
	}

	other := newReleaseFixture(t, core.NewReleaseVersion(2026, 0, 56), 3)
	_, err = distribution.PreparePublicationPlan(distribution.PublicationPlanRequest{
		Grant: fixture.verifiedGrant, Manifest: other.manifest,
		Sources: sources, Policy: objectstorePolicy(t),
	})
	if !errors.Is(err, core.ErrDistributionBinding) {
		t.Fatalf("distribution.PreparePublicationPlan(cross manifest) error = %v, want %v", err, core.ErrDistributionBinding)
	}

	var emptySources [release.PublicationObjectCount]distribution.PublicationSource
	_, err = distribution.PreparePublicationPlan(distribution.PublicationPlanRequest{
		Grant: fixture.verifiedGrant, Manifest: fixture.release.manifest,
		Sources: emptySources, Policy: objectstorePolicy(t),
	})
	if !errors.Is(err, core.ErrDistributionContract) {
		t.Fatalf("distribution.PreparePublicationPlan(no sources) error = %v, want %v", err, core.ErrDistributionContract)
	}
}

func TestVerifyPublicationGrantPressuresEveryBindingAndLifetimeEdge(t *testing.T) {
	t.Parallel()

	fixture := newPublicationExchangeFixture(t)
	base := distribution.PublicationGrantExpectation{
		Request: fixture.request, Document: fixture.grantDocument,
		TrustedKeys: fixture.authorityKeys, ObservedAt: temporal.InstantFromNanoseconds(3_000),
	}
	otherRequest := fixture.request
	otherRequest.Nonce = requestNonce(t, 42)
	otherKeys := trustedKeys(t, signingKey(151))
	swapped := fixture.grantDocument
	swapped.Capabilities[0], swapped.Capabilities[1] = swapped.Capabilities[1], swapped.Capabilities[0]

	cases := []struct {
		wantErr error
		mutate  func(distribution.PublicationGrantExpectation) distribution.PublicationGrantExpectation
		name    string
	}{
		{name: "issued-at boundary is accepted", mutate: func(v distribution.PublicationGrantExpectation) distribution.PublicationGrantExpectation {
			v.ObservedAt = temporal.InstantFromNanoseconds(2_500)
			return v
		}},
		{name: "one nanosecond after issue is accepted", mutate: func(v distribution.PublicationGrantExpectation) distribution.PublicationGrantExpectation {
			v.ObservedAt = temporal.InstantFromNanoseconds(2_501)
			return v
		}},
		{name: "one nanosecond before expiry is accepted", mutate: func(v distribution.PublicationGrantExpectation) distribution.PublicationGrantExpectation {
			v.ObservedAt = temporal.InstantFromNanoseconds(3_499)
			return v
		}},
		{name: "one nanosecond before issue is rejected", mutate: func(v distribution.PublicationGrantExpectation) distribution.PublicationGrantExpectation {
			v.ObservedAt = temporal.InstantFromNanoseconds(2_499)
			return v
		}, wantErr: core.ErrDistributionBinding},
		{name: "exact expiry is rejected", mutate: func(v distribution.PublicationGrantExpectation) distribution.PublicationGrantExpectation {
			v.ObservedAt = temporal.InstantFromNanoseconds(3_500)
			return v
		}, wantErr: core.ErrDistributionBinding},
		{name: "one nanosecond after expiry is rejected", mutate: func(v distribution.PublicationGrantExpectation) distribution.PublicationGrantExpectation {
			v.ObservedAt = temporal.InstantFromNanoseconds(3_501)
			return v
		}, wantErr: core.ErrDistributionBinding},
		{name: "zero observation is rejected", mutate: func(v distribution.PublicationGrantExpectation) distribution.PublicationGrantExpectation {
			v.ObservedAt = temporal.Instant{}
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "different request nonce is rejected", mutate: func(v distribution.PublicationGrantExpectation) distribution.PublicationGrantExpectation {
			v.Request = otherRequest
			return v
		}, wantErr: core.ErrDistributionBinding},
		{name: "swapped bearer slots are rejected", mutate: func(v distribution.PublicationGrantExpectation) distribution.PublicationGrantExpectation {
			v.Document = swapped
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "untrusted authority signer is rejected", mutate: func(v distribution.PublicationGrantExpectation) distribution.PublicationGrantExpectation {
			v.TrustedKeys = otherKeys
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "unset document is rejected", mutate: func(v distribution.PublicationGrantExpectation) distribution.PublicationGrantExpectation {
			v.Document = distribution.PublicationGrantDocument{}
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "unset request is rejected", mutate: func(v distribution.PublicationGrantExpectation) distribution.PublicationGrantExpectation {
			v.Request = distribution.PublicationRequestPayload{}
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "unset trust is rejected", mutate: func(v distribution.PublicationGrantExpectation) distribution.PublicationGrantExpectation {
			v.TrustedKeys = attest.TrustedKeys{}
			return v
		}, wantErr: core.ErrDistributionVerification},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := distribution.VerifyPublicationGrant(tc.mutate(base))
			if tc.wantErr == nil {
				if gotErr != nil || got.Validate() != nil {
					t.Fatalf("distribution.VerifyPublicationGrant(boundary) = (%v, %v), want valid proof", got, gotErr)
				}
				return
			}
			if !errors.Is(gotErr, tc.wantErr) || !errors.Is(gotErr, core.ErrDistributionContract) {
				t.Fatalf("distribution.VerifyPublicationGrant(boundary) error = %v, want %v and %v", gotErr, tc.wantErr, core.ErrDistributionContract)
			}
			if err := got.Validate(); !errors.Is(err, core.ErrDistributionVerification) {
				t.Fatalf("rejected distribution.VerifiedPublicationGrant.Validate() error = %v, want %v", err, core.ErrDistributionVerification)
			}
		})
	}
}

func TestVerifyPublicationCompletionRefusesCrossGrantManifestEvidenceAndTrust(t *testing.T) {
	t.Parallel()

	fixture := newPublicationExchangeFixture(t)
	document := completedPublicationDocument(t, fixture, 0)
	alternateDocument := completedPublicationDocument(t, fixture, 100)
	base := distribution.PublicationCompletionExpectation{
		Request: fixture.verifiedRequest, Grant: fixture.grantPayload,
		GrantAttestation: fixture.grantProjection.Attestation, Document: document,
		GrantKeys: fixture.authorityKeys, CompletionKeys: fixture.callerKeys,
	}
	otherRelease := newReleaseFixture(t, core.NewReleaseVersion(2026, 0, 56), 3)
	otherKey := trustedKeys(t, signingKey(151))
	otherAuthorization := authorityNonce(t, 91)
	otherRequest := fixture.request
	otherRequest.Nonce = requestNonce(t, 92)
	otherCommitment, err := distribution.CommitRequest(otherRequest)
	if err != nil {
		t.Fatalf("distribution.CommitRequest(other publication) error = %v, want nil", err)
	}
	resignCompletion := func(v distribution.PublicationCompletionExpectation) distribution.PublicationCompletionExpectation {
		envelope, signErr := attest.Sign(attest.SignRequest[distribution.SigningDomain]{
			Body: v.Document.Payload, Signer: fixture.callerKey,
		})
		if signErr != nil {
			t.Fatalf("attest.Sign(completion mutation) error = %v, want nil", signErr)
		}
		v.Document.Attestation = envelope
		return v
	}
	resignGrant := func(v distribution.PublicationCompletionExpectation) distribution.PublicationCompletionExpectation {
		envelope, signErr := attest.Sign(attest.SignRequest[distribution.SigningDomain]{
			Body: v.Grant, Signer: fixture.authorityKey,
		})
		if signErr != nil {
			t.Fatalf("attest.Sign(grant mutation) error = %v, want nil", signErr)
		}
		v.GrantAttestation = envelope
		return v
	}

	cases := []struct {
		wantErr error
		mutate  func(distribution.PublicationCompletionExpectation) distribution.PublicationCompletionExpectation
		name    string
	}{
		{name: "exact request grant manifest evidence and trust are accepted", mutate: func(v distribution.PublicationCompletionExpectation) distribution.PublicationCompletionExpectation {
			return v
		}},
		{name: "untrusted grant signer is rejected", mutate: func(v distribution.PublicationCompletionExpectation) distribution.PublicationCompletionExpectation {
			v.GrantKeys = otherKey
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "untrusted completion signer is rejected", mutate: func(v distribution.PublicationCompletionExpectation) distribution.PublicationCompletionExpectation {
			v.CompletionKeys = otherKey
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "swapped artifact evidence slots are rejected", mutate: func(v distribution.PublicationCompletionExpectation) distribution.PublicationCompletionExpectation {
			v.Document.Payload.Evidence[0], v.Document.Payload.Evidence[1] =
				v.Document.Payload.Evidence[1], v.Document.Payload.Evidence[0]
			return resignCompletion(v)
		}, wantErr: core.ErrDistributionBinding},
		{name: "manifest identity from another release is rejected", mutate: func(v distribution.PublicationCompletionExpectation) distribution.PublicationCompletionExpectation {
			v.Document.Payload.Manifest = otherRelease.manifest.Identity()
			return resignCompletion(v)
		}, wantErr: core.ErrDistributionBinding},
		{name: "manifest document digest from another release is rejected", mutate: func(v distribution.PublicationCompletionExpectation) distribution.PublicationCompletionExpectation {
			v.Document.Payload.ManifestDocument = otherRelease.manifest.DocumentDigest()
			return resignCompletion(v)
		}, wantErr: core.ErrDistributionBinding},
		{name: "grant authorization from another decision is rejected", mutate: func(v distribution.PublicationCompletionExpectation) distribution.PublicationCompletionExpectation {
			v.Grant.Authorization = otherAuthorization
			return resignGrant(v)
		}, wantErr: core.ErrDistributionBinding},
		{name: "completion authorization from another decision is rejected", mutate: func(v distribution.PublicationCompletionExpectation) distribution.PublicationCompletionExpectation {
			v.Document.Payload.Authorization = otherAuthorization
			return resignCompletion(v)
		}, wantErr: core.ErrDistributionBinding},
		{name: "grant request commitment from another request is rejected", mutate: func(v distribution.PublicationCompletionExpectation) distribution.PublicationCompletionExpectation {
			v.Grant.Request = otherCommitment
			return resignGrant(v)
		}, wantErr: core.ErrDistributionBinding},
		{name: "matching authorization mutation reaches grant signature refusal", mutate: func(v distribution.PublicationCompletionExpectation) distribution.PublicationCompletionExpectation {
			v.Grant.Authorization = otherAuthorization
			v.Document.Payload.Authorization = otherAuthorization
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "valid alternate provider generations reach completion signature refusal", mutate: func(v distribution.PublicationCompletionExpectation) distribution.PublicationCompletionExpectation {
			v.Document.Payload.Evidence = alternateDocument.Payload.Evidence
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "grant-domain attestation cannot authenticate completion", mutate: func(v distribution.PublicationCompletionExpectation) distribution.PublicationCompletionExpectation {
			v.Document.Attestation = fixture.grantProjection.Attestation
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "unset grant trust is rejected", mutate: func(v distribution.PublicationCompletionExpectation) distribution.PublicationCompletionExpectation {
			v.GrantKeys = attest.TrustedKeys{}
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "unset completion trust is rejected", mutate: func(v distribution.PublicationCompletionExpectation) distribution.PublicationCompletionExpectation {
			v.CompletionKeys = attest.TrustedKeys{}
			return v
		}, wantErr: core.ErrDistributionVerification},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := distribution.VerifyPublicationCompletion(tc.mutate(base))
			if tc.wantErr == nil {
				if gotErr != nil || got.Validate() != nil {
					t.Fatalf("distribution.VerifyPublicationCompletion(exact) = (%v, %v), want valid proof", got, gotErr)
				}
				return
			}
			if !errors.Is(gotErr, tc.wantErr) || !errors.Is(gotErr, core.ErrDistributionContract) {
				t.Fatalf("distribution.VerifyPublicationCompletion(hostile) error = %v, want %v and %v", gotErr, tc.wantErr, core.ErrDistributionContract)
			}
			if err := got.Validate(); !errors.Is(err, core.ErrDistributionVerification) {
				t.Fatalf("rejected distribution.VerifiedPublicationCompletion.Validate() error = %v, want %v", err, core.ErrDistributionVerification)
			}
		})
	}
}
