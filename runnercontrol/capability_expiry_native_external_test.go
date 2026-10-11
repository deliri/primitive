package runnercontrol_test

import (
	"context"
	"errors"
	"math"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestSignedCapabilityExpiryUsesTrustedTemporalObservation(t *testing.T) {
	t.Parallel()
	claim, trusted := schedulingClaimDocumentFixture(t)
	for index := range 64 {
		t.Run(strconv.Itoa(index), func(t *testing.T) {
			t.Parallel()
			for _, observed := range [...]int64{math.MinInt64 + int64(index), 2_000_000 + int64(index), 2_999_999 - int64(index), 3_000_000, 3_249_999 - int64(index), 3_250_000, 3_499_999 - int64(index), 3_500_000, math.MaxInt64 - int64(index)} {
				verification := runnercontrol.CapabilityVerification{TrustedKeys: trusted, ObservedAt: temporal.InstantFromNanoseconds(observed)}
				results := [...]struct {
					err     error
					expires int64
				}{
					{runnercontrol.VerifySchedulingCapability(claim.Capability, verification), 3_500_000},
					{runnercontrol.VerifyMemberCapability(claim.Members[0], verification), 3_250_000},
					{runnercontrol.VerifyExperimentCapability(claim.Direct[0], verification), 3_000_000},
					{runnercontrol.VerifySchedulingClaim(claim, verification), 3_000_000},
				}
				for _, result := range results {
					if observed < result.expires {
						if result.err != nil {
							t.Fatalf("genuine native-signed capability before expiry: %v", result.err)
						}
					} else if !errors.Is(result.err, core.ErrRunnerControlExpired) {
						t.Fatalf("trusted time %d at/after signed expiry %d: %v", observed, result.expires, result.err)
					}
				}
			}
			if err := runnercontrol.VerifySchedulingClaim(claim, runnercontrol.CapabilityVerification{TrustedKeys: trusted}); err == nil {
				t.Fatal("missing trusted observation admitted authority")
			}
		})
	}
}

func TestExpiredSourceFenceCannotUseOldPeerRequestTime(t *testing.T) {
	t.Parallel()
	for index := range 64 {
		t.Run(strconv.Itoa(index), func(t *testing.T) {
			t.Parallel()
			request, projection := sourceAcquisitionSocketFixture(t)
			request.Fence.Machine.ExpiresAt = temporal.InstantFromNanoseconds(4_000_000)
			request.RequestedAt = temporal.InstantFromNanoseconds(int64(index) + 1)
			projection.Fence = request.Fence
			if err := request.Validate(); err != nil {
				t.Fatalf("old request remains structurally valid: %v", err)
			}
			calls := 0
			repository := sourceAcquisitionRepositoryFunc(func(context.Context, runnercontrol.SourceAcquisitionRequest) (runnercontrol.SourceAcquisitionProjection, error) {
				calls++
				return projection, nil
			})
			contract := sourceAcquisitionSocketContractFixture(t, "/runner/source-expired")
			boundary, err := runnercontrol.NewSourceAcquisitionServer(contract, repository)
			if err != nil {
				t.Fatal(err)
			}
			peer := runnerPeerFixture(t, request.Fence.Machine.Machine, request.Fence.Machine.Generation)
			server, result := newRunnerControlSocketServer(t, &peer, boundary.Serve)
			client, err := runnercontrol.NewSourceAcquisitionClient(runnerControlClientConfiguration(t, server, contract))
			if err != nil {
				t.Fatal(err)
			}
			_, clientErr := client.Acquire(t.Context(), request)
			serverErr := waitRunnerControlSocketServer(t, result)
			if clientErr == nil || !errors.Is(serverErr, core.ErrRunnerControlExpired) || calls != 0 {
				t.Fatalf("expired fence replay: client %v, server %v, repository calls %d", clientErr, serverErr, calls)
			}
		})
	}
}

func FuzzSignedClaimTrustedExpiry(f *testing.F) {
	claim, trusted := schedulingClaimDocumentFixture(f)
	for _, observed := range [...]int64{math.MinInt64, 2_000_000, 2_999_999, 3_000_000, 3_500_000, math.MaxInt64} {
		f.Add(observed)
	}
	f.Fuzz(func(t *testing.T, observed int64) {
		verification := runnercontrol.CapabilityVerification{TrustedKeys: trusted, ObservedAt: temporal.InstantFromNanoseconds(observed)}
		err := runnercontrol.VerifySchedulingClaim(claim, verification)
		if observed < 3_000_000 {
			if err != nil {
				t.Fatalf("genuine signed claim before earliest expiry: %v", err)
			}
		} else if !errors.Is(err, core.ErrRunnerControlExpired) {
			t.Fatalf("genuine signed claim at/after earliest expiry: %v", err)
		}
	})
}
