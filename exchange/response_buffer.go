package exchange

import (
	"context"
	"errors"
	"io"
	"maps"
	"net/http"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// ResponseBufferRequest holds one response until Serve returns nil. Serve owns
// the decision to release; Exchange owns framing and write results. This API
// retains the complete response in memory. Use WriteStream for incremental output.
// It is synchronous and Serve must honor the supplied context's lifetime.
type ResponseBufferRequest struct {
	Call  SocketServerCall
	Serve func(SocketServerCall) error
}

func (r ResponseBufferRequest) Validate() error {
	if r.Serve == nil {
		return core.ErrExchangeContract
	}
	if err := r.Call.Validate(); err != nil {
		return err
	}
	return nil
}

// ResponseBufferResult reports the write that actually crossed the destination.
// Committed means the destination acknowledged WriteHeader by returning, even
// when a later body write failed. Bytes counts acknowledged Write results.
// A panic cannot acknowledge its in-flight effect: a zero receipt is absent
// evidence, not proof that a misbehaving destination performed no effect.
type ResponseBufferResult struct {
	Status    core.HTTPStatusCode
	Bytes     core.ByteLength
	Committed bool
}

func (r ResponseBufferResult) Validate() error {
	if !r.Committed {
		if r.Status != (core.HTTPStatusCode{}) || r.Bytes != (core.ByteLength{}) {
			return core.ErrExchangeContract
		}
		return nil
	}
	return errors.Join(r.Status.Validate(), r.Bytes.Validate())
}

type responseBuffer struct {
	header  http.Header
	sealed  http.Header
	body    []byte
	failure error
	status  int
}

// BufferResponse releases the buffered response once, only after both the
// product callback and every mechanical write have succeeded. An ignored Write
// error remains sticky. The result preserves uncertainty after partial release.
func BufferResponse(ctx context.Context, request ResponseBufferRequest) (ResponseBufferResult, error) {
	if ctx == nil {
		return ResponseBufferResult{}, core.ErrExchangeContract
	}
	if err := request.Validate(); err != nil {
		return ResponseBufferResult{}, err
	}
	if err := contextstate.Validate(ctx); err != nil {
		return ResponseBufferResult{}, err
	}
	buffer := &responseBuffer{header: make(http.Header)}
	if err := buffer.serve(ctx, request); err != nil {
		return ResponseBufferResult{}, err
	}
	if buffer.status == 0 {
		buffer.WriteHeader(http.StatusOK)
	}
	if err := buffer.failure; err != nil {
		return ResponseBufferResult{}, err
	}
	return buffer.release(ctx, request.Call.writer, request.Call.request.Method)
}

func (b *responseBuffer) serve(ctx context.Context, request ResponseBufferRequest) error {
	return executeResponseWriterOperation(func() error {
		call := SocketServerCall{writer: b, request: request.Call.request.WithContext(ctx)}
		return errors.Join(request.Serve(call), b.failure, contextstate.Validate(ctx))
	})
}

func (b *responseBuffer) Header() http.Header { return b.header }

func (b *responseBuffer) WriteHeader(status int) {
	if b.failure != nil || b.status != 0 {
		return
	}
	if status < http.StatusOK || status > 599 {
		b.failure = core.ErrExchangeResponse
		return
	}
	if err := validateBufferedHeaders(b.header); err != nil {
		b.failure = err
		return
	}
	b.status = status
	b.sealed = b.header.Clone()
}

func (b *responseBuffer) Write(data []byte) (int, error) {
	if b.failure != nil {
		return 0, b.failure
	}
	if b.status == 0 {
		b.WriteHeader(http.StatusOK)
	}
	if b.failure != nil {
		return 0, b.failure
	}
	if b.status == http.StatusNoContent || b.status == http.StatusNotModified {
		b.failure = errors.Join(core.ErrExchangeResponse, http.ErrBodyNotAllowed)
		return 0, b.failure
	}
	b.body = append(b.body, data...)
	return len(data), nil
}

func validateBufferedHeaders(headers http.Header) error {
	if len(headers) > HeaderMaximumCount {
		return core.ErrExchangeBodyLimit
	}
	for name, values := range headers {
		if err := validateBufferedHeader(name, values); err != nil {
			return err
		}
	}
	return nil
}

func validateBufferedHeader(name string, values []string) error {
	if strings.HasPrefix(name, http.TrailerPrefix) || strings.EqualFold(name, core.HTTPHeaderTrailer().String()) {
		return core.ErrExchangeResponse
	}
	canonical, err := core.ParseHTTPHeaderName(name)
	if err != nil || canonical.String() != name {
		return responseError(err)
	}
	if len(values) > HeaderValueMaximumCount {
		return core.ErrExchangeBodyLimit
	}
	for _, value := range values {
		if _, err := NewHeaderValue(value); err != nil {
			return responseError(err)
		}
	}
	return nil
}

func (b *responseBuffer) validateExtent(method string) error {
	values := b.sealed.Values(core.HTTPHeaderContentLength().String())
	if len(values) == 0 {
		return nil
	}
	if len(values) != 1 {
		return core.ErrExchangeResponse
	}
	length, err := strconv.ParseUint(values[0], 10, 64)
	if err != nil {
		return responseError(err)
	}
	// HEAD and 304 may advertise the selected representation's size without
	// generating it. A generated HEAD representation still proves its extent.
	if len(b.body) == 0 && (method == http.MethodHead || b.status == http.StatusNotModified) {
		return nil
	}
	if length != uint64(len(b.body)) {
		return core.ErrExchangeResponse
	}
	return nil
}

// witness:waiver doctrine/error/named_returns -- Deferred panic containment must set the returned error while preserving the status and byte acknowledgments already recorded before the Go writer panics.
func (b *responseBuffer) release(ctx context.Context, destination http.ResponseWriter, method string) (result ResponseBufferResult, err error) {
	defer containResponseWriterPanic(&err)
	if err := contextstate.Validate(ctx); err != nil {
		return ResponseBufferResult{}, err
	}
	var status core.HTTPStatusCode
	err = status.AdmitInt(b.status)
	if err != nil {
		return ResponseBufferResult{}, responseError(err)
	}
	header := destination.Header()
	if err := b.prepareRelease(header, method); err != nil {
		return ResponseBufferResult{}, err
	}
	maps.Copy(header, b.sealed)
	destination.WriteHeader(b.status)
	result = ResponseBufferResult{Status: status, Committed: true}
	if len(b.body) == 0 {
		return result, result.Validate()
	}
	count, writeErr := destination.Write(b.body)
	if count < 0 || count > len(b.body) {
		return result, errors.Join(core.ErrExchangeResponse, core.ErrExchangeWrite, io.ErrShortWrite, writeErr)
	}
	result.Bytes, err = core.NewByteLength(uint64(count))
	if count != len(b.body) {
		writeErr = errors.Join(writeErr, io.ErrShortWrite)
	}
	if writeErr != nil {
		writeErr = errors.Join(core.ErrExchangeResponse, core.ErrExchangeWrite, writeErr)
	}
	return result, errors.Join(err, writeErr, result.Validate())
}

// Outer middleware fields and buffered fields form the actual HTTP response.
// Validate that complete framing before changing the destination or suppressing
// a generated HEAD body whose extent still needs to be proved.
func (b *responseBuffer) prepareRelease(destination http.Header, method string) error {
	merged := destination.Clone()
	if merged == nil {
		merged = make(http.Header)
	}
	maps.Copy(merged, b.sealed)
	if err := validateBufferedHeaders(merged); err != nil {
		return err
	}
	b.sealed = merged
	if err := b.validateExtent(method); err != nil {
		return err
	}
	if method == http.MethodHead {
		b.body = nil
	}
	return nil
}

var _ http.ResponseWriter = (*responseBuffer)(nil)
