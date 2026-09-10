package exchange_test

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type bindingCallbackFault uint8

const (
	bindingCallbackIntact bindingCallbackFault = iota
	bindingCallbackError
	bindingCallbackValueAndError
	bindingCallbackZero
	bindingCallbackPanic
	bindingValidationError
	bindingValidationPanic
)

// Fault is test-owned input that crosses the real decoder. It avoids a global
// callback switch: each parallel row owns the behavior of its decoded value.
type callbackBoundDocument struct {
	Operation string               `json:"operation"`
	Fault     bindingCallbackFault `json:"fault"`
	marshals  *int
}

func (d callbackBoundDocument) Validate() error {
	switch d.Fault {
	case bindingValidationError:
		return io.ErrUnexpectedEOF
	case bindingValidationPanic:
		panic(io.ErrUnexpectedEOF)
	}
	if d.Fault > bindingValidationPanic {
		return core.ErrExchangeContract
	}
	_, err := exchange.ParseIdempotencyKey(d.Operation)
	return err
}

func (d callbackBoundDocument) MarshalJSON() ([]byte, error) {
	if d.marshals != nil {
		*d.marshals++
	}
	type wire callbackBoundDocument
	return json.Marshal(wire(d))
}

func (d callbackBoundDocument) IdempotencyKey() (exchange.IdempotencyKey, error) {
	key, err := exchange.ParseIdempotencyKey(d.Operation)
	if err != nil {
		return exchange.IdempotencyKey{}, err
	}
	switch d.Fault {
	case bindingCallbackError:
		return exchange.IdempotencyKey{}, io.ErrClosedPipe
	case bindingCallbackValueAndError:
		return key, io.ErrClosedPipe
	case bindingCallbackZero:
		return exchange.IdempotencyKey{}, nil
	case bindingCallbackPanic:
		panic(io.ErrClosedPipe)
	}
	return key, nil
}

type bindingObservedBody struct {
	reader io.Reader
	err    error
	reads  int
	closes int
}

func (b *bindingObservedBody) Read(p []byte) (int, error) {
	n, err := b.reader.Read(p)
	b.reads += n
	return n, err
}

func (b *bindingObservedBody) Close() error { b.closes++; return b.err }

type bindingTransport func(*http.Request) (*http.Response, error)

