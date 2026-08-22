package exchange

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/keygen"
	"github.com/deliri/primitive/v2026/temporal"
)

// Client is a validated reference to the caller-owned real net/http client.
// Exchange never mutates the referenced client.
type Client struct {
	http *http.Client
}

// NewClient admits one real net/http client.
func NewClient(client *http.Client) (Client, error) {
	candidate := Client{http: client}
	if err := candidate.Validate(); err != nil {
		return Client{}, core.ErrExchangeContract
	}
	return candidate, nil
}

// NewStandardClient produces the client shape NewClient otherwise only
// demands: the standard library's default transport with no caller
// customization and no client-wide timeout, because exchange owns every
// timing policy per operation. Without this door every consumer that wants
// exactly the standard transport imports net/http to write an empty literal.
// A caller with a genuinely customized transport still builds its own client
// and admits it through NewClient.
func NewStandardClient() (Client, error) {
	return NewClient(&http.Client{})
}

// Validate rejects an unset client or a competing client-wide timeout.
func (c Client) Validate() error {
	if c.http == nil || c.http.Timeout != 0 {
		return core.ErrExchangeContract
	}
	return nil
}

// JSONCall supplies one complete typed JSON client operation.
type JSONCall[Body core.ValidatedJSONMarshaler] struct {
	Context context.Context
	Client  Client
	Request JSONRequest[Body]
	Policy  JSONPolicy
}

// NoBodyJSONCall supplies one body-absent request and typed JSON response.
type NoBodyJSONCall struct {
	Context context.Context
	Client  Client
	Request NoBodyRequest
	Policy  NoBodyJSONPolicy
}

// BoundedCall supplies one aggregate byte operation.
type BoundedCall struct {
	Context context.Context
	Client  Client
	Request BoundedRequest
	Policy  BoundedPolicy
}

// NoBodyBoundedCall supplies one body-absent aggregate byte operation.
type NoBodyBoundedCall struct {
	Context context.Context
	Client  Client
	Request NoBodyBoundedRequest
	Policy  NoBodyBoundedPolicy
}

type aggregateRequest struct {
	requestContentType          core.HTTPMediaType
	expectedResponseContentType core.HTTPMediaType
	semantics                   RequestSemantics
	body                        []byte
	headers                     Headers
	capture                     HeaderSelection
	target                      core.HTTPEndpoint
	expectedStatus              core.HTTPStatusCode
}

type aggregateCall struct {
	context context.Context
	client  Client
	request aggregateRequest
	policy  OperationPolicy
	limit   core.ByteCount
}

type aggregateResponse struct {
	body     []byte
	metadata ResponseMetadata
}

type attemptResponse struct {
	retryAfter string
	headers    CapturedHeaders
	body       []byte
	status     core.HTTPStatusCode
}

type retryProgress struct {
	waited   temporal.Duration
	attempts uint64
}

// SendJSON validates and encodes one typed request, then strictly decodes one
// typed response. Both documents are bounded independently.
func SendJSON[
	RequestBody core.ValidatedJSONMarshaler,
	ResponseBody core.Validatable,
](call JSONCall[RequestBody]) (JSONResponse[ResponseBody], error) {
	var zero JSONResponse[ResponseBody]
	if err := call.Validate(); err != nil {
		return zero, err
	}
	requestLimits := strictJSONLimits(call.Policy.RequestBodyLimit)
	body, err := core.EncodeValidatedJSON(call.Request.Body, requestLimits)
	if err != nil {
		return zero, requestError(err)
	}
	target, err := validatedTarget(call.Request.Target)
	if err != nil {
		return zero, err
	}
	jsonType, err := StandardMediaTypeJSON.HTTPMediaType()
	if err != nil {
		return zero, err
	}
	raw, err := executeAggregate(aggregateCall{
		context: call.Context,
		client:  call.Client,
		request: aggregateRequest{
			target: target, body: body, semantics: call.Request.Semantics,
			headers: call.Request.Headers, capture: call.Request.CaptureHeaders,
			requestContentType:          jsonType,
			expectedResponseContentType: jsonType,
			expectedStatus:              call.Request.ExpectedStatus,
		},
		policy: call.Policy.Operation,
		limit:  call.Policy.ResponseBodyLimit,
	})
	if raw.metadata.Attempts == 0 {
		return zero, err
	}
	return decodeJSONResponse[ResponseBody](raw, call.Policy.ResponseBodyLimit, err)
}

