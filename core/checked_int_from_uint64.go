package core

import "math"

// CheckedIntFromUint64 converts value or returns ErrNumericOverflow when it
// exceeds the native int width of the current platform.
func CheckedIntFromUint64(value uint64) (int, error) {
	if value > uint64(math.MaxInt) {
		return 0, numericOverflow("uint64 does not fit int")
	}
	return int(value), nil
}
