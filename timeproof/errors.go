package timeproof

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

func contractError(causes ...error) error {
	joined := []error{core.ErrTimeProofContract}
	for _, cause := range causes {
		if cause != nil {
			joined = append(joined, cause)
		}
	}
	return errors.Join(joined...)
}

func invalidError(causes ...error) error {
	joined := []error{core.ErrTimeProofInvalid}
	for _, cause := range causes {
		if cause != nil {
			joined = append(joined, cause)
		}
	}
	return errors.Join(joined...)
}

// errorsJSON is the single spelling of a Timeproof JSON contract violation.
func errorsJSON() error {
	return contractError(core.ErrJSONContract)
}
