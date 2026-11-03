package jsonio

import (
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"strconv"

	"github.com/deliri/primitive/v2026/core"
)

// MarshalNativeObject spells the Go object materialized by a native SDK.
// This mechanical edge owns neither a product schema nor its validation.
// Product boundaries use the concrete projection/owned doors instead. Go owns
// ordering, Unicode, scalar spelling and nested custom codecs; no parallel JSON
// formatter or object model is constructed here. A nil root is refused.
// Source: https://pkg.go.dev/encoding/json/v2#Marshal
func MarshalNativeObject(document map[string]any) ([]byte, error) {
	if document == nil {
		return nil, core.ErrJSONContract
	}
	data, err := jsonv2.Marshal(document, jsonv2.Deterministic(true))
	if err != nil {
		return nil, jsonOperationError(err)
	}
	return data, nil
}

// DecodeNativeIntegerObject admits a complete SDK-native object while lending
// Go only its integer-number conversion. Every number must be a signed int64
// integer token: decimal fractions, exponents and overflow refuse. Go owns
// objects, arrays, duplicate names, Unicode and error provenance. A failed
// decode publishes no partially populated native object.
// Source: https://pkg.go.dev/encoding/json/v2#UnmarshalFromFunc
func DecodeNativeIntegerObject(data []byte) (map[string]any, error) {
	if err := ValidateBytes(data); err != nil {
		return nil, err
	}
	var document map[string]any
	if err := jsonv2.Unmarshal(data, &document, jsonv2.WithUnmarshalers(nativeIntegerNumbers)); err != nil {
		return nil, jsonOperationError(err)
	}
	if document == nil {
		return nil, core.ErrJSONContract
	}
	return document, nil
}

var nativeIntegerNumbers = jsonv2.UnmarshalFromFunc(func(decoder *jsontext.Decoder, target *any) error {
	if decoder.PeekKind() != jsontext.KindNumber {
		return errors.ErrUnsupported
	}
	value, err := decoder.ReadValue()
	if err != nil {
		return err
	}
	integer, err := strconv.ParseInt(string(value), 10, 64)
	if err != nil {
		return err
	}
	*target = integer
	return nil
})
