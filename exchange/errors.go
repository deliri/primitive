package exchange

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/deliri/primitive/v2026/core"
)

func requestError(cause error) error {
	return errors.Join(core.ErrExchangeRequest, cause)
}

func responseError(cause error) error {
	return errors.Join(core.ErrExchangeResponse, cause)
}

func responseHeaderError(cause error) error {
	return errors.Join(core.ErrExchangeResponseHeaders, cause)
}

func transportError(cause error) error {
	if cause == nil {
		return core.ErrExchangeTransport
	}
	return errors.Join(core.ErrExchangeTransport, cause)
}

// StatusError reports a response status outside the request's expected status.
type StatusError struct {
	status   core.HTTPStatusCode
	expected core.HTTPStatusCode
}

// Error returns a bounded status-only diagnostic.
func (e StatusError) Error() string {
	status, _ := e.status.Int()
	expected, _ := e.expected.Int()
	return fmt.Sprintf("exchange status %d, expected %d", status, expected)
}

// Unwrap exposes the provider status family, independently of malformed
// response contracts. The native status remains available through Status.
func (e StatusError) Unwrap() error {
	status, err := e.status.Int()
	if err != nil {
		return core.ErrExchangeResponse
	}
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return core.ErrExchangeAuthentication
	case status == http.StatusTooManyRequests:
		return core.ErrExchangeRateLimited
	case status == http.StatusRequestTimeout || status == http.StatusTooEarly || status >= http.StatusInternalServerError:
		return core.ErrExchangeUnavailable
	case status >= http.StatusBadRequest:
		return core.ErrExchangeRejected
	}
	return core.ErrExchangeResponse
}

// Status returns the observed status.
func (e StatusError) Status() core.HTTPStatusCode {
	return e.status
}

// Expected returns the expected status.
func (e StatusError) Expected() core.HTTPStatusCode {
	return e.expected
}

// RetryExhaustedError reports that every permitted attempt failed.
type RetryExhaustedError struct {
	cause    error
	attempts uint64
}

// Error returns a bounded attempt-count diagnostic.
func (e RetryExhaustedError) Error() string {
	return fmt.Sprintf("exchange retry budget exhausted after %d attempts", e.attempts)
}

// Unwrap preserves retry exhaustion and the final stable cause.
func (e RetryExhaustedError) Unwrap() error {
	return errors.Join(core.ErrExchangeRetryExhausted, e.cause)
}

// Attempts returns the number of completed attempts.
func (e RetryExhaustedError) Attempts() uint64 {
	return e.attempts
}

// Cause returns the final contained cause.
func (e RetryExhaustedError) Cause() error {
	return e.cause
}
