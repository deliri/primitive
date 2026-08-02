package exchange

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/deliri/primitive/v2026/core"
)

// RouteSemantics owns the method and replay contract expected by a server
// route. An idempotency key is observed from the real request, not configured
// in the route.
type RouteSemantics struct {
	Method Method
	Replay ReplayMode
}

// Validate closes the route method/replay lattice.
func (s RouteSemantics) Validate() error {
	if err := s.Method.Validate(); err != nil {
		return core.ErrExchangeContract
	}
	if err := s.Replay.Validate(); err != nil {
		return err
	}
	return validateReplayMethod(s.Method, s.Replay)
}

// ServerPolicy bounds one received strict JSON document.
type ServerPolicy struct {
	RequestBodyLimit core.ByteCount
}

// Validate enforces Core's strict JSON maximum.
func (p ServerPolicy) Validate() error {
	return validateJSONLimit(p.RequestBodyLimit)
}

// JSONWritePolicy bounds one emitted strict JSON document.
type JSONWritePolicy struct {
	ResponseBodyLimit core.ByteCount
}

// Validate enforces Core's strict JSON maximum.
func (p JSONWritePolicy) Validate() error {
	return validateJSONLimit(p.ResponseBodyLimit)
}

// NoBody is the compiler-visible absence of an HTTP body.
type NoBody struct{}

// Validate admits the sole no-body value.
func (NoBody) Validate() error { return nil }

// Received is one validated request body plus any validated idempotency key
// required by the route.
type Received[Body core.Validatable] struct {
	Body           Body
	IdempotencyKey IdempotencyKey
}

// Validate checks the complete received value.
func (r Received[Body]) Validate() error {
	if err := validateCallerValue(r.Body); err != nil {
		return requestError(err)
	}
	if !r.IdempotencyKey.IsZero() {
		if err := r.IdempotencyKey.Validate(); err != nil {
			return requestError(err)
		}
	}
	return nil
}

// JSONReceiveCall supplies one body-only server receive boundary.
type JSONReceiveCall struct {
	Request *http.Request
	Route   RouteSemantics
	Policy  ServerPolicy
}

// JSONProjector completes a decoded wire structure with typed state from the
// real HTTP request before the owning body validates.
type JSONProjector[
	Body any,
	BodyPtr interface {
		*Body
		core.Validatable
	},
] func(context.Context, *http.Request, BodyPtr) error

// ProjectedJSONReceiveCall supplies one strict decode/project/validate
// boundary. Project must be non-nil.
type ProjectedJSONReceiveCall[
	Body any,
	BodyPtr interface {
		*Body
		core.Validatable
	},
] struct {
	Request *http.Request
	Project JSONProjector[Body, BodyPtr]
	Policy  ServerPolicy
	Route   RouteSemantics
}

// NoBodyReceiveCall supplies one server boundary that forbids a body.
type NoBodyReceiveCall struct {
	Request *http.Request
	Route   RouteSemantics
}

// Validate checks one strict JSON server receive boundary before reading.
func (call JSONReceiveCall) Validate() error {
	if err := validateServerIngress(call.Request, call.Route); err != nil {
		return err
	}
	if err := call.Policy.Validate(); err != nil {
		return requestError(err)
	}
	return validateJSONRequestMetadata(call.Request)
}

// Validate checks one projected strict JSON server receive boundary.
func (call ProjectedJSONReceiveCall[Body, BodyPtr]) Validate() error {
	if call.Project == nil {
		return requestError(core.ErrExchangeContract)
	}
	return JSONReceiveCall{
		Request: call.Request,
		Route:   call.Route,
		Policy:  call.Policy,
	}.Validate()
}

// Validate checks one body-absent server receive boundary.
func (call NoBodyReceiveCall) Validate() error {
	return validateServerIngress(call.Request, call.Route)
}

