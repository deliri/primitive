package core

import (
	"bytes"
	jsonv2 "encoding/json/v2"
	"errors"
	"unicode/utf8"
)

const (
	jsonStringInvalidUTF8ErrorText = "json string value is not valid utf-8"
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
// which is why MarshalCanonicalJSONString projects from it instead of
// repeating it.
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
// UTF-8 before any protocol sees it, and takes the canonical encoding itself
// from MarshalCanonicalJSONDocument.
//
// A second string-escaping grammar anywhere in the repository would let two
// owners disagree about the bytes a signature covers, which is why this rule
// has one home instead of one copy per package.
func MarshalCanonicalJSONString(value string) ([]byte, error) {
	if !utf8.ValidString(value) {
		return nil, errors.Join(ErrJSONContract, errors.New(jsonStringInvalidUTF8ErrorText))
	}
	return MarshalCanonicalJSONDocument(value)
}

// DecodeJSONStringToken decodes one JSON string token into its exact Go string.
// It is the single owner of the repository's JSON string-token admission rule:
// absence, JSON null, invalid UTF-8, and unpaired surrogates are refused before
// any domain parse sees the value, so every typed enum and identity that arrives
// as a JSON string inherits one hardening contract instead of restating it.
// It enforces the shared JSON document ceiling before scanning or decoding;
// the owning caller remains responsible for any tighter domain-specific cap.
func DecodeJSONStringToken(data []byte) (string, error) {
	var value string
	if len(data) > JSONDocumentMaximumBytes {
		return "", jsonContractError(jsonDocumentLimitExceededErrorText, nil)
	}
	if len(data) == 0 || bytes.Equal(bytes.TrimSpace(data), []byte(jsonNullLiteralText)) {
		return "", errors.Join(ErrJSONContract, errors.New("json string is absent"))
	}
	if err := jsonv2.Unmarshal(data, &value); err != nil {
		return "", errors.Join(ErrJSONContract, err)
	}
	return value, nil
}
