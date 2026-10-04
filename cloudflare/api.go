package cloudflare

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// ServerOptions owns account binding and the application's aggregate JSON
// response budget. Media transfers do not use this document budget.
type ServerOptions struct {
	Account        AccountID
	Token          APIToken
	ResponseLimits core.StrictJSONLimits
}

func (o ServerOptions) Validate() error {
	if err := errors.Join(o.Token.Validate(), o.Account.Validate(), o.ResponseLimits.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

type apiServer struct {
	client  exchange.Client
	options ServerOptions
}

func newAPIServer(client exchange.Client, options ServerOptions) (apiServer, error) {
	if err := errors.Join(client.Validate(), options.Validate()); err != nil {
		return apiServer{}, contractError(err)
	}
	token, err := ParseAPIToken(options.Token.value)
	if err != nil {
		return apiServer{}, err
	}
	options.Token = token
	return apiServer{client: client, options: options}, nil
}
func (s apiServer) Validate() error {
	if err := errors.Join(s.client.Validate(), s.options.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

// APIIssue retains the provider's documented code and safe diagnostic payload.
// https://developers.cloudflare.com/api/resources/images/
type APIIssue struct {
	Source           *APIIssueSource `json:"source,omitempty"`
	Message          string          `json:"message"`
	DocumentationURL string          `json:"documentation_url,omitempty"`
	Code             int64           `json:"code"`
}
type APIIssueSource struct {
	Pointer string `json:"pointer"`
}
type APIRefusal struct{ Issues []APIIssue }

func (e APIRefusal) Error() string { return core.ErrCloudflareResponse.Error() }
func (e APIRefusal) Unwrap() error { return core.ErrCloudflareResponse }

type apiEnvelope[T any] struct {
	Result   T          `json:"result"`
	Success  *bool      `json:"success"`
	Errors   []APIIssue `json:"errors"`
	Messages []APIIssue `json:"messages"`
}

type apiIntent struct {
	source io.Reader
	suffix string
	media  core.HTTPMediaType
	method exchange.Method
}

type boundedResponse struct {
	buffer  bytes.Buffer
	maximum uint64
}

func (b *boundedResponse) Write(p []byte) (int, error) {
	length, err := core.CheckedUint64FromInt64(int64(b.buffer.Len()))
	if err != nil || length > b.maximum {
		return 0, responseError(err)
	}
	if uint64(len(p)) > b.maximum-length {
		return 0, core.ErrCloudflareResponse
	}
	return b.buffer.Write(p)
}

func executeAPI[T core.Validatable](ctx context.Context, server apiServer, intent apiIntent, policy exchange.StreamPolicy) (T, error) {
	var zero T
	if err := errors.Join(server.Validate(), validatePolicy(policy)); err != nil {
		return zero, err
	}
	maximum, err := server.options.ResponseLimits.DocumentMaximumBytes.Uint64()
	if err != nil {
		return zero, contractError(err)
	}
	response := boundedResponse{maximum: maximum}
	if err := server.transfer(ctx, intent, &response, policy); err != nil {
		return zero, err
	}
	envelope, err := core.DecodeStrictJSONStructure[apiEnvelope[T]](response.buffer.Bytes(), server.options.ResponseLimits)
	if err != nil {
		return zero, responseError(err)
	}
	if envelope.Success == nil {
		return zero, core.ErrCloudflareResponse
	}
	if !*envelope.Success {
		return zero, APIRefusal{Issues: envelope.Errors}
	}
	if len(envelope.Errors) != 0 {
		return zero, core.ErrCloudflareResponse
	}
	if err := envelope.Result.Validate(); err != nil {
		return zero, responseError(err)
	}
	return envelope.Result, nil
}

func (server apiServer) transfer(ctx context.Context, intent apiIntent, destination io.Writer, policy exchange.StreamPolicy) error {
	target, err := core.ParseHTTPEndpoint("https://" + core.CloudflareAPIHost + "/client/v4/accounts/" + server.options.Account.value + intent.suffix)
	if err != nil {
		return contractError(err)
	}
	authorization, err := exchange.NewBearerAuthorizationHeader(exchange.BearerAuthorization{Token: server.options.Token.value})
	if err != nil {
		return authenticationError(err)
	}
	headers := exchange.Headers{Values: []exchange.Header{authorization}}
	semantics := exchange.RequestSemantics{Method: intent.method, Replay: exchange.ReplaySingleAttempt}
	if intent.source == nil {
		_, err = exchange.Download(exchange.DownloadCall{Context: ctx, Client: server.client, Policy: policy,
			Request: exchange.DownloadRequest{Target: target, Destination: destination, Headers: headers, Semantics: semantics,
				ExpectedStatus: core.HTTPStatusOK(), ExpectedResponseContentType: core.HTTPMediaTypeJSON()}})
		return err
	}
	_, err = exchange.RoundTripStream(exchange.StreamRoundTripCall{Context: ctx, Client: server.client, Policy: policy,
		Request: exchange.StreamRoundTripRequest{Target: target, Source: intent.source, Destination: destination, Headers: headers,
			Semantics: semantics, RequestContentType: intent.media, ExpectedResponseContentType: core.HTTPMediaTypeJSON(), ExpectedStatus: core.HTTPStatusOK()}})
	return err
}

func validatePolicy(policy exchange.StreamPolicy) error {
	if err := policy.Validate(); err != nil {
		return contractError(err)
	}
	if policy.Redirect.Mode != exchange.RedirectReject {
		return core.ErrCloudflareBinding
	}
	return nil
}

func uploadEndpoint(value, host string) (core.HTTPEndpoint, error) {
	endpoint, err := core.ParseHTTPEndpoint(value)
	if err != nil {
		return core.HTTPEndpoint{}, contractError(err)
	}
	u := endpoint.HTTPURL()
	if u.Scheme != core.SchemeHTTPS || u.Host != host || u.Path == "" || u.Path == "/" || u.User != nil || u.Fragment != "" {
		return core.HTTPEndpoint{}, core.ErrCloudflareBinding
	}
	return endpoint, nil
}
