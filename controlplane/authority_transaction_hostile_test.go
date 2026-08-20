package controlplane_test

import (
	"crypto/ed25519"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
	"github.com/deliri/primitive/v2026/receipt"
	"github.com/deliri/primitive/v2026/temporal"
)

// TestRegistrationAuthorityTransactionLayerTriad proves the authority consumes
// only the one-way verifier, admits a byte-identical retry, and refuses a
// second request that tries to spend the same token on different facts.
func TestRegistrationAuthorityTransactionLayerTriad(t *testing.T) {
	t.Parallel()

	request := registrationRequestFixture(t)
	verifier, err := request.Token.Verifier()
	if err != nil {
		t.Fatalf("RegistrationToken.Verifier() error = %v, want nil", err)
	}
	canonical, err := request.MarshalJSON()
	if err != nil {
		t.Fatalf("RegistrationRequest.MarshalJSON() error = %v, want nil", err)
	}

	fresh, err := controlplane.VerifyRegistrationAuthority(controlplane.RegistrationAuthorityVerification{
		Request: request, ExpectedVerifier: verifier,
	})
	if err != nil {
		t.Fatalf("VerifyRegistrationAuthority(fresh) error = %v, want nil", err)
	}
	replay, disposition, err := fresh.Replay()
	if err != nil || disposition != controlwire.ReplayDispositionFresh {
		t.Fatalf("fresh Replay() = (%v, %v, %v), want validated replay, %v, nil", replay, disposition, err, controlwire.ReplayDispositionFresh)
	}
	if err := request.Token.Validate(); !errors.Is(err, core.ErrControlWireToken) {
		t.Fatalf("consumed request token Validate() error = %v, want %v", err, core.ErrControlWireToken)
	}

	var retry controlplane.RegistrationRequest
	if err := retry.UnmarshalJSON(canonical); err != nil {
		t.Fatalf("RegistrationRequest.UnmarshalJSON(retry) error = %v, want nil", err)
	}
	exact, err := controlplane.VerifyRegistrationAuthority(controlplane.RegistrationAuthorityVerification{
		Request: retry, ExpectedVerifier: verifier, PriorReplay: &replay,
	})
	if err != nil {
		t.Fatalf("VerifyRegistrationAuthority(exact retry) error = %v, want nil", err)
	}
	_, disposition, err = exact.Replay()
	if err != nil || disposition != controlwire.ReplayDispositionExact {
		t.Fatalf("exact Replay() = (%v, %v), want (%v, nil)", disposition, err, controlwire.ReplayDispositionExact)
	}

	var changed controlplane.RegistrationRequest
	if err := changed.UnmarshalJSON(canonical); err != nil {
		t.Fatalf("RegistrationRequest.UnmarshalJSON(changed retry) error = %v, want nil", err)
	}
	changed.RequestNonce = otherRequestNonce(t)
	refused, err := controlplane.VerifyRegistrationAuthority(controlplane.RegistrationAuthorityVerification{
		Request: changed, ExpectedVerifier: verifier, PriorReplay: &replay,
	})
	if !errors.Is(err, core.ErrControlWireReplayConflict) ||
		!errors.Is(err, core.ErrControlPlaneRegistration) {
		t.Fatalf("VerifyRegistrationAuthority(changed reuse) error = %v, want errors.Is(..., %v, %v)", err, core.ErrControlWireReplayConflict, core.ErrControlPlaneRegistration)
	}
	if validateErr := refused.Validate(); !errors.Is(validateErr, core.ErrControlPlaneRegistration) {
		t.Fatalf("refused VerifiedRegistrationAuthority.Validate() error = %v, want %v", validateErr, core.ErrControlPlaneRegistration)
	}
}

// TestRegistrationAuthorityAcceptsTheClosedOfferingAndReplayMatrix exercises
// twenty successful authority transactions: fresh and exact for ten distinct
// compiler-built requests spanning every offering and multiple nonce, token,
// and build identities.
func TestRegistrationAuthorityAcceptsTheClosedOfferingAndReplayMatrix(t *testing.T) {
	t.Parallel()

	offerings := validOfferings(t)
	for index := range 10 {
		offering := offerings[index%len(offerings)]
		t.Run(offering.String()+" fresh and exact request "+string(rune('A'+index)), func(t *testing.T) {
			t.Parallel()

			request := registrationRequestVariant(t, offering, byte(index+1))
			verifier, err := request.Token.Verifier()
			if err != nil {
				t.Fatalf("RegistrationToken.Verifier() error = %v, want nil", err)
			}
			canonical, err := request.MarshalJSON()
			if err != nil {
				t.Fatalf("RegistrationRequest.MarshalJSON() error = %v, want nil", err)
			}
			fresh, err := controlplane.VerifyRegistrationAuthority(controlplane.RegistrationAuthorityVerification{
				Request: request, ExpectedVerifier: verifier,
			})
			if err != nil {
				t.Fatalf("VerifyRegistrationAuthority(fresh) error = %v, want nil", err)
			}
			replay, disposition, err := fresh.Replay()
			if err != nil || disposition != controlwire.ReplayDispositionFresh {
				t.Fatalf("fresh Replay() = (%v, %v, %v), want validated replay, %v, nil", replay, disposition, err, controlwire.ReplayDispositionFresh)
			}
			var retry controlplane.RegistrationRequest
			if err := retry.UnmarshalJSON(canonical); err != nil {
				t.Fatalf("RegistrationRequest.UnmarshalJSON(retry) error = %v, want nil", err)
			}
			exact, err := controlplane.VerifyRegistrationAuthority(controlplane.RegistrationAuthorityVerification{
				Request: retry, ExpectedVerifier: verifier, PriorReplay: &replay,
			})
			if err != nil {
				t.Fatalf("VerifyRegistrationAuthority(exact) error = %v, want nil", err)
			}
			if _, disposition, err := exact.Replay(); err != nil || disposition != controlwire.ReplayDispositionExact {
				t.Fatalf("exact Replay() = (%v, %v), want (%v, nil)", disposition, err, controlwire.ReplayDispositionExact)
			}
			identity, err := exact.Identity()
			if err != nil || identity.Build.Offering() != offering || identity.RequestNonce != request.RequestNonce ||
				identity.DeviceKey != request.DeviceKey || identity.Installation != request.Installation {
				t.Fatalf("Identity() = (%+v, %v), want exact non-secret request facts", identity, err)
			}
		})
	}
}

