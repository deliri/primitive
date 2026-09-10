package exchange_test

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type jsonReceiveDoor uint8

const (
	jsonReceivePlain jsonReceiveDoor = iota
	jsonReceiveBound
	jsonReceiveProjected
	jsonReceiveSocket
	jsonReceiveBoundSocket
)

// Every callback calls all five public decoders. Go's strict JSON decoder and
// independently supplied source/header facts decide admission before production
// results are inspected. Socket construction is also exercised inside callbacks.
func FuzzReceiveJSONProjectedJSONAndSocketJSONCustody(f *testing.F) {
	const maximumFuzzBytes = 8192
	seed := replayBoundDocument{Operation: "operation-A"}
	if err := seed.Validate(); err != nil {
		f.Fatal(err)
	}
	canonical, err := seed.MarshalJSON()
	if err != nil {
		f.Fatal(err)
	}
	for _, limit := range []uint16{uint16(len(canonical) - 1), uint16(len(canonical)), uint16(len(canonical) + 1), 0} {
		f.Add(canonical, seed.Operation, limit, false, uint8(0))
	}
	f.Add(canonical, "foreign", uint16(len(canonical)), false, uint8(0))
	f.Add(canonical, "", uint16(len(canonical)), false, uint8(0))
	f.Add(canonical, seed.Operation, uint16(len(canonical)), true, uint8(0))
	f.Add(canonical, seed.Operation, uint16(len(canonical)), false, uint8(1))
	f.Add(canonical, seed.Operation, uint16(len(canonical)), false, uint8(2))
	for _, wire := range [][]byte{nil, []byte("null"), []byte(`{}`), []byte(`{"operation":"a","operation":"b"}`), []byte(`{"operation":1}`), []byte(`{"operation":"a","unknown":1}`)} {
		f.Add(wire, seed.Operation, uint16(len(canonical)), false, uint8(0))
	}
	f.Fuzz(func(t *testing.T, wire []byte, headerKey string, window uint16, closeFailure bool, projectionFault uint8) {
		if len(wire) > maximumFuzzBytes || len(headerKey) > exchange.IdempotencyKeyMaximumBytes+1 || projectionFault > 2 {
			return
		}
		var decoded *replayBoundDocument
		decodeErr := json.Unmarshal(wire, &decoded, json.RejectUnknownMembers(true))
		bodyAccepted := decodeErr == nil && decoded != nil && replayIdentityInputAdmitted(decoded.Operation)
		// Structure-only projection intentionally has Go's value semantics:
		// null yields a zero private struct, which the product projector may
		// complete. Ordinary validated document decoding refuses null.
		var projectedWire replayBoundDocument
		structureAccepted := json.Unmarshal(wire, &projectedWire, json.RejectUnknownMembers(true)) == nil
		keyAccepted := replayIdentityInputAdmitted(headerKey)
		for _, door := range []jsonReceiveDoor{jsonReceivePlain, jsonReceiveBound, jsonReceiveProjected, jsonReceiveSocket, jsonReceiveBoundSocket} {
			route := exchange.RouteSemantics{Method: exchange.MethodPost, Replay: exchange.ReplayIdempotencyKey}
			if door == jsonReceiveSocket {
				route.Replay = exchange.ReplaySingleAttempt
			}
			source := &bindingObservedBody{reader: &streamFuzzReader{source: bytes.NewReader(wire), window: int(window)}}
			if closeFailure {
				source.err = io.ErrClosedPipe
			}
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/socket", nil)
			request.Body, request.ContentLength = source, -1
			request.Header.Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
			if door != jsonReceiveSocket {
				request.Header.Set(core.HTTPHeaderIdempotencyKey().String(), headerKey)
			}
			destination := httptest.NewRecorder()
			call, err := exchange.NewSocketServerCall(destination, request)
			if err != nil {
				t.Fatal(err)
			}
			receive := exchange.JSONReceiveCall{Call: call, Route: route}
			var got exchange.Received[*replayBoundDocument]
			var gotErr error
			var projections int
			var projectedInput replayBoundDocument
			switch door {
			case jsonReceivePlain:
				got, gotErr = exchange.ReceiveJSON[replayBoundDocument, *replayBoundDocument](receive)
			case jsonReceiveBound:
				got, gotErr = exchange.ReceiveReplayBoundJSON[replayBoundDocument, *replayBoundDocument](receive)
			case jsonReceiveProjected:
				got, gotErr = exchange.ReceiveProjectedJSON[replayBoundDocument, *replayBoundDocument](exchange.ProjectedJSONReceiveCall[replayBoundDocument, *replayBoundDocument]{Call: call, Route: route, Project: func(ctx context.Context, projectedCall exchange.SocketServerCall, body *replayBoundDocument) error {
					projections++
					projectedInput = *body
					if ctx != request.Context() || projectedCall != call {
						return core.ErrPrimitiveContract
					}
					body.Operation = seed.Operation
					switch projectionFault {
					case 1:
						return io.ErrUnexpectedEOF
					case 2:
						body.Operation = ""
					}
					return nil
				}})
			case jsonReceiveSocket, jsonReceiveBoundSocket:
				contract := socketPairContract(t, "/socket", route.Replay)
				socket, constructorErr := exchange.NewServerSocket(contract)
				if constructorErr != nil {
					t.Fatalf("socket construction = %v, want admitted positive bound", constructorErr)
				}
				if door == jsonReceiveSocket {
					got, gotErr = exchange.ReceiveSocketJSON[replayBoundDocument, *replayBoundDocument](socket, call)
				} else {
					got, gotErr = exchange.ReceiveReplayBoundSocketJSON[replayBoundDocument, *replayBoundDocument](socket, call)
				}
			default:
				t.Fatalf("unclassified public receive door %d", door)
			}
			wantIngress := door == jsonReceiveSocket || keyAccepted
			wantRead := 0
			if wantIngress {
				wantRead = len(wire)
			}
			withinExtent := wantIngress
			wantProjections := 0
			if door == jsonReceiveProjected && withinExtent && structureAccepted {
				wantProjections = 1
			}
			wantAccepted := withinExtent && bodyAccepted
			wantOperation := ""
			if wantAccepted {
				wantOperation = decoded.Operation
			}
			if door == jsonReceiveProjected {
				wantAccepted = wantProjections == 1 && projectionFault == 0
				if wantAccepted {
					wantOperation = seed.Operation
				}
			}
			wantBindingRefusal := (door == jsonReceiveBound || door == jsonReceiveBoundSocket) && wantAccepted && decoded.Operation != headerKey
			if wantBindingRefusal || closeFailure {
				wantAccepted = false
			}
			if wantAccepted {
				wantKey := headerKey
				if door == jsonReceiveSocket {
					wantKey = ""
				}
				if gotErr != nil || got.Body == nil || got.Body.Operation != wantOperation || got.IdempotencyKey.String() != wantKey || got.Validate() != nil {
					t.Fatalf("door %d admission = (%+v,%v), want operation %q and key %q", door, got, gotErr, wantOperation, wantKey)
				}
			} else if !errors.Is(gotErr, core.ErrExchangeRequest) || got.Body != nil || !got.IdempotencyKey.IsZero() {
				t.Fatalf("door %d refusal = (%+v,%v), want zero and typed request refusal", door, got, gotErr)
			}
			if errors.Is(gotErr, io.ErrClosedPipe) != closeFailure || errors.Is(gotErr, core.ErrExchangeIdempotencyBinding) != wantBindingRefusal {
				t.Fatalf("door %d retained causes = %v, want close/binding (%t,%t)", door, gotErr, closeFailure, wantBindingRefusal)
			}
			if wantProjections == 1 && projectionFault == 1 && !errors.Is(gotErr, io.ErrUnexpectedEOF) {
				t.Fatalf("projection refusal = %v, want original callback cause", gotErr)
			}
			if source.reads != wantRead || source.closes != 1 || projections != wantProjections {
				t.Fatalf("door %d read/close/project = (%d,%d,%d), want (%d,1,%d)", door, source.reads, source.closes, projections, wantRead, wantProjections)
			}
			if wantProjections == 1 && projectedInput != projectedWire {
				t.Fatalf("projector input = %+v, want independently decoded %+v", projectedInput, projectedWire)
			}
			if destination.Body.Len() != 0 || len(destination.Header()) != 0 || destination.Flushed {
				t.Fatalf("response body/headers/flush=%d/%d/%t, want 0/0/false", destination.Body.Len(), len(destination.Header()), destination.Flushed)
			}
		}
	})
}

