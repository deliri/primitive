package exchange

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"path"

	"github.com/deliri/primitive/v2026/core"
)

const SocketRoutePathMaximumBytes = 2 * 1024

// SocketRoutePath is one canonical absolute HTTP route path. Products own the
// route constants; Exchange owns their transport-safe representation.
type SocketRoutePath struct{ value string }

// ParseSocketRoutePath admits one canonical absolute route path without a
// query, fragment, authority, or path normalization ambiguity.
func ParseSocketRoutePath(value string) (SocketRoutePath, error) {
	candidate := SocketRoutePath{value: value}
	if err := candidate.Validate(); err != nil {
		return SocketRoutePath{}, err
	}
	return candidate, nil
}

// String returns the validated route path.
func (p SocketRoutePath) String() string { return p.value }

// Validate rejects unset, relative, noncanonical, or decorated route paths.
func (p SocketRoutePath) Validate() error {
	if !validSocketRoutePathShape(p.value) {
		return core.ErrExchangeContract
	}
	parsed, err := url.ParseRequestURI(p.value)
	if err != nil || !validParsedSocketRoutePath(parsed, p.value) {
		return errors.Join(core.ErrExchangeContract, err)
	}
	return nil
}

func validSocketRoutePathShape(value string) bool {
	return len(value) > 0 && len(value) <= SocketRoutePathMaximumBytes && value[0] == '/' && path.Clean(value) == value
}

func validParsedSocketRoutePath(parsed *url.URL, value string) bool {
	return parsed != nil && parsed.Path == value && parsed.RawPath == "" && parsed.RawQuery == "" && !parsed.ForceQuery && parsed.Fragment == "" && parsed.Host == "" && parsed.Scheme == ""
}

// JSONSocketContract is the shared, domain-blind contract for one paired
// typed JSON route. A product supplies its compiler-owned route constant and
// exact request and response structures; Primitive owns framing and effects.
type JSONSocketContract struct {
	Path              SocketRoutePath
	Route             RouteSemantics
	RequestBodyLimit  core.ByteCount
	ResponseBodyLimit core.ByteCount
	SuccessStatus     core.HTTPStatusCode
}

// Validate closes the complete paired route contract.
func (c JSONSocketContract) Validate() error {
	if err := errors.Join(
		c.Path.Validate(),
		c.Route.Validate(),
		validateJSONLimit(c.RequestBodyLimit),
		validateJSONLimit(c.ResponseBodyLimit),
		c.SuccessStatus.Validate(),
	); err != nil {
		return errors.Join(core.ErrExchangeContract, err)
	}
	if !c.SuccessStatus.PermitsResponseBody() {
		return core.ErrExchangeContract
	}
	return nil
}

// ClientSocketConfiguration supplies one real client capability for one exact
// paired route. Target includes the product-owned authority and the exact path
// named by Contract.
type ClientSocketConfiguration struct {
	Client         Client
	Target         Target
	Headers        Headers
	CaptureHeaders HeaderSelection
	Operation      OperationPolicy
	Contract       JSONSocketContract
}

// Validate closes the client side before a network operation can begin.
func (c ClientSocketConfiguration) Validate() error {
	if c.Target == nil {
		return core.ErrExchangeContract
	}
	if err := errors.Join(
		c.Client.Validate(),
		c.Target.Validate(),
		c.Headers.Validate(),
		c.CaptureHeaders.Validate(),
		c.Operation.Validate(),
		c.Contract.Validate(),
	); err != nil {
		return errors.Join(core.ErrExchangeContract, err)
	}
	return validateSocketTarget(c.Target, c.Contract.Path)
}

// ClientSocket is a sealed domain-blind client capability for one exact
// paired route.
type ClientSocket struct{ configuration ClientSocketConfiguration }

// NewClientSocket constructs one paired-route client capability.
func NewClientSocket(configuration ClientSocketConfiguration) (ClientSocket, error) {
	if err := configuration.Validate(); err != nil {
		return ClientSocket{}, err
	}
	return ClientSocket{configuration: configuration}, nil
}

// Validate rejects an unconstructed or contradictory client socket.
func (s ClientSocket) Validate() error { return s.configuration.Validate() }

// SendSocketJSON sends one exact typed request and decodes its paired typed
// response. Replay-key routes must use SendReplayBoundSocketJSON.
func SendSocketJSON[
	Request core.ValidatedJSONMarshaler,
	Response core.Validatable,
](ctx context.Context, socket ClientSocket, request Request) (JSONResponse[Response], error) {
	var zero JSONResponse[Response]
	if err := socket.Validate(); err != nil || socket.configuration.Contract.Route.Replay == ReplayIdempotencyKey {
		return zero, errors.Join(core.ErrExchangeContract, err)
	}
	semantics, err := socketRequestSemantics(socket.configuration.Contract.Route, IdempotencyKey{})
	if err != nil {
		return zero, err
	}
	return SendJSON[Request, Response](socketJSONCall(ctx, socket, request, semantics))
}

// SendReplayBoundSocketJSON binds a request-owned idempotency key to the real
// HTTP replay identity before sending one exact typed mutation.
func SendReplayBoundSocketJSON[
	Request interface {
		core.ValidatedJSONMarshaler
		IdempotencyBound
	},
	Response core.Validatable,
](ctx context.Context, socket ClientSocket, request Request) (JSONResponse[Response], error) {
	var zero JSONResponse[Response]
	if err := socket.Validate(); err != nil || socket.configuration.Contract.Route.Replay != ReplayIdempotencyKey {
		return zero, errors.Join(core.ErrExchangeContract, err)
	}
	key, err := request.IdempotencyKey()
	if err != nil {
		return zero, requestError(err)
	}
	semantics, err := socketRequestSemantics(socket.configuration.Contract.Route, key)
	if err != nil {
		return zero, err
	}
	return SendReplayBoundJSON[Request, Response](socketJSONCall(ctx, socket, request, semantics))
}