// SendNoBodyJSON performs one body-absent request and strictly decodes one
// typed JSON response.
func SendNoBodyJSON[
	ResponseBody core.Validatable,
](call NoBodyJSONCall) (JSONResponse[ResponseBody], error) {
	var zero JSONResponse[ResponseBody]
	if err := call.Validate(); err != nil {
		return zero, err
	}
	target, err := validatedTarget(call.Request.Target)
	if err != nil {
		return zero, err
	}
	jsonType, err := StandardMediaTypeJSON.HTTPMediaType()
	if err != nil {
		return zero, err
	}
	raw, err := executeAggregate(aggregateCall{
		context: call.Context,
		client:  call.Client,
		request: aggregateRequest{
			target: target, semantics: call.Request.Semantics,
			headers: call.Request.Headers, capture: call.Request.CaptureHeaders,
			expectedResponseContentType: jsonType,
			expectedStatus:              call.Request.ExpectedStatus,
		},
		policy: call.Policy.Operation,
		limit:  call.Policy.ResponseBodyLimit,
	})
	if raw.metadata.Attempts == 0 {
		return zero, err
	}
	return decodeJSONResponse[ResponseBody](raw, call.Policy.ResponseBodyLimit, err)
}

// SendBounded performs one aggregate byte request and response. Both are
// bounded; callers needing extent-independent memory use Upload or Download.
func SendBounded(call BoundedCall) (BoundedResponse, error) {
	var zero BoundedResponse
	if err := call.Validate(); err != nil {
		return zero, err
	}
	requestLimit, _ := call.Policy.RequestBodyLimit.Uint64()
	if uint64(len(call.Request.Body)) > requestLimit {
		return zero, requestError(core.ErrExchangeBodyLimit)
	}
	target, err := validatedTarget(call.Request.Target)
	if err != nil {
		return zero, err
	}
	body := call.Request.Body
	if body == nil {
		body = []byte{}
	}
	raw, err := executeAggregate(aggregateCall{
		context: call.Context,
		client:  call.Client,
		request: aggregateRequest{
			target: target, body: body,
			semantics: call.Request.Semantics, headers: call.Request.Headers,
			capture:                     call.Request.CaptureHeaders,
			requestContentType:          call.Request.RequestContentType,
			expectedResponseContentType: call.Request.ExpectedResponseContentType,
			expectedStatus:              call.Request.ExpectedStatus,
		},
		policy: call.Policy.Operation,
		limit:  call.Policy.ResponseBodyLimit,
	})
	if raw.metadata.Attempts == 0 {
		return zero, err
	}
	response := BoundedResponse{Metadata: raw.metadata, Body: raw.body}
	if validationErr := response.Validate(); validationErr != nil {
		return zero, errors.Join(err, validationErr)
	}
	return response, err
}

// SendNoBodyBounded performs one body-absent request and returns a bounded
// aggregate byte response.
func SendNoBodyBounded(
	call NoBodyBoundedCall,
) (BoundedResponse, error) {
	var zero BoundedResponse
	if err := call.Validate(); err != nil {
		return zero, err
	}
	target, err := validatedTarget(call.Request.Target)
	if err != nil {
		return zero, err
	}
	raw, err := executeAggregate(aggregateCall{
		context: call.Context,
		client:  call.Client,
		request: aggregateRequest{
			target: target, semantics: call.Request.Semantics,
			headers:                     call.Request.Headers,
			capture:                     call.Request.CaptureHeaders,
			expectedResponseContentType: call.Request.ExpectedResponseContentType,
			expectedStatus:              call.Request.ExpectedStatus,
		},
		policy: call.Policy.Operation,
		limit:  call.Policy.ResponseBodyLimit,
	})
	if raw.metadata.Attempts == 0 {
		return zero, err
	}
	response := BoundedResponse{
		Metadata: raw.metadata,
		Body:     raw.body,
	}
	if validationErr := response.Validate(); validationErr != nil {
		return zero, errors.Join(err, validationErr)
	}
	return response, err
}

