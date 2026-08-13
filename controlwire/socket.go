package controlwire

import (
	"context"
	"encoding/json"
	"net/http"

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

// ClientJSONCall is one complete client-side control exchange. Authority must
// be an origin: the request document owns the complete path through its route.
type ClientJSONCall[Body RoutedJSONRequest] struct {
	Context   context.Context
	Client    exchange.Client
	Authority core.HTTPEndpoint
	Body      Body
}

// AuthorityJSONReceiveCall is one authority-side receive boundary mounted for
// an exact route. The real request path must equal that route and the decoded
// document must independently project the same route.
type AuthorityJSONReceiveCall struct {
	Request *http.Request
	Route   RouteContract
}

// ControlJSONWriteCall is one authority-side successful control response.
// Every control response is a typed JSON document and every successful route
// uses the same compiler-owned status and document ceiling.
type ControlJSONWriteCall[Body AuthenticatedResponseProjection] struct {
	Writer http.ResponseWriter
	Body   Body
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
		Client:  call.Client,
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
](call AuthorityJSONReceiveCall) (exchange.Received[BodyPtr], error) {
	var zero exchange.Received[BodyPtr]
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
		Request: call.Request,
		Route:   controlRouteSemantics(),
		Policy:  policy,
	})
	if err != nil {
		return zero, err
	}
	if err := bindReceivedRequest(call.Route, received); err != nil {
		return zero, err
	}
	return received, nil
}

// WriteControlJSON emits one strictly bounded successful response.
func WriteControlJSON[
	Body AuthenticatedResponseProjection,
](call ControlJSONWriteCall[Body]) error {
	policy, err := controlJSONWritePolicy()
	if err != nil {
		return err
	}
	return exchange.WriteJSON(exchange.JSONWriteCall[Body]{
		Writer: call.Writer,
		Response: exchange.ServerJSONResponse[Body]{
			Body: call.Body, Status: core.HTTPStatusOK(),
		},
		Policy: policy,
	})
}

// Validate closes the authority ingress and binds the real request path to the
// mounted compiler-owned route before Exchange reads a byte of the body.
func (call AuthorityJSONReceiveCall) Validate() error {
	if call.Request == nil || call.Request.URL == nil {
		return routeError(core.ErrExchangeContract)
	}
	if err := call.Route.Validate(); err != nil {
		return err
	}
	path, err := call.Route.Path()
	if err != nil {
		return err
	}
	if call.Request.URL.Path != path || call.Request.URL.RawPath != "" ||
		call.Request.URL.RawQuery != "" || call.Request.URL.ForceQuery {
		return routeError(core.ErrExchangeContract)
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
	target, err := controlTarget(call.Authority, route)
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
	if err := authority.Validate(); err != nil {
		return core.HTTPEndpoint{}, routeError(err)
	}
	address := authority.HTTPURL()
	if (address.Path != "" && address.Path != "/") || address.RawPath != "" ||
		address.RawQuery != "" || address.ForceQuery {
		return core.HTTPEndpoint{}, routeError(core.ErrExchangeContract)
	}
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

func controlRouteSemantics() exchange.RouteSemantics {
	return exchange.RouteSemantics{
		Method: exchange.MethodPost, Replay: exchange.ReplayIdempotencyKey,
	}
}

func controlServerPolicy(maximum core.ByteCount) (exchange.ServerPolicy, error) {
	policy := exchange.ServerPolicy{RequestBodyLimit: maximum}
	if err := policy.Validate(); err != nil {
		return exchange.ServerPolicy{}, exchangePolicyError(err)
	}
	return policy, nil
}

func controlJSONWritePolicy() (exchange.JSONWritePolicy, error) {
	maximum, err := core.NewByteCount(core.JSONDocumentMaximumBytes)
	if err != nil {
		return exchange.JSONWritePolicy{}, exchangePolicyError(err)
	}
	return exchange.JSONWritePolicy{ResponseBodyLimit: maximum}, nil
}

var (
	_ core.Validatable = AuthorityJSONReceiveCall{}
)
