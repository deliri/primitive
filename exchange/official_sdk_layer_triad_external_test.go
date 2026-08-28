package exchange_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
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
	selectedBody := strings.Repeat("s", 128)
	neutralBody := strings.Repeat("n", 256)
	cases := []struct {
		name          string
		path          string
		limit         uint64
		wantBody      string
		wantErr       error
		wantResponse  bool
		wantCallDelta int64
	}{
		{name: "positive selected response at exact ceiling is released intact", path: selectedPath, limit: uint64(len(selectedBody)), wantBody: selectedBody, wantResponse: true, wantCallDelta: 1},
		{name: "negative selected response above ceiling is refused without partial response", path: selectedPath, limit: uint64(len(selectedBody) - 1), wantErr: core.ErrExchangeBodyLimit, wantCallDelta: 1},
		{name: "neutral unselected response remains SDK streaming data", path: neutralPath, limit: 1, wantBody: neutralBody, wantResponse: true, wantCallDelta: 1},
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
				case neutralPath:
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
				Method: exchange.MethodGet, PathPrefix: "/storage/v1/b/", PathSuffix: "/iam",
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

func TestOfficialSDKHTTPClientRefusesRedirectCancellationAndTransportFailure(t *testing.T) {
	t.Parallel()

	limit, limitErr := core.NewByteCount(128)
	if limitErr != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", limitErr)
	}
	boundary, boundaryErr := exchange.NewOfficialSDKResponseCeiling(exchange.OfficialSDKResponseCeilingRequest{
		Method: exchange.MethodGet, Representation: exchange.OfficialSDKResponseRepresentationBinary,
		MaximumBytes: limit,
	})
	if boundaryErr != nil {
		t.Fatalf("exchange.NewOfficialSDKResponseCeiling() error = %v, want nil", boundaryErr)
	}

	t.Run("redirect is refused before a second provider request", func(t *testing.T) {
		t.Parallel()

		var calls atomic.Int64
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			calls.Add(1)
			http.Redirect(writer, request, "/moved", http.StatusFound)
		}))
		t.Cleanup(server.Close)
		client := officialSDKClient(t, boundary)
		response, gotErr := client.Get(server.URL + "/start")
		if !errors.Is(gotErr, core.ErrExchangeRedirect) || response == nil || response.StatusCode != http.StatusFound {
			if response != nil {
				if closeErr := response.Body.Close(); closeErr != nil {
					t.Errorf("redirect response close error = %v, want nil", closeErr)
				}
			}
			t.Fatalf("redirect request = (%v, %v), want refused 302 response and %v", response, gotErr, core.ErrExchangeRedirect)
		}
		if closeErr := response.Body.Close(); closeErr != nil {
			t.Fatalf("redirect response close error = %v, want nil", closeErr)
		}
		if got := calls.Load(); got != 1 {
			t.Fatalf("provider calls = %d, want 1", got)
		}
	})

	t.Run("pre-cancelled context reaches typed cancellation and no provider", func(t *testing.T) {
		t.Parallel()

		var calls atomic.Int64
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			calls.Add(1)
			writer.WriteHeader(http.StatusNoContent)
		}))
		t.Cleanup(server.Close)
		client := officialSDKClient(t, boundary)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		request, requestErr := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
		if requestErr != nil {
			t.Fatalf("http.NewRequestWithContext() error = %v, want nil", requestErr)
		}
		response, gotErr := client.Do(request)
		if response != nil {
			_ = response.Body.Close()
		}
		if !errors.Is(gotErr, core.ErrExchangeCancelled) || !errors.Is(gotErr, context.Canceled) || response != nil {
			t.Fatalf("cancelled request = (%v, %v), want nil, %v, and %v", response, gotErr, core.ErrExchangeCancelled, context.Canceled)
		}
		if got := calls.Load(); got != 0 {
			t.Fatalf("provider calls = %d, want 0", got)
		}
	})

	t.Run("closed real transport preserves typed and native failure", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusNoContent)
		}))
		endpoint := server.URL
		server.Close()
		client := officialSDKClient(t, boundary)
		response, gotErr := client.Get(endpoint)
		if response != nil {
			_ = response.Body.Close()
		}
		if !errors.Is(gotErr, core.ErrExchangeTransport) || response != nil {
			t.Fatalf("closed-provider request = (%v, %v), want nil and %v", response, gotErr, core.ErrExchangeTransport)
		}
	})

	t.Run("cancellation during a provider body read closes the response and returns typed cancellation", func(t *testing.T) {
		t.Parallel()

		started := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Set("Content-Length", "128")
			writer.WriteHeader(http.StatusOK)
			flusher, ok := writer.(http.Flusher)
			if !ok {
				t.Errorf("provider writer implements http.Flusher = false, want true")
				return
			}
			flusher.Flush()
			close(started)
			<-request.Context().Done()
		}))
		t.Cleanup(server.Close)
		client := officialSDKClient(t, boundary)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		request, requestErr := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
		if requestErr != nil {
			t.Fatalf("http.NewRequestWithContext() error = %v, want nil", requestErr)
		}
		type requestResult struct {
			response *http.Response
			err      error
		}
		done := make(chan requestResult, 1)
		go func() {
			response, gotErr := client.Do(request)
			done <- requestResult{response: response, err: gotErr}
		}()

		select {
		case <-started:
			cancel()
		case <-time.After(officialSDKTestTimeout):
			cancel()
			t.Fatal("provider body read started = false, want true before timeout")
		}
		select {
		case got := <-done:
			if got.response != nil {
				if closeErr := got.response.Body.Close(); closeErr != nil {
					t.Errorf("cancelled in-flight response close error = %v, want nil", closeErr)
				}
			}
			if got.response != nil || !errors.Is(got.err, core.ErrExchangeCancelled) ||
				!errors.Is(got.err, context.Canceled) {
				t.Fatalf("in-flight cancelled SDK exchange = (%v, %v), want nil plus typed and native cancellation identities", got.response, got.err)
			}
		case <-time.After(officialSDKTestTimeout):
			cancel()
			t.Fatal("cancelled provider body read returned = false, want true before timeout")
		}
	})
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
