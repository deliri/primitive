package exchange

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// V4PresignRequest binds the exact request to the caller-owned signing scope.
// It makes no network request, observes no clock, and never exports a raw Go
// request. Provider policy owns expiry, query names, regions and service names.
// Go SHA-256 and HMAC own cryptography; canonical input remains borrowed.
type V4PresignRequest struct {
	Region                 string
	Service                string
	PayloadHash            string
	ContentType            core.HTTPMediaType
	AccessKey              []byte
	SecretKey              []byte
	Headers                Headers
	Target                 core.HTTPEndpoint
	SignedAt               temporal.Instant
	Method                 Method
	DisableURIPathEscaping bool
}

func (r V4PresignRequest) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

func (r V4PresignRequest) Validate() error {
	if err := errors.Join(r.Target.Validate(), r.Headers.Validate(), r.SignedAt.Validate(), r.Method.Validate()); err != nil {
		return requestError(err)
	}
	if len(r.AccessKey) == 0 || len(r.SecretKey) == 0 || r.Region == "" || r.Service == "" || r.PayloadHash == "" {
		return core.ErrExchangeRequest
	}
	if r.Target.HTTPURL().Scheme != core.SchemeHTTPS || strings.ContainsAny(r.Region+r.Service, "/\r\n") {
		return core.ErrExchangeRequest
	}
	if !r.ContentType.IsZero() {
		return r.ContentType.Validate()
	}
	return nil
}

// PresignV4 constructs and signs one validated request without transferring
// custody of transport or importing provider policy into Exchange. Header
// hoisting is disabled: explicitly supplied headers must remain signed headers.
func PresignV4(ctx context.Context, intent V4PresignRequest) (core.HTTPEndpoint, error) {
	if err := errors.Join(contextstate.Validate(ctx), intent.Validate()); err != nil {
		return core.HTTPEndpoint{}, err
	}
	return v4PresignedEndpoint(ctx, intent)
}
