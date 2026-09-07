package exchange

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"syscall"
	"testing"
	"testing/synctest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

type transportFailureDoor uint8

const (
	transportFailureNoBody transportFailureDoor = iota
	transportFailureBounded
	transportFailureUpload
	transportFailureDownload
	transportFailureRoundTrip
)

// A Go RoundTripper supplies the actual native failure. Its cancellation runs
// synchronously before returning, so no scheduler delay chooses which fact wins.
// Every public door must retain the native error and Go's typed URL wrapper.
func TestHTTPTransportFailureHandoffTable(t *testing.T) {
	t.Parallel()
	doors := []struct {
		name  string
		value transportFailureDoor
	}{
		{name: "SendNoBodyBounded", value: transportFailureNoBody},
		{name: "SendBounded", value: transportFailureBounded},
		{name: "Upload", value: transportFailureUpload},
		{name: "Download", value: transportFailureDownload},
		{name: "RoundTripStream", value: transportFailureRoundTrip},
	}
	reset := &net.OpError{Op: "read", Net: "tcp", Err: syscall.ECONNRESET}
	cases := []struct {
		name                         string
		native                       error
		cancelBefore, cancelAtReturn bool
		wantCalls                    int
		wantErr, wantNative          error
		wantCanceled, wantURL        bool
		wantNetError                 *net.OpError
		attemptNanoseconds           int64
		wantElapsedNanoseconds       int64
		wantDeadline                 bool
	}{
		{name: "native closed pipe remains an exact transport refusal", native: io.ErrClosedPipe, wantCalls: 1, wantErr: core.ErrExchangeTransport, wantNative: io.ErrClosedPipe, wantURL: true},
		{name: "cancellation cannot erase the same completed closed pipe failure", native: io.ErrClosedPipe, cancelAtReturn: true, wantCalls: 1, wantErr: core.ErrExchangeCancelled, wantNative: io.ErrClosedPipe, wantCanceled: true, wantURL: true},
		{name: "typed connection reset retains the original Go operation error", native: reset, wantCalls: 1, wantErr: core.ErrExchangeTransport, wantNative: syscall.ECONNRESET, wantURL: true, wantNetError: reset},
		{name: "cancellation cannot erase typed connection reset evidence", native: reset, cancelAtReturn: true, wantCalls: 1, wantErr: core.ErrExchangeCancelled, wantNative: syscall.ECONNRESET, wantCanceled: true, wantURL: true, wantNetError: reset},
		{name: "native cancellation does not falsely cancel the caller context", native: context.Canceled, wantCalls: 1, wantErr: core.ErrExchangeTransport, wantNative: context.Canceled, wantURL: true},
		{name: "nil response and nil cause retain Go client refusal", wantCalls: 1, wantErr: core.ErrExchangeTransport, wantURL: true},
		{name: "pre-canceled ingress cannot manufacture an unexecuted native error", native: io.ErrClosedPipe, cancelBefore: true, wantErr: core.ErrExchangeRequest, wantNative: context.Canceled, wantCanceled: true},
		{name: "minimum attempt deadline retains native error while operation remains live", native: reset, attemptNanoseconds: 1, wantElapsedNanoseconds: 1, wantDeadline: true, wantCalls: 1, wantErr: core.ErrExchangeCancelled, wantNative: syscall.ECONNRESET, wantCanceled: true, wantURL: true, wantNetError: reset},
		{name: "one above minimum attempt deadline is not rounded to its neighbor", native: reset, attemptNanoseconds: 2, wantElapsedNanoseconds: 2, wantDeadline: true, wantCalls: 1, wantErr: core.ErrExchangeCancelled, wantNative: syscall.ECONNRESET, wantCanceled: true, wantURL: true, wantNetError: reset},
	}
	for _, door := range doors {
		t.Run(door.name, func(t *testing.T) {
			t.Parallel()
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					// Each parallel row owns a Go time bubble. Go forbids
					// T.Parallel inside this callback; it is not another subtest.
					synctest.Test(t, func(t *testing.T) {
						start, err := temporal.Observe()
						if err != nil {
							t.Fatalf("initial Go clock observation = %v, want nil", err)
						}
						ctx, cancel := context.WithCancel(t.Context())
						defer cancel()
						if tc.cancelBefore {
							cancel()
						}
						calls := 0
						client, err := NewClient(&http.Client{Transport: replayHandoffTransport(func(request *http.Request) (*http.Response, error) {
							calls++
							if tc.attemptNanoseconds != 0 {
								<-request.Context().Done()
							}
							if tc.cancelAtReturn {
								cancel()
							}
							return nil, tc.native
						})})
						if err != nil {
							t.Fatalf("client fixture = %v, want nil", err)
						}
						target, err := core.ParseHTTPEndpoint("https://provider.example.test/native-failure")
						if err != nil {
							t.Fatalf("endpoint fixture = %v, want nil", err)
						}
						limit, err := core.NewByteCount(2)
						if err != nil {
							t.Fatalf("limit fixture = %v, want nil", err)
						}
						length, err := core.NewByteLength(2)
						if err != nil {
							t.Fatalf("length fixture = %v, want nil", err)
						}
						semantics := RequestSemantics{Method: MethodGet, Replay: ReplaySingleAttempt}
						timeout := runtimeAgreementPolicy(t).ReadTimeout
						operation := OperationPolicy{OperationTimeout: timeout, AttemptTimeout: timeout, Retry: RetryPolicy{MaximumAttempts: 1}, Redirect: RedirectPolicy{Mode: RedirectReject}}
						stream := StreamPolicy{OperationTimeout: timeout, AttemptTimeout: timeout, ErrorBodyLimit: limit, Redirect: RedirectPolicy{Mode: RedirectReject}}
						if tc.attemptNanoseconds != 0 {
							attempt, err := temporal.DurationFromNanoseconds(tc.attemptNanoseconds)
							if err != nil {
								t.Fatalf("attempt budget fixture = %v, want nil", err)
							}
							operation.AttemptTimeout, stream.AttemptTimeout = attempt, attempt
						}
						source := bytes.NewReader([]byte{0, 0xff})
						var destination bytes.Buffer
						var metadata ResponseMetadata
						var gotErr error
						switch door.value {
						case transportFailureNoBody:
							got, err := SendNoBodyBounded(NoBodyBoundedCall{Context: ctx, Client: client, Request: NoBodyBoundedRequest{Target: target, Semantics: semantics, ExpectedStatus: core.HTTPStatusOK()}, Policy: NoBodyBoundedPolicy{Operation: operation, ResponseBodyLimit: limit}})
							metadata, gotErr = got.Metadata, err
							if got.Body != nil {
								t.Fatalf("unobserved bounded body = %x, want nil", got.Body)
							}
						case transportFailureBounded:
							got, err := SendBounded(BoundedCall{Context: ctx, Client: client, Request: BoundedRequest{Target: target, Semantics: semantics, ExpectedStatus: core.HTTPStatusOK(), Body: []byte{0, 0xff}, RequestContentType: core.HTTPMediaTypeOctetStream()}, Policy: BoundedPolicy{Operation: operation, RequestBodyLimit: limit, ResponseBodyLimit: limit}})
							metadata, gotErr = got.Metadata, err
							if got.Body != nil {
								t.Fatalf("unobserved bounded body = %x, want nil", got.Body)
							}
						case transportFailureUpload:
							got, err := Upload(UploadCall{Context: ctx, Client: client, Request: UploadRequest{Target: target, Semantics: semantics, ExpectedStatus: core.HTTPStatusOK(), Source: source, ContentLength: length, ContentType: core.HTTPMediaTypeOctetStream()}, Policy: stream})
							metadata, gotErr = got.Metadata, err
						case transportFailureDownload:
							got, err := Download(DownloadCall{Context: ctx, Client: client, Request: DownloadRequest{Target: target, Semantics: semantics, ExpectedStatus: core.HTTPStatusOK(), Destination: &destination, ResponseBodyLimit: limit}, Policy: stream})
							metadata, gotErr = got.Metadata, err
						case transportFailureRoundTrip:
							got, err := RoundTripStream(StreamRoundTripCall{Context: ctx, Client: client, Request: StreamRoundTripRequest{Target: target, Semantics: semantics, ExpectedStatus: core.HTTPStatusOK(), Source: source, Destination: &destination, RequestContentLength: length, RequestContentType: core.HTTPMediaTypeOctetStream(), ResponseBodyLimit: limit}, Policy: stream})
							metadata, gotErr = got.Metadata, err
							if got.DeclaredRequestBytes != (core.ByteLength{}) {
								t.Fatalf("unconsumed request receipt = %v, want zero", got.DeclaredRequestBytes)
							}
						default:
							t.Fatalf("unknown transport test door = %d", door.value)
						}
						if calls != tc.wantCalls || !errors.Is(gotErr, tc.wantErr) {
							t.Fatalf("calls/error = %d/%v, want %d/%v", calls, gotErr, tc.wantCalls, tc.wantErr)
						}
						if tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) {
							t.Errorf("native cause = %v, want %v", gotErr, tc.wantNative)
						}
						if errors.Is(gotErr, core.ErrExchangeCancelled) != tc.wantCanceled {
							t.Errorf("caller cancellation identity = %t, want %t; error %v", errors.Is(gotErr, core.ErrExchangeCancelled), tc.wantCanceled, gotErr)
						}
						if errors.Is(gotErr, context.DeadlineExceeded) != tc.wantDeadline {
							t.Errorf("deadline identity = %t, want %t; error %v", errors.Is(gotErr, context.DeadlineExceeded), tc.wantDeadline, gotErr)
						}
						finish, err := temporal.Observe()
						if err != nil {
							t.Fatalf("final Go clock observation = %v, want nil", err)
						}
						elapsed, err := finish.Since(start)
						if err != nil || elapsed.Nanoseconds() != tc.wantElapsedNanoseconds {
							t.Fatalf("Go virtual elapsed = (%v,%v), want %d nanoseconds", elapsed, err, tc.wantElapsedNanoseconds)
						}
						urlErr, hasURL := errors.AsType[*url.Error](gotErr)
						if hasURL != tc.wantURL {
							t.Errorf("Go URL failure presence = %t, want %t; error %v", hasURL, tc.wantURL, gotErr)
						}
						if hasURL && (urlErr.URL != target.String() || (tc.native != nil && !errors.Is(urlErr.Err, tc.native))) {
							t.Errorf("Go URL failure = %v, want exact target and native %v", urlErr, tc.native)
						}
						netErr, _ := errors.AsType[*net.OpError](gotErr)
						if netErr != tc.wantNetError {
							t.Errorf("Go operation error = %v, want original %v", netErr, tc.wantNetError)
						}
						if tc.cancelBefore && errors.Is(gotErr, tc.native) {
							t.Errorf("unexecuted native cause escaped: %v", gotErr)
						}
						if metadata.Status != (core.HTTPStatusCode{}) || metadata.Attempts != 0 || metadata.Bytes != (core.ByteLength{}) || metadata.Headers.Values != nil || destination.Len() != 0 || source.Len() != 2 {
							t.Fatalf("unobserved transfer = metadata %+v, destination %x, remaining source %d; want no response or byte effects", metadata, destination.Bytes(), source.Len())
						}
					})
				})
			}
		})
	}
}
