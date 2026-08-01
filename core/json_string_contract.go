package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"unicode/utf8"
)

const (
	jsonEncoderTerminatorByte      = '\n'
	jsonEncoderTerminatorErrorText = "json encoder omitted its terminator"
	jsonStringInvalidUTF8ErrorText = "json string value is not valid utf-8"
)

// MarshalCanonicalJSONDocument encodes one Go value as the repository's single
// canonical JSON document. HTML escaping is off, so a value has exactly one
// accepted spelling rather than one spelling per encoder setting, and the
// encoder's trailing terminator is removed so the result is exactly the
// document and nothing else.
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
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(document); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	encoded := buffer.Bytes()
	if len(encoded) == 0 || encoded[len(encoded)-1] != jsonEncoderTerminatorByte {
		return nil, errors.Join(ErrJSONContract, errors.New(jsonEncoderTerminatorErrorText))
	}
	return encoded[:len(encoded)-1], nil
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
// It does not impose a domain-specific byte ceiling; the owning caller must
// reject an over-extent document before calling it.
func DecodeJSONStringToken(data []byte) (string, error) {
	var value string
	if len(data) == 0 || bytes.Equal(bytes.TrimSpace(data), []byte(jsonNullLiteralText)) {
		return "", errors.Join(ErrJSONContract, errors.New("json string is absent"))
	}
	if !utf8.Valid(data) {
		return "", errors.Join(ErrJSONContract, errors.New("json string is not valid utf-8"))
	}
	if err := rejectUnpairedJSONSurrogates(data); err != nil {
		return "", err
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return "", errors.Join(ErrJSONContract, err)
	}
	return value, nil
}

const (
	jsonUnicodeEscapeLength            = 6
	jsonHighSurrogateMinimum           = rune(0xd800)
	jsonHighSurrogateMaximum           = rune(0xdbff)
	jsonLowSurrogateMinimum            = rune(0xdc00)
	jsonLowSurrogateMaximum            = rune(0xdfff)
	jsonUnicodeEscapeMarker            = 'u'
	jsonStringEscapeMarker             = '\\'
	jsonStringDelimiterMarker          = '"'
	jsonUnpairedLowSurrogateErrorText  = "json string contains an unpaired low surrogate"
	jsonUnpairedHighSurrogateErrorText = "json string contains an unpaired high surrogate"
)

func rejectUnpairedJSONSurrogates(data []byte) error {
	inString := false
	for index := 0; index < len(data); index++ {
		switch data[index] {
		case jsonStringDelimiterMarker:
			inString = !inString
		case jsonStringEscapeMarker:
			skip, err := scanJSONSurrogateEscape(data, index, inString)
			if err != nil {
				return err
			}
			index += skip
		}
	}
	return nil
}

func scanJSONSurrogateEscape(data []byte, index int, inString bool) (int, error) {
	if !inString || index+1 >= len(data) {
		return 0, nil
	}
	if data[index+1] != jsonUnicodeEscapeMarker {
		return 1, nil
	}
	first, ok := parseJSONCodeUnit(data, index)
	if !ok {
		return 0, nil
	}
	if isJSONLowSurrogate(first) {
		return 0, jsonContractError(jsonUnpairedLowSurrogateErrorText, nil)
	}
	if !isJSONHighSurrogate(first) {
		return jsonUnicodeEscapeLength - 1, nil
	}
	return scanJSONHighSurrogatePair(data, index)
}

func scanJSONHighSurrogatePair(data []byte, index int) (int, error) {
	secondIndex := index + jsonUnicodeEscapeLength
	second, secondOK := parseJSONCodeUnit(data, secondIndex)
	if !secondOK || !isJSONLowSurrogate(second) {
		return 0, jsonContractError(jsonUnpairedHighSurrogateErrorText, nil)
	}
	return jsonUnicodeEscapeLength*2 - 1, nil
}

func parseJSONCodeUnit(data []byte, escapeIndex int) (rune, bool) {
	if escapeIndex+jsonUnicodeEscapeLength > len(data) ||
		data[escapeIndex] != jsonStringEscapeMarker ||
		data[escapeIndex+1] != jsonUnicodeEscapeMarker {
		return 0, false
	}
	var value rune
	for _, digit := range data[escapeIndex+2 : escapeIndex+jsonUnicodeEscapeLength] {
		part, ok := jsonHexDigitValue(digit)
		if !ok {
			return 0, false
		}
		value <<= 4
		value += part
	}
	return value, true
}

func jsonHexDigitValue(digit byte) (rune, bool) {
	switch {
	case digit >= '0' && digit <= '9':
		return rune(digit - '0'), true
	case digit >= 'a' && digit <= 'f':
		return rune(digit-'a') + 10, true
	case digit >= 'A' && digit <= 'F':
		return rune(digit-'A') + 10, true
	default:
		return 0, false
	}
}

func isJSONHighSurrogate(value rune) bool {
	return value >= jsonHighSurrogateMinimum && value <= jsonHighSurrogateMaximum
}

func isJSONLowSurrogate(value rune) bool {
	return value >= jsonLowSurrogateMinimum && value <= jsonLowSurrogateMaximum
}
