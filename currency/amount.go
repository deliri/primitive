package currency

import (
	"math"

	"github.com/deliri/primitive/v2026/core"
)

// Amount is an exact signed minor-unit value inseparably bound to one currency.
type Amount struct {
	minorUnits int64
	code       Code
}

// New constructs an exact amount from signed minor units.
func New(code Code, minorUnits int64) (Amount, error) {
	if err := code.Validate(); err != nil {
		return Amount{}, err
	}
	return Amount{minorUnits: minorUnits, code: code}, nil
}

// Validate rejects the invalid unknown-currency zero value.
func (a Amount) Validate() error {
	if err := a.code.Validate(); err != nil {
		return contractError("amount has an invalid currency")
	}
	return nil
}

// Code returns the amount's validated currency.
func (a Amount) Code() (Code, error) {
	if err := a.Validate(); err != nil {
		return CodeUnknown, err
	}
	return a.code, nil
}

// MinorUnits returns the amount's exact signed minor units.
func (a Amount) MinorUnits() (int64, error) {
	if err := a.Validate(); err != nil {
		return 0, err
	}
	return a.minorUnits, nil
}

// Add returns an exact same-currency sum.
func (a Amount) Add(other Amount) (Amount, error) {
	if err := validatePair(a, other); err != nil {
		return Amount{}, err
	}
	value, err := addInt64(a.minorUnits, other.minorUnits)
	if err != nil {
		return Amount{}, err
	}
	return Amount{minorUnits: value, code: a.code}, nil
}

// Subtract returns an exact same-currency difference.
func (a Amount) Subtract(other Amount) (Amount, error) {
	if err := validatePair(a, other); err != nil {
		return Amount{}, err
	}
	value, err := subtractInt64(a.minorUnits, other.minorUnits)
	if err != nil {
		return Amount{}, err
	}
	return Amount{minorUnits: value, code: a.code}, nil
}

// Compare orders two amounts of the same currency.
func (a Amount) Compare(other Amount) (Order, error) {
	if err := validatePair(a, other); err != nil {
		return OrderUnknown, err
	}
	switch {
	case a.minorUnits < other.minorUnits:
		return OrderLess, nil
	case a.minorUnits > other.minorUnits:
		return OrderGreater, nil
	default:
		return OrderEqual, nil
	}
}

func validatePair(left, right Amount) error {
	if err := left.Validate(); err != nil {
		return err
	}
	if err := right.Validate(); err != nil {
		return err
	}
	if left.code != right.code {
		return mismatchError()
	}
	return nil
}

func addInt64(left, right int64) (int64, error) {
	if right > 0 && left > math.MaxInt64-right ||
		right < 0 && left < math.MinInt64-right {
		return 0, overflowError()
	}
	return left + right, nil
}

func subtractInt64(left, right int64) (int64, error) {
	if right > 0 && left < math.MinInt64+right ||
		right < 0 && left > math.MaxInt64+right {
		return 0, overflowError()
	}
	return left - right, nil
}

var _ core.Validatable = Amount{}
