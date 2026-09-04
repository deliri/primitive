package controlwire

import (
	"context"
	json "encoding/json/v2"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// RoutedJSONRequest is one product-owned control request that projects the
// route and nonce already carried by its signed body. The transport never asks
// a caller to repeat either fact beside the document and therefore cannot send
// a valid body to a different control route or under a different idempotency
// identity.
type RoutedJSONRequest interface {
	core.ValidatedJSONMarshaler
	ControlRoute() (RouteContract, error)
	ControlRevision() Revision
	ControlNonce() RequestNonce
	ControlRequestBodyLimit() (core.ByteCount, error)
}

// AuthenticatedResponseProjection is an authority-issued response whose
// compiler-owned authentication contract is closed before HTTP output.
type AuthenticatedResponseProjection interface {
	core.ValidatedJSONProjection
	ControlResponseProjection()
}

// AuthenticatedResponseDocument is a structurally authenticated response
// admitted from external JSON. Its owner still performs caller-selected trust
// and request-binding verification before exposing the product body.
type AuthenticatedResponseDocument interface {
	core.Validatable
	json.Unmarshaler
	ControlResponseDocument()
}

// ClientConfiguration binds the installed-tool side of the control socket to
// one bounded Exchange client and one authority origin. It carries no protocol
// support, signing authority, handler, or response writer.
type ClientConfiguration struct {
	Exchange  exchange.Client
	Authority core.HTTPEndpoint
}

// Validate closes every installed-tool execution dependency.
func (c ClientConfiguration) Validate() error {
	if err := c.Exchange.Validate(); err != nil {
		return contractError(err)
	}
	return validateAuthorityOrigin(c.Authority)
}

// Client is the opaque installed-tool capability for the paired control
// socket. Its private configuration prevents callers from assembling a send
// from loose transport and authority facts at the operation boundary.
type Client struct {
	configuration ClientConfiguration
}

// NewClient closes one installed-tool control socket capability.
func NewClient(configuration ClientConfiguration) (Client, error) {
	if err := configuration.Validate(); err != nil {
		return Client{}, err
	}
	return Client{configuration: configuration}, nil
}

// Validate rejects the zero capability and revalidates its owned dependencies.
func (c Client) Validate() error {
	return c.configuration.Validate()
}

// AuthorityConfiguration binds the authority side of the control socket to its
// exact supported protocol set. It carries no client transport or authority
// endpoint.
type AuthorityConfiguration struct {
	Support ProtocolSupport
}

// Validate closes the authority-side protocol dependency.
func (c AuthorityConfiguration) Validate() error {
	if err := c.Support.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// Authority is the opaque authority capability for the paired control socket.
type Authority struct {
	configuration AuthorityConfiguration
}

// NewAuthority closes one authority control socket capability.
func NewAuthority(configuration AuthorityConfiguration) (Authority, error) {
	if err := configuration.Validate(); err != nil {
		return Authority{}, err
	}
	return Authority{configuration: configuration}, nil
}

// Validate rejects the zero capability and revalidates its protocol support.
func (a Authority) Validate() error {
	return a.configuration.Validate()
}

// ClientJSONCall is one complete client-side control exchange. Authority must
// be an origin: the request document owns the complete path through its route.
type ClientJSONCall[Body RoutedJSONRequest] struct {
	Context context.Context
	Body    Body
	Client  Client
}

// AuthorityJSONReceiveCall is one authority-side receive boundary mounted for
// an exact route. The real request path must equal that route and the decoded
// document must independently project the same route.
type AuthorityJSONReceiveCall struct {
	Call      exchange.SocketServerCall
	Route     RouteContract
	Authority Authority
}

// RoutedJSONReceive is the authority-side transport result. The exact decoded
// request and its route/revision support assessment travel together, so no
// handler can accidentally skip compatibility policy after ingress.
type RoutedJSONReceive[Body RoutedJSONRequest] struct {
	exchange.Received[Body]
	Replay     ReplayIdentity
	Assessment ProtocolAssessment
}

// ControlJSONWriteCall is one authority-side successful control response.
// Every control response is a typed JSON document and every successful route
// uses the same compiler-owned status and document ceiling.
type ControlJSONWriteCall[Body AuthenticatedResponseProjection] struct {
	Call      exchange.SocketServerCall
	Body      Body
	Authority Authority
}

// SendRoutedJSON executes one request against the route and nonce projected by
// its body and strictly decodes one typed response.
func SendRoutedJSON[
	RequestBody RoutedJSONRequest,
	ResponseBody any,
	ResponsePtr interface {
		*ResponseBody
		AuthenticatedResponseDocument
	},
](call ClientJSONCall[RequestBody]) (exchange.JSONResponse[ResponsePtr], error) {
	var zero exchange.JSONResponse[ResponsePtr]
	request, policy, err := clientExchange(call)
	if err != nil {
		return zero, err
	}
	return exchange.SendJSON[RequestBody, ResponsePtr](exchange.JSONCall[RequestBody]{
		Context: call.Context,
		Client:  call.Client.configuration.Exchange,
		Request: request,
		Policy:  policy,
	})
}

// ReceiveRoutedJSON receives one strict request document, then binds the real
// path and idempotency field to the independently projected facts in that
// document.
func ReceiveRoutedJSON[
	Body any,
	BodyPtr interface {
		*Body
		RoutedJSONRequest
	},
](call AuthorityJSONReceiveCall) (RoutedJSONReceive[BodyPtr], error) {
	var zero RoutedJSONReceive[BodyPtr]
	if err := call.Validate(); err != nil {
		return zero, err
	}
	bodyLimit, err := BodyPtr(new(Body)).ControlRequestBodyLimit()
	if err != nil {
		return zero, err
	}
	policy, err := controlServerPolicy(bodyLimit)
	if err != nil {
		return zero, err
	}
	received, err := exchange.ReceiveJSON[Body, BodyPtr](exchange.JSONReceiveCall{
		Call: call.Call, Route: controlRouteSemantics(), Policy: policy,
	})
	if err != nil {
		return zero, err
	}
	if err := bindReceivedRequest(call.Route, received); err != nil {
		return zero, err
	}
	replay, err := CommitReplayIdentity(received.Body)
	if err != nil {
		return zero, err
	}
	assessment, err := assessReceivedProtocol(call.Authority.configuration.Support, received.Body)
	if err != nil {
		return zero, err
	}
	result := RoutedJSONReceive[BodyPtr]{
		Received: received, Assessment: assessment, Replay: replay,
	}
	return result, result.Validate()
}

// WriteControlJSON emits one strictly bounded successful response.
func WriteControlJSON[
	Body AuthenticatedResponseProjection,
](call ControlJSONWriteCall[Body]) error {
	if err := call.Authority.Validate(); err != nil {
		return err
	}
	policy, err := controlJSONWritePolicy()
	if err != nil {
		return err
	}
	return exchange.WriteJSON(exchange.JSONWriteCall[Body]{
		Call: call.Call,
		Response: exchange.ServerJSONResponse[Body]{
			Body: call.Body, Status: core.HTTPStatusOK(),
		},
		Policy: policy,
	})
}

// Validate closes the authority ingress and binds the real request path to the
// mounted compiler-owned route before Exchange reads a byte of the body.
func (call AuthorityJSONReceiveCall) Validate() error {
	if err := call.Call.Validate(); err != nil {
		return routeError(err)
	}
	if err := call.Route.Validate(); err != nil {
		return err
	}
	if err := call.Authority.Validate(); err != nil {
		return err
	}
	pathText, err := call.Route.Path()
	if err != nil {
		return err
	}
	path, err := exchange.ParseSocketRoutePath(pathText)
	if err != nil {
		return routeError(err)
	}
	if err := exchange.ValidateSocketCallPath(call.Call, path); err != nil {
		return routeError(err)
	}
	return nil
}

// Validate closes both authority outputs and proves that the assessment names
// the exact route/revision pair carried by the received request.
func (r RoutedJSONReceive[Body]) Validate() error {
	if err := r.Received.Validate(); err != nil {
		return protocolSupportError(err)
	}
	if err := r.Assessment.Validate(); err != nil {
		return err
	}
	route, err := r.Received.Body.ControlRoute()
	if err != nil {
		return err
	}
	capability, err := route.ProtocolCapability(r.Received.Body.ControlRevision())
	if err != nil {
		return err
	}
	if r.Assessment.Capability != capability {
		return protocolSupportError()
	}
	wantReplay, err := CommitReplayIdentity(r.Received.Body)
	if err != nil {
		return err
	}
	if r.Replay != wantReplay {
		return contractError()
	}
	return nil
}

func clientExchange[Body RoutedJSONRequest](
	call ClientJSONCall[Body],
) (exchange.JSONRequest[Body], exchange.JSONPolicy, error) {
	var zero exchange.JSONRequest[Body]
	if err := call.Body.Validate(); err != nil {
		return zero, exchange.JSONPolicy{}, err
	}
	if err := call.Client.Validate(); err != nil {
		return zero, exchange.JSONPolicy{}, err
	}
	if call.Context == nil {
		return zero, exchange.JSONPolicy{}, core.ErrExchangeContract
	}
	route, err := call.Body.ControlRoute()
	if err != nil {
		return zero, exchange.JSONPolicy{}, err
	}
	target, err := controlTarget(call.Client.configuration.Authority, route)
	if err != nil {
		return zero, exchange.JSONPolicy{}, err
	}
	semantics, err := route.Semantics(call.Body.ControlNonce())
	if err != nil {
		return zero, exchange.JSONPolicy{}, err
	}
	policy, err := ControlExchangePolicy()
	if err != nil {
		return zero, exchange.JSONPolicy{}, err
	}
	request := exchange.JSONRequest[Body]{
		Target: target, Body: call.Body, Semantics: semantics,
		ExpectedStatus: core.HTTPStatusOK(),
	}
	if err := request.Validate(); err != nil {
		return zero, exchange.JSONPolicy{}, err
	}
	return request, policy, nil
}

func controlTarget(
	authority core.HTTPEndpoint,
	route RouteContract,
) (core.HTTPEndpoint, error) {
	if err := validateAuthorityOrigin(authority); err != nil {
		return core.HTTPEndpoint{}, err
	}
	address := authority.HTTPURL()
	path, err := route.Path()
	if err != nil {
		return core.HTTPEndpoint{}, err
	}
	address.Path = path
	target, err := core.ParseHTTPEndpoint(address.String())
	if err != nil {
		return core.HTTPEndpoint{}, routeError(err)
	}
	return target, nil
}

func validateAuthorityOrigin(authority core.HTTPEndpoint) error {
	if err := authority.Validate(); err != nil {
		return routeError(err)
	}
	address := authority.HTTPURL()
	if (address.Path != "" && address.Path != "/") || address.RawPath != "" ||
		address.RawQuery != "" || address.ForceQuery {
		return routeError(core.ErrExchangeContract)
	}
	return nil
}

func bindReceivedRequest[Body RoutedJSONRequest](
	expected RouteContract,
	received exchange.Received[Body],
) error {
	actual, err := received.Body.ControlRoute()
	if err != nil {
		return err
	}
	if actual.Offering() != expected.Offering() || actual.Family() != expected.Family() {
		return routeError(core.ErrExchangeContract)
	}
	wantKey, err := received.Body.ControlNonce().IdempotencyKey()
	if err != nil {
		return err
	}
	if received.IdempotencyKey.String() != wantKey.String() {
		return nonceError(core.ErrExchangeContract)
	}
	return nil
}

func assessReceivedProtocol[Body RoutedJSONRequest](
	support ProtocolSupport,
	body Body,
) (ProtocolAssessment, error) {
	route, err := body.ControlRoute()
	if err != nil {
		return ProtocolAssessment{}, err
	}
	capability, err := route.ProtocolCapability(body.ControlRevision())
	if err != nil {
		return ProtocolAssessment{}, err
	}
	return AssessProtocol(ProtocolAssessmentRequest{Support: support, Capability: capability})
}

func controlRouteSemantics() exchange.RouteSemantics {
	return exchange.RouteSemantics{
		Method: exchange.MethodPost, Replay: exchange.ReplayIdempotencyKey,
	}
}

func controlServerPolicy(maximum core.ByteCount) (exchange.ServerPolicy, error) {
	policy := exchange.ServerPolicy{RequestBodyLimit: maximum}
	if err := policy.Validate(); err != nil {
		return exchange.ServerPolicy{}, contractError(err)
	}
	return policy, nil
}

func controlJSONWritePolicy() (exchange.JSONWritePolicy, error) {
	maximum, err := core.NewByteCount(core.JSONDocumentMaximumBytes)
	if err != nil {
		return exchange.JSONWritePolicy{}, contractError(err)
	}
	return exchange.JSONWritePolicy{ResponseBodyLimit: maximum}, nil
}

var (
	_ core.Validatable = AuthorityJSONReceiveCall{}
)
