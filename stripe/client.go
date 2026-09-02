package stripe

import (
	"context"
	"errors"
	"net/url"
	"path"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type Request struct {
	Stream exchange.StreamRoundTripRequest
}
type Response struct {
	Stream exchange.StreamRoundTripResponse
}
type DownloadRequest struct{ Stream exchange.DownloadRequest }
type DownloadResponse struct{ Stream exchange.StreamResponse }

func (r Request) Validate() error         { return validateRequest(r.Stream) }
func (r DownloadRequest) Validate() error { return validateDownload(r.Stream) }

func (r Response) Validate() error {
	if err := r.Stream.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

func (r DownloadResponse) Validate() error {
	if err := r.Stream.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

type clientState struct {
	client     exchange.Client
	credential Credential
}

type Client struct{ state *clientState }

func NewClient(client exchange.Client, credential Credential) (Client, error) {
	if err := errors.Join(client.Validate(), credential.Validate()); err != nil {
		return Client{}, contractError(err)
	}
	owned, err := ParseCredential(credential.value)
	if err != nil {
		return Client{}, err
	}
	return Client{state: &clientState{client: client, credential: owned}}, nil
}

func (c Client) Validate() error {
	if c.state == nil {
		return core.ErrStripeContract
	}
	if err := errors.Join(c.state.client.Validate(), c.state.credential.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

func (c *Client) Close() error {
	if c == nil || c.state == nil {
		return core.ErrStripeContract
	}
	err := c.state.credential.Close()
	c.state = nil
	return err
}

func (c Client) RoundTrip(ctx context.Context, request Request, policy exchange.StreamPolicy) (Response, error) {
	if err := errors.Join(c.Validate(), request.Validate(), policy.Validate()); err != nil {
		return Response{}, contractError(err)
	}
	authorization, err := exchange.NewBearerAuthorizationHeader(exchange.BearerAuthorization{Token: c.state.credential.value})
	if err != nil {
		return Response{}, authenticationError(err)
	}
	version, err := providerHeader(core.StripeVersionHeaderName, core.StripeAPIVersion)
	if err != nil {
		return Response{}, contractError(err)
	}
	stream := request.Stream
	stream.Headers = exchange.Headers{Values: []exchange.Header{authorization, version}}
	response, err := exchange.RoundTripStream(exchange.StreamRoundTripCall{Context: ctx, Client: c.state.client, Request: stream, Policy: policy})
	result := Response{Stream: response}
	if err != nil {
		return result, err
	}
	return result, result.Validate()
}

func (c Client) Download(ctx context.Context, request DownloadRequest, policy exchange.StreamPolicy) (DownloadResponse, error) {
	if err := errors.Join(c.Validate(), request.Validate(), policy.Validate()); err != nil {
		return DownloadResponse{}, contractError(err)
	}
	authorization, err := exchange.NewBearerAuthorizationHeader(exchange.BearerAuthorization{Token: c.state.credential.value})
	if err != nil {
		return DownloadResponse{}, authenticationError(err)
	}
	version, err := providerHeader(core.StripeVersionHeaderName, core.StripeAPIVersion)
	if err != nil {
		return DownloadResponse{}, contractError(err)
	}
	stream := request.Stream
	stream.Headers = exchange.Headers{Values: []exchange.Header{authorization, version}}
	response, err := exchange.Download(exchange.DownloadCall{Context: ctx, Client: c.state.client, Request: stream, Policy: policy})
	result := DownloadResponse{Stream: response}
	if err != nil {
		return result, err
	}
	return result, result.Validate()
}

func validateRequest(request exchange.StreamRoundTripRequest) error {
	if len(request.Headers.Values) != 0 || request.Validate() != nil || !validURL(request.Target.HTTPURL()) {
		return core.ErrStripeBinding
	}
	want := core.HTTPMediaTypeFormURLEncoded()
	if strings.HasPrefix(request.Target.HTTPURL().Path, apiV2PathPrefix) {
		want = core.HTTPMediaTypeJSON()
	} else if !strings.HasPrefix(request.Target.HTTPURL().Path, apiV1PathPrefix) {
		return core.ErrStripeBinding
	}
	if request.Semantics.Method != exchange.MethodPost || !mediaMatches(request.RequestContentType, want) ||
		!mediaMatches(request.ExpectedResponseContentType, core.HTTPMediaTypeJSON()) {
		return core.ErrStripeBinding
	}
	return validateReplay(request.Semantics)
}

func validateDownload(request exchange.DownloadRequest) error {
	if len(request.Headers.Values) != 0 || request.Validate() != nil || !validURL(request.Target.HTTPURL()) ||
		request.Semantics.Method != exchange.MethodGet || request.Semantics.Replay != exchange.ReplaySingleAttempt ||
		!strings.HasPrefix(request.Target.HTTPURL().Path, apiV1PathPrefix) && !strings.HasPrefix(request.Target.HTTPURL().Path, apiV2PathPrefix) ||
		!mediaMatches(request.ExpectedResponseContentType, core.HTTPMediaTypeJSON()) {
		return core.ErrStripeBinding
	}
	return nil
}

func validateReplay(semantics exchange.RequestSemantics) error {
	switch semantics.Replay {
	case exchange.ReplaySingleAttempt:
		return nil
	case exchange.ReplaySingleAttemptWithIdempotencyKey:
		if semantics.IdempotencyKey.Validate() == nil && len(semantics.IdempotencyKey.String()) <= core.StripeIdempotencyKeyMaximumBytes {
			return nil
		}
	}
	return core.ErrStripeBinding
}

func validURL(value url.URL) bool {
	endpoint, err := core.ParseHTTPEndpoint(value.String())
	if err != nil || value.Scheme != core.SchemeHTTPS || value.Host != core.StripeAPIHost || value.RawPath != "" || value.Path == "" || path.Clean(value.Path) != value.Path {
		return false
	}
	return endpoint.Validate() == nil
}

func mediaMatches(got core.HTTPMediaType, want core.HTTPMediaType) bool {
	matches, err := got.SameBase(want)
	return err == nil && matches
}

func providerHeader(nameText, valueText string) (exchange.Header, error) {
	name, err := core.ParseHTTPHeaderName(nameText)
	if err != nil {
		return exchange.Header{}, err
	}
	value, err := exchange.NewHeaderValue(valueText)
	if err != nil {
		return exchange.Header{}, err
	}
	header := exchange.Header{Name: name, Values: []exchange.HeaderValue{value}}
	return header, header.Validate()
}

var (
	_ core.Validatable = Request{}
	_ core.Validatable = Response{}
	_ core.Validatable = DownloadRequest{}
	_ core.Validatable = DownloadResponse{}
	_ core.Validatable = Client{}
)
