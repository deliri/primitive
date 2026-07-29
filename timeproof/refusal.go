package timeproof

import (
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

// RefusalStatus is the closed RFC 3161 PKIStatus set.
type RefusalStatus uint8

const (
	// RefusalStatusUnknown is the invalid zero status.
	RefusalStatusUnknown RefusalStatus = iota
	// RefusalStatusGranted is an unmodified granted request.
	RefusalStatusGranted
	// RefusalStatusGrantedWithMods is a granted but modified request.
	RefusalStatusGrantedWithMods
	// RefusalStatusRejection is an outright rejected request.
	RefusalStatusRejection
	// RefusalStatusWaiting is a deferred request carrying no token.
	RefusalStatusWaiting
	// RefusalStatusRevocationWarning is an imminent revocation warning.
	RefusalStatusRevocationWarning
	// RefusalStatusRevocationNotification is a completed revocation notice.
	RefusalStatusRevocationNotification
	refusalStatusLimit
)

// Validate rejects statuses outside the closed RFC 3161 set.
func (s RefusalStatus) Validate() error {
	if s <= RefusalStatusUnknown || s >= refusalStatusLimit {
		return contractError(nil)
	}
	return nil
}

// IsValid reports whether s belongs to the closed refusal-status domain.
func (s RefusalStatus) IsValid() bool { return s.Validate() == nil }

// OffWireEnum declares that refusal status is error detail, not persistence.
func (RefusalStatus) OffWireEnum() {}

// String returns the canonical diagnostic token.
func (s RefusalStatus) String() string {
	switch s {
	case RefusalStatusGranted:
		return "granted"
	case RefusalStatusGrantedWithMods:
		return "granted_with_modifications"
	case RefusalStatusRejection:
		return "rejection"
	case RefusalStatusWaiting:
		return "waiting"
	case RefusalStatusRevocationWarning:
		return "revocation_warning"
	case RefusalStatusRevocationNotification:
		return "revocation_notification"
	case RefusalStatusUnknown, refusalStatusLimit:
		return ""
	}
	return ""
}

// granted reports whether the authority issued a token for the request.
func (s RefusalStatus) granted() bool {
	return s == RefusalStatusGranted || s == RefusalStatusGrantedWithMods
}

// rfcValue returns the PKIStatus integer RFC 3161 assigns to s.
func (s RefusalStatus) rfcValue() (int, error) {
	switch s {
	case RefusalStatusGranted:
		return 0, nil
	case RefusalStatusGrantedWithMods:
		return 1, nil
	case RefusalStatusRejection:
		return 2, nil
	case RefusalStatusWaiting:
		return 3, nil
	case RefusalStatusRevocationWarning:
		return 4, nil
	case RefusalStatusRevocationNotification:
		return 5, nil
	case RefusalStatusUnknown, refusalStatusLimit:
		return 0, contractError(nil)
	}
	return 0, contractError(nil)
}

// refusalStatusFromRFC projects one wire integer through the closed enum's own
// forward mapping, so the two directions cannot drift apart.
func refusalStatusFromRFC(value int) (RefusalStatus, error) {
	for status := RefusalStatusGranted; status < refusalStatusLimit; status++ {
		statusValue, err := status.rfcValue()
		if err == nil && statusValue == value {
			return status, nil
		}
	}
	return RefusalStatusUnknown, invalidError(nil)
}

// RefusalCode is the closed RFC 3161 PKIFailureInfo set.
type RefusalCode uint8

const (
	// RefusalCodeUnknown is the invalid zero code.
	RefusalCodeUnknown RefusalCode = iota
	// RefusalCodeBadAlgorithm reports an unrecognized or unsupported algorithm.
	RefusalCodeBadAlgorithm
	// RefusalCodeBadRequest reports a transaction the authority forbids.
	RefusalCodeBadRequest
	// RefusalCodeBadDataFormat reports a request the authority cannot parse.
	RefusalCodeBadDataFormat
	// RefusalCodeTimeNotAvailable reports an unavailable time source.
	RefusalCodeTimeNotAvailable
	// RefusalCodeUnacceptedPolicy reports a refused requested policy.
	RefusalCodeUnacceptedPolicy
	// RefusalCodeUnacceptedExtension reports a refused requested extension.
	RefusalCodeUnacceptedExtension
	// RefusalCodeAdditionalInfoMissing reports missing required information.
	RefusalCodeAdditionalInfoMissing
	// RefusalCodeSystemFailure reports an authority-side failure.
	RefusalCodeSystemFailure
	refusalCodeLimit
)

// Validate rejects codes outside the closed RFC 3161 set.
func (c RefusalCode) Validate() error {
	if c <= RefusalCodeUnknown || c >= refusalCodeLimit {
		return contractError(nil)
	}
	return nil
}

// IsValid reports whether c belongs to the closed refusal-code domain.
func (c RefusalCode) IsValid() bool { return c.Validate() == nil }

// OffWireEnum declares that refusal codes are error detail, not persistence.
func (RefusalCode) OffWireEnum() {}

// String returns the canonical diagnostic token.
func (c RefusalCode) String() string {
	switch c {
	case RefusalCodeBadAlgorithm:
		return "bad_algorithm"
	case RefusalCodeBadRequest:
		return "bad_request"
	case RefusalCodeBadDataFormat:
		return "bad_data_format"
	case RefusalCodeTimeNotAvailable:
		return "time_not_available"
	case RefusalCodeUnacceptedPolicy:
		return "unaccepted_policy"
	case RefusalCodeUnacceptedExtension:
		return "unaccepted_extension"
	case RefusalCodeAdditionalInfoMissing:
		return "additional_info_missing"
	case RefusalCodeSystemFailure:
		return "system_failure"
	case RefusalCodeUnknown, refusalCodeLimit:
		return ""
	}
	return ""
}

// rfcBit returns the PKIFailureInfo bit position RFC 3161 assigns to c.
func (c RefusalCode) rfcBit() (uint8, error) {
	switch c {
	case RefusalCodeBadAlgorithm:
		return 0, nil
	case RefusalCodeBadRequest:
		return 2, nil
	case RefusalCodeBadDataFormat:
		return 5, nil
	case RefusalCodeTimeNotAvailable:
		return 14, nil
	case RefusalCodeUnacceptedPolicy:
		return 15, nil
	case RefusalCodeUnacceptedExtension:
		return 16, nil
	case RefusalCodeAdditionalInfoMissing:
		return 17, nil
	case RefusalCodeSystemFailure:
		return 25, nil
	case RefusalCodeUnknown, refusalCodeLimit:
		return 0, contractError(nil)
	}
	return 0, contractError(nil)
}

// refusalCodeSet is the bounded closed set of declared failure codes.
type refusalCodeSet struct {
	bits uint32
}

func newRefusalCodeSet(codes ...RefusalCode) (refusalCodeSet, error) {
	var set refusalCodeSet
	for _, code := range codes {
		bit, err := code.rfcBit()
		if err != nil || set.bits&(uint32(1)<<bit) != 0 {
			return refusalCodeSet{}, contractError(err)
		}
		set.bits |= uint32(1) << bit
	}
	return set, set.validate()
}

func (s refusalCodeSet) validate() error {
	unknown := s.bits
	for code := RefusalCodeBadAlgorithm; code < refusalCodeLimit; code++ {
		bit, err := code.rfcBit()
		if err != nil {
			return contractError(err)
		}
		unknown &^= uint32(1) << bit
	}
	if unknown != 0 {
		return contractError(nil)
	}
	return nil
}

func (s refusalCodeSet) isZero() bool { return s.bits == 0 }

// codes projects the set back into ascending closed-enum order.
func (s refusalCodeSet) codes() []RefusalCode {
	codes := make([]RefusalCode, 0, refusalMaximumCodeCount)
	for code := RefusalCodeBadAlgorithm; code < refusalCodeLimit; code++ {
		bit, err := code.rfcBit()
		if err != nil || s.bits&(uint32(1)<<bit) == 0 {
			continue
		}
		codes = append(codes, code)
	}
	return codes
}

func refusalMaximumRFCBit() (uint8, error) {
	maximum := uint8(0)
	for code := RefusalCodeBadAlgorithm; code < refusalCodeLimit; code++ {
		bit, err := code.rfcBit()
		if err != nil {
			return 0, contractError(err)
		}
		if bit > maximum {
			maximum = bit
		}
	}
	return maximum, nil
}

func refusalCodeFromRFCBit(bit int) (RefusalCode, error) {
	for code := RefusalCodeBadAlgorithm; code < refusalCodeLimit; code++ {
		codeBit, err := code.rfcBit()
		if err == nil && bit == int(codeBit) {
			return code, nil
		}
	}
	return RefusalCodeUnknown, contractError(nil)
}

// Refusal is the typed authority refusal. Callers reach the authority's own
// reason with errors.As; errors.Is still matches core.ErrTimeProofRefused.
type Refusal struct {
	codes  refusalCodeSet
	status RefusalStatus
}

// authorityConclusion is the authority's own PKIStatusInfo conclusion, before
// the package decides whether it grants a token.
type authorityConclusion struct {
	codes  refusalCodeSet
	status RefusalStatus
}

func newRefusal(conclusion authorityConclusion) (Refusal, error) {
	refusal := Refusal(conclusion)
	if err := refusal.Validate(); err != nil {
		return Refusal{}, err
	}
	return refusal, nil
}

// Validate rejects any refusal that is not a closed non-granting conclusion.
func (r Refusal) Validate() error {
	if err := r.status.Validate(); err != nil {
		return err
	}
	if r.status.granted() {
		return contractError(nil)
	}
	return r.codes.validate()
}

// Status returns the authority's closed PKIStatus conclusion.
func (r Refusal) Status() RefusalStatus { return r.status }

// Codes returns the authority's declared failure codes in enum order.
func (r Refusal) Codes() []RefusalCode { return r.codes.codes() }

// Unwrap keeps the Core-owned refusal identity reachable with errors.Is.
func (r Refusal) Unwrap() error { return core.ErrTimeProofRefused }

// Error renders the operator-facing diagnostic. The typed identity, status,
// and codes are the contract; this text is not.
func (r Refusal) Error() string {
	text := core.ErrTimeProofRefused.Error() + ": " + r.status.String()
	codes := r.Codes()
	if len(codes) == 0 {
		return text
	}
	tokens := make([]string, 0, len(codes))
	for _, code := range codes {
		tokens = append(tokens, code.String())
	}
	return text + " [" + strings.Join(tokens, " ") + "]"
}

var (
	_ core.OffWireEnum = RefusalStatusUnknown
	_ core.OffWireEnum = RefusalCodeUnknown
)