func socketJSONCall[Request core.ValidatedJSONMarshaler](
	ctx context.Context,
	socket ClientSocket,
	request Request,
	semantics RequestSemantics,
) JSONCall[Request] {
	configuration := socket.configuration
	return JSONCall[Request]{
		Context: ctx,
		Client:  configuration.Client,
		Request: JSONRequest[Request]{
			Target:         configuration.Target,
			Body:           request,
			Semantics:      semantics,
			Headers:        configuration.Headers,
			CaptureHeaders: configuration.CaptureHeaders,
			ExpectedStatus: configuration.Contract.SuccessStatus,
		},
		Policy: JSONPolicy{
			Operation:         configuration.Operation,
			RequestBodyLimit:  configuration.Contract.RequestBodyLimit,
			ResponseBodyLimit: configuration.Contract.ResponseBodyLimit,
		},
	}
}

func socketRequestSemantics(route RouteSemantics, key IdempotencyKey) (RequestSemantics, error) {
	semantics := RequestSemantics{Method: route.Method, Replay: route.Replay, IdempotencyKey: key}
	if err := semantics.Validate(); err != nil {
		return RequestSemantics{}, err
	}
	return semantics, nil
}

// ServerSocket is the matching domain-blind server capability for one exact
// paired route.
type ServerSocket struct{ contract JSONSocketContract }

// NewServerSocket constructs the server side from the same contract consumed
// by the client side.
func NewServerSocket(contract JSONSocketContract) (ServerSocket, error) {
	if err := contract.Validate(); err != nil {
		return ServerSocket{}, err
	}
	return ServerSocket{contract: contract}, nil
}

// Validate rejects an unconstructed server socket.
func (s ServerSocket) Validate() error { return s.contract.Validate() }

// ReceiveSocketJSON validates the exact route and strictly decodes one typed
// request. Replay-key routes must use ReceiveReplayBoundSocketJSON.
func ReceiveSocketJSON[
	Body any,
	BodyPtr interface {
		*Body
		core.Validatable
	},
](socket ServerSocket, request *http.Request) (Received[BodyPtr], error) {
	var zero Received[BodyPtr]
	if err := validateServerSocketRequest(socket, request); err != nil || socket.contract.Route.Replay == ReplayIdempotencyKey {
		return zero, errors.Join(core.ErrExchangeContract, err)
	}
	return ReceiveJSON[Body, BodyPtr](JSONReceiveCall{
		Request: request,
		Route:   socket.contract.Route,
		Policy:  ServerPolicy{RequestBodyLimit: socket.contract.RequestBodyLimit},
	})
}

// ReceiveReplayBoundSocketJSON validates the exact route and binds the real
// HTTP replay identity to the decoded typed mutation.
func ReceiveReplayBoundSocketJSON[
	Body any,
	BodyPtr interface {
		*Body
		IdempotencyBound
	},
](socket ServerSocket, request *http.Request) (Received[BodyPtr], error) {
	var zero Received[BodyPtr]
	if err := validateServerSocketRequest(socket, request); err != nil || socket.contract.Route.Replay != ReplayIdempotencyKey {
		return zero, errors.Join(core.ErrExchangeContract, err)
	}
	return ReceiveReplayBoundJSON[Body, BodyPtr](JSONReceiveCall{
		Request: request,
		Route:   socket.contract.Route,
		Policy:  ServerPolicy{RequestBodyLimit: socket.contract.RequestBodyLimit},
	})
}

// WriteSocketJSON strictly encodes one typed response under the paired route's
// declared success status and response bound.
func WriteSocketJSON[Body core.ValidatedJSONMarshaler](
	socket ServerSocket,
	writer http.ResponseWriter,
	body Body,
) error {
	if err := socket.Validate(); err != nil {
		return err
	}
	return WriteJSON(JSONWriteCall[Body]{
		Writer: writer,
		Response: ServerJSONResponse[Body]{
			Body:   body,
			Status: socket.contract.SuccessStatus,
		},
		Policy: JSONWritePolicy{ResponseBodyLimit: socket.contract.ResponseBodyLimit},
	})
}

func validateServerSocketRequest(socket ServerSocket, request *http.Request) error {
	if err := socket.Validate(); err != nil {
		return err
	}
	if request == nil || request.URL == nil || request.URL.Path != socket.contract.Path.String() || request.URL.RawPath != "" || request.URL.RawQuery != "" || request.URL.ForceQuery {
		return core.ErrExchangeContract
	}
	return nil
}

func validateSocketTarget(target Target, route SocketRoutePath) error {
	urlValue := target.HTTPURL()
	if urlValue.Path != route.String() || urlValue.RawPath != "" || urlValue.RawQuery != "" || urlValue.ForceQuery || urlValue.Fragment != "" {
		return core.ErrExchangeContract
	}
	return nil
}

var (
	_ core.Validatable = SocketRoutePath{}
	_ core.Validatable = JSONSocketContract{}
	_ core.Validatable = ClientSocketConfiguration{}
	_ core.Validatable = ClientSocket{}
	_ core.Validatable = ServerSocket{}
)
