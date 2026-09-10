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

type receiveCustodyLane uint8

const (
	receiveCustodySocket receiveCustodyLane = iota
	receiveCustodyBoundSocket
	receiveCustodyJSON
	receiveCustodyProjectedJSON
)

func TestServerReceiveCustodyLayerTriad(t *testing.T) {
	t.Parallel()
	document := replayBoundDocument{Operation: "op-A"}
	wire, err := document.MarshalJSON()
	if err != nil {
		t.Fatalf("fixture encoding error = %v, want nil", err)
	}
	cases := []struct {
		name           string
		lane           receiveCustodyLane
		path           string
		query          string
		rawPath        string
		method         string
		replay         exchange.ReplayMode
		closeErr       error
		cancelled      bool
		unsetSocket    bool
		wantErr        error
		wantNative     error
		wantBody       bool
		wantKey        string
		wantReads      int
		wantCloses     int
		wantProjection int
	}{
		{name: "positive plain socket retains exact decoded bytes", lane: receiveCustodySocket, path: "/socket", method: http.MethodPost, replay: exchange.ReplaySingleAttempt, wantBody: true, wantReads: len(wire), wantCloses: 1},
		{name: "positive bound socket retains exact body and header identity", lane: receiveCustodyBoundSocket, path: "/socket", method: http.MethodPost, replay: exchange.ReplayIdempotencyKey, wantBody: true, wantKey: "op-A", wantReads: len(wire), wantCloses: 1},
		{name: "negative plain socket wrong route closes unread input", lane: receiveCustodySocket, path: "/other", method: http.MethodPost, replay: exchange.ReplaySingleAttempt, wantErr: core.ErrExchangeContract, wantCloses: 1},
		{name: "negative bound socket wrong route closes unread input", lane: receiveCustodyBoundSocket, path: "/other", method: http.MethodPost, replay: exchange.ReplayIdempotencyKey, wantErr: core.ErrExchangeContract, wantCloses: 1},
		{name: "negative query cannot decorate the exact plain route", lane: receiveCustodySocket, path: "/socket", query: "x=1", method: http.MethodPost, replay: exchange.ReplaySingleAttempt, wantErr: core.ErrExchangeContract, wantCloses: 1},
		{name: "negative escaped path cannot bypass the exact bound route", lane: receiveCustodyBoundSocket, path: "/socket", rawPath: "/%73ocket", method: http.MethodPost, replay: exchange.ReplayIdempotencyKey, wantErr: core.ErrExchangeContract, wantCloses: 1},
		{name: "negative plain lane cannot consume a key-bound route", lane: receiveCustodySocket, path: "/socket", method: http.MethodPost, replay: exchange.ReplayIdempotencyKey, wantErr: core.ErrExchangeContract, wantCloses: 1},
		{name: "negative bound lane cannot consume a plain route", lane: receiveCustodyBoundSocket, path: "/socket", method: http.MethodPost, replay: exchange.ReplaySingleAttempt, wantErr: core.ErrExchangeContract, wantCloses: 1},
		{name: "negative wrong method closes before JSON read", lane: receiveCustodySocket, path: "/socket", method: http.MethodPut, replay: exchange.ReplaySingleAttempt, wantErr: core.ErrExchangeContract, wantCloses: 1},
		{name: "negative unset plain socket cannot retain request custody", lane: receiveCustodySocket, path: "/socket", method: http.MethodPost, replay: exchange.ReplaySingleAttempt, unsetSocket: true, wantErr: core.ErrExchangeContract, wantCloses: 1},
		{name: "negative unset bound socket cannot retain request custody", lane: receiveCustodyBoundSocket, path: "/socket", method: http.MethodPost, replay: exchange.ReplayIdempotencyKey, unsetSocket: true, wantErr: core.ErrExchangeContract, wantCloses: 1},
		{name: "negative route refusal retains a simultaneous close failure", lane: receiveCustodySocket, path: "/other", method: http.MethodPost, replay: exchange.ReplaySingleAttempt, closeErr: io.ErrClosedPipe, wantErr: core.ErrExchangeContract, wantNative: io.ErrClosedPipe, wantCloses: 1},
		{name: "negative plain socket close failure clears decoded body", lane: receiveCustodySocket, path: "/socket", method: http.MethodPost, replay: exchange.ReplaySingleAttempt, closeErr: io.ErrClosedPipe, wantErr: core.ErrExchangeRequest, wantNative: io.ErrClosedPipe, wantReads: len(wire), wantCloses: 1},
		{name: "negative bound socket close failure clears body and key", lane: receiveCustodyBoundSocket, path: "/socket", method: http.MethodPost, replay: exchange.ReplayIdempotencyKey, closeErr: io.ErrClosedPipe, wantErr: core.ErrExchangeRequest, wantNative: io.ErrClosedPipe, wantReads: len(wire), wantCloses: 1},
		{name: "negative JSON close failure clears body and key", lane: receiveCustodyJSON, path: "/socket", method: http.MethodPost, replay: exchange.ReplayIdempotencyKey, closeErr: io.ErrClosedPipe, wantErr: core.ErrExchangeRequest, wantNative: io.ErrClosedPipe, wantReads: len(wire), wantCloses: 1},
		{name: "negative projected JSON close failure clears completed body and key", lane: receiveCustodyProjectedJSON, path: "/socket", method: http.MethodPost, replay: exchange.ReplayIdempotencyKey, closeErr: io.ErrClosedPipe, wantErr: core.ErrExchangeRequest, wantNative: io.ErrClosedPipe, wantReads: len(wire), wantCloses: 1, wantProjection: 1},
		{name: "positive projection keeps request context and body", lane: receiveCustodyProjectedJSON, path: "/socket", method: http.MethodPost, replay: exchange.ReplayIdempotencyKey, wantBody: true, wantKey: "op-A", wantReads: len(wire), wantCloses: 1, wantProjection: 1},
		{name: "neutral cancelled plain ingress reads no input", lane: receiveCustodySocket, path: "/socket", method: http.MethodPost, replay: exchange.ReplaySingleAttempt, cancelled: true, wantErr: core.ErrExchangeCancelled, wantNative: context.Canceled, wantCloses: 1},
		{name: "neutral cancelled bound ingress reads no input", lane: receiveCustodyBoundSocket, path: "/socket", method: http.MethodPost, replay: exchange.ReplayIdempotencyKey, cancelled: true, wantErr: core.ErrExchangeCancelled, wantNative: context.Canceled, wantCloses: 1},
		{name: "neutral cancelled projected ingress invokes no projector", lane: receiveCustodyProjectedJSON, path: "/socket", method: http.MethodPost, replay: exchange.ReplayIdempotencyKey, cancelled: true, wantErr: core.ErrExchangeCancelled, wantNative: context.Canceled, wantCloses: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancelled {
				cancel()
			}
			body := &bindingObservedBody{reader: bytes.NewReader(wire), err: tc.closeErr}
			request := httptest.NewRequestWithContext(ctx, tc.method, tc.path, body)
			request.URL.RawQuery, request.URL.RawPath = tc.query, tc.rawPath
			request.Header.Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
			if tc.replay == exchange.ReplayIdempotencyKey {
				request.Header.Set(core.HTTPHeaderIdempotencyKey().String(), document.Operation)
			}
			contract := socketPairContract(t, "/socket", tc.replay)
			socket, err := exchange.NewServerSocket(contract)
			if err != nil {
				t.Fatalf("NewServerSocket() setup error = %v, want nil", err)
			}
			if tc.unsetSocket {
				socket = exchange.ServerSocket{}
			}
			writer := httptest.NewRecorder()
			call, err := exchange.NewSocketServerCall(writer, request)
			if err != nil {
				t.Fatalf("NewSocketServerCall() setup error = %v, want nil", err)
			}
			var got exchange.Received[*replayBoundDocument]
			var gotErr error
			var projections int
			var projectedContext context.Context
			switch tc.lane {
			case receiveCustodySocket:
				got, gotErr = exchange.ReceiveSocketJSON[replayBoundDocument, *replayBoundDocument](socket, call)
			case receiveCustodyBoundSocket:
				got, gotErr = exchange.ReceiveReplayBoundSocketJSON[replayBoundDocument, *replayBoundDocument](socket, call)
			case receiveCustodyJSON:
				got, gotErr = exchange.ReceiveJSON[replayBoundDocument, *replayBoundDocument](exchange.JSONReceiveCall{Call: call, Route: contract.Route})
			case receiveCustodyProjectedJSON:
				got, gotErr = exchange.ReceiveProjectedJSON[replayBoundDocument, *replayBoundDocument](exchange.ProjectedJSONReceiveCall[replayBoundDocument, *replayBoundDocument]{
					Call: call, Route: contract.Route,
					Project: func(ctx context.Context, _ exchange.SocketServerCall, _ *replayBoundDocument) error {
						projections++
						projectedContext = ctx
						return nil
					},
				})
			default:
				t.Fatalf("fixture receive lane = %v, want a declared lane", tc.lane)
			}
			if !errors.Is(gotErr, tc.wantErr) || tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) {
				t.Fatalf("receive error = %v, want identities (%v, %v)", gotErr, tc.wantErr, tc.wantNative)
			}
			if body.reads != tc.wantReads || body.closes != tc.wantCloses || projections != tc.wantProjection {
				t.Fatalf("read bytes/closes/projections = (%d, %d, %d), want (%d, %d, %d)", body.reads, body.closes, projections, tc.wantReads, tc.wantCloses, tc.wantProjection)
			}
			if tc.wantProjection > 0 && projectedContext != ctx {
				t.Fatalf("projected context = %v, want exact request context %v", projectedContext, ctx)
			}
			if (got.Body != nil) != tc.wantBody || got.IdempotencyKey.String() != tc.wantKey {
				t.Fatalf("received body/key = (%+v, %q), want (present %t, %q)", got.Body, got.IdempotencyKey.String(), tc.wantBody, tc.wantKey)
			}
			if tc.wantBody && *got.Body != document {
				t.Fatalf("received document = %+v, want %+v", *got.Body, document)
			}
			if writer.Body.Len() != 0 || len(writer.Header()) != 0 || writer.Flushed {
				t.Fatalf("ingress response effects = (%q, %v, %t), want absent", writer.Body.Bytes(), writer.Header(), writer.Flushed)
			}
		})
	}
}

func TestNoBodyReceiveCustodyLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		method     string
		replay     exchange.ReplayMode
		key        string
		closeErr   error
		wantErr    error
		wantNative error
		wantKey    string
		wantCloses int
	}{
		{name: "neutral body absence preserves absent identity", method: http.MethodPost, replay: exchange.ReplaySingleAttempt, wantCloses: 1},
		{name: "positive keyed body absence preserves exact identity", method: http.MethodPost, replay: exchange.ReplayIdempotencyKey, key: "op-A", wantKey: "op-A", wantCloses: 1},
		{name: "negative close failure clears observed identity", method: http.MethodPost, replay: exchange.ReplayIdempotencyKey, key: "op-A", closeErr: io.ErrClosedPipe, wantErr: core.ErrExchangeRequest, wantNative: io.ErrClosedPipe, wantCloses: 1},
		{name: "negative close failure remains visible without an identity", method: http.MethodPost, replay: exchange.ReplaySingleAttempt, closeErr: io.ErrClosedPipe, wantErr: core.ErrExchangeRequest, wantNative: io.ErrClosedPipe, wantCloses: 1},
		{name: "negative method refusal still ends body custody", method: http.MethodPut, replay: exchange.ReplayIdempotencyKey, key: "op-A", wantErr: core.ErrExchangeRequest, wantCloses: 1},
		{name: "negative method refusal cannot hide close failure", method: http.MethodPut, replay: exchange.ReplayIdempotencyKey, key: "op-A", closeErr: io.ErrClosedPipe, wantErr: core.ErrExchangeRequest, wantNative: io.ErrClosedPipe, wantCloses: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := &bindingObservedBody{reader: bytes.NewReader(nil), err: tc.closeErr}
			request := httptest.NewRequest(tc.method, "/", nil)
			request.Body = body
			if tc.key != "" {
				request.Header.Set(core.HTTPHeaderIdempotencyKey().String(), tc.key)
			}
			call, err := exchange.NewSocketServerCall(httptest.NewRecorder(), request)
			if err != nil {
				t.Fatalf("NewSocketServerCall() setup error = %v, want nil", err)
			}
			got, gotErr := exchange.ReceiveNoBody(exchange.NoBodyReceiveCall{Call: call, Route: exchange.RouteSemantics{Method: exchange.MethodPost, Replay: tc.replay}})
			if !errors.Is(gotErr, tc.wantErr) || tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) {
				t.Fatalf("ReceiveNoBody() error = %v, want identities (%v, %v)", gotErr, tc.wantErr, tc.wantNative)
			}
			if got.IdempotencyKey.String() != tc.wantKey || body.closes != tc.wantCloses || body.reads != 0 {
				t.Fatalf("received key/closes/read bytes = (%q, %d, %d), want (%q, %d, 0)", got.IdempotencyKey.String(), body.closes, body.reads, tc.wantKey, tc.wantCloses)
			}
		})
	}
}

