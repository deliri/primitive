package exchange

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestOfficialSDKBinaryBodyTransfersUnreadOwnership(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		size     uint64
		terminal error
		abandon  bool
	}{
		{name: "empty body remains an owned stream"},
		{name: "body beyond former SDK cutoff remains unread", size: (1 << 20) + 1},
		{name: "64 MiB streams through one caller window", size: 64 << 20},
		{name: "terminal read error survives all preceding bytes", size: (1 << 20) + 1, terminal: io.ErrUnexpectedEOF},
		{name: "caller may close before consuming the remainder", size: 64 << 20, abandon: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := &streamingReviewBody{remaining: tc.size, terminal: tc.terminal}
			boundary, err := NewOfficialSDKMethodResponseBoundary(OfficialSDKMethodResponseBoundaryRequest{Method: MethodGet, Representation: OfficialSDKResponseRepresentationBinary})
			if err != nil {
				t.Fatalf("boundary error=%v, want nil", err)
			}
			calls := 0
			transport, err := NewOfficialSDKResponseTransport(OfficialSDKResponseTransportRequest{Boundary: boundary, Base: replayHandoffTransport(func(request *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: http.StatusOK, Body: source, ContentLength: -1, Request: request}, nil
			})})
			if err != nil {
				t.Fatalf("transport error=%v, want nil", err)
			}
			response, err := transport.RoundTrip(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://sdk-stream.invalid/body", nil))
			if err != nil || response == nil {
				t.Fatalf("RoundTrip=(%v,%v), want response and nil", response, err)
			}
			if response.Body != source || calls != 1 || source.reads != 0 || source.closes != 0 || source.remaining != tc.size {
				t.Fatalf("custody=(%T,%d calls,%d reads,%d closes,%d remaining), want original unread stream of %d bytes", response.Body, calls, source.reads, source.closes, source.remaining, tc.size)
			}
			sink := &streamingReviewSink{}
			if !tc.abandon {
				count, readErr := io.CopyBuffer(sink, response.Body, make([]byte, TransferBufferBytes))
				if !errors.Is(readErr, tc.terminal) || uint64(count) != tc.size || sink.bytes != tc.size || source.remaining != 0 {
					t.Fatalf("stream=(%d,%v,%d remaining), want %d bytes and %v", count, readErr, source.remaining, tc.size, tc.terminal)
				}
			}
			closeErr := response.Body.Close()
			if closeErr != nil || source.closes != 1 || tc.abandon && source.remaining != tc.size {
				t.Fatalf("close=(%v,%d closes,%d remaining), want one close preserving unread custody", closeErr, source.closes, source.remaining)
			}
		})
	}
}

func TestResponseBufferRetainsWholeValuesWithoutTransferQuotas(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		size   uint64
		refuse bool
	}{
		{name: "one beyond a copy window is complete", size: TransferBufferBytes + 1},
		{name: "one beyond the former sixteen MiB cutoff is complete", size: (16 << 20) + 1},
		{name: "callback refusal after a large body publishes nothing", size: (16 << 20) + 1, refuse: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			destination := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			call, err := NewSocketServerCall(destination, request)
			if err != nil {
				t.Fatalf("socket=%v, want nil", err)
			}
			result, err := BufferResponse(t.Context(), ResponseBufferRequest{Call: call, Serve: func(socket SocketServerCall) error {
				source := &streamingReviewBody{remaining: tc.size}
				count, err := io.CopyBuffer(socket.writer, source, make([]byte, TransferBufferBytes))
				if err != nil || uint64(count) != tc.size {
					return errors.Join(err, io.ErrShortWrite)
				}
				if tc.refuse {
					return io.ErrClosedPipe
				}
				return nil
			}})
			if tc.refuse {
				if !errors.Is(err, io.ErrClosedPipe) || result != (ResponseBufferResult{}) || destination.Body.Len() != 0 {
					t.Fatalf("refusal=(%+v,%v,%d bytes), want zero result and native failure", result, err, destination.Body.Len())
				}
				return
			}
			if err != nil || result.Validate() != nil || !result.Committed || result.Bytes.Uint64() != tc.size || result.Status != core.HTTPStatusOK() {
				t.Fatalf("release=(%+v,%v), want complete %d-byte result", result, err, tc.size)
			}
			sink := &streamingReviewSink{}
			count, err := io.Copy(sink, destination.Body)
			if err != nil || uint64(count) != tc.size || sink.bytes != tc.size {
				t.Fatalf("released bytes=(%d,%v), want exact %d-byte value", count, err, tc.size)
			}
		})
	}
}
func TestOfficialSDKResponseRefusesAbsentReaderBeforeHandoff(t *testing.T) {
	t.Parallel()
	var typedNil *streamingReviewBody
	for _, tc := range []struct {
		name string
		body io.ReadCloser
	}{
		{name: "nil interface"},
		{name: "typed nil reader", body: typedNil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, representation := range []OfficialSDKResponseRepresentation{OfficialSDKResponseRepresentationBinary, OfficialSDKResponseRepresentationJSON} {
				boundary, err := NewOfficialSDKMethodResponseBoundary(OfficialSDKMethodResponseBoundaryRequest{Method: MethodGet, Representation: representation})
				if err != nil {
					t.Fatalf("boundary=%v, want nil", err)
				}
				transport, err := NewOfficialSDKResponseTransport(OfficialSDKResponseTransportRequest{Boundary: boundary, Base: replayHandoffTransport(func(request *http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: http.StatusOK, ContentLength: -1, Body: tc.body, Request: request}, nil
				})})
				if err != nil {
					t.Fatalf("transport=%v, want nil", err)
				}
				response, err := transport.RoundTrip(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://sdk-stream.invalid/body", nil))
				if response != nil || !errors.Is(err, core.ErrExchangeContract) || !errors.Is(err, core.ErrExchangeTransport) {
					t.Fatalf("%v handoff=(%v,%v), want nil response and typed absent-reader refusal", representation, response, err)
				}
			}
		})
	}
}
