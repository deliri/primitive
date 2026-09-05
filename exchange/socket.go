package exchange

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"path"

	"github.com/deliri/primitive/v2026/core"
)

const SocketRequestTargetMaximumBytes = 16 * 1024

// HTTPStatusAccepted returns net/http's canonical 202 status through the
// compiler-owned status contract.
func HTTPStatusAccepted() (core.HTTPStatusCode, error) {
	var status core.HTTPStatusCode
	return status, status.AdmitInt(http.StatusAccepted)
}

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
	RequestBodyLimit  core.ByteCount
	ResponseBodyLimit core.ByteCount
	SuccessStatus     core.HTTPStatusCode
	Route             RouteSemantics
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
	Target         Target
	Client         Client
	Headers        Headers
	CaptureHeaders HeaderSelection
	Contract       JSONSocketContract
	Operation      OperationPolicy
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

// SocketServerCall carries one HTTP ingress across the Exchange ownership
// boundary without requiring a domain package to import net/http.
type SocketServerCall struct {
	request *http.Request
	writer  http.ResponseWriter
}

// NewSocketServerCall binds the real HTTP request and response writer at the
// Exchange boundary.
func NewSocketServerCall(writer http.ResponseWriter, request *http.Request) (SocketServerCall, error) {
	call := SocketServerCall{writer: writer, request: request}
	return call, call.Validate()
}

// Validate rejects a partially populated HTTP ingress.
func (c SocketServerCall) Validate() error {
	if responseWriterIsNil(c.writer) || c.request == nil {
		return core.ErrExchangeContract
	}
	return nil
}

// Context returns the request lifetime owned by this ingress.
func (c SocketServerCall) Context() (context.Context, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c.request.Context(), nil
}

// WithContext derives one call carrying domain-authenticated context while
// retaining Exchange ownership of the underlying HTTP request and writer.
func (c SocketServerCall) WithContext(ctx context.Context) (SocketServerCall, error) {
	if err := c.Validate(); err != nil || ctx == nil {
		return SocketServerCall{}, errors.Join(core.ErrExchangeContract, err)
	}
	call := SocketServerCall{request: c.request.WithContext(ctx), writer: c.writer}
	return call, call.Validate()
}

// ServeHTTP continues one standard-library handler chain with the exact
// request and writer sealed into this call. Authentication and other boundary
// packages can derive a call with typed context without exposing or rebuilding
// either raw HTTP value.
func (c SocketServerCall) ServeHTTP(next http.Handler) error {
	if err := c.Validate(); err != nil || next == nil {
		return errors.Join(core.ErrExchangeContract, err)
	}
	next.ServeHTTP(c.writer, c.request)
	return nil
}

// VerifiedClientCertificateDigest returns the SHA-256 identity of the leaf
// certificate admitted by net/http's verified mutual-TLS chain.
func (c SocketServerCall) VerifiedClientCertificateDigest() (core.SHA256Digest, error) {
	if err := c.Validate(); err != nil {
		return core.SHA256Digest{}, err
	}
	tlsState := c.request.TLS
	if tlsState == nil || len(tlsState.VerifiedChains) == 0 || len(tlsState.VerifiedChains[0]) == 0 {
		return core.SHA256Digest{}, core.ErrExchangeContract
	}
	certificate := tlsState.VerifiedChains[0][0]
	if certificate == nil || len(certificate.Raw) == 0 {
		return core.SHA256Digest{}, core.ErrExchangeContract
	}
	return core.SHA256Of(certificate.Raw), nil
}

// UniqueHeader admits one bounded, exactly-once HTTP field from the bound
// request. Header storage and duplicate handling remain Exchange mechanics.
func (c SocketServerCall) UniqueHeader(name core.HTTPHeaderName, maximum core.ByteCount) (HeaderValue, error) {
	if err := c.Validate(); err != nil {
		return HeaderValue{}, err
	}
	if err := name.Validate(); err != nil {
		return HeaderValue{}, errors.Join(core.ErrExchangeContract, err)
	}
	limit, err := maximum.Int64()
	if err != nil {
		return HeaderValue{}, errors.Join(core.ErrExchangeContract, err)
	}
	values := c.request.Header.Values(name.String())
	if len(values) != 1 || len(values[0]) == 0 || int64(len(values[0])) > limit {
		return HeaderValue{}, core.ErrExchangeContract
	}
	value, err := NewHeaderValue(values[0])
	if err != nil {
		return HeaderValue{}, err
	}
	return value, nil
}

// MatchesEndpointPath reports whether the observed escaped path is exactly
// the path bound into a configured public endpoint.
func (c SocketServerCall) MatchesEndpointPath(endpoint core.HTTPEndpoint) (bool, error) {
	if err := c.Validate(); err != nil {
		return false, err
	}
	if err := endpoint.Validate(); err != nil {
		return false, errors.Join(core.ErrExchangeContract, err)
	}
	public := endpoint.HTTPURL()
	return c.request.URL != nil && c.request.URL.Path == public.Path && c.request.URL.RawPath == public.RawPath, nil
}

