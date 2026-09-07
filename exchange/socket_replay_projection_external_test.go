package exchange_test

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type sequencedBoundDocument struct {
	Operation   string `json:"operation"`
	first       bindingCallbackFault
	laterKey    string
	validation  error
	projections *int
	marshals    *int
}

func (d sequencedBoundDocument) Validate() error {
	if d.validation != nil {
		return d.validation
	}
	_, err := exchange.ParseIdempotencyKey(d.Operation)
	return err
}

func (d sequencedBoundDocument) MarshalJSON() ([]byte, error) {
	if d.marshals != nil {
		*d.marshals++
	}
	type wire sequencedBoundDocument
	return json.Marshal(wire(d))
}

func (d sequencedBoundDocument) IdempotencyKey() (exchange.IdempotencyKey, error) {
	*d.projections++
	fault, operation := d.first, d.Operation
	if *d.projections > 1 {
		if d.laterKey != "" {
			operation = d.laterKey
		}
	}
	return (callbackBoundDocument{Operation: operation, Fault: fault}).IdempotencyKey()
}

func TestSocketReplayIdentityLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name            string
		plain           bool
		first           bindingCallbackFault
		laterKey        string
		validation      error
		cancelled       bool
		wantErr         error
		wantNative      error
		wantBinding     bool
		wantProjections int
		wantMarshals    int
		wantCalls       int
		wantKey         string
		wantAccepted    bool
	}{
		{name: "positive one key observation binds the exact body to transport", laterKey: "op-B", wantProjections: 1, wantMarshals: 1, wantCalls: 1, wantKey: "op-A", wantAccepted: true},
		{name: "neutral plain route has no binding and never invokes the key projection", plain: true, first: bindingCallbackPanic, wantMarshals: 1, wantCalls: 1, wantAccepted: true},
		{name: "first unset projection refuses before encoding", first: bindingCallbackZero, wantErr: core.ErrExchangeRequest, wantBinding: true, wantProjections: 1},
		{name: "matching value beside first failure cannot reach transport", first: bindingCallbackValueAndError, wantErr: core.ErrExchangeRequest, wantNative: io.ErrClosedPipe, wantBinding: true, wantProjections: 1},
		{name: "first callback panic remains contained", first: bindingCallbackPanic, wantErr: core.ErrExchangeRequest, wantBinding: true, wantProjections: 1},
		{name: "invalid body cannot execute its key callback", validation: io.ErrUnexpectedEOF, wantErr: core.ErrExchangeRequest, wantNative: io.ErrUnexpectedEOF},
		{name: "cancelled call cannot execute its key callback", cancelled: true, wantErr: core.ErrExchangeCancelled, wantNative: context.Canceled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancelled {
				cancel()
			}
			responseWire, err := (replayBoundResponse{Accepted: true}).MarshalJSON()
			if err != nil {
				t.Fatalf("response fixture encoding error = %v, want nil", err)
			}
			responseBody := &bindingObservedBody{reader: bytes.NewReader(responseWire)}
			var projections, marshals, calls int
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
				return &http.Response{StatusCode: http.StatusAccepted, Header: header, Body: responseBody, ContentLength: int64(len(responseWire)), Request: r}, nil
			})
			replay := exchange.ReplayIdempotencyKey
			if tc.plain {
				replay = exchange.ReplaySingleAttempt
			}
			socket, err := exchange.NewClientSocket(exchange.ClientSocketConfiguration{
				Target: mustEndpoint(t, "https://binding.example.test/socket"), Client: mustExchangeClient(t, &http.Client{Transport: transport}),
				Contract: socketPairContract(t, "/socket", replay), Operation: singleAttemptOperationPolicy(t),
			})
			if err != nil {
				t.Fatalf("socket fixture admission error = %v, want nil", err)
			}
			document := sequencedBoundDocument{Operation: "op-A", first: tc.first, laterKey: tc.laterKey, validation: tc.validation, projections: &projections, marshals: &marshals}
			var got exchange.JSONResponse[replayBoundResponse]
			var gotErr error
			if tc.plain {
				got, gotErr = exchange.SendSocketJSON[sequencedBoundDocument, replayBoundResponse](ctx, socket, document)
			} else {
				got, gotErr = exchange.SendReplayBoundSocketJSON[sequencedBoundDocument, replayBoundResponse](ctx, socket, document)
			}
			if !errors.Is(gotErr, tc.wantErr) || errors.Is(gotErr, core.ErrExchangeIdempotencyBinding) != tc.wantBinding {
				t.Fatalf("socket send error/binding = (%v, %t), want (%v, %t)", gotErr, errors.Is(gotErr, core.ErrExchangeIdempotencyBinding), tc.wantErr, tc.wantBinding)
			}
			for _, native := range []error{io.ErrClosedPipe, io.ErrUnexpectedEOF, context.Canceled} {
				if errors.Is(gotErr, native) != errors.Is(tc.wantNative, native) {
					t.Fatalf("retained native identity %v = %t, want %t", native, errors.Is(gotErr, native), errors.Is(tc.wantNative, native))
				}
			}
			if projections != tc.wantProjections || marshals != tc.wantMarshals || calls != tc.wantCalls || observedKey != tc.wantKey || got.Body.Accepted != tc.wantAccepted {
				t.Fatalf("projection/encoding/transport/key/accepted = (%d, %d, %d, %q, %t), want (%d, %d, %d, %q, %t)", projections, marshals, calls, observedKey, got.Body.Accepted, tc.wantProjections, tc.wantMarshals, tc.wantCalls, tc.wantKey, tc.wantAccepted)
			}
			if tc.wantAccepted {
				wantWire, err := json.Marshal(struct {
					Operation string `json:"operation"`
				}{Operation: "op-A"})
				if err != nil || observedReadErr != nil || !bytes.Equal(observedWire, wantWire) {
					t.Fatalf("request wire/read/fixture error = (%q, %v, %v), want (%q, nil, nil)", observedWire, observedReadErr, err, wantWire)
				}
				if got.Metadata.Status != mustHTTPStatus(t, http.StatusAccepted) || got.Metadata.Attempts != 1 || got.Metadata.Bytes.Uint64() != uint64(len(responseWire)) || got.Metadata.Headers.Values != nil || responseBody.closes != 1 || responseBody.reads != len(responseWire) {
					t.Fatalf("response metadata/close/read = (%+v, %d, %d), want one accepted attempt with exact response extent and custody", got.Metadata, responseBody.closes, responseBody.reads)
				}
			} else if got.Metadata.Status != (core.HTTPStatusCode{}) || got.Metadata.Attempts != 0 || got.Metadata.Bytes != (core.ByteLength{}) || got.Metadata.Headers.Values != nil || responseBody.closes != 0 || responseBody.reads != 0 || observedWire != nil {
				t.Fatalf("refused send response/wire = (%+v, %q), want exact zero with untouched response source", got, observedWire)
			}
		})
	}
}
