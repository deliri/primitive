package temporal

import (
	"bytes"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"strconv"
)

func decodeNanosecondJSON(data []byte, maximumBytes int) (string, error) {
	if len(data) == 0 || len(data) > maximumBytes {
		return "", jsonContractError("temporal JSON extent is outside its bound")
	}
	var decimal string
	if err := json.Unmarshal(data, &decimal); err != nil {
		return "", jsonContractError("temporal JSON is not one string", err)
	}
	var quoted [AggregateDurationJSONMaximumBytes]byte
	canonical, err := jsontext.AppendQuote(quoted[:0], decimal)
	if err != nil || !bytes.Equal(bytes.TrimSpace(data), canonical) {
		return "", jsonContractError("temporal JSON string is not canonical", err)
	}
	return decimal, nil
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
func parseSignedNanoseconds(decimal string) (int64, error) {
	value, err := strconv.ParseInt(decimal, 10, 64)
	var canonical [NumericInstantCanonicalJSONMaximumBytes]byte
	if err != nil || string(strconv.AppendInt(canonical[:0], value, 10)) != decimal {
		return 0, contractError("signed nanosecond decimal is not canonical", err)
	}
	return value, nil
}

func canonicalUnsignedDecimal(decimal string) bool {
	if len(decimal) == 0 || len(decimal) > AggregateDurationMaximumDecimalDigits {
		return false
	}
	if decimal == "0" {
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
