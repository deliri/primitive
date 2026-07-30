package exchange

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// transferBuffer is one fixed streaming extent. Reuse across transfers rests on
// two contracts that the type system cannot enforce, so both are stated here and
// ratcheted by the package's internal tests.
//
// First, io.Writer forbids an implementation from retaining the slice it is
// given. A destination that retains it would observe a later transfer's bytes
// through a buffer this one no longer owns.
//
// Second, releaseTransferBuffer is the only path back into the pool, because the
// scrub lives there. A direct Put would leave one transfer's bytes resident for
// the next acquirer.
//
// io.CopyBuffer documents that it ignores the supplied buffer when the source
// implements io.WriterTo or the destination implements io.ReaderFrom, so a
// destination such as io.Discard or bytes.Buffer never reads this extent.
type transferBuffer [TransferBufferBytes]byte

var transferBuffers = sync.Pool{
	New: func() any {
		return new(transferBuffer)
	},
}

func acquireTransferBuffer() *transferBuffer {
	return transferBuffers.Get().(*transferBuffer)
}

// releaseTransferBuffer scrubs the complete extent before returning it, so no
// acquirer can read the bytes of the transfer that released it.
func releaseTransferBuffer(buffer *transferBuffer) {
	scrubTransferBuffer(buffer)
	transferBuffers.Put(buffer)
}

func scrubTransferBuffer(buffer *transferBuffer) {
	clear(buffer[:])
}

// UploadCall supplies one complete streaming upload.
type UploadCall struct {
	Context context.Context
	Client  Client
	Request UploadRequest
	Policy  StreamPolicy
}

// DownloadCall supplies one complete streaming download.
type DownloadCall struct {
	Context context.Context
	Client  Client
	Request DownloadRequest
	Policy  StreamPolicy
}

// Upload sends one caller-owned stream exactly once. Exchange does not retain,
// rewind, or replay the source.
func Upload(call UploadCall) (StreamResponse, error) {
	var zero StreamResponse
	if err := call.Validate(); err != nil {
		return zero, err
	}
	target, err := validatedTarget(call.Request.Target)
	if err != nil {
		return zero, err
	}
	operationContext, cancel, err := temporal.WithTimeout(
		temporal.TimeoutRequest{
			Parent: call.Context, Duration: call.Policy.OperationTimeout,
		},
	)
	if err != nil {
		return zero, requestError(err)
	}
	defer cancel()
	attemptContext, attemptCancel, err := temporal.WithTimeout(
		temporal.TimeoutRequest{
			Parent: operationContext, Duration: call.Policy.AttemptTimeout,
		},
	)
	if err != nil {
		return zero, requestError(err)
	}
	defer attemptCancel()
	request, err := newUploadHTTPRequest(
		attemptContext,
		uploadHTTPRequest{
			target: target, request: call.Request,
		},
	)
	if err != nil {
		return zero, err
	}
	client := clientForPolicy(
		call.Client.http,
		call.Policy.Redirect,
		target,
	)
	httpResponse, err := client.Do(request)
	if err != nil {
		return zero, errors.Join(
			classifyStreamTransport(
				streamTransportFailure{
					attemptContext:   attemptContext,
					operationContext: operationContext,
					cause:            err,
				},
			),
			closeHTTPResponse(httpResponse),
		)
	}
	return finishUploadResponse(
		uploadResponseRequest{
			context: attemptContext, response: httpResponse,
			request: call.Request, limit: call.Policy.ErrorBodyLimit,
		},
	)
}

// Download receives one response into the caller-owned destination exactly
// once. It writes at most ResponseBodyLimit bytes and probes one additional
// byte before declaring the response complete.
func Download(call DownloadCall) (StreamResponse, error) {
	var zero StreamResponse
	if err := call.Validate(); err != nil {
		return zero, err
	}
	target, err := validatedTarget(call.Request.Target)
	if err != nil {
		return zero, err
	}
	operationContext, cancel, err := temporal.WithTimeout(
		temporal.TimeoutRequest{
			Parent: call.Context, Duration: call.Policy.OperationTimeout,
		},
	)
	if err != nil {
		return zero, requestError(err)
	}
	defer cancel()
	attemptContext, attemptCancel, err := temporal.WithTimeout(
		temporal.TimeoutRequest{
			Parent: operationContext, Duration: call.Policy.AttemptTimeout,
		},
	)
	if err != nil {
		return zero, requestError(err)
	}
	defer attemptCancel()
	request, err := newDownloadHTTPRequest(
		attemptContext,
		downloadHTTPRequest{
			target: target, request: call.Request,
		},
	)
	if err != nil {
		return zero, err
	}
	client := clientForPolicy(
		call.Client.http,
		call.Policy.Redirect,
		target,
	)
	httpResponse, err := client.Do(request)
	if err != nil {
		return zero, errors.Join(
			classifyStreamTransport(
				streamTransportFailure{
					attemptContext:   attemptContext,
					operationContext: operationContext,
					cause:            err,
				},
			),
			closeHTTPResponse(httpResponse),
		)
	}
	return finishDownloadResponse(
		downloadResponseRequest{
			context: attemptContext, response: httpResponse,
			request: call.Request, errorLimit: call.Policy.ErrorBodyLimit,
		},
	)
}

