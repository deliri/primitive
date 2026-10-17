package hashio

import "crypto/subtle"

// EqualBytes delegates exact byte equality to Go's constant-time comparator.
// Unequal lengths refuse immediately, as in crypto/subtle; equal-length work
// depends on extent rather than the first differing byte. Inputs are borrowed.
func EqualBytes(first, second []byte) bool {
	return subtle.ConstantTimeCompare(first, second) == 1
}
