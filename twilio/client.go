package twilio

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

func (r Request) Validate() error         { return validateRequest(r.Stream, "") }
func (r DownloadRequest) Validate() error { return validateDownload(r.Stream, "") }

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
	owned, err := NewCredential(credential.AccountSID, credential.APIKeySID, credential.secret)
	if err != nil {
		return Client{}, err
	}
	return Client{state: &clientState{client: client, credential: owned}}, nil
}
func (c Client) Validate() error {
	if c.state == nil {
		return core.ErrTwilioContract
	}
	if err := errors.Join(c.state.client.Validate(), c.state.credential.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}
func (c *Client) Close() error {
	if c == nil || c.state == nil {
		return core.ErrTwilioContract
	}
	err := c.state.credential.Close()
	c.state = nil
	return err
}

func (c Client) RoundTrip(ctx context.Context, request Request, policy exchange.StreamPolicy) (Response, error) {
	if err := errors.Join(c.Validate(), validateRequest(request.Stream, c.state.credential.AccountSID), policy.Validate()); err != nil {
		return Response{}, contractError(err)
	}
	authorization, err := exchange.NewBasicAuthorizationHeader(exchange.BasicAuthorizationRequest{
		Identity: exchange.BasicAuthorizationIdentity(c.state.credential.APIKeySID), Secret: c.state.credential.secret,
	})
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

func (c Client) Download(ctx context.Context, request DownloadRequest, policy exchange.StreamPolicy) (DownloadResponse, error) {
	if err := errors.Join(c.Validate(), validateDownload(request.Stream, c.state.credential.AccountSID), policy.Validate()); err != nil {
		return DownloadResponse{}, contractError(err)
	}
	authorization, err := exchange.NewBasicAuthorizationHeader(exchange.BasicAuthorizationRequest{
		Identity: exchange.BasicAuthorizationIdentity(c.state.credential.APIKeySID), Secret: c.state.credential.secret,
	})
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

func validateRequest(request exchange.StreamRoundTripRequest, account AccountSID) error {
	if len(request.Headers.Values) != 0 || request.Validate() != nil || !validURL(request.Target.HTTPURL()) ||
		request.Semantics.Method != exchange.MethodPost || request.Semantics.Replay != exchange.ReplaySingleAttempt ||
		!mediaMatches(request.RequestContentType, core.HTTPMediaTypeFormURLEncoded()) ||
		!mediaMatches(request.ExpectedResponseContentType, core.HTTPMediaTypeJSON()) {
		return core.ErrTwilioBinding
	}
	if account != "" && !strings.HasPrefix(request.Target.HTTPURL().Path, "/2010-04-01/Accounts/"+account.String()+"/") {
		return core.ErrTwilioBinding
	}
	return nil
}

func validateDownload(request exchange.DownloadRequest, account AccountSID) error {
	if len(request.Headers.Values) != 0 || request.Validate() != nil || !validURL(request.Target.HTTPURL()) ||
		request.Semantics.Method != exchange.MethodGet || request.Semantics.Replay != exchange.ReplaySingleAttempt ||
		!mediaMatches(request.ExpectedResponseContentType, core.HTTPMediaTypeJSON()) {
		return core.ErrTwilioBinding
	}
	if account != "" && !strings.HasPrefix(request.Target.HTTPURL().Path, "/2010-04-01/Accounts/"+account.String()+"/") {
		return core.ErrTwilioBinding
	}
	return nil
}

func validURL(value url.URL) bool {
	endpoint, err := core.ParseHTTPEndpoint(value.String())
	return err == nil && endpoint.Validate() == nil && value.Scheme == core.SchemeHTTPS && value.Host == core.TwilioAPIHost &&
		value.RawPath == "" && value.Path != "" && path.Clean(value.Path) == value.Path
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