// Validate checks the complete typed JSON client operation.
func (call JSONCall[Body]) Validate() error {
	if err := validateCallIngress(call.Context, call.Client); err != nil {
		return err
	}
	if err := call.Request.Validate(); err != nil {
		return err
	}
	if err := call.Policy.Validate(); err != nil {
		return err
	}
	return validatePolicyForSemantics(
		call.Request.Semantics,
		call.Policy.Operation,
	)
}

// Validate checks the complete body-absent JSON client operation.
func (call NoBodyJSONCall) Validate() error {
	if err := validateCallIngress(call.Context, call.Client); err != nil {
		return err
	}
	if err := call.Request.Validate(); err != nil {
		return err
	}
	if err := call.Policy.Validate(); err != nil {
		return err
	}
	return validatePolicyForSemantics(
		call.Request.Semantics,
		call.Policy.Operation,
	)
}

// Validate checks the complete aggregate byte client operation.
func (call BoundedCall) Validate() error {
	if err := validateCallIngress(call.Context, call.Client); err != nil {
		return err
	}
	if err := call.Request.Validate(); err != nil {
		return err
	}
	if err := call.Policy.Validate(); err != nil {
		return err
	}
	return validatePolicyForSemantics(
		call.Request.Semantics,
		call.Policy.Operation,
	)
}

// Validate checks the complete body-absent aggregate byte client operation.
func (call NoBodyBoundedCall) Validate() error {
	if err := validateCallIngress(call.Context, call.Client); err != nil {
		return err
	}
	if err := call.Request.Validate(); err != nil {
		return err
	}
	if err := call.Policy.Validate(); err != nil {
		return err
	}
	return validatePolicyForSemantics(
		call.Request.Semantics,
		call.Policy.Operation,
	)
}

func validatePolicyForSemantics(
	semantics RequestSemantics,
	policy OperationPolicy,
) error {
	if semantics.Replay == ReplaySingleAttempt &&
		policy.Retry.MaximumAttempts != 1 {
		return requestError(core.ErrExchangeContract)
	}
	return nil
}

func validateCallIngress(ctx context.Context, client Client) error {
	if err := exchangeContextError(ctx); err != nil {
		return requestError(err)
	}
	if err := client.Validate(); err != nil {
		return requestError(err)
	}
	return nil
}

func validatedTarget(target Target) (value core.HTTPEndpoint, err error) {
	defer func() {
		if recover() != nil {
			value = core.HTTPEndpoint{}
			err = requestError(core.ErrExchangeContract)
		}
	}()
	if target == nil {
		return core.HTTPEndpoint{}, requestError(core.ErrExchangeContract)
	}
	if err := target.Validate(); err != nil {
		return core.HTTPEndpoint{}, requestError(err)
	}
	projected := target.HTTPURL()
	endpoint, err := core.ParseHTTPEndpoint(projected.String())
	if err != nil {
		return core.HTTPEndpoint{}, requestError(err)
	}
	return endpoint, nil
}

func strictJSONLimits(documentMaximum core.ByteCount) core.StrictJSONLimits {
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = documentMaximum
	return limits
}

func decodeJSONResponse[Body core.Validatable](
	raw aggregateResponse,
	limit core.ByteCount,
	operationErr error,
) (JSONResponse[Body], error) {
	var zero JSONResponse[Body]
	if operationErr != nil {
		return JSONResponse[Body]{Metadata: raw.metadata}, operationErr
	}
	body, err := core.DecodeStrictJSON[Body](bytes.NewReader(raw.body), strictJSONLimits(limit))
	if err != nil {
		return JSONResponse[Body]{Metadata: raw.metadata}, responseError(err)
	}
	response := JSONResponse[Body]{Metadata: raw.metadata, Body: body}
	if err := response.Validate(); err != nil {
		return zero, err
	}
	return response, nil
}

