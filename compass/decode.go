package compass

import (
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
)

// DocumentMaximumBytes bounds every project Compass document before decoding.
const DocumentMaximumBytes uint64 = 1 << 20

// Decode reads one bounded strict project configuration into the project-owned
// type T. Unknown or duplicated members, malformed JSON, oversized input, and
// a rejected T.Validate all return the zero T with typed Compass identity.
func Decode[T core.Validatable](reader io.Reader) (T, error) {
	maximum, err := core.NewByteCount(DocumentMaximumBytes)
	if err != nil {
		var zero T
		return zero, contractError("configuration byte bound is invalid", err)
	}
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = maximum
	value, err := core.DecodeStrictJSON[T](reader, limits)
	if err != nil {
		var zero T
		return zero, errors.Join(core.ErrCompassContract, err)
	}
	return value, nil
}