// TestRegistrationAuthorityRefusesEveryInvalidOrSecondUse pressure-tests the
// verifier, request, and prior-commit boundaries. Every rejection returns a
// zero proof whose own validation fails with the registration identity.
func TestRegistrationAuthorityRefusesEveryInvalidOrSecondUse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		mutate func(*testing.T, *controlplane.RegistrationAuthorityVerification, *controlwire.ReplayIdentity)
		want   []error
	}{
		{name: "zero verification carries no authority", mutate: func(_ *testing.T, verification *controlplane.RegistrationAuthorityVerification, _ *controlwire.ReplayIdentity) {
			*verification = controlplane.RegistrationAuthorityVerification{}
		}, want: []error{core.ErrControlPlaneRegistration}},
		{name: "zero expected verifier cannot authenticate", mutate: func(_ *testing.T, verification *controlplane.RegistrationAuthorityVerification, _ *controlwire.ReplayIdentity) {
			verification.ExpectedVerifier = controlwire.RegistrationTokenVerifier{}
		}, want: []error{core.ErrControlPlaneRegistration, core.ErrControlWireToken}},
		{name: "zero prior replay cannot prove consumption", mutate: func(_ *testing.T, verification *controlplane.RegistrationAuthorityVerification, _ *controlwire.ReplayIdentity) {
			prior := controlwire.ReplayIdentity{}
			verification.PriorReplay = &prior
		}, want: []error{core.ErrControlPlaneRegistration, core.ErrControlWireContract}},
		{name: "different token does not match persisted verifier", mutate: func(t *testing.T, verification *controlplane.RegistrationAuthorityVerification, _ *controlwire.ReplayIdentity) {
			verification.Request = registrationRequestVariant(t, verification.Request.Build.Offering(), 31)
		}, want: []error{core.ErrControlPlaneRegistration, core.ErrControlWireToken}},
		{name: "second use with another nonce conflicts", mutate: func(t *testing.T, verification *controlplane.RegistrationAuthorityVerification, replay *controlwire.ReplayIdentity) {
			verification.PriorReplay = replay
			verification.Request.RequestNonce = otherRequestNonce(t)
		}, want: []error{core.ErrControlPlaneRegistration, core.ErrControlWireReplayConflict}},
		{name: "second use with another offering conflicts", mutate: func(t *testing.T, verification *controlplane.RegistrationAuthorityVerification, replay *controlwire.ReplayIdentity) {
			verification.PriorReplay = replay
			verification.Request.Build = testBuildForOffering(t, core.OfferingBug)
		}, want: []error{core.ErrControlPlaneRegistration, core.ErrControlWireReplayConflict}},
		{name: "second use with another build version conflicts", mutate: func(t *testing.T, verification *controlplane.RegistrationAuthorityVerification, replay *controlwire.ReplayIdentity) {
			verification.PriorReplay = replay
			verification.Request.Build = testBuildIdentity(t, 2026, 0, 99)
		}, want: []error{core.ErrControlPlaneRegistration, core.ErrControlWireReplayConflict}},
		{name: "second use with another device key and bound installation conflicts", mutate: func(t *testing.T, verification *controlplane.RegistrationAuthorityVerification, replay *controlwire.ReplayIdentity) {
			verification.PriorReplay = replay
			verification.Request.DeviceKey, verification.Request.Installation = testDeviceKey(t, checkInOtherDeviceSeed)
		}, want: []error{core.ErrControlPlaneRegistration, core.ErrControlWireReplayConflict}},
		{name: "second use with mismatched device and installation is invalid before replay", mutate: func(t *testing.T, verification *controlplane.RegistrationAuthorityVerification, replay *controlwire.ReplayIdentity) {
			verification.PriorReplay = replay
			verification.Request.DeviceKey, _ = testDeviceKey(t, checkInOtherDeviceSeed)
		}, want: []error{core.ErrControlPlaneRegistration, core.ErrControlPlaneInstallationBinding}},
		{name: "missing nonce is rejected before replay", mutate: func(_ *testing.T, verification *controlplane.RegistrationAuthorityVerification, replay *controlwire.ReplayIdentity) {
			verification.PriorReplay = replay
			verification.Request.RequestNonce = controlwire.RequestNonce{}
		}, want: []error{core.ErrControlPlaneRegistration, core.ErrControlWireNonce}},
		{name: "missing device key is rejected before replay", mutate: func(_ *testing.T, verification *controlplane.RegistrationAuthorityVerification, replay *controlwire.ReplayIdentity) {
			verification.PriorReplay = replay
			verification.Request.DeviceKey = core.Ed25519PublicKey{}
		}, want: []error{core.ErrControlPlaneRegistration}},
		{name: "missing build is rejected before replay", mutate: func(_ *testing.T, verification *controlplane.RegistrationAuthorityVerification, replay *controlwire.ReplayIdentity) {
			verification.PriorReplay = replay
			verification.Request.Build = core.BuildIdentity{}
		}, want: []error{core.ErrControlPlaneRegistration}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := registrationRequestFixture(t)
			verifier, err := request.Token.Verifier()
			if err != nil {
				t.Fatalf("RegistrationToken.Verifier() error = %v, want nil", err)
			}
			replay, err := controlwire.CommitReplayIdentity(request)
			if err != nil {
				t.Fatalf("CommitReplayIdentity() error = %v, want nil", err)
			}
			verification := controlplane.RegistrationAuthorityVerification{Request: request, ExpectedVerifier: verifier}
			tc.mutate(t, &verification, &replay)
			hadLiveToken := verification.Request.Token.Validate() == nil
			got, gotErr := controlplane.VerifyRegistrationAuthority(verification)
			for _, wantErr := range tc.want {
				if !errors.Is(gotErr, wantErr) {
					t.Fatalf("VerifyRegistrationAuthority() error = %v, want errors.Is %v", gotErr, wantErr)
				}
			}
			if validateErr := got.Validate(); !errors.Is(validateErr, core.ErrControlPlaneRegistration) {
				t.Fatalf("refused proof Validate() error = %v, want %v", validateErr, core.ErrControlPlaneRegistration)
			}
			if hadLiveToken {
				if tokenErr := verification.Request.Token.Validate(); !errors.Is(tokenErr, core.ErrControlWireToken) {
					t.Fatalf("refused request token Validate() error = %v, want destroyed %v", tokenErr, core.ErrControlWireToken)
				}
			}
		})
	}
}