func executeAggregate(call aggregateCall) (aggregateResponse, error) {
	var zero aggregateResponse
	operationContext, cancel, err := temporal.WithTimeout(
		temporal.TimeoutRequest{
			Parent: call.context, Duration: call.policy.OperationTimeout,
		},
	)
	if err != nil {
		return zero, requestError(err)
	}
	defer cancel()
	client := clientForPolicy(call.client.http, call.policy.Redirect, call.request.target)
	progress := retryProgress{}
	for progress.attempts < call.policy.Retry.MaximumAttempts {
		progress.attempts++
		response, attemptErr := executeAggregateAttempt(
			aggregateAttempt{
				context: operationContext, client: client,
				request: call.request, timeout: call.policy.AttemptTimeout,
				limit: call.limit,
			},
		)
		raw, observationErr := observedAggregateResponse(
			response,
			progress.attempts,
		)
		if attemptErr == nil {
			attemptErr = observationErr
		}
		disposition, decisionErr := classifyAggregateAttempt(
			aggregateAttemptResult{
				operationContext: operationContext, response: response,
				cause: attemptErr, expected: call.request.expectedStatus,
				semantics: call.request.semantics,
				attemptsRemaining: call.policy.Retry.MaximumAttempts -
					progress.attempts,
			},
		)
		switch disposition {
		case attemptComplete:
			return raw, decisionErr
		case attemptExhausted:
			return raw, RetryExhaustedError{
				cause: decisionErr, attempts: progress.attempts,
			}
		case attemptRetry:
		default:
			return raw, requestError(core.ErrExchangeContract)
		}
		progress, err = waitForRetry(
			retryWaitRequest{
				context: operationContext, policy: call.policy.Retry,
				progress: progress, retryAfter: response.retryAfter,
			},
		)
		if err != nil {
			return raw, RetryExhaustedError{
				cause: err, attempts: progress.attempts,
			}
		}
	}
	return zero, RetryExhaustedError{
		cause: core.ErrExchangeTransport, attempts: progress.attempts,
	}
}

func observedAggregateResponse(
	response attemptResponse,
	attempts uint64,
) (aggregateResponse, error) {
	if err := response.status.Validate(); err != nil {
		return aggregateResponse{}, responseError(err)
	}
	responseBytes, err := core.NewByteLength(uint64(len(response.body)))
	if err != nil {
		return aggregateResponse{}, responseError(err)
	}
	result := aggregateResponse{
		metadata: ResponseMetadata{
			Status: response.status, Headers: response.headers,
			Bytes:    responseBytes,
			Attempts: attempts,
		},
		body: response.body,
	}
	if err := result.metadata.Validate(); err != nil {
		return aggregateResponse{}, err
	}
	return result, nil
}

type aggregateAttempt struct {
	context context.Context
	client  *http.Client
	request aggregateRequest
	timeout temporal.Duration
	limit   core.ByteCount
}

func executeAggregateAttempt(input aggregateAttempt) (attemptResponse, error) {
	var zero attemptResponse
	attemptContext, cancel, err := temporal.WithTimeout(
		temporal.TimeoutRequest{
			Parent: input.context, Duration: input.timeout,
		},
	)
	if err != nil {
		return zero, requestError(err)
	}
	defer cancel()
	request, err := newAggregateHTTPRequest(attemptContext, input.request)
	if err != nil {
		return zero, err
	}
	response, err := input.client.Do(request)
	if err != nil {
		if terminal := terminalOperationError(attemptContext); terminal != nil {
			return zero, errors.Join(
				terminal,
				closeHTTPResponse(response),
			)
		}
		return zero, errors.Join(err, closeHTTPResponse(response))
	}
	return readAggregateHTTPResponse(
		aggregateReadRequest{
			context: attemptContext, response: response,
			limit: input.limit, capture: input.request.capture,
			expectedContentType: input.request.expectedResponseContentType,
			expectedStatus:      input.request.expectedStatus,
		},
	)
}

