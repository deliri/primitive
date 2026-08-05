package controlplane

import (
	"encoding/json"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
)

// These are the exact status tokens an authority emits. They are the
// commercial state of one installation, so this package does not invent,
// abbreviate, or reorder them.
const (
	ProductStatusActiveToken          = "active"
	ProductStatusPaymentRetryToken    = "payment_retry"
	ProductStatusReadOnlyToken        = "read_only"
	ProductStatusStoppedToken         = "stopped"
	ProductStatusUpgradeRequiredToken = "upgrade_required"
	ProductStatusRevokedToken         = "revoked"
)

// ProductStatus is the complete commercial status domain an authority may
// return for one installation.
//
// The whole set is named even though several members are treated the same way,
// because an unrecognised status must fail loudly rather than fall through a
// default branch. The default a client would pick under uncertainty is "keep
// working", and that is precisely the case where it must not.
type ProductStatus uint8

const (
	// ProductStatusInvalid is the unset status and is never admitted on the wire.
	ProductStatusInvalid ProductStatus = iota
	// ProductStatusActive is a paid installation in good standing.
	ProductStatusActive
	// ProductStatusPaymentRetry is a payment failure inside its grace period.
	ProductStatusPaymentRetry
	// ProductStatusReadOnly may inspect existing records but start no new work.
	ProductStatusReadOnly
	// ProductStatusStopped has passed grace and starts no new work.
	ProductStatusStopped
	// ProductStatusUpgradeRequired runs a build the authority no longer accepts.
	ProductStatusUpgradeRequired
	// ProductStatusRevoked has had its entitlement withdrawn.
	ProductStatusRevoked
	productStatusLimit
)

func productStatusTokens() [productStatusLimit]string {
	return [...]string{
		ProductStatusInvalid:         "",
		ProductStatusActive:          ProductStatusActiveToken,
		ProductStatusPaymentRetry:    ProductStatusPaymentRetryToken,
		ProductStatusReadOnly:        ProductStatusReadOnlyToken,
		ProductStatusStopped:         ProductStatusStoppedToken,
		ProductStatusUpgradeRequired: ProductStatusUpgradeRequiredToken,
		ProductStatusRevoked:         ProductStatusRevokedToken,
	}
}

// Validate rejects the unset status and every status outside the domain.
func (s ProductStatus) Validate() error {
	if s <= ProductStatusInvalid || s >= productStatusLimit || productStatusTokens()[s] == "" {
		return productStatusError()
	}
	return nil
}

// IsValid reports whether s is a published commercial status.
func (s ProductStatus) IsValid() bool { return s.Validate() == nil }

// String returns the canonical token, or empty text for an invalid status.
func (s ProductStatus) String() string {
	if s >= productStatusLimit {
		return ""
	}
	return productStatusTokens()[s]
}

// ParseProductStatus accepts one exact published token.
func ParseProductStatus(value string) (ProductStatus, error) {
	tokens := productStatusTokens()
	for status := ProductStatusInvalid + 1; status < productStatusLimit; status++ {
		if tokens[status] != "" && tokens[status] == value {
			return status, nil
		}
	}
	return ProductStatusInvalid, productStatusError()
}

// AdmitsGrant reports whether a signed Lease grant is consistent with s.
//
// This is the consistency rule, not the gate. Gate decides whether new work may
// begin, and it decides that from the authenticated Lease assessment alone.
// What this answers is narrower and comes first: an authority must never issue
// a grant alongside a status that contradicts it, so a document pairing the two
// is either an authority bug or a forgery, and it is refused rather than acted
// on in whichever half the reader prefers.
//
// Active and payment-retry admit a grant. Payment retry is deliberately
// included: a failed charge inside its grace period is still a paying customer,
// and cutting work off at the first failure would be a refund request rather
// than a sale. Read-only, stopped, upgrade-required, and revoked do not.
func (s ProductStatus) AdmitsGrant() bool {
	return s == ProductStatusActive || s == ProductStatusPaymentRetry
}

// AdmitsOutcome reports whether s may accompany the given Lease outcome.
//
// A refusal or a revocation may arrive under any valid status, because an
// authority refuses for reasons this package does not model. A grant is the
// constrained case, and it is constrained in one direction only: the statuses
// that stop work must never arrive carrying permission to continue it.
func (s ProductStatus) AdmitsOutcome(outcome lease.Outcome) bool {
	if s.Validate() != nil || outcome.Validate() != nil {
		return false
	}
	if outcome == lease.OutcomeGrant {
		return s.AdmitsGrant()
	}
	return true
}

// ValidateOutcome closes the exact status a decision may travel beside, for one
// offering.
//
// This is stricter than AdmitsOutcome and is the rule an authenticated document
// is held to. Every outcome pins its admissible statuses: a grant needs a
// paying status, a revocation must say revoked, and a refusal must name why.
// Read-only is the offering-dependent case: it means "your evidence is still
// readable, you just cannot make more", which is a real product state for Bug
// and Witness and meaningless for an offering that only produces new work.
func (s ProductStatus) ValidateOutcome(offering core.Offering, outcome lease.Outcome) error {
	if err := errors.Join(s.Validate(), offering.Validate(), outcome.Validate()); err != nil {
		return productStatusError(err)
	}
	switch outcome {
	case lease.OutcomeGrant:
		return admitted(s.AdmitsGrant())
	case lease.OutcomeRevocation:
		return admitted(s == ProductStatusRevoked)
	case lease.OutcomeRefusal:
		return admitted(s.admitsRefusal(offering))
	}
	return consistencyError()
}

func (s ProductStatus) admitsRefusal(offering core.Offering) bool {
	if s == ProductStatusStopped || s == ProductStatusUpgradeRequired {
		return true
	}
	return s == ProductStatusReadOnly && offeringAdmitsReadOnly(offering)
}

// offeringAdmitsReadOnly names the offerings for which read-only is a state a
// customer can actually be in.
func offeringAdmitsReadOnly(offering core.Offering) bool {
	return offering == core.OfferingBug || offering == core.OfferingWitness
}

func admitted(ok bool) error {
	if ok {
		return nil
	}
	return consistencyError()
}

// MarshalJSON emits the canonical token and refuses an unset status.
func (s ProductStatus) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONString(s.String())
	if err != nil {
		return nil, jsonError(productStatusError(err))
	}
	return encoded, nil
}

// UnmarshalJSON accepts only a published token and leaves s unchanged on every
// rejection.
func (s *ProductStatus) UnmarshalJSON(data []byte) error {
	if s == nil {
		return jsonError(productStatusError())
	}
	token, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return jsonError(productStatusError(err))
	}
	parsed, err := ParseProductStatus(token)
	if err != nil {
		return jsonError(err)
	}
	*s = parsed
	return nil
}

var (
	_ core.Validatable = ProductStatusInvalid
	_ json.Marshaler   = ProductStatusInvalid
	_ json.Unmarshaler = (*ProductStatus)(nil)
)