// TestIssueRegisteredInstallationDerivesEveryCertificateIdentityFact proves
// the signer receives only a body derived from the authenticated registration;
// there is no caller-supplied build, offering, device, installation, or
// revision available to substitute.
func TestIssueRegisteredInstallationDerivesEveryCertificateIdentityFact(t *testing.T) {
	t.Parallel()

	request := registrationRequestFixture(t)
	verifier, err := request.Token.Verifier()
	if err != nil {
		t.Fatalf("RegistrationToken.Verifier() error = %v, want nil", err)
	}
	registration, err := controlplane.VerifyRegistrationAuthority(controlplane.RegistrationAuthorityVerification{
		Request: request, ExpectedVerifier: verifier,
	})
	if err != nil {
		t.Fatalf("VerifyRegistrationAuthority() error = %v, want nil", err)
	}
	existing := issueTestRegistration(t)
	existingBody := existing.document.Payload.Certificate.Body
	_, signer := testSigningKey(t, checkInAuthoritySeed)
	certificate, err := controlplane.IssueRegisteredInstallation(controlplane.RegistrationCertificateIssuance{
		Registration: registration, IssuedAt: existingBody.IssuedAt,
		Account: existingBody.Account, Entitlement: existingBody.Subject.EntitlementID,
	}, signer)
	if err != nil {
		t.Fatalf("IssueRegisteredInstallation() error = %v, want nil", err)
	}
	if certificate.Body.Build != request.Build || certificate.Body.DeviceKey != request.DeviceKey ||
		certificate.Body.Subject.DeviceID != request.Installation || certificate.Body.Revision != request.Revision {
		t.Fatalf("issued certificate body = %+v, want exact authenticated request identity", certificate.Body)
	}
	public, _ := testSigningKey(t, checkInAuthoritySeed)
	trusted, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{Keys: []core.Ed25519PublicKey{public}})
	if err != nil {
		t.Fatalf("NewTrustedKeys() error = %v, want nil", err)
	}
	verified, err := controlplane.VerifyInstallationCertificate(certificate, trusted)
	if err != nil || verified.Validate() != nil {
		t.Fatalf("VerifyInstallationCertificate(issued) = (%v, %v), want authenticated proof and nil", verified, err)
	}
}

