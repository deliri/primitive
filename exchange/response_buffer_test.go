package exchange

import (
	"bytes"
	"context"
	"errors"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestResponseBufferLayerTriad(t *testing.T) {
	t.Parallel()
	const ceiling = 8
	cases := []struct {
		name          string
		body          []byte
		status        int
		cause         error
		wantErr       error
		wantCommitted bool
	}{
		{"one below ceiling releases exact bytes", bytes.Repeat([]byte{'a'}, ceiling-1), http.StatusCreated, nil, nil, true},
		{"exact ceiling releases exact bytes", bytes.Repeat([]byte{'b'}, ceiling), http.StatusOK, nil, nil, true},
		{"one above ceiling refuses even ignored write error", bytes.Repeat([]byte{'c'}, ceiling+1), http.StatusOK, nil, core.ErrExchangeBodyLimit, false},
		{"extreme payload refuses before allocation", bytes.Repeat([]byte{'d'}, TransferBufferBytes), http.StatusOK, nil, core.ErrExchangeBodyLimit, false},
		{"product refusal withholds already written bytes", []byte("secret"), http.StatusOK, context.Canceled, context.Canceled, false},
		{"empty successful response has zero body bytes", nil, http.StatusNoContent, nil, nil, true},
		{"no-content status rejects body", []byte("body"), http.StatusNoContent, nil, http.ErrBodyNotAllowed, false},
		{"not-modified status rejects body", []byte("body"), http.StatusNotModified, nil, http.ErrBodyNotAllowed, false},
		{"informational status is outside buffered scope", nil, http.StatusEarlyHints, nil, core.ErrExchangeResponse, false},
		{"out of domain status cannot panic", nil, 1000, nil, core.ErrExchangeResponse, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			maximum, err := core.NewByteCount(ceiling)
			if err != nil {
				t.Fatal(err)
			}
			destination := httptest.NewRecorder()
			result, err := BufferResponse(context.Background(), ResponseBufferRequest{Call: SocketServerCall{writer: destination, request: httptest.NewRequest(http.MethodGet, "/", nil)}, BodyMaximum: maximum, Serve: func(call SocketServerCall) error {
				writer := call.writer
				writer.Header().Set("X-Buffer-Test", "retained")
				writer.WriteHeader(tc.status)
				if len(tc.body) > 0 {
					// Deliberately exercise handlers which ignore their own write refusal.
					_, writeErr := writer.Write(tc.body)
					if writeErr != nil && !errors.Is(writeErr, tc.wantErr) {
						return writeErr
					}
				}
				return tc.cause
			}})
			if !errors.Is(err, tc.wantErr) || result.Committed != tc.wantCommitted {
				t.Fatalf("buffer result = (%+v,%v), want committed:%t error:%v", result, err, tc.wantCommitted, tc.wantErr)
			}
			if err := result.Validate(); err != nil {
				t.Fatal(err)
			}
			if !tc.wantCommitted {
				if result != (ResponseBufferResult{}) || destination.Body.Len() != 0 || len(destination.Header()) != 0 {
					t.Fatalf("refused output = (%+v,%q,%v), want no released facts", result, destination.Body.Bytes(), destination.Header())
				}
				return
			}
			if destination.Code != tc.status || !bytes.Equal(destination.Body.Bytes(), tc.body) {
				t.Fatalf("released response = %d/%q, want %d/%q", destination.Code, destination.Body.Bytes(), tc.status, tc.body)
			}
		})
	}
}

// This adapter sends the body through a real closed pipe; the recorder only
// observes HTTP headers. It does not synthesize the transport's write outcome.
type pipeResponseWriter struct {
	*httptest.ResponseRecorder
	body *io.PipeWriter
}

func (w pipeResponseWriter) Write(data []byte) (int, error) { return w.body.Write(data) }