// ReceiveJSON reads, closes, strictly decodes, and validates one request body.
func ReceiveJSON[
	Body any,
	BodyPtr interface {
		*Body
		core.Validatable
	},
](call JSONReceiveCall) (Received[BodyPtr], error) {
	return executeRequestBodyOperation(
		call.Request,
		func() (Received[BodyPtr], error) {
			return receiveJSON[Body, BodyPtr](call)
		},
	)
}

func receiveJSON[
	Body any,
	BodyPtr interface {
		*Body
		core.Validatable
	},
](call JSONReceiveCall) (Received[BodyPtr], error) {
	var zero Received[BodyPtr]
	if err := call.Validate(); err != nil {
		return zero, err
	}
	data, key, err := receiveValidatedJSONDocument(call)
	if err != nil {
		return zero, err
	}
	body, err := core.DecodeStrictJSON[BodyPtr](
		data,
		strictJSONLimits(call.Policy.RequestBodyLimit),
	)
	if err != nil {
		return zero, requestError(err)
	}
	received := Received[BodyPtr]{Body: body, IdempotencyKey: key}
	if err := received.Validate(); err != nil {
		return zero, err
	}
	return received, nil
}

// ReceiveProjectedJSON reads and strictly decodes one private temporary,
// projects request-owned state into it, validates the completed body, and only
// then returns it.
func ReceiveProjectedJSON[
	Body any,
	BodyPtr interface {
		*Body
		core.Validatable
	},
](call ProjectedJSONReceiveCall[Body, BodyPtr]) (
	Received[BodyPtr],
	error,
) {
	return executeRequestBodyOperation(
		call.Request,
		func() (Received[BodyPtr], error) {
			return receiveProjectedJSON(call)
		},
	)
}

func receiveProjectedJSON[
	Body any,
	BodyPtr interface {
		*Body
		core.Validatable
	},
](call ProjectedJSONReceiveCall[Body, BodyPtr]) (
	Received[BodyPtr],
	error,
) {
	var zero Received[BodyPtr]
	if err := call.Validate(); err != nil {
		return zero, err
	}
	data, key, err := receiveValidatedJSONDocument(
		JSONReceiveCall{
			Request: call.Request, Route: call.Route, Policy: call.Policy,
		},
	)
	if err != nil {
		return zero, err
	}
	body, err := core.DecodeStrictJSONStructure[Body](
		data,
		strictJSONLimits(call.Policy.RequestBodyLimit),
	)
	if err != nil {
		return zero, requestError(err)
	}
	bodyPtr := BodyPtr(&body)
	if err := projectReceivedBody(
		projectionRequest[Body, BodyPtr]{
			context: call.Request.Context(), request: call.Request,
			body: bodyPtr, project: call.Project,
		},
	); err != nil {
		return zero, err
	}
	received := Received[BodyPtr]{Body: bodyPtr, IdempotencyKey: key}
	if err := received.Validate(); err != nil {
		return zero, err
	}
	return received, nil
}

type projectionRequest[
	Body any,
	BodyPtr interface {
		*Body
		core.Validatable
	},
] struct {
	context context.Context
	request *http.Request
	body    BodyPtr
	project JSONProjector[Body, BodyPtr]
}

func projectReceivedBody[
	Body any,
	BodyPtr interface {
		*Body
		core.Validatable
	},
](input projectionRequest[Body, BodyPtr]) (err error) {
	defer func() {
		if recover() != nil {
			err = requestError(core.ErrExchangeContract)
		}
	}()
	if err := input.project(input.context, input.request, input.body); err != nil {
		return requestError(err)
	}
	if err := input.body.Validate(); err != nil {
		return requestError(err)
	}
	return nil
}

// ReceiveNoBody validates request metadata and refuses every body-bearing
// signal before returning the compiler-visible no-body value.
func ReceiveNoBody(
	call NoBodyReceiveCall,
) (Received[NoBody], error) {
	return executeRequestBodyOperation(
		call.Request,
		func() (Received[NoBody], error) {
			return receiveNoBody(call)
		},
	)
}

