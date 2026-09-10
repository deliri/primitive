package exchange_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type streamFuzzFault uint8

const (
	streamFuzzIntact streamFuzzFault = iota
	streamFuzzCancelOnRead
	streamFuzzReadFailure
	streamFuzzCloseFailure
	streamFuzzShortWrite
)

type streamFuzzReader struct {
	window int
	source *bytes.Reader
	fault  streamFuzzFault
	cancel context.CancelFunc
}

func (r *streamFuzzReader) Read(p []byte) (int, error) {
	if len(p) > max(1, r.window) {
		p = p[:max(1, r.window)]
	}
	n, err := r.source.Read(p)
	if r.fault == streamFuzzCancelOnRead {
		r.cancel()
	}
	if errors.Is(err, io.EOF) && r.fault == streamFuzzReadFailure {
		return n, io.ErrUnexpectedEOF
	}
	return n, err
}

func FuzzReceiveStreamCustodySemanticClosure(f *testing.F) {
	// The valid binary seed comes from the real validated writer. The oracle
	// below uses source bytes, a one-byte reader, and count conservation; it
	// does not use Exchange's copy or classification helpers to compute wants.
	seed := []byte{0x00, 0xff, 0x41}
	writer := httptest.NewRecorder()
	response := exchange.ServerStreamResponse{Source: bytes.NewReader(seed), ContentType: core.HTTPMediaTypeOctetStream(), ContentLength: new(mustByteLength(f, uint64(len(seed)))), Status: core.HTTPStatusOK()}
	if err := response.Validate(); err != nil {
		f.Fatalf("typed stream seed validation = %v, want nil", err)
	}
	if err := exchange.WriteStream(exchange.StreamWriteCall{Call: socketServerCallFrom(f, writer, httptest.NewRequest(http.MethodGet, "/", nil)), Response: response}); err != nil {
		f.Fatalf("production stream seed write = %v, want nil", err)
	}
	if !bytes.Equal(writer.Body.Bytes(), seed) {
		f.Fatalf("production stream seed = %x, want %x", writer.Body.Bytes(), seed)
	}
	canonical := writer.Body.Bytes()
	f.Add(canonical, uint16(len(seed)), uint8(streamFuzzIntact))
	f.Add(canonical, uint16(len(seed)-1), uint8(streamFuzzIntact))
	f.Add(canonical, uint16(len(seed)+1), uint8(streamFuzzIntact))
	f.Add(canonical, uint16(0), uint8(streamFuzzIntact))
	f.Add(canonical, uint16(len(seed)), uint8(streamFuzzCancelOnRead))
	f.Add(canonical, uint16(len(seed)), uint8(streamFuzzReadFailure))
	f.Add(canonical, uint16(len(seed)-1), uint8(streamFuzzCloseFailure))
	f.Add(canonical, uint16(len(seed)), uint8(streamFuzzShortWrite))
	f.Add([]byte{}, uint16(1), uint8(streamFuzzIntact))
	f.Fuzz(func(t *testing.T, data []byte, window uint16, faultByte uint8) {
		const oracleMaximumBytes = 4096
		if len(data) > oracleMaximumBytes || faultByte > uint8(streamFuzzShortWrite) {
			return
		}
		fault := streamFuzzFault(faultByte)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		source := &streamFuzzReader{source: bytes.NewReader(data), fault: fault, cancel: cancel}
		body := &bindingObservedBody{reader: source}
		if fault == streamFuzzCloseFailure {
			body.err = io.ErrClosedPipe
		}
		request := httptest.NewRequestWithContext(ctx, http.MethodPost, "/", body)
		request.Header.Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeOctetStream().String())
		destination := &streamStepWriter{}
		if fault == streamFuzzShortWrite {
			destination.steps = []streamWriteStep{{}}
		}
		output := httptest.NewRecorder()
		got, gotErr := exchange.ReceiveStream(exchange.StreamReceiveCall{
			Call: socketServerCallFrom(t, output, request), Destination: destination, Buffer: make([]byte, int(window)%4097),
			Route:               exchange.RouteSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt},
			ExpectedContentType: core.HTTPMediaTypeOctetStream(),
		})

		wantBytes := len(data)
		wantRead := len(data)
		wantWrites := wantBytes
		var wantErr error
		wantCloseFailure := fault == streamFuzzCloseFailure
		wantOverflow := false
		wantReadFailure := fault == streamFuzzReadFailure
		switch {
		case fault == streamFuzzCancelOnRead:
			wantErr = context.Canceled
			wantBytes = min(len(data), 1)
			wantRead, wantWrites = wantBytes, wantBytes
			wantOverflow = false
		case fault == streamFuzzShortWrite && len(data) > 0:
			wantErr = io.ErrShortWrite
			wantBytes, wantRead, wantWrites = 0, 1, 1
			wantOverflow = false
		case wantReadFailure:
			wantErr = io.ErrUnexpectedEOF
		case wantCloseFailure:
			wantErr = io.ErrClosedPipe
		}
		if !errors.Is(gotErr, wantErr) || errors.Is(gotErr, core.ErrExchangeBodyLimit) != wantOverflow || errors.Is(gotErr, io.ErrUnexpectedEOF) != wantReadFailure || errors.Is(gotErr, io.ErrClosedPipe) != wantCloseFailure {
			t.Fatalf("stream refusal = %v, want primary %v, overflow/read/close (%t, %t, %t)", gotErr, wantErr, wantOverflow, wantReadFailure, wantCloseFailure)
		}
		if gotErr != nil && !errors.Is(gotErr, core.ErrExchangeContract) {
			t.Fatalf("stream refusal lost Exchange identity: %v", gotErr)
		}
		if got.Bytes.Uint64() != uint64(wantBytes) || !bytes.Equal(destination.body.Bytes(), data[:wantBytes]) || body.reads != wantRead || body.closes != 1 || destination.calls != wantWrites || !got.IdempotencyKey.IsZero() {
			t.Fatalf("stream bytes/destination/read bytes/closes/writes/key = (%d, %x, %d, %d, %d, %q), want (%d, %x, %d, 1, %d, absent)", got.Bytes.Uint64(), destination.body.Bytes(), body.reads, body.closes, destination.calls, got.IdempotencyKey.String(), wantBytes, data[:wantBytes], wantRead, wantWrites)
		}
		if output.Body.Len() != 0 || len(output.Header()) != 0 || output.Flushed {
			t.Fatalf("receive emitted response effects = (%q, %v, %t), want absent", output.Body.Bytes(), output.Header(), output.Flushed)
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("stream observation validation = %v, want nil", err)
		}
	})
}
