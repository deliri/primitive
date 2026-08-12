package exchange_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// TestHTTPHeaderValueGrammarLayerTriad pins the field-value grammar that every
// typed header owes net/http. A value net/http refuses to carry is a
// permanent caller contract violation; admitting it here would defer the
// rejection into an opaque transport failure that the retry classifier treats
// as retryable.
func TestHTTPHeaderValueGrammarLayerTriad(t *testing.T) {
	t.Parallel()

	name := mustHeaderName(t, "X-Exchange-Trace")

	t.Run("positive every transmittable field byte is admitted", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name  string
			value string
		}{
			{name: "empty value is a present field with no content", value: ""},
			{name: "horizontal tab is admitted field content", value: "a\tb"},
			{name: "space is admitted field content", value: "a b"},
			{name: "the exact lowest visible byte is admitted", value: "\x20"},
			{name: "one above the lowest visible byte is admitted", value: "\x21"},
			{name: "the exact highest visible ASCII byte is admitted", value: "\x7e"},
			{name: "one above DEL is obs-text and is admitted", value: "\x80"},
			{name: "the maximum byte is obs-text and is admitted", value: "\xff"},
			{name: "UTF-8 content is obs-text and is admitted", value: "grüße"},
			{name: "every visible ASCII byte together is admitted", value: visibleASCII()},
			{name: "leading and trailing space is admitted", value: " padded "},
			{name: "the exact value byte bound is admitted", value: strings.Repeat("a", exchange.HeaderValueMaximumBytes)},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				value := mustHeaderValue(t, tc.value)

				request := exchange.Headers{
					Values: []exchange.Header{{Name: name, Values: []exchange.HeaderValue{value}}},
				}
				if gotErr := request.Validate(); gotErr != nil {
					t.Fatalf("Headers.Validate() error = %v, want nil", gotErr)
				}
				response := exchange.ResponseHeaders{
					Values: []exchange.Header{{Name: name, Values: []exchange.HeaderValue{value}}},
				}
				if gotErr := response.Validate(); gotErr != nil {
					t.Fatalf("ResponseHeaders.Validate() error = %v, want nil", gotErr)
				}
				captured := exchange.CapturedHeaders{
					Values: []exchange.Header{{Name: name, Values: []exchange.HeaderValue{value}}},
				}
				if gotErr := captured.Validate(); gotErr != nil {
					t.Fatalf("CapturedHeaders.Validate() error = %v, want nil", gotErr)
				}
			})
		}
	})

	t.Run("negative every untransmittable field byte is refused", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name  string
			value string
		}{
			{name: "NUL is refused", value: "a\x00b"},
			{name: "the exact lowest control byte is refused", value: "\x00"},
			{name: "one below horizontal tab is refused", value: "\x08"},
			{name: "one above horizontal tab is refused as bare line feed", value: "\n"},
			{name: "vertical tab is refused", value: "a\x0bb"},
			{name: "form feed is refused", value: "a\x0cb"},
			{name: "carriage return is refused", value: "a\rb"},
			{name: "one below the lowest visible byte is refused", value: "\x1f"},
			{name: "escape is refused", value: "a\x1bb"},
			{name: "DEL is refused", value: "a\x7fb"},
			{name: "the exact DEL byte alone is refused", value: "\x7f"},
			{name: "CRLF header injection is refused", value: "safe\r\nX-Injected: yes"},
			{name: "a control byte hidden after admitted content is refused", value: strings.Repeat("a", 512) + "\x01"},
			{name: "one byte above the value bound is refused", value: strings.Repeat("a", exchange.HeaderValueMaximumBytes+1)},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				got, gotErr := exchange.NewHeaderValue(tc.value)
				if !errors.Is(gotErr, core.ErrExchangeContract) || got != (exchange.HeaderValue{}) {
					t.Fatalf(
						"NewHeaderValue(%d bytes) = (%v, %v), want zero and errors.Is %v",
						len(tc.value), got, gotErr, core.ErrExchangeContract,
					)
				}
			})
		}
	})

	t.Run("neutral admitted obs-text survives captured projection validation", func(t *testing.T) {
		t.Parallel()

		captured := exchange.CapturedHeaders{
			Values: []exchange.Header{{Name: name, Values: []exchange.HeaderValue{mustHeaderValue(t, "observed\x80")}}},
		}
		if gotErr := captured.Validate(); gotErr != nil {
			t.Fatalf("CapturedHeaders.Validate() error = %v, want nil", gotErr)
		}
	})
}

