package exchange

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestWriteStreamUnknownLengthPreservesEveryByteLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		size     uint64
		terminal error
	}{
		{name: "unknown empty source completes without a fabricated declaration"},
		{name: "one byte does not require a declaration", size: 1},
		{name: "one beyond a transfer window reaches the destination", size: TransferBufferBytes + 1},
		{name: "multiple windows retain the final byte", size: (1 << 20) + 1},
		{name: "late native failure preserves the delivered prefix", size: TransferBufferBytes + 1, terminal: io.ErrUnexpectedEOF},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			destination := httptest.NewRecorder()
			call, err := NewSocketServerCall(destination, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
			if err != nil {
				t.Fatalf("socket=%v, want nil", err)
			}
			source := &streamingReviewBody{remaining: tc.size, terminal: tc.terminal}
			err = WriteStream(StreamWriteCall{Call: call, Response: ServerStreamResponse{Source: source, ContentType: core.HTTPMediaTypeOctetStream(), Status: core.HTTPStatusOK()}})
			if !errors.Is(err, tc.terminal) || errors.Is(err, core.ErrExchangeBodyLimit) || source.remaining != 0 || uint64(destination.Body.Len()) != tc.size {
				t.Fatalf("unknown stream=(%v,%d remaining,%d written), want %v and exact %d bytes", err, source.remaining, destination.Body.Len(), tc.terminal, tc.size)
			}
			if tc.terminal != nil && !errors.Is(err, core.ErrExchangeWrite) {
				t.Fatalf("native failure=%v, want write identity", err)
			}
			if length := destination.Header().Get(core.HTTPHeaderContentLength().String()); length != "" {
				t.Fatalf("declared length=%q, want absent declaration", length)
			}
			if !bytes.Equal(destination.Body.Bytes(), bytes.Repeat([]byte{0xa5}, int(tc.size))) {
				t.Fatalf("written payload differs at size %d", tc.size)
			}
		})
	}
}

