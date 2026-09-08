package exchange_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

const officialSDKTestTimeout = 10 * time.Second

func TestOfficialSDKResponseTransportLayerTriad(t *testing.T) {
	t.Parallel()

	const selectedPath = "/storage/v1/b/evidence/iam"
	const neutralPath = "/unselected/provider/path"
	const siblingPrefixPath = "/storage/v1/backup/evidence/iam"
	selectedBody := strings.Repeat("s", 128)
	neutralBody := strings.Repeat("n", 256)
	cases := []struct {
		wantErr       error
		name          string
		path          string
		wantBody      string
		limit         uint64
		wantCallDelta int64
		wantResponse  bool
	}{
		{name: "positive selected response at exact ceiling is released intact", path: selectedPath, limit: uint64(len(selectedBody)), wantBody: selectedBody, wantResponse: true, wantCallDelta: 1},
		{name: "negative selected response above ceiling is refused without partial response", path: selectedPath, limit: uint64(len(selectedBody) - 1), wantErr: core.ErrExchangeBodyLimit, wantCallDelta: 1},
		{name: "neutral unselected response remains SDK streaming data", path: neutralPath, limit: 1, wantBody: neutralBody, wantResponse: true, wantCallDelta: 1},
		{name: "neutral sibling path segment remains SDK streaming data", path: siblingPrefixPath, limit: 1, wantBody: neutralBody, wantResponse: true, wantCallDelta: 1},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				calls.Add(1)
				switch request.URL.Path {
				case selectedPath:
					_, _ = io.WriteString(writer, selectedBody)
				case neutralPath, siblingPrefixPath:
					_, _ = io.WriteString(writer, neutralBody)
				default:
					http.NotFound(writer, request)
				}
			}))
			t.Cleanup(server.Close)
			before := calls.Load()
			limit, limitErr := core.NewByteCount(testCase.limit)
			if limitErr != nil {
				t.Fatalf("core.NewByteCount() error = %v, want nil", limitErr)
			}
			boundary, boundaryErr := exchange.NewOfficialSDKResponseBoundary(exchange.OfficialSDKResponseBoundaryRequest{
				Method: exchange.MethodGet, PathPrefix: "/storage/v1/b", PathSuffix: "/iam",
				Representation: exchange.OfficialSDKResponseRepresentationBinary,
				MaximumBytes:   limit,
			})
			if boundaryErr != nil {
				t.Fatalf("exchange.NewOfficialSDKResponseBoundary() error = %v, want nil", boundaryErr)
			}
			transport, transportErr := exchange.NewStandardOfficialSDKResponseTransport(boundary)
			if transportErr != nil {
				t.Fatalf("exchange.NewStandardOfficialSDKResponseTransport() error = %v, want nil", transportErr)
			}
			client, clientErr := exchange.NewOfficialSDKHTTPClient(transport)
			if clientErr != nil {
				t.Fatalf("exchange.NewOfficialSDKHTTPClient() error = %v, want nil", clientErr)
			}
			response, gotErr := client.Get(server.URL + testCase.path)
			gotCallDelta := calls.Load() - before
			if testCase.wantErr != nil {
				if !errors.Is(gotErr, core.ErrExchangeResponse) || !errors.Is(gotErr, testCase.wantErr) {
					t.Fatalf("official SDK request error = %v, want %v and %v", gotErr, core.ErrExchangeResponse, testCase.wantErr)
				}
				if response != nil {
					_ = response.Body.Close()
					t.Fatalf("official SDK request response = %v, want nil", response)
				}
				if gotCallDelta != testCase.wantCallDelta {
					t.Fatalf("provider call delta = %d, want %d", gotCallDelta, testCase.wantCallDelta)
				}
				return
			}
			gotResponse := response != nil
			if gotErr != nil || gotResponse != testCase.wantResponse {
				t.Fatalf("official SDK request = (%v, %v), want response=%t and nil", response, gotErr, testCase.wantResponse)
			}
			gotBody, readErr := io.ReadAll(response.Body)
			closeErr := response.Body.Close()
			if readErr != nil || closeErr != nil {
				t.Fatalf("official SDK response read/close = (%v, %v), want nil/nil", readErr, closeErr)
			}
			if got := string(gotBody); got != testCase.wantBody {
				t.Fatalf("official SDK response body = %q, want %q", got, testCase.wantBody)
			}
			if gotCallDelta != testCase.wantCallDelta {
				t.Fatalf("provider call delta = %d, want %d", gotCallDelta, testCase.wantCallDelta)
			}
		})
	}
}

