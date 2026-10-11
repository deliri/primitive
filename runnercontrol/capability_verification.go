package runnercontrol

import (
	"errors"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// CapabilityVerification borrows caller-owned trust and a trusted observation.
// ObservedAt must come from the verifier's Temporal clock, never from a peer's
// signed or unsigned request time. Expiry is exclusive. No replay registry or
// scheduling policy is inferred; an unexpired capability may be replayed until
// the product's nonce and fence policy consumes or revokes it.
type CapabilityVerification struct {
	TrustedKeys attest.TrustedKeys
	ObservedAt  temporal.Instant
}

func (v CapabilityVerification) Validate() error {
	return errors.Join(v.TrustedKeys.Validate(), v.ObservedAt.Validate())
}

func verifyCapabilityExpiry(observed, expires temporal.Instant) error {
	comparison, err := observed.Compare(expires)
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	if comparison != core.ComparisonLess {
		return core.ErrRunnerControlExpired
	}
	return nil
}

func (CapabilityVerification) runnerControlInternalFlow() {}

var _ internalFlow = CapabilityVerification{}
