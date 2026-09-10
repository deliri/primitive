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

func TestStreamLifetimeBelongsToCallerLayerTriad(t *testing.T) {
	t.Parallel()
	duration := runtimeAgreementPolicy(t).ReadTimeout
	cases := []struct {
		name                                   string
		operation, attempt                     temporal.Duration
		parentDeadline, canceled, wantDeadline bool
		wantErr                                error
	}{
		{name: "caller keeps lifetime open"},
		{name: "caller deadline is inherited exactly", parentDeadline: true, wantDeadline: true},
		{name: "only operation timeout is explicit", operation: duration, wantDeadline: true},
		{name: "only attempt timeout is explicit", attempt: duration, wantDeadline: true},
		{name: "both timeouts remain enforceable", operation: duration, attempt: duration, wantDeadline: true},
		{name: "canceled parent cannot reach transport", canceled: true, wantErr: context.Canceled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, door := range []Method{MethodGet, MethodPut, MethodPost} {
				t.Run(door.String(), func(t *testing.T) {
					t.Parallel()
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					if tc.parentDeadline {
						parent, closeParent, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: ctx, Duration: duration})
						if err != nil {
							t.Fatal(err)
						}
						defer closeParent()
						ctx = parent
					}
					if tc.canceled {
						cancel()
					}
					var observed context.Context
					calls := 0
					client, err := NewClient(&http.Client{Transport: replayHandoffTransport(func(r *http.Request) (*http.Response, error) {
						calls++
						observed = r.Context()
						if r.Body != nil {
							_, readErr := io.Copy(io.Discard, r.Body)
							closeErr := r.Body.Close()
							if readErr != nil || closeErr != nil {
								return nil, errors.Join(readErr, closeErr)
							}
						}
						return &http.Response{StatusCode: http.StatusOK, ContentLength: 1, Body: io.NopCloser(bytes.NewReader([]byte{0xa5})), Request: r}, nil
					})})
					if err != nil {
						t.Fatal(err)
					}
					target, err := core.ParseHTTPEndpoint("https://stream-lifetime.invalid/data")
					if err != nil {
						t.Fatal(err)
					}
					policy := StreamPolicy{OperationTimeout: tc.operation, AttemptTimeout: tc.attempt, Redirect: RedirectPolicy{Mode: RedirectReject}}
					var gotErr error
					semantics := RequestSemantics{Method: door, Replay: ReplaySingleAttempt}
					switch door {
					case MethodGet:
						_, gotErr = Download(DownloadCall{Context: ctx, Client: client, Policy: policy, Request: DownloadRequest{Target: target, Destination: io.Discard, Semantics: semantics, ExpectedStatus: core.HTTPStatusOK()}})
					case MethodPut:
						_, gotErr = Upload(UploadCall{Context: ctx, Client: client, Policy: policy, Request: UploadRequest{Target: target, Source: bytes.NewReader([]byte{1}), Semantics: semantics, ContentType: core.HTTPMediaTypeOctetStream(), ExpectedStatus: core.HTTPStatusOK()}})
					case MethodPost:
						_, gotErr = RoundTripStream(StreamRoundTripCall{Context: ctx, Client: client, Policy: policy, Request: StreamRoundTripRequest{Target: target, Source: bytes.NewReader([]byte{1}), Destination: io.Discard, Semantics: semantics, RequestContentType: core.HTTPMediaTypeOctetStream(), ExpectedStatus: core.HTTPStatusOK()}})
					}
					if !errors.Is(gotErr, tc.wantErr) {
						t.Fatalf("stream=%v, want %v", gotErr, tc.wantErr)
					}
					wantCalls := 1
					if tc.canceled {
						wantCalls = 0
					}
					if calls != wantCalls {
						t.Fatalf("HTTP calls=%d, want %d", calls, wantCalls)
					}
					if observed == nil {
						return
					}
					deadline, hasDeadline := observed.Deadline()
					if hasDeadline != tc.wantDeadline || !errors.Is(observed.Err(), context.Canceled) {
						t.Fatalf("deadline present=%t context after return=%v, want %t and owned cancellation", hasDeadline, observed.Err(), tc.wantDeadline)
					}
					if tc.parentDeadline {
						want, _ := ctx.Deadline()
						if deadline != want {
							t.Fatalf("inherited deadline=%v, want exact parent %v", deadline, want)
						}
					}
				})
			}
		})
	}
}