func receiveNoBody(call NoBodyReceiveCall) (Received[NoBody], error) {
	if err := call.Validate(); err != nil {
		return Received[NoBody]{}, err
	}
	if requestCarriesBody(call.Request) {
		return Received[NoBody]{}, requestError(core.ErrExchangeContract)
	}
	key, err := receiveIdempotencyKey(call.Request, call.Route)
	if err != nil {
		return Received[NoBody]{}, err
	}
	if err := refuseRequestBody(call.Request); err != nil {
		return Received[NoBody]{}, err
	}
	received := Received[NoBody]{
		Body: NoBody{}, IdempotencyKey: key,
	}
	return received, received.Validate()
}

func receiveValidatedJSONDocument(
	call JSONReceiveCall,
) ([]byte, IdempotencyKey, error) {
	key, err := receiveIdempotencyKey(call.Request, call.Route)
	if err != nil {
		return nil, IdempotencyKey{}, err
	}
	declared, err := admittedBodyLength(
		call.Request.ContentLength,
		call.Policy.RequestBodyLimit,
	)
	if err != nil {
		return nil, IdempotencyKey{}, requestError(err)
	}
	data, readErr := readBoundedBody(boundedBodyRead{
		context:  call.Request.Context(),
		source:   call.Request.Body,
		declared: declared,
		limit:    call.Policy.RequestBodyLimit,
	})
	if readErr != nil {
		return nil, IdempotencyKey{}, asRequestReadError(readErr)
	}
	return data, key, nil
}

func validateServerIngress(
	request *http.Request,
	route RouteSemantics,
) error {
	if request == nil {
		return requestError(core.ErrExchangeContract)
	}
	if err := route.Validate(); err != nil {
		return requestError(err)
	}
	if err := exchangeContextError(request.Context()); err != nil {
		return requestError(err)
	}
	method, err := parseMethod(request.Method)
	if err != nil {
		return requestError(err)
	}
	if method != route.Method {
		return requestError(core.ErrExchangeContract)
	}
	return nil
}

func validateJSONRequestMetadata(request *http.Request) error {
	if request.Body == nil {
		return requestError(core.ErrExchangeContract)
	}
	if err := validateRequestContentCoding(request.Header); err != nil {
		return err
	}
	values := request.Header.Values(core.HTTPHeaderContentType().String())
	if len(values) != 1 {
		return requestError(core.ErrExchangeContentType)
	}
	contentType, err := core.ParseHTTPMediaType(values[0])
	if err != nil {
		return requestError(errors.Join(core.ErrExchangeContentType, err))
	}
	jsonType, err := jsonMediaType()
	if err != nil {
		return requestError(err)
	}
	matches, err := contentType.SameBase(jsonType)
	if err != nil {
		return requestError(errors.Join(core.ErrExchangeContentType, err))
	}
	if !matches {
		return requestError(core.ErrExchangeContentType)
	}
	return nil
}

func validateRequestContentCoding(headers http.Header) error {
	values := headers.Values(core.HTTPHeaderContentEncoding().String())
	if len(values) == 0 {
		return nil
	}
	if len(values) != 1 {
		return requestError(core.ErrExchangeContentType)
	}
	coding, err := parseHTTPContentCoding(values[0])
	if err != nil {
		return requestError(errors.Join(core.ErrExchangeContentType, err))
	}
	if coding != identityContentCoding() {
		return requestError(core.ErrExchangeContentType)
	}
	return nil
}

func requestCarriesBody(request *http.Request) bool {
	return request.ContentLength != 0 ||
		len(request.TransferEncoding) != 0 ||
		len(request.Header.Values(
			core.HTTPHeaderContentType().String(),
		)) != 0 ||
		len(request.Header.Values(
			core.HTTPHeaderContentEncoding().String(),
		)) != 0
}