func TestResponseBufferPreservesRealDestinationFailure(t *testing.T) {
	t.Parallel()
	reader, writer := io.Pipe()
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := writer.Close(); err != nil {
			t.Error(err)
		}
	}()
	limit, err := core.NewByteCount(8)
	if err != nil {
		t.Fatal(err)
	}
	destination := pipeResponseWriter{httptest.NewRecorder(), writer}
	result, err := BufferResponse(context.Background(), ResponseBufferRequest{Call: SocketServerCall{writer: destination, request: httptest.NewRequest(http.MethodGet, "/", nil)}, BodyMaximum: limit, Serve: func(call SocketServerCall) error {
		w := call.writer
		_, err := w.Write([]byte("body"))
		return err
	}})
	if !errors.Is(err, io.ErrClosedPipe) || !errors.Is(err, io.ErrShortWrite) || !result.Committed || destination.Code != http.StatusOK {
		t.Fatalf("closed-pipe release = (%+v,%v,status:%d), want committed headers and closed-pipe/short-write errors", result, err, destination.Code)
	}
	if result.Bytes != (core.ByteLength{}) {
		t.Fatalf("delivered bytes = %+v, want zero", result.Bytes)
	}
}

func TestResponseBufferHeaderAndCancellationLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		header  http.Header
		cancel  bool
		wantErr error
	}{
		{"canonical declared length matches", http.Header{core.HTTPHeaderContentLength().String(): []string{"4"}}, false, nil},
		{"false declared length refuses", http.Header{core.HTTPHeaderContentLength().String(): []string{"5"}}, false, core.ErrExchangeResponse},
		{"duplicate declared length refuses", http.Header{core.HTTPHeaderContentLength().String(): []string{"4", "4"}}, false, core.ErrExchangeResponse},
		{"noncanonical name cannot bypass framing", http.Header{"content-length": []string{"5"}}, false, core.ErrExchangeResponse},
		{"trailer protocol requires streaming path", http.Header{core.HTTPHeaderTrailer().String(): []string{"Digest"}}, false, core.ErrExchangeResponse},
		{"header injection refuses", http.Header{"X-Test": []string{"a\r\nb"}}, false, core.ErrExchangeResponse},
		{"cancellation before release withholds bytes", nil, true, context.Canceled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			limit, err := core.NewByteCount(8)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			destination := httptest.NewRecorder()
			result, err := BufferResponse(ctx, ResponseBufferRequest{Call: SocketServerCall{writer: destination, request: httptest.NewRequest(http.MethodGet, "/", nil)}, BodyMaximum: limit, Serve: func(call SocketServerCall) error {
				w := call.writer
				maps.Copy(w.Header(), tc.header)
				_, writeErr := w.Write([]byte("body"))
				if tc.cancel {
					cancel()
				}
				return writeErr
			}})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("header/cancel error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil && (result.Committed || destination.Body.Len() != 0 || len(destination.Header()) != 0) {
				t.Fatalf("refused response = %+v/%q/%v, want unreleased", result, destination.Body.Bytes(), destination.Header())
			}
		})
	}
}

