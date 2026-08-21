package distribution_test

import (
	json "encoding/json/v2"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/distribution"
	"github.com/deliri/primitive/v2026/temporal"
)

type upgradeExchangeFixture struct {
	request         distribution.UpgradeRequestPayload
	grantProjection distribution.UpgradeGrantProjection
	grantDoc        distribution.UpgradeGrantDocument
	requestDoc      distribution.UpgradeRequestDocument
	callerKeys      attest.TrustedKeys
	authorityKeys   attest.TrustedKeys
}

func newUpgradeExchangeFixture(t testing.TB) upgradeExchangeFixture {
	t.Helper()

	installed := newReleaseFixture(t, core.NewReleaseVersion(2026, 0, 54), 1)
	candidate := newReleaseFixture(t, core.NewReleaseVersion(2026, 0, 55), 2)
	callerKey := signingKey(71)
	authorityKey := signingKey(101)
	request := distribution.UpgradeRequestPayload{
		Available: availableSummaryFixture(t, installed, candidate),
		Nonce:     requestNonce(t, 61), Revision: controlwire.Revision2026V1,
	}
	issuedRequest, err := distribution.IssueUpgradeRequest(distribution.UpgradeRequestIssuance{
		Signer: callerKey, Payload: request,
	})
	if err != nil {
		t.Fatalf("distribution.IssueUpgradeRequest() error = %v, want nil", err)
	}
	requestWire, err := json.Marshal(issuedRequest)
	if err != nil {
		t.Fatalf("json.Marshal(UpgradeRequestDocument) error = %v, want nil", err)
	}
	var requestDoc distribution.UpgradeRequestDocument
	if err := json.Unmarshal(requestWire, &requestDoc); err != nil {
		t.Fatalf("json.Unmarshal(UpgradeRequestDocument) error = %v, want nil", err)
	}
	verifiedRequest, err := distribution.VerifyUpgradeRequest(distribution.UpgradeRequestVerification{
		Document: requestDoc, TrustedKeys: trustedKeys(t, callerKey),
	})
	if err != nil {
		t.Fatalf("distribution.VerifyUpgradeRequest() error = %v, want nil", err)
	}
	verifiedPayload, err := verifiedRequest.Payload()
	if err != nil || verifiedPayload != request {
		t.Fatalf("VerifiedUpgradeRequest.Payload() = (%v, %v), want exact request", verifiedPayload, err)
	}
	requestCommitment, err := distribution.CommitRequest(request)
	if err != nil {
		t.Fatalf("distribution.CommitRequest(upgrade) error = %v, want nil", err)
	}
	if got := requestCommitment.Domain(); got != distribution.SigningDomainUpgradeRequestV1 {
		t.Fatalf(
			"distribution.CommitRequest(upgrade).Domain() = %v, want %v",
			got,
			distribution.SigningDomainUpgradeRequestV1,
		)
	}
	capability, _ := downloadCapabilityProjection(t, 7)
	commitment, err := capability.Commitment()
	if err != nil {
		t.Fatalf("DownloadCapabilityProjection.Commitment() error = %v, want nil", err)
	}
	projection, err := distribution.IssueUpgradeGrant(distribution.UpgradeGrantIssuance{
		Signer: authorityKey, Capability: capability,
		Payload: distribution.UpgradeGrantPayload{
			Request: requestCommitment, Authorization: authorityNonce(t, 62),
			Capability: commitment, IssuedAt: temporal.InstantFromNanoseconds(2_500),
			ExpiresAt: temporal.InstantFromNanoseconds(3_500),
		},
	})
	if err != nil {
		t.Fatalf("distribution.IssueUpgradeGrant() error = %v, want nil", err)
	}
	grantWire, err := json.Marshal(projection)
	if err != nil {
		t.Fatalf("json.Marshal(UpgradeGrantProjection) error = %v, want nil", err)
	}
	var grantDoc distribution.UpgradeGrantDocument
	if err := json.Unmarshal(grantWire, &grantDoc); err != nil {
		t.Fatalf("json.Unmarshal(UpgradeGrantDocument) error = %v, want nil", err)
	}
	return upgradeExchangeFixture{
		request: request, requestDoc: requestDoc, grantDoc: grantDoc, grantProjection: projection,
		callerKeys: trustedKeys(t, callerKey), authorityKeys: trustedKeys(t, authorityKey),
	}
}

func TestUpgradeExchangeAuthenticatesExactSummaryAndDownloadBearer(t *testing.T) {
	t.Parallel()

	fixture := newUpgradeExchangeFixture(t)
	encoded, err := core.EncodeValidatedJSON(fixture.grantProjection, core.DefaultStrictJSONLimits())
	if err != nil || len(encoded) == 0 {
		t.Fatalf("core.EncodeValidatedJSON(UpgradeGrantProjection) = (%d bytes, %v), want non-empty receive-only projection and nil", len(encoded), err)
	}
	verified, err := distribution.VerifyUpgradeGrant(distribution.UpgradeGrantExpectation{
		Request: fixture.request, Document: fixture.grantDoc,
		TrustedKeys: fixture.authorityKeys, ObservedAt: temporal.InstantFromNanoseconds(3_000),
	})
	if err != nil {
		t.Fatalf("distribution.VerifyUpgradeGrant() error = %v, want nil", err)
	}
	request, err := verified.Request()
	if err != nil || request != fixture.request || request.Available.ManifestDocument != fixture.request.Available.ManifestDocument {
		t.Fatalf("VerifiedUpgradeGrant.Request() = (%v, %v), want byte-exact available release", request, err)
	}
	if _, err := distribution.PrepareUpgradeStage(distribution.UpgradeStageRequest{
		Grant: verified,
	}); !errors.Is(err, core.ErrDistributionContract) {
		t.Fatalf("distribution.PrepareUpgradeStage(without local effects) error = %v, want %v", err, core.ErrDistributionContract)
	}
}

