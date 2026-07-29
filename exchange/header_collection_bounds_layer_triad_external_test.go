package exchange_test

import (
	"context"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func indexedHeaderNames(t testing.TB, count int) []core.HTTPHeaderName {
	t.Helper()

	names := make([]core.HTTPHeaderName, 0, count)
	for index := range count {
		names = append(
			names,
			mustHeaderName(t, "X-Exchange-"+strconv.Itoa(index)),
		)
	}
	return names
}

func indexedHeaders(t testing.TB, count int) []exchange.Header {
	t.Helper()

	values := make([]exchange.Header, 0, count)
	for _, name := range indexedHeaderNames(t, count) {
		values = append(values, exchange.Header{
			Name: name, Values: []string{"value"},
		})
	}
	return values
}

// TestHeaderCollectionBoundsLayerTriad pins the count and uniqueness bounds of
// every header collection. Each collection owns an independent maximum, so a
// bound proved on one is no evidence for another.
func TestHeaderCollectionBoundsLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive each collection admits its exact declared maximum", func(t *testing.T) {
		t.Parallel()

		request := exchange.Headers{
			Values: indexedHeaders(t, exchange.HeaderMaximumCount),
		}
		if gotErr := request.Validate(); gotErr != nil {
			t.Fatalf(
				"Headers.Validate() at %d fields error = %v, want nil",
				exchange.HeaderMaximumCount,
				gotErr,
			)
		}
		response := exchange.ResponseHeaders{
			Values: indexedHeaders(t, exchange.HeaderMaximumCount),
		}
		if gotErr := response.Validate(); gotErr != nil {
			t.Fatalf(
				"ResponseHeaders.Validate() at %d fields error = %v, want nil",
				exchange.HeaderMaximumCount,
				gotErr,
			)
		}
		selection := exchange.HeaderSelection{
			Names: indexedHeaderNames(t, exchange.CapturedHeaderMaximumCount),
		}
		if gotErr := selection.Validate(); gotErr != nil {
			t.Fatalf(
				"HeaderSelection.Validate() at %d names error = %v, want nil",
				exchange.CapturedHeaderMaximumCount,
				gotErr,
			)
		}
		captured := exchange.CapturedHeaders{
			Values: indexedHeaders(t, exchange.CapturedHeaderMaximumCount),
		}
		if gotErr := captured.Validate(); gotErr != nil {
			t.Fatalf(
				"CapturedHeaders.Validate() at %d fields error = %v, want nil",
				exchange.CapturedHeaderMaximumCount,
				gotErr,
			)
		}
	})

	t.Run("negative one field above each declared maximum is refused", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			validate func(testing.TB) error
			name     string
		}{
			{
				name: "one request field above the request bound is refused",
				validate: func(t testing.TB) error {
					return exchange.Headers{
						Values: indexedHeaders(t, exchange.HeaderMaximumCount+1),
					}.Validate()
				},
			},
			{
				name: "one response field above the request bound is refused",
				validate: func(t testing.TB) error {
					return exchange.ResponseHeaders{
						Values: indexedHeaders(t, exchange.HeaderMaximumCount+1),
					}.Validate()
				},
			},
			{
				name: "one selected name above the capture bound is refused",
				validate: func(t testing.TB) error {
					return exchange.HeaderSelection{
						Names: indexedHeaderNames(
							t,
							exchange.CapturedHeaderMaximumCount+1,
						),
					}.Validate()
				},
			},
			{
				name: "one captured field above the capture bound is refused",
				validate: func(t testing.TB) error {
					return exchange.CapturedHeaders{
						Values: indexedHeaders(
							t,
							exchange.CapturedHeaderMaximumCount+1,
						),
					}.Validate()
				},
			},
			{
				name: "a duplicate selected name is refused",
				validate: func(t testing.TB) error {
					name := mustHeaderName(t, "X-Exchange-Trace")
					return exchange.HeaderSelection{
						Names: []core.HTTPHeaderName{name, name},
					}.Validate()
				},
			},
			{
				name: "a duplicate captured field is refused",
				validate: func(t testing.TB) error {
					name := mustHeaderName(t, "X-Exchange-Trace")
					return exchange.CapturedHeaders{
						Values: []exchange.Header{
							{Name: name, Values: []string{"a"}},
							{Name: name, Values: []string{"b"}},
						},
					}.Validate()
				},
			},
			{
				name: "a duplicate response field is refused",
				validate: func(t testing.TB) error {
					name := mustHeaderName(t, "X-Exchange-Trace")
					return exchange.ResponseHeaders{
						Values: []exchange.Header{
							{Name: name, Values: []string{"a"}},
							{Name: name, Values: []string{"b"}},
						},
					}.Validate()
				},
			},
			{
				name: "an unset selected name is refused",
				validate: func(testing.TB) error {
					return exchange.HeaderSelection{
						Names: []core.HTTPHeaderName{{}},
					}.Validate()
				},
			},
			{
				name: "a captured field with no values is refused",
				validate: func(t testing.TB) error {
					return exchange.CapturedHeaders{
						Values: []exchange.Header{{
							Name: mustHeaderName(t, "X-Exchange-Trace"),
						}},
					}.Validate()
				},
			},
			{
				name: "a captured value above the value byte bound is refused",
				validate: func(t testing.TB) error {
					return exchange.CapturedHeaders{
						Values: []exchange.Header{{
							Name: mustHeaderName(t, "X-Exchange-Trace"),
							Values: []string{strings.Repeat(
								"a",
								exchange.HeaderValueMaximumBytes+1,
							)},
						}},
					}.Validate()
				},
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				gotErr := tc.validate(t)
				if !errors.Is(gotErr, core.ErrExchangeContract) {
					t.Fatalf("Validate() error = %v, want %v", gotErr, core.ErrExchangeContract)
				}
			})
		}
	})

	t.Run("neutral a nil collection is an absent projection not an error", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			validate func() error
			name     string
		}{
			{name: "request headers", validate: func() error { return (exchange.Headers{}).Validate() }},
			{name: "response headers", validate: func() error { return (exchange.ResponseHeaders{}).Validate() }},
			{name: "header selection", validate: func() error { return (exchange.HeaderSelection{}).Validate() }},
			{name: "captured headers", validate: func() error { return (exchange.CapturedHeaders{}).Validate() }},
		}
		for _, tc := range cases {
			if gotErr := tc.validate(); gotErr != nil {
				t.Fatalf("zero %s Validate() error = %v, want nil", tc.name, gotErr)
			}
		}
	})
}