func newAggregateHTTPRequest(
	ctx context.Context,
	input aggregateRequest,
) (*http.Request, error) {
	var body io.Reader
	if input.body != nil {
		body = bytes.NewReader(input.body)
	}
	request, err := http.NewRequestWithContext(
		ctx,
		input.semantics.Method.String(),
		input.target.String(),
		body,
	)
	if err != nil {
		return nil, requestError(err)
	}
	if input.body != nil {
		request.ContentLength = int64(len(input.body))
		request.Header.Set(
			core.HTTPHeaderContentType().String(),
			input.requestContentType.String(),
		)
	}
	if !input.expectedResponseContentType.IsZero() {
		request.Header.Set(
			core.HTTPHeaderAccept().String(),
			input.expectedResponseContentType.String(),
		)
	}
	request.Header.Set(
		core.HTTPHeaderAcceptEncoding().String(),
		identityContentCoding().String(),
	)
	applyRequestHeaders(request, input.headers)
	applyIdempotencyKey(request, input.semantics)
	return request, nil
}

func applyRequestHeaders(request *http.Request, headers Headers) {
	for _, header := range headers.Values {
		for _, value := range header.Values {
			wire, _ := value.Value()
			request.Header.Add(header.Name.String(), wire)
		}
	}
}

func applyIdempotencyKey(request *http.Request, semantics RequestSemantics) {
	if semantics.IdempotencyKey.IsZero() {
		return
	}
	request.Header.Set(
		core.HTTPHeaderIdempotencyKey().String(),
		semantics.IdempotencyKey.String(),
	)
}

type aggregateReadRequest struct {
	context             context.Context
	response            *http.Response
	expectedContentType core.HTTPMediaType
	capture             HeaderSelection
	limit               core.ByteCount
	expectedStatus      core.HTTPStatusCode
}

func readAggregateHTTPResponse(
	input aggregateReadRequest,
) (attemptResponse, error) {
	result, err := readAggregateResponseMetadata(input)
	if err != nil {
		return result, err
	}
	result.body, err = readAggregateResponseBody(input)
	closeErr := closeResponseBody(input.response.Body)
	if err != nil && !errors.Is(err, core.ErrExchangeCancelled) {
		err = responseError(err)
	}
	if err == nil {
		if validationErr := result.headers.Validate(); validationErr != nil {
			err = responseError(validationErr)
		}
	}
	return result, errors.Join(err, closeErr)
}

func readAggregateResponseMetadata(input aggregateReadRequest) (attemptResponse, error) {
	var result attemptResponse
	if input.response == nil || input.response.Body == nil {
		return result, responseError(core.ErrExchangeContract)
	}
	var status core.HTTPStatusCode
	if err := status.AdmitInt(input.response.StatusCode); err != nil {
		return result, errors.Join(
			responseError(err),
			closeResponseBody(input.response.Body),
		)
	}
	result.status = status
	captured, err := captureHeaders(input.response.Header, input.capture)
	if err != nil {
		return attemptResponse{}, errors.Join(
			responseError(err),
			closeResponseBody(input.response.Body),
		)
	}
	result.headers = captured
	retryAfter, err := StandardHeaderRetryAfter.Name()
	if err != nil {
		return result, errors.Join(
			responseError(err),
			closeResponseBody(input.response.Body),
		)
	}
	result.retryAfter = input.response.Header.Get(retryAfter.String())
	if err := validateAggregateResponseHeaders(input, status); err != nil {
		return result, errors.Join(
			err,
			closeResponseBody(input.response.Body),
		)
	}
	return result, nil
}

// validateAggregateResponseHeaders always refuses content transformation. The
// caller-declared media type applies only to success: an unexpected status may
// carry a provider error document with a different representation type.
func validateAggregateResponseHeaders(
	input aggregateReadRequest,
	status core.HTTPStatusCode,
) error {
	if err := validateIdentityContentCoding(input.response.Header); err != nil {
		return err
	}
	if status != input.expectedStatus {
		return nil
	}
	return validateResponseContentType(
		input.response.Header,
		input.expectedContentType,
	)
}

// readAggregateResponseBody reads one bounded response body, reserving the extent
// the response declares. The declaration is only a reservation: the limit still
// bounds what is read, so an understated or absent declaration cannot widen it.
func readAggregateResponseBody(
	input aggregateReadRequest,
) ([]byte, error) {
	declared, err := admittedBodyLength(
		input.response.ContentLength,
		input.limit,
	)
	if err != nil {
		return nil, err
	}
	return readBoundedBody(boundedBodyRead{
		context:  input.context,
		source:   input.response.Body,
		declared: declared,
		limit:    input.limit,
	})
}

