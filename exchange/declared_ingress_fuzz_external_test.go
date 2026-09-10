package exchange_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func FuzzReceiveBoundedDeclaredExtentCustody(f *testing.F) {
	writer := httptest.NewRecorder()
	payload := []byte{0, 0xff, 'a'}
	response := exchange.ServerBoundedResponse{Body: payload, ContentType: core.HTTPMediaTypeOctetStream(), Status: core.HTTPStatusOK()}
	if err := response.Validate(); err != nil {
		f.Fatal(err)
	}
	if err := exchange.WriteBounded(exchange.BoundedWriteCall{Call: socketServerCallFrom(f, writer, httptest.NewRequest(http.MethodGet, "/", nil)), Response: response}); err != nil {
		f.Fatal(err)
	}
	seed := writer.Body.Bytes()
	for _, declared := range []int32{-2, -1, 0, int32(len(seed) - 1), int32(len(seed)), int32(len(seed) + 1)} {
		for _, limit := range []uint16{0, uint16(len(seed) - 1), uint16(len(seed)), uint16(len(seed) + 1)} {
			f.Add(seed, declared, limit, false, false)
		}
	}
	f.Add(seed, int32(len(seed)), uint16(9), true, false)
	f.Add(seed, int32(len(seed)-1), uint16(9), true, true)
	f.Add(seed, int32(len(seed)+1), uint16(9), false, true)
	f.Add([]byte{}, int32(1), uint16(9), true, true)
	f.Fuzz(func(t *testing.T, data []byte, declared int32, window uint16, readFailure, closeFailure bool) {
		if len(data) > 8192 {
			return
		}
		fault := streamFuzzIntact
		if readFailure {
			fault = streamFuzzReadFailure
		}
		source := &bindingObservedBody{reader: &streamFuzzReader{source: bytes.NewReader(data), fault: fault, window: int(window)}}
		if closeFailure {
			source.err = io.ErrClosedPipe
		}
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
		request.Body, request.ContentLength = source, int64(declared)
		request.Header.Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeOctetStream().String())
		writer := httptest.NewRecorder()
		got, err := exchange.ReceiveBounded(exchange.BoundedReceiveCall{Call: socketServerCallFrom(t, writer, request), Route: exchange.RouteSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt}, ExpectedContentType: core.HTTPMediaTypeOctetStream()})
		wantIngress := declared >= -1
		wantRead := 0
		if wantIngress {
			wantRead = len(data)
		}
		wantOverflow := false
		wantReadFailure := wantIngress && readFailure
		wantAccepted := wantIngress && !wantOverflow && !wantReadFailure && !closeFailure
		if wantAccepted {
			if err != nil || !bytes.Equal(got.Body, data) || !got.IdempotencyKey.IsZero() || got.Validate() != nil {
				t.Fatalf("declared receive=(%+v,%v), want exact accepted bytes", got, err)
			}
		} else if !errors.Is(err, core.ErrExchangeRequest) || got.Body != nil || !got.IdempotencyKey.IsZero() {
			t.Fatalf("declared refusal=(%+v,%v), want withheld body and request identity", got, err)
		}
		if errors.Is(err, core.ErrExchangeBodyLimit) != wantOverflow || errors.Is(err, io.ErrUnexpectedEOF) != wantReadFailure || errors.Is(err, io.ErrClosedPipe) != closeFailure {
			t.Fatalf("declared receive causes=%v, want extent/read/close=%t/%t/%t", err, wantOverflow, wantReadFailure, closeFailure)
		}
		if source.reads != wantRead || source.closes != 1 || writer.Body.Len() != 0 || len(writer.Header()) != 0 || writer.Flushed {
			t.Fatalf("declared receive custody=read %d,close %d,output %q,%v, want %d/1/no output", source.reads, source.closes, writer.Body.Bytes(), writer.Header(), wantRead)
		}
	})
}
