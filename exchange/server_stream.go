package exchange

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// ServerBoundedPolicy bounds one aggregate received body.
type ServerBoundedPolicy struct {
	RequestBodyLimit core.ByteCount
}

// Validate admits one positive aggregate bound.
func (p ServerBoundedPolicy) Validate() error {
	if _, err := p.RequestBodyLimit.Int64(); err != nil {
		return core.ErrExchangeContract
	}
	return nil
}

// ServerStreamPolicy bounds one streamed received body.
type ServerStreamPolicy struct {
	RequestBodyLimit core.ByteCount
}

// Validate admits one positive standard-library-representable stream bound.
func (p ServerStreamPolicy) Validate() error {
	_, err := p.RequestBodyLimit.Int64()
	if err != nil {
		return core.ErrExchangeContract
	}
	return nil
}

// BoundedReceiveCall supplies one aggregate byte server boundary.
type BoundedReceiveCall struct {
	Request             *http.Request
	ExpectedContentType core.HTTPMediaType
	Policy              ServerBoundedPolicy
	Route               RouteSemantics
}

// StreamReceiveCall supplies one O(1)-memory server receive boundary.
type StreamReceiveCall struct {
	Destination         io.Writer
	Request             *http.Request
	ExpectedContentType core.HTTPMediaType
	Policy              ServerStreamPolicy
	Route               RouteSemantics
}

// ReceivedBytes is one bounded body plus its optional idempotency key.
type ReceivedBytes struct {
	IdempotencyKey IdempotencyKey
	Body           []byte
}

// Validate checks the optional key. The body may be empty because structural
// body presence is carried by the receive operation, not inferred from length.
func (r ReceivedBytes) Validate() error {
	if r.IdempotencyKey.IsZero() {
		return nil
	}
	return r.IdempotencyKey.Validate()
}

// ReceivedStream reports one completed server-side stream.
type ReceivedStream struct {
	IdempotencyKey IdempotencyKey
	Bytes          core.ByteLength
}

// Validate checks the optional key.
func (r ReceivedStream) Validate() error {
	if r.IdempotencyKey.IsZero() {
		return nil
	}
	return r.IdempotencyKey.Validate()
}

// ReceiveBounded reads and closes one aggregate body.
func ReceiveBounded(
	call BoundedReceiveCall,
) (ReceivedBytes, error) {
	return executeRequestBodyOperation(
		call.Request,
		func() (ReceivedBytes, error) {
			return receiveBounded(call)
		},
	)
}

func receiveBounded(call BoundedReceiveCall) (ReceivedBytes, error) {
	if err := call.Validate(); err != nil {
		return ReceivedBytes{}, err
	}
	key, err := receiveIdempotencyKey(call.Request, call.Route)
	if err != nil {
		return ReceivedBytes{}, err
	}
	limit, _ := call.Policy.RequestBodyLimit.Int64()
	if err := validateRequestBodyLength(
		call.Request.ContentLength,
		limit,
	); err != nil {
		return ReceivedBytes{}, err
	}
	body, readErr := readBoundedBody(
		call.Request.Context(),
		call.Request.Body,
		call.Policy.RequestBodyLimit,
	)
	if readErr != nil {
		return ReceivedBytes{}, asRequestReadError(readErr)
	}
	received := ReceivedBytes{Body: body, IdempotencyKey: key}
	return received, received.Validate()
}

// ReceiveStream copies one request body into the caller-owned destination,
// writes no more than the configured limit, probes one excess byte, and closes
// the request body.
func ReceiveStream(
	call StreamReceiveCall,
) (ReceivedStream, error) {
	return executeRequestBodyOperation(
		call.Request,
		func() (ReceivedStream, error) {
			return receiveStream(call)
		},
	)
}

func receiveStream(call StreamReceiveCall) (ReceivedStream, error) {
	if err := call.Validate(); err != nil {
		return ReceivedStream{}, err
	}
	key, err := receiveIdempotencyKey(call.Request, call.Route)
	if err != nil {
		return ReceivedStream{}, err
	}
	limit, _ := call.Policy.RequestBodyLimit.Int64()
	if err := validateRequestBodyLength(
		call.Request.ContentLength,
		limit,
	); err != nil {
		return ReceivedStream{}, err
	}
	bytes, copyErr := copyDownload(
		downloadCopyRequest{
			context: call.Request.Context(),
			source:  call.Request.Body, destination: call.Destination,
			limit: call.Policy.RequestBodyLimit,
		},
	)
	received := ReceivedStream{
		Bytes: core.NewByteLength(bytes), IdempotencyKey: key,
	}
	if copyErr != nil {
		return received, asRequestReadError(copyErr)
	}
	return received, received.Validate()
}

