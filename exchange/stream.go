package exchange

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"net/http"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

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

// StreamRoundTripCall supplies one complete streamed request and response.
type StreamRoundTripCall struct {
	Context context.Context
	Client  Client
	Request StreamRoundTripRequest
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
	operationContext, cancel, err := streamContext(
		temporal.TimeoutRequest{
			Parent: call.Context, Duration: call.Policy.OperationTimeout,
		},
	)
	if err != nil {
		return zero, requestError(err)
	}
	defer cancel()
	attemptContext, attemptCancel, err := streamContext(
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
			request: call.Request,
		},
	)
}

// Download receives one response into the caller-owned destination exactly
// once, continuing until EOF or an execution failure without an extent quota.
func Download(call DownloadCall) (StreamResponse, error) {
	var zero StreamResponse
	if err := call.Validate(); err != nil {
		return zero, err
	}
	target, err := validatedTarget(call.Request.Target)
	if err != nil {
		return zero, err
	}
	operationContext, cancel, err := streamContext(
		temporal.TimeoutRequest{
			Parent: call.Context, Duration: call.Policy.OperationTimeout,
		},
	)
	if err != nil {
		return zero, requestError(err)
	}
	defer cancel()
	attemptContext, attemptCancel, err := streamContext(
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
			request: call.Request,
		},
	)
}

// RoundTripStream sends one caller-owned request stream and copies the
// successful response into one caller-owned destination. It is structurally
// single-attempt because Exchange cannot rewind either custody capability.
func RoundTripStream(call StreamRoundTripCall) (StreamRoundTripResponse, error) {
	var zero StreamRoundTripResponse
	if err := call.Validate(); err != nil {
		return zero, err
	}
	target, err := validatedTarget(call.Request.Target)
	if err != nil {
		return zero, err
	}
	operationContext, cancel, err := streamContext(temporal.TimeoutRequest{
		Parent: call.Context, Duration: call.Policy.OperationTimeout,
	})
	if err != nil {
		return zero, requestError(err)
	}
	defer cancel()
	attemptContext, attemptCancel, err := streamContext(temporal.TimeoutRequest{
		Parent: operationContext, Duration: call.Policy.AttemptTimeout,
	})
	if err != nil {
		return zero, requestError(err)
	}
	defer attemptCancel()
	request, err := newStreamRoundTripHTTPRequest(attemptContext, target, call.Request)
	if err != nil {
		return zero, err
	}
	client := clientForPolicy(call.Client.http, call.Policy.Redirect, target)
	httpResponse, err := client.Do(request)
	if err != nil {
		return zero, errors.Join(classifyStreamTransport(streamTransportFailure{
			attemptContext: attemptContext, operationContext: operationContext, cause: err,
		}), closeHTTPResponse(httpResponse))
	}
	return finishStreamRoundTrip(attemptContext, httpResponse, call.Request)
}

func newStreamRoundTripHTTPRequest(
	ctx context.Context,
	target core.HTTPEndpoint,
	request StreamRoundTripRequest,
) (*http.Request, error) {
	return newUploadHTTPRequest(ctx, uploadHTTPRequest{
		target: target,
		request: UploadRequest{
			Target: request.Target, Source: request.Source,
			Semantics: request.Semantics, ContentType: request.RequestContentType,
			Headers: request.Headers, CaptureHeaders: request.CaptureHeaders,
			ContentLength: request.RequestContentLength, ExpectedStatus: request.ExpectedStatus,
		},
	})
}

func finishStreamRoundTrip(
	ctx context.Context,
	response *http.Response,
	request StreamRoundTripRequest,
) (StreamRoundTripResponse, error) {
	var zero StreamRoundTripResponse
	if response == nil || response.Body == nil {
		return zero, responseError(core.ErrExchangeContract)
	}
	status, headers, err := streamRoundTripMetadata(response, request.CaptureHeaders)
	result := StreamRoundTripResponse{
		Metadata: ResponseMetadata{Status: status, Headers: headers, Attempts: 1},
	}
	if request.RequestContentLength != nil {
		result.DeclaredRequestBytes = *request.RequestContentLength
		result.RequestLengthKnown = true
	}
	if err != nil {
		return zero, errors.Join(err, closeHTTPResponse(response))
	}
	if status != request.ExpectedStatus {
		drainErr := drainAndClose(streamDrainRequest{context: ctx, body: response.Body})
		return result, errors.Join(StatusError{status: status, expected: request.ExpectedStatus}, drainErr)
	}
	download := downloadResponseRequest{
		context: ctx, response: response,
		request: DownloadRequest{
			Destination:                 request.Destination,
			Buffer:                      request.Buffer,
			ExpectedResponseContentType: request.ExpectedResponseContentType,
		},
	}
	if err := validateDownloadResponse(download); err != nil {
		return result, errors.Join(err, closeResponseBody(response.Body))
	}
	streamResult, err := transferDownloadResponse(download, StreamResponse{Metadata: result.Metadata})
	result.Metadata = streamResult.Metadata
	if err != nil {
		return result, err
	}
	return result, result.Validate()
}