// TestCheckInAuthorityCommitLayerTriad proves one authenticated window advances
// once, repeats idempotently, and conflicts neutrally against unrelated
// authoritative state.
func TestCheckInAuthorityCommitLayerTriad(t *testing.T) {
	t.Parallel()

	issued := issueTestCheckIn(t, core.OfferingPeachfuzz, testCheckInWindow())
	verified, err := controlplane.VerifyCheckIn(controlplane.CheckInVerification{
		Request: issued.request, TrustedKeys: issued.trusted,
	})
	if err != nil {
		t.Fatalf("VerifyCheckIn() error = %v, want nil", err)
	}
	base := controlplane.CheckInCommitRequest{
		CheckIn: verified, Current: issued.request.Payload.PreviousWatermark,
		RequiredPolicy: issued.request.Payload.AppliedPolicy,
	}

	accepted, err := controlplane.CommitCheckIn(base)
	if err != nil {
		t.Fatalf("CommitCheckIn(fresh) error = %v, want nil", err)
	}
	acceptedWatermark, err := accepted.Watermark()
	if err != nil {
		t.Fatalf("accepted Watermark() error = %v, want nil", err)
	}
	if disposition, err := accepted.Disposition(); err != nil || disposition != controlplane.UsageDispositionAccepted {
		t.Fatalf("accepted Disposition() = (%v, %v), want (%v, nil)", disposition, err, controlplane.UsageDispositionAccepted)
	}

	replayRequest := base
	replayRequest.Current = acceptedWatermark
	replayed, err := controlplane.CommitCheckIn(replayRequest)
	if err != nil {
		t.Fatalf("CommitCheckIn(exact replay) error = %v, want nil", err)
	}
	if disposition, err := replayed.Disposition(); err != nil || disposition != controlplane.UsageDispositionReplay {
		t.Fatalf("replay Disposition() = (%v, %v), want (%v, nil)", disposition, err, controlplane.UsageDispositionReplay)
	}

	other := issueTestCheckIn(t, core.OfferingBug, testCheckInWindow())
	conflictRequest := base
	conflictRequest.Current = other.request.Payload.PreviousWatermark
	conflicted, err := controlplane.CommitCheckIn(conflictRequest)
	if !errors.Is(err, core.ErrControlPlaneDecisionConsistency) ||
		!errors.Is(conflicted.Validate(), core.ErrControlPlaneCheckIn) {
		t.Fatalf("CommitCheckIn(foreign subject) = (%v, %v), want zero proof and errors.Is(..., %v)", conflicted, err, core.ErrControlPlaneDecisionConsistency)
	}
}

// TestCheckInAuthorityCommitExhaustsOfferingAndDispositionMatrix proves every
// offering follows the same accepted/replay/conflict transition and that a
// conflict preserves the authority's current watermark byte-for-byte.
func TestCheckInAuthorityCommitExhaustsOfferingAndDispositionMatrix(t *testing.T) {
	t.Parallel()

	for _, offering := range validOfferings(t) {
		t.Run(offering.String(), func(t *testing.T) {
			t.Parallel()

			issued := issueTestCheckIn(t, offering, testCheckInWindow())
			verified, err := controlplane.VerifyCheckIn(controlplane.CheckInVerification{Request: issued.request, TrustedKeys: issued.trusted})
			if err != nil {
				t.Fatalf("VerifyCheckIn() error = %v, want nil", err)
			}
			base := controlplane.CheckInCommitRequest{CheckIn: verified, Current: issued.request.Payload.PreviousWatermark, RequiredPolicy: issued.request.Payload.AppliedPolicy}
			accepted, err := controlplane.CommitCheckIn(base)
			if err != nil {
				t.Fatalf("CommitCheckIn(accepted) error = %v, want nil", err)
			}
			next, err := accepted.Watermark()
			if err != nil {
				t.Fatalf("accepted Watermark() error = %v, want nil", err)
			}
			replayed := base
			replayed.Current = next
			gotReplay, err := controlplane.CommitCheckIn(replayed)
			if err != nil {
				t.Fatalf("CommitCheckIn(replay) error = %v, want nil", err)
			}
			if got, err := gotReplay.Disposition(); err != nil || got != controlplane.UsageDispositionReplay {
				t.Fatalf("replay Disposition() = (%v, %v), want (%v, nil)", got, err, controlplane.UsageDispositionReplay)
			}

			foreignNext, err := controlplane.AdvanceUsageWatermark(
				base.Current,
				testWindow(unitsOf(1, 3), outcomesOf(1, 3)),
			)
			if err != nil {
				t.Fatalf("AdvanceUsageWatermark(conflict fixture) error = %v, want nil", err)
			}
			conflicting := base
			conflicting.Current = foreignNext
			gotConflict, err := controlplane.CommitCheckIn(conflicting)
			if err != nil {
				t.Fatalf("CommitCheckIn(conflict) error = %v, want nil", err)
			}
			if got, err := gotConflict.Disposition(); err != nil || got != controlplane.UsageDispositionConflict {
				t.Fatalf("conflict Disposition() = (%v, %v), want (%v, nil)", got, err, controlplane.UsageDispositionConflict)
			}
			if got, err := gotConflict.Watermark(); err != nil || got != foreignNext {
				t.Fatalf("conflict Watermark() = (%v, %v), want authoritative %v and nil", got, err, foreignNext)
			}
		})
	}
}

