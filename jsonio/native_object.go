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
func MarshalNativeObject[Document ~map[string]any](document Document) ([]byte, error) {
	data, err := jsonv2.Marshal(document, jsonv2.Deterministic(true))
	if err != nil {
		return nil, jsonOperationError(err)
	}
	if jsontext.Value(data).Kind() != jsontext.KindBeginObject {
		return nil, core.ErrJSONContract
	}
	return data, nil
}

// DecodeNativeIntegerObject admits a complete SDK-native object while lending
// Go only its integer-number conversion. Every number must be a signed int64
// integer token: decimal fractions, exponents and overflow refuse. Go owns
// objects, arrays, duplicate names, Unicode and error provenance. A failed
// decode publishes no partially populated native object.
// Source: https://pkg.go.dev/encoding/json/v2#UnmarshalFromFunc
func DecodeNativeIntegerObject[Document ~map[string]any](data []byte) (Document, error) {
	var zero Document
	if err := ValidateBytes(data); err != nil {
		return zero, err
	}
	if jsontext.Value(data).Kind() != jsontext.KindBeginObject {
		return zero, core.ErrJSONContract
	}
	var document Document
	if err := jsonv2.Unmarshal(data, &document, jsonv2.WithUnmarshalers(nativeIntegerNumbers)); err != nil {
		return zero, jsonOperationError(err)
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
