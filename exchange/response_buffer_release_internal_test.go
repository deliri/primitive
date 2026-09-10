package exchange

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type bufferDestinationFault uint8

const (
	bufferDestinationIntact bufferDestinationFault = iota
	bufferDestinationClosed
	bufferDestinationPartial
	bufferDestinationShort
	bufferDestinationInvalidCount
)

// The standard ResponseWriter seam makes exact write counts and native errors
// observable. TestResponseBufferPreservesRealDestinationFailure separately
// drives a real io.Pipe failure.
type bufferReleaseDestination struct {
	admissionResponseWriter
	fault bufferDestinationFault
}

func (w *bufferReleaseDestination) Write(data []byte) (int, error) {
	w.writes++
	switch w.fault {
	case bufferDestinationClosed:
		return 0, io.ErrClosedPipe
	case bufferDestinationPartial:
		n, _ := w.body.Write(data[:2])
		return n, io.ErrUnexpectedEOF
	case bufferDestinationShort:
		return w.body.Write(data[:2])
	case bufferDestinationInvalidCount:
		return len(data) + 1, io.ErrClosedPipe
	default:
		return w.body.Write(data)
	}
}

func TestResponseBufferReleaseLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name            string
		method          string
		body            string
		beforeLength    []string
		bufferLength    []string
		fault           bufferDestinationFault
		servePanic      bool
		serveErr        error
		wantErr         error
		wantNative      error
		wantShort       bool
		wantCommitted   bool
		wantBytes       uint64
		wantBody        string
		wantLength      []string
		wantStatusCalls int
		wantWrites      int
	}{
		{name: "positive exact release retains receipt and bytes", method: http.MethodGet, body: "abcd", wantCommitted: true, wantBytes: 4, wantBody: "abcd", wantStatusCalls: 1, wantWrites: 1},
		{name: "positive matching outer length remains valid", method: http.MethodGet, body: "abcd", beforeLength: []string{"4"}, wantCommitted: true, wantBytes: 4, wantBody: "abcd", wantLength: []string{"4"}, wantStatusCalls: 1, wantWrites: 1},
		{name: "positive explicit buffered framing replaces stale outer length", method: http.MethodGet, body: "abcd", beforeLength: []string{"5"}, bufferLength: []string{"4"}, wantCommitted: true, wantBytes: 4, wantBody: "abcd", wantLength: []string{"4"}, wantStatusCalls: 1, wantWrites: 1},
		{name: "negative outer extent larger than body cannot commit", method: http.MethodGet, body: "abcd", beforeLength: []string{"5"}, wantErr: core.ErrExchangeResponse, wantLength: []string{"5"}},
		{name: "negative outer extent smaller than body cannot commit", method: http.MethodGet, body: "abcd", beforeLength: []string{"3"}, wantErr: core.ErrExchangeResponse, wantLength: []string{"3"}},
		{name: "negative duplicated outer framing cannot bypass buffer validation", method: http.MethodGet, body: "abcd", beforeLength: []string{"4", "4"}, wantErr: core.ErrExchangeResponse, wantLength: []string{"4", "4"}},
		{name: "negative malformed outer framing cannot commit", method: http.MethodGet, body: "abcd", beforeLength: []string{"-1"}, wantErr: core.ErrExchangeResponse, wantLength: []string{"-1"}},
		{name: "negative generated HEAD must prove outer representation extent", method: http.MethodHead, body: "abcd", beforeLength: []string{"5"}, wantErr: core.ErrExchangeResponse, wantLength: []string{"5"}},
		{name: "neutral ungenerated HEAD may retain representation size", method: http.MethodHead, beforeLength: []string{"5"}, wantCommitted: true, wantLength: []string{"5"}, wantStatusCalls: 1},
		{name: "neutral no body release must not call destination Write", method: http.MethodGet, wantCommitted: true, wantStatusCalls: 1},
		{name: "negative closed destination preserves committed status and zero bytes", method: http.MethodGet, body: "abcd", fault: bufferDestinationClosed, wantErr: core.ErrExchangeWrite, wantNative: io.ErrClosedPipe, wantShort: true, wantCommitted: true, wantStatusCalls: 1, wantWrites: 1},
		{name: "negative native partial write retains exact prefix and count", method: http.MethodGet, body: "abcd", fault: bufferDestinationPartial, wantErr: core.ErrExchangeWrite, wantNative: io.ErrUnexpectedEOF, wantShort: true, wantCommitted: true, wantBytes: 2, wantBody: "ab", wantStatusCalls: 1, wantWrites: 1},
		{name: "negative short nil write cannot report complete release", method: http.MethodGet, body: "abcd", fault: bufferDestinationShort, wantErr: core.ErrExchangeWrite, wantShort: true, wantCommitted: true, wantBytes: 2, wantBody: "ab", wantStatusCalls: 1, wantWrites: 1},
		{name: "negative impossible write count cannot become a byte receipt", method: http.MethodGet, body: "abcd", fault: bufferDestinationInvalidCount, wantErr: core.ErrExchangeWrite, wantNative: io.ErrClosedPipe, wantShort: true, wantCommitted: true, wantStatusCalls: 1, wantWrites: 1},
		{name: "negative callback refusal withholds written private bytes", method: http.MethodGet, body: "abcd", serveErr: io.ErrClosedPipe, wantErr: io.ErrClosedPipe},
		{name: "negative callback panic withholds written private bytes", method: http.MethodGet, body: "abcd", servePanic: true, wantErr: core.ErrExchangeResponse},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			destination := &bufferReleaseDestination{header: make(http.Header), fault: tc.fault}
			lengthName := core.HTTPHeaderContentLength().String()
			if tc.beforeLength != nil {
				destination.header[lengthName] = slices.Clone(tc.beforeLength)
			}
			var result ResponseBufferResult
			var gotErr error
			var gotPanic any
			var callbackCalls int
			var producerBytes int
			var producerErr error
			func() {
				defer func() { gotPanic = recover() }()
				result, gotErr = BufferResponse(t.Context(), ResponseBufferRequest{
					Call: SocketServerCall{writer: destination, request: httptest.NewRequest(tc.method, "/", nil)},
					Serve: func(call SocketServerCall) error {
						callbackCalls++
						if tc.bufferLength != nil {
							call.writer.Header()[lengthName] = slices.Clone(tc.bufferLength)
						}
						if tc.body != "" {
							producerBytes, producerErr = call.writer.Write([]byte(tc.body))
						}
						if tc.servePanic {
							panic(io.ErrClosedPipe)
						}
						return errors.Join(producerErr, tc.serveErr)
					},
				})
			}()
			if gotPanic != nil || !errors.Is(gotErr, tc.wantErr) || tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) || errors.Is(gotErr, io.ErrShortWrite) != tc.wantShort {
				t.Fatalf("release panic/error/short = (%v, %v, %t), want (nil, %v retaining %v, %t)", gotPanic, gotErr, errors.Is(gotErr, io.ErrShortWrite), tc.wantErr, tc.wantNative, tc.wantShort)
			}
			if callbackCalls != 1 || producerErr != nil || producerBytes != len(tc.body) {
				t.Fatalf("callback calls/produced bytes/error = (%d, %d, %v), want (1, %d, nil)", callbackCalls, producerBytes, producerErr, len(tc.body))
			}
			if result.Committed != tc.wantCommitted || result.Bytes.Uint64() != tc.wantBytes || destination.body.String() != tc.wantBody || destination.statusCalls != tc.wantStatusCalls || destination.writes != tc.wantWrites || !slices.Equal(destination.header[lengthName], tc.wantLength) {
				t.Fatalf("result/body/status calls/writes/length = (%+v, %q, %d, %d, %q), want (committed %t, %d bytes, %q, %d, %d, %q)", result, destination.body.String(), destination.statusCalls, destination.writes, destination.header[lengthName], tc.wantCommitted, tc.wantBytes, tc.wantBody, tc.wantStatusCalls, tc.wantWrites, tc.wantLength)
			}
			if !tc.wantCommitted && result != (ResponseBufferResult{}) {
				t.Fatalf("uncommitted receipt = %+v, want exact zero", result)
			}
			if tc.wantCommitted && (result.Status != core.HTTPStatusOK() || destination.status != http.StatusOK) {
				t.Fatalf("receipt/written status = (%v, %d), want OK", result.Status, destination.status)
			}
			if err := result.Validate(); err != nil {
				t.Fatalf("receipt validation error = %v, want nil", err)
			}
			if len(tc.wantBody) > 0 && !bytes.Equal(destination.body.Bytes(), []byte(tc.body)[:tc.wantBytes]) {
				t.Fatalf("released prefix = %q, want exact producer prefix", destination.body.Bytes())
			}
		})
	}
}
