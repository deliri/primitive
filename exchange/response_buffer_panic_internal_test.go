package exchange

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type bufferPanicPhase uint8

const (
	bufferNoPanic bufferPanicPhase = iota
	bufferHeaderPanic
	bufferStatusPanic
	bufferBodyPanic
	bufferStatusPanicAfterEffect
	bufferBodyPanicAfterEffect
)

type bufferPanicDestination struct {
	admissionResponseWriter
	phase bufferPanicPhase
}

func (w *bufferPanicDestination) Header() http.Header {
	w.headerCalls++
	if w.phase == bufferHeaderPanic {
		panic(io.ErrClosedPipe)
	}
	return w.header
}
func (w *bufferPanicDestination) WriteHeader(status int) {
	w.statusCalls++
	if w.phase == bufferStatusPanic {
		panic(io.ErrClosedPipe)
	}
	w.status = status
	if w.phase == bufferStatusPanicAfterEffect {
		panic(io.ErrClosedPipe)
	}
}
func (w *bufferPanicDestination) Write(data []byte) (int, error) {
	w.writes++
	if w.phase == bufferBodyPanic {
		panic(io.ErrClosedPipe)
	}
	n, err := w.body.Write(data)
	if w.phase == bufferBodyPanicAfterEffect {
		panic(io.ErrClosedPipe)
	}
	return n, err
}

func TestResponseBufferDestinationPanicRetainsCompletedReceipt(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, body                            string
		phase                                 bufferPanicPhase
		wantErr                               error
		wantCommitted                         bool
		wantHeaders, wantStatuses, wantWrites int
		wantBody                              string
		wantBytes                             uint64
	}{
		{name: "completed destination returns exact body and status", body: "abcd", wantCommitted: true, wantHeaders: 1, wantStatuses: 1, wantWrites: 1, wantBody: "abcd", wantBytes: 4},
		{name: "Header panic cannot escape or invent a status receipt", body: "abcd", phase: bufferHeaderPanic, wantErr: core.ErrExchangeWrite, wantHeaders: 1},
		{name: "WriteHeader panic cannot acknowledge an unfinished status call", body: "abcd", phase: bufferStatusPanic, wantErr: core.ErrExchangeWrite, wantHeaders: 1, wantStatuses: 1},
		{name: "body panic retains completed status without inventing acknowledged bytes", body: "abcd", phase: bufferBodyPanic, wantErr: core.ErrExchangeWrite, wantCommitted: true, wantHeaders: 1, wantStatuses: 1, wantWrites: 1},
		{name: "status panic after effect still cannot acknowledge the unfinished call", body: "abcd", phase: bufferStatusPanicAfterEffect, wantErr: core.ErrExchangeWrite, wantHeaders: 1, wantStatuses: 1},
		{name: "body panic after effect cannot invent an acknowledged byte count", body: "abcd", phase: bufferBodyPanicAfterEffect, wantErr: core.ErrExchangeWrite, wantCommitted: true, wantHeaders: 1, wantStatuses: 1, wantWrites: 1, wantBody: "abcd"},
		{name: "empty body never enters a panicking Write", phase: bufferBodyPanic, wantCommitted: true, wantHeaders: 1, wantStatuses: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			destination := &bufferPanicDestination{header: make(http.Header), phase: tc.phase}
			call, err := NewSocketServerCall(destination, httptest.NewRequest(http.MethodGet, "/", nil))
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			var result ResponseBufferResult
			var gotErr error
			var escaped any
			func() {
				defer func() { escaped = recover() }()
				result, gotErr = BufferResponse(t.Context(), ResponseBufferRequest{Call: call, BodyMaximum: mustInternalByteCount(t, 4), Serve: func(socket SocketServerCall) error {
					calls++
					socket.writer.WriteHeader(http.StatusCreated)
					if len(tc.body) == 0 {
						return nil
					}
					_, err := io.WriteString(socket.writer, tc.body)
					return err
				}})
			}()
			if escaped != nil || !errors.Is(gotErr, tc.wantErr) || tc.wantErr != nil && (!errors.Is(gotErr, core.ErrExchangeResponse) || !errors.Is(gotErr, core.ErrExchangeContract)) {
				t.Fatalf("destination panic/error=(%v,%v),want no escaped panic and %v with response/contract identity", escaped, gotErr, tc.wantErr)
			}
			if calls != 1 || destination.headerCalls != tc.wantHeaders || destination.statusCalls != tc.wantStatuses || destination.writes != tc.wantWrites || destination.body.String() != tc.wantBody {
				t.Fatalf("callback/effects=%d/%+v,want exact reached boundaries", calls, destination)
			}
			if result.Committed != tc.wantCommitted || result.Bytes.Uint64() != tc.wantBytes || result.Validate() != nil {
				t.Fatalf("receipt=%+v,want committed=%t and acknowledged bytes=%d", result, tc.wantCommitted, tc.wantBytes)
			}
			if tc.phase == bufferStatusPanicAfterEffect && destination.status != http.StatusCreated {
				t.Fatalf("pre-panic status=%d, want %d", destination.status, http.StatusCreated)
			}
			if errors.Is(gotErr, io.ErrClosedPipe) {
				t.Fatalf("panic error=%v, want no %v identity", gotErr, io.ErrClosedPipe)
			}
			if !tc.wantCommitted {
				if result != (ResponseBufferResult{}) {
					t.Fatalf("unfinished status returned receipt %+v", result)
				}
				return
			}
			status, statusErr := result.Status.Int()
			if statusErr != nil || status != http.StatusCreated || destination.status != http.StatusCreated {
				t.Fatalf("completed status=%d,%v,destination=%d,want Go created status", status, statusErr, destination.status)
			}
		})
	}
}