func visibleASCII() string {
	var builder strings.Builder
	for value := 0x20; value <= 0x7e; value++ {
		builder.WriteByte(byte(value))
	}
	return builder.String()
}

// TestUntransmittableHeaderSpendsNoAttempt proves the production consequence
// of the grammar gate: an impossible request is refused before the first
// attempt instead of consuming every permitted replay.
func TestUntransmittableHeaderSpendsNoAttempt(t *testing.T) {
	t.Parallel()

	var serverHits atomic.Uint64
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		serverHits.Add(1)
		writer.Header().Set(
			core.HTTPHeaderContentType().String(),
			mustHTTPMediaType(t, "text/plain").String(),
		)
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("ready"))
	}))
	defer server.Close()

	ok := mustHTTPStatus(t, http.StatusOK)
	name := mustHeaderName(t, "X-Exchange-Trace")
	invalid, invalidErr := exchange.NewHeaderValue("a\x00b")
	if !errors.Is(invalidErr, core.ErrExchangeContract) || invalid != (exchange.HeaderValue{}) {
		t.Fatalf("NewHeaderValue(NUL) = (%v, %v), want zero and errors.Is %v",
			invalid, invalidErr, core.ErrExchangeContract)
	}
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
				Headers: exchange.Headers{
					Values: []exchange.Header{{
						Name: name, Values: []exchange.HeaderValue{invalid},
					}},
				},
				ExpectedResponseContentType: mustHTTPMediaType(t, "text/plain"),
				ExpectedStatus:              ok,
			},
			Policy: exchange.NoBodyBoundedPolicy{
				Operation:         retryOperationPolicy(t, 5),
				ResponseBodyLimit: mustByteCount(t, 4*1024),
			},
		},
	)
	if !errors.Is(gotErr, core.ErrExchangeRequest) {
		t.Fatalf(
			"SendNoBodyBounded() error = %v, want %v",
			gotErr,
			core.ErrExchangeRequest,
		)
	}
	var exhausted exchange.RetryExhaustedError
	if errors.As(gotErr, &exhausted) {
		t.Fatalf(
			"SendNoBodyBounded() spent the replay budget on an impossible request: attempts = %d, want a pre-attempt rejection",
			exhausted.Attempts(),
		)
	}
	if got.Metadata.Attempts != 0 || serverHits.Load() != 0 {
		t.Fatalf(
			"attempts/server hits = (%d, %d), want (0, 0)",
			got.Metadata.Attempts,
			serverHits.Load(),
		)
	}
}

// TestAggregateTransportFailureDoesNotFabricateResponseIdentity proves that a
// failed real TCP connection remains a transport failure with its native error
// reachable and no fabricated response observation.
func TestAggregateTransportFailureDoesNotFabricateResponseIdentity(t *testing.T) {
	t.Parallel()

	var serverHits atomic.Uint64
	server := httptest.NewServer(http.HandlerFunc(func(
		http.ResponseWriter,
		*http.Request,
	) {
		serverHits.Add(1)
	}))
	client := server.Client()
	target := mustEndpoint(t, server.URL)
	server.Close()

	ok := mustHTTPStatus(t, http.StatusOK)
	got, gotErr := exchange.SendNoBodyBounded(
		exchange.NoBodyBoundedCall{
			Context: context.Background(),
			Client:  mustExchangeClient(t, client),
			Request: exchange.NoBodyBoundedRequest{
				Target: target,
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
	if !errors.Is(gotErr, core.ErrExchangeTransport) {
		t.Fatalf(
			"SendNoBodyBounded() error = %v, want %v",
			gotErr,
			core.ErrExchangeTransport,
		)
	}
	if errors.Is(gotErr, core.ErrExchangeResponse) {
		t.Fatalf(
			"SendNoBodyBounded() error = %v, want no fabricated %v identity",
			gotErr,
			core.ErrExchangeResponse,
		)
	}
	var native *net.OpError
	if !errors.As(gotErr, &native) {
		t.Fatalf(
			"SendNoBodyBounded() error = %v, want reachable *net.OpError",
			gotErr,
		)
	}
	if got.Metadata.Attempts != 0 || serverHits.Load() != 0 {
		t.Fatalf(
			"SendNoBodyBounded() metadata attempts/server hits = (%d, %d), want (0, 0)",
			got.Metadata.Attempts,
			serverHits.Load(),
		)
	}
}