func TestOfficialSDKColonActionSuffixAppliesSelectedResponseCeiling(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, "oversized")
	}))
	t.Cleanup(server.Close)
	limit, limitErr := core.NewByteCount(1)
	if limitErr != nil {
		t.Fatalf("core.NewByteCount(1) error = %v, want nil", limitErr)
	}
	boundary, boundaryErr := exchange.NewOfficialSDKResponseBoundary(exchange.OfficialSDKResponseBoundaryRequest{
		Method: exchange.MethodPost, PathPrefix: "/v1/accounts/", PathSuffix: ":signBlob",
		Representation: exchange.OfficialSDKResponseRepresentationBinary,
		MaximumBytes:   limit,
	})
	if boundaryErr != nil {
		t.Fatalf("exchange.NewOfficialSDKResponseBoundary(:signBlob) error = %v, want nil", boundaryErr)
	}
	client := officialSDKClient(t, boundary)
	request, requestErr := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+"/v1/accounts/123:signBlob", nil)
	if requestErr != nil {
		t.Fatalf("http.NewRequestWithContext(:signBlob) error = %v, want nil", requestErr)
	}
	response, gotErr := client.Do(request)
	if response != nil {
		_ = response.Body.Close()
	}
	if response != nil || !errors.Is(gotErr, core.ErrExchangeResponse) || !errors.Is(gotErr, core.ErrExchangeBodyLimit) {
		t.Fatalf("official SDK :signBlob response = (%v, %v), want nil, %v, and %v", response, gotErr, core.ErrExchangeResponse, core.ErrExchangeBodyLimit)
	}
}

func TestOfficialSDKStreamingSuccessTransportLayerTriad(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr      error
		name         string
		query        string
		body         string
		status       int
		wantResponse bool
	}{
		{name: "positive exact media query leaves successful body streaming beyond aggregate ceiling", query: "alt=media", body: strings.Repeat("m", 257), status: http.StatusOK, wantResponse: true},
		{name: "negative media query keeps provider failure bounded for SDK error decoding", query: "alt=media", body: strings.Repeat("e", 9), status: http.StatusInternalServerError, wantErr: core.ErrExchangeBodyLimit},
		{name: "neutral JSON query remains aggregate validated response", query: "alt=json", body: `{}`, status: http.StatusOK, wantResponse: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(testCase.status)
				_, _ = io.WriteString(writer, testCase.body)
			}))
			t.Cleanup(server.Close)
			limit, limitErr := core.NewByteCount(8)
			if limitErr != nil {
				t.Fatalf("core.NewByteCount(8) error = %v, want nil", limitErr)
			}
			boundary, boundaryErr := exchange.NewOfficialSDKStreamingSuccessCeiling(
				exchange.OfficialSDKStreamingSuccessCeilingRequest{
					Method: exchange.MethodGet, StreamQueryName: "alt", StreamQueryValue: "media",
					AggregateRepresentation: exchange.OfficialSDKResponseRepresentationJSON,
					AggregateMaximumBytes:   limit,
				},
			)
			if boundaryErr != nil {
				t.Fatalf("exchange.NewOfficialSDKStreamingSuccessCeiling() error = %v, want nil", boundaryErr)
			}
			client := officialSDKClient(t, boundary)
			response, gotErr := client.Get(server.URL + "/object?" + testCase.query)
			if testCase.wantErr != nil {
				if response != nil {
					_ = response.Body.Close()
				}
				if response != nil || !errors.Is(gotErr, core.ErrExchangeResponse) || !errors.Is(gotErr, testCase.wantErr) {
					t.Fatalf("streaming-success SDK response = (%v, %v), want nil, %v, and %v", response, gotErr, core.ErrExchangeResponse, testCase.wantErr)
				}
				return
			}
			if gotErr != nil || (response != nil) != testCase.wantResponse {
				t.Fatalf("streaming-success SDK response = (%v, %v), want response=%t and nil", response, gotErr, testCase.wantResponse)
			}
			gotBody, readErr := io.ReadAll(response.Body)
			closeErr := response.Body.Close()
			if readErr != nil || closeErr != nil || string(gotBody) != testCase.body {
				t.Fatalf("streaming-success SDK body = (%d bytes, %v, %v), want exact %d bytes and nil/nil", len(gotBody), readErr, closeErr, len(testCase.body))
			}
		})
	}
}