func refuseRequestBody(request *http.Request) error {
	if request.Body == nil {
		return nil
	}
	var probe [1]byte
	read, readErr := io.ReadFull(&progressReader{
		context: request.Context(), source: request.Body,
	}, probe[:])
	if read > 0 {
		return requestError(core.ErrExchangeContract)
	}
	if errors.Is(readErr, io.EOF) {
		return nil
	}
	if readErr != nil {
		return requestError(readErr)
	}
	return requestError(core.ErrExchangeContract)
}

func receiveIdempotencyKey(
	request *http.Request,
	route RouteSemantics,
) (IdempotencyKey, error) {
	values := request.Header.Values(
		core.HTTPHeaderIdempotencyKey().String(),
	)
	if route.Replay != ReplayIdempotencyKey {
		if len(values) != 0 {
			return IdempotencyKey{}, requestError(core.ErrExchangeContract)
		}
		return IdempotencyKey{}, nil
	}
	if len(values) != 1 {
		return IdempotencyKey{}, requestError(core.ErrExchangeContract)
	}
	key, err := ParseIdempotencyKey(values[0])
	if err != nil {
		return IdempotencyKey{}, requestError(err)
	}
	return key, nil
}

func asRequestReadError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, core.ErrExchangeCancelled) {
		return err
	}
	return requestError(err)
}

// ResponseHeaders is a bounded response-header collection. Exchange emits
// identity representation bytes and therefore forbids Content-Type,
// Content-Length, and Content-Encoding overrides.
type ResponseHeaders struct {
	Values []Header
}

// Validate checks unique fields and protects Exchange-owned framing.
func (h ResponseHeaders) Validate() error {
	if len(h.Values) > HeaderMaximumCount {
		return core.ErrExchangeContract
	}
	for index, header := range h.Values {
		if err := header.Validate(); err != nil ||
			header.Name == core.HTTPHeaderContentType() ||
			header.Name == core.HTTPHeaderContentLength() ||
			header.Name == core.HTTPHeaderContentEncoding() {
			return core.ErrExchangeContract
		}
		for prior := range index {
			if h.Values[prior].Name == header.Name {
				return core.ErrExchangeContract
			}
		}
	}
	return nil
}

// ServerJSONResponse supplies one typed JSON response.
type ServerJSONResponse[Body core.ValidatedJSONMarshaler] struct {
	Body    Body
	Headers ResponseHeaders
	Status  core.HTTPStatusCode
}

// Validate checks the complete pre-write response.
func (r ServerJSONResponse[Body]) Validate() error {
	if err := validateCallerValue(r.Body); err != nil {
		return responseError(err)
	}
	if err := r.Headers.Validate(); err != nil {
		return responseError(err)
	}
	if err := r.Status.Validate(); err != nil {
		return responseError(err)
	}
	if !r.Status.PermitsResponseBody() {
		return responseError(core.ErrExchangeContract)
	}
	return nil
}

// ServerNoBodyResponse supplies one body-absent response.
type ServerNoBodyResponse struct {
	Headers ResponseHeaders
	Status  core.HTTPStatusCode
}

// Validate checks the complete pre-write no-body response.
func (r ServerNoBodyResponse) Validate() error {
	if err := r.Headers.Validate(); err != nil {
		return responseError(err)
	}
	if err := r.Status.Validate(); err != nil {
		return responseError(err)
	}
	return nil
}

// JSONWriteCall supplies one complete typed JSON response effect.
type JSONWriteCall[Body core.ValidatedJSONMarshaler] struct {
	Writer   http.ResponseWriter
	Response ServerJSONResponse[Body]
	Policy   JSONWritePolicy
}

// NoBodyWriteCall supplies one complete body-absent response effect.
type NoBodyWriteCall struct {
	Writer   http.ResponseWriter
	Response ServerNoBodyResponse
}

// Validate checks one complete typed JSON response effect.
func (call JSONWriteCall[Body]) Validate() error {
	if call.Writer == nil {
		return responseError(core.ErrExchangeContract)
	}
	if err := call.Response.Validate(); err != nil {
		return err
	}
	if err := call.Policy.Validate(); err != nil {
		return responseError(err)
	}
	return nil
}

