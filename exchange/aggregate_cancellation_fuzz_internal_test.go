package exchange

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// Cancel only after Close has produced its native result. The public client
// then owns both facts and must preserve both through retry classification.
type aggregateCancelBody struct {
	*replayHandoffBody
	cancel context.CancelFunc
}

func (b *aggregateCancelBody) Close() error {
	err := b.replayHandoffBody.Close()
	if b.cancel != nil {
		b.cancel()
	}
	return err
}

func FuzzBoundedClientsPreserveCompletedProducerCause(f *testing.F) {
	seeds := []struct {
		payload []byte
		limit   uint16
		fault   replayHandoffBodyFault
		cancel  bool
	}{
		{limit: 2},
		{limit: 2, cancel: true},
		{payload: []byte{0xff}, limit: 2},
		{payload: []byte{0, 0xff}, limit: 2},
		{payload: []byte{0, 0xff}, limit: 2, cancel: true},
		{payload: []byte{0, 0xff}, limit: 2, fault: replayHandoffReadFailure},
		{payload: []byte{0, 0xff}, limit: 2, fault: replayHandoffReadFailure, cancel: true},
		{payload: []byte{0, 0xff}, limit: 2, fault: replayHandoffCloseFailure},
		{payload: []byte{0, 0xff}, limit: 2, fault: replayHandoffCloseFailure, cancel: true},
		{payload: []byte{0, 0xff, 1}, limit: 2, cancel: true},
		{payload: []byte{0, 0xff, 1}, limit: 2, fault: replayHandoffCloseFailure, cancel: true},
		{payload: bytes.Repeat([]byte{0xff}, 4096), limit: 2, cancel: true},
		{payload: bytes.Repeat([]byte{0xff}, 4095), limit: 4096},
		{payload: bytes.Repeat([]byte{0xff}, 4096), limit: 4096},
		{payload: bytes.Repeat([]byte{0xff}, 4097), limit: 4096},
		{limit: 1, fault: replayHandoffReadFailure, cancel: true},
		{limit: 1, fault: replayHandoffCloseFailure, cancel: true},
	}
	for _, seed := range seeds {
		recorder := httptest.NewRecorder()
		call, err := NewSocketServerCall(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
		if err != nil {
			f.Fatalf("seed socket = %v, want nil", err)
		}
		response := ServerBoundedResponse{Body: seed.payload, ContentType: core.HTTPMediaTypeOctetStream(), Status: core.HTTPStatusOK()}
		if err := response.Validate(); err != nil {
			f.Fatalf("seed response validation = %v, want nil", err)
		}
		if err := WriteBounded(BoundedWriteCall{Call: call, Response: response}); err != nil {
			f.Fatalf("seed wire emission = %v, want nil", err)
		}
		canonical := bytes.Clone(recorder.Body.Bytes())
		f.Add(canonical, seed.limit, uint8(seed.fault), seed.cancel)
	}
	f.Fuzz(func(t *testing.T, payload []byte, rawLimit uint16, rawFault uint8, cancelAtClose bool) {
		fault := replayHandoffBodyFault(rawFault)
		if len(payload) > 8192 || rawLimit == 0 || rawLimit > 4096 || (fault != replayHandoffExact && fault != replayHandoffReadFailure && fault != replayHandoffCloseFailure) {
			return
		}
		limit, err := core.NewByteCount(uint64(rawLimit))
		if err != nil {
			t.Fatalf("bounded ceiling fixture = %v, want nil", err)
		}
		target, err := core.ParseHTTPEndpoint("https://provider.example.test/aggregate")
		if err != nil {
			t.Fatalf("endpoint fixture = %v, want nil", err)
		}
		policy := OperationPolicy{OperationTimeout: runtimeAgreementPolicy(t).ReadTimeout, AttemptTimeout: runtimeAgreementPolicy(t).ReadTimeout, Retry: RetryPolicy{MaximumAttempts: 1}, Redirect: RedirectPolicy{Mode: RedirectReject}}
		if err := policy.Validate(); err != nil {
			t.Fatalf("single-attempt policy fixture = %v, want nil", err)
		}
		semantics := RequestSemantics{Method: MethodGet, Replay: ReplaySingleAttempt}
		doors := []struct {
			name string
			send func(context.Context, Client) (BoundedResponse, error)
		}{
			{name: "SendNoBodyBounded", send: func(ctx context.Context, client Client) (BoundedResponse, error) {
				return SendNoBodyBounded(NoBodyBoundedCall{Context: ctx, Client: client,
					Request: NoBodyBoundedRequest{Target: target, Semantics: semantics, ExpectedStatus: core.HTTPStatusOK()},
					Policy:  NoBodyBoundedPolicy{Operation: policy, ResponseBodyLimit: limit}})
			}},
			{name: "SendBounded", send: func(ctx context.Context, client Client) (BoundedResponse, error) {
				return SendBounded(BoundedCall{Context: ctx, Client: client,
					Request: BoundedRequest{Target: target, Semantics: semantics, ExpectedStatus: core.HTTPStatusOK(), RequestContentType: core.HTTPMediaTypeOctetStream(), Body: []byte{0xff}},
					Policy:  BoundedPolicy{Operation: policy, RequestBodyLimit: limit, ResponseBodyLimit: limit}})
			}},
		}
		for _, door := range doors {
			ctx, cancel := context.WithCancel(t.Context())
			body := &aggregateCancelBody{replayHandoffBody: &replayHandoffBody{reader: bytes.NewReader(payload), fault: fault}}
			if cancelAtClose {
				body.cancel = cancel
			}
			calls := 0
			client, err := NewClient(&http.Client{Transport: replayHandoffTransport(func(request *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: http.StatusOK, ContentLength: -1, Body: body, Request: request}, nil
			})})
			if err != nil {
				cancel()
				t.Fatalf("Go client fixture = %v, want nil", err)
			}
			got, gotErr := door.send(ctx, client)
			cancel()
			wantRead := min(len(payload), int(rawLimit)+1)
			if calls != 1 || body.closes != 1 || body.readBytes != wantRead {
				t.Fatalf("%s producer call/close/read = %d/%d/%d, want 1/1/%d", door.name, calls, body.closes, body.readBytes, wantRead)
			}
			var wantNative error
			if len(payload) > int(rawLimit) {
				wantNative = core.ErrExchangeBodyLimit
			} else if fault == replayHandoffReadFailure {
				wantNative = io.ErrUnexpectedEOF
			}
			if fault == replayHandoffCloseFailure {
				wantNative = errors.Join(wantNative, io.ErrClosedPipe)
			}
			wantRefused := cancelAtClose || wantNative != nil
			if (gotErr != nil) != wantRefused {
				t.Fatalf("%s error = %v, want refusal %t", door.name, gotErr, wantRefused)
			}
			if errors.Is(gotErr, core.ErrExchangeCancelled) != cancelAtClose || errors.Is(gotErr, context.Canceled) != cancelAtClose {
				t.Fatalf("%s cancellation = %v, want cancellation identities %t", door.name, gotErr, cancelAtClose)
			}
			for _, native := range []error{core.ErrExchangeBodyLimit, io.ErrUnexpectedEOF, io.ErrClosedPipe} {
				if errors.Is(gotErr, native) != errors.Is(wantNative, native) {
					t.Errorf("%s native cause %v retained = %t, want %t; error %v", door.name, native, errors.Is(gotErr, native), errors.Is(wantNative, native), gotErr)
				}
			}
			if errors.Is(gotErr, core.ErrExchangeResponse) != (wantNative != nil) {
				t.Errorf("%s response refusal identity = %t, want %t; error %v", door.name, errors.Is(gotErr, core.ErrExchangeResponse), wantNative != nil, gotErr)
			}
			// These entry points return an observation even when transfer or
			// classification fails. Only an incomplete aggregate is withheld;
			// already observed status and attempt count must remain exact.
			wantBody := payload
			if len(payload) > int(rawLimit) || fault == replayHandoffReadFailure {
				wantBody = nil
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("%s admitted response validation = %v, want nil", door.name, err)
			}
			if !bytes.Equal(got.Body, wantBody) || (wantBody == nil && got.Body != nil) || got.Metadata.Status != core.HTTPStatusOK() || got.Metadata.Attempts != 1 || got.Metadata.Bytes.Uint64() != uint64(len(wantBody)) || len(got.Metadata.Headers.Values) != 0 {
				t.Fatalf("%s observed response = %+v, want exact %x, OK, one attempt and no captured fields", door.name, got, wantBody)
			}
		}
	})
}
