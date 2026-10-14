package currency

import (
	"iter"
	"math"
	"math/bits"

	"github.com/deliri/primitive/v2026/core"
)

// Quantity is an admitted nonnegative count. A constructed zero is valid.
type Quantity struct {
	value uint64
	set   bool
}

// NewQuantity admits the full unsigned count domain without a flow quota.
func NewQuantity(value uint64) (Quantity, error) { return Quantity{value: value, set: true}, nil }

// Validate refuses an unconstructed quantity.
func (q Quantity) Validate() error {
	if !q.set {
		return core.ErrCurrencyContract
	}
	return nil
}

// BasisPoints is an exact nonnegative rate with denominator 10,000.
// Rates above 100% remain valid; the resulting amount must fit int64.
type BasisPoints struct {
	value uint64
	set   bool
}

// NewBasisPoints admits an exact unsigned basis-point rate.
func NewBasisPoints(value uint64) (BasisPoints, error) {
	return BasisPoints{value: value, set: true}, nil
}

// Validate refuses an unconstructed rate.
func (b BasisPoints) Validate() error {
	if !b.set {
		return core.ErrCurrencyContract
	}
	return nil
}

// Rounding is the closed minor-unit rounding policy.
type Rounding uint8

const (
	// RoundingUnknown is invalid, requiring callers to name their policy.
	RoundingUnknown Rounding = iota
	// RoundingDown rounds toward negative infinity, including negative amounts.
	RoundingDown
	// RoundingHalfUp rounds nearest, with exact ties away from zero.
	RoundingHalfUp
	// RoundingHalfEven rounds nearest, with exact ties to an even minor unit.
	RoundingHalfEven
)

// IsValid reports whether the rounding policy is admitted.
func (r Rounding) IsValid() bool { return r >= RoundingDown && r <= RoundingHalfEven }

// Validate refuses unknown or future rounding policies.
func (r Rounding) Validate() error {
	if !r.IsValid() {
		return core.ErrCurrencyContract
	}
	return nil
}

// String renders the admitted rounding policy.
func (r Rounding) String() string {
	switch r {
	case RoundingDown:
		return "down"
	case RoundingHalfUp:
		return "half-up"
	case RoundingHalfEven:
		return "half-even"
	default:
		return ""
	}
}

// OffWireEnum marks rounding as caller policy rather than an implicit wire number.
func (Rounding) OffWireEnum() {}

// Sign is the closed sign of an admitted amount.
type Sign uint8

const (
	// SignUnknown is invalid.
	SignUnknown Sign = iota
	// SignNegative identifies an amount below zero.
	SignNegative
	// SignZero identifies an exact zero amount.
	SignZero
	// SignPositive identifies an amount above zero.
	SignPositive
)

// IsValid reports membership in the sign domain.
func (s Sign) IsValid() bool { return s >= SignNegative && s <= SignPositive }

// Validate refuses unknown or future sign values.
func (s Sign) Validate() error {
	if !s.IsValid() {
		return core.ErrCurrencyContract
	}
	return nil
}

// String renders an admitted sign.
func (s Sign) String() string {
	switch s {
	case SignNegative:
		return "negative"
	case SignZero:
		return "zero"
	case SignPositive:
		return "positive"
	default:
		return ""
	}
}

// OffWireEnum marks the sign as an off-wire observation.
func (Sign) OffWireEnum() {}

// Sign observes the amount without exposing its numeric representation.
func (a Amount) Sign() (Sign, error) {
	if err := a.Validate(); err != nil {
		return SignUnknown, err
	}
	if a.minorUnits < 0 {
		return SignNegative, nil
	}
	if a.minorUnits > 0 {
		return SignPositive, nil
	}
	return SignZero, nil
}

// Negate returns the exact inverse, refusing the int64 minimum whose inverse cannot fit.
func (a Amount) Negate() (Amount, error) {
	if err := a.Validate(); err != nil {
		return Amount{}, err
	}
	if a.minorUnits == math.MinInt64 {
		return Amount{}, overflowError()
	}
	return Amount{minorUnits: -a.minorUnits, code: a.code}, nil
}

