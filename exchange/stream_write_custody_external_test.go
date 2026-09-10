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

type streamWriteStep struct {
	accepted int
	err      error
	panics   bool
}

type streamStepWriter struct {
	steps []streamWriteStep
	body  bytes.Buffer
	calls int
}

func (w *streamStepWriter) Write(p []byte) (int, error) {
	index := w.calls
	w.calls++
	if index >= len(w.steps) {
		return w.body.Write(p)
	}
	step := w.steps[index]
	if step.accepted > 0 && step.accepted <= len(p) {
		_, _ = w.body.Write(p[:step.accepted])
	}
	if step.panics {
		panic(io.ErrClosedPipe)
	}
	return step.accepted, step.err
}

// A single byte per read makes earlier acknowledged Write calls observable
// independently of a later panicking call, without relying on scratch size.
type bytewiseStreamReader struct{ source *bytes.Reader }

func (r bytewiseStreamReader) Read(p []byte) (int, error) {
	if len(p) > 1 {
		p = p[:1]
	}
	return r.source.Read(p)
}

func TestReceiveStreamDestinationCustodyLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		payload      string
		steps        []streamWriteStep
		wantErr      error
		wantNative   error
		wantBody     string
		wantBytes    uint64
		wantReads    int
		wantWrites   int
		wantContract bool
	}{
		{name: "first write panic reports no invented acknowledgment", payload: "ab", steps: []streamWriteStep{{panics: true}}, wantErr: core.ErrExchangeRequest, wantContract: true, wantReads: 1, wantWrites: 1},
		{name: "second write panic retains the first acknowledged byte", payload: "ab", steps: []streamWriteStep{{accepted: 1}, {panics: true}}, wantErr: core.ErrExchangeRequest, wantContract: true, wantBody: "a", wantBytes: 1, wantReads: 2, wantWrites: 2},
		{name: "panic after an unacknowledged effect must not fabricate its count", payload: "ab", steps: []streamWriteStep{{accepted: 1}, {accepted: 1, panics: true}}, wantErr: core.ErrExchangeRequest, wantContract: true, wantBody: "ab", wantBytes: 1, wantReads: 2, wantWrites: 2},
		{name: "native failure beside a full count retains that acknowledged byte", payload: "ab", steps: []streamWriteStep{{accepted: 1, err: io.ErrClosedPipe}}, wantErr: core.ErrExchangeRequest, wantNative: io.ErrClosedPipe, wantContract: true, wantBody: "a", wantBytes: 1, wantReads: 1, wantWrites: 1},
		{name: "native failure after earlier progress cannot erase it", payload: "ab", steps: []streamWriteStep{{accepted: 1}, {err: io.ErrClosedPipe}}, wantErr: core.ErrExchangeRequest, wantNative: io.ErrClosedPipe, wantContract: true, wantBody: "a", wantBytes: 1, wantReads: 2, wantWrites: 2},
		{name: "short write without error must become the Go short-write refusal", payload: "ab", steps: []streamWriteStep{{accepted: 1}, {}}, wantErr: core.ErrExchangeRequest, wantNative: io.ErrShortWrite, wantContract: true, wantBody: "a", wantBytes: 1, wantReads: 2, wantWrites: 2},
		{name: "negative acknowledgment cannot subtract earlier bytes", payload: "ab", steps: []streamWriteStep{{accepted: 1}, {accepted: -1}}, wantErr: core.ErrExchangeRequest, wantContract: true, wantBody: "a", wantBytes: 1, wantReads: 2, wantWrites: 2},
		{name: "excess acknowledgment cannot manufacture an extra byte", payload: "ab", steps: []streamWriteStep{{accepted: 1}, {accepted: 2}}, wantErr: core.ErrExchangeRequest, wantContract: true, wantBody: "a", wantBytes: 1, wantReads: 2, wantWrites: 2},
		{name: "valid binary stream retains every acknowledged byte", payload: "a\x00z", wantBody: "a\x00z", wantBytes: 3, wantReads: 3, wantWrites: 3},
		{name: "empty source invokes no destination writes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := &bindingObservedBody{reader: bytewiseStreamReader{source: bytes.NewReader([]byte(tc.payload))}}
			request := httptest.NewRequest(http.MethodPost, "/", body)
			request.Header.Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeOctetStream().String())
			request.Header.Set(core.HTTPHeaderIdempotencyKey().String(), "op-A")
			destination := &streamStepWriter{steps: tc.steps}
			got, gotErr := exchange.ReceiveStream(exchange.StreamReceiveCall{
				Call: socketServerCall(t, request), Destination: destination,
				Route:               exchange.RouteSemantics{Method: exchange.MethodPost, Replay: exchange.ReplayIdempotencyKey},
				ExpectedContentType: core.HTTPMediaTypeOctetStream(),
			})
			if !errors.Is(gotErr, tc.wantErr) || errors.Is(gotErr, core.ErrExchangeContract) != tc.wantContract {
				t.Fatalf("receive error/contract = (%v, %t), want (%v, %t)", gotErr, errors.Is(gotErr, core.ErrExchangeContract), tc.wantErr, tc.wantContract)
			}
			for _, native := range []error{io.ErrClosedPipe, io.ErrShortWrite} {
				if errors.Is(gotErr, native) != errors.Is(tc.wantNative, native) {
					t.Fatalf("retained native %v = %t, want %t", native, errors.Is(gotErr, native), errors.Is(tc.wantNative, native))
				}
			}
			if destination.body.String() != tc.wantBody || got.Bytes.Uint64() != tc.wantBytes || destination.calls != tc.wantWrites || body.reads != tc.wantReads || body.closes != 1 || got.IdempotencyKey.String() != "op-A" {
				t.Fatalf("destination/acknowledged bytes/write calls/consumed bytes/closes/key = (%q, %d, %d, %d, %d, %q), want (%q, %d, %d, %d, 1, op-A)", destination.body.String(), got.Bytes.Uint64(), destination.calls, body.reads, body.closes, got.IdempotencyKey.String(), tc.wantBody, tc.wantBytes, tc.wantWrites, tc.wantReads)
			}
		})
	}
}
