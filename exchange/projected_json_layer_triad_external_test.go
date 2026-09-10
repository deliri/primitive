package exchange_test

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type projectionDisposition uint8

const (
	projectionComplete projectionDisposition = iota
	projectionMissing
	projectionUnchanged
	projectionRefused
	projectionPanicked
)

func TestProjectedJSONReceiveLayerTriad(t *testing.T) {
	t.Parallel()
	document := transportDocument{Message: "candidate"}
	wire, err := document.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	unknownWire := []byte(`{"message":"candidate","unknown":true}`)
	duplicateWire := []byte(`{"message":"a","message":"b"}`)
	nonwireMember := []byte(`{"message":"a","Method":"POST"}`)
	wrongType := []byte(`{"message":1}`)
	truncatedWire := []byte(`{"message":`)
	cases := []struct {
		name        string
		wire        []byte
		method      string
		disposition projectionDisposition
		closeErr    error
		wantErr     error
		wantCause   error
		wantRead    int
		wantProject int
		wantMessage string
	}{
		{name: "exact extent completes nonwire method and preserves source message", wire: wire, wantRead: len(wire), wantProject: 1, wantMessage: document.Message},
		{name: "missing projector closes unread body", wire: wire, disposition: projectionMissing, wantErr: core.ErrExchangeContract},
		{name: "wrong method closes before decode or projection", wire: wire, method: http.MethodPut, wantErr: core.ErrExchangeContract},
		{name: "no-op projector cannot publish an invalid nonwire method", wire: wire, disposition: projectionUnchanged, wantRead: len(wire), wantProject: 1, wantErr: core.ErrExchangeContract},
		{name: "callback refusal withholds a partly completed value", wire: wire, disposition: projectionRefused, wantRead: len(wire), wantProject: 1, wantErr: core.ErrExchangeRequest, wantCause: io.ErrUnexpectedEOF},
		{name: "callback panic withholds a partly completed value", wire: wire, disposition: projectionPanicked, wantRead: len(wire), wantProject: 1, wantErr: core.ErrExchangeContract},
		{name: "close failure withholds a fully validated projected value", wire: wire, closeErr: io.ErrClosedPipe, wantRead: len(wire), wantProject: 1, wantErr: core.ErrExchangeRequest, wantCause: io.ErrClosedPipe},
		{name: "callback refusal cannot erase a simultaneous close failure", wire: wire, disposition: projectionRefused, closeErr: io.ErrClosedPipe, wantRead: len(wire), wantProject: 1, wantErr: core.ErrExchangeRequest, wantCause: io.ErrUnexpectedEOF},
		{name: "unknown field never reaches projector", wire: unknownWire, wantRead: len(unknownWire), wantErr: core.ErrJSONContract},
		{name: "duplicate field cannot select an arbitrary source value", wire: duplicateWire, wantRead: len(duplicateWire), wantErr: core.ErrJSONContract},
		{name: "nonwire method cannot be supplied by JSON", wire: nonwireMember, wantRead: len(nonwireMember), wantErr: core.ErrJSONContract},
		{name: "wrong member type never reaches projector", wire: wrongType, wantRead: len(wrongType), wantErr: core.ErrJSONContract},
		{name: "truncated document cannot partially reach projector", wire: truncatedWire, wantRead: len(truncatedWire), wantErr: core.ErrJSONContract},
		{name: "absent document cannot manufacture projected value", wantErr: core.ErrJSONContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := &bindingObservedBody{reader: bytes.NewReader(tc.wire), err: tc.closeErr}
			method := tc.method
			if method == "" {
				method = http.MethodPost
			}
			request := httptest.NewRequestWithContext(t.Context(), method, "/", body)
			request.Header.Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
			writer := httptest.NewRecorder()
			call := socketServerCallFrom(t, writer, request)
			var projects int
			var projected projectedTransportDocument
			projector := func(ctx context.Context, gotCall exchange.SocketServerCall, got *projectedTransportDocument) error {
				projects++
				projected = *got
				if ctx != request.Context() || gotCall != call {
					return core.ErrPrimitiveContract
				}
				if tc.disposition == projectionUnchanged {
					return nil
				}
				got.Method = exchange.MethodPost
				if tc.disposition == projectionRefused {
					return io.ErrUnexpectedEOF
				}
				if tc.disposition == projectionPanicked {
					panic(io.ErrUnexpectedEOF)
				}
				return nil
			}
			if tc.disposition == projectionMissing {
				projector = nil
			}
			got, gotErr := exchange.ReceiveProjectedJSON[projectedTransportDocument, *projectedTransportDocument](exchange.ProjectedJSONReceiveCall[projectedTransportDocument, *projectedTransportDocument]{Call: call, Project: projector, Route: exchange.RouteSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt}})
			if !errors.Is(gotErr, tc.wantErr) || tc.wantCause != nil && !errors.Is(gotErr, tc.wantCause) || tc.closeErr != nil && !errors.Is(gotErr, tc.closeErr) {
				t.Fatalf("receive error = %v, want (%v,%v,%v)", gotErr, tc.wantErr, tc.wantCause, tc.closeErr)
			}
			if body.reads != tc.wantRead || body.closes != 1 || projects != tc.wantProject {
				t.Fatalf("read/close/project = (%d,%d,%d), want (%d,1,%d)", body.reads, body.closes, projects, tc.wantRead, tc.wantProject)
			}
			if projects != 0 && (projected.Message != document.Message || projected.Method != exchange.MethodUnknown) {
				t.Fatalf("pre-projection facts = %+v, want original message and absent method", projected)
			}
			if tc.wantErr != nil {
				if got.Body != nil || !got.IdempotencyKey.IsZero() || !errors.Is(gotErr, core.ErrExchangeRequest) {
					t.Fatalf("refusal = (%+v,%v), want no published value and typed request error", got, gotErr)
				}
			} else if got.Body == nil || got.Body.Message != tc.wantMessage || got.Body.Method != exchange.MethodPost || !got.IdempotencyKey.IsZero() {
				t.Fatalf("projected value = %+v, want exact message %q and method", got, tc.wantMessage)
			}
			if writer.Body.Len() != 0 || len(writer.Header()) != 0 || writer.Flushed {
				t.Fatalf("response body/headers/flush=%d/%d/%t, want 0/0/false", writer.Body.Len(), len(writer.Header()), writer.Flushed)
			}
		})
	}
}