func validateIdentityContentCoding(headers http.Header) error {
	values := headers.Values(core.HTTPHeaderContentEncoding().String())
	if len(values) == 0 {
		return nil
	}
	if len(values) != 1 {
		return responseError(core.ErrExchangeContentType)
	}
	coding, err := parseHTTPContentCoding(values[0])
	if err != nil {
		return responseError(errors.Join(core.ErrExchangeContentType, err))
	}
	if coding != identityContentCoding() {
		return responseError(core.ErrExchangeContentType)
	}
	return nil
}

func closeResponseBody(body io.Closer) (err error) {
	defer func() {
		if recover() != nil {
			err = responseError(core.ErrExchangeContract)
		}
	}()
	if err := body.Close(); err != nil {
		return responseError(err)
	}
	return nil
}

func validateResponseContentType(
	headers http.Header,
	expected core.HTTPMediaType,
) error {
	if expected.IsZero() {
		return nil
	}
	values := headers.Values(core.HTTPHeaderContentType().String())
	if len(values) != 1 {
		return responseError(core.ErrExchangeContentType)
	}
	actual, err := core.ParseHTTPMediaType(values[0])
	if err != nil {
		return responseError(errors.Join(core.ErrExchangeContentType, err))
	}
	matches, err := actual.SameBase(expected)
	if err != nil {
		return responseError(errors.Join(core.ErrExchangeContentType, err))
	}
	if !matches {
		return responseError(core.ErrExchangeContentType)
	}
	return nil
}

// boundedBodyRead is one complete bounded aggregate body read. The declared
// extent travels with the read so the buffer can be reserved once instead of
// doubled through every intermediate size on the way to the real length.
type boundedBodyRead struct {
	context  context.Context
	source   io.Reader
	declared declaredBodyLength
	limit    core.ByteCount
}

// boundedBodyDestination deliberately exposes only io.Writer. It writes into
// the one exact declared reservation and refuses an extent mismatch rather than
// asking bytes.Buffer to grow through intermediate capacities.
type boundedBodyDestination struct {
	storage []byte
	written int
}

func (d *boundedBodyDestination) Write(payload []byte) (int, error) {
	if d == nil || len(payload) > len(d.storage)-d.written {
		return 0, core.ErrExchangeBodyLimit
	}
	written := copy(d.storage[d.written:], payload)
	d.written += written
	return written, nil
}

func readBoundedBody(read boundedBodyRead) (data []byte, err error) {
	defer func() {
		if recover() != nil {
			data = nil
			err = core.ErrExchangeContract
		}
	}()
	if err := contextstate.Validate(read.context); err != nil {
		return nil, cancelledError(err)
	}
	reserved, err := read.declared.reservedExtent(read.limit)
	if err != nil {
		return nil, err
	}
	destination := &boundedBodyDestination{storage: make([]byte, reserved)}
	_, err = copyDownload(
		downloadCopyRequest{
			context: read.context, source: read.source,
			destination: destination,
			limit:       read.limit,
		},
	)
	if err != nil {
		return nil, err
	}
	return destination.storage[:destination.written], nil
}

func captureHeaders(
	headers http.Header,
	selection HeaderSelection,
) (CapturedHeaders, error) {
	if len(selection.Names) == 0 {
		return CapturedHeaders{}, nil
	}
	values := make([]Header, 0, len(selection.Names))
	for _, name := range selection.Names {
		selected := headers.Values(name.String())
		if len(selected) == 0 {
			continue
		}
		typed, err := headerValues(selected)
		if err != nil {
			return CapturedHeaders{}, err
		}
		values = append(values, Header{
			Name: name, Values: typed,
		})
	}
	result := CapturedHeaders{Values: values}
	return result, result.Validate()
}

func headerValues(values []string) ([]HeaderValue, error) {
	result := make([]HeaderValue, len(values))
	for index, value := range values {
		parsed, err := NewHeaderValue(value)
		if err != nil {
			return nil, err
		}
		result[index] = parsed
	}
	return result, nil
}

