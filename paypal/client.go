package paypal

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

func (r Request) Validate() error         { return validateRequestShape(r.Stream) }
func (r DownloadRequest) Validate() error { return validateDownloadShape(r.Stream) }

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
	client  exchange.Client
	token   AccessToken
	sandbox bool
}
type Client struct{ state *clientState }

func NewClient(client exchange.Client, token AccessToken, sandbox bool) (Client, error) {
	if err := errors.Join(client.Validate(), token.Validate()); err != nil {
		return Client{}, contractError(err)
	}
	owned, err := ParseAccessToken(token.authorization.Token)
	if err != nil {
		return Client{}, err
	}
	return Client{state: &clientState{client: client, token: owned, sandbox: sandbox}}, nil
}
func (c Client) Validate() error {
	if c.state == nil {
		return core.ErrPayPalContract
	}
	if err := errors.Join(c.state.client.Validate(), c.state.token.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}
func (c *Client) Close() error {
	if c == nil || c.state == nil {
		return core.ErrPayPalContract
	}
	err := c.state.token.Close()
	c.state = nil
	return err
}
func (c Client) host() string {
	if c.state != nil && c.state.sandbox {
		return core.PayPalSandboxAPIHost
	}
	return core.PayPalLiveAPIHost
}

func (c Client) RoundTrip(ctx context.Context, request Request, policy exchange.StreamPolicy) (Response, error) {
	if err := errors.Join(c.Validate(), validateRequestHost(request.Stream, c.host()), policy.Validate()); err != nil {
		return Response{}, contractError(err)
	}
	authorization, err := exchange.NewBearerAuthorizationHeader(c.state.token.authorization)
	if err != nil {
		return Response{}, authenticationError(err)
	}
	stream := request.Stream
	headers := []exchange.Header{authorization}
	if stream.Semantics.Replay == exchange.ReplaySingleAttemptWithIdempotencyKey {
		replay, headerErr := providerHeader(core.PayPalRequestIDHeaderName, stream.Semantics.IdempotencyKey.String())
		if headerErr != nil {
			return Response{}, contractError(headerErr)
		}
		headers = append(headers, replay)
	}
	stream.Headers = exchange.Headers{Values: headers}
	stream.Semantics = exchange.RequestSemantics{Method: stream.Semantics.Method, Replay: exchange.ReplaySingleAttempt}
	response, err := exchange.RoundTripStream(exchange.StreamRoundTripCall{Context: ctx, Client: c.state.client, Request: stream, Policy: policy})
	result := Response{Stream: response}
	if err != nil {
		return result, err
	}
	return result, result.Validate()
}

func (c Client) Download(ctx context.Context, request DownloadRequest, policy exchange.StreamPolicy) (DownloadResponse, error) {
	if err := errors.Join(c.Validate(), validateDownloadHost(request.Stream, c.host()), policy.Validate()); err != nil {
		return DownloadResponse{}, contractError(err)
	}
	authorization, err := exchange.NewBearerAuthorizationHeader(c.state.token.authorization)
	if err != nil {
		return DownloadResponse{}, authenticationError(err)
	}
	stream := request.Stream
	stream.Headers = exchange.Headers{Values: []exchange.Header{authorization}}
	response, err := exchange.Download(exchange.DownloadCall{Context: ctx, Client: c.state.client, Request: stream, Policy: policy})
	result := DownloadResponse{Stream: response}
	if err != nil {
		return result, err
	}
	return result, result.Validate()
}

func validateRequestShape(request exchange.StreamRoundTripRequest) error {
	if len(request.Headers.Values) != 0 || request.Validate() != nil || !validURL(request.Target.HTTPURL()) ||
		!versionedPath(request.Target.HTTPURL().Path) || request.Semantics.Method != exchange.MethodPost ||
		!mediaMatches(request.RequestContentType, core.HTTPMediaTypeJSON()) || !mediaMatches(request.ExpectedResponseContentType, core.HTTPMediaTypeJSON()) {
		return core.ErrPayPalBinding
	}
	return validateReplay(request.Semantics)
}
func validateRequestHost(request exchange.StreamRoundTripRequest, host string) error {
	if err := validateRequestShape(request); err != nil || request.Target.HTTPURL().Host != host {
		return errors.Join(core.ErrPayPalBinding, err)
	}
	return nil
}
func validateDownloadShape(request exchange.DownloadRequest) error {
	if len(request.Headers.Values) != 0 || request.Validate() != nil || !validURL(request.Target.HTTPURL()) ||
		!versionedPath(request.Target.HTTPURL().Path) || request.Semantics.Method != exchange.MethodGet ||
		request.Semantics.Replay != exchange.ReplaySingleAttempt || !mediaMatches(request.ExpectedResponseContentType, core.HTTPMediaTypeJSON()) {
		return core.ErrPayPalBinding
	}
	return nil
}
func validateDownloadHost(request exchange.DownloadRequest, host string) error {
	if err := validateDownloadShape(request); err != nil || request.Target.HTTPURL().Host != host {
		return errors.Join(core.ErrPayPalBinding, err)
	}
	return nil
}
func validateReplay(semantics exchange.RequestSemantics) error {
	switch semantics.Replay {
	case exchange.ReplaySingleAttempt:
		return nil
	case exchange.ReplaySingleAttemptWithIdempotencyKey:
		if semantics.IdempotencyKey.Validate() == nil && len(semantics.IdempotencyKey.String()) <= core.PayPalRequestIDMaximumBytes {
			return nil
		}
	}
	return core.ErrPayPalBinding
}
func validURL(value url.URL) bool {
	endpoint, err := core.ParseHTTPEndpoint(value.String())
	return err == nil && endpoint.Validate() == nil && value.Scheme == core.SchemeHTTPS &&
		value.RawPath == "" && value.Path != "" && path.Clean(value.Path) == value.Path
}
func versionedPath(value string) bool {
	return strings.HasPrefix(value, apiV1PathPrefix) || strings.HasPrefix(value, apiV2PathPrefix)
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