// Validate checks one aggregate byte server receive boundary.
func (call BoundedReceiveCall) Validate() error {
	if err := validateServerIngress(call.Request, call.Route); err != nil {
		return err
	}
	if err := call.Policy.Validate(); err != nil {
		return requestError(err)
	}
	return validateRawRequestMetadata(
		rawRequestMetadata{
			request:             call.Request,
			expectedContentType: call.ExpectedContentType,
		},
	)
}

// Validate checks one streaming server receive boundary.
func (call StreamReceiveCall) Validate() error {
	if call.Destination == nil {
		return requestError(core.ErrExchangeContract)
	}
	if err := validateServerIngress(call.Request, call.Route); err != nil {
		return err
	}
	if err := call.Policy.Validate(); err != nil {
		return requestError(err)
	}
	return validateRawRequestMetadata(
		rawRequestMetadata{
			request:             call.Request,
			expectedContentType: call.ExpectedContentType,
		},
	)
}

type rawRequestMetadata struct {
	request             *http.Request
	expectedContentType core.HTTPMediaType
}

func validateRawRequestMetadata(input rawRequestMetadata) error {
	if input.request.Body == nil {
		return requestError(core.ErrExchangeContract)
	}
	if err := validateRequestContentCoding(input.request.Header); err != nil {
		return err
	}
	if input.expectedContentType.IsZero() {
		return nil
	}
	values := input.request.Header.Values(
		core.HTTPHeaderContentType().String(),
	)
	if len(values) != 1 {
		return requestError(core.ErrExchangeContentType)
	}
	actual, err := core.ParseHTTPMediaType(values[0])
	if err != nil {
		return requestError(core.ErrExchangeContentType)
	}
	matches, err := actual.SameBase(input.expectedContentType)
	if err != nil || !matches {
		return requestError(core.ErrExchangeContentType)
	}
	return nil
}

func executeRequestBodyOperation[Result any](
	request *http.Request,
	operation func() (Result, error),
) (Result, error) {
	var result Result
	var operationErr error
	var closeErr error
	func() {
		defer func() {
			closeErr = closeServerRequestBody(request)
		}()
		result, operationErr = operation()
	}()
	return result, errors.Join(operationErr, closeErr)
}

func closeServerRequestBody(request *http.Request) error {
	if request == nil || request.Body == nil {
		return nil
	}
	return closeRequestBody(request.Body)
}

func closeRequestBody(body io.Closer) (err error) {
	defer func() {
		if recover() != nil {
			err = requestError(core.ErrExchangeContract)
		}
	}()
	if err := body.Close(); err != nil {
		return requestError(err)
	}
	return nil
}

// ServerStreamResponse supplies one raw streaming response. ContentLength is
// exact; Exchange writes that many bytes and rejects both truncation and an
// additional source byte.
type ServerStreamResponse struct {
	Source        io.Reader
	ContentType   core.HTTPMediaType
	Headers       ResponseHeaders
	ContentLength core.ByteLength
	Status        core.HTTPStatusCode
}

// Validate checks the complete pre-write streaming response.
func (r ServerStreamResponse) Validate() error {
	if r.Source == nil {
		return responseError(core.ErrExchangeContract)
	}
	if _, err := r.ContentLength.Int64(); err != nil {
		return responseError(err)
	}
	if err := r.ContentType.Validate(); err != nil {
		return responseError(core.ErrExchangeContentType)
	}
	if err := r.Headers.Validate(); err != nil {
		return responseError(err)
	}
	if err := r.Status.Validate(); err != nil {
		return responseError(err)
	}
	status, _ := r.Status.Int()
	if !statusPermitsBody(status) {
		return responseError(core.ErrExchangeContract)
	}
	return nil
}

// StreamWriteCall supplies one complete raw streaming response effect.
type StreamWriteCall struct {
	Context  context.Context
	Writer   http.ResponseWriter
	Response ServerStreamResponse
}

// Validate checks one complete streaming response effect.
func (call StreamWriteCall) Validate() error {
	if call.Writer == nil {
		return responseError(core.ErrExchangeContract)
	}
	if err := contextstate.Validate(call.Context); err != nil {
		return responseError(err)
	}
	return call.Response.Validate()
}

// WriteStream writes one exact caller-owned source through ResponseWriter.
func WriteStream(call StreamWriteCall) error {
	if err := call.Validate(); err != nil {
		return err
	}
	return writeServerStream(call)
}

func writeServerStream(call StreamWriteCall) error {
	return executeResponseWriterOperation(func() error {
		applyResponseHeaders(call.Writer.Header(), call.Response.Headers)
		call.Writer.Header().Set(
			core.HTTPHeaderContentType().String(),
			call.Response.ContentType.String(),
		)
		length, _ := call.Response.ContentLength.Int64()
		call.Writer.Header().Set(
			core.HTTPHeaderContentLength().String(),
			strconv.FormatInt(length, 10),
		)
		status, _ := call.Response.Status.Int()
		call.Writer.WriteHeader(status)
		return writeExactStream(call)
	})
}

