package temporal

import (
	"encoding/json"
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
	return decimal, nil
}

func parseSignedNanoseconds(decimal string) (int64, error) {
	if !canonicalSignedDecimal(decimal) {
		return 0, contractError("signed nanosecond decimal is not canonical")
	}
	value, err := strconv.ParseInt(decimal, 10, 64)
	if err != nil {
		return 0, overflowError("signed nanosecond decimal exceeded int64")
	}
	return value, nil
}

func canonicalSignedDecimal(decimal string) bool {
	if decimal == "0" {
		return true
	}
	if len(decimal) == 0 {
		return false
	}
	start := 0
	if decimal[0] == '-' {
		start = 1
	}
	if start == len(decimal) || decimal[start] == '0' {
		return false
	}
	for index := start; index < len(decimal); index++ {
		if decimal[index] < '0' || decimal[index] > '9' {
			return false
		}
	}
	return true
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
