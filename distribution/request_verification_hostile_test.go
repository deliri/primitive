package distribution_test

import (
	"crypto/ed25519"
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
	secondNonce := requestNonce(t, 96)
	otherSigner, err := core.NewEd25519PublicKey(signingKey(151).Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatalf("core.NewEd25519PublicKey() error = %v, want nil", err)
	}
	oneByte, err := core.NewByteCount(1)
	if err != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
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
		{name: "different valid Darwin AMD64 build invalidates signature", mutate: func(v distribution.UpdateRequestVerification) distribution.UpdateRequestVerification {
			v.Document.Payload.Build = otherRelease.builds[0]
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "different valid Darwin ARM64 build invalidates signature", mutate: func(v distribution.UpdateRequestVerification) distribution.UpdateRequestVerification {
			v.Document.Payload.Build = otherRelease.builds[1]
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "different valid nonce invalidates signature", mutate: func(v distribution.UpdateRequestVerification) distribution.UpdateRequestVerification {
			v.Document.Payload.Nonce = otherNonce
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "second different valid nonce invalidates signature", mutate: func(v distribution.UpdateRequestVerification) distribution.UpdateRequestVerification {
			v.Document.Payload.Nonce = secondNonce
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "upgrade-domain envelope cannot authenticate update", mutate: func(v distribution.UpdateRequestVerification) distribution.UpdateRequestVerification {
			v.Document.Attestation.Domain = distribution.SigningDomainUpgradeRequestV1
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "publication-domain envelope cannot authenticate update", mutate: func(v distribution.UpdateRequestVerification) distribution.UpdateRequestVerification {
			v.Document.Attestation.Domain = distribution.SigningDomainPublicationRequestV1
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "unset caller trust is rejected", mutate: func(v distribution.UpdateRequestVerification) distribution.UpdateRequestVerification {
			v.TrustedKeys = attest.TrustedKeys{}
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "substituted envelope signer is rejected", mutate: func(v distribution.UpdateRequestVerification) distribution.UpdateRequestVerification {
			v.Document.Attestation.Signer = otherSigner
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "substituted envelope body digest is rejected", mutate: func(v distribution.UpdateRequestVerification) distribution.UpdateRequestVerification {
			v.Document.Attestation.BodySHA256 = core.SHA256Of([]byte("another update request"))
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "substituted envelope body length is rejected", mutate: func(v distribution.UpdateRequestVerification) distribution.UpdateRequestVerification {
			v.Document.Attestation.BodyLength = oneByte
			return v
		}, wantErr: core.ErrDistributionVerification},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			verification := tc.mutate(base)
			if tc.wantErr != nil && verification == base {
				t.Fatalf("update mutation %q left verification unchanged: %v", tc.name, verification)
			}
			got, gotErr := distribution.VerifyUpdateRequest(verification)
			if tc.wantErr == nil {
				payload, payloadErr := got.Payload()
				if gotErr != nil || got.Validate() != nil || payloadErr != nil || payload != base.Document.Payload {
					t.Fatalf("distribution.VerifyUpdateRequest(exact) = (%v, %v), payload (%v, %v), want exact valid proof",
						got, gotErr, payload, payloadErr)
				}
				return
			}
			if !errors.Is(gotErr, tc.wantErr) || !errors.Is(gotErr, core.ErrDistributionContract) {
				t.Fatalf("distribution.VerifyUpdateRequest(hostile) error = %v, want %v and %v", gotErr, tc.wantErr, core.ErrDistributionContract)
			}
			if err := got.Validate(); !errors.Is(err, core.ErrDistributionVerification) {
				t.Fatalf("rejected distribution.VerifiedUpdateRequest.Validate() error = %v, want %v", err, core.ErrDistributionVerification)
			}
			if payload, err := got.Payload(); payload != (distribution.UpdateRequestPayload{}) ||
				!errors.Is(err, core.ErrDistributionVerification) {
				t.Fatalf("rejected distribution.VerifiedUpdateRequest.Payload() = (%v, %v), want zero and %v",
					payload, err, core.ErrDistributionVerification)
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
	secondCandidate := newReleaseFixture(t, core.NewReleaseVersion(2026, 0, 57), 4)
	secondSummary := availableSummaryFixture(t, installed, secondCandidate)
	otherInstalled := newReleaseFixture(t, core.NewReleaseVersion(2026, 0, 53), 1)
	thirdSummary := availableSummaryFixture(t, otherInstalled, otherCandidate)
	otherKeys := trustedKeys(t, signingKey(151))
	otherNonce := requestNonce(t, 95)
	secondNonce := requestNonce(t, 97)
	otherSigner, err := core.NewEd25519PublicKey(signingKey(151).Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatalf("core.NewEd25519PublicKey() error = %v, want nil", err)
	}
	oneByte, err := core.NewByteCount(1)
	if err != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
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
		{name: "second valid candidate closure invalidates signature", mutate: func(v distribution.UpgradeRequestVerification) distribution.UpgradeRequestVerification {
			v.Document.Payload.Available = secondSummary
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "different valid installed closure invalidates signature", mutate: func(v distribution.UpgradeRequestVerification) distribution.UpgradeRequestVerification {
			v.Document.Payload.Available = thirdSummary
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "different valid nonce invalidates signature", mutate: func(v distribution.UpgradeRequestVerification) distribution.UpgradeRequestVerification {
			v.Document.Payload.Nonce = otherNonce
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "second different valid nonce invalidates signature", mutate: func(v distribution.UpgradeRequestVerification) distribution.UpgradeRequestVerification {
			v.Document.Payload.Nonce = secondNonce
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "publication-domain envelope cannot authenticate upgrade", mutate: func(v distribution.UpgradeRequestVerification) distribution.UpgradeRequestVerification {
			v.Document.Attestation.Domain = distribution.SigningDomainPublicationRequestV1
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "update-domain envelope cannot authenticate upgrade", mutate: func(v distribution.UpgradeRequestVerification) distribution.UpgradeRequestVerification {
			v.Document.Attestation.Domain = distribution.SigningDomainUpdateRequestV1
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "unset caller trust is rejected", mutate: func(v distribution.UpgradeRequestVerification) distribution.UpgradeRequestVerification {
			v.TrustedKeys = attest.TrustedKeys{}
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "substituted envelope signer is rejected", mutate: func(v distribution.UpgradeRequestVerification) distribution.UpgradeRequestVerification {
			v.Document.Attestation.Signer = otherSigner
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "substituted envelope body digest is rejected", mutate: func(v distribution.UpgradeRequestVerification) distribution.UpgradeRequestVerification {
			v.Document.Attestation.BodySHA256 = core.SHA256Of([]byte("another upgrade request"))
			return v
		}, wantErr: core.ErrDistributionVerification},
		{name: "substituted envelope body length is rejected", mutate: func(v distribution.UpgradeRequestVerification) distribution.UpgradeRequestVerification {
			v.Document.Attestation.BodyLength = oneByte
			return v
		}, wantErr: core.ErrDistributionVerification},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			verification := tc.mutate(base)
			if tc.wantErr != nil && verification == base {
				t.Fatalf("upgrade mutation %q left verification unchanged: %v", tc.name, verification)
			}
			got, gotErr := distribution.VerifyUpgradeRequest(verification)
			if tc.wantErr == nil {
				payload, payloadErr := got.Payload()
				if gotErr != nil || got.Validate() != nil || payloadErr != nil || payload != base.Document.Payload {
					t.Fatalf("distribution.VerifyUpgradeRequest(exact) = (%v, %v), payload (%v, %v), want exact valid proof",
						got, gotErr, payload, payloadErr)
				}
				return
			}
			if !errors.Is(gotErr, tc.wantErr) || !errors.Is(gotErr, core.ErrDistributionContract) {
				t.Fatalf("distribution.VerifyUpgradeRequest(hostile) error = %v, want %v and %v", gotErr, tc.wantErr, core.ErrDistributionContract)
			}
			if err := got.Validate(); !errors.Is(err, core.ErrDistributionVerification) {
				t.Fatalf("rejected distribution.VerifiedUpgradeRequest.Validate() error = %v, want %v", err, core.ErrDistributionVerification)
			}
			if payload, err := got.Payload(); payload != (distribution.UpgradeRequestPayload{}) ||
				!errors.Is(err, core.ErrDistributionVerification) {
				t.Fatalf("rejected distribution.VerifiedUpgradeRequest.Payload() = (%v, %v), want zero and %v",
					payload, err, core.ErrDistributionVerification)
			}
		})
	}
}