func TestVerifyUpgradeGrantPressuresCommitmentTrustAndLifetimeEdges(t *testing.T) {
	t.Parallel()

	fixture := newUpgradeExchangeFixture(t)
	base := distribution.UpgradeGrantExpectation{
		Request: fixture.request, Document: fixture.grantDoc,
		TrustedKeys: fixture.authorityKeys, ObservedAt: temporal.InstantFromNanoseconds(3_000),
	}
	otherRequest := fixture.request
	otherRequest.Nonce = requestNonce(t, 63)
	otherKeys := trustedKeys(t, signingKey(151))
	otherProjection, otherCapability := downloadCapabilityProjection(t, 8)
	otherCommitment, err := otherProjection.Commitment()
	if err != nil {
		t.Fatalf("other DownloadCapabilityProjection.Commitment() error = %v, want nil", err)
	}
	capabilitySwap := fixture.grantDoc
	capabilitySwap.Capability = otherCapability
	signedBodyMutation := capabilitySwap
	signedBodyMutation.Payload.Capability = otherCommitment

	cases := []struct {
		wantErr error
		mutate  func(distribution.UpgradeGrantExpectation) distribution.UpgradeGrantExpectation
		name    string
	}{
		{name: "issued-at boundary is accepted", mutate: func(v distribution.UpgradeGrantExpectation) distribution.UpgradeGrantExpectation {
			v.ObservedAt = temporal.InstantFromNanoseconds(2_500)
			return v
		}},
		{name: "one nanosecond before expiry is accepted", mutate: func(v distribution.UpgradeGrantExpectation) distribution.UpgradeGrantExpectation {
			v.ObservedAt = temporal.InstantFromNanoseconds(3_499)
			return v
		}},
		{name: "one nanosecond before issue is rejected", mutate: func(v distribution.UpgradeGrantExpectation) distribution.UpgradeGrantExpectation {
			v.ObservedAt = temporal.InstantFromNanoseconds(2_499)
			return v
		}, wantErr: core.ErrDistributionBinding},
		{name: "exact expiry is rejected", mutate: func(v distribution.UpgradeGrantExpectation) distribution.UpgradeGrantExpectation {
			v.ObservedAt = temporal.InstantFromNanoseconds(3_500)
			return v
		}, wantErr: core.ErrDistributionBinding},
		{name: "one nanosecond after expiry is rejected", mutate: func(v distribution.UpgradeGrantExpectation) distribution.UpgradeGrantExpectation {
			v.ObservedAt = temporal.InstantFromNanoseconds(3_501)
			return v
		}, wantErr: core.ErrDistributionBinding},
		{name: "different request nonce is rejected", mutate: func(v distribution.UpgradeGrantExpectation) distribution.UpgradeGrantExpectation {
			v.Request = otherRequest
			return v
		}, wantErr: core.ErrDistributionBinding},
		{name: "bearer swapped without signed commitment is rejected structurally", mutate: func(v distribution.UpgradeGrantExpectation) distribution.UpgradeGrantExpectation {
			v.Document = capabilitySwap
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "bearer and commitment mutation reaches signature refusal", mutate: func(v distribution.UpgradeGrantExpectation) distribution.UpgradeGrantExpectation {
			v.Document = signedBodyMutation
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "untrusted authority signer is rejected", mutate: func(v distribution.UpgradeGrantExpectation) distribution.UpgradeGrantExpectation {
			v.TrustedKeys = otherKeys
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "zero observation is rejected", mutate: func(v distribution.UpgradeGrantExpectation) distribution.UpgradeGrantExpectation {
			v.ObservedAt = temporal.Instant{}
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "unset document is rejected", mutate: func(v distribution.UpgradeGrantExpectation) distribution.UpgradeGrantExpectation {
			v.Document = distribution.UpgradeGrantDocument{}
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "unset request is rejected", mutate: func(v distribution.UpgradeGrantExpectation) distribution.UpgradeGrantExpectation {
			v.Request = distribution.UpgradeRequestPayload{}
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "unset trust is rejected", mutate: func(v distribution.UpgradeGrantExpectation) distribution.UpgradeGrantExpectation {
			v.TrustedKeys = attest.TrustedKeys{}
			return v
		}, wantErr: core.ErrDistributionVerification},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := distribution.VerifyUpgradeGrant(tc.mutate(base))
			if tc.wantErr == nil {
				if gotErr != nil || got.Validate() != nil {
					t.Fatalf("distribution.VerifyUpgradeGrant(boundary) = (%v, %v), want valid proof", got, gotErr)
				}
				return
			}
			if !errors.Is(gotErr, tc.wantErr) || !errors.Is(gotErr, core.ErrDistributionContract) {
				t.Fatalf("distribution.VerifyUpgradeGrant(boundary) error = %v, want %v and %v", gotErr, tc.wantErr, core.ErrDistributionContract)
			}
			if err := got.Validate(); !errors.Is(err, core.ErrDistributionVerification) {
				t.Fatalf("rejected distribution.VerifiedUpgradeGrant.Validate() error = %v, want %v", err, core.ErrDistributionVerification)
			}
		})
	}
}
