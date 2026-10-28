package stripe

import (
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
)

// CheckoutSessionStatus is a closed in-process observation of the provider value.
type CheckoutSessionStatus uint8

const (
	CheckoutSessionStatusUnknown CheckoutSessionStatus = iota
	CheckoutSessionStatusNull
	CheckoutSessionStatusOpen
	CheckoutSessionStatusComplete
	CheckoutSessionStatusExpired
)

func (v CheckoutSessionStatus) IsValid() bool {
	return v >= CheckoutSessionStatusNull && v <= CheckoutSessionStatusExpired
}
func (v CheckoutSessionStatus) Validate() error {
	if !v.IsValid() {
		return core.ErrStripeContract
	}
	return nil
}
func (CheckoutSessionStatus) OffWireEnum() {}
func (v CheckoutSessionStatus) String() string {
	if !v.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return [...]string{"", "null", "open", "complete", "expired"}[v]
}

// CheckoutSessionPaymentStatus is a closed in-process observation of the provider value.
type CheckoutSessionPaymentStatus uint8

const (
	CheckoutSessionPaymentStatusUnknown CheckoutSessionPaymentStatus = iota
	CheckoutSessionPaymentStatusPaid
	CheckoutSessionPaymentStatusUnpaid
	CheckoutSessionPaymentStatusNoPaymentRequired
)

func (v CheckoutSessionPaymentStatus) IsValid() bool {
	return v >= CheckoutSessionPaymentStatusPaid && v <= CheckoutSessionPaymentStatusNoPaymentRequired
}
func (v CheckoutSessionPaymentStatus) Validate() error {
	if !v.IsValid() {
		return core.ErrStripeContract
	}
	return nil
}
func (CheckoutSessionPaymentStatus) OffWireEnum() {}
func (v CheckoutSessionPaymentStatus) String() string {
	if !v.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return [...]string{"", "paid", "unpaid", "no_payment_required"}[v]
}

// CheckoutSessionMode is a closed in-process observation of the provider value.
type CheckoutSessionMode uint8

const (
	CheckoutSessionModeUnknown CheckoutSessionMode = iota
	CheckoutSessionModePayment
	CheckoutSessionModeSetup
	CheckoutSessionModeSubscription
)

func (v CheckoutSessionMode) IsValid() bool {
	return v >= CheckoutSessionModePayment && v <= CheckoutSessionModeSubscription
}
func (v CheckoutSessionMode) Validate() error {
	if !v.IsValid() {
		return core.ErrStripeContract
	}
	return nil
}
func (CheckoutSessionMode) OffWireEnum() {}
func (v CheckoutSessionMode) String() string {
	if !v.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return [...]string{"", "payment", "setup", "subscription"}[v]
}

func checkoutStatus(token jsonio.Token) (CheckoutSessionStatus, error) {
	if token.Kind == jsonio.TokenNull {
		return CheckoutSessionStatusNull, nil
	}
	if token.Kind != jsonio.TokenString {
		return 0, core.ErrJSONContract
	}
	for value := CheckoutSessionStatusOpen; value <= CheckoutSessionStatusExpired; value++ {
		if token.Text == value.String() {
			return value, nil
		}
	}
	return 0, core.ErrJSONContract
}

func checkoutPaymentStatus(token jsonio.Token) (CheckoutSessionPaymentStatus, error) {
	if token.Kind != jsonio.TokenString {
		return 0, core.ErrJSONContract
	}
	for value := CheckoutSessionPaymentStatusPaid; value <= CheckoutSessionPaymentStatusNoPaymentRequired; value++ {
		if token.Text == value.String() {
			return value, nil
		}
	}
	return 0, core.ErrJSONContract
}

func checkoutMode(token jsonio.Token) (CheckoutSessionMode, error) {
	if token.Kind != jsonio.TokenString {
		return 0, core.ErrJSONContract
	}
	for value := CheckoutSessionModePayment; value <= CheckoutSessionModeSubscription; value++ {
		if token.Text == value.String() {
			return value, nil
		}
	}
	return 0, core.ErrJSONContract
}

var (
	_ core.OffWireEnum = CheckoutSessionStatus(0)
	_ core.OffWireEnum = CheckoutSessionPaymentStatus(0)
	_ core.OffWireEnum = CheckoutSessionMode(0)
)
