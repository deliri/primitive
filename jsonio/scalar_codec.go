package jsonio

import (
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"io"

	"github.com/deliri/primitive/v2026/core"
)

// MarshalScalar emits exactly one compact native JSON scalar without a final
// newline. Text and returned bytes are data; TokenKind owns scalar selection.
// Whole-value storage belongs to the caller. Use TokenEncoder for sequences.
// Sources: https://pkg.go.dev/encoding/json/v2#Marshal
// https://pkg.go.dev/encoding/json/jsontext#Value
func MarshalScalar(token Token) ([]byte, error) {
	if err := token.Validate(); err != nil {
		return nil, err
	}
	var data []byte
	var err error
	switch token.Kind {
	case TokenString:
		data, err = jsonv2.Marshal(token.Text)
	case TokenNumber:
		data, err = jsonv2.Marshal(jsontext.Value(token.Text))
	case TokenTrue:
		data, err = jsonv2.Marshal(true)
	case TokenFalse:
		data, err = jsonv2.Marshal(false)
	case TokenNull:
		data, err = jsonv2.Marshal(jsontext.Value(nil))
	default:
		return nil, core.ErrJSONContract
	}
	if err != nil {
		return nil, jsonOperationError(err)
	}
	return data, nil
}

// DecodeScalar admits exactly one native scalar, preserving decoded strings
// and exact number spelling. Arrays, objects and trailing values are refused.
// Source: https://pkg.go.dev/encoding/json/jsontext#Decoder.ReadToken
func DecodeScalar(data []byte) (Token, error) {
	decoder := borrowedBytesDecoder(data)
	native, err := decoder.ReadToken()
	if err != nil {
		return Token{}, jsonOperationError(err)
	}
	token, err := projectToken(native)
	if err != nil {
		return Token{}, err
	}
	if token.Kind < TokenString || token.Kind > TokenNull {
		return Token{}, core.ErrJSONContract
	}
	if _, err := decoder.ReadToken(); err != io.EOF {
		if err == nil {
			return Token{}, core.ErrJSONContract
		}
		return Token{}, jsonOperationError(err)
	}
	return token, nil
}

// MarshalText quotes rendered text through Go, without a top-level newline.
func MarshalText(text string) ([]byte, error) {
	return MarshalScalar(Token{Kind: TokenString, Text: text})
}

// DecodeText admits one JSON string, including the neutral empty string.
func DecodeText(data []byte) (string, error) {
	token, err := DecodeScalar(data)
	if err != nil {
		return "", err
	}
	if token.Kind != TokenString {
		return "", core.ErrJSONContract
	}
	return token.Text, nil
}

// MarshalBoolean emits one compact native JSON boolean.
func MarshalBoolean(value bool) ([]byte, error) {
	kind := TokenFalse
	if value {
		kind = TokenTrue
	}
	return MarshalScalar(Token{Kind: kind})
}

// DecodeBoolean refuses every non-boolean scalar and preserves false as data.
func DecodeBoolean(data []byte) (bool, error) {
	token, err := DecodeScalar(data)
	if err != nil {
		return false, err
	}
	if token.Kind != TokenFalse && token.Kind != TokenTrue {
		return false, core.ErrJSONContract
	}
	return token.Kind == TokenTrue, nil
}

// MarshalInteger emits a signed integer without floating-point conversion.
func MarshalInteger(value int64) ([]byte, error) {
	data, err := jsonv2.Marshal(value)
	if err != nil {
		return nil, jsonOperationError(err)
	}
	return data, nil
}

// DecodeInteger delegates signed integer grammar and range to Go.
func DecodeInteger(data []byte) (int64, error) {
	token, err := DecodeScalar(data)
	if err != nil {
		return 0, err
	}
	if token.Kind != TokenNumber {
		return 0, core.ErrJSONContract
	}
	var value int64
	if err := jsonv2.Unmarshal(data, &value); err != nil {
		return 0, jsonOperationError(err)
	}
	return value, nil
}

// MarshalUnsignedInteger preserves the full native uint64 range.
func MarshalUnsignedInteger(value uint64) ([]byte, error) {
	data, err := jsonv2.Marshal(value)
	if err != nil {
		return nil, jsonOperationError(err)
	}
	return data, nil
}

// DecodeUnsignedInteger delegates unsigned integer grammar and range to Go.
func DecodeUnsignedInteger(data []byte) (uint64, error) {
	token, err := DecodeScalar(data)
	if err != nil {
		return 0, err
	}
	if token.Kind != TokenNumber {
		return 0, core.ErrJSONContract
	}
	var value uint64
	if err := jsonv2.Unmarshal(data, &value); err != nil {
		return 0, jsonOperationError(err)
	}
	return value, nil
}
