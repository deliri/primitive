package jsonio

import (
	"context"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"reflect"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// EncodeObject writes one validated declared struct at the token encoder's
// current value position, including inside an array or after an object key.
// It uses the same Go engine as Encode; no record sequence is aggregated.
// Source: https://pkg.go.dev/encoding/json/v2#MarshalEncode
func EncodeObject[Document core.Validatable](ctx context.Context, encoder *TokenEncoder, document Document) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if err := encoder.Validate(); err != nil {
		return err
	}
	if err := validateObjectType[Document](); err != nil {
		return err
	}
	if value := reflect.ValueOf(document); value.Kind() == reflect.Pointer && value.IsNil() {
		return core.ErrJSONContract
	}
	if err := document.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	return objectReadOutcome(ctx, jsonv2.MarshalEncode(encoder.encoder, document, jsonv2.Deterministic(true)))
}

// EncodeMarshaledObject explicitly admits a type-owned MarshalJSON contract.
// Its owner validates meaning; Go checks that the emitted bytes are exactly
// one object with valid UTF-8 and unique member names before any write.
// MarshalJSON owns a whole-value allocation. This door retains that one value
// only, never its surrounding array or sequence. Declared-field encoders keep
// rejecting opaque root hooks; opting in here does not weaken their contract.
// Source: https://pkg.go.dev/encoding/json/jsontext#Encoder.WriteValue
func EncodeMarshaledObject[Document core.ValidatedJSONMarshaler](ctx context.Context, encoder *TokenEncoder, document Document) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if err := encoder.Validate(); err != nil {
		return err
	}
	if value := reflect.ValueOf(document); value.Kind() == reflect.Pointer && value.IsNil() {
		return core.ErrJSONContract
	}
	if err := document.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	data, err := document.MarshalJSON()
	if err != nil {
		return errors.Join(err, contextstate.Validate(ctx))
	}
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	value := jsontext.Value(data)
	if value.Kind() != jsontext.KindBeginObject || !value.IsValid() {
		return core.ErrJSONContract
	}
	return objectReadOutcome(ctx, encoder.encoder.WriteValue(value))
}