// The singleton NoBody document supplies a real neutral projection domain:
// an admitted empty struct remains empty and emits no HTTP response. It does
// not manufacture a message to make the ordinary document validator pass.
func TestProjectedJSONEmptyDocumentLayerTriad(t *testing.T) {
	t.Parallel()
	wire, err := json.Marshal(exchange.NoBody{})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name         string
		inputs       [][]byte
		wantErr      error
		wantProjects int
	}{
		{name: "neutral empty object and null preserve the same empty typed state", inputs: [][]byte{wire, []byte("null")}, wantProjects: 1},
		{name: "unknown content cannot disappear into the empty struct", inputs: [][]byte{[]byte(`{"unexpected":1}`)}, wantErr: core.ErrJSONContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, input := range tc.inputs {
				body := &bindingObservedBody{reader: bytes.NewReader(input)}
				request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", body)
				request.Header.Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
				writer := httptest.NewRecorder()
				var projects int
				got, gotErr := exchange.ReceiveProjectedJSON[exchange.NoBody, *exchange.NoBody](exchange.ProjectedJSONReceiveCall[exchange.NoBody, *exchange.NoBody]{Call: socketServerCallFrom(t, writer, request), Route: exchange.RouteSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt}, Project: func(_ context.Context, _ exchange.SocketServerCall, _ *exchange.NoBody) error { projects++; return nil }})
				if !errors.Is(gotErr, tc.wantErr) || (got.Body != nil) != (tc.wantErr == nil) || !got.IdempotencyKey.IsZero() || projects != tc.wantProjects || body.closes != 1 || body.reads != len(input) {
					t.Fatalf("empty projection = (%+v,%v,%d projects,%d closes,%d bytes), want exact empty state and (%v,%d,1,%d)", got, gotErr, projects, body.closes, body.reads, tc.wantErr, tc.wantProjects, len(input))
				}
				if writer.Body.Len() != 0 || len(writer.Header()) != 0 || writer.Flushed {
					t.Fatalf("response body/headers/flush=%d/%d/%t, want 0/0/false", writer.Body.Len(), len(writer.Header()), writer.Flushed)
				}
			}
		})
	}
}
