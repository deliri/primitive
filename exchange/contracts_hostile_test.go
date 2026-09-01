package exchange_test

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestRequestSemanticsLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive every admitted replay declaration matches its method", func(t *testing.T) {
		t.Parallel()

		key, gotKeyErr := exchange.ParseIdempotencyKey("operation-123")
		if gotKeyErr != nil {
			t.Fatalf("ParseIdempotencyKey() setup error = %v, want nil", gotKeyErr)
		}
		if got := key.String(); got != "operation-123" {
			t.Fatalf("IdempotencyKey.String() = %q, want %q", got, "operation-123")
		}
		cases := []struct {
			name   string
			key    exchange.IdempotencyKey
			method exchange.Method
			replay exchange.ReplayMode
		}{
			{name: "safe GET can replay", method: exchange.MethodGet, replay: exchange.ReplaySafe},
			{name: "safe HEAD can replay", method: exchange.MethodHead, replay: exchange.ReplaySafe},
			{name: "safe OPTIONS can replay", method: exchange.MethodOptions, replay: exchange.ReplaySafe},
			{name: "idempotent PUT can replay", method: exchange.MethodPut, replay: exchange.ReplayIdempotent},
			{name: "idempotent DELETE can replay", method: exchange.MethodDelete, replay: exchange.ReplayIdempotent},
			{name: "POST can replay under an explicit key", method: exchange.MethodPost, replay: exchange.ReplayIdempotencyKey, key: key},
			{name: "PATCH can replay under an explicit key", method: exchange.MethodPatch, replay: exchange.ReplayIdempotencyKey, key: key},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				semantics := exchange.RequestSemantics{
					Method: tc.method, Replay: tc.replay,
					IdempotencyKey: tc.key,
				}
				gotErr := semantics.Validate()
				if gotErr != nil {
					t.Fatalf("RequestSemantics.Validate() error = %v, want nil", gotErr)
				}
				gotRetry, gotRetryErr := semantics.AllowsRetry()
				if gotRetryErr != nil || !gotRetry {
					t.Fatalf("RequestSemantics.AllowsRetry() = (%t, %v), want (true, nil)", gotRetry, gotRetryErr)
				}
				var offWire core.OffWireEnum = tc.replay
				offWire.OffWireEnum()
				if got := tc.replay.String(); got == "" {
					t.Fatalf("ReplayMode.String() = %q, want a diagnostic", got)
				}
			})
		}
	})

	t.Run("negative every mismatched replay declaration is refused", func(t *testing.T) {
		t.Parallel()

		key, gotKeyErr := exchange.ParseIdempotencyKey("operation-123")
		if gotKeyErr != nil {
			t.Fatalf("ParseIdempotencyKey() setup error = %v, want nil", gotKeyErr)
		}
		cases := []struct {
			name      string
			semantics exchange.RequestSemantics
		}{
			{name: "zero semantics is rejected", semantics: exchange.RequestSemantics{}},
			{name: "unknown future method is rejected", semantics: exchange.RequestSemantics{Method: exchange.Method(math.MaxUint8), Replay: exchange.ReplaySingleAttempt}},
			{name: "unknown future replay mode is rejected", semantics: exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplayMode(math.MaxUint8)}},
			{name: "POST is not declared safe", semantics: exchange.RequestSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySafe}},
			{name: "PATCH is not declared safe", semantics: exchange.RequestSemantics{Method: exchange.MethodPatch, Replay: exchange.ReplaySafe}},
			{name: "DELETE is not declared safe", semantics: exchange.RequestSemantics{Method: exchange.MethodDelete, Replay: exchange.ReplaySafe}},
			{name: "GET does not use the mutation idempotent declaration", semantics: exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplayIdempotent}},
			{name: "POST is not inherently idempotent", semantics: exchange.RequestSemantics{Method: exchange.MethodPost, Replay: exchange.ReplayIdempotent}},
			{name: "PATCH is not inherently idempotent", semantics: exchange.RequestSemantics{Method: exchange.MethodPatch, Replay: exchange.ReplayIdempotent}},
			{name: "POST key replay requires a key", semantics: exchange.RequestSemantics{Method: exchange.MethodPost, Replay: exchange.ReplayIdempotencyKey}},
			{name: "PATCH key replay requires a key", semantics: exchange.RequestSemantics{Method: exchange.MethodPatch, Replay: exchange.ReplayIdempotencyKey}},
			{name: "GET cannot use mutation key replay", semantics: exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplayIdempotencyKey, IdempotencyKey: key}},
			{name: "PUT cannot use mutation key replay", semantics: exchange.RequestSemantics{Method: exchange.MethodPut, Replay: exchange.ReplayIdempotencyKey, IdempotencyKey: key}},
			{name: "single attempt rejects an irrelevant key", semantics: exchange.RequestSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt, IdempotencyKey: key}},
			{name: "safe replay rejects an irrelevant key", semantics: exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySafe, IdempotencyKey: key}},
			{name: "idempotent replay rejects an irrelevant key", semantics: exchange.RequestSemantics{Method: exchange.MethodPut, Replay: exchange.ReplayIdempotent, IdempotencyKey: key}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				gotErr := tc.semantics.Validate()
				if !errors.Is(gotErr, core.ErrExchangeContract) {
					t.Fatalf("RequestSemantics.Validate() error = %v, want %v", gotErr, core.ErrExchangeContract)
				}
				gotRetry, gotRetryErr := tc.semantics.AllowsRetry()
				if gotRetry || !errors.Is(gotRetryErr, core.ErrExchangeContract) {
					t.Fatalf(
						"RequestSemantics.AllowsRetry() = (%t, %v), want (false, %v)",
						gotRetry,
						gotRetryErr,
						core.ErrExchangeContract,
					)
				}
			})
		}
	})

	t.Run("neutral single attempt admits every method without replay state", func(t *testing.T) {
		t.Parallel()

		methods := [...]exchange.Method{
			exchange.MethodGet,
			exchange.MethodHead,
			exchange.MethodPost,
			exchange.MethodPut,
			exchange.MethodPatch,
			exchange.MethodDelete,
			exchange.MethodOptions,
		}
		for _, method := range methods {
			semantics := exchange.RequestSemantics{
				Method: method,
				Replay: exchange.ReplaySingleAttempt,
			}
			gotRetry, gotErr := semantics.AllowsRetry()
			if gotErr != nil || gotRetry {
				t.Fatalf(
					"%s single-attempt AllowsRetry() = (%t, %v), want (false, nil)",
					method,
					gotRetry,
					gotErr,
				)
			}
		}
	})

	t.Run("neutral keyed single attempt carries identity without permitting retry", func(t *testing.T) {
		t.Parallel()

		key, gotKeyErr := exchange.ParseIdempotencyKey("operation-123")
		if gotKeyErr != nil {
			t.Fatalf("ParseIdempotencyKey() setup error = %v, want nil", gotKeyErr)
		}
		for _, method := range [...]exchange.Method{exchange.MethodPost, exchange.MethodPatch} {
			semantics := exchange.RequestSemantics{
				Method:         method,
				Replay:         exchange.ReplaySingleAttemptWithIdempotencyKey,
				IdempotencyKey: key,
			}
			gotRetry, gotErr := semantics.AllowsRetry()
			if gotErr != nil || gotRetry {
				t.Fatalf(
					"%s keyed single-attempt AllowsRetry() = (%t, %v), want (false, nil)",
					method,
					gotRetry,
					gotErr,
				)
			}
		}
	})
}

