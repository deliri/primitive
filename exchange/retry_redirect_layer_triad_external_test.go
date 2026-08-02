package exchange_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestRetryTransportLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("retryable proxy representation cannot preempt status retry", func(t *testing.T) {
		t.Parallel()

		var attempts atomic.Uint64
		ok := mustHTTPStatus(t, http.StatusOK)
		server := httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			_ *http.Request,
		) {
			current := attempts.Add(1)
			if current == 1 {
				writer.Header().Set(
					core.HTTPHeaderContentType().String(),
					mustHTTPMediaType(t, "text/plain").String(),
				)
				writer.WriteHeader(http.StatusServiceUnavailable)
				_, _ = writer.Write([]byte("proxy unavailable"))
				return
			}
			writer.Header().Set(
				core.HTTPHeaderContentType().String(),
				mustHTTPMediaType(t, "application/json").String(),
			)
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte(`{"ready":true}`))
		}))
		defer server.Close()

		got, gotErr := exchange.SendNoBodyBounded(
			exchange.NoBodyBoundedCall{
				Context: context.Background(),
				Client:  mustExchangeClient(t, server.Client()),
				Request: exchange.NoBodyBoundedRequest{
					Target: mustEndpoint(t, server.URL),
					Semantics: exchange.RequestSemantics{
						Method: exchange.MethodGet,
						Replay: exchange.ReplaySafe,
					},
					ExpectedResponseContentType: mustHTTPMediaType(t, "application/json"),
					ExpectedStatus:              ok,
				},
				Policy: exchange.NoBodyBoundedPolicy{
					Operation:         retryOperationPolicy(t, 2),
					ResponseBodyLimit: mustByteCount(t, 4*1024),
				},
			},
		)
		if gotErr != nil {
			t.Fatalf("exchange.SendNoBodyBounded() error = %v, want nil", gotErr)
		}
		if string(got.Body) != `{"ready":true}` ||
			got.Metadata.Attempts != 2 ||
			attempts.Load() != 2 {
			t.Fatalf(
				"proxy retry result body/metadata/server attempts = (%q, %d, %d), want (%q, 2, 2)",
				got.Body,
				got.Metadata.Attempts,
				attempts.Load(),
				`{"ready":true}`,
			)
		}
	})

	t.Run("positive Retry-After is bounded and a safe request succeeds on its second attempt", func(t *testing.T) {
		t.Parallel()

		var attempts atomic.Uint64
		ok := mustHTTPStatus(t, http.StatusOK)
		server := httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			_ *http.Request,
		) {
			current := attempts.Add(1)
			writer.Header().Set(
				core.HTTPHeaderContentType().String(),
				mustHTTPMediaType(t, "text/plain").String(),
			)
			if current == 1 {
				writer.Header().Set(
					retryAfterHeaderName,
					"999999999",
				)
				writer.WriteHeader(http.StatusServiceUnavailable)
				_, _ = writer.Write([]byte("busy"))
				return
			}
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte("ready"))
		}))
		defer server.Close()

		got, gotErr := exchange.SendNoBodyBounded(
			exchange.NoBodyBoundedCall{
				Context: context.Background(),
				Client:  mustExchangeClient(t, server.Client()),
				Request: exchange.NoBodyBoundedRequest{
					Target: mustEndpoint(t, server.URL),
					Semantics: exchange.RequestSemantics{
						Method: exchange.MethodGet,
						Replay: exchange.ReplaySafe,
					},
					ExpectedResponseContentType: mustHTTPMediaType(t, "text/plain"),
					ExpectedStatus:              ok,
				},
				Policy: exchange.NoBodyBoundedPolicy{
					Operation:         retryOperationPolicy(t, 2),
					ResponseBodyLimit: mustByteCount(t, 4*1024),
				},
			},
		)
		if gotErr != nil {
			t.Fatalf("exchange.SendNoBodyBounded() error = %v, want nil", gotErr)
		}
		if string(got.Body) != "ready" ||
			got.Metadata.Attempts != 2 ||
			attempts.Load() != 2 {
			t.Fatalf(
				"retry result body/metadata/server attempts = (%q, %d, %d), want (%q, 2, 2)",
				got.Body,
				got.Metadata.Attempts,
				attempts.Load(),
				"ready",
			)
		}
	})

	t.Run("negative final retryable status carries exhaustion and status identities", func(t *testing.T) {
		t.Parallel()

		var attempts atomic.Uint64
		ok := mustHTTPStatus(t, http.StatusOK)
		unavailable := mustHTTPStatus(t, http.StatusServiceUnavailable)
		server := httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			_ *http.Request,
		) {
			attempts.Add(1)
			writer.Header().Set(
				core.HTTPHeaderContentType().String(),
				mustHTTPMediaType(t, "text/plain").String(),
			)
			writer.WriteHeader(http.StatusServiceUnavailable)
			_, _ = writer.Write([]byte("still busy"))
		}))
		defer server.Close()

		got, gotErr := exchange.SendNoBodyBounded(
			exchange.NoBodyBoundedCall{
				Context: context.Background(),
				Client:  mustExchangeClient(t, server.Client()),
				Request: exchange.NoBodyBoundedRequest{
					Target: mustEndpoint(t, server.URL),
					Semantics: exchange.RequestSemantics{
						Method: exchange.MethodGet,
						Replay: exchange.ReplaySafe,
					},
					ExpectedResponseContentType: mustHTTPMediaType(t, "text/plain"),
					ExpectedStatus:              ok,
				},
				Policy: exchange.NoBodyBoundedPolicy{
					Operation:         retryOperationPolicy(t, 2),
					ResponseBodyLimit: mustByteCount(t, 4*1024),
				},
			},
		)
		if !errors.Is(gotErr, core.ErrExchangeRetryExhausted) ||
			!errors.Is(gotErr, core.ErrExchangeResponse) {
			t.Fatalf(
				"exchange.SendNoBodyBounded(exhausted) error = %v, want %v and %v",
				gotErr,
				core.ErrExchangeRetryExhausted,
				core.ErrExchangeResponse,
			)
		}
		var exhausted exchange.RetryExhaustedError
		if !errors.As(gotErr, &exhausted) || exhausted.Attempts() != 2 {
			t.Fatalf(
				"retry exhaustion = (%v, %d), want typed error with 2 attempts",
				gotErr,
				exhausted.Attempts(),
			)
		}
		if !errors.Is(exhausted.Cause(), core.ErrExchangeResponse) {
			t.Fatalf(
				"retry exhaustion cause = %v, want %v",
				exhausted.Cause(),
				core.ErrExchangeResponse,
			)
		}
		if gotDiagnostic := exhausted.Error(); !strings.Contains(
			gotDiagnostic,
			strconv.FormatUint(exhausted.Attempts(), 10),
		) {
			t.Fatalf(
				"retry exhaustion diagnostic = %q, want attempt count %d",
				gotDiagnostic,
				exhausted.Attempts(),
			)
		}
		var statusErr exchange.StatusError
		if !errors.As(gotErr, &statusErr) ||
			statusErr.Status() != unavailable ||
			statusErr.Expected() != ok {
			t.Fatalf(
				"status error = (%v, %v, %v), want (%v, %v)",
				statusErr,
				statusErr.Status(),
				statusErr.Expected(),
				unavailable,
				ok,
			)
		}
		status, _ := statusErr.Status().Int()
		wantStatus, _ := statusErr.Expected().Int()
		if gotDiagnostic := statusErr.Error(); !strings.Contains(gotDiagnostic, strconv.Itoa(status)) ||
			!strings.Contains(gotDiagnostic, strconv.Itoa(wantStatus)) {
			t.Fatalf(
				"status diagnostic = %q, want status %d and configured status %d",
				gotDiagnostic,
				status,
				wantStatus,
			)
		}
		if got.Metadata.Attempts != 2 ||
			attempts.Load() != 2 ||
			string(got.Body) != "still busy" {
			t.Fatalf(
				"exhausted result = (%d attempts, %d server, %q body), want (2, 2, %q)",
				got.Metadata.Attempts,
				attempts.Load(),
				got.Body,
				"still busy",
			)
		}
	})

	t.Run("neutral single-attempt request creates no retry or exhaustion state", func(t *testing.T) {
		t.Parallel()

		var attempts atomic.Uint64
		ok := mustHTTPStatus(t, http.StatusOK)
		server := httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			_ *http.Request,
		) {
			attempts.Add(1)
			writer.Header().Set(
				core.HTTPHeaderContentType().String(),
				mustHTTPMediaType(t, "text/plain").String(),
			)
			writer.WriteHeader(http.StatusServiceUnavailable)
			_, _ = writer.Write([]byte("once"))
		}))
		defer server.Close()

		got, gotErr := exchange.SendNoBodyBounded(
			exchange.NoBodyBoundedCall{
				Context: context.Background(),
				Client:  mustExchangeClient(t, server.Client()),
				Request: exchange.NoBodyBoundedRequest{
					Target: mustEndpoint(t, server.URL),
					Semantics: exchange.RequestSemantics{
						Method: exchange.MethodGet,
						Replay: exchange.ReplaySingleAttempt,
					},
					ExpectedResponseContentType: mustHTTPMediaType(t, "text/plain"),
					ExpectedStatus:              ok,
				},
				Policy: exchange.NoBodyBoundedPolicy{
					Operation:         singleAttemptOperationPolicy(t),
					ResponseBodyLimit: mustByteCount(t, 4*1024),
				},
			},
		)
		if !errors.Is(gotErr, core.ErrExchangeResponse) ||
			errors.Is(gotErr, core.ErrExchangeRetryExhausted) {
			t.Fatalf(
				"single-attempt status error = %v, want response without %v",
				gotErr,
				core.ErrExchangeRetryExhausted,
			)
		}
		if attempts.Load() != 1 || got.Metadata.Attempts != 1 {
			t.Fatalf(
				"single-attempt counts = (%d server, %d result), want (1, 1)",
				attempts.Load(),
				got.Metadata.Attempts,
			)
		}
	})
}

func TestRedirectTransportLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive relative same-origin redirect preserves method and caller client", func(t *testing.T) {
		t.Parallel()

		var originalRedirectCalls atomic.Uint64
		var finalCalls atomic.Uint64
		ok := mustHTTPStatus(t, http.StatusOK)
		server := httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			if request.URL.Path == "/final" {
				finalCalls.Add(1)
				writer.Header().Set(
					core.HTTPHeaderContentType().String(),
					mustHTTPMediaType(t, "text/plain").String(),
				)
				writer.WriteHeader(http.StatusOK)
				_, _ = writer.Write([]byte(request.Method))
				return
			}
			writer.Header().Set(
				locationHeaderName,
				"/final",
			)
			writer.WriteHeader(http.StatusTemporaryRedirect)
		}))
		defer server.Close()

		client := server.Client()
		client.CheckRedirect = func(
			_ *http.Request,
			_ []*http.Request,
		) error {
			originalRedirectCalls.Add(1)
			return http.ErrUseLastResponse
		}
		got, gotErr := exchange.SendNoBodyBounded(
			exchange.NoBodyBoundedCall{
				Context: context.Background(),
				Client:  mustExchangeClient(t, client),
				Request: exchange.NoBodyBoundedRequest{
					Target: mustEndpoint(t, server.URL),
					Semantics: exchange.RequestSemantics{
						Method: exchange.MethodGet,
						Replay: exchange.ReplaySingleAttempt,
					},
					ExpectedResponseContentType: mustHTTPMediaType(t, "text/plain"),
					ExpectedStatus:              ok,
				},
				Policy: exchange.NoBodyBoundedPolicy{
					Operation:         redirectOperationPolicy(t),
					ResponseBodyLimit: mustByteCount(t, 4*1024),
				},
			},
		)
		if gotErr != nil || string(got.Body) != exchange.MethodGet.String() {
			t.Fatalf(
				"same-origin redirect result = (%q, %v), want (%q, nil)",
				got.Body,
				gotErr,
				exchange.MethodGet,
			)
		}
		if finalCalls.Load() != 1 || originalRedirectCalls.Load() != 0 {
			t.Fatalf(
				"redirect callbacks = (%d final, %d original), want (1, 0)",
				finalCalls.Load(),
				originalRedirectCalls.Load(),
			)
		}
	})

	t.Run("negative cross-origin redirect is rejected before target transmission", func(t *testing.T) {
		t.Parallel()

		var targetCalls atomic.Uint64
		ok := mustHTTPStatus(t, http.StatusOK)
		target := httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			_ *http.Request,
		) {
			targetCalls.Add(1)
			writer.WriteHeader(http.StatusOK)
		}))
		defer target.Close()
		origin := httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			_ *http.Request,
		) {
			writer.Header().Set(
				locationHeaderName,
				target.URL,
			)
			writer.WriteHeader(http.StatusTemporaryRedirect)
		}))
		defer origin.Close()

		got, gotErr := exchange.SendNoBodyBounded(
			exchange.NoBodyBoundedCall{
				Context: context.Background(),
				Client:  mustExchangeClient(t, origin.Client()),
				Request: exchange.NoBodyBoundedRequest{
					Target: mustEndpoint(t, origin.URL),
					Semantics: exchange.RequestSemantics{
						Method: exchange.MethodGet,
						Replay: exchange.ReplaySingleAttempt,
					},
					ExpectedStatus: ok,
				},
				Policy: exchange.NoBodyBoundedPolicy{
					Operation:         redirectOperationPolicy(t),
					ResponseBodyLimit: mustByteCount(t, 4*1024),
				},
			},
		)
		if !errors.Is(gotErr, core.ErrExchangeRedirect) {
			t.Fatalf("cross-origin redirect error = %v, want %v", gotErr, core.ErrExchangeRedirect)
		}
		if targetCalls.Load() != 0 {
			t.Fatalf("cross-origin target calls = %d, want 0", targetCalls.Load())
		}
		if got.Metadata.Attempts != 0 || len(got.Body) != 0 {
			t.Fatalf("cross-origin redirect response = %+v, want zero", got)
		}
	})

	t.Run("negative same-origin credential redirect is rejected before authorization synthesis", func(t *testing.T) {
		t.Parallel()

		var targetCalls atomic.Uint64
		ok := mustHTTPStatus(t, http.StatusOK)
		var server *httptest.Server
		server = httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			if request.URL.Path == "/final" {
				targetCalls.Add(1)
				writer.WriteHeader(http.StatusOK)
				return
			}
			redirect, parseErr := url.Parse(server.URL)
			if parseErr != nil {
				writer.WriteHeader(http.StatusInternalServerError)
				return
			}
			redirect.Path = "/final"
			redirect.User = url.UserPassword("attacker", "secret")
			writer.Header().Set(
				locationHeaderName,
				redirect.String(),
			)
			writer.WriteHeader(http.StatusTemporaryRedirect)
		}))
		defer server.Close()

		got, gotErr := exchange.SendNoBodyBounded(
			exchange.NoBodyBoundedCall{
				Context: context.Background(),
				Client:  mustExchangeClient(t, server.Client()),
				Request: exchange.NoBodyBoundedRequest{
					Target: mustEndpoint(t, server.URL),
					Semantics: exchange.RequestSemantics{
						Method: exchange.MethodGet,
						Replay: exchange.ReplaySingleAttempt,
					},
					ExpectedStatus: ok,
				},
				Policy: exchange.NoBodyBoundedPolicy{
					Operation:         redirectOperationPolicy(t),
					ResponseBodyLimit: mustByteCount(t, 4*1024),
				},
			},
		)
		if !errors.Is(gotErr, core.ErrExchangeRedirect) {
			t.Fatalf(
				"same-origin credential redirect error = %v, want %v",
				gotErr,
				core.ErrExchangeRedirect,
			)
		}
		if targetCalls.Load() != 0 {
			t.Fatalf(
				"same-origin credential redirect target calls = %d, want 0",
				targetCalls.Load(),
			)
		}
		if got.Metadata.Attempts != 0 || len(got.Body) != 0 {
			t.Fatalf("same-origin credential redirect response = %+v, want zero", got)
		}
	})

	t.Run("neutral reject policy changes nothing when no redirect occurs", func(t *testing.T) {
		t.Parallel()

		ok := mustHTTPStatus(t, http.StatusOK)
		server := httptest.NewServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			_ *http.Request,
		) {
			writer.Header().Set(
				core.HTTPHeaderContentType().String(),
				mustHTTPMediaType(t, "text/plain").String(),
			)
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte("direct"))
		}))
		defer server.Close()

		got, gotErr := exchange.SendNoBodyBounded(
			exchange.NoBodyBoundedCall{
				Context: context.Background(),
				Client:  mustExchangeClient(t, server.Client()),
				Request: exchange.NoBodyBoundedRequest{
					Target: mustEndpoint(t, server.URL),
					Semantics: exchange.RequestSemantics{
						Method: exchange.MethodGet,
						Replay: exchange.ReplaySingleAttempt,
					},
					ExpectedResponseContentType: mustHTTPMediaType(t, "text/plain"),
					ExpectedStatus:              ok,
				},
				Policy: exchange.NoBodyBoundedPolicy{
					Operation:         singleAttemptOperationPolicy(t),
					ResponseBodyLimit: mustByteCount(t, 4*1024),
				},
			},
		)
		if gotErr != nil || string(got.Body) != "direct" {
			t.Fatalf(
				"direct reject-policy result = (%q, %v), want (%q, nil)",
				got.Body,
				gotErr,
				"direct",
			)
		}
	})
}

func retryOperationPolicy(
	t *testing.T,
	maximumAttempts uint64,
) exchange.OperationPolicy {
	t.Helper()

	return exchange.OperationPolicy{
		OperationTimeout: mustDurationMilliseconds(t, 5_000),
		AttemptTimeout:   mustDurationMilliseconds(t, 1_000),
		Retry: exchange.RetryPolicy{
			BaseDelay:         mustDurationMilliseconds(t, 1),
			MaximumDelay:      mustDurationMilliseconds(t, 1),
			MaximumJitter:     mustDurationMilliseconds(t, 1),
			MaximumRetryAfter: mustDurationMilliseconds(t, 1),
			MaximumWait:       mustDurationMilliseconds(t, 10),
			MaximumAttempts:   maximumAttempts,
		},
		Redirect: exchange.RedirectPolicy{
			Mode: exchange.RedirectReject,
		},
	}
}

func redirectOperationPolicy(t *testing.T) exchange.OperationPolicy {
	t.Helper()

	policy := singleAttemptOperationPolicy(t)
	policy.Redirect = exchange.RedirectPolicy{
		Mode:        exchange.RedirectSameOrigin,
		MaximumHops: 1,
	}
	return policy
}