// RawQuery returns the bounded, exact encoded query observed on the request.
func (c SocketServerCall) RawQuery() (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}
	if c.request.URL == nil || len(c.request.URL.RawQuery) > SocketRequestTargetMaximumBytes {
		return "", core.ErrExchangeContract
	}
	return c.request.URL.RawQuery, nil
}

func validateSocketCallPath(call SocketServerCall, path SocketRoutePath) error {
	if err := path.Validate(); err != nil {
		return err
	}
	request := call.request
	if request.URL == nil || request.URL.Path != path.String() || request.URL.RawPath != "" || request.URL.RawQuery != "" || request.URL.ForceQuery {
		return core.ErrExchangeContract
	}
	return nil
}

// ValidateSocketCallPath proves that a bound call addresses one exact
// compiler-owned route without exposing the raw HTTP target.
func ValidateSocketCallPath(call SocketServerCall, path SocketRoutePath) error {
	if err := call.Validate(); err != nil {
		return err
	}
	return validateSocketCallPath(call, path)
}

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

func receiveSocketJSON[
	Body any,
	BodyPtr interface {
		*Body
		core.Validatable
	},
](socket ServerSocket, call SocketServerCall) (Received[BodyPtr], error) {
	var zero Received[BodyPtr]
	if err := call.Validate(); err != nil {
		return zero, err
	}
	if err := validateServerSocketRequest(socket, call.request); err != nil || socket.contract.Route.Replay == ReplayIdempotencyKey {
		return zero, errors.Join(core.ErrExchangeContract, err)
	}
	return ReceiveJSON[Body, BodyPtr](JSONReceiveCall{
		Call:   call,
		Route:  socket.contract.Route,
		Policy: ServerPolicy{RequestBodyLimit: socket.contract.RequestBodyLimit},
	})
}

// ReceiveSocketJSON admits one typed request through Exchange's sole public
// server boundary. Replay-key routes must use ReceiveReplayBoundSocketJSON.
func ReceiveSocketJSON[
	Body any,
	BodyPtr interface {
		*Body
		core.Validatable
	},
](socket ServerSocket, call SocketServerCall) (Received[BodyPtr], error) {
	var zero Received[BodyPtr]
	if err := call.Validate(); err != nil {
		return zero, err
	}
	return receiveSocketJSON[Body, BodyPtr](socket, call)
}

func receiveReplayBoundSocketJSON[
	Body any,
	BodyPtr interface {
		*Body
		IdempotencyBound
	},
](socket ServerSocket, call SocketServerCall) (Received[BodyPtr], error) {
	var zero Received[BodyPtr]
	if err := call.Validate(); err != nil {
		return zero, err
	}
	if err := validateServerSocketRequest(socket, call.request); err != nil || socket.contract.Route.Replay != ReplayIdempotencyKey {
		return zero, errors.Join(core.ErrExchangeContract, err)
	}
	return ReceiveReplayBoundJSON[Body, BodyPtr](JSONReceiveCall{
		Call:   call,
		Route:  socket.contract.Route,
		Policy: ServerPolicy{RequestBodyLimit: socket.contract.RequestBodyLimit},
	})
}

// ReceiveReplayBoundSocketJSON admits one replay-bound typed mutation through
// Exchange's sole public server boundary.
func ReceiveReplayBoundSocketJSON[
	Body any,
	BodyPtr interface {
		*Body
		IdempotencyBound
	},
](socket ServerSocket, call SocketServerCall) (Received[BodyPtr], error) {
	var zero Received[BodyPtr]
	if err := call.Validate(); err != nil {
		return zero, err
	}
	return receiveReplayBoundSocketJSON[Body, BodyPtr](socket, call)
}

func writeSocketJSON[Body core.ValidatedJSONMarshaler](
	socket ServerSocket,
	call SocketServerCall,
	body Body,
) error {
	if err := errors.Join(socket.Validate(), call.Validate()); err != nil {
		return err
	}
	return WriteJSON(JSONWriteCall[Body]{
		Call: call,
		Response: ServerJSONResponse[Body]{
			Body:   body,
			Status: socket.contract.SuccessStatus,
		},
		Policy: JSONWritePolicy{ResponseBodyLimit: socket.contract.ResponseBodyLimit},
	})
}

// WriteSocketJSON emits one typed response through Exchange's sole public
// server boundary.
func WriteSocketJSON[Body core.ValidatedJSONMarshaler](
	socket ServerSocket,
	call SocketServerCall,
	body Body,
) error {
	if err := call.Validate(); err != nil {
		return err
	}
	return writeSocketJSON(socket, call, body)
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