func TestOperationPolicyLayerTriad(t *testing.T) {
	t.Parallel()

	oneNanosecond, gotDurationErr := temporal.DurationFromNanoseconds(1)
	if gotDurationErr != nil {
		t.Fatalf("DurationFromNanoseconds(1) setup error = %v, want nil", gotDurationErr)
	}
	fiveNanoseconds, gotDurationErr := temporal.DurationFromNanoseconds(5)
	if gotDurationErr != nil {
		t.Fatalf("DurationFromNanoseconds(5) setup error = %v, want nil", gotDurationErr)
	}
	maximumDuration, gotDurationErr := temporal.DurationFromNanoseconds(
		temporal.DurationMaximumNanoseconds,
	)
	if gotDurationErr != nil {
		t.Fatalf("DurationFromNanoseconds(maximum) setup error = %v, want nil", gotDurationErr)
	}

	t.Run("positive caller owns finite attempts and redirect hops", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name   string
			policy exchange.OperationPolicy
		}{
			{
				name: "minimum positive retry durations and one same-origin hop",
				policy: exchange.OperationPolicy{
					OperationTimeout: fiveNanoseconds,
					AttemptTimeout:   oneNanosecond,
					Retry: exchange.RetryPolicy{
						BaseDelay: oneNanosecond, MaximumDelay: fiveNanoseconds,
						MaximumJitter: oneNanosecond, MaximumRetryAfter: oneNanosecond,
						MaximumWait: fiveNanoseconds, MaximumAttempts: 2,
					},
					Redirect: exchange.RedirectPolicy{
						Mode: exchange.RedirectSameOrigin, MaximumHops: 1,
					},
				},
			},
			{
				name: "attempt may consume the complete operation budget",
				policy: exchange.OperationPolicy{
					OperationTimeout: fiveNanoseconds,
					AttemptTimeout:   fiveNanoseconds,
					Retry:            exchange.RetryPolicy{MaximumAttempts: 1},
					Redirect:         exchange.RedirectPolicy{Mode: exchange.RedirectReject},
				},
			},
			{
				name: "maximum temporal budget remains representable",
				policy: exchange.OperationPolicy{
					OperationTimeout: maximumDuration,
					AttemptTimeout:   maximumDuration,
					Retry:            exchange.RetryPolicy{MaximumAttempts: 1},
					Redirect:         exchange.RedirectPolicy{Mode: exchange.RedirectReject},
				},
			},
			{
				name: "attempt count has no Exchange ceiling inside the total budget",
				policy: exchange.OperationPolicy{
					OperationTimeout: fiveNanoseconds,
					AttemptTimeout:   oneNanosecond,
					Retry: exchange.RetryPolicy{
						BaseDelay: oneNanosecond, MaximumDelay: fiveNanoseconds,
						MaximumJitter: oneNanosecond, MaximumRetryAfter: oneNanosecond,
						MaximumWait: fiveNanoseconds, MaximumAttempts: math.MaxUint64,
					},
					Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject},
				},
			},
			{
				name: "redirect hop count has no Exchange ceiling",
				policy: exchange.OperationPolicy{
					OperationTimeout: fiveNanoseconds,
					AttemptTimeout:   oneNanosecond,
					Retry:            exchange.RetryPolicy{MaximumAttempts: 1},
					Redirect: exchange.RedirectPolicy{
						Mode: exchange.RedirectSameOrigin, MaximumHops: math.MaxUint64,
					},
				},
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				gotErr := tc.policy.Validate()
				if gotErr != nil {
					t.Fatalf("OperationPolicy.Validate() error = %v, want nil", gotErr)
				}
				var offWire core.OffWireEnum = tc.policy.Redirect.Mode
				offWire.OffWireEnum()
				if got := tc.policy.Redirect.Mode.String(); got == "" {
					t.Fatalf("RedirectMode.String() = %q, want a diagnostic", got)
				}
			})
		}
	})

	t.Run("negative impossible budgets and conflicting owners are rejected", func(t *testing.T) {
		t.Parallel()

		validRetry := exchange.RetryPolicy{
			BaseDelay: oneNanosecond, MaximumDelay: fiveNanoseconds,
			MaximumJitter: oneNanosecond, MaximumRetryAfter: oneNanosecond,
			MaximumWait: fiveNanoseconds, MaximumAttempts: 2,
		}
		cases := []struct {
			name   string
			policy exchange.OperationPolicy
		}{
			{name: "zero operation policy is rejected"},
			{name: "zero operation timeout is rejected", policy: exchange.OperationPolicy{AttemptTimeout: oneNanosecond, Retry: exchange.RetryPolicy{MaximumAttempts: 1}, Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject}}},
			{name: "zero attempt timeout is rejected", policy: exchange.OperationPolicy{OperationTimeout: fiveNanoseconds, Retry: exchange.RetryPolicy{MaximumAttempts: 1}, Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject}}},
			{name: "attempt beyond operation timeout is rejected", policy: exchange.OperationPolicy{OperationTimeout: oneNanosecond, AttemptTimeout: fiveNanoseconds, Retry: exchange.RetryPolicy{MaximumAttempts: 1}, Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject}}},
			{name: "zero maximum attempts is rejected", policy: exchange.OperationPolicy{OperationTimeout: fiveNanoseconds, AttemptTimeout: oneNanosecond, Retry: exchange.RetryPolicy{}, Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject}}},
			{name: "single attempt with base delay is rejected", policy: exchange.OperationPolicy{OperationTimeout: fiveNanoseconds, AttemptTimeout: oneNanosecond, Retry: exchange.RetryPolicy{BaseDelay: oneNanosecond, MaximumAttempts: 1}, Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject}}},
			{name: "multiple attempts with zero base delay are rejected", policy: exchange.OperationPolicy{OperationTimeout: fiveNanoseconds, AttemptTimeout: oneNanosecond, Retry: exchange.RetryPolicy{MaximumDelay: fiveNanoseconds, MaximumJitter: oneNanosecond, MaximumRetryAfter: oneNanosecond, MaximumWait: fiveNanoseconds, MaximumAttempts: 2}, Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject}}},
			{name: "base delay beyond maximum delay is rejected", policy: exchange.OperationPolicy{OperationTimeout: fiveNanoseconds, AttemptTimeout: oneNanosecond, Retry: exchange.RetryPolicy{BaseDelay: fiveNanoseconds, MaximumDelay: oneNanosecond, MaximumJitter: oneNanosecond, MaximumRetryAfter: oneNanosecond, MaximumWait: fiveNanoseconds, MaximumAttempts: 2}, Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject}}},
			{name: "jitter beyond maximum delay is rejected", policy: exchange.OperationPolicy{OperationTimeout: fiveNanoseconds, AttemptTimeout: oneNanosecond, Retry: exchange.RetryPolicy{BaseDelay: oneNanosecond, MaximumDelay: oneNanosecond, MaximumJitter: fiveNanoseconds, MaximumRetryAfter: oneNanosecond, MaximumWait: fiveNanoseconds, MaximumAttempts: 2}, Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject}}},
			{name: "Retry-After beyond total wait is rejected", policy: exchange.OperationPolicy{OperationTimeout: fiveNanoseconds, AttemptTimeout: oneNanosecond, Retry: exchange.RetryPolicy{BaseDelay: oneNanosecond, MaximumDelay: fiveNanoseconds, MaximumJitter: oneNanosecond, MaximumRetryAfter: fiveNanoseconds, MaximumWait: oneNanosecond, MaximumAttempts: 2}, Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject}}},
			{name: "unknown redirect mode is rejected", policy: exchange.OperationPolicy{OperationTimeout: fiveNanoseconds, AttemptTimeout: oneNanosecond, Retry: validRetry, Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectMode(math.MaxUint8)}}},
			{name: "reject redirect mode refuses a hop budget", policy: exchange.OperationPolicy{OperationTimeout: fiveNanoseconds, AttemptTimeout: oneNanosecond, Retry: validRetry, Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject, MaximumHops: 1}}},
			{name: "same-origin redirect requires a hop budget", policy: exchange.OperationPolicy{OperationTimeout: fiveNanoseconds, AttemptTimeout: oneNanosecond, Retry: validRetry, Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectSameOrigin}}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				gotErr := tc.policy.Validate()
				if !errors.Is(gotErr, core.ErrExchangeContract) {
					t.Fatalf("OperationPolicy.Validate() error = %v, want %v", gotErr, core.ErrExchangeContract)
				}
			})
		}
	})

	t.Run("neutral one attempt requires and creates no retry state", func(t *testing.T) {
		t.Parallel()

		gotErr := singleAttemptOperationPolicy(t).Validate()
		if gotErr != nil {
			t.Fatalf("single-attempt OperationPolicy.Validate() error = %v, want nil", gotErr)
		}
	})
}

