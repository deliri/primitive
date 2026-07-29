package currency

import (
	"bytes"
	"encoding/json"
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
		return decimalError("currency minor units are unset")
	}
	return nil
}

func (u minorUnitsJSON) MarshalJSON() ([]byte, error) {
	if err := u.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return json.Marshal(strconv.FormatInt(u.value, 10))
}

func (u *minorUnitsJSON) UnmarshalJSON(data []byte) error {
	if u == nil {
		return jsonError(decimalError("currency minor-unit receiver is nil"))
	}
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return jsonError(errors.Join(decimalError("currency minor units must be a JSON string"), err))
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || strconv.FormatInt(value, 10) != raw {
		return jsonError(decimalError("currency minor units are not a canonical int64"))
	}
	*u = newMinorUnitsJSON(value)
	return nil
}

type amountWire struct {
	Currency   Code           `json:"currency"`
	MinorUnits minorUnitsJSON `json:"minor_units"`
}

func (w *amountWire) UnmarshalJSON(data []byte) error {
	if w == nil {
		return contractError("currency amount wire receiver is nil")
	}
	decoded, err := decodeAmountWire(data)
	if err != nil {
		return err
	}
	if err := decoded.Validate(); err != nil {
		return err
	}
	*w = decoded
	return nil
}

func decodeAmountWire(data []byte) (amountWire, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	open, err := decoder.Token()
	if err != nil || open != json.Delim('{') {
		return amountWire{}, errors.Join(
			contractError("currency amount wire is not an object"),
			err,
		)
	}
	var decoded amountWire
	for decoder.More() {
		if err := decodeAmountWireField(decoder, &decoded); err != nil {
			return amountWire{}, err
		}
	}
	closeObject, err := decoder.Token()
	if err != nil || closeObject != json.Delim('}') {
		return amountWire{}, errors.Join(
			contractError("currency amount wire is not closed"),
			err,
		)
	}
	return decoded, nil
}

func decodeAmountWireField(
	decoder *json.Decoder,
	decoded *amountWire,
) error {
	field, err := decoder.Token()
	if err != nil {
		return errors.Join(
			contractError("currency amount wire field is invalid"),
			err,
		)
	}
	name, ok := field.(string)
	if !ok {
		return contractError("currency amount wire field is not a string")
	}
	switch name {
	case JSONFieldCurrency:
		err = decoder.Decode(&decoded.Currency)
	case JSONFieldMinorUnits:
		err = decoder.Decode(&decoded.MinorUnits)
	default:
		return contractError("currency amount wire field is not admitted")
	}
	if err != nil {
		return errors.Join(
			contractError("currency amount wire value is invalid"),
			err,
		)
	}
	return nil
}

func (w amountWire) Validate() error {
	if err := w.Currency.Validate(); err != nil {
		return err
	}
	return w.MinorUnits.Validate()
}

func (w amountWire) MarshalJSON() ([]byte, error) {
	if err := w.Validate(); err != nil {
		return nil, jsonError(err)
	}
	currencyJSON, err := w.Currency.MarshalJSON()
	if err != nil {
		return nil, err
	}
	minorUnits, err := w.MinorUnits.MarshalJSON()
	if err != nil {
		return nil, err
	}
	document := []byte{'{'}
	document = strconv.AppendQuote(document, JSONFieldCurrency)
	document = append(document, ':')
	document = append(document, currencyJSON...)
	document = append(document, ',')
	document = strconv.AppendQuote(document, JSONFieldMinorUnits)
	document = append(document, ':')
	document = append(document, minorUnits...)
	document = append(document, '}')
	return document, nil
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
	limits, err := amountJSONLimits()
	if err != nil {
		return nil, jsonError(err)
	}
	return core.EncodeValidatedJSON(amountWire{
		Currency:   a.code,
		MinorUnits: newMinorUnitsJSON(a.minorUnits),
	}, limits)
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
	wire, err := core.DecodeStrictJSON[amountWire](data, limits)
	if err != nil {
		return jsonError(errors.Join(
			contractError("currency amount JSON is invalid"),
			err,
		))
	}
	*a = Amount{minorUnits: wire.MinorUnits.value, code: wire.Currency}
	return nil
}

var _ core.ValidatedJSONMarshaler = amountWire{}