func streamRoundTripMetadata(
	response *http.Response,
	capture HeaderSelection,
) (core.HTTPStatusCode, CapturedHeaders, error) {
	var status core.HTTPStatusCode
	if err := status.AdmitInt(response.StatusCode); err != nil {
		return core.HTTPStatusCode{}, CapturedHeaders{}, responseError(err)
	}
	headers, err := captureHeaders(response.Header, capture)
	if err != nil {
		return core.HTTPStatusCode{}, CapturedHeaders{}, responseError(err)
	}
	return status, headers, nil
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

// Validate checks the complete streamed request-response operation.
func (call StreamRoundTripCall) Validate() error {
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
	contentLength, err := streamContentLength(input.request.ContentLength)
	if err != nil {
		return nil, requestError(err)
	}
	if err := validateUploadSourceExtent(input.request, contentLength); err != nil {
		return nil, err
	}
	var source io.Reader = io.NopCloser(input.request.Source)
	if input.request.ContentLength != nil && contentLength == 0 {
		if err := probeStreamEnd(ctx, input.request.Source); err != nil {
			return nil, requestError(err)
		}
		source = http.NoBody
	}
	request, err := http.NewRequestWithContext(
		ctx,
		input.request.Semantics.Method.String(),
		input.target.String(),
		source,
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
		identityContentCoding().String(),
	)
	applyRequestHeaders(request, input.request.Headers)
	applyIdempotencyKey(request, input.request.Semantics)
	return request, nil
}

func validateUploadSourceExtent(request UploadRequest, length int64) error {
	if request.ContentLength == nil {
		return nil
	}
	remaining, proven, err := exactUploadSourceExtent(request.Source)
	if err != nil || proven && remaining != length {
		return requestError(errors.Join(core.ErrExchangeContract, err))
	}
	return nil
}

type uploadRemainingByteReader interface {
	Len() int
}

type uploadExactExtentReader interface {
	io.Reader
	RemainingBytes() uint64
}

type uploadFileReader interface {
	io.Seeker
	Stat() (fs.FileInfo, error)
}

func exactUploadSourceExtent(source io.Reader) (int64, bool, error) {
	if exact, ok := source.(uploadExactExtentReader); ok {
		return uploadUint64Extent(exact.RemainingBytes())
	}
	if remaining, ok := source.(uploadRemainingByteReader); ok {
		return uploadInt64Extent(int64(remaining.Len()))
	}
	if section, ok := source.(*io.SectionReader); ok {
		return uploadSectionExtent(section)
	}
	if file, ok := source.(uploadFileReader); ok {
		return uploadFileExtent(file)
	}
	return 0, false, nil
}

func uploadUint64Extent(remaining uint64) (int64, bool, error) {
	if remaining > uint64(^uint64(0)>>1) {
		return 0, true, errors.New("upload source extent exceeds the signed integer domain")
	}
	return int64(remaining), true, nil
}

func uploadInt64Extent(remaining int64) (int64, bool, error) {
	if remaining < 0 {
		return 0, true, errors.New("upload source reported a negative remaining extent")
	}
	return remaining, true, nil
}

func uploadSectionExtent(section *io.SectionReader) (int64, bool, error) {
	position, err := section.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, true, err
	}
	return uploadInt64Extent(section.Size() - position)
}

