package distribution_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/distribution"
	"github.com/deliri/primitive/v2026/release"
	"github.com/deliri/primitive/v2026/temporal"
)

type updateExchangeFixture struct {
	latest        release.VerifiedLatest
	responseDoc   distribution.UpdateResponseDocument
	callerKeys    attest.TrustedKeys
	authorityKeys attest.TrustedKeys
	releaseKeys   attest.TrustedKeys
	requestDoc    distribution.UpdateRequestDocument
	request       distribution.UpdateRequestPayload
}

func newUpdateExchangeFixture(t *testing.T) updateExchangeFixture {
	t.Helper()
	installed := newReleaseFixture(t, core.NewReleaseVersion(2026, 0, 54), 1)
	candidate := newReleaseFixture(t, core.NewReleaseVersion(2026, 0, 55), 2)
	callerKey := signingKey(71)
	authorityKey := signingKey(101)
	releaseKey := signingKey(19)
	request := distribution.UpdateRequestPayload{
		Build: installed.builds[2], Nonce: requestNonce(t, 11),
		Revision: controlwire.Revision2026V1,
	}
	requestDocument, err := distribution.IssueUpdateRequest(distribution.UpdateRequestIssuance{
		Signer: callerKey, Payload: request,
	})
	if err != nil {
		t.Fatalf("distribution.IssueUpdateRequest() error = %v, want nil", err)
	}
	requestWire, err := json.Marshal(requestDocument)
	if err != nil {
		t.Fatalf("json.Marshal(distribution.UpdateRequestDocument) error = %v, want nil", err)
	}
	var receivedRequest distribution.UpdateRequestDocument
	if err := json.Unmarshal(requestWire, &receivedRequest); err != nil {
		t.Fatalf("json.Unmarshal(distribution.UpdateRequestDocument) error = %v, want nil", err)
	}
	verifiedRequest, err := distribution.VerifyUpdateRequest(distribution.UpdateRequestVerification{
		Document: receivedRequest, TrustedKeys: trustedKeys(t, callerKey),
	})
	if err != nil {
		t.Fatalf("distribution.VerifyUpdateRequest() error = %v, want nil", err)
	}
	verifiedPayload, err := verifiedRequest.Payload()
	if err != nil || verifiedPayload != request {
		t.Fatalf("VerifiedUpdateRequest.Payload() = (%v, %v), want exact request", verifiedPayload, err)
	}
	commitment, err := distribution.CommitRequest(request)
	if err != nil {
		t.Fatalf("distribution.CommitRequest(update) error = %v, want nil", err)
	}
	if got := commitment.Domain(); got != distribution.SigningDomainUpdateRequestV1 {
		t.Fatalf(
			"distribution.CommitRequest(update).Domain() = %v, want %v",
			got,
			distribution.SigningDomainUpdateRequestV1,
		)
	}
	response, err := distribution.IssueUpdateResponse(distribution.UpdateResponseIssuance{
		Signer: authorityKey,
		Payload: distribution.UpdateResponsePayload{
			Request: commitment, Latest: candidate.latest.Document(),
			IssuedAt:  temporal.InstantFromNanoseconds(2_500),
			ExpiresAt: temporal.InstantFromNanoseconds(3_500),
		},
	})
	if err != nil {
		t.Fatalf("distribution.IssueUpdateResponse() error = %v, want nil", err)
	}
	responseWire, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("json.Marshal(distribution.UpdateResponseDocument) error = %v, want nil", err)
	}
	var receivedResponse distribution.UpdateResponseDocument
	if err := json.Unmarshal(responseWire, &receivedResponse); err != nil {
		t.Fatalf("json.Unmarshal(distribution.UpdateResponseDocument) error = %v, want nil", err)
	}
	return updateExchangeFixture{
		request: request, requestDoc: receivedRequest, responseDoc: receivedResponse,
		callerKeys: trustedKeys(t, callerKey), authorityKeys: trustedKeys(t, authorityKey),
		releaseKeys: trustedKeys(t, releaseKey), latest: candidate.latest,
	}
}

func TestUpdateExchangeLayerTriadAuthenticatesLatestWithoutInstallingAnything(t *testing.T) {
	t.Parallel()

	fixture := newUpdateExchangeFixture(t)
	verification := distribution.UpdateResponseVerification{
		Request: fixture.request, Document: fixture.responseDoc,
		ResponseKeys: fixture.authorityKeys, LatestKeys: fixture.releaseKeys,
		ManifestKeys: fixture.releaseKeys, ExpectedOffering: core.OfferingBug,
		ObservedAt: temporal.InstantFromNanoseconds(3_000),
	}
	verified, err := distribution.VerifyUpdateResponse(verification)
	if err != nil {
		t.Fatalf("distribution.VerifyUpdateResponse() error = %v, want nil", err)
	}
	latest, err := verified.Latest()
	if err != nil || latest.Document() != fixture.latest.Document() {
		t.Fatalf("VerifiedUpdateResponse.Latest() = (%v, %v), want exact authenticated Latest", latest, err)
	}
	assessment, err := release.AssessLatest(release.AssessLatestRequest{
		Latest: latest, Observation: temporal.InstantFromNanoseconds(3_000),
	})
	if err != nil || assessment.Freshness() != release.LatestFreshnessCurrent {
		t.Fatalf("release.AssessLatest(update result) = (%v, %v), want current", assessment.Freshness(), err)
	}

	otherRequest := fixture.request
	otherRequest.Nonce = requestNonce(t, 12)
	verification.Request = otherRequest
	if _, err := distribution.VerifyUpdateResponse(verification); !errors.Is(err, core.ErrDistributionBinding) {
		t.Fatalf("distribution.VerifyUpdateResponse(other request) error = %v, want %v", err, core.ErrDistributionBinding)
	}

	if gotErr := (distribution.VerifiedUpdateResponse{}).Validate(); !errors.Is(gotErr, core.ErrDistributionVerification) {
		t.Fatalf("distribution.VerifiedUpdateResponse{}.Validate() error = %v, want %v", gotErr, core.ErrDistributionVerification)
	}
}