func amountMagnitude(value int64) uint64 {
	if value < 0 {
		return uint64(-(value + 1)) + 1
	}
	return uint64(value)
}

// MultiplyQuantity multiplies the exact amount by a nonnegative count.
// Go's standard-library 128-bit product owns the intermediate arithmetic.
// Source: https://pkg.go.dev/math/bits#Mul64
func (a Amount) MultiplyQuantity(quantity Quantity) (Amount, error) {
	if err := a.Validate(); err != nil {
		return Amount{}, err
	}
	if err := quantity.Validate(); err != nil {
		return Amount{}, err
	}
	high, low := bits.Mul64(amountMagnitude(a.minorUnits), quantity.value)
	if high != 0 {
		return Amount{}, overflowError()
	}
	value, err := signedValue(a.minorUnits < 0, low)
	if err != nil {
		return Amount{}, err
	}
	return Amount{minorUnits: value, code: a.code}, nil
}

// ApplyBasisPoints computes the exact rate product before rounding once.
// No float or intermediate signed overflow is used.
// Sources: https://pkg.go.dev/math/bits#Mul64 and https://pkg.go.dev/math/bits#Div64
func (a Amount) ApplyBasisPoints(rate BasisPoints, rounding Rounding) (Amount, error) {
	if err := a.Validate(); err != nil {
		return Amount{}, err
	}
	if err := rate.Validate(); err != nil {
		return Amount{}, err
	}
	if err := rounding.Validate(); err != nil {
		return Amount{}, err
	}
	const denominator uint64 = 10000
	high, low := bits.Mul64(amountMagnitude(a.minorUnits), rate.value)
	if high >= denominator {
		return Amount{}, overflowError()
	}
	quotient, remainder := bits.Div64(high, low, denominator)
	negative := a.minorUnits < 0
	if roundMagnitudeUp(rounding, negative, quotient, remainder, denominator) {
		var carry uint64
		quotient, carry = bits.Add64(quotient, 1, 0)
		if carry != 0 {
			return Amount{}, overflowError()
		}
	}
	value, err := signedValue(negative, quotient)
	if err != nil {
		return Amount{}, err
	}
	return Amount{minorUnits: value, code: a.code}, nil
}

func roundMagnitudeUp(rounding Rounding, negative bool, quotient, remainder, denominator uint64) bool {
	switch rounding {
	case RoundingDown:
		return negative && remainder != 0
	case RoundingHalfUp:
		return remainder >= denominator/2
	case RoundingHalfEven:
		return remainder > denominator/2 || remainder == denominator/2 && quotient&1 != 0
	default:
		return false // Caller has validated the closed policy.
	}
}

// Allocate streams equal shares in ordinal order with O(1) retained memory.
// The first abs(remainder) shares receive one extra minor unit of the amount's
// sign. Every share differs by at most one minor unit and the complete sequence
// sums exactly to a. Zero shares are refused. Stopping iteration stops work;
// no unrequested share is retained or produced. Each traversal starts afresh.
func (a Amount) Allocate(shares Quantity) (iter.Seq[Amount], error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}
	if err := shares.Validate(); err != nil {
		return nil, err
	}
	if shares.value == 0 {
		return nil, core.ErrCurrencyContract
	}
	magnitude := amountMagnitude(a.minorUnits)
	quotient, remainder := magnitude/shares.value, magnitude%shares.value
	return func(yield func(Amount) bool) {
		for ordinal := uint64(0); ordinal < shares.value; ordinal++ {
			part := quotient
			if ordinal < remainder {
				part++
			}
			// Each magnitude is at most the admitted original magnitude.
			value := int64(part)
			if a.minorUnits < 0 {
				value = -value
			}
			if !yield(Amount{minorUnits: value, code: a.code}) {
				return
			}
		}
	}, nil
}

var (
	_ core.Validatable = Quantity{}
	_ core.Validatable = BasisPoints{}
	_ core.OffWireEnum = RoundingUnknown
	_ core.OffWireEnum = SignUnknown
)
