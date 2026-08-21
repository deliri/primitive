package payment

import (
	json "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/id"
)

type paymentIdentityDomain uint8

// PaymentID is the authority-issued UUIDv7 for one settled customer payment.
type PaymentID struct {
	value id.UUIDv7
	_     paymentIdentityDomain
}

// NewPaymentID applies Payment's nominal identity boundary to a UUIDv7.
func NewPaymentID(value id.UUIDv7) (PaymentID, error) {
	candidate := PaymentID{value: value}
	if err := candidate.Validate(); err != nil {
		return PaymentID{}, err
	}
	return candidate, nil
}

// ParsePaymentID parses one canonical UUIDv7 payment identity.
func ParsePaymentID(value string) (PaymentID, error) {
	parsed, err := id.ParseUUIDv7(value)
	if err != nil {
		return PaymentID{}, contractError(err)
	}
	return NewPaymentID(parsed)
}

// Validate rejects the unset identity.
func (i PaymentID) Validate() error {
	if err := i.value.Validate(); err != nil {
		return contractError(errors.New("payment identity is invalid"), err)
	}
	return nil
}

// String returns the canonical UUIDv7 or empty text for an invalid identity.
func (i PaymentID) String() string {
	if i.Validate() != nil {
		return ""
	}
	return i.value.String()
}

// MarshalJSON emits the canonical UUIDv7 string.
func (i PaymentID) MarshalJSON() ([]byte, error) {
	if err := i.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return json.Marshal(i.value)
}

// UnmarshalJSON accepts one canonical UUIDv7 and preserves the receiver on rejection.
func (i *PaymentID) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError(errors.New("nil payment identity receiver"))
	}
	var value id.UUIDv7
	if err := json.Unmarshal(data, &value); err != nil {
		return jsonError(err)
	}
	candidate, err := NewPaymentID(value)
	if err != nil {
		return jsonError(err)
	}
	*i = candidate
	return nil
}

var (
	_ core.Validatable            = PaymentID{}
	_ core.ValidatedJSONMarshaler = PaymentID{}
)