func writeExactStream(call StreamWriteCall) error {
	length := call.Response.ContentLength.Uint64()
	if length == 0 {
		return probeEmptyResponseSource(call.Response.Source)
	}
	limit, err := core.NewByteCount(length)
	if err != nil {
		return responseError(err)
	}
	written, copyErr := copyDownload(
		downloadCopyRequest{
			context: call.Context, source: call.Response.Source,
			destination: call.Writer, limit: limit,
		},
	)
	if copyErr != nil {
		return errors.Join(
			core.ErrExchangeResponse,
			core.ErrExchangeWrite,
			copyErr,
		)
	}
	if written != length {
		return errors.Join(
			core.ErrExchangeResponse,
			core.ErrExchangeWrite,
			io.ErrUnexpectedEOF,
		)
	}
	return nil
}

func probeEmptyResponseSource(source io.Reader) error {
	var probe [1]byte
	read, err := source.Read(probe[:])
	if read < 0 || read > len(probe) {
		return errors.Join(
			core.ErrExchangeResponse,
			core.ErrExchangeWrite,
			core.ErrExchangeContract,
		)
	}
	if read > 0 {
		return errors.Join(
			core.ErrExchangeResponse,
			core.ErrExchangeWrite,
			core.ErrExchangeBodyLimit,
		)
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return errors.Join(core.ErrExchangeResponse, core.ErrExchangeWrite, err)
	}
	return nil
}

// ServerBoundedResponse supplies one aggregate byte response.
type ServerBoundedResponse struct {
	ContentType core.HTTPMediaType
	Body        []byte
	Headers     ResponseHeaders
	Status      core.HTTPStatusCode
}

// Validate checks the complete pre-write bounded response.
func (r ServerBoundedResponse) Validate() error {
	if err := r.ContentType.Validate(); err != nil {
		return responseError(core.ErrExchangeContentType)
	}
	if err := r.Headers.Validate(); err != nil {
		return responseError(err)
	}
	if err := r.Status.Validate(); err != nil {
		return responseError(err)
	}
	status, _ := r.Status.Int()
	if !statusPermitsBody(status) {
		return responseError(core.ErrExchangeContract)
	}
	return nil
}

// BoundedWriteCall supplies one complete aggregate byte response effect.
type BoundedWriteCall struct {
	Context  context.Context
	Writer   http.ResponseWriter
	Response ServerBoundedResponse
}

// Validate checks one complete aggregate byte response effect.
func (call BoundedWriteCall) Validate() error {
	if call.Writer == nil {
		return responseError(core.ErrExchangeContract)
	}
	if err := contextstate.Validate(call.Context); err != nil {
		return responseError(err)
	}
	return call.Response.Validate()
}

// WriteBounded writes one bounded byte response without changing its content.
func WriteBounded(call BoundedWriteCall) error {
	if err := call.Validate(); err != nil {
		return err
	}
	return writeServerBounded(call)
}

func writeServerBounded(call BoundedWriteCall) error {
	return executeResponseWriterOperation(func() error {
		applyResponseHeaders(call.Writer.Header(), call.Response.Headers)
		call.Writer.Header().Set(
			core.HTTPHeaderContentType().String(),
			call.Response.ContentType.String(),
		)
		call.Writer.Header().Set(
			core.HTTPHeaderContentLength().String(),
			strconv.Itoa(len(call.Response.Body)),
		)
		status, _ := call.Response.Status.Int()
		call.Writer.WriteHeader(status)
		written, writeErr := call.Writer.Write(call.Response.Body)
		if writeErr != nil {
			return errors.Join(
				core.ErrExchangeResponse,
				core.ErrExchangeWrite,
				writeErr,
			)
		}
		if written != len(call.Response.Body) {
			return errors.Join(
				core.ErrExchangeResponse,
				core.ErrExchangeWrite,
				io.ErrShortWrite,
			)
		}
		return nil
	})
}

var (
	_ core.Validatable = ServerBoundedPolicy{}
	_ core.Validatable = ServerStreamPolicy{}
	_ core.Validatable = BoundedReceiveCall{}
	_ core.Validatable = StreamReceiveCall{}
	_ core.Validatable = ReceivedBytes{}
	_ core.Validatable = ReceivedStream{}
	_ core.Validatable = ServerStreamResponse{}
	_ core.Validatable = StreamWriteCall{}
	_ core.Validatable = ServerBoundedResponse{}
	_ core.Validatable = BoundedWriteCall{}
)