// TestCheckInAuthorityCommitRefusesInvalidPolicyAndAuthorityFacts pins the
// neutral failure boundary: no proof, disposition, or watermark escapes.
func TestCheckInAuthorityCommitRefusesInvalidPolicyAndAuthorityFacts(t *testing.T) {
	t.Parallel()

	issued := issueTestCheckIn(t, core.OfferingWitness, testCheckInWindow())
	verified, err := controlplane.VerifyCheckIn(controlplane.CheckInVerification{Request: issued.request, TrustedKeys: issued.trusted})
	if err != nil {
		t.Fatalf("VerifyCheckIn() error = %v, want nil", err)
	}
	base := controlplane.CheckInCommitRequest{CheckIn: verified, Current: issued.request.Payload.PreviousWatermark, RequiredPolicy: issued.request.Payload.AppliedPolicy}
	otherPolicy := base.RequiredPolicy
	activation, err := controlwire.NewPolicyActivation(otherPolicy.Activation.Uint64() + 1)
	if err != nil {
		t.Fatalf("NewPolicyActivation() error = %v, want nil", err)
	}
	otherPolicy.Activation = activation
	foreign := issueTestCheckIn(t, core.OfferingBug, testCheckInWindow())
	zeroRevisionPolicy := base.RequiredPolicy
	zeroRevisionPolicy.Revision = controlwire.PolicyRevisionID{}
	zeroActivationPolicy := base.RequiredPolicy
	zeroActivationPolicy.Activation = 0
	zeroSubjectCurrent := base.Current
	zeroSubjectCurrent.Subject = lease.Subject{}
	zeroGenerationCurrent := base.Current
	zeroGenerationCurrent.Generation = lease.Generation{}
	zeroWindowDigestCurrent := base.Current
	zeroWindowDigestCurrent.WindowDigest = core.SHA256Digest{}
	zeroChainDigestCurrent := base.Current
	zeroChainDigestCurrent.ChainDigest = core.SHA256Digest{}
	cases := []struct {
		want    error
		name    string
		request controlplane.CheckInCommitRequest
	}{
		{name: "zero request has no authenticated check-in", request: controlplane.CheckInCommitRequest{}, want: core.ErrControlPlaneCheckIn},
		{name: "zero verified proof cannot authorize usage", request: controlplane.CheckInCommitRequest{Current: base.Current, RequiredPolicy: base.RequiredPolicy}, want: core.ErrControlPlaneCheckIn},
		{name: "zero authoritative watermark cannot be advanced", request: controlplane.CheckInCommitRequest{CheckIn: verified, RequiredPolicy: base.RequiredPolicy}, want: core.ErrControlPlaneUsageWatermark},
		{name: "zero required policy cannot authorize usage", request: controlplane.CheckInCommitRequest{CheckIn: verified, Current: base.Current}, want: core.ErrControlWirePolicyCursor},
		{name: "required policy without revision is not a cursor", request: controlplane.CheckInCommitRequest{CheckIn: verified, Current: base.Current, RequiredPolicy: zeroRevisionPolicy}, want: core.ErrControlWirePolicyCursor},
		{name: "required policy without activation is not a cursor", request: controlplane.CheckInCommitRequest{CheckIn: verified, Current: base.Current, RequiredPolicy: zeroActivationPolicy}, want: core.ErrControlWirePolicyCursor},
		{name: "different required policy refuses stale applied facts", request: controlplane.CheckInCommitRequest{CheckIn: verified, Current: base.Current, RequiredPolicy: otherPolicy}, want: core.ErrControlWirePolicyCursor},
		{name: "foreign authoritative subject cannot disclose or advance", request: controlplane.CheckInCommitRequest{CheckIn: verified, Current: foreign.request.Payload.PreviousWatermark, RequiredPolicy: base.RequiredPolicy}, want: core.ErrControlPlaneDecisionConsistency},
		{name: "authoritative watermark without subject cannot advance", request: controlplane.CheckInCommitRequest{CheckIn: verified, Current: zeroSubjectCurrent, RequiredPolicy: base.RequiredPolicy}, want: core.ErrControlPlaneUsageWatermark},
		{name: "authoritative watermark without generation cannot advance", request: controlplane.CheckInCommitRequest{CheckIn: verified, Current: zeroGenerationCurrent, RequiredPolicy: base.RequiredPolicy}, want: core.ErrControlPlaneUsageWatermark},
		{name: "authoritative watermark without window digest cannot advance", request: controlplane.CheckInCommitRequest{CheckIn: verified, Current: zeroWindowDigestCurrent, RequiredPolicy: base.RequiredPolicy}, want: core.ErrControlPlaneUsageWatermark},
		{name: "authoritative watermark without chain digest cannot advance", request: controlplane.CheckInCommitRequest{CheckIn: verified, Current: zeroChainDigestCurrent, RequiredPolicy: base.RequiredPolicy}, want: core.ErrControlPlaneUsageWatermark},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := controlplane.CommitCheckIn(tc.request)
			if !errors.Is(gotErr, tc.want) || !errors.Is(gotErr, core.ErrControlPlaneCheckIn) {
				t.Fatalf("CommitCheckIn() error = %v, want errors.Is(..., %v, %v)", gotErr, tc.want, core.ErrControlPlaneCheckIn)
			}
			if disposition, err := got.Disposition(); disposition != controlplane.UsageDispositionUnknown || !errors.Is(err, core.ErrControlPlaneCheckIn) {
				t.Fatalf("refused Disposition() = (%v, %v), want zero and %v", disposition, err, core.ErrControlPlaneCheckIn)
			}
			if watermark, err := got.Watermark(); watermark != (controlplane.UsageWatermark{}) || !errors.Is(err, core.ErrControlPlaneCheckIn) {
				t.Fatalf("refused Watermark() = (%v, %v), want zero and %v", watermark, err, core.ErrControlPlaneCheckIn)
			}
		})
	}
}

