package compass

import (
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
)

// Decode reads one strict project configuration into the project-owned type T.
// Unknown or duplicated members, malformed JSON, and a rejected T.Validate
// return the zero T with typed Compass identity. Compass imposes no document
// byte or array count quota. Core's strict decoder retains the complete document;
// memory therefore grows with the input and the returned configuration.
func Decode[T core.Validatable](reader io.Reader) (T, error) {
	value, err := core.DecodeStrictJSON[T](reader, core.ExtensibleJSONLimits())
	if err != nil {
		var zero T
		return zero, errors.Join(core.ErrCompassContract, err)
	}
	return value, nil
}