func uploadFileExtent(file uploadFileReader) (int64, bool, error) {
	info, err := file.Stat()
	if err != nil || info == nil || !info.Mode().IsRegular() {
		return 0, false, nil
	}
	position, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, false, nil
	}
	// Only a regular file with an observed cursor supplies extent evidence.
	// Pipes and devices remain ordinary readers owned by Go's transport.
	return uploadInt64Extent(info.Size() - position)
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
		identityContentCoding().String(),
	)
	applyRequestHeaders(request, input.request.Headers)
	applyIdempotencyKey(request, input.request.Semantics)
	return request, nil
}

type uploadResponseRequest struct {
	context  context.Context
	response *http.Response
	request  UploadRequest
}

func finishUploadResponse(
	input uploadResponseRequest,
) (StreamResponse, error) {
	var zero StreamResponse
	if input.response == nil || input.response.Body == nil {
		return zero, responseError(core.ErrExchangeContract)
	}
	var status core.HTTPStatusCode
	if err := status.AdmitInt(input.response.StatusCode); err != nil {
		return zero, errors.Join(
			responseError(err),
			closeHTTPResponse(input.response),
		)
	}
	headers, err := captureHeaders(input.response.Header, input.request.CaptureHeaders)
	if err != nil {
		return zero, errors.Join(responseError(err), closeHTTPResponse(input.response))
	}
	metadata := ResponseMetadata{
		Status:   status,
		Headers:  headers,
		Attempts: 1,
	}
	response := StreamResponse{Metadata: metadata}
	if input.request.ContentLength != nil {
		response.DeclaredRequestBytes = *input.request.ContentLength
		response.RequestLengthKnown = true
	}
	if err := response.Validate(); err != nil {
		closeErr := closeResponseBody(input.response.Body)
		return zero, errors.Join(err, closeErr)
	}
	drainErr := drainAndClose(
		streamDrainRequest{
			context: input.context, body: input.response.Body,
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
	context  context.Context
	response *http.Response
	request  DownloadRequest
}

func finishDownloadResponse(
	input downloadResponseRequest,
) (StreamResponse, error) {
	var zero StreamResponse
	if input.response == nil || input.response.Body == nil {
		return zero, responseError(core.ErrExchangeContract)
	}
	var status core.HTTPStatusCode
	if err := status.AdmitInt(input.response.StatusCode); err != nil {
		return zero, errors.Join(
			responseError(err),
			closeHTTPResponse(input.response),
		)
	}
	headers, err := captureHeaders(input.response.Header, input.request.CaptureHeaders)
	if err != nil {
		return zero, errors.Join(responseError(err), closeHTTPResponse(input.response))
	}
	metadata := ResponseMetadata{
		Status:   status,
		Headers:  headers,
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
	return validateDownloadResponseLength(input.response.ContentLength)
}

func transferDownloadResponse(
	input downloadResponseRequest,
	response StreamResponse,
) (StreamResponse, error) {
	bytes, copyErr := copyDownload(
		downloadCopyRequest{
			context: input.context, source: input.response.Body,
			destination: input.request.Destination,
			buffer:      input.request.Buffer,
		},
	)
	closeErr := closeResponseBody(input.response.Body)
	responseBytes, lengthErr := core.NewByteLength(bytes)
	response.Metadata.Bytes = responseBytes
	if err := errors.Join(copyErr, closeErr, lengthErr); err != nil {
		return response, responseError(err)
	}
	if err := response.Validate(); err != nil {
		return StreamResponse{}, err
	}
	return response, nil
}

func validateDownloadResponseLength(
	contentLength int64,
) error {
	if _, err := parseDeclaredBodyLength(contentLength); err != nil {
		return responseError(err)
	}
	return nil
}

type streamDrainRequest struct {
	context context.Context
	body    io.ReadCloser
}

func drainAndClose(request streamDrainRequest) error {
	_, drainErr := copyDownload(
		downloadCopyRequest{
			context: request.context, source: request.body,
			destination: io.Discard,
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
	// A non-nil limit belongs to an aggregate or exact-extent operation.
	// Ordinary downloads leave it nil and continue to EOF.
	limit  *core.ByteCount
	buffer []byte
}

// progressReader adds only Primitive's bounded cancellation and no-progress
// policy around a caller-controlled reader. io.Copy and io.ReadFull stay
// the sole owners of streaming and exact-read mechanics.
type progressReader struct {
	context    context.Context
	source     io.Reader
	emptyReads int
}

// observedStreamWriter contains a broken Write at that individual call so
// io.Copy can retain the count from earlier acknowledged writes. A
// panicking call has supplied no count; its unreported effects are unknown.
type observedStreamWriter struct{ destination io.Writer }

func (w observedStreamWriter) Write(p []byte) (count int, err error) {
	defer func() {
		if recover() != nil {
			count, err = 0, core.ErrExchangeContract
		}
	}()
	return w.destination.Write(p)
}

func streamCopyDestination(destination io.Writer) io.Writer {
	if _, ok := destination.(io.ReaderFrom); ok {
		return destination
	}
	return observedStreamWriter{destination: destination}
}

func (r *progressReader) Read(buffer []byte) (count int, readErr error) {
	defer func() {
		if recover() != nil {
			count, readErr = 0, core.ErrExchangeContract
		}
	}()
	if err := contextAfterTransfer(r.context); err != nil {
		return 0, err
	}
	count, readErr = r.source.Read(buffer)
	if count < 0 || count > len(buffer) {
		return 0, core.ErrExchangeContract
	}
	// A native read failure describes the bytes returned by that exact call and
	// therefore wins over cancellation observed immediately afterward. In
	// particular, a truncated HTTP body remains an extent defect rather than
	// becoming timing-dependent cancellation.
	if readErr != nil {
		return count, readErr
	}
	if err := contextAfterTransfer(r.context); err != nil {
		return count, err
	}
	if count > 0 {
		r.emptyReads = 0
		return count, nil
	}
	r.emptyReads++
	if r.emptyReads >= core.ReaderConsecutiveEmptyReadMaximum {
		return 0, io.ErrNoProgress
	}
	return 0, nil
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
	source, limit, err := streamCopySource(request)
	if err != nil {
		return 0, err
	}
	buffer := request.buffer
	if len(buffer) == 0 {
		buffer = nil
	}
	count, err := io.CopyBuffer(streamCopyDestination(request.destination), source, buffer)
	written, conversionErr := core.CheckedUint64FromInt64(count)
	if conversionErr != nil {
		return 0, errors.Join(core.ErrExchangeContract, conversionErr)
	}
	if err != nil {
		return written, err
	}
	if request.limit == nil || count < limit {
		return written, contextAfterTransfer(request.context)
	}
	return written, probeStreamEnd(request.context, request.source)
}

// streamCopySource keeps Go's LimitedReader visible for operations with an
// actual bound, while ordinary streams use only the progress observation.
func streamCopySource(request downloadCopyRequest) (io.Reader, int64, error) {
	source := &progressReader{context: request.context, source: request.source}
	if request.limit == nil {
		return source, 0, nil
	}
	limit, err := request.limit.Int64()
	if err != nil {
		return nil, 0, err
	}
	return &io.LimitedReader{R: source, N: limit}, limit, nil
}

func probeStreamEnd(ctx context.Context, source io.Reader) error {
	var probe [1]byte
	count, err := io.ReadFull(&progressReader{context: ctx, source: source}, probe[:])
	if count > 0 {
		return core.ErrExchangeBodyLimit
	}
	// witness:waiver doctrine/error/sentinel_compare -- Go recognizes only the unwrapped EOF sentinel as clean termination; joined EOF and native failures must remain failures.
	if err == io.EOF {
		return contextAfterTransfer(ctx)
	}
	if err != nil {
		return err
	}
	return core.ErrExchangeContract
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
		return errors.Join(terminal, failure.cause)
	}
	if terminal := terminalOperationError(
		failure.attemptContext,
	); terminal != nil {
		return errors.Join(terminal, failure.cause)
	}
	if errors.Is(failure.cause, core.ErrExchangeRedirect) {
		return errors.Join(core.ErrExchangeRedirect, failure.cause)
	}
	return transportError(failure.cause)
}

var (
	_ core.Validatable = UploadCall{}
	_ core.Validatable = DownloadCall{}
	_ core.Validatable = StreamRoundTripCall{}
)

// streamContext keeps every stream lifetime owned even when no timeout is
// requested. Go supplies cancellation; Temporal supplies an explicit clock bound.
func streamContext(request temporal.TimeoutRequest) (context.Context, context.CancelFunc, error) {
	if err := request.Validate(); err != nil {
		return nil, nil, err
	}
	if request.Duration.IsZero() {
		ctx, cancel := context.WithCancel(request.Parent)
		return ctx, cancel, nil
	}
	return temporal.WithTimeout(request)
}