// Validate checks the complete streaming upload operation.
func (call UploadCall) Validate() error {
	if err := validateCallIngress(call.Context, call.Client); err != nil {
		return err
	}
	if err := call.Request.Validate(); err != nil {
		return err
	}
	return call.Policy.Validate()
}

// Validate checks the complete streaming download operation.
func (call DownloadCall) Validate() error {
	if err := validateCallIngress(call.Context, call.Client); err != nil {
		return err
	}
	if err := call.Request.Validate(); err != nil {
		return err
	}
	return call.Policy.Validate()
}

type uploadHTTPRequest struct {
	target  core.HTTPEndpoint
	request UploadRequest
}

func newUploadHTTPRequest(
	ctx context.Context,
	input uploadHTTPRequest,
) (*http.Request, error) {
	contentLength, err := input.request.ContentLength.Int64()
	if err != nil {
		return nil, requestError(err)
	}
	limited := &io.LimitedReader{
		R: input.request.Source,
		N: contentLength,
	}
	request, err := http.NewRequestWithContext(
		ctx,
		input.request.Semantics.Method.String(),
		input.target.String(),
		limited,
	)
	if err != nil {
		return nil, requestError(err)
	}
	request.ContentLength = contentLength
	request.Header.Set(
		core.HTTPHeaderContentType().String(),
		input.request.ContentType.String(),
	)
	request.Header.Set(
		core.HTTPHeaderAcceptEncoding().String(),
		core.HTTPContentCodingIdentity().String(),
	)
	applyRequestHeaders(request, input.request.Headers)
	applyIdempotencyKey(request, input.request.Semantics)
	return request, nil
}

type downloadHTTPRequest struct {
	target  core.HTTPEndpoint
	request DownloadRequest
}

func newDownloadHTTPRequest(
	ctx context.Context,
	input downloadHTTPRequest,
) (*http.Request, error) {
	request, err := http.NewRequestWithContext(
		ctx,
		input.request.Semantics.Method.String(),
		input.target.String(),
		nil,
	)
	if err != nil {
		return nil, requestError(err)
	}
	if !input.request.ExpectedResponseContentType.IsZero() {
		request.Header.Set(
			core.HTTPHeaderAccept().String(),
			input.request.ExpectedResponseContentType.String(),
		)
	}
	request.Header.Set(
		core.HTTPHeaderAcceptEncoding().String(),
		core.HTTPContentCodingIdentity().String(),
	)
	applyRequestHeaders(request, input.request.Headers)
	applyIdempotencyKey(request, input.request.Semantics)
	return request, nil
}

type uploadResponseRequest struct {
	context  context.Context
	response *http.Response
	request  UploadRequest
	limit    core.ByteCount
}

func finishUploadResponse(
	input uploadResponseRequest,
) (StreamResponse, error) {
	var zero StreamResponse
	if input.response == nil || input.response.Body == nil {
		return zero, responseError(core.ErrExchangeContract)
	}
	status, err := core.NewHTTPStatusCode(input.response.StatusCode)
	if err != nil {
		return zero, errors.Join(
			responseError(err),
			closeHTTPResponse(input.response),
		)
	}
	metadata := ResponseMetadata{
		Status: status,
		Headers: captureHeaders(
			input.response.Header,
			input.request.CaptureHeaders,
		),
		Bytes:    input.request.ContentLength,
		Attempts: 1,
	}
	response := StreamResponse{Metadata: metadata}
	if err := response.Validate(); err != nil {
		closeErr := closeResponseBody(input.response.Body)
		return zero, errors.Join(err, closeErr)
	}
	drainErr := drainAndClose(
		streamDrainRequest{
			context: input.context, body: input.response.Body,
			limit: input.limit,
		},
	)
	if status != input.request.ExpectedStatus {
		return response, errors.Join(
			StatusError{
				status: status, expected: input.request.ExpectedStatus,
			},
			drainErr,
		)
	}
	return response, drainErr
}

type downloadResponseRequest struct {
	context    context.Context
	response   *http.Response
	request    DownloadRequest
	errorLimit core.ByteCount
}