// Validate checks one complete body-absent response effect.
func (call NoBodyWriteCall) Validate() error {
	if call.Writer == nil {
		return responseError(core.ErrExchangeContract)
	}
	return call.Response.Validate()
}

// WriteJSON validates, strictly encodes, and writes one response.
func WriteJSON[
	Body core.ValidatedJSONMarshaler,
](call JSONWriteCall[Body]) error {
	if err := call.Validate(); err != nil {
		return err
	}
	body, err := core.EncodeValidatedJSON(
		call.Response.Body,
		strictJSONLimits(call.Policy.ResponseBodyLimit),
	)
	if err != nil {
		return responseError(err)
	}
	return writeJSONBytes(
		jsonWriteRequest{
			writer: call.Writer, body: body,
			headers: call.Response.Headers, status: call.Response.Status,
		},
	)
}

// WriteNoBody writes one validated body-absent response.
func WriteNoBody(call NoBodyWriteCall) error {
	if err := call.Validate(); err != nil {
		return err
	}
	return writeNoBodyResponse(call)
}

type jsonWriteRequest struct {
	writer  http.ResponseWriter
	body    []byte
	headers ResponseHeaders
	status  core.HTTPStatusCode
}

func writeJSONBytes(request jsonWriteRequest) error {
	return executeResponseWriterOperation(func() error {
		jsonType, err := jsonMediaType()
		if err != nil {
			return err
		}
		applyResponseHeaders(request.writer.Header(), request.headers)
		request.writer.Header().Set(
			core.HTTPHeaderContentType().String(),
			jsonType.String(),
		)
		request.writer.Header().Set(
			core.HTTPHeaderContentLength().String(),
			strconv.Itoa(len(request.body)),
		)
		status, _ := request.status.Int()
		request.writer.WriteHeader(status)
		written, writeErr := request.writer.Write(request.body)
		if writeErr != nil {
			return errors.Join(
				core.ErrExchangeResponse,
				core.ErrExchangeWrite,
				writeErr,
			)
		}
		if written != len(request.body) {
			return errors.Join(
				core.ErrExchangeResponse,
				core.ErrExchangeWrite,
				io.ErrShortWrite,
			)
		}
		return nil
	})
}

func writeNoBodyResponse(call NoBodyWriteCall) error {
	return executeResponseWriterOperation(func() error {
		applyResponseHeaders(call.Writer.Header(), call.Response.Headers)
		call.Writer.Header().Set(
			core.HTTPHeaderContentLength().String(),
			strconv.Itoa(0),
		)
		status, _ := call.Response.Status.Int()
		call.Writer.WriteHeader(status)
		return nil
	})
}

func applyResponseHeaders(
	destination http.Header,
	headers ResponseHeaders,
) {
	for _, header := range headers.Values {
		for _, value := range header.Values {
			destination.Add(header.Name.String(), value)
		}
	}
}

func containResponseWriterPanic(err *error) {
	if recover() != nil {
		*err = errors.Join(
			core.ErrExchangeResponse,
			core.ErrExchangeWrite,
			core.ErrExchangeContract,
		)
	}
}

func executeResponseWriterOperation(operation func() error) error {
	var result error
	func() {
		defer containResponseWriterPanic(&result)
		result = operation()
	}()
	return result
}

var (
	_ core.Validatable = RouteSemantics{}
	_ core.Validatable = ServerPolicy{}
	_ core.Validatable = JSONWritePolicy{}
	_ core.Validatable = NoBody{}
	_ core.Validatable = JSONReceiveCall{}
	_ core.Validatable = NoBodyReceiveCall{}
	_ core.Validatable = ResponseHeaders{}
	_ core.Validatable = ServerNoBodyResponse{}
	_ core.Validatable = NoBodyWriteCall{}
)
