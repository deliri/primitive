package exchange

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestStreamDeclarationHandoffLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                                string
		declared                            uint64
		closeFault, dropMetadata, dropWhole bool
		wantErr                             error
		wantZero                            bool
	}{
		{name: "admitted upload declaration survives replay projection", declared: 2},
		{name: "empty upload cannot invent request or response bytes"},
		{name: "declaration without its HTTP observation is not an absent refusal", declared: 2, closeFault: true, dropMetadata: true, wantErr: core.ErrExchangeRequest, wantZero: true},
		{name: "absent failed observation remains absent while exhaustion owns its count", declared: 2, closeFault: true, dropWhole: true, wantErr: core.ErrExchangeRetryExhausted, wantZero: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			target, err := core.ParseHTTPEndpoint("https://provider.example.test/declaration-handoff")
			if err != nil {
				t.Fatal(err)
			}
			length, err := core.NewByteLength(tc.declared)
			if err != nil {
				t.Fatal(err)
			}
			body := &replayHandoffBody{reader: bytes.NewReader(nil)}
			if tc.closeFault {
				body.fault = replayHandoffCloseFailure
			}
			calls := 0
			client, err := NewClient(&http.Client{Transport: replayHandoffTransport(func(request *http.Request) (*http.Response, error) {
				calls++
				if err := request.Body.Close(); err != nil {
					return nil, err
				}
				return &http.Response{StatusCode: http.StatusOK, ContentLength: 0, Body: body, Request: request}, nil
			})})
			if err != nil {
				t.Fatal(err)
			}
			produced, producerErr := Upload(UploadCall{Context: t.Context(), Client: client, Request: UploadRequest{Target: target, Source: bytes.NewReader(make([]byte, tc.declared)), ContentLength: new(length), ContentType: core.HTTPMediaTypeOctetStream(), ExpectedStatus: core.HTTPStatusOK(), Semantics: RequestSemantics{Method: MethodPut, Replay: ReplaySingleAttempt}}, Policy: StreamPolicy{OperationTimeout: runtimeAgreementPolicy(t).ReadTimeout, AttemptTimeout: runtimeAgreementPolicy(t).ReadTimeout, Redirect: RedirectPolicy{Mode: RedirectReject}}})
			if produced.DeclaredRequestBytes != length || produced.Metadata.Status != core.HTTPStatusOK() || produced.Metadata.Attempts != 1 || produced.Metadata.Bytes != (core.ByteLength{}) || produced.Metadata.Headers.Values != nil || calls != 1 || body.closes != 1 {
				t.Fatalf("upload producer = %+v, calls/close %d/%d, want exact declaration %v and HTTP observation", produced, calls, body.closes, length)
			}
			if (producerErr != nil) != tc.closeFault || tc.closeFault && !errors.Is(producerErr, io.ErrClosedPipe) {
				t.Fatalf("upload producer cause = %v, want native close fault=%t", producerErr, tc.closeFault)
			}
			if tc.dropMetadata {
				produced.Metadata = ResponseMetadata{}
			}
			if tc.dropWhole {
				produced = StreamResponse{}
			}
			callbacks := 0
			got, gotErr := ReplayStream(StreamReplayCall{Context: t.Context(), Policy: StreamReplayPolicy{OperationTimeout: runtimeAgreementPolicy(t).ReadTimeout, Retry: RetryPolicy{MaximumAttempts: 1}}, Attempt: func(context.Context, uint64) (StreamResponse, error) { callbacks++; return produced, producerErr }})
			if !errors.Is(gotErr, tc.wantErr) || callbacks != 1 || calls != 1 || body.closes != 1 {
				t.Fatalf("replay = (%+v,%v) callback/provider/close %d/%d/%d, want %v and 1/1/1", got, gotErr, callbacks, calls, body.closes, tc.wantErr)
			}
			if tc.closeFault && !errors.Is(gotErr, io.ErrClosedPipe) {
				t.Fatalf("replay dropped native close cause: %v", gotErr)
			}
			wantStatus := core.HTTPStatusOK()
			wantAttempts := uint64(1)
			wantDeclared := length
			if tc.wantZero {
				wantStatus = core.HTTPStatusCode{}
				wantAttempts = 0
				wantDeclared = core.ByteLength{}
			}
			if got.Metadata.Status != wantStatus || got.Metadata.Attempts != wantAttempts || got.DeclaredRequestBytes != wantDeclared || got.Metadata.Bytes != (core.ByteLength{}) || got.Metadata.Headers.Values != nil {
				t.Fatalf("replay facts = %+v, want status %v, attempts %d, declaration %v and absent bytes/headers", got, wantStatus, wantAttempts, wantDeclared)
			}
		})
	}
}