func TestHeadersLayerTriad(t *testing.T) {
	t.Parallel()

	name, gotNameErr := core.ParseHTTPHeaderName("X-Exchange-Trace")
	if gotNameErr != nil {
		t.Fatalf("ParseHTTPHeaderName() setup error = %v, want nil", gotNameErr)
	}

	t.Run("positive exact header and value boundaries remain typed", func(t *testing.T) {
		t.Parallel()

		values := make([]exchange.HeaderValue, exchange.HeaderValueMaximumCount)
		for index := range values {
			values[index] = mustHeaderValue(t, strings.Repeat(
				string(rune('a'+index%26)),
				exchange.HeaderValueMaximumBytes,
			))
		}
		headers := exchange.Headers{
			Values: []exchange.Header{{Name: name, Values: values}},
		}
		gotErr := headers.Validate()
		if gotErr != nil {
			t.Fatalf("Headers.Validate() error = %v, want nil", gotErr)
		}
	})

	t.Run("negative duplicates framing overrides and injection are rejected", func(t *testing.T) {
		t.Parallel()

		for _, value := range []string{
			strings.Repeat("a", exchange.HeaderValueMaximumBytes+1),
			"safe\rinjected",
			"safe\ninjected",
		} {
			got, gotErr := exchange.NewHeaderValue(value)
			if !errors.Is(gotErr, core.ErrExchangeContract) || got != (exchange.HeaderValue{}) {
				t.Fatalf("NewHeaderValue(%d bytes) = (%v, %v), want zero and errors.Is %v",
					len(value), got, gotErr, core.ErrExchangeContract)
			}
		}

		cases := []struct {
			name    string
			headers exchange.Headers
		}{
			{
				name: "header without values is rejected",
				headers: exchange.Headers{
					Values: []exchange.Header{{Name: name}},
				},
			},
			{
				name: "one value above the count bound is rejected",
				headers: exchange.Headers{
					Values: []exchange.Header{{
						Name: name,
						Values: make(
							[]exchange.HeaderValue,
							exchange.HeaderValueMaximumCount+1,
						),
					}},
				},
			},
			{
				name: "duplicate canonical name is rejected",
				headers: exchange.Headers{
					Values: []exchange.Header{
						{Name: name, Values: []exchange.HeaderValue{mustHeaderValue(t, "a")}},
						{Name: name, Values: []exchange.HeaderValue{mustHeaderValue(t, "b")}},
					},
				},
			},
			{
				name: "Content-Type ownership cannot be overridden",
				headers: exchange.Headers{
					Values: []exchange.Header{{
						Name:   core.HTTPHeaderContentType(),
						Values: []exchange.HeaderValue{mustHeaderValue(t, "text/plain")},
					}},
				},
			},
			{
				name: "Content-Length ownership cannot be overridden",
				headers: exchange.Headers{
					Values: []exchange.Header{{
						Name:   core.HTTPHeaderContentLength(),
						Values: []exchange.HeaderValue{mustHeaderValue(t, "1")},
					}},
				},
			},
			{
				name: "Accept-Encoding ownership cannot be overridden",
				headers: exchange.Headers{
					Values: []exchange.Header{{
						Name:   core.HTTPHeaderAcceptEncoding(),
						Values: []exchange.HeaderValue{mustHeaderValue(t, "gzip")},
					}},
				},
			},
			{
				name: "Content-Encoding ownership cannot mislabel identity bytes",
				headers: exchange.Headers{
					Values: []exchange.Header{{
						Name:   core.HTTPHeaderContentEncoding(),
						Values: []exchange.HeaderValue{mustHeaderValue(t, "gzip")},
					}},
				},
			},
			{
				name: "Idempotency-Key ownership cannot be overridden",
				headers: exchange.Headers{
					Values: []exchange.Header{{
						Name:   core.HTTPHeaderIdempotencyKey(),
						Values: []exchange.HeaderValue{mustHeaderValue(t, "other")},
					}},
				},
			},
			{
				name: "Host ownership cannot be overridden",
				headers: exchange.Headers{Values: []exchange.Header{{
					Name: core.HTTPHeaderHost(), Values: []exchange.HeaderValue{mustHeaderValue(t, "other.example")},
				}}},
			},
			{
				name: "Transfer-Encoding ownership cannot be overridden",
				headers: exchange.Headers{Values: []exchange.Header{{
					Name: core.HTTPHeaderTransferEncoding(), Values: []exchange.HeaderValue{mustHeaderValue(t, "chunked")},
				}}},
			},
			{
				name: "Connection ownership cannot be overridden",
				headers: exchange.Headers{Values: []exchange.Header{{
					Name: core.HTTPHeaderConnection(), Values: []exchange.HeaderValue{mustHeaderValue(t, "close")},
				}}},
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				gotErr := tc.headers.Validate()
				if !errors.Is(gotErr, core.ErrExchangeContract) {
					t.Fatalf("Headers.Validate() error = %v, want %v", gotErr, core.ErrExchangeContract)
				}
			})
		}
	})

	t.Run("neutral empty header selection creates no implicit metadata", func(t *testing.T) {
		t.Parallel()

		if gotErr := (exchange.Headers{}).Validate(); gotErr != nil {
			t.Fatalf("empty Headers.Validate() error = %v, want nil", gotErr)
		}
		if gotErr := (exchange.HeaderSelection{}).Validate(); gotErr != nil {
			t.Fatalf("empty HeaderSelection.Validate() error = %v, want nil", gotErr)
		}
		if gotErr := (exchange.CapturedHeaders{}).Validate(); gotErr != nil {
			t.Fatalf("empty CapturedHeaders.Validate() error = %v, want nil", gotErr)
		}
	})
}
