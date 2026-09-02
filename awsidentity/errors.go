package awsidentity

import (
	"errors"
	"fmt"
	"io"

	"github.com/deliri/primitive/v2026/core"
)

const requestFailureText = "AWS identity request failed"

type requestError struct{ cause error }

func requestFailure(cause error) error { return requestError{cause: cause} }
func (e requestError) Error() string   { return requestFailureText }
func (e requestError) Unwrap() error   { return e.cause }
func (e requestError) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, requestFailureText)
}

func contractError(cause error) error {
	if cause == nil {
		return core.ErrAWSIdentityContract
	}
	return errors.Join(core.ErrAWSIdentityContract, cause)
}
