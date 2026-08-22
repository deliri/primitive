package currency

import (
	"bytes"
	"errors"
	"strconv"

	"github.com/deliri/primitive/v2026/core"
)

type minorUnitsJSON struct {
	value int64
	set   bool
}

func newMinorUnitsJSON(value int64) minorUnitsJSON {
	return minorUnitsJSON{value: value, set: true}
}

func (u minorUnitsJSON) Validate() error {
	if !u.set {
		return decimalError(decimalRejectionMinorUnitsUnset)
	}
	return nil
}

func (u minorUnitsJSON) MarshalJSON() ([]byte, error) {
	if err := u.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return core.MarshalCanonicalJSONString(strconv.FormatInt(u.value, 10))
}

func (u *minorUnitsJSON) UnmarshalJSON(data []byte) error {
	if u == nil {
		return jsonError(decimalError(decimalRejectionReceiverNil))
	}
	raw, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return jsonError(errors.Join(decimalError(decimalRejectionJSONString), err))
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || strconv.FormatInt(value, 10) != raw {
		return jsonError(decimalError(decimalRejectionCanonicalInt64))
	}
	*u = newMinorUnitsJSON(value)
	return nil
}

type amountWire struct {
	Currency   Code           `json:"currency"`
	MinorUnits minorUnitsJSON `json:"minor_units"`
}

func (w amountWire) Validate() error {
	if err := w.Currency.Validate(); err != nil {
		return err
	}
	return w.MinorUnits.Validate()
}

func amountJSONLimits() (core.StrictJSONLimits, error) {
	maximum, err := core.NewByteCount(AmountJSONMaximumBytes)
	if err != nil {
		return core.StrictJSONLimits{}, err
	}
	return core.StrictJSONLimits{
		DocumentMaximumBytes: maximum,
		NestingDepthMaximum:  1,
		ObjectFieldMaximum:   2,
		ArrayItemMaximum:     1,
	}, nil
}

// MarshalJSON emits the closed exact amount object.
func (a Amount) MarshalJSON() ([]byte, error) {
	if err := a.Validate(); err != nil {
		return nil, jsonError(err)
	}
	wire := amountWire{
		Currency:   a.code,
		MinorUnits: newMinorUnitsJSON(a.minorUnits),
	}
	if err := wire.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(wire)
	if err != nil {
		return nil, jsonError(err)
	}
	if len(encoded) > AmountCanonicalJSONMaximumBytes {
		return nil, jsonError(contractError("canonical currency amount exceeds its byte limit"))
	}
	return encoded, nil
}

// UnmarshalJSON accepts the closed amount object and preserves the receiver on failure.
func (a *Amount) UnmarshalJSON(data []byte) error {
	if a == nil {
		return jsonError(contractError("currency amount receiver is nil"))
	}
	limits, err := amountJSONLimits()
	if err != nil {
		return jsonError(err)
	}
	wire, err := core.DecodeStrictJSON[amountWire](bytes.NewReader(data), limits)
	if err != nil {
		return jsonError(errors.Join(
			contractError("currency amount JSON is invalid"),
			err,
		))
	}
	*a = Amount{minorUnits: wire.MinorUnits.value, code: wire.Currency}
	return nil
}