type aggregateAttemptResult struct {
	operationContext  context.Context
	cause             error
	semantics         RequestSemantics
	response          attemptResponse
	expected          core.HTTPStatusCode
	attemptsRemaining uint64
}

type attemptDisposition uint8

const (
	attemptDispositionUnknown attemptDisposition = iota
	attemptComplete
	attemptRetry
	attemptExhausted
)

func classifyAggregateAttempt(
	result aggregateAttemptResult,
) (attemptDisposition, error) {
	if contextErr := terminalOperationError(result.operationContext); contextErr != nil {
		return attemptComplete, contextErr
	}
	if result.cause != nil {
		return classifyAggregateCause(result)
	}
	if result.response.status != result.expected {
		statusErr := StatusError{
			status: result.response.status, expected: result.expected,
		}
		if retryableStatus(result.response.status) {
			return retryDecision(result, statusErr)
		}
		return attemptComplete, statusErr
	}
	return attemptComplete, nil
}

func classifyAggregateCause(
	result aggregateAttemptResult,
) (attemptDisposition, error) {
	switch {
	case errors.Is(result.cause, core.ErrExchangeCancelled):
		return attemptComplete, result.cause
	case errors.Is(result.cause, core.ErrExchangeRedirect):
		return attemptComplete, result.cause
	case errors.Is(result.cause, core.ErrExchangeContentType):
		return attemptComplete, result.cause
	case errors.Is(result.cause, core.ErrExchangeBodyLimit):
		return attemptComplete, result.cause
	case errors.Is(result.cause, core.ErrExchangeResponse):
		return retryDecision(result, result.cause)
	default:
		return retryDecision(result, transportError(result.cause))
	}
}

func retryDecision(
	result aggregateAttemptResult,
	cause error,
) (attemptDisposition, error) {
	allowed, err := result.semantics.AllowsRetry()
	if err != nil {
		return attemptComplete, requestError(err)
	}
	if !allowed {
		return attemptComplete, cause
	}
	if result.attemptsRemaining == 0 {
		return attemptExhausted, cause
	}
	return attemptRetry, nil
}

func retryableStatus(status core.HTTPStatusCode) bool {
	value, err := status.Int()
	if err != nil {
		return false
	}
	return value == http.StatusRequestTimeout ||
		value == http.StatusTooEarly ||
		value == http.StatusTooManyRequests ||
		status.IsServerError()
}

type retryWaitRequest struct {
	context    context.Context
	retryAfter string
	policy     RetryPolicy
	progress   retryProgress
}

func waitForRetry(
	request retryWaitRequest,
) (retryProgress, error) {
	delay, err := retryDelay(request)
	if err != nil {
		return request.progress, err
	}
	waited, err := request.progress.waited.Add(delay)
	if err != nil || greaterDuration(waited, request.policy.MaximumWait) {
		return request.progress, core.ErrExchangeRetryExhausted
	}
	if err := temporal.Wait(temporal.WaitRequest{
		Context: request.context, Duration: delay,
	}); err != nil {
		return request.progress, cancelledError(err)
	}
	request.progress.waited = waited
	return request.progress, nil
}

func retryDelay(request retryWaitRequest) (temporal.Duration, error) {
	if retryAfter, ok := parseRetryAfter(
		request.retryAfter,
		request.policy.MaximumRetryAfter,
	); ok {
		return retryAfter, nil
	}
	if request.progress.attempts > 64 {
		return request.policy.MaximumDelay, nil
	}
	exponent := uint64(1) << (request.progress.attempts - 1)
	delay, err := request.policy.BaseDelay.Multiply(exponent)
	if err != nil || greaterDuration(delay, request.policy.MaximumDelay) {
		delay = request.policy.MaximumDelay
	}
	jitter, err := randomJitter(request.policy.MaximumJitter)
	if err != nil {
		return temporal.Duration{}, err
	}
	delay, err = delay.Add(jitter)
	if err != nil || greaterDuration(delay, request.policy.MaximumDelay) {
		return request.policy.MaximumDelay, nil
	}
	return delay, nil
}

