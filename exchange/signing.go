package exchange

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// V4PresignRequest binds the exact request to the caller-owned signing scope.
// It makes no network request, observes no clock, and never exports a raw Go
// request. Provider policy owns expiry, query names, regions and service names.
// The official signer owns SigV4: https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/aws/signer/v4
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
	request, err := http.NewRequestWithContext(ctx, intent.Method.String(), intent.Target.String(), nil)
	if err != nil {
		return core.HTTPEndpoint{}, requestError(err)
	}
	applyRequestHeaders(request, intent.Headers)
	if !intent.ContentType.IsZero() {
		request.Header.Set(core.HTTPHeaderContentType().String(), intent.ContentType.String())
	}
	stamp, err := intent.SignedAt.Time()
	if err != nil {
		return core.HTTPEndpoint{}, err
	}
	signer := v4.NewSigner(func(options *v4.SignerOptions) {
		options.DisableURIPathEscaping = intent.DisableURIPathEscaping
		options.DisableHeaderHoisting = true
	})
	endpoint, _, err := signer.PresignHTTP(ctx, aws.Credentials{AccessKeyID: string(intent.AccessKey), SecretAccessKey: string(intent.SecretKey)}, request, intent.PayloadHash, intent.Service, intent.Region, stamp)
	if err != nil {
		return core.HTTPEndpoint{}, requestError(err)
	}
	if err := contextstate.Validate(ctx); err != nil {
		return core.HTTPEndpoint{}, err
	}
	return core.ParseHTTPEndpoint(endpoint)
}
