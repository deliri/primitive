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

type jsonClientDoor uint8

const (
	jsonClientPlain jsonClientDoor = iota
	jsonClientBound
	jsonClientNoBody
	jsonClientSocket
	jsonClientBoundSocket
)

func FuzzSendJSONReplayNoBodyAndSocketResponses(f *testing.F) {
	seed := transportDocument{Message: "response-fact"}
	if err := seed.Validate(); err != nil {
		f.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := exchange.WriteJSON(exchange.JSONWriteCall[transportDocument]{Call: socketServerCallFrom(f, recorder, httptest.NewRequest(http.MethodGet, "/", nil)), Response: exchange.ServerJSONResponse[transportDocument]{Body: seed, Status: core.HTTPStatusOK()}}); err != nil {
		f.Fatal(err)
	}
	canonical := recorder.Body.Bytes()
	intent := replayBoundDocument{Operation: "operation-A"}
	if err := intent.Validate(); err != nil {
		f.Fatal(err)
	}
	for _, limit := range []uint16{uint16(len(canonical) - 1), uint16(len(canonical)), uint16(len(canonical) + 1)} {
		f.Add(canonical, intent.Operation, limit, false, false, false, false)
	}
	f.Add(canonical, intent.Operation, uint16(len(canonical)), true, false, false, false)
	f.Add(canonical, intent.Operation, uint16(len(canonical)), false, true, false, false)
	f.Add(canonical, intent.Operation, uint16(len(canonical)), false, false, true, false)
	f.Add(canonical, intent.Operation, uint16(len(canonical)), false, false, false, true)
	for _, operation := range []string{"", "invalid operation", "foreign"} {
		f.Add(canonical, operation, uint16(len(canonical)), false, false, false, false)
	}
	for _, hostile := range [][]byte{nil, []byte("null"), []byte(`{}`), []byte(`{"message":1}`), []byte(`{"message":"a","message":"b"}`), []byte(`{"message":"a","unknown":1}`)} {
		f.Add(hostile, intent.Operation, uint16(len(canonical)), false, false, false, false)
	}
	f.Fuzz(func(t *testing.T, wire []byte, operation string, window uint16, unexpectedStatus, closeFailure, foreignMedia, foreignKey bool) {
		if len(wire) > 8192 || len(operation) > exchange.IdempotencyKeyMaximumBytes+1 {
			return
		}
		var decoded *transportDocument
		decodeErr := json.Unmarshal(wire, &decoded, json.RejectUnknownMembers(true))
		wantDocument := decodeErr == nil && decoded != nil && decoded.Message != ""
		requestDocument := replayBoundDocument{Operation: operation}
		for _, door := range []jsonClientDoor{jsonClientPlain, jsonClientBound, jsonClientNoBody, jsonClientSocket, jsonClientBoundSocket} {
			var requests int
			var requestWire []byte
			var requestMethod, requestPath, requestKey, requestType string
			source := &bindingObservedBody{reader: &streamFuzzReader{source: bytes.NewReader(wire), window: int(window)}}
			if closeFailure {
				source.err = io.ErrClosedPipe
			}
			status := core.HTTPStatusOK()
			if unexpectedStatus {
				if err := status.AdmitInt(http.StatusBadRequest); err != nil {
					t.Fatal(err)
				}
			}
			client := mustExchangeClient(t, &http.Client{Transport: bindingTransport(func(request *http.Request) (*http.Response, error) {
				requests++
				requestMethod, requestPath, requestKey, requestType = request.Method, request.URL.Path, request.Header.Get(core.HTTPHeaderIdempotencyKey().String()), request.Header.Get(core.HTTPHeaderContentType().String())
				if request.Body != nil {
					var err error
					requestWire, err = io.ReadAll(request.Body)
					if err != nil {
						return nil, err
					}
					if err = request.Body.Close(); err != nil {
						return nil, err
					}
				}
				header := make(http.Header)
				media := core.HTTPMediaTypeJSON()
				if foreignMedia {
					media = core.HTTPMediaTypeOctetStream()
				}
				header.Set(core.HTTPHeaderContentType().String(), media.String())
				statusInt, err := status.Int()
				if err != nil {
					return nil, err
				}
				return &http.Response{StatusCode: statusInt, Header: header, Body: source, ContentLength: -1, Request: request}, nil
			})})
			target := mustEndpoint(t, "https://response-oracle.invalid/socket")
			keyText := intent.Operation
			if foreignKey {
				keyText = "foreign-key"
			}
			key, err := exchange.ParseIdempotencyKey(keyText)
			if err != nil {
				t.Fatal(err)
			}
			semantics := exchange.RequestSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt}
			if door == jsonClientBound {
				semantics.Replay = exchange.ReplayIdempotencyKey
				semantics.IdempotencyKey = key
			}
			call := exchange.JSONCall[replayBoundDocument]{Context: t.Context(), Client: client, Request: exchange.JSONRequest[replayBoundDocument]{Target: target, Body: requestDocument, Semantics: semantics, ExpectedStatus: core.HTTPStatusOK()}, Policy: exchange.JSONPolicy{Operation: singleAttemptOperationPolicy(t)}}
			var got exchange.JSONResponse[transportDocument]
			var gotErr error
			switch door {
			case jsonClientPlain:
				got, gotErr = exchange.SendJSON[replayBoundDocument, transportDocument](call)
			case jsonClientBound:
				got, gotErr = exchange.SendReplayBoundJSON[replayBoundDocument, transportDocument](call)
			case jsonClientNoBody:
				got, gotErr = exchange.SendNoBodyJSON[transportDocument](exchange.NoBodyJSONCall{Context: t.Context(), Client: client, Request: exchange.NoBodyRequest{Target: target, Semantics: exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt}, ExpectedStatus: core.HTTPStatusOK()}, Policy: exchange.NoBodyJSONPolicy{Operation: call.Policy.Operation}})
			case jsonClientSocket, jsonClientBoundSocket:
				replay := exchange.ReplaySingleAttempt
				if door == jsonClientBoundSocket {
					replay = exchange.ReplayIdempotencyKey
				}
				contract := socketPairContract(t, "/socket", replay)
				contract.SuccessStatus = core.HTTPStatusOK()
				socket, err := exchange.NewClientSocket(exchange.ClientSocketConfiguration{Target: target, Client: client, Contract: contract, Operation: call.Policy.Operation})
				if err != nil {
					t.Fatal(err)
				}
				if door == jsonClientSocket {
					got, gotErr = exchange.SendSocketJSON[replayBoundDocument, transportDocument](t.Context(), socket, requestDocument)
				} else {
					got, gotErr = exchange.SendReplayBoundSocketJSON[replayBoundDocument, transportDocument](t.Context(), socket, requestDocument)
				}
			default:
				t.Fatalf("unclassified client door %d", door)
			}
			wantRequest := door == jsonClientNoBody || replayIdentityInputAdmitted(operation)
			wantBindingRefusal := door == jsonClientBound && wantRequest && operation != keyText
			if wantBindingRefusal {
				wantRequest = false
			}
			if !wantRequest {
				if !errors.Is(gotErr, core.ErrExchangeRequest) || got.Body != (transportDocument{}) || got.Metadata.Attempts != 0 || got.Metadata.Status != (core.HTTPStatusCode{}) || got.Metadata.Bytes != (core.ByteLength{}) || got.Metadata.Headers.Values != nil {
					t.Fatalf("door %d ingress refusal = (%+v,%v), want zero and request identity", door, got, gotErr)
				}
				if errors.Is(gotErr, core.ErrExchangeIdempotencyBinding) != wantBindingRefusal || requests != 0 || source.reads != 0 || source.closes != 0 {
					t.Fatalf("door %d preflight made effects or lost binding refusal: requests/read/close=%d/%d/%d, error=%v", door, requests, source.reads, source.closes, gotErr)
				}
				if err := source.Close(); !errors.Is(err, source.err) {
					t.Fatal(err)
				}
				continue
			}
			wantMethod, wantKey, wantType := http.MethodPost, "", core.HTTPMediaTypeJSON().String()
			var expectedRequest []byte
			if door == jsonClientNoBody {
				wantMethod, wantType = http.MethodGet, ""
			} else {
				var err error
				expectedRequest, err = json.Marshal(requestDocument)
				if err != nil {
					t.Fatal(err)
				}
			}
			if door == jsonClientBound {
				wantKey = keyText
			}
			if door == jsonClientBoundSocket {
				wantKey = operation
			}
			if requests != 1 || requestMethod != wantMethod || requestPath != "/socket" || requestKey != wantKey || requestType != wantType || !bytes.Equal(requestWire, expectedRequest) {
				t.Fatalf("door %d wire = (%d,%q,%q,%q,%q,%q), want exact request (%q,%q,%q,%q)", door, requests, requestMethod, requestPath, requestKey, requestType, requestWire, wantMethod, wantKey, wantType, expectedRequest)
			}
			wantMediaRefusal := foreignMedia && !unexpectedStatus
			wantRead := len(wire)
			if wantMediaRefusal {
				wantRead = 0
			}
			wantOverflow := false
			wantBytes := len(wire)
			if wantMediaRefusal || wantOverflow {
				wantBytes = 0
			}
			if source.reads != wantRead || source.closes != 1 {
				t.Fatalf("door %d response custody read/close = %d/%d, want %d/1", door, source.reads, source.closes, wantRead)
			}
			if got.Metadata.Status != status || got.Metadata.Attempts != 1 || got.Metadata.Bytes.Uint64() != uint64(wantBytes) || got.Metadata.Headers.Values != nil {
				t.Fatalf("door %d metadata = %+v, want exact status/one attempt/%d retained bytes/no captured fields", door, got.Metadata, wantBytes)
			}
			wantAccepted := !unexpectedStatus && !wantMediaRefusal && !wantOverflow && !closeFailure && wantDocument
			if wantAccepted {
				if gotErr != nil || got.Body != *decoded || got.Validate() != nil {
					t.Fatalf("door %d accepted response=(%+v,%v), want exact independently decoded %+v", door, got, gotErr, decoded)
				}
				reencoded, err := got.Body.MarshalJSON()
				if err != nil {
					t.Fatal(err)
				}
				var roundTrip transportDocument
				if err := json.Unmarshal(reencoded, &roundTrip, json.RejectUnknownMembers(true)); err != nil || roundTrip != got.Body {
					t.Fatalf("accepted response lost canonical closure: (%+v,%v)", roundTrip, err)
				}
			} else {
				if gotErr == nil || got.Body != (transportDocument{}) {
					t.Fatalf("door %d refusal = (%+v,%v), want withheld document", door, got, gotErr)
				}
				if wantMediaRefusal || wantOverflow || closeFailure || !unexpectedStatus {
					if !errors.Is(gotErr, core.ErrExchangeResponse) {
						t.Fatalf("door %d lost response error identity: %v", door, gotErr)
					}
				} else {
					var statusErr exchange.StatusError
					if !errors.As(gotErr, &statusErr) || statusErr.Status() != status || statusErr.Expected() != core.HTTPStatusOK() {
						t.Fatalf("door %d lost exact provider status: %v", door, gotErr)
					}
				}
			}
			if errors.Is(gotErr, io.ErrClosedPipe) != closeFailure || errors.Is(gotErr, core.ErrExchangeBodyLimit) != wantOverflow || errors.Is(gotErr, core.ErrExchangeContentType) != wantMediaRefusal {
				t.Fatalf("door %d causes = %v, want close/extent/media=%t/%t/%t", door, gotErr, closeFailure, wantOverflow, wantMediaRefusal)
			}
		}
	})
}
