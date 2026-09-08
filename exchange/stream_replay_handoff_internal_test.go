package exchange

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

type replayHandoffBodyFault uint8

const (
	replayHandoffExact replayHandoffBodyFault = iota
	replayHandoffOverflow
	replayHandoffReadFailure
	replayHandoffCloseFailure
)

type replayHandoffBody struct {
	reader    *bytes.Reader
	fault     replayHandoffBodyFault
	closes    int
	readBytes int
}

func (b *replayHandoffBody) Read(p []byte) (int, error) {
	n, err := b.reader.Read(p)
	b.readBytes += n
	if errors.Is(err, io.EOF) && b.fault == replayHandoffReadFailure {
		return n, io.ErrUnexpectedEOF
	}
	return n, err
}
func (b *replayHandoffBody) Close() error {
	b.closes++
	if b.fault == replayHandoffCloseFailure {
		return io.ErrClosedPipe
	}
	return nil
}

type replayHandoffTransport func(*http.Request) (*http.Response, error)

func (r replayHandoffTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return r(request)
}

func TestReplayStreamDownloadHandoffLayerTriad(t *testing.T) {
	t.Parallel()
	// These are named regression and status-boundary rows. They are not the
	// complete producer/classifier matrix required for the package sweep.
	cases := []struct {
		name            string
		status          int
		payload         []byte
		fault           replayHandoffBodyFault
		cancelAtHandoff bool
		wantProducerErr error
		wantErr         error
		wantBytes       uint64
		wantAttempts    uint64
		wantStatusError bool
		wantExhausted   bool
	}{
		{name: "exact expected status preserves both binary bytes", status: http.StatusOK, payload: []byte{0x00, 0xff}, wantBytes: 2, wantAttempts: 1},
		{name: "other successful status cannot satisfy exact expected status", status: http.StatusCreated, wantProducerErr: core.ErrExchangeResponse, wantErr: core.ErrExchangeResponse, wantAttempts: 1, wantStatusError: true},
		{name: "status immediately below request-timeout retry remains terminal", status: http.StatusProxyAuthRequired, wantProducerErr: core.ErrExchangeResponse, wantErr: core.ErrExchangeResponse, wantAttempts: 1, wantStatusError: true},
		{name: "request-timeout status consumes exactly the retry budget", status: http.StatusRequestTimeout, wantProducerErr: core.ErrExchangeResponse, wantErr: core.ErrExchangeRetryExhausted, wantAttempts: 2, wantStatusError: true, wantExhausted: true},
		{name: "status immediately above request-timeout retry remains terminal", status: http.StatusConflict, wantProducerErr: core.ErrExchangeResponse, wantErr: core.ErrExchangeResponse, wantAttempts: 1, wantStatusError: true},
		{name: "status immediately below too-early retry remains terminal", status: http.StatusFailedDependency, wantProducerErr: core.ErrExchangeResponse, wantErr: core.ErrExchangeResponse, wantAttempts: 1, wantStatusError: true},
		{name: "too-early status consumes exactly the retry budget", status: http.StatusTooEarly, wantProducerErr: core.ErrExchangeResponse, wantErr: core.ErrExchangeRetryExhausted, wantAttempts: 2, wantStatusError: true, wantExhausted: true},
		{name: "status immediately above too-early retry remains terminal", status: http.StatusUpgradeRequired, wantProducerErr: core.ErrExchangeResponse, wantErr: core.ErrExchangeResponse, wantAttempts: 1, wantStatusError: true},
		{name: "status immediately below rate-limit retry remains terminal", status: http.StatusPreconditionRequired, wantProducerErr: core.ErrExchangeResponse, wantErr: core.ErrExchangeResponse, wantAttempts: 1, wantStatusError: true},
		{name: "rate-limit status consumes exactly the retry budget", status: http.StatusTooManyRequests, wantProducerErr: core.ErrExchangeResponse, wantErr: core.ErrExchangeRetryExhausted, wantAttempts: 2, wantStatusError: true, wantExhausted: true},
		{name: "status immediately above rate-limit retry remains terminal", status: http.StatusTooManyRequests + 1, wantProducerErr: core.ErrExchangeResponse, wantErr: core.ErrExchangeResponse, wantAttempts: 1, wantStatusError: true},
		{name: "last client-error status cannot acquire server-error retry", status: http.StatusInternalServerError - 1, wantProducerErr: core.ErrExchangeResponse, wantErr: core.ErrExchangeResponse, wantAttempts: 1, wantStatusError: true},
		{name: "first server-error status consumes exactly the retry budget", status: http.StatusInternalServerError, wantProducerErr: core.ErrExchangeResponse, wantErr: core.ErrExchangeRetryExhausted, wantAttempts: 2, wantStatusError: true, wantExhausted: true},
		{name: "last expressible server-error status remains retryable", status: 599, wantProducerErr: core.ErrExchangeResponse, wantErr: core.ErrExchangeRetryExhausted, wantAttempts: 2, wantStatusError: true, wantExhausted: true},
		{name: "oversized successful stream retains exact partial destination and refuses replay", status: http.StatusOK, payload: []byte{0x00, 0xff, 0x01}, fault: replayHandoffOverflow, wantProducerErr: core.ErrExchangeBodyLimit, wantErr: core.ErrExchangeBodyLimit, wantBytes: 2, wantAttempts: 1},
		{name: "oversized error body overrides retryable status without entering destination", status: http.StatusServiceUnavailable, payload: []byte{0x00, 0xff, 0x01}, fault: replayHandoffOverflow, wantProducerErr: core.ErrExchangeBodyLimit, wantErr: core.ErrExchangeBodyLimit, wantAttempts: 1, wantStatusError: true},
		{name: "truncated successful stream retains native failure and partial byte count", status: http.StatusOK, payload: []byte{0x00}, fault: replayHandoffReadFailure, wantProducerErr: io.ErrUnexpectedEOF, wantErr: core.ErrExchangeRetryExhausted, wantBytes: 1, wantAttempts: 2, wantExhausted: true},
		{name: "truncated error body retains both status and native failure", status: http.StatusServiceUnavailable, payload: []byte{0x00}, fault: replayHandoffReadFailure, wantProducerErr: io.ErrUnexpectedEOF, wantErr: core.ErrExchangeRetryExhausted, wantAttempts: 2, wantStatusError: true, wantExhausted: true},
		{name: "failed close after successful transfer cannot become success", status: http.StatusOK, payload: []byte{0x00, 0xff}, fault: replayHandoffCloseFailure, wantProducerErr: io.ErrClosedPipe, wantErr: core.ErrExchangeRetryExhausted, wantBytes: 2, wantAttempts: 2, wantExhausted: true},
		{name: "failed error-body close retains status and native close failure", status: http.StatusServiceUnavailable, payload: []byte{0x00, 0xff}, fault: replayHandoffCloseFailure, wantProducerErr: io.ErrClosedPipe, wantErr: core.ErrExchangeRetryExhausted, wantAttempts: 2, wantStatusError: true, wantExhausted: true},
		{name: "handoff cancellation retains a completed binary transfer", status: http.StatusOK, payload: []byte{0x00, 0xff}, cancelAtHandoff: true, wantErr: context.Canceled, wantBytes: 2, wantAttempts: 1},
		{name: "empty successful response creates no destination bytes", status: http.StatusOK, wantAttempts: 1},
		{name: "handoff cancellation preserves retryable status without another attempt", status: http.StatusServiceUnavailable, cancelAtHandoff: true, wantProducerErr: core.ErrExchangeResponse, wantErr: context.Canceled, wantAttempts: 1, wantStatusError: true},
		{name: "handoff cancellation preserves terminal status without replacing its cause", status: http.StatusCreated, cancelAtHandoff: true, wantProducerErr: core.ErrExchangeResponse, wantErr: context.Canceled, wantAttempts: 1, wantStatusError: true},
		{name: "handoff cancellation cannot erase successful-stream native read refusal", status: http.StatusOK, payload: []byte{0x00}, fault: replayHandoffReadFailure, cancelAtHandoff: true, wantProducerErr: io.ErrUnexpectedEOF, wantErr: context.Canceled, wantBytes: 1, wantAttempts: 1},
		{name: "handoff cancellation cannot erase successful-stream native close refusal", status: http.StatusOK, payload: []byte{0x00, 0xff}, fault: replayHandoffCloseFailure, cancelAtHandoff: true, wantProducerErr: io.ErrClosedPipe, wantErr: context.Canceled, wantBytes: 2, wantAttempts: 1},
		{name: "handoff cancellation cannot erase acknowledged overflow refusal", status: http.StatusOK, payload: []byte{0x00, 0xff, 0x01}, fault: replayHandoffOverflow, cancelAtHandoff: true, wantProducerErr: core.ErrExchangeBodyLimit, wantErr: context.Canceled, wantBytes: 2, wantAttempts: 1},
		{name: "handoff cancellation preserves both status and error-body native read refusal", status: http.StatusServiceUnavailable, payload: []byte{0x00}, fault: replayHandoffReadFailure, cancelAtHandoff: true, wantProducerErr: io.ErrUnexpectedEOF, wantErr: context.Canceled, wantAttempts: 1, wantStatusError: true},
		{name: "handoff cancellation preserves both status and error-body native close refusal", status: http.StatusServiceUnavailable, payload: []byte{0x00, 0xff}, fault: replayHandoffCloseFailure, cancelAtHandoff: true, wantProducerErr: io.ErrClosedPipe, wantErr: context.Canceled, wantAttempts: 1, wantStatusError: true},
		{name: "handoff cancellation preserves status plus terminal error-body overflow", status: http.StatusServiceUnavailable, payload: []byte{0x00, 0xff, 0x01}, fault: replayHandoffOverflow, cancelAtHandoff: true, wantProducerErr: core.ErrExchangeBodyLimit, wantErr: context.Canceled, wantAttempts: 1, wantStatusError: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			limit, err := core.NewByteCount(2)
			if err != nil {
				t.Fatalf("NewByteCount(2) setup error = %v, want nil", err)
			}
			target, err := core.ParseHTTPEndpoint("https://provider.example.test/stream")
			if err != nil {
				t.Fatalf("ParseHTTPEndpoint() setup error = %v, want nil", err)
			}
			var wantStatus core.HTTPStatusCode
			if err := wantStatus.AdmitInt(tc.status); err != nil {
				t.Fatalf("AdmitInt(%d) setup error = %v, want nil", tc.status, err)
			}
			var gotAttempts, gotProviderCalls uint64
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			got, gotErr := ReplayStream(StreamReplayCall{
				Context: ctx, Policy: replayHandoffPolicy(t),
				Attempt: func(ctx context.Context, attempt uint64) (StreamResponse, error) {
					gotAttempts++
					if attempt != gotAttempts {
						t.Fatalf("attempt index = %d, want %d", attempt, gotAttempts)
					}
					body := &replayHandoffBody{reader: bytes.NewReader(tc.payload), fault: tc.fault}
					client, err := NewClient(&http.Client{Transport: replayHandoffTransport(func(request *http.Request) (*http.Response, error) {
						gotProviderCalls++
						return &http.Response{StatusCode: tc.status, Body: body, ContentLength: -1, Request: request}, nil
					})})
					if err != nil {
						t.Fatalf("NewClient() setup error = %v, want nil", err)
					}
					var destination bytes.Buffer
					produced, producedErr := Download(DownloadCall{
						Context: ctx, Client: client,
						Request: DownloadRequest{Target: target, Destination: &destination,
							Semantics:      RequestSemantics{Method: MethodGet, Replay: ReplaySingleAttempt},
							ExpectedStatus: core.HTTPStatusOK(), ResponseBodyLimit: limit},
						Policy: StreamPolicy{OperationTimeout: replayDuration(t, 60*int64(temporal.NanosecondsPerSecond)),
							AttemptTimeout: replayDuration(t, 30*int64(temporal.NanosecondsPerSecond)),
							ErrorBodyLimit: limit, Redirect: RedirectPolicy{Mode: RedirectReject}},
					})
					if produced.Metadata.Status != wantStatus || produced.Metadata.Attempts != 1 || produced.Metadata.Bytes.Uint64() != tc.wantBytes {
						t.Fatalf("producer metadata = %+v, want status %v, one attempt and %d written bytes", produced.Metadata, wantStatus, tc.wantBytes)
					}
					if body.closes != 1 || body.readBytes != len(tc.payload) {
						t.Fatalf("producer body read/close = %d/%d, want %d/1", body.readBytes, body.closes, len(tc.payload))
					}
					if !bytes.Equal(destination.Bytes(), tc.payload[:tc.wantBytes]) {
						t.Fatalf("producer destination = %x, want %x", destination.Bytes(), tc.payload[:tc.wantBytes])
					}
					if !errors.Is(producedErr, tc.wantProducerErr) {
						t.Fatalf("producer error = %v, want %v", producedErr, tc.wantProducerErr)
					}
					producerStatus, gotProducerStatusError := errors.AsType[StatusError](producedErr)
					if gotProducerStatusError != tc.wantStatusError {
						t.Fatalf("producer status error present = %t, want %t; cause %v", gotProducerStatusError, tc.wantStatusError, producedErr)
					}
					if gotProducerStatusError && (producerStatus.Status() != wantStatus || producerStatus.Expected() != core.HTTPStatusOK()) {
						t.Fatalf("producer status error = %v, want observed %v and required OK", producerStatus, wantStatus)
					}
					if tc.cancelAtHandoff {
						cancel()
					}
					return produced, producedErr
				},
			})
			if gotAttempts != tc.wantAttempts || gotProviderCalls != tc.wantAttempts || got.Metadata.Attempts != tc.wantAttempts {
				t.Fatalf("replay calls/provider/metadata = %d/%d/%d, want %d each; error %v", gotAttempts, gotProviderCalls, got.Metadata.Attempts, tc.wantAttempts, gotErr)
			}
			if got.Metadata.Status != wantStatus || got.Metadata.Bytes.Uint64() != tc.wantBytes || got.Metadata.Headers.Values != nil {
				t.Fatalf("replay metadata = %+v, want status %v, bytes %d and absent headers", got.Metadata, wantStatus, tc.wantBytes)
			}
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("final error = %v, want %v", gotErr, tc.wantErr)
			}
			if errors.Is(gotErr, core.ErrExchangeCancelled) != tc.cancelAtHandoff {
				t.Fatalf("final cancellation identity = %t, want %t; cause %v", errors.Is(gotErr, core.ErrExchangeCancelled), tc.cancelAtHandoff, gotErr)
			}
			if tc.wantProducerErr != nil && !errors.Is(gotErr, tc.wantProducerErr) {
				t.Fatalf("retained producer error = %v, want %v", gotErr, tc.wantProducerErr)
			}
			statusError, gotStatusError := errors.AsType[StatusError](gotErr)
			if gotStatusError != tc.wantStatusError {
				t.Fatalf("final status error present = %t, want %t; error %v", gotStatusError, tc.wantStatusError, gotErr)
			}
			if gotStatusError && (statusError.Status() != wantStatus || statusError.Expected() != core.HTTPStatusOK()) {
				t.Fatalf("final status error = %v, want observed %v and required OK", statusError, wantStatus)
			}
			exhausted, gotExhausted := errors.AsType[RetryExhaustedError](gotErr)
			if gotExhausted != tc.wantExhausted || errors.Is(gotErr, core.ErrExchangeRetryExhausted) != tc.wantExhausted {
				t.Fatalf("exhaustion = %t/%v, want %t", gotExhausted, gotErr, tc.wantExhausted)
			}
			if gotExhausted && exhausted.Attempts() != tc.wantAttempts {
				t.Fatalf("exhaustion attempts = %d, want %d", exhausted.Attempts(), tc.wantAttempts)
			}
		})
	}
}

