package loops

import (
	"context"
	"errors"
	"net/url"
	"path"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// Request crosses the body-bearing and DELETE doors. GET reads cross Download.
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
		return core.ErrLoopsContract
	}
	if err := errors.Join(c.state.client.Validate(), c.state.credential.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

func (c *Client) Close() error {
	if c == nil || c.state == nil {
		return core.ErrLoopsContract
	}
	err := c.state.credential.Close()
	c.state = nil
	return err
}

// RoundTrip sends one POST, PUT or DELETE exactly once. Exchange owns the
// single attempt; a 429 or 409 is returned as the provider's exact status.
func (c Client) RoundTrip(ctx context.Context, request Request, policy exchange.StreamPolicy) (Response, error) {
	if err := errors.Join(c.Validate(), request.Validate(), policy.Validate()); err != nil {
		return Response{}, contractError(err)
	}
	authorization, err := exchange.NewBearerAuthorizationHeader(exchange.BearerAuthorization{Token: c.state.credential.value})
	if err != nil {
		return Response{}, authenticationError(err)
	}
	stream := request.Stream
	stream.Headers = exchange.Headers{Values: []exchange.Header{authorization}}
	response, err := exchange.RoundTripStream(exchange.StreamRoundTripCall{Context: ctx, Client: c.state.client, Request: stream, Policy: policy})
	result := Response{Stream: response}
	if err != nil {
		return result, err
	}
	return result, result.Validate()
}

// Download streams one GET read's JSON representation to the caller.
func (c Client) Download(ctx context.Context, request DownloadRequest, policy exchange.StreamPolicy) (DownloadResponse, error) {
	if err := errors.Join(c.Validate(), request.Validate(), policy.Validate()); err != nil {
		return DownloadResponse{}, contractError(err)
	}
	authorization, err := exchange.NewBearerAuthorizationHeader(exchange.BearerAuthorization{Token: c.state.credential.value})
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

func validateRequest(request exchange.StreamRoundTripRequest) error {
	if !callerOwnsNoAuthority(request.Headers, request.SecretHeaders) || request.Validate() != nil ||
		request.Semantics.Method == exchange.MethodGet || !jsonExchange(request.RequestContentType, request.ExpectedResponseContentType) {
		return core.ErrLoopsBinding
	}
	route, err := routeFromTarget(request.Target, request.Semantics.Method)
	if err != nil {
		return err
	}
	if !validRequestExtent(request) {
		return core.ErrLoopsBinding
	}
	return validateReplay(route.Operation, request.Semantics)
}

// validRequestExtent admits DELETE /v1/contacts/suppression only with a
// declared zero length: it identifies its contact in the query and documents
// no request body.
func validRequestExtent(request exchange.StreamRoundTripRequest) bool {
	if request.Semantics.Method != exchange.MethodDelete {
		return true
	}
	return request.RequestContentLength != nil && request.RequestContentLength.Uint64() == 0
}

// callerOwnsNoAuthority keeps authorization an SDK-owned projection.
func callerOwnsNoAuthority(headers exchange.Headers, secret []exchange.SecretHeader) bool {
	return len(headers.Values) == 0 && len(secret) == 0
}

func jsonExchange(request, response core.HTTPMediaType) bool {
	return mediaMatches(request, core.HTTPMediaTypeJSON()) && mediaMatches(response, core.HTTPMediaTypeJSON())
}

func validateDownload(request exchange.DownloadRequest) error {
	if !callerOwnsNoAuthority(request.Headers, request.SecretHeaders) || request.Validate() != nil ||
		request.Semantics.Method != exchange.MethodGet || request.Semantics.Replay != exchange.ReplaySingleAttempt ||
		!mediaMatches(request.ExpectedResponseContentType, core.HTTPMediaTypeJSON()) {
		return core.ErrLoopsBinding
	}
	_, err := routeFromTarget(request.Target, request.Semantics.Method)
	return err
}

func validateReplay(operation Operation, semantics exchange.RequestSemantics) error {
	switch semantics.Replay {
	case exchange.ReplaySingleAttempt:
		return nil
	case exchange.ReplaySingleAttemptWithIdempotencyKey:
		if operation.admitsIdempotencyKey() && semantics.IdempotencyKey.Validate() == nil &&
			len(semantics.IdempotencyKey.String()) <= core.LoopsIdempotencyKeyMaximumBytes {
			return nil
		}
		return core.ErrLoopsBinding
	default:
		return core.ErrLoopsBinding
	}
}

func validURL(value url.URL) bool {
	endpoint, err := core.ParseHTTPEndpoint(value.String())
	return err == nil && endpoint.Validate() == nil && value.Scheme == core.SchemeHTTPS && value.Host == core.LoopsAPIHost &&
		canonicalURLForm(value)
}

// canonicalURLForm refuses every URL spelling the Route projection never emits.
func canonicalURLForm(value url.URL) bool {
	return value.User == nil && value.Opaque == "" && value.RawPath == "" && !value.ForceQuery && value.Fragment == "" &&
		value.Path != "" && path.Clean(value.Path) == value.Path
}

func mediaMatches(got core.HTTPMediaType, want core.HTTPMediaType) bool {
	matches, err := got.SameBase(want)
	return err == nil && matches
}

var (
	_ core.Validatable = Request{}
	_ core.Validatable = Response{}
	_ core.Validatable = DownloadRequest{}
	_ core.Validatable = DownloadResponse{}
	_ core.Validatable = Client{}
)
