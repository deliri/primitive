package temporal

import (
	"bytes"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"strconv"
)

// decodeNanosecondJSON returns the decimal text of one canonical JSON string.
// The returned bytes may alias data.
func decodeNanosecondJSON(data []byte, maximumBytes int) ([]byte, error) {
	if len(data) == 0 || len(data) > maximumBytes {
		return nil, jsonContractError("temporal JSON extent is outside its bound")
	}
	if decimal, ok := plainDecimalJSONString(data); ok {
		return decimal, nil
	}
	var decimal string
	if err := json.Unmarshal(data, &decimal); err != nil {
		return nil, jsonContractError("temporal JSON is not one string", err)
	}
	var quoted [AggregateDurationJSONMaximumBytes]byte
	canonical, err := jsontext.AppendQuote(quoted[:0], decimal)
	if err != nil || !bytes.Equal(bytes.TrimSpace(data), canonical) {
		return nil, jsonContractError("temporal JSON string is not canonical", err)
	}
	return []byte(decimal), nil
}

// plainDecimalJSONString recognizes the only spelling a nanosecond decimal can
// have and still pass the canonical check: JSON whitespace, a quote, sign and
// digit bytes, a quote, JSON whitespace. Such a string decodes to its literal
// interior and AppendQuote reproduces it byte for byte, so this is exactly
// what the general json.Unmarshal path returns for the same input. Anything
// else takes that general path, which owns every refusal.
func plainDecimalJSONString(data []byte) ([]byte, bool) {
	start, end := 0, len(data)
	for start < end && isJSONWhitespace(data[start]) {
		start++
	}
	for end > start && isJSONWhitespace(data[end-1]) {
		end--
	}
	if end-start < 2 || data[start] != '"' || data[end-1] != '"' {
		return nil, false
	}
	interior := data[start+1 : end-1]
	for _, character := range interior {
		if character != '-' && (character < '0' || character > '9') {
			return nil, false
		}
	}
	return interior, true
}

func isJSONWhitespace(character byte) bool {
	return character == ' ' || character == '\t' || character == '\n' || character == '\r'
}

// appendNanosecondJSON appends the canonical JSON string spelling of signed
// nanoseconds. Decimal digits and the sign need no JSON escaping, so this is
// byte-for-byte what encoding/json emits for strconv.FormatInt's text.
func appendNanosecondJSON(destination []byte, nanoseconds int64) []byte {
	destination = append(destination, '"')
	destination = strconv.AppendInt(destination, nanoseconds, 10)
	return append(destination, '"')
}

// decodeNumericNanoseconds admits one bare JSON number. Unlike the string
// projection it allows no insignificant whitespace: encoding/json hands a
// member's exact literal bytes to UnmarshalJSON, so one value keeps exactly one
// accepted encoding.
func decodeNumericNanoseconds(data []byte, maximumBytes int) (int64, error) {
	if len(data) == 0 || len(data) > maximumBytes {
		return 0, jsonContractError("temporal numeric JSON extent is outside its bound")
	}
	value, err := parseSignedNanoseconds(string(data))
	if err != nil {
		return 0, jsonContractError("temporal numeric JSON decimal is invalid", err)
	}
	return value, nil
}

// parseSignedNanoseconds admits exactly the decimal spelling strconv emits for
// one signed nanosecond value. Temporal is the sole owner of this persistence
// grammar; the standard library remains the parser and canonical projector.
func parseSignedNanoseconds[Text ~string | ~[]byte](decimal Text) (int64, error) {
	value, err := strconv.ParseInt(string(decimal), 10, 64)
	var canonical [NumericInstantCanonicalJSONMaximumBytes]byte
	if err != nil || string(strconv.AppendInt(canonical[:0], value, 10)) != string(decimal) {
		return 0, contractError("signed nanosecond decimal is not canonical", err)
	}
	return value, nil
}

func canonicalUnsignedDecimal[Text ~string | ~[]byte](decimal Text) bool {
	if len(decimal) == 0 || len(decimal) > AggregateDurationMaximumDecimalDigits {
		return false
	}
	if string(decimal) == "0" {
		return true
	}
	if decimal[0] == '0' {
		return false
	}
	for index := range len(decimal) {
		if decimal[index] < '0' || decimal[index] > '9' {
			return false
		}
	}
	return true
}