// The real Go client and server own framing. Each side crosses Exchange's
// streaming boundary; the sinks verify bytes without retaining either stream.
func TestUnknownLengthHTTPStreamsCrossBothDirectionsLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		size     uint64
		known    bool
		declared uint64
		terminal error
		wantErr  error
	}{
		{name: "unknown empty request"},
		{name: "unknown request beyond one copy window", size: TransferBufferBytes + 1},
		{name: "unknown sixteen MiB request", size: 16 << 20},
		{name: "declared empty request", known: true},
		{name: "declared binary request", known: true, size: 3, declared: 3},
		{name: "declared empty cannot transmit a byte", known: true, size: 1, wantErr: core.ErrExchangeBodyLimit},
		{name: "joined EOF cannot hide an empty-source failure", known: true, terminal: errors.Join(io.EOF, io.ErrClosedPipe), wantErr: io.ErrClosedPipe},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, roundTrip := range []bool{false, true} {
				const replyBytes = TransferBufferBytes + 1
				type received struct {
					length            int64
					count             uint64
					readErr, writeErr error
				}
				observed := make(chan received, 1)
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					call, err := NewSocketServerCall(w, r)
					if err != nil {
						observed <- received{readErr: err}
						return
					}
					sink := &streamingReviewSink{}
					_, readErr := ReceiveStream(StreamReceiveCall{Call: call, Route: RouteSemantics{Method: MethodPost, Replay: ReplaySingleAttempt}, Destination: sink, ExpectedContentType: core.HTTPMediaTypeOctetStream(), Buffer: make([]byte, TransferBufferBytes)})
					writeErr := WriteStream(StreamWriteCall{Call: call, Response: ServerStreamResponse{Source: &streamingReviewBody{remaining: replyBytes}, ContentType: core.HTTPMediaTypeOctetStream(), Status: core.HTTPStatusOK(), Buffer: make([]byte, TransferBufferBytes)}})
					observed <- received{length: r.ContentLength, count: sink.bytes, readErr: readErr, writeErr: writeErr}
				}))
				t.Cleanup(server.Close)
				client, err := NewClient(server.Client())
				if err != nil {
					server.Close()
					t.Fatalf("client=%v, want nil", err)
				}
				target, err := core.ParseHTTPEndpoint(server.URL)
				if err != nil {
					server.Close()
					t.Fatalf("target=%v, want nil", err)
				}
				length, err := core.NewByteLength(tc.declared)
				if err != nil {
					server.Close()
					t.Fatalf("length=%v, want nil", err)
				}
				var declared *core.ByteLength
				if tc.known {
					declared = &length
				}
				source := &streamingReviewBody{remaining: tc.size, terminal: tc.terminal}
				destination := &streamingReviewSink{}
				timeout := runtimeAgreementPolicy(t).ReadTimeout
				policy := StreamPolicy{OperationTimeout: timeout, AttemptTimeout: timeout, Redirect: RedirectPolicy{Mode: RedirectReject}}
				semantics := RequestSemantics{Method: MethodPost, Replay: ReplaySingleAttempt}
				var result StreamResponse
				if roundTrip {
					value, callErr := RoundTripStream(StreamRoundTripCall{Context: t.Context(), Client: client, Policy: policy, Request: StreamRoundTripRequest{Target: target, Source: source, Destination: destination, RequestContentLength: declared, Semantics: semantics, RequestContentType: core.HTTPMediaTypeOctetStream(), ExpectedResponseContentType: core.HTTPMediaTypeOctetStream(), ExpectedStatus: core.HTTPStatusOK(), Buffer: make([]byte, TransferBufferBytes)}})
					result, err = StreamResponse(value), callErr
				} else {
					result, err = Upload(UploadCall{Context: t.Context(), Client: client, Policy: policy, Request: UploadRequest{Target: target, Source: source, ContentLength: declared, Semantics: semantics, ContentType: core.HTTPMediaTypeOctetStream(), ExpectedStatus: core.HTTPStatusOK()}})
				}
				server.Close()
				var got received
				select {
				case got = <-observed:
					calls++
				default:
				}
				if !errors.Is(err, tc.wantErr) || source.closes != 0 {
					t.Fatalf("roundTrip=%t result=(%+v,%v,%d source closes), want %v and borrowed source", roundTrip, result, err, source.closes, tc.wantErr)
				}
				if tc.wantErr != nil {
					if calls != 0 || result.Metadata.Attempts != 0 || result.RequestLengthKnown || result.DeclaredRequestBytes != (core.ByteLength{}) || !errors.Is(err, core.ErrExchangeRequest) {
						t.Fatalf("refusal=(%+v,%v,%d HTTP calls), want absent observation before transport", result, err, calls)
					}
					continue
				}
				wantLength := int64(declaredBodyLengthAbsent)
				if tc.known {
					wantLength = int64(tc.declared)
				}
				wantResponseBytes := uint64(0)
				if roundTrip {
					wantResponseBytes = replyBytes
				}
				if calls != 1 || got.readErr != nil || got.writeErr != nil || got.length != wantLength || got.count != tc.size || source.remaining != 0 || destination.bytes != wantResponseBytes {
					t.Fatalf("roundTrip=%t wire=(%+v,%d calls,%d remaining,%d response bytes), want length=%d and %d request bytes; read/write errors=(%v,%v)", roundTrip, got, calls, source.remaining, destination.bytes, wantLength, tc.size, got.readErr, got.writeErr)
				}
				// Input extent is borrowed only during the call; returned facts own their value.
				length = core.ByteLength{}
				if result.Validate() != nil || result.Metadata.Status != core.HTTPStatusOK() || result.Metadata.Attempts != 1 || result.RequestLengthKnown != tc.known || result.DeclaredRequestBytes.Uint64() != tc.declared || result.Metadata.Bytes.Uint64() != wantResponseBytes {
					t.Fatalf("roundTrip=%t observation=%+v, want exact independent declaration and received extent", roundTrip, result)
				}
			}
		})
	}
}

func TestExactStreamEndPreservesJoinedNativeFailures(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                    string
		size                    uint64
		terminal, errorIdentity error
	}{
		{name: "clean empty EOF"},
		{name: "clean EOF after exact data", size: 3},
		{name: "empty EOF joined with pipe failure", terminal: errors.Join(io.EOF, io.ErrClosedPipe), errorIdentity: io.ErrClosedPipe},
		{name: "EOF joined with failure after exact data", size: 3, terminal: errors.Join(io.EOF, io.ErrUnexpectedEOF), errorIdentity: io.ErrUnexpectedEOF},
		{name: "joined terminal cause after a complete window", size: TransferBufferBytes, terminal: errors.Join(io.EOF, io.ErrClosedPipe), errorIdentity: io.ErrClosedPipe},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			writer := httptest.NewRecorder()
			call, err := NewSocketServerCall(writer, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
			if err != nil {
				t.Fatalf("socket=%v, want nil", err)
			}
			length, err := core.NewByteLength(tc.size)
			if err != nil {
				t.Fatalf("extent=%v, want nil", err)
			}
			source := &streamingReviewBody{remaining: tc.size, terminal: tc.terminal}
			err = WriteStream(StreamWriteCall{Call: call, Response: ServerStreamResponse{Source: source, ContentLength: &length, ContentType: core.HTTPMediaTypeOctetStream(), Status: core.HTTPStatusOK()}})
			if !errors.Is(err, tc.errorIdentity) || uint64(writer.Body.Len()) != tc.size || source.remaining != 0 || tc.errorIdentity != nil && !errors.Is(err, core.ErrExchangeWrite) {
				t.Fatalf("exact stream=(%v,%d written,%d remaining), want %v and exact %d-byte prefix", err, writer.Body.Len(), source.remaining, tc.errorIdentity, tc.size)
			}
		})
	}
}