func TestOfficialSDKHTTPClientRefusesRedirectCancellationAndTransportFailure(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                                  string
		redirect, cancelled, closedConnection bool
		wantCalls                             int64
		wantStatus                            int
		wantErr, wantNative                   error
	}{
		{name: "redirect cannot issue a second provider request", redirect: true, wantCalls: 1, wantStatus: http.StatusFound, wantErr: core.ErrExchangeRedirect},
		{name: "pre-cancelled context cannot reach provider", cancelled: true, wantErr: core.ErrExchangeCancelled, wantNative: context.Canceled},
		{name: "closed Go connection retains native transport refusal", closedConnection: true, wantErr: core.ErrExchangeTransport, wantNative: io.ErrClosedPipe},
		{name: "empty successful provider response invents no content", wantCalls: 1, wantStatus: http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			boundary, err := exchange.NewOfficialSDKResponseCeiling(exchange.OfficialSDKResponseCeilingRequest{Method: exchange.MethodGet, Representation: exchange.OfficialSDKResponseRepresentationBinary, MaximumBytes: mustByteCount(t, 128)})
			if err != nil {
				t.Fatal(err)
			}
			var calls atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if tc.redirect {
					http.Redirect(w, r, "/moved", http.StatusFound)
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()
			base := http.DefaultTransport.(*http.Transport).Clone()
			base.Proxy = nil
			defer base.CloseIdleConnections()
			if tc.closedConnection {
				base.DialContext = func(context.Context, string, string) (net.Conn, error) {
					local, peer := net.Pipe()
					err := errors.Join(local.Close(), peer.Close())
					return local, err
				}
			}
			transport, err := exchange.NewOfficialSDKResponseTransport(exchange.OfficialSDKResponseTransportRequest{Base: base, Boundary: boundary})
			if err != nil {
				t.Fatal(err)
			}
			client, err := exchange.NewOfficialSDKHTTPClient(transport)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancelled {
				cancel()
			}
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			response, gotErr := client.Do(request)
			var body []byte
			if response != nil {
				var readErr error
				body, readErr = io.ReadAll(response.Body)
				closeErr := response.Body.Close()
				if readErr != nil || closeErr != nil {
					t.Fatalf("response custody=(%v,%v), want complete read and close", readErr, closeErr)
				}
			}
			if !errors.Is(gotErr, tc.wantErr) || tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) || calls.Load() != tc.wantCalls {
				t.Fatalf("SDK request=(%v,%d calls), want (%v,%v,%d)", gotErr, calls.Load(), tc.wantErr, tc.wantNative, tc.wantCalls)
			}
			if tc.wantStatus == 0 {
				if response != nil {
					t.Fatalf("failed transport produced response %+v", response)
				}
				return
			}
			if response == nil || response.StatusCode != tc.wantStatus {
				t.Fatalf("SDK response=%+v, want status %d", response, tc.wantStatus)
			}
			if !tc.redirect && len(body) != 0 {
				t.Fatalf("empty provider produced %q", body)
			}
		})
	}
}

func TestOfficialSDKActiveReadOwnershipLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                          string
		cancelActive, cancelAfterDone bool
		wantErr, wantNative           error
		wantBytes                     int
	}{
		{name: "cancellation after response headers closes active body", cancelActive: true, wantErr: core.ErrExchangeCancelled, wantNative: context.Canceled},
		{name: "completed provider body retains exact bytes", wantBytes: 128},
		{name: "cancellation after completion cannot erase owned response", cancelAfterDone: true, wantBytes: 128},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			backstop := exchangeFixtureBackstop(t, officialSDKTestTimeout)
			started := make(chan struct{})
			release, stopHandler := context.WithCancel(t.Context())
			defer stopHandler()
			payload := strings.Repeat("p", 128)
			var calls atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) != 1 {
					return
				}
				w.Header().Set(core.HTTPHeaderContentLength().String(), strconv.Itoa(len(payload)))
				w.WriteHeader(http.StatusOK)
				if err := http.NewResponseController(w).Flush(); err != nil {
					return
				}
				close(started)
				if tc.cancelActive {
					select {
					case <-r.Context().Done():
					case <-release.Done():
					}
					return
				}
				_, _ = io.WriteString(w, payload)
			}))
			defer server.Close()
			boundary, err := exchange.NewOfficialSDKResponseCeiling(exchange.OfficialSDKResponseCeilingRequest{Method: exchange.MethodGet, Representation: exchange.OfficialSDKResponseRepresentationBinary, MaximumBytes: mustByteCount(t, uint64(len(payload)))})
			if err != nil {
				t.Fatal(err)
			}
			client := officialSDKClient(t, boundary)
			ctx, cancel := context.WithCancel(t.Context())
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
			if err != nil {
				cancel()
				t.Fatal(err)
			}
			type requestResult struct {
				response *http.Response
				err      error
			}
			var result requestResult
			done := make(chan struct{})
			// Register cancellation and join before starting the worker. A closed done
			// channel supports both the behavioral wait and the unconditional cleanup.
			defer func() {
				cancel()
				stopHandler()
				select {
				case <-done:
				case <-exchangeFixtureBackstop(t, officialSDKTestTimeout):
					t.Errorf("SDK client worker did not join during cleanup; owned completion channel=%p", done)
					return
				}
				if result.response != nil {
					if err := result.response.Body.Close(); err != nil {
						t.Errorf("SDK response cleanup close=%v", err)
					}
				}
			}()
			go func() { result.response, result.err = client.Do(request); close(done) }()
			select {
			case <-started:
			case <-backstop:
				t.Fatalf("provider headers did not reach active read; owned completion channel=%p", started)
			}
			if tc.cancelActive {
				cancel()
			}
			select {
			case <-done:
			case <-backstop:
				t.Fatalf("SDK operation did not finish; owned completion channel=%p", done)
			}
			if tc.cancelAfterDone {
				cancel()
			}
			if !errors.Is(result.err, tc.wantErr) || tc.wantNative != nil && !errors.Is(result.err, tc.wantNative) || calls.Load() != 1 {
				t.Fatalf("active SDK result=(%v,%d calls), want (%v,%v,1)", result.err, calls.Load(), tc.wantErr, tc.wantNative)
			}
			if tc.wantErr != nil {
				if result.response != nil {
					t.Fatalf("cancelled response=%v, want nil", result.response)
				}
				return
			}
			if result.response == nil || result.response.StatusCode != http.StatusOK {
				t.Fatalf("completed response=%+v, want HTTP success", result.response)
			}
			body, err := io.ReadAll(result.response.Body)
			if err != nil || len(body) != tc.wantBytes || string(body) != payload {
				t.Fatalf("completed body=(%q,%v), want exact provider bytes", body, err)
			}
		})
	}
}

func officialSDKClient(t *testing.T, boundary exchange.OfficialSDKResponseBoundary) *http.Client {
	t.Helper()
	transport, gotTransportErr := exchange.NewStandardOfficialSDKResponseTransport(boundary)
	if gotTransportErr != nil {
		t.Fatalf("exchange.NewStandardOfficialSDKResponseTransport() error = %v, want nil", gotTransportErr)
	}
	client, gotClientErr := exchange.NewOfficialSDKHTTPClient(transport)
	if gotClientErr != nil {
		t.Fatalf("exchange.NewOfficialSDKHTTPClient() error = %v, want nil", gotClientErr)
	}
	return client
}