type custodyTerminalReader struct {
	prefix           *bytes.Reader
	panicAfterPrefix bool
	terminal         error
}

func (r *custodyTerminalReader) Read(p []byte) (int, error) {
	if r.prefix.Len() != 0 {
		return r.prefix.Read(p)
	}
	if r.panicAfterPrefix {
		panic(io.ErrUnexpectedEOF)
	}
	return 0, r.terminal
}

func TestAggregateAndStreamFailureCustodyLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name             string
		stream           bool
		payload          string
		limit            uint64
		closeErr         error
		terminal         error
		panicAfterPrefix bool
		wantErr          error
		wantNative       error
		wantAggregate    string
		wantDestination  string
		wantKey          string
		wantBytes        uint64
		wantReadBytes    int
		wantCloses       int
	}{
		{name: "positive aggregate publishes exact owned bytes after close", payload: "abc", limit: 3, terminal: io.EOF, wantAggregate: "abc", wantKey: "op-A", wantReadBytes: 3, wantCloses: 1},
		{name: "positive stream reports exact destination effect", stream: true, payload: "abc", limit: 3, terminal: io.EOF, wantDestination: "abc", wantKey: "op-A", wantBytes: 3, wantReadBytes: 3, wantCloses: 1},
		{name: "negative aggregate close failure cannot publish bytes or key", payload: "abc", limit: 3, terminal: io.EOF, closeErr: io.ErrClosedPipe, wantErr: core.ErrExchangeRequest, wantNative: io.ErrClosedPipe, wantReadBytes: 3, wantCloses: 1},
		{name: "negative stream close failure must retain the actual write", stream: true, payload: "abc", limit: 3, terminal: io.EOF, closeErr: io.ErrClosedPipe, wantErr: core.ErrExchangeRequest, wantNative: io.ErrClosedPipe, wantDestination: "abc", wantKey: "op-A", wantBytes: 3, wantReadBytes: 3, wantCloses: 1},
		{name: "whole value retains the complete source", payload: "abc", limit: 2, terminal: io.EOF, wantAggregate: "abc", wantKey: "op-A", wantReadBytes: 3, wantCloses: 1},
		{name: "stream continues beyond former aggregate budget", stream: true, payload: "abc", limit: 2, terminal: io.EOF, wantDestination: "abc", wantKey: "op-A", wantBytes: 3, wantReadBytes: 3, wantCloses: 1},
		{name: "negative aggregate native read failure withholds partial document", payload: "abc", limit: 4, terminal: io.ErrUnexpectedEOF, wantErr: core.ErrExchangeRequest, wantNative: io.ErrUnexpectedEOF, wantReadBytes: 3, wantCloses: 1},
		{name: "negative stream native read failure retains exact earlier writes", stream: true, payload: "abc", limit: 4, terminal: io.ErrUnexpectedEOF, wantErr: core.ErrExchangeRequest, wantNative: io.ErrUnexpectedEOF, wantDestination: "abc", wantKey: "op-A", wantBytes: 3, wantReadBytes: 3, wantCloses: 1},
		{name: "negative aggregate reader panic withholds private bytes", payload: "abc", limit: 4, panicAfterPrefix: true, wantErr: core.ErrExchangeRequest, wantReadBytes: 3, wantCloses: 1},
		{name: "negative stream reader panic cannot erase earlier writes", stream: true, payload: "abc", limit: 4, panicAfterPrefix: true, wantErr: core.ErrExchangeRequest, wantDestination: "abc", wantKey: "op-A", wantBytes: 3, wantReadBytes: 3, wantCloses: 1},
		{name: "neutral empty aggregate retains only the real request key", limit: 1, terminal: io.EOF, wantKey: "op-A", wantCloses: 1},
		{name: "neutral empty stream reports no destination effect", stream: true, limit: 1, terminal: io.EOF, wantKey: "op-A", wantCloses: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := &bindingObservedBody{reader: &custodyTerminalReader{prefix: bytes.NewReader([]byte(tc.payload)), terminal: tc.terminal, panicAfterPrefix: tc.panicAfterPrefix}, err: tc.closeErr}
			request := httptest.NewRequest(http.MethodPost, "/", body)
			request.Header.Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeOctetStream().String())
			request.Header.Set(core.HTTPHeaderIdempotencyKey().String(), "op-A")
			call, err := exchange.NewSocketServerCall(httptest.NewRecorder(), request)
			if err != nil {
				t.Fatalf("NewSocketServerCall() setup error = %v, want nil", err)
			}
			route := exchange.RouteSemantics{Method: exchange.MethodPost, Replay: exchange.ReplayIdempotencyKey}
			var destination bytes.Buffer
			var aggregate exchange.ReceivedBytes
			var stream exchange.ReceivedStream
			var gotErr error
			var gotKey exchange.IdempotencyKey
			if tc.stream {
				stream, gotErr = exchange.ReceiveStream(exchange.StreamReceiveCall{Call: call, Route: route, Destination: &destination, ExpectedContentType: core.HTTPMediaTypeOctetStream()})
				gotKey = stream.IdempotencyKey
			} else {
				aggregate, gotErr = exchange.ReceiveBounded(exchange.BoundedReceiveCall{Call: call, Route: route, ExpectedContentType: core.HTTPMediaTypeOctetStream()})
				gotKey = aggregate.IdempotencyKey
			}
			if !errors.Is(gotErr, tc.wantErr) || tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) {
				t.Fatalf("receive error = %v, want identities (%v, %v)", gotErr, tc.wantErr, tc.wantNative)
			}
			if string(aggregate.Body) != tc.wantAggregate || destination.String() != tc.wantDestination || gotKey.String() != tc.wantKey || stream.Bytes.Uint64() != tc.wantBytes || body.reads != tc.wantReadBytes || body.closes != tc.wantCloses {
				t.Fatalf("aggregate/destination/key/reported bytes/read bytes/closes = (%q, %q, %q, %d, %d, %d), want (%q, %q, %q, %d, %d, %d)", aggregate.Body, destination.String(), gotKey.String(), stream.Bytes.Uint64(), body.reads, body.closes, tc.wantAggregate, tc.wantDestination, tc.wantKey, tc.wantBytes, tc.wantReadBytes, tc.wantCloses)
			}
			if tc.wantErr != nil && !tc.stream && aggregate.Body != nil {
				t.Fatalf("refused aggregate storage = %q, want nil", aggregate.Body)
			}
		})
	}
}
