package searchconsole

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const sitesEndpoint = "https://www.googleapis.com/webmasters/v3/sites/"
const failureMaximum = 16 * 1024

// AccessSource lends an independently closable Search Console read-only
// bearer. A Client closes it after a single operation. Implementations may use
// an attached service account or a separately consented user credential.
type AccessSource interface {
	BorrowReadAccess(context.Context) (core.OAuthClientSecret, error)
}

// Client borrows an Exchange and a purpose-scoped access source.
type Client struct {
	network exchange.Client
	source  AccessSource
}

func NewClient(network exchange.Client, source AccessSource) (Client, error) {
	if source == nil {
		return Client{}, core.ErrSearchConsoleContract
	}
	if err := network.Validate(); err != nil {
		return Client{}, errors.Join(core.ErrSearchConsoleContract, err)
	}
	return Client{network: network, source: source}, nil
}

func (c Client) Query(ctx context.Context, r QueryRequest) (QueryPage, error) {
	if err := r.Validate(); err != nil {
		return QueryPage{}, err
	}
	payload, err := encodeQuery(r)
	if err != nil {
		return QueryPage{}, err
	}
	data, err := c.read(ctx, providerRead{
		property: r.Property, policy: r.Policy, method: exchange.MethodPost,
		body: payload, maximum: responseMaximum,
	})
	if err != nil {
		return QueryPage{}, err
	}
	return decodePage(data, r)
}

func (c Client) GetSite(ctx context.Context, r SiteRequest) (Site, error) {
	if err := r.Validate(); err != nil {
		return Site{}, err
	}
	data, err := c.read(ctx, providerRead{
		property: r.Property, policy: r.Policy, method: exchange.MethodGet, maximum: siteMaximum,
	})
	if err != nil {
		return Site{}, err
	}
	return decodeSite(data, r.Property)
}

type providerRead struct {
	property Property
	policy   exchange.OperationPolicy
	method   exchange.Method
	body     []byte
	maximum  int
}

func (c Client) read(ctx context.Context, request providerRead) ([]byte, error) {
	if c.source == nil {
		return nil, core.ErrSearchConsoleContract
	}
	if err := contextstate.Validate(ctx); err != nil {
		return nil, errors.Join(core.ErrSearchConsoleContract, err)
	}
	ctx, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: ctx, Duration: request.policy.OperationTimeout})
	if err != nil {
		return nil, errors.Join(core.ErrSearchConsoleContract, err)
	}
	defer cancel()

	path := sitesEndpoint + url.PathEscape(request.property.String())
	if request.method == exchange.MethodPost {
		path += "/searchAnalytics/query"
	}
	target, targetErr := core.ParseHTTPEndpoint(path)
	header, headerErr := core.ParseHTTPHeaderName("Authorization")
	if err := errors.Join(targetErr, headerErr); err != nil {
		return nil, errors.Join(core.ErrSearchConsoleContract, err)
	}

	token, err := c.source.BorrowReadAccess(ctx)
	if err != nil {
		return nil, errors.Join(core.ErrSearchConsoleContract, err)
	}
	defer func() { _ = token.Close() }()
	bearer, err := token.CopyBytes()
	if err != nil {
		return nil, errors.Join(core.ErrSearchConsoleContract, err)
	}
	defer clear(bearer)

	headers := []exchange.SecretHeader{{Name: header, Value: bearer, Scheme: exchange.SecretHeaderBearer}}
	policy := exchange.StreamPolicy{
		OperationTimeout: request.policy.OperationTimeout, AttemptTimeout: request.policy.AttemptTimeout,
		Redirect: request.policy.Redirect,
	}
	body := responseBuffer{maximum: request.maximum}
	failure := responseBuffer{maximum: failureMaximum}
	defer func() { clear(failure.data) }()

	semantics := exchange.RequestSemantics{Method: request.method, Replay: exchange.ReplaySingleAttempt}
	var metadata exchange.ResponseMetadata
	if request.method == exchange.MethodPost {
		length, lengthErr := core.NewByteLength(uint64(len(request.body)))
		if lengthErr != nil {
			return nil, errors.Join(core.ErrSearchConsoleContract, lengthErr)
		}
		response, runErr := exchange.RoundTripStream(exchange.StreamRoundTripCall{
			Context: ctx, Client: c.network, Policy: policy,
			Request: exchange.StreamRoundTripRequest{
				Target: target, Semantics: semantics, SecretHeaders: headers,
				Source: bytes.NewReader(request.body), RequestContentLength: &length,
				RequestContentType: core.HTTPMediaTypeJSON(), ExpectedResponseContentType: core.HTTPMediaTypeJSON(),
				ExpectedStatus: core.HTTPStatusOK(), Destination: &body, FailureDestination: &failure,
			},
		})
		metadata, err = response.Metadata, runErr
	} else {
		response, runErr := exchange.Download(exchange.DownloadCall{
			Context: ctx, Client: c.network, Policy: policy,
			Request: exchange.DownloadRequest{
				Target: target, Semantics: semantics, SecretHeaders: headers,
				ExpectedResponseContentType: core.HTTPMediaTypeJSON(), ExpectedStatus: core.HTTPStatusOK(),
				Destination: &body, FailureDestination: &failure,
			},
		})
		metadata, err = response.Metadata, runErr
	}
	if err != nil {
		code, _ := metadata.Status.Int()
		return nil, failureForStatus(code, failure.data, err)
	}
	return body.data, nil
}

type responseBuffer struct {
	data    []byte
	maximum int
}

func (b *responseBuffer) Write(p []byte) (int, error) {
	if len(p) > b.maximum-len(b.data) {
		return 0, core.ErrExchangeBodyLimit
	}
	b.data = append(b.data, p...)
	return len(p), nil
}

func failureForStatus(code int, data []byte, cause error) error {
	if code == http.StatusTooManyRequests {
		return errors.Join(core.ErrSearchConsoleQuota, cause)
	}
	if (code == http.StatusForbidden || code == http.StatusNotFound) && !errors.Is(cause, core.ErrExchangeBodyLimit) {
		switch decodeFault(data, code) {
		case faultQuota:
			return errors.Join(core.ErrSearchConsoleQuota, cause)
		case faultProperty:
			return errors.Join(core.ErrSearchConsoleProperty, cause)
		}
	}
	return errors.Join(core.ErrSearchConsoleResponse, cause)
}
