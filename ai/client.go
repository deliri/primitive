package ai

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// Request binds borrowed streams to one closed provider route. Payload bytes
// are data; the application bridge owns their schema and interpretation.
type Request struct {
	Stream  exchange.StreamRoundTripRequest
	Route   Route
	Options []OptionalHeader
	Version APIVersion
}

// Response carries native Exchange metadata; body bytes stay with the sink.
type Response struct {
	Stream exchange.StreamRoundTripResponse
}

// Validate refuses foreign targets, caller authentication headers, retries and
// media or optional-header selections outside the closed provider agreement.
func (r Request) Validate() error {
	target, err := r.Route.Endpoint()
	if err != nil {
		return err
	}
	if err := r.Stream.Validate(); err != nil {
		return contractError(err)
	}
	if r.Stream.Target.HTTPURL() != target.HTTPURL() || len(r.Stream.Headers.Values) != 0 || len(r.Stream.SecretHeaders) != 0 ||
		r.Stream.Semantics.Method != exchange.MethodPost ||
		r.Stream.Semantics.Replay != exchange.ReplaySingleAttempt {
		return core.ErrAIBinding
	}
	if _, err := optionalHeaders(r); err != nil {
		return err
	}
	want := core.HTTPMediaTypeJSON()
	if r.Route.Operation == AudioTranscriptions {
		want, err = core.ParseHTTPMediaType(core.AIMultipartMediaType)
		if err != nil {
			return contractError(err)
		}
	}
	matches, err := r.Stream.RequestContentType.SameBase(want)
	if err != nil || !matches {
		return errors.Join(core.ErrAIBinding, err)
	}
	return nil
}

// Validate checks the native response metadata agreement without reading bodies.
func (r Response) Validate() error {
	if err := r.Stream.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// Server is the replaceable behavior seam. Implementations receive validated
// typed requests and borrowed streams, never an untyped payload or registry.
type Server interface {
	core.Validatable
	RoundTrip(context.Context, Request, exchange.StreamPolicy) (Response, error)
}

// Client owns no transport or streams and delegates through one validated SDK seam.
type Client struct{ server Server }

// NewClient binds a validated server without starting effects or retaining payloads.
func NewClient(server Server) (Client, error) {
	client := Client{server: server}
	if err := client.Validate(); err != nil {
		return Client{}, err
	}
	return client, nil
}

// Validate checks the replaceable server's current authority.
func (c Client) Validate() error {
	if c.server == nil {
		return core.ErrAIContract
	}
	if err := c.server.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// RoundTrip validates context, request and policy before one server delegation.
// Native or typed server failures preserve their cause and acknowledged progress.
func (c Client) RoundTrip(ctx context.Context, request Request, policy exchange.StreamPolicy) (Response, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return Response{}, err
	}
	if err := c.Validate(); err != nil {
		return Response{}, err
	}
	if err := request.Validate(); err != nil {
		return Response{}, err
	}
	if err := validatePolicy(policy); err != nil {
		return Response{}, err
	}
	response, err := c.server.RoundTrip(ctx, request, policy)
	if err != nil {
		return response, err
	}
	if err := response.Validate(); err != nil {
		return Response{}, err
	}
	return response, nil
}

func validatePolicy(policy exchange.StreamPolicy) error {
	if err := policy.Validate(); err != nil {
		return contractError(err)
	}
	if policy.Redirect.Mode != exchange.RedirectReject {
		return core.ErrAIBinding
	}
	return nil
}