// TestClosedModeDomainLayerTriad exhausts both closed policy enums. The domains
// are small enough to prove completely, so every admitted value, the invalid
// zero, and the first value past each domain are all pinned.
func TestClosedModeDomainLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive every admitted mode is valid and carries a diagnostic", func(t *testing.T) {
		t.Parallel()

		replays := [...]exchange.ReplayMode{
			exchange.ReplaySingleAttempt,
			exchange.ReplaySafe,
			exchange.ReplayIdempotent,
			exchange.ReplayIdempotencyKey,
		}
		for _, replay := range replays {
			if !replay.IsValid() || replay.Validate() != nil {
				t.Fatalf(
					"ReplayMode(%d) IsValid/Validate = (%t, %v), want (true, nil)",
					replay,
					replay.IsValid(),
					replay.Validate(),
				)
			}
			if got := replay.String(); got == "" {
				t.Fatalf("ReplayMode(%d).String() = %q, want a diagnostic", replay, got)
			}
		}
		redirects := [...]exchange.RedirectMode{
			exchange.RedirectReject,
			exchange.RedirectSameOrigin,
		}
		for _, redirect := range redirects {
			if !redirect.IsValid() || redirect.Validate() != nil {
				t.Fatalf(
					"RedirectMode(%d) IsValid/Validate = (%t, %v), want (true, nil)",
					redirect,
					redirect.IsValid(),
					redirect.Validate(),
				)
			}
			if got := redirect.String(); got == "" {
				t.Fatalf("RedirectMode(%d).String() = %q, want a diagnostic", redirect, got)
			}
		}
	})

	t.Run("negative the zero and every future mode is refused with no diagnostic", func(t *testing.T) {
		t.Parallel()

		replays := [...]exchange.ReplayMode{
			exchange.ReplayUnknown,
			exchange.ReplayIdempotencyKey + 1,
			exchange.ReplayIdempotencyKey + 2,
			exchange.ReplayMode(math.MaxUint8),
		}
		for _, replay := range replays {
			if replay.IsValid() ||
				!errors.Is(replay.Validate(), core.ErrExchangeContract) {
				t.Fatalf(
					"ReplayMode(%d) IsValid/Validate = (%t, %v), want (false, %v)",
					replay,
					replay.IsValid(),
					replay.Validate(),
					core.ErrExchangeContract,
				)
			}
			if got := replay.String(); got != "" {
				t.Fatalf("ReplayMode(%d).String() = %q, want %q", replay, got, "")
			}
		}
		redirects := [...]exchange.RedirectMode{
			exchange.RedirectUnknown,
			exchange.RedirectSameOrigin + 1,
			exchange.RedirectSameOrigin + 2,
			exchange.RedirectMode(math.MaxUint8),
		}
		for _, redirect := range redirects {
			if redirect.IsValid() ||
				!errors.Is(redirect.Validate(), core.ErrExchangeContract) {
				t.Fatalf(
					"RedirectMode(%d) IsValid/Validate = (%t, %v), want (false, %v)",
					redirect,
					redirect.IsValid(),
					redirect.Validate(),
					core.ErrExchangeContract,
				)
			}
			if got := redirect.String(); got != "" {
				t.Fatalf("RedirectMode(%d).String() = %q, want %q", redirect, got, "")
			}
		}
	})

	t.Run("neutral every admitted replay declaration reaches a method decision", func(t *testing.T) {
		t.Parallel()

		// The replay lattice indexes a fixed-width method array. Every method
		// core admits must land inside that array rather than panicking, so a
		// method added to core cannot silently escape the replay contract.
		methods := [...]core.HTTPMethod{
			core.HTTPMethodGet,
			core.HTTPMethodHead,
			core.HTTPMethodPost,
			core.HTTPMethodPut,
			core.HTTPMethodPatch,
			core.HTTPMethodDelete,
			core.HTTPMethodOptions,
		}
		for _, method := range methods {
			if !method.IsValid() {
				t.Fatalf("core.HTTPMethod(%d).IsValid() = false, want true", method)
			}
			semantics := exchange.RequestSemantics{
				Method: method, Replay: exchange.ReplaySingleAttempt,
			}
			if gotErr := semantics.Validate(); gotErr != nil {
				t.Fatalf(
					"single-attempt RequestSemantics.Validate() for %s error = %v, want nil",
					method,
					gotErr,
				)
			}
		}
		if gotCount := uint8(len(methods) + 1); gotCount != core.HTTPMethodCount {
			t.Fatalf(
				"method table size = %d, want compiler-owned count %d",
				gotCount,
				core.HTTPMethodCount,
			)
		}
	})
}