// TestIssueCommittedCheckInResponseBindsUsagePolicyProviderTimeAndRequest
// exercises the complete authority path from device authentication through
// usage commitment, response signing, and client verification.
func TestIssueCommittedCheckInResponseBindsUsagePolicyProviderTimeAndRequest(t *testing.T) {
	t.Parallel()

	issued := issueTestCheckIn(t, core.OfferingPeachfuzz, testCheckInWindow())
	verified, err := controlplane.VerifyCheckIn(controlplane.CheckInVerification{Request: issued.request, TrustedKeys: issued.trusted})
	if err != nil {
		t.Fatalf("VerifyCheckIn() error = %v, want nil", err)
	}
	commit, err := controlplane.CommitCheckIn(controlplane.CheckInCommitRequest{
		CheckIn: verified, Current: issued.request.Payload.PreviousWatermark,
		RequiredPolicy: issued.request.Payload.AppliedPolicy,
	})
	if err != nil {
		t.Fatalf("CommitCheckIn() error = %v, want nil", err)
	}
	watermark, err := commit.Watermark()
	if err != nil {
		t.Fatalf("commit Watermark() error = %v, want nil", err)
	}
	providerTime := testInstant(t, checkInResponseFutureInstant)
	header := controlplane.ResponseHeader{
		ProviderTime: providerTime, RequestNonce: issued.request.Payload.RequestNonce,
		Account: issued.certificate.Body.Account, Installation: issued.request.Payload.Installation,
		Revision: issued.request.Payload.Revision, Family: controlwire.RouteFamilyCheckIns,
		Status: controlplane.ProductStatusActive, Offering: issued.request.Payload.Build.Offering(),
		Policy: issued.request.Payload.AppliedPolicy,
	}
	decision, err := lease.NewGrantDecision(lease.GrantDecisionRequest{
		Header: lease.Header{
			Revision: lease.RevisionV1, Subject: watermark.Subject,
			Generation: watermark.Generation, IssuedAt: providerTime,
		},
		Grant: lease.Grant{
			NotBefore:    testInstant(t, checkInResponseFutureInstant),
			ContactAfter: testInstant(t, checkInResponseFutureInstant+10),
			NotAfter:     testInstant(t, checkInResponseFutureInstant+20),
			GoodUntil:    testInstant(t, checkInResponseFutureInstant+30),
		},
	})
	if err != nil {
		t.Fatalf("lease.NewGrantDecision() error = %v, want nil", err)
	}
	envelope, err := attest.Sign(attest.SignRequest[lease.Domain]{Body: decision, Signer: issued.authority})
	if err != nil {
		t.Fatalf("attest.Sign(lease) error = %v, want nil", err)
	}
	document, err := controlplane.IssueCommittedCheckInResponse(controlplane.CheckInResponsePreparation{
		Commit: commit, Header: header, Lease: lease.Document{Decision: decision, Attestation: envelope},
	}, issued.authority)
	if err != nil {
		t.Fatalf("IssueCommittedCheckInResponse() error = %v, want nil", err)
	}
	response, err := controlplane.VerifyCheckInResponse(controlplane.CheckInResponseVerification{
		Document: document,
		Expected: controlplane.ResponseExpectation{
			RequestNonce: header.RequestNonce, Account: header.Account,
			Installation: header.Installation, Revision: header.Revision,
			Family: header.Family, Offering: header.Offering,
		},
		TrustedKeys: issued.trusted,
	})
	if err != nil {
		t.Fatalf("VerifyCheckInResponse() error = %v, want nil", err)
	}
	payload, err := response.Payload()
	if err != nil || payload.Disposition != controlplane.UsageDispositionAccepted ||
		payload.Watermark != watermark || payload.Header.ProviderTime != providerTime ||
		payload.Header.Policy != issued.request.Payload.AppliedPolicy {
		t.Fatalf("verified response payload = (%+v, %v), want exact accepted usage, provider time, and policy", payload, err)
	}
}

