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
	ContentLength          SignedContentLength
	Method                 Method
	DisableURIPathEscaping bool
}

func (r V4PresignRequest) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

func (r V4PresignRequest) Validate() error {
	if err := errors.Join(r.Target.Validate(), r.Headers.Validate(), r.SignedAt.Validate(), r.Method.Validate(), r.ContentLength.Validate()); err != nil {
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

// SignedContentLength binds one exact request body extent into a presigned
// signature, so the holder cannot send a different number of bytes. The zero
// value signs no length. Transport derives the header from the body.
type SignedContentLength struct {
	bytes   core.ByteLength
	present bool
}

func NewSignedContentLength(bytes core.ByteLength) (SignedContentLength, error) {
	length := SignedContentLength{bytes: bytes, present: true}
	if err := length.Validate(); err != nil {
		return SignedContentLength{}, err
	}
	return length, nil
}

func (l SignedContentLength) Validate() error {
	if !l.present && l.bytes != (core.ByteLength{}) {
		return core.ErrExchangeRequest
	}
	return l.bytes.Validate()
}
func (l SignedContentLength) IsZero() bool           { return !l.present }
func (l SignedContentLength) Bytes() core.ByteLength { return l.bytes }

// PresignV4 constructs and signs one validated request without transferring
// custody of transport or importing provider policy into Exchange. Header
// hoisting is disabled: explicitly supplied headers must remain signed headers.
func PresignV4(ctx context.Context, intent V4PresignRequest) (core.HTTPEndpoint, error) {
	if err := errors.Join(contextstate.Validate(ctx), intent.Validate()); err != nil {
		return core.HTTPEndpoint{}, err
	}
	return v4PresignedEndpoint(ctx, intent)
}