func replayIdentityInputAdmitted(value string) bool {
	return len(value) > 0 && len(value) <= exchange.IdempotencyKeyMaximumBytes && strings.IndexFunc(value, func(r rune) bool { return r < '!' || r > '~' }) < 0
}

func FuzzReceiveBoundedAndNoBodyCustody(f *testing.F) {
	const maximumFuzzBytes = 8192
	payload := []byte{0, 0xff, 'a'}
	recorder := httptest.NewRecorder()
	response := exchange.ServerBoundedResponse{Body: payload, ContentType: core.HTTPMediaTypeOctetStream(), Status: core.HTTPStatusOK()}
	if err := exchange.WriteBounded(exchange.BoundedWriteCall{Call: socketServerCallFrom(f, recorder, httptest.NewRequest(http.MethodGet, "/", nil)), Response: response}); err != nil {
		f.Fatal(err)
	}
	seed := recorder.Body.Bytes()
	identity := replayBoundDocument{Operation: "operation-A"}
	key, err := identity.IdempotencyKey()
	if err != nil {
		f.Fatal(err)
	}
	for _, limit := range []uint16{0, uint16(len(seed) - 1), uint16(len(seed)), uint16(len(seed) + 1)} {
		f.Add(seed, key.String(), limit, false)
	}
	f.Add([]byte{}, key.String(), uint16(1), false)
	f.Add(seed, key.String(), uint16(len(seed)), true)
	f.Add([]byte{}, key.String(), uint16(1), true)
	f.Add(seed, "", uint16(len(seed)), false)
	f.Add(seed, "key space", uint16(len(seed)), true)
	f.Fuzz(func(t *testing.T, data []byte, headerKey string, window uint16, closeFailure bool) {
		if len(data) > maximumFuzzBytes || len(headerKey) > exchange.IdempotencyKeyMaximumBytes+1 {
			return
		}
		for _, noBody := range []bool{false, true} {
			source := &bindingObservedBody{reader: &streamFuzzReader{source: bytes.NewReader(data), window: int(window)}}
			if closeFailure {
				source.err = io.ErrClosedPipe
			}
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
			request.Body = source
			request.Header.Set(core.HTTPHeaderIdempotencyKey().String(), headerKey)
			if !noBody {
				request.ContentLength = -1
				request.Header.Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeOctetStream().String())
			}
			writer := httptest.NewRecorder()
			call := socketServerCallFrom(t, writer, request)
			route := exchange.RouteSemantics{Method: exchange.MethodPost, Replay: exchange.ReplayIdempotencyKey}
			var gotBody []byte
			var gotKey exchange.IdempotencyKey
			var gotErr error
			if noBody {
				got, err := exchange.ReceiveNoBody(exchange.NoBodyReceiveCall{Call: call, Route: route})
				gotErr, gotKey = err, got.IdempotencyKey
			} else {
				got, err := exchange.ReceiveBounded(exchange.BoundedReceiveCall{Call: call, Route: route, ExpectedContentType: core.HTTPMediaTypeOctetStream()})
				gotErr, gotBody, gotKey = err, got.Body, got.IdempotencyKey
			}
			wantIngress := replayIdentityInputAdmitted(headerKey)
			wantRead := 0
			if wantIngress {
				if noBody {
					wantRead = min(len(data), 1)
				} else {
					wantRead = len(data)
				}
			}
			wantAccepted := wantIngress && !closeFailure
			if noBody {
				wantAccepted = wantAccepted && len(data) == 0

			}
			if wantAccepted {
				if gotErr != nil || gotKey.String() != headerKey || !noBody && !bytes.Equal(gotBody, data) {
					t.Fatalf("noBody %t admission = (%x,%q,%v), want exact source and key %q", noBody, gotBody, gotKey.String(), gotErr, headerKey)
				}
			} else if !errors.Is(gotErr, core.ErrExchangeRequest) || gotBody != nil || !gotKey.IsZero() {
				t.Fatalf("noBody %t refusal = (%x,%q,%v), want zero and request refusal", noBody, gotBody, gotKey.String(), gotErr)
			}
			wantOverflow := false
			if errors.Is(gotErr, core.ErrExchangeBodyLimit) != wantOverflow || errors.Is(gotErr, io.ErrClosedPipe) != closeFailure {
				t.Fatalf("noBody %t error = %v, want overflow/close (%t,%t)", noBody, gotErr, wantOverflow, closeFailure)
			}
			if source.reads != wantRead || source.closes != 1 {
				t.Fatalf("noBody %t read/close = (%d,%d), want (%d,1)", noBody, source.reads, source.closes, wantRead)
			}
			if writer.Body.Len() != 0 || len(writer.Header()) != 0 || writer.Flushed {
				t.Fatalf("response body/headers/flush=%d/%d/%t, want 0/0/false", writer.Body.Len(), len(writer.Header()), writer.Flushed)
			}
		}
	})
}
