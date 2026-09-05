package exchange

import (
	"context"
	"errors"
	"io"
	"maps"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

// ResponseBufferMaximumBytes is the mechanical allocation ceiling. Callers
// choose a smaller budget appropriate to their response contract.
const ResponseBufferMaximumBytes = 16 << 20

// ResponseBufferRequest holds one response until Serve returns nil. Serve owns
// the decision to release; Exchange owns byte bounds, framing and write results.
// It is synchronous and Serve must honor the supplied context's lifetime.
type ResponseBufferRequest struct {
	Call        SocketServerCall
	Serve       func(SocketServerCall) error
	BodyMaximum core.ByteCount
}

func (r ResponseBufferRequest) Validate() error {
	if r.Serve == nil {
		return core.ErrExchangeContract
	}
	if err := r.Call.Validate(); err != nil {
		return err
	}
	if err := r.BodyMaximum.Validate(); err != nil {
		return errors.Join(core.ErrExchangeContract, err)
	}
	maximum, err := r.BodyMaximum.Uint64()
	if err != nil {
		return err
	}
	if maximum > ResponseBufferMaximumBytes {
		return core.ErrExchangeBodyLimit
	}
	return nil
}

// ResponseBufferResult reports the write that actually crossed the destination.
// Committed means headers were released, including when the body write failed.
// The zero value means nothing was released; it never claims delivery.
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
	maximum int
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
	if err := ctx.Err(); err != nil {
		return ResponseBufferResult{}, err
	}
	maximum, err := request.BodyMaximum.Uint64()
	if err != nil {
		return ResponseBufferResult{}, err
	}
	buffer := &responseBuffer{header: make(http.Header), maximum: int(maximum)}
	if err := errors.Join(request.Serve(SocketServerCall{writer: buffer, request: request.Call.request.WithContext(ctx)}), buffer.failure, ctx.Err()); err != nil {
		return ResponseBufferResult{}, err
	}
	if buffer.status == 0 {
		buffer.WriteHeader(http.StatusOK)
	}
	if err := errors.Join(buffer.failure, buffer.validateExtent(request.Call.request.Method)); err != nil {
		return ResponseBufferResult{}, err
	}
	if request.Call.request.Method == http.MethodHead {
		buffer.body = nil
	}
	return buffer.release(ctx, request.Call.writer)
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
	if len(data) > b.maximum-len(b.body) {
		b.failure = core.ErrExchangeBodyLimit
		return 0, b.failure
	}
	b.reserve(len(data))
	b.body = append(b.body, data...)
	return len(data), nil
}

func (b *responseBuffer) reserve(additional int) {
	needed := len(b.body) + additional
	if needed <= cap(b.body) {
		return
	}
	capacity := min(b.maximum, max(needed, 2*cap(b.body), TransferBufferBytes))
	body := make([]byte, len(b.body), capacity)
	copy(body, b.body)
	b.body = body
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
	if strings.HasPrefix(name, http.TrailerPrefix) || strings.EqualFold(name, "Trailer") {
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
	values := b.sealed.Values("Content-Length")
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

func (b *responseBuffer) release(ctx context.Context, destination http.ResponseWriter) (ResponseBufferResult, error) {
	if err := ctx.Err(); err != nil {
		return ResponseBufferResult{}, err
	}
	var status core.HTTPStatusCode
	err := status.AdmitInt(b.status)
	if err != nil {
		return ResponseBufferResult{}, responseError(err)
	}
	maps.Copy(destination.Header(), b.sealed)
	destination.WriteHeader(b.status)
	result := ResponseBufferResult{Status: status, Committed: true}
	if len(b.body) == 0 {
		return result, result.Validate()
	}
	count, writeErr := destination.Write(b.body)
	if count < 0 || count > len(b.body) {
		return result, errors.Join(core.ErrExchangeResponse, io.ErrShortWrite, writeErr)
	}
	result.Bytes, err = core.NewByteLength(uint64(count))
	if count != len(b.body) {
		writeErr = errors.Join(writeErr, io.ErrShortWrite)
	}
	return result, errors.Join(err, writeErr, result.Validate())
}

var _ http.ResponseWriter = (*responseBuffer)(nil)

func responseWriterIsNil(writer http.ResponseWriter) bool {
	if writer == nil {
		return true
	}
	value := reflect.ValueOf(writer)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
