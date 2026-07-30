package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"unicode/utf8"
)

const jsonEncoderTerminatorByte = '\n'

func marshalJSONString(value string) ([]byte, error) {
	if !utf8.ValidString(value) {
		return nil, errors.Join(ErrJSONContract, errors.New("json string value is not valid utf-8"))
	}
	var document bytes.Buffer
	encoder := json.NewEncoder(&document)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	encoded := document.Bytes()
	if len(encoded) == 0 || encoded[len(encoded)-1] != jsonEncoderTerminatorByte {
		return nil, errors.Join(ErrJSONContract, errors.New("json string encoder omitted its terminator"))
	}
	return encoded[:len(encoded)-1], nil
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