func TestVerifyUpdateResponsePressuresBindingLifetimeAndAuthorityEdges(t *testing.T) {
	t.Parallel()

	fixture := newUpdateExchangeFixture(t)
	base := distribution.UpdateResponseVerification{
		Request: fixture.request, Document: fixture.responseDoc,
		ResponseKeys: fixture.authorityKeys, LatestKeys: fixture.releaseKeys,
		ManifestKeys: fixture.releaseKeys, ExpectedOffering: core.OfferingBug,
		ObservedAt: temporal.InstantFromNanoseconds(3_000),
	}
	otherKey := signingKey(151)
	otherKeys := trustedKeys(t, otherKey)
	otherRequest := fixture.request
	otherRequest.Nonce = requestNonce(t, 33)
	otherCommitment, err := distribution.CommitRequest(otherRequest)
	if err != nil {
		t.Fatalf("distribution.CommitRequest(other update) error = %v, want nil", err)
	}
	tampered := fixture.responseDoc
	tampered.Payload.Request = otherCommitment

	cases := []struct {
		wantErr error
		mutate  func(distribution.UpdateResponseVerification) distribution.UpdateResponseVerification
		name    string
	}{
		{name: "issued-at boundary is accepted", mutate: func(v distribution.UpdateResponseVerification) distribution.UpdateResponseVerification {
			v.ObservedAt = temporal.InstantFromNanoseconds(2_500)
			return v
		}},
		{name: "one nanosecond after issue is accepted", mutate: func(v distribution.UpdateResponseVerification) distribution.UpdateResponseVerification {
			v.ObservedAt = temporal.InstantFromNanoseconds(2_501)
			return v
		}},
		{name: "one nanosecond before expiry is accepted", mutate: func(v distribution.UpdateResponseVerification) distribution.UpdateResponseVerification {
			v.ObservedAt = temporal.InstantFromNanoseconds(3_499)
			return v
		}},
		{name: "one nanosecond before issue is rejected", mutate: func(v distribution.UpdateResponseVerification) distribution.UpdateResponseVerification {
			v.ObservedAt = temporal.InstantFromNanoseconds(2_499)
			return v
		}, wantErr: core.ErrDistributionBinding},
		{name: "exact expiry is rejected", mutate: func(v distribution.UpdateResponseVerification) distribution.UpdateResponseVerification {
			v.ObservedAt = temporal.InstantFromNanoseconds(3_500)
			return v
		}, wantErr: core.ErrDistributionBinding},
		{name: "one nanosecond after expiry is rejected", mutate: func(v distribution.UpdateResponseVerification) distribution.UpdateResponseVerification {
			v.ObservedAt = temporal.InstantFromNanoseconds(3_501)
			return v
		}, wantErr: core.ErrDistributionBinding},
		{name: "zero observation is rejected", mutate: func(v distribution.UpdateResponseVerification) distribution.UpdateResponseVerification {
			v.ObservedAt = temporal.Instant{}
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "different request nonce is rejected", mutate: func(v distribution.UpdateResponseVerification) distribution.UpdateResponseVerification {
			v.Request = otherRequest
			return v
		}, wantErr: core.ErrDistributionBinding},
		{name: "tampered request commitment reaches signature refusal", mutate: func(v distribution.UpdateResponseVerification) distribution.UpdateResponseVerification {
			v.Request, v.Document = otherRequest, tampered
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "untrusted response signer is rejected", mutate: func(v distribution.UpdateResponseVerification) distribution.UpdateResponseVerification {
			v.ResponseKeys = otherKeys
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "untrusted Latest signer is rejected", mutate: func(v distribution.UpdateResponseVerification) distribution.UpdateResponseVerification {
			v.LatestKeys = otherKeys
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "untrusted manifest signer is rejected", mutate: func(v distribution.UpdateResponseVerification) distribution.UpdateResponseVerification {
			v.ManifestKeys = otherKeys
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "different offering is rejected", mutate: func(v distribution.UpdateResponseVerification) distribution.UpdateResponseVerification {
			v.ExpectedOffering = core.OfferingPeachfuzz
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "unset response document is rejected", mutate: func(v distribution.UpdateResponseVerification) distribution.UpdateResponseVerification {
			v.Document = distribution.UpdateResponseDocument{}
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "unset response trust is rejected", mutate: func(v distribution.UpdateResponseVerification) distribution.UpdateResponseVerification {
			v.ResponseKeys = attest.TrustedKeys{}
			return v
		}, wantErr: core.ErrDistributionVerification},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := distribution.VerifyUpdateResponse(tc.mutate(base))
			if tc.wantErr == nil {
				if gotErr != nil {
					t.Fatalf("distribution.VerifyUpdateResponse(boundary) error = %v, want nil", gotErr)
				}
				if err := got.Validate(); err != nil {
					t.Fatalf("accepted distribution.VerifiedUpdateResponse.Validate() error = %v, want nil", err)
				}
				return
			}
			if !errors.Is(gotErr, tc.wantErr) || !errors.Is(gotErr, core.ErrDistributionContract) {
				t.Fatalf("distribution.VerifyUpdateResponse(boundary) error = %v, want %v and %v", gotErr, tc.wantErr, core.ErrDistributionContract)
			}
			if got != (distribution.VerifiedUpdateResponse{}) {
				t.Fatalf("rejected distribution.VerifiedUpdateResponse = %v, want zero", got)
			}
		})
	}
}