// randomJitter spreads retries across the closed interval [0, maximum].
//
// The draw comes from keygen, the entropy boundary, and an entropy failure
// keeps keygen's identity: it is a local substrate refusal, not a transport
// outcome, and classifying it as transport once sent callers down a network
// diagnosis for a machine problem. The modulo keeps the full closed interval;
// jitter carries no uniformity contract, and the bias it introduces is far
// below anything a backoff spread could observe.
func randomJitter(maximum temporal.Duration) (temporal.Duration, error) {
	nanoseconds := maximum.Nanoseconds()
	if nanoseconds <= 0 {
		return temporal.Duration{}, core.ErrExchangeContract
	}
	draw, err := keygen.RandomUint64()
	if err != nil {
		return temporal.Duration{}, err
	}
	value := draw % (uint64(nanoseconds) + 1)
	converted, err := core.CheckedInt64FromUint64(value)
	if err != nil {
		return temporal.Duration{}, err
	}
	return temporal.DurationFromNanoseconds(converted)
}

func parseRetryAfter(
	value string,
	maximum temporal.Duration,
) (temporal.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return temporal.Duration{}, false
	}
	if seconds, err := strconv.ParseUint(value, 10, 63); err == nil {
		delay, durationErr := temporal.DurationFromSeconds(seconds)
		return boundedRetryAfter(delay, durationErr, maximum)
	}
	at, err := http.ParseTime(value)
	if err != nil {
		return temporal.Duration{}, false
	}
	now, err := temporal.Observe()
	if err != nil {
		return temporal.Duration{}, false
	}
	current, err := now.Instant()
	if err != nil {
		return temporal.Duration{}, false
	}
	target, err := temporal.NewInstant(at)
	if err != nil {
		return temporal.Duration{}, false
	}
	delay, err := target.Since(current)
	return boundedRetryAfter(delay, err, maximum)
}

func boundedRetryAfter(
	delay temporal.Duration,
	err error,
	maximum temporal.Duration,
) (temporal.Duration, bool) {
	if err != nil || delay.IsZero() {
		return temporal.Duration{}, false
	}
	if greaterDuration(delay, maximum) {
		return maximum, true
	}
	return delay, true
}

func terminalOperationError(ctx context.Context) error {
	return exchangeContextError(ctx)
}

func exchangeContextError(ctx context.Context) error {
	state, err := contextstate.Observe(ctx)
	if err != nil {
		return err
	}
	switch state {
	case contextstate.StateCancelled:
		return cancelledError(context.Canceled)
	case contextstate.StateDeadlineExceeded:
		return cancelledError(context.DeadlineExceeded)
	default:
		return nil
	}
}

func clientForPolicy(
	client *http.Client,
	policy RedirectPolicy,
	origin core.HTTPEndpoint,
) *http.Client {
	copy := *client
	copy.CheckRedirect = redirectChecker(
		redirectCheckRequest{policy: policy, origin: origin},
	)
	return &copy
}

type redirectCheckRequest struct {
	origin core.HTTPEndpoint
	policy RedirectPolicy
}

func redirectChecker(
	policy redirectCheckRequest,
) func(*http.Request, []*http.Request) error {
	return func(request *http.Request, via []*http.Request) error {
		if policy.policy.Mode == RedirectReject {
			return core.ErrExchangeRedirect
		}
		if request == nil || request.URL == nil || len(via) == 0 ||
			uint64(len(via)) > policy.policy.MaximumHops {
			return core.ErrExchangeRedirect
		}
		redirect, err := core.ParseHTTPEndpoint(request.URL.String())
		if err != nil {
			return core.ErrExchangeRedirect
		}
		if request.Method != via[0].Method ||
			!policy.origin.SameOrigin(redirect) {
			return core.ErrExchangeRedirect
		}
		return nil
	}
}

func closeHTTPResponse(response *http.Response) error {
	if response == nil || response.Body == nil {
		return nil
	}
	return closeResponseBody(response.Body)
}

func cancelledError(cause error) error {
	return errors.Join(core.ErrExchangeCancelled, cause)
}

var (
	_ core.Validatable = Client{}
	_ core.Validatable = NoBodyJSONCall{}
	_ core.Validatable = BoundedCall{}
	_ core.Validatable = NoBodyBoundedCall{}
)