func (f bindingTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestReceiveReplayBindingCallbackLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		fault       bindingCallbackFault
		replay      exchange.ReplayMode
		header      string
		closeErr    error
		wantErr     error
		wantNative  error
		wantBinding bool
		wantBody    bool
		wantKey     string
		wantCloses  int
	}{
		{name: "positive exact identity survives decoding and projection", replay: exchange.ReplayIdempotencyKey, header: "op-A", wantBody: true, wantKey: "op-A", wantCloses: 1},
		{name: "positive keyed single attempt retains the same binding", replay: exchange.ReplaySingleAttemptWithIdempotencyKey, header: "op-A", wantBody: true, wantKey: "op-A", wantCloses: 1},
		{name: "negative callback error retains its native cause", fault: bindingCallbackError, replay: exchange.ReplayIdempotencyKey, header: "op-A", wantErr: core.ErrExchangeRequest, wantNative: io.ErrClosedPipe, wantBinding: true, wantCloses: 1},
		{name: "negative matching key beside callback error cannot authorize", fault: bindingCallbackValueAndError, replay: exchange.ReplayIdempotencyKey, header: "op-A", wantErr: core.ErrExchangeRequest, wantNative: io.ErrClosedPipe, wantBinding: true, wantCloses: 1},
		{name: "negative unset projected key cannot erase a real key", fault: bindingCallbackZero, replay: exchange.ReplayIdempotencyKey, header: "op-A", wantErr: core.ErrExchangeRequest, wantBinding: true, wantCloses: 1},
		{name: "negative callback panic stays inside the boundary", fault: bindingCallbackPanic, replay: exchange.ReplayIdempotencyKey, header: "op-A", wantErr: core.ErrExchangeRequest, wantBinding: true, wantCloses: 1},
		{name: "negative body validation error precedes projection", fault: bindingValidationError, replay: exchange.ReplayIdempotencyKey, header: "op-A", wantErr: core.ErrExchangeRequest, wantNative: io.ErrUnexpectedEOF, wantCloses: 1},
		{name: "negative body validation panic stays inside the decoder", fault: bindingValidationPanic, replay: exchange.ReplayIdempotencyKey, header: "op-A", wantErr: core.ErrExchangeRequest, wantCloses: 1},
		{name: "negative close failure cannot publish an admitted document", replay: exchange.ReplayIdempotencyKey, header: "op-A", closeErr: io.ErrClosedPipe, wantErr: core.ErrExchangeRequest, wantNative: io.ErrClosedPipe, wantCloses: 1},
		{name: "negative decode refusal and close refusal both survive", fault: bindingValidationError, replay: exchange.ReplayIdempotencyKey, header: "op-A", closeErr: io.ErrClosedPipe, wantErr: core.ErrExchangeRequest, wantNative: errors.Join(io.ErrUnexpectedEOF, io.ErrClosedPipe), wantCloses: 1},
		{name: "neutral absent header and absent projection cannot invent a binding", fault: bindingCallbackZero, replay: exchange.ReplaySingleAttempt, wantErr: core.ErrExchangeRequest, wantBinding: true, wantCloses: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Go encodes a typed hostile fixture; fault rows deliberately bypass
			// Validate so the actual ingress must reject the injected behavior.
			document := callbackBoundDocument{Operation: "op-A", Fault: tc.fault}
			wire, err := document.MarshalJSON()
			if err != nil {
				t.Fatalf("fixture encoding error = %v, want nil", err)
			}
			body := &bindingObservedBody{reader: bytes.NewReader(wire), err: tc.closeErr}
			request := httptest.NewRequest(http.MethodPost, "/", body)
			request.Header.Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
			if tc.header != "" {
				request.Header.Set(core.HTTPHeaderIdempotencyKey().String(), tc.header)
			}
			call, err := exchange.NewSocketServerCall(httptest.NewRecorder(), request)
			if err != nil {
				t.Fatalf("NewSocketServerCall() setup error = %v, want nil", err)
			}
			var got exchange.Received[*callbackBoundDocument]
			var gotErr error
			var gotPanic any
			func() {
				defer func() { gotPanic = recover() }()
				got, gotErr = exchange.ReceiveReplayBoundJSON[callbackBoundDocument, *callbackBoundDocument](exchange.JSONReceiveCall{
					Call: call, Route: exchange.RouteSemantics{Method: exchange.MethodPost, Replay: tc.replay},
				})
			}()
			if gotPanic != nil || !errors.Is(gotErr, tc.wantErr) || errors.Is(gotErr, core.ErrExchangeIdempotencyBinding) != tc.wantBinding {
				t.Fatalf("ReceiveReplayBoundJSON() panic/error/binding = (%v, %v, %t), want (nil, %v, %t)", gotPanic, gotErr, errors.Is(gotErr, core.ErrExchangeIdempotencyBinding), tc.wantErr, tc.wantBinding)
			}
			if tc.wantNative != nil {
				// The join case has two independent causes, neither may be lost.
				for _, native := range []error{io.ErrUnexpectedEOF, io.ErrClosedPipe} {
					if errors.Is(gotErr, native) != errors.Is(tc.wantNative, native) {
						t.Fatalf("native %v retained = %t, want %t", native, errors.Is(gotErr, native), errors.Is(tc.wantNative, native))
					}
				}
			}
			if (got.Body != nil) != tc.wantBody || got.IdempotencyKey.String() != tc.wantKey || body.closes != tc.wantCloses {
				t.Fatalf("returned body/key/closes = (%v, %q, %d), want (present %t, %q, %d)", got.Body, got.IdempotencyKey.String(), body.closes, tc.wantBody, tc.wantKey, tc.wantCloses)
			}
			if tc.wantBody && (got.Body.Operation != document.Operation || got.Body.Fault != document.Fault) {
				t.Fatalf("received body = %+v, want %+v", got.Body, document)
			}
		})
	}
}

func TestSendReplayBindingCallbackLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		fault       bindingCallbackFault
		replay      exchange.ReplayMode
		header      string
		wantErr     error
		wantNative  error
		wantBinding bool
		wantCalls   int
		wantBody    bool
	}{
		{name: "positive exact binding reaches the standard transport", replay: exchange.ReplayIdempotencyKey, header: "op-A", wantCalls: 1, wantBody: true},
		{name: "positive keyed single attempt remains bound", replay: exchange.ReplaySingleAttemptWithIdempotencyKey, header: "op-A", wantCalls: 1, wantBody: true},
		{name: "negative callback error spends no encoding or attempt", fault: bindingCallbackError, replay: exchange.ReplayIdempotencyKey, header: "op-A", wantErr: core.ErrExchangeRequest, wantNative: io.ErrClosedPipe, wantBinding: true},
		{name: "negative matching callback value cannot hide its error", fault: bindingCallbackValueAndError, replay: exchange.ReplayIdempotencyKey, header: "op-A", wantErr: core.ErrExchangeRequest, wantNative: io.ErrClosedPipe, wantBinding: true},
		{name: "negative zero callback key cannot erase the configured key", fault: bindingCallbackZero, replay: exchange.ReplayIdempotencyKey, header: "op-A", wantErr: core.ErrExchangeRequest, wantBinding: true},
		{name: "negative callback panic spends no encoding or attempt", fault: bindingCallbackPanic, replay: exchange.ReplayIdempotencyKey, header: "op-A", wantErr: core.ErrExchangeRequest, wantBinding: true},
		{name: "negative failed validation precedes the key callback", fault: bindingValidationError, replay: exchange.ReplayIdempotencyKey, header: "op-A", wantErr: core.ErrExchangeRequest, wantNative: io.ErrUnexpectedEOF},
		{name: "negative validation panic spends no encoding or attempt", fault: bindingValidationPanic, replay: exchange.ReplayIdempotencyKey, header: "op-A", wantErr: core.ErrExchangeRequest},
		{name: "negative valid foreign identity cannot reach transport", replay: exchange.ReplayIdempotencyKey, header: "op-B", wantErr: core.ErrExchangeRequest, wantBinding: true},
		{name: "neutral two absent keys cannot authorize the bound lane", fault: bindingCallbackZero, replay: exchange.ReplaySingleAttempt, wantErr: core.ErrExchangeRequest, wantBinding: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			responseWire, err := (replayBoundResponse{Accepted: true}).MarshalJSON()
			if err != nil {
				t.Fatalf("response fixture encoding error = %v, want nil", err)
			}
			responseBody := &bindingObservedBody{reader: bytes.NewReader(responseWire)}
			var calls, marshals int
			var observedKey string
			var observedWire []byte
			var observedReadErr error
			transport := bindingTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				observedKey = r.Header.Get(core.HTTPHeaderIdempotencyKey().String())
				observedWire, observedReadErr = io.ReadAll(r.Body)
				observedReadErr = errors.Join(observedReadErr, r.Body.Close())
				header := make(http.Header)
				header.Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
				return &http.Response{StatusCode: http.StatusOK, Header: header, Body: responseBody, ContentLength: int64(len(responseWire)), Request: r}, nil
			})
			var key exchange.IdempotencyKey
			if tc.header != "" {
				key, err = exchange.ParseIdempotencyKey(tc.header)
				if err != nil {
					t.Fatalf("header fixture admission error = %v, want nil", err)
				}
			}
			document := callbackBoundDocument{Operation: "op-A", Fault: tc.fault, marshals: &marshals}
			var got exchange.JSONResponse[replayBoundResponse]
			var gotErr error
			var gotPanic any
			func() {
				defer func() { gotPanic = recover() }()
				got, gotErr = exchange.SendReplayBoundJSON[callbackBoundDocument, replayBoundResponse](exchange.JSONCall[callbackBoundDocument]{
					Context: t.Context(), Client: mustExchangeClient(t, &http.Client{Transport: transport}),
					Request: exchange.JSONRequest[callbackBoundDocument]{
						Target: mustEndpoint(t, "https://binding.example.test/"), Body: document,
						Semantics: exchange.RequestSemantics{Method: exchange.MethodPost, Replay: tc.replay, IdempotencyKey: key}, ExpectedStatus: core.HTTPStatusOK(),
					},
					Policy: exchange.JSONPolicy{Operation: singleAttemptOperationPolicy(t)},
				})
			}()
			if gotPanic != nil || !errors.Is(gotErr, tc.wantErr) || errors.Is(gotErr, core.ErrExchangeIdempotencyBinding) != tc.wantBinding {
				t.Fatalf("SendReplayBoundJSON() panic/error/binding = (%v, %v, %t), want (nil, %v, %t)", gotPanic, gotErr, errors.Is(gotErr, core.ErrExchangeIdempotencyBinding), tc.wantErr, tc.wantBinding)
			}
			if tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) {
				t.Fatalf("native cause = %v, want %v", gotErr, tc.wantNative)
			}
			if calls != tc.wantCalls || marshals != tc.wantCalls || responseBody.closes != tc.wantCalls || got.Body.Accepted != tc.wantBody {
				t.Fatalf("transport/encoding/close/body = (%d, %d, %d, %t), want (%d, %d, %d, %t)", calls, marshals, responseBody.closes, got.Body.Accepted, tc.wantCalls, tc.wantCalls, tc.wantCalls, tc.wantBody)
			}
			if !tc.wantBody {
				if got.Metadata.Attempts != 0 || got.Metadata.Status != (core.HTTPStatusCode{}) || got.Metadata.Bytes != (core.ByteLength{}) || got.Metadata.Headers.Values != nil || observedWire != nil || observedKey != "" {
					t.Fatalf("refused response/wire/key = (%+v, %q, %q), want exact zero", got, observedWire, observedKey)
				}
				return
			}
			document.marshals = nil
			wantWire, err := document.MarshalJSON()
			if err != nil {
				t.Fatalf("wanted typed wire encoding error = %v, want nil", err)
			}
			if observedReadErr != nil || observedKey != tc.header || !bytes.Equal(observedWire, wantWire) {
				t.Fatalf("transport read/key/wire = (%v, %q, %q), want (nil, %q, %q)", observedReadErr, observedKey, observedWire, tc.header, wantWire)
			}
			if got.Metadata.Attempts != 1 || got.Metadata.Status != core.HTTPStatusOK() || got.Metadata.Bytes.Uint64() != uint64(len(responseWire)) || got.Metadata.Headers.Values != nil || responseBody.reads != len(responseWire) {
				t.Fatalf("response metadata/read bytes = (%+v, %d), want one OK attempt, %d bytes, absent capture", got.Metadata, responseBody.reads, len(responseWire))
			}
		})
	}
}