func FuzzResponseBufferSemanticExtent(f *testing.F) {
	seed := ServerBoundedResponse{Body: []byte{0, 'a', 0xff}, ContentType: core.HTTPMediaTypeOctetStream(), Status: core.HTTPStatusOK()}
	if err := seed.Validate(); err != nil {
		f.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := WriteBounded(BoundedWriteCall{Call: SocketServerCall{writer: recorder, request: httptest.NewRequest(http.MethodGet, "/", nil)}, Response: seed}); err != nil {
		f.Fatal(err)
	}
	for _, limit := range []uint16{0, uint16(len(seed.Body) - 2), uint16(len(seed.Body) - 1), uint16(len(seed.Body))} {
		f.Add(recorder.Body.Bytes(), limit)
	}
	f.Add([]byte{}, uint16(0))
	f.Fuzz(func(t *testing.T, data []byte, rawLimit uint16) {
		if len(data) > 65536 {
			return
		}
		maximum, err := core.NewByteCount(uint64(rawLimit) + 1)
		if err != nil {
			t.Fatal(err)
		}
		cases := []struct {
			name                                                              string
			head, cancelled, callbackFailure, falseLength, destinationFailure bool
			panicWrite, panicAfterWrite                                       bool
		}{
			{name: "exact release after completed callback"},
			{name: "HEAD retains representation framing but releases no body", head: true},
			{name: "callback refusal withholds buffered bytes", callbackFailure: true},
			{name: "cancellation before release withholds buffered bytes", cancelled: true},
			{name: "false representation extent withholds buffered bytes", falseLength: true},
			{name: "partial destination failure retains actual committed count", destinationFailure: true},
			{name: "destination panic before effect retains only completed status", panicWrite: true},
			{name: "destination panic after effect cannot invent acknowledgment", panicAfterWrite: true},
		}
		for _, tc := range cases {
			destination := &materialFuzzWriter{header: make(http.Header), fail: tc.destinationFailure, acknowledge: len(data) / 2, panicWrite: tc.panicWrite, panicAfterWrite: tc.panicAfterWrite}
			ctx, cancel := context.WithCancel(t.Context())
			method := http.MethodGet
			if tc.head {
				method = http.MethodHead
			}
			calls := 0
			result, gotErr := BufferResponse(ctx, ResponseBufferRequest{Call: SocketServerCall{writer: destination, request: httptest.NewRequestWithContext(ctx, method, "/", nil)}, BodyMaximum: maximum, Serve: func(call SocketServerCall) error {
				calls++
				length := len(data)
				if tc.falseLength {
					length++
				}
				call.writer.Header().Set(core.HTTPHeaderContentLength().String(), strconv.Itoa(length))
				call.writer.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeOctetStream().String())
				call.writer.WriteHeader(http.StatusCreated)
				// Ignore the local write result: the buffer must retain mechanical refusal.
				_, _ = call.writer.Write(data)
				if tc.cancelled {
					cancel()
				}
				if tc.callbackFailure {
					return io.ErrUnexpectedEOF
				}
				return nil
			}})
			cancel()
			overflow := len(data) > int(rawLimit)+1
			withheld := overflow || tc.cancelled || tc.callbackFailure || tc.falseLength
			if calls != 1 || result.Validate() != nil {
				t.Fatalf("%s callback/result=(%d,%+v),want one callback and valid receipt", tc.name, calls, result)
			}
			if errors.Is(gotErr, core.ErrExchangeBodyLimit) != overflow || errors.Is(gotErr, context.Canceled) != tc.cancelled || errors.Is(gotErr, io.ErrUnexpectedEOF) != tc.callbackFailure {
				t.Fatalf("%s causes=%v,want overflow/cancel/callback=%t/%t/%t", tc.name, gotErr, overflow, tc.cancelled, tc.callbackFailure)
			}
			if withheld {
				if gotErr == nil || result != (ResponseBufferResult{}) || destination.commits != 0 || destination.writes != 0 || destination.body.Len() != 0 || len(destination.header) != 0 {
					t.Fatalf("%s withheld result=(%+v,%v),destination=%+v", tc.name, result, gotErr, destination)
				}
				continue
			}
			wantBody := data
			if tc.head {
				wantBody = nil
			}
			wantWrites := 0
			if len(wantBody) > 0 {
				wantWrites = 1
			}
			panickedWrite := (tc.panicWrite || tc.panicAfterWrite) && wantWrites == 1
			if tc.panicWrite {
				wantBody = nil
			}
			failedWrite := tc.destinationFailure && wantWrites == 1
			if failedWrite {
				wantBody = wantBody[:len(wantBody)/2]
			}
			if errors.Is(gotErr, io.ErrClosedPipe) != failedWrite || errors.Is(gotErr, io.ErrShortWrite) != failedWrite || errors.Is(gotErr, core.ErrExchangeWrite) != (failedWrite || panickedWrite) || panickedWrite && !errors.Is(gotErr, core.ErrExchangeContract) || !failedWrite && !panickedWrite && gotErr != nil {
				t.Fatalf("%s destination causes=%v,want write failure=%t", tc.name, gotErr, failedWrite)
			}
			wantAcknowledged := len(wantBody)
			if panickedWrite {
				wantAcknowledged = 0
			}
			status, statusErr := result.Status.Int()
			if !result.Committed || statusErr != nil || status != http.StatusCreated || result.Bytes.Uint64() != uint64(wantAcknowledged) || destination.status != http.StatusCreated || destination.commits != 1 || destination.writes != wantWrites || !bytes.Equal(destination.body.Bytes(), wantBody) {
				t.Fatalf("%s receipt=(%+v,%v),destination=%+v,want exact committed bytes %q", tc.name, result, gotErr, destination, wantBody)
			}
			if len(destination.header) != 2 || destination.header.Get(core.HTTPHeaderContentLength().String()) != strconv.Itoa(len(data)) || destination.header.Get(core.HTTPHeaderContentType().String()) != core.HTTPMediaTypeOctetStream().String() {
				t.Fatalf("%s framing=%v,want exact representation extent and MIME", tc.name, destination.header)
			}
		}
	})
}

func TestResponseBufferRepresentationLengthLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		method   string
		status   int
		body     string
		length   string
		wantBody string
		wantErr  error
	}{
		{"HEAD suppresses generated representation", http.MethodHead, http.StatusOK, "body", "4", "", nil},
		{"HEAD permits ungenerated representation length", http.MethodHead, http.StatusOK, "", "4", "", nil},
		{"HEAD refuses incorrect generated length", http.MethodHead, http.StatusOK, "body", "5", "", core.ErrExchangeResponse},
		{"HEAD refuses malformed length", http.MethodHead, http.StatusOK, "", "-1", "", core.ErrExchangeResponse},
		{"not modified retains representation length", http.MethodGet, http.StatusNotModified, "", "4", "", nil},
		{"ordinary empty response refuses false length", http.MethodGet, http.StatusOK, "", "4", "", core.ErrExchangeResponse},
		{"ordinary response releases representation", http.MethodGet, http.StatusOK, "body", "4", "body", nil},
		{"HEAD empty representation remains empty", http.MethodHead, http.StatusOK, "", "0", "", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			limit, err := core.NewByteCount(8)
			if err != nil {
				t.Fatal(err)
			}
			destination := httptest.NewRecorder()
			result, err := BufferResponse(t.Context(), ResponseBufferRequest{Call: SocketServerCall{writer: destination, request: httptest.NewRequest(tc.method, "/", nil)}, BodyMaximum: limit, Serve: func(call SocketServerCall) error {
				call.writer.Header().Set(core.HTTPHeaderContentLength().String(), tc.length)
				call.writer.WriteHeader(tc.status)
				if tc.body == "" {
					return nil
				}
				_, err := io.WriteString(call.writer, tc.body)
				return err
			}})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("BufferResponse error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if result != (ResponseBufferResult{}) || destination.Body.Len() != 0 || len(destination.Header()) != 0 {
					t.Fatalf("refused output = (%+v,%q,%v), want no release", result, destination.Body.String(), destination.Header())
				}
				return
			}
			wantBytes, err := core.NewByteLength(uint64(len(tc.wantBody)))
			if err != nil {
				t.Fatal(err)
			}
			if !result.Committed || result.Bytes != wantBytes || destination.Code != tc.status || destination.Body.String() != tc.wantBody || destination.Header().Get(core.HTTPHeaderContentLength().String()) != tc.length {
				t.Fatalf("released representation = (%+v,%d,%q,%v), want status %d body %q length %s", result, destination.Code, destination.Body.String(), destination.Header(), tc.status, tc.wantBody, tc.length)
			}
		})
	}
}
