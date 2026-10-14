package ai

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func contractError(cause error) error {
	if errors.Is(cause, core.ErrAIBinding) || errors.Is(cause, core.ErrAIAuthentication) || errors.Is(cause, core.ErrAIContract) ||
		errors.Is(cause, core.ErrAIRateLimited) || errors.Is(cause, core.ErrAIUnavailable) || errors.Is(cause, core.ErrAIRequestRejected) {
		return cause
	}
	return errors.Join(core.ErrAIContract, cause)
}

// Native status classification preserves Exchange's StatusError and any
// partial-stream cause. Error payloads go to the caller's FailureDestination;
// the SDK never aggregates or replaces provider type/message bytes.
// https://platform.claude.com/docs/en/api/errors
// https://developers.openai.com/api/docs/guides/error-codes
func providerError(cause error) error {
	var status exchange.StatusError
	if !errors.As(cause, &status) {
		return cause
	}
	value, err := status.Status().Int()
	if err != nil {
		return errors.Join(core.ErrAIContract, cause, err)
	}
	switch {
	case value == core.AIStatusAuthentication:
		return errors.Join(core.ErrAIAuthentication, cause)
	case value == core.AIStatusRateLimited:
		return errors.Join(core.ErrAIRateLimited, cause)
	case value == core.AIStatusTimeout || value >= core.AIStatusServiceFailure:
		return errors.Join(core.ErrAIUnavailable, cause)
	default:
		return errors.Join(core.ErrAIRequestRejected, cause)
	}
}
