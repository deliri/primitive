package distribution_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/distribution"
)

func TestVerifyPublicationRequestRequiresCallerManifestAndOfferingClosure(t *testing.T) {
	t.Parallel()

	fixture := newPublicationExchangeFixture(t)
	otherRelease := newReleaseFixture(t, core.NewReleaseVersion(2026, 0, 56), 3)
	otherKeys := trustedKeys(t, signingKey(151))
	otherNonce := requestNonce(t, 93)
	base := distribution.PublicationRequestVerification{
		Document: fixture.requestDocument, RequestKeys: fixture.callerKeys,
		ManifestKeys: fixture.releaseKeys, ExpectedOffering: core.OfferingBug,
	}
	cases := []struct {
		wantErr error
		mutate  func(distribution.PublicationRequestVerification) distribution.PublicationRequestVerification
		name    string
	}{
		{name: "exact caller manifest and offering are accepted", mutate: func(v distribution.PublicationRequestVerification) distribution.PublicationRequestVerification {
			return v
		}},
		{name: "untrusted caller signer is rejected", mutate: func(v distribution.PublicationRequestVerification) distribution.PublicationRequestVerification {
			v.RequestKeys = otherKeys
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "untrusted nested manifest signer is rejected", mutate: func(v distribution.PublicationRequestVerification) distribution.PublicationRequestVerification {
			v.ManifestKeys = otherKeys
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "different expected offering is rejected", mutate: func(v distribution.PublicationRequestVerification) distribution.PublicationRequestVerification {
			v.ExpectedOffering = core.OfferingWitness
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "valid alternate signed manifest invalidates caller signature", mutate: func(v distribution.PublicationRequestVerification) distribution.PublicationRequestVerification {
			v.Document.Payload.Manifest = otherRelease.manifest.Document()
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "different valid nonce invalidates caller signature", mutate: func(v distribution.PublicationRequestVerification) distribution.PublicationRequestVerification {
			v.Document.Payload.Nonce = otherNonce
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "update-domain envelope cannot authenticate publication", mutate: func(v distribution.PublicationRequestVerification) distribution.PublicationRequestVerification {
			v.Document.Attestation.Domain = distribution.SigningDomainUpdateRequestV1
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "unset request trust is rejected", mutate: func(v distribution.PublicationRequestVerification) distribution.PublicationRequestVerification {
			v.RequestKeys = attest.TrustedKeys{}
			return v
		}, wantErr: core.ErrDistributionVerification},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := distribution.VerifyPublicationRequest(tc.mutate(base))
			if tc.wantErr == nil {
				if gotErr != nil || got.Validate() != nil {
					t.Fatalf("distribution.VerifyPublicationRequest(exact) = (%v, %v), want valid proof", got, gotErr)
				}
				return
			}
			if !errors.Is(gotErr, tc.wantErr) || !errors.Is(gotErr, core.ErrDistributionContract) {
				t.Fatalf("distribution.VerifyPublicationRequest(hostile) error = %v, want %v and %v", gotErr, tc.wantErr, core.ErrDistributionContract)
			}
			if err := got.Validate(); !errors.Is(err, core.ErrDistributionVerification) {
				t.Fatalf("rejected distribution.VerifiedPublicationRequest.Validate() error = %v, want %v", err, core.ErrDistributionVerification)
			}
		})
	}
}

func TestVerifyUpdateRequestRequiresExactCallerSignedBuild(t *testing.T) {
	t.Parallel()

	fixture := newUpdateExchangeFixture(t)
	otherRelease := newReleaseFixture(t, core.NewReleaseVersion(2026, 0, 53), 3)
	otherKeys := trustedKeys(t, signingKey(151))
	otherNonce := requestNonce(t, 94)
	base := distribution.UpdateRequestVerification{
		Document: fixture.requestDoc, TrustedKeys: fixture.callerKeys,
	}
	cases := []struct {
		wantErr error
		mutate  func(distribution.UpdateRequestVerification) distribution.UpdateRequestVerification
		name    string
	}{
		{name: "exact caller-signed installed build is accepted", mutate: func(v distribution.UpdateRequestVerification) distribution.UpdateRequestVerification { return v }},
		{name: "untrusted caller signer is rejected", mutate: func(v distribution.UpdateRequestVerification) distribution.UpdateRequestVerification {
			v.TrustedKeys = otherKeys
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "different valid installed build invalidates signature", mutate: func(v distribution.UpdateRequestVerification) distribution.UpdateRequestVerification {
			v.Document.Payload.Build = otherRelease.builds[2]
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "different valid nonce invalidates signature", mutate: func(v distribution.UpdateRequestVerification) distribution.UpdateRequestVerification {
			v.Document.Payload.Nonce = otherNonce
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "upgrade-domain envelope cannot authenticate update", mutate: func(v distribution.UpdateRequestVerification) distribution.UpdateRequestVerification {
			v.Document.Attestation.Domain = distribution.SigningDomainUpgradeRequestV1
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "unset caller trust is rejected", mutate: func(v distribution.UpdateRequestVerification) distribution.UpdateRequestVerification {
			v.TrustedKeys = attest.TrustedKeys{}
			return v
		}, wantErr: core.ErrDistributionVerification},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := distribution.VerifyUpdateRequest(tc.mutate(base))
			if tc.wantErr == nil {
				if gotErr != nil || got.Validate() != nil {
					t.Fatalf("distribution.VerifyUpdateRequest(exact) = (%v, %v), want valid proof", got, gotErr)
				}
				return
			}
			if !errors.Is(gotErr, tc.wantErr) || !errors.Is(gotErr, core.ErrDistributionContract) {
				t.Fatalf("distribution.VerifyUpdateRequest(hostile) error = %v, want %v and %v", gotErr, tc.wantErr, core.ErrDistributionContract)
			}
			if err := got.Validate(); !errors.Is(err, core.ErrDistributionVerification) {
				t.Fatalf("rejected distribution.VerifiedUpdateRequest.Validate() error = %v, want %v", err, core.ErrDistributionVerification)
			}
		})
	}
}

func TestVerifyUpgradeRequestRequiresExactCallerSignedCandidateClosure(t *testing.T) {
	t.Parallel()

	fixture := newUpgradeExchangeFixture(t)
	installed := newReleaseFixture(t, core.NewReleaseVersion(2026, 0, 54), 1)
	otherCandidate := newReleaseFixture(t, core.NewReleaseVersion(2026, 0, 56), 3)
	otherSummary := availableSummaryFixture(t, installed, otherCandidate)
	otherKeys := trustedKeys(t, signingKey(151))
	otherNonce := requestNonce(t, 95)
	base := distribution.UpgradeRequestVerification{
		Document: fixture.requestDoc, TrustedKeys: fixture.callerKeys,
	}
	cases := []struct {
		wantErr error
		mutate  func(distribution.UpgradeRequestVerification) distribution.UpgradeRequestVerification
		name    string
	}{
		{name: "exact caller-signed candidate closure is accepted", mutate: func(v distribution.UpgradeRequestVerification) distribution.UpgradeRequestVerification { return v }},
		{name: "untrusted caller signer is rejected", mutate: func(v distribution.UpgradeRequestVerification) distribution.UpgradeRequestVerification {
			v.TrustedKeys = otherKeys
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "different valid candidate closure invalidates signature", mutate: func(v distribution.UpgradeRequestVerification) distribution.UpgradeRequestVerification {
			v.Document.Payload.Available = otherSummary
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "different valid nonce invalidates signature", mutate: func(v distribution.UpgradeRequestVerification) distribution.UpgradeRequestVerification {
			v.Document.Payload.Nonce = otherNonce
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "publication-domain envelope cannot authenticate upgrade", mutate: func(v distribution.UpgradeRequestVerification) distribution.UpgradeRequestVerification {
			v.Document.Attestation.Domain = distribution.SigningDomainPublicationRequestV1
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "unset caller trust is rejected", mutate: func(v distribution.UpgradeRequestVerification) distribution.UpgradeRequestVerification {
			v.TrustedKeys = attest.TrustedKeys{}
			return v
		}, wantErr: core.ErrDistributionVerification},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := distribution.VerifyUpgradeRequest(tc.mutate(base))
			if tc.wantErr == nil {
				if gotErr != nil || got.Validate() != nil {
					t.Fatalf("distribution.VerifyUpgradeRequest(exact) = (%v, %v), want valid proof", got, gotErr)
				}
				return
			}
			if !errors.Is(gotErr, tc.wantErr) || !errors.Is(gotErr, core.ErrDistributionContract) {
				t.Fatalf("distribution.VerifyUpgradeRequest(hostile) error = %v, want %v and %v", gotErr, tc.wantErr, core.ErrDistributionContract)
			}
			if err := got.Validate(); !errors.Is(err, core.ErrDistributionVerification) {
				t.Fatalf("rejected distribution.VerifiedUpgradeRequest.Validate() error = %v, want %v", err, core.ErrDistributionVerification)
			}
		})
	}
}