// TestPrepareCheckInResponseRefusesEverySubstitutedAuthorityFact pressures the
// seam a server uses immediately before signing. A valid lease or header from
// another request must not become authoritative merely because each part is
// well formed on its own.
func TestPrepareCheckInResponseRefusesEverySubstitutedAuthorityFact(t *testing.T) {
	t.Parallel()

	cases := []struct {
		want   error
		mutate func(*testing.T, *controlplane.CheckInResponsePreparation, issuedCheckIn)
		name   string
	}{
		{name: "zero preparation has no committed request", mutate: func(_ *testing.T, preparation *controlplane.CheckInResponsePreparation, _ issuedCheckIn) {
			*preparation = controlplane.CheckInResponsePreparation{}
		}, want: core.ErrControlPlaneCheckIn},
		{name: "zero commit cannot manufacture accepted usage", mutate: func(_ *testing.T, preparation *controlplane.CheckInResponsePreparation, _ issuedCheckIn) {
			preparation.Commit = controlplane.VerifiedCheckInCommit{}
		}, want: core.ErrControlPlaneCheckIn},
		{name: "zero header cannot name an authority answer", mutate: func(_ *testing.T, preparation *controlplane.CheckInResponsePreparation, _ issuedCheckIn) {
			preparation.Header = controlplane.ResponseHeader{}
		}, want: core.ErrControlPlaneResponseHeader},
		{name: "zero lease cannot authorize future work", mutate: func(_ *testing.T, preparation *controlplane.CheckInResponsePreparation, _ issuedCheckIn) {
			preparation.Lease = lease.Document{}
		}, want: core.ErrLeaseContract},
		{name: "another request nonce cannot receive this commit", mutate: func(t *testing.T, preparation *controlplane.CheckInResponsePreparation, _ issuedCheckIn) {
			preparation.Header.RequestNonce = otherRequestNonce(t)
		}, want: core.ErrControlPlaneResponseBinding},
		{name: "another account cannot receive this installation commit", mutate: func(t *testing.T, preparation *controlplane.CheckInResponsePreparation, _ issuedCheckIn) {
			account, err := receipt.ParseAccountIdentity(checkInResponseOtherAccountHex)
			if err != nil {
				t.Fatalf("ParseAccountIdentity() error = %v, want nil", err)
			}
			preparation.Header.Account = account
		}, want: core.ErrControlPlaneResponseBinding},
		{name: "another installation cannot receive this commit", mutate: func(t *testing.T, preparation *controlplane.CheckInResponsePreparation, _ issuedCheckIn) {
			_, preparation.Header.Installation = testDeviceKey(t, checkInOtherDeviceSeed)
		}, want: core.ErrControlPlaneResponseBinding},
		{name: "another route family cannot receive this commit", mutate: func(_ *testing.T, preparation *controlplane.CheckInResponsePreparation, _ issuedCheckIn) {
			preparation.Header.Family = controlwire.RouteFamilyRegistrations
		}, want: core.ErrControlPlaneResponseBinding},
		{name: "another offering cannot receive this commit", mutate: func(_ *testing.T, preparation *controlplane.CheckInResponsePreparation, _ issuedCheckIn) {
			preparation.Header.Offering = core.OfferingBug
		}, want: core.ErrControlPlaneResponseBinding},
		{name: "another policy cannot be substituted after usage verification", mutate: func(t *testing.T, preparation *controlplane.CheckInResponsePreparation, _ issuedCheckIn) {
			activation, err := controlwire.NewPolicyActivation(preparation.Header.Policy.Activation.Uint64() + 1)
			if err != nil {
				t.Fatalf("NewPolicyActivation() error = %v, want nil", err)
			}
			preparation.Header.Policy.Activation = activation
		}, want: core.ErrControlWirePolicyCursor},
		{name: "lease generation must equal the committed watermark", mutate: func(t *testing.T, preparation *controlplane.CheckInResponsePreparation, issued issuedCheckIn) {
			preparation.Lease = authorityLeaseDocument(t, authorityLeaseFixtureRequest{
				Signer: issued.authority, Subject: issued.subject,
				Generation: issued.request.Payload.PreviousWatermark.Generation,
				IssuedAt:   preparation.Header.ProviderTime,
			})
		}, want: core.ErrControlPlaneDecisionConsistency},
		{name: "lease provider time must equal the response provider time", mutate: func(t *testing.T, preparation *controlplane.CheckInResponsePreparation, issued issuedCheckIn) {
			watermark, err := preparation.Commit.Watermark()
			if err != nil {
				t.Fatalf("commit Watermark() error = %v, want nil", err)
			}
			preparation.Lease = authorityLeaseDocument(t, authorityLeaseFixtureRequest{
				Signer: issued.authority, Subject: issued.subject,
				Generation: watermark.Generation,
				IssuedAt:   testInstant(t, checkInResponseFutureInstant+1),
			})
		}, want: core.ErrControlPlaneDecisionConsistency},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			issued, preparation := committedCheckInResponseFixture(t)
			tc.mutate(t, &preparation, issued)
			payload, gotErr := controlplane.PrepareCheckInResponse(preparation)
			if !errors.Is(gotErr, tc.want) || !errors.Is(gotErr, core.ErrControlPlaneCheckInResponse) {
				t.Fatalf("PrepareCheckInResponse() error = %v, want errors.Is(..., %v, %v)", gotErr, tc.want, core.ErrControlPlaneCheckInResponse)
			}
			if payload != (controlplane.CheckInResponsePayload{}) {
				t.Fatalf("PrepareCheckInResponse(refused) payload = %+v, want exact zero", payload)
			}
		})
	}
}

