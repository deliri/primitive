package core

import (
	"bytes"
	jsontext "encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"unicode/utf8"
)

const (
	// JSONWhitespace is the complete JSON whitespace alphabet admitted by Go's
	// JSON v2 decoder. Native byte trimming must not erase other Unicode space
	// or control bytes before that decoder observes them.
	JSONWhitespace                 = " \t\r\n"
	jsonStringInvalidUTF8ErrorText = "json string value is not valid utf-8"
	// jsonStringTokenStackBytes covers the identities and paths typical of
	// string tokens; longer tokens unquote into one heap buffer instead.
	jsonStringTokenStackBytes = 128
)

// MarshalCanonicalJSONDocument encodes one Go value as the repository's single
// canonical JSON document. It uses encoding/json/v2's compact strict
// projection, so invalid UTF-8 is rejected and direct and embedded values have
// one spelling and one byte extent.
//
// The caller owns the document invariant. This helper owns only the shared
// encoder configuration; typed protocol values still validate before calling
// it, and raw strings use MarshalCanonicalJSONString for the UTF-8 gate.
//
// This is the one owner of that mechanic. A second copy anywhere in the
// repository would let two owners disagree about the bytes a protocol carries,
// which is why MarshalCanonicalJSONString is held byte-identical to it.
func MarshalCanonicalJSONDocument[Document any](document Document) ([]byte, error) {
	encoded, err := jsonv2.Marshal(document)
	if err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return encoded, nil
}

// MarshalCanonicalJSONString encodes one Go string as the repository's single
// canonical JSON string token. It is the encoding counterpart of
// DecodeJSONStringToken: it adds the string-specific gate, refusing invalid
// UTF-8 before any protocol sees it.
//
// The token is the exact spelling MarshalCanonicalJSONDocument gives the same
// string: both use encoding/json/v2's default string quoting, which is
// jsontext.AppendQuote. Quoting directly into one buffer sized for the common
// unescaped case avoids the document encoder's boxing and copy for the most
// frequent encoding in the repository; a differential test holds the two
// spellings equal.
//
// A second string-escaping grammar anywhere in the repository would let two
// owners disagree about the bytes a signature covers, which is why this rule
// has one home instead of one copy per package.
func MarshalCanonicalJSONString(value string) ([]byte, error) {
	if !utf8.ValidString(value) {
		return nil, errors.Join(ErrJSONContract, errors.New(jsonStringInvalidUTF8ErrorText))
	}
	encoded, err := jsontext.AppendQuote(make([]byte, 0, len(value)+len(`""`)), value)
	if err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return encoded, nil
}

// DecodeJSONStringToken decodes one JSON string token into its exact Go string.
// It is the single owner of the repository's JSON string-token admission rule:
// absence, JSON null, invalid UTF-8, and unpaired surrogates are refused before
// any domain parse sees the value, so every typed enum and identity that arrives
// as a JSON string inherits one hardening contract instead of restating it.
// The owning caller validates any domain-specific extent; this decoder owns
// string syntax and encoding only.
//
// A token that jsontext.AppendUnquote admits whole, after trimming only JSON
// whitespace, is exactly a token encoding/json/v2 admits into a string with the
// same value; that common case unquotes through a stack buffer. Every other
// token takes the full decoder, which owns the refusal and its error identity.
func DecodeJSONStringToken(data []byte) (string, error) {
	if len(data) == 0 || bytes.Equal(bytes.TrimSpace(data), []byte(jsonNullLiteralText)) {
		return "", errors.Join(ErrJSONContract, errors.New("json string is absent"))
	}
	if token := bytes.Trim(data, JSONWhitespace); len(token) >= len(`""`) && token[0] == '"' {
		var buffer [jsonStringTokenStackBytes]byte
		if unquoted, err := jsontext.AppendUnquote(buffer[:0], token); err == nil {
			return string(unquoted), nil
		}
	}
	var value string
	if err := jsonv2.Unmarshal(data, &value); err != nil {
		return "", errors.Join(ErrJSONContract, err)
	}
	return value, nil
}
