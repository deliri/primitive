package temporal

import (
	"encoding/json"

	"github.com/deliri/primitive/v2026/core"
)

func decodeNanosecondJSON(data []byte, maximumBytes int) (string, error) {
	if len(data) == 0 || len(data) > maximumBytes {
		return "", jsonContractError("temporal JSON extent is outside its bound")
	}
	var decimal string
	if err := json.Unmarshal(data, &decimal); err != nil {
		return "", jsonContractError("temporal JSON is not one string", err)
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
	return parseSignedNanoseconds(string(data))
}

// parseSignedNanoseconds projects Core's canonical-integer owner. Temporal
// previously carried its own hand-written signed-decimal grammar beside Core's
// round-trip rule; two grammars for one fact is the duplication this package
// exists to remove, so the rule now has exactly one home.
func parseSignedNanoseconds(decimal string) (int64, error) {
	value, err := core.ParseCanonicalInt64JSON([]byte(decimal))
	if err != nil {
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