func registrationRequestFixture(t testing.TB) controlplane.RegistrationRequest {
	t.Helper()

	var request controlplane.RegistrationRequest
	if err := request.UnmarshalJSON(readGolden(t, "registration_request.json")); err != nil {
		t.Fatalf("RegistrationRequest.UnmarshalJSON(golden) error = %v, want nil", err)
	}
	return request
}

func registrationRequestVariant(t testing.TB, offering core.Offering, seed byte) controlplane.RegistrationRequest {
	t.Helper()

	request := registrationRequestFixture(t)
	request.Build = testBuildForOffering(t, offering)
	tokenBytes := [controlwire.RegistrationTokenBytes]byte{}
	nonceBytes := [core.SHA256DigestBytes]byte{}
	for index := range tokenBytes {
		tokenBytes[index] = seed
		nonceBytes[index] = seed + 1
	}
	token, err := controlwire.NewRegistrationToken(tokenBytes)
	if err != nil {
		t.Fatalf("NewRegistrationToken() error = %v, want nil", err)
	}
	nonce, err := controlwire.NewRequestNonce(nonceBytes)
	if err != nil {
		t.Fatalf("NewRequestNonce() error = %v, want nil", err)
	}
	request.Token = token
	request.RequestNonce = nonce
	if err := request.Validate(); err != nil {
		t.Fatalf("registration request variant Validate() error = %v, want nil", err)
	}
	return request
}

func validOfferings(t testing.TB) []core.Offering {
	t.Helper()

	var offerings []core.Offering
	for value := 0; value <= 255; value++ {
		offering := core.Offering(value)
		if offering.IsValid() {
			offerings = append(offerings, offering)
		}
	}
	if len(offerings) < 3 {
		t.Fatalf("valid offerings = %d, want at least three", len(offerings))
	}
	return offerings
}

func committedCheckInResponseFixture(t testing.TB) (issuedCheckIn, controlplane.CheckInResponsePreparation) {
	t.Helper()

	issued := issueTestCheckIn(t, core.OfferingPeachfuzz, testCheckInWindow())
	verified, err := controlplane.VerifyCheckIn(controlplane.CheckInVerification{Request: issued.request, TrustedKeys: issued.trusted})
	if err != nil {
		t.Fatalf("VerifyCheckIn() error = %v, want nil", err)
	}
	commit, err := controlplane.CommitCheckIn(controlplane.CheckInCommitRequest{
		CheckIn: verified, Current: issued.request.Payload.PreviousWatermark,
		RequiredPolicy: issued.request.Payload.AppliedPolicy,
	})
	if err != nil {
		t.Fatalf("CommitCheckIn() error = %v, want nil", err)
	}
	watermark, err := commit.Watermark()
	if err != nil {
		t.Fatalf("commit Watermark() error = %v, want nil", err)
	}
	providerTime := testInstant(t, checkInResponseFutureInstant)
	header := controlplane.ResponseHeader{
		ProviderTime: providerTime, RequestNonce: issued.request.Payload.RequestNonce,
		Account: issued.certificate.Body.Account, Installation: issued.request.Payload.Installation,
		Revision: issued.request.Payload.Revision, Family: controlwire.RouteFamilyCheckIns,
		Status: controlplane.ProductStatusActive, Offering: issued.request.Payload.Build.Offering(),
		Policy: issued.request.Payload.AppliedPolicy,
	}
	return issued, controlplane.CheckInResponsePreparation{
		Commit: commit, Header: header,
		Lease: authorityLeaseDocument(t, authorityLeaseFixtureRequest{
			Signer: issued.authority, Subject: watermark.Subject,
			Generation: watermark.Generation, IssuedAt: providerTime,
		}),
	}
}

type authorityLeaseFixtureRequest struct {
	Signer     ed25519.PrivateKey
	Subject    lease.Subject
	Generation lease.Generation
	IssuedAt   temporal.Instant
}

func authorityLeaseDocument(t testing.TB, request authorityLeaseFixtureRequest) lease.Document {
	t.Helper()

	decision, err := lease.NewGrantDecision(lease.GrantDecisionRequest{
		Header: lease.Header{
			Revision: lease.RevisionV1, Subject: request.Subject,
			Generation: request.Generation, IssuedAt: request.IssuedAt,
		},
		Grant: lease.Grant{
			NotBefore:    request.IssuedAt,
			ContactAfter: testInstant(t, checkInResponseFutureInstant+10),
			NotAfter:     testInstant(t, checkInResponseFutureInstant+20),
			GoodUntil:    testInstant(t, checkInResponseFutureInstant+30),
		},
	})
	if err != nil {
		t.Fatalf("lease.NewGrantDecision() error = %v, want nil", err)
	}
	envelope, err := attest.Sign(attest.SignRequest[lease.Domain]{Body: decision, Signer: request.Signer})
	if err != nil {
		t.Fatalf("attest.Sign(lease) error = %v, want nil", err)
	}
	return lease.Document{Decision: decision, Attestation: envelope}
}
