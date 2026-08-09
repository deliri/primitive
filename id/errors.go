package id

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// entropyUnreadableDiagnostic is the one spelling of the entropy read
// refusal, shared by both identifier mints.
const entropyUnreadableDiagnostic = "request entropy is unreadable"

func contractError(message string) error {
	return errors.Join(core.ErrIDContract, errors.New(message))
}

func contractCause(message string, cause error) error {
	if cause == nil {
		return contractError(message)
	}
	return errors.Join(core.ErrIDContract, errors.New(message), cause)
}