func replayHandoffPolicy(t testing.TB) StreamReplayPolicy {
	t.Helper()
	policy := replayStreamPolicy(t, 2)
	policy.OperationTimeout = replayDuration(t, 60*int64(temporal.NanosecondsPerSecond))
	return policy
}

func TestReplayStreamCannotRepairInvalidAttemptEvidence(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		status    core.HTTPStatusCode
		attempts  uint64
		wantCalls int
		wantErr   error
	}{
		{name: "zero attempt count", status: core.HTTPStatusOK(), wantCalls: 1, wantErr: core.ErrExchangeRequest},
		{name: "multiple attempts behind single attempt callback", status: core.HTTPStatusOK(), attempts: 2, wantCalls: 1, wantErr: core.ErrExchangeRequest},
		{name: "missing observed status", attempts: 1, wantCalls: 1, wantErr: core.ErrExchangeRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			got, err := ReplayStream(StreamReplayCall{Context: t.Context(), Policy: replayHandoffPolicy(t),
				Attempt: func(context.Context, uint64) (StreamResponse, error) {
					calls++
					return StreamResponse{Metadata: ResponseMetadata{Status: tc.status, Attempts: tc.attempts}}, nil
				},
			})
			if calls != tc.wantCalls || !errors.Is(err, tc.wantErr) || !errors.Is(err, core.ErrExchangeContract) || errors.Is(err, core.ErrExchangeRetryExhausted) {
				t.Fatalf("invalid callback = %d calls/%v, want one call and terminal request/contract refusal", calls, err)
			}
			if got.Metadata.Status != (core.HTTPStatusCode{}) || got.Metadata.Attempts != 0 || got.Metadata.Bytes.Uint64() != 0 || len(got.Metadata.Headers.Values) != 0 {
				t.Fatalf("invalid callback released metadata = %+v, want zero", got.Metadata)
			}
		})
	}
}