func finishDownloadResponse(
	input downloadResponseRequest,
) (StreamResponse, error) {
	var zero StreamResponse
	if input.response == nil || input.response.Body == nil {
		return zero, responseError(core.ErrExchangeContract)
	}
	status, err := core.NewHTTPStatusCode(input.response.StatusCode)
	if err != nil {
		return zero, errors.Join(
			responseError(err),
			closeHTTPResponse(input.response),
		)
	}
	metadata := ResponseMetadata{
		Status: status,
		Headers: captureHeaders(
			input.response.Header,
			input.request.CaptureHeaders,
		),
		Attempts: 1,
	}
	response := StreamResponse{Metadata: metadata}
	if err := response.Validate(); err != nil {
		closeErr := closeResponseBody(input.response.Body)
		return zero, errors.Join(err, closeErr)
	}
	if status != input.request.ExpectedStatus {
		drainErr := drainAndClose(
			streamDrainRequest{
				context: input.context, body: input.response.Body,
				limit: input.errorLimit,
			},
		)
		statusErr := StatusError{
			status: status, expected: input.request.ExpectedStatus,
		}
		return response, errors.Join(statusErr, drainErr)
	}
	if err := validateDownloadResponse(input); err != nil {
		closeErr := closeResponseBody(input.response.Body)
		return response, errors.Join(err, closeErr)
	}
	return transferDownloadResponse(input, response)
}

func validateDownloadResponse(input downloadResponseRequest) error {
	if err := validateIdentityContentCoding(input.response.Header); err != nil {
		return err
	}
	if err := validateResponseContentType(
		input.response.Header,
		input.request.ExpectedResponseContentType,
	); err != nil {
		return err
	}
	return validateDownloadResponseLength(
		input.response.ContentLength,
		input.request.ResponseBodyLimit,
	)
}

func transferDownloadResponse(
	input downloadResponseRequest,
	response StreamResponse,
) (StreamResponse, error) {
	bytes, copyErr := copyDownload(
		downloadCopyRequest{
			context: input.context, source: input.response.Body,
			destination: input.request.Destination,
			limit:       input.request.ResponseBodyLimit,
		},
	)
	closeErr := closeResponseBody(input.response.Body)
	response.Metadata.Bytes = core.NewByteLength(bytes)
	if err := errors.Join(copyErr, closeErr); err != nil {
		return response, responseError(err)
	}
	if err := response.Validate(); err != nil {
		return StreamResponse{}, err
	}
	return response, nil
}

func validateDownloadResponseLength(
	contentLength int64,
	limit core.ByteCount,
) error {
	if _, err := admittedBodyLength(contentLength, limit); err != nil {
		return responseError(err)
	}
	return nil
}

type streamDrainRequest struct {
	context context.Context
	body    io.ReadCloser
	limit   core.ByteCount
}

func drainAndClose(request streamDrainRequest) error {
	_, drainErr := copyDownload(
		downloadCopyRequest{
			context: request.context, source: request.body,
			destination: io.Discard, limit: request.limit,
		},
	)
	closeErr := closeResponseBody(request.body)
	if err := errors.Join(drainErr, closeErr); err != nil {
		return responseError(err)
	}
	return nil
}

type downloadCopyRequest struct {
	context     context.Context
	source      io.Reader
	destination io.Writer
	limit       core.ByteCount
}

func copyDownload(
	request downloadCopyRequest,
) (written uint64, err error) {
	defer func() {
		if recover() != nil {
			err = core.ErrExchangeContract
		}
	}()
	if err := contextstate.Validate(request.context); err != nil {
		return 0, cancelledError(err)
	}
	limit, err := request.limit.Int64()
	if err != nil {
		return 0, err
	}
	limited := &io.LimitedReader{
		R: request.source,
		N: limit,
	}
	buffer := acquireTransferBuffer()
	defer releaseTransferBuffer(buffer)
	count, err := io.CopyBuffer(
		request.destination,
		limited,
		buffer[:],
	)
	written, conversionErr := core.CheckedUint64FromInt64(count)
	if conversionErr != nil {
		return 0, errors.Join(core.ErrExchangeContract, conversionErr)
	}
	if err != nil {
		return written, err
	}
	if count < limit {
		return written, contextAfterTransfer(request.context)
	}
	return probeDownloadEnd(request, written)
}

func probeDownloadEnd(
	request downloadCopyRequest,
	written uint64,
) (uint64, error) {
	var probe [1]byte
	read, readErr := request.source.Read(probe[:])
	if read < 0 || read > len(probe) {
		return written, core.ErrExchangeContract
	}
	if read > 0 {
		return written, core.ErrExchangeBodyLimit
	}
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return written, readErr
	}
	return written, contextAfterTransfer(request.context)
}

func contextAfterTransfer(ctx context.Context) error {
	if err := contextstate.Validate(ctx); err != nil {
		return cancelledError(err)
	}
	return nil
}

type streamTransportFailure struct {
	attemptContext   context.Context
	operationContext context.Context
	cause            error
}

func classifyStreamTransport(failure streamTransportFailure) error {
	if terminal := terminalOperationError(
		failure.operationContext,
	); terminal != nil {
		return terminal
	}
	if terminal := terminalOperationError(
		failure.attemptContext,
	); terminal != nil {
		return terminal
	}
	if errors.Is(failure.cause, core.ErrExchangeRedirect) {
		return errors.Join(core.ErrExchangeRedirect, failure.cause)
	}
	return transportError(failure.cause)
}

var (
	_ core.Validatable = UploadCall{}
	_ core.Validatable = DownloadCall{}
)
