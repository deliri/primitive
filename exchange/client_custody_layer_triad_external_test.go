package exchange_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// clientCustody is the compiler-visible projection of every caller-owned
// net/http client field Exchange is forbidden to change.
type clientCustody struct {
	transport     http.RoundTripper
	jar           http.CookieJar
	checkRedirect uintptr
	timeout       time.Duration
}

func observeClientCustody(client *http.Client) clientCustody {
	custody := clientCustody{
		transport: client.Transport,
		jar:       client.Jar,
		timeout:   client.Timeout,
	}
	if client.CheckRedirect != nil {
		custody.checkRedirect = reflect.ValueOf(client.CheckRedirect).Pointer()
	}
	return custody
}

// TestNewStandardClientProducesTheShapeNewClientDemands proves the produced
// default is exactly an admitted standard client: valid, timeout-free, and
// indistinguishable from admitting an empty net/http literal by hand.
func TestNewStandardClientProducesTheShapeNewClientDemands(t *testing.T) {
	t.Parallel()

	got, gotErr := exchange.NewStandardClient()
	if gotErr != nil {
		t.Fatalf("exchange.NewStandardClient() error = %v, want nil", gotErr)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("NewStandardClient().Validate() = %v, want nil", err)
	}
	want, wantErr := exchange.NewClient(&http.Client{})
	if wantErr != nil {
		t.Fatalf("exchange.NewClient(empty literal) error = %v, want nil", wantErr)
	}
	if err := want.Validate(); err != nil {
		t.Fatalf("hand-admitted client Validate() = %v, want nil", err)
	}
}

func TestClientTimeoutOwnershipLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive a client without a competing timeout is admitted", func(t *testing.T) {
		t.Parallel()

		client := &http.Client{}
		got, gotErr := exchange.NewClient(client)
		if gotErr != nil {
			t.Fatalf("exchange.NewClient(zero timeout) error = %v, want nil", gotErr)
		}
		if gotValidateErr := got.Validate(); gotValidateErr != nil {
			t.Fatalf("exchange.Client.Validate() = %v, want nil", gotValidateErr)
		}
		if client.Timeout != 0 {
			t.Fatalf("caller http.Client.Timeout = %v, want 0", client.Timeout)
		}
	})

	t.Run("negative a competing client timeout is refused so one layer owns budgets", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			timeout time.Duration
		}{
			{name: "one nanosecond timeout", timeout: time.Nanosecond},
			{name: "one second timeout", timeout: time.Second},
			{name: "one hour timeout", timeout: time.Hour},
			{name: "negative timeout", timeout: -time.Second},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				client := &http.Client{Timeout: tc.timeout}
				got, gotErr := exchange.NewClient(client)
				if !errors.Is(gotErr, core.ErrExchangeContract) {
					t.Fatalf(
						"exchange.NewClient(timeout %v) error = %v, want %v",
						tc.timeout,
						gotErr,
						core.ErrExchangeContract,
					)
				}
				if got != (exchange.Client{}) {
					t.Fatalf("exchange.NewClient(timeout %v) value = %+v, want zero", tc.timeout, got)
				}
				if client.Timeout != tc.timeout {
					t.Fatalf(
						"refused caller http.Client.Timeout = %v, want unchanged %v",
						client.Timeout,
						tc.timeout,
					)
				}
			})
		}
	})

	t.Run("neutral an absent client is refused without creating one", func(t *testing.T) {
		t.Parallel()

		got, gotErr := exchange.NewClient(nil)
		if !errors.Is(gotErr, core.ErrExchangeContract) {
			t.Fatalf("exchange.NewClient(nil) error = %v, want %v", gotErr, core.ErrExchangeContract)
		}
		if got != (exchange.Client{}) {
			t.Fatalf("exchange.NewClient(nil) value = %+v, want zero", got)
		}
		if gotValidateErr := got.Validate(); !errors.Is(gotValidateErr, core.ErrExchangeContract) {
			t.Fatalf(
				"zero exchange.Client.Validate() = %v, want %v",
				gotValidateErr,
				core.ErrExchangeContract,
			)
		}
	})
}

func TestCallerClientImmutabilityLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive a redirected operation leaves every caller client field unchanged", func(t *testing.T) {
		t.Parallel()

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
				_, _ = writer.Write([]byte("arrived"))
				return
			}
			writer.Header().Set(locationHeaderName, "/final")
			writer.WriteHeader(http.StatusTemporaryRedirect)
		}))
		defer server.Close()

		client := server.Client()
		want := observeClientCustody(client)
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
		if gotErr != nil || string(got.Body) != "arrived" {
			t.Fatalf(
				"redirected operation = (%q, %v), want (%q, nil)",
				got.Body,
				gotErr,
				"arrived",
			)
		}
		if finalCalls.Load() != 1 {
			t.Fatalf("redirect target calls = %d, want 1", finalCalls.Load())
		}
		if gotCustody := observeClientCustody(client); gotCustody != want {
			t.Fatalf("caller http.Client custody = %+v, want unchanged %+v", gotCustody, want)
		}
		if client.CheckRedirect != nil {
			t.Fatalf(
				"caller http.Client.CheckRedirect = %p, want nil",
				client.CheckRedirect,
			)
		}
	})

	t.Run("negative a caller redirect decision never governs an Exchange operation", func(t *testing.T) {
		t.Parallel()

		var callerDecisions atomic.Uint64
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
			writer.Header().Set(locationHeaderName, target.URL)
			writer.WriteHeader(http.StatusTemporaryRedirect)
		}))
		defer origin.Close()

		client := origin.Client()
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			callerDecisions.Add(1)
			return nil
		}
		want := observeClientCustody(client)
		_, gotErr := exchange.SendNoBodyBounded(
			exchange.NoBodyBoundedCall{
				Context: context.Background(),
				Client:  mustExchangeClient(t, client),
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
			t.Fatalf(
				"cross-origin redirect under a permissive caller decision = %v, want %v",
				gotErr,
				core.ErrExchangeRedirect,
			)
		}
		if callerDecisions.Load() != 0 || targetCalls.Load() != 0 {
			t.Fatalf(
				"caller decisions/target calls = (%d, %d), want (0, 0)",
				callerDecisions.Load(),
				targetCalls.Load(),
			)
		}
		if gotCustody := observeClientCustody(client); gotCustody != want {
			t.Fatalf("caller http.Client custody = %+v, want unchanged %+v", gotCustody, want)
		}
	})

	t.Run("neutral a completed operation restores nothing because it changed nothing", func(t *testing.T) {
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

		client := server.Client()
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}
		want := observeClientCustody(client)
		exchangeClient := mustExchangeClient(t, client)
		for attempt := 1; attempt <= 3; attempt++ {
			_, gotErr := exchange.SendNoBodyBounded(
				exchange.NoBodyBoundedCall{
					Context: context.Background(),
					Client:  exchangeClient,
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
			if gotErr != nil {
				t.Fatalf("repeated operation %d error = %v, want nil", attempt, gotErr)
			}
			if gotCustody := observeClientCustody(client); gotCustody != want {
				t.Fatalf(
					"caller http.Client custody after operation %d = %+v, want unchanged %+v",
					attempt,
					gotCustody,
					want,
				)
			}
		}
	})
}