// TestUploadStatusAndDrainCompositionLayerTriad pins that an unexpected upload
// status stays reachable even when the error body cannot be drained inside the
// policy bound. Losing the status would leave the caller unable to distinguish
// a rejected upload from an oversized diagnostic.
func TestUploadStatusAndDrainCompositionLayerTriad(t *testing.T) {
	t.Parallel()

	policy := singleAttemptStreamPolicy(t)
	errorLimit, errorLimitErr := policy.ErrorBodyLimit.Uint64()
	if errorLimitErr != nil {
		t.Fatalf("StreamPolicy.ErrorBodyLimit.Uint64() setup error = %v, want nil", errorLimitErr)
	}

	cases := []struct {
		name           string
		diagnosticSize uint64
		wantBodyLimit  bool
	}{
		{
			name:           "positive a drainable diagnostic reports only the status",
			diagnosticSize: errorLimit / 2,
		},
		{
			name:           "neutral an exactly bounded diagnostic reports only the status",
			diagnosticSize: errorLimit,
		},
		{
			name:           "negative an oversized diagnostic still reports the status",
			diagnosticSize: errorLimit + 1,
			wantBodyLimit:  true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(
				writer http.ResponseWriter,
				request *http.Request,
			) {
				_, _ = io.Copy(io.Discard, request.Body)
				writer.Header().Set(
					core.HTTPHeaderContentType().String(),
					core.HTTPMediaTypeTextPlain().String(),
				)
				writer.WriteHeader(http.StatusInternalServerError)
				_, _ = writer.Write(
					[]byte(strings.Repeat("E", int(tc.diagnosticSize))),
				)
			}))
			defer server.Close()

			payload := "payload"
			_, gotErr := exchange.Upload(
				exchange.UploadCall{
					Context: context.Background(),
					Client:  mustExchangeClient(t, server.Client()),
					Request: exchange.UploadRequest{
						Target: mustEndpoint(t, server.URL),
						Source: strings.NewReader(payload),
						Semantics: exchange.RequestSemantics{
							Method: core.HTTPMethodPut,
							Replay: exchange.ReplaySingleAttempt,
						},
						ContentLength: core.NewByteLength(uint64(len(payload))),
						ContentType:   core.HTTPMediaTypeOctetStream(),
						ExpectedStatus: mustHTTPStatus(
							t,
							http.StatusCreated,
						),
					},
					Policy: policy,
				},
			)
			var statusErr exchange.StatusError
			if !errors.As(gotErr, &statusErr) {
				t.Fatalf(
					"Upload() error = %v, want a reachable exchange.StatusError",
					gotErr,
				)
			}
			gotStatus, gotStatusErr := statusErr.Status().Int()
			if gotStatusErr != nil || gotStatus != http.StatusInternalServerError {
				t.Fatalf(
					"StatusError.Status() = (%d, %v), want (%d, nil)",
					gotStatus,
					gotStatusErr,
					http.StatusInternalServerError,
				)
			}
			gotBodyLimit := errors.Is(gotErr, core.ErrExchangeBodyLimit)
			if gotBodyLimit != tc.wantBodyLimit {
				t.Fatalf(
					"Upload() body-limit identity = %t, want %t (error = %v)",
					gotBodyLimit,
					tc.wantBodyLimit,
					gotErr,
				)
			}
		})
	}
}
