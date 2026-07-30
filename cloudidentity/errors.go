package cloudidentity

import (
	"errors"
	"fmt"
	"io"

	"github.com/deliri/primitive/v2026/core"
)

const amazonRequestFailureText = "cloud identity AWS request failed"

// amazonRequestError redacts one failed AWS request. Construction failures that
// have observed a signed capability and every failure from the outbound effect
// onward carry this shape, so no step can print the capability by omission. The
// native cause stays reachable through errors.Is and errors.As.
type amazonRequestError struct {
	cause error
}

// amazonFailure seals one AWS request failure behind the redacting shape. The
// cause already carries core.ErrCloudIdentityContract, so identity survives.
func amazonFailure(cause error) error {
	return amazonRequestError{cause: cause}
}

func (e amazonRequestError) Error() string {
	return amazonRequestFailureText
}

func (e amazonRequestError) Unwrap() error {
	return e.cause
}

func (e amazonRequestError) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, amazonRequestFailureText)
}

func contractError(cause error) error {
	if cause == nil {
		return core.ErrCloudIdentityContract
	}
	return errors.Join(core.ErrCloudIdentityContract, cause)
}
