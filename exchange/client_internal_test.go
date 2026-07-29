package exchange

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestRetryAfterParserHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	maximum, gotMaximumErr := temporal.DurationFromSeconds(3)
	if gotMaximumErr != nil {
		t.Fatalf("DurationFromSeconds(3) setup error = %v, want nil", gotMaximumErr)
	}
	twoSeconds, gotTwoSecondsErr := temporal.DurationFromSeconds(2)
	if gotTwoSecondsErr != nil {
		t.Fatalf("DurationFromSeconds(2) setup error = %v, want nil", gotTwoSecondsErr)
	}
	futureHTTPDate := time.Date(
		2200,
		time.January,
		1,
		0,
		0,
		0,
		0,
		time.UTC,
	).Format(http.TimeFormat)
	pastHTTPDate := time.Date(
		2000,
		time.January,
		1,
		0,
		0,
		0,
		0,
		time.UTC,
	).Format(http.TimeFormat)

	cases := []struct {
		name   string
		value  string
		want   temporal.Duration
		wantOK bool
	}{
		{name: "numeric seconds below maximum are preserved", value: "2", want: twoSeconds, wantOK: true},
		{name: "numeric seconds at maximum are preserved", value: "3", want: maximum, wantOK: true},
		{name: "numeric seconds above maximum are clamped", value: "4", want: maximum, wantOK: true},
		{name: "large numeric seconds are clamped", value: "999999999", want: maximum, wantOK: true},
		{name: "surrounding optional whitespace is ignored", value: " 2 ", want: twoSeconds, wantOK: true},
		{name: "future HTTP date is clamped", value: futureHTTPDate, want: maximum, wantOK: true},
		{name: "empty value has no server hint"},
		{name: "ASCII whitespace has no server hint", value: " \t"},
		{name: "zero seconds defers to caller backoff", value: "0"},
		{name: "negative seconds are malformed", value: "-1"},
		{name: "fractional seconds are malformed", value: "1.5"},
		{name: "overflowing seconds are malformed", value: "18446744073709551615"},
		{name: "arbitrary text is malformed", value: "later"},
		{name: "malformed HTTP date is rejected", value: "Mon, 99 Jan 2200 00:00:00 GMT"},
		{name: "past HTTP date has no positive delay", value: pastHTTPDate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotOK := parseRetryAfter(tc.value, maximum)
			if gotOK != tc.wantOK || got != tc.want {
				t.Fatalf(
					"parseRetryAfter(%q) = (%v, %t), want (%v, %t)",
					tc.value,
					got,
					gotOK,
					tc.want,
					tc.wantOK,
				)
			}
		})
	}
}

func TestObservedAggregateResponseLayerTriad(t *testing.T) {
	t.Parallel()

	status, gotStatusErr := core.NewHTTPStatusCode(http.StatusOK)
	if gotStatusErr != nil {
		t.Fatalf("NewHTTPStatusCode(200) setup error = %v, want nil", gotStatusErr)
	}

	t.Run("positive valid observation returns complete metadata and bytes", func(t *testing.T) {
		t.Parallel()

		got, gotErr := observedAggregateResponse(
			attemptResponse{status: status, body: []byte("ok")},
			2,
		)
		if gotErr != nil {
			t.Fatalf("observedAggregateResponse() error = %v, want nil", gotErr)
		}
		if got.metadata.Status != status ||
			got.metadata.Attempts != 2 ||
			got.metadata.Bytes.Uint64() != 2 ||
			string(got.body) != "ok" {
			t.Fatalf(
				"observed response = (%v, %d, %d, %q), want (%v, 2, 2, %q)",
				got.metadata.Status,
				got.metadata.Attempts,
				got.metadata.Bytes.Uint64(),
				got.body,
				status,
				"ok",
			)
		}
	})

	t.Run("negative invalid status returns no observation and a response error", func(t *testing.T) {
		t.Parallel()

		got, gotErr := observedAggregateResponse(attemptResponse{}, 1)
		if len(got.body) != 0 ||
			got.metadata.Status != (core.HTTPStatusCode{}) ||
			got.metadata.Attempts != 0 ||
			len(got.metadata.Headers.Values) != 0 ||
			!errors.Is(gotErr, core.ErrExchangeResponse) {
			t.Fatalf(
				"invalid-status observation = (%v, %v), want (zero, %v)",
				got,
				gotErr,
				core.ErrExchangeResponse,
			)
		}
	})

	t.Run("neutral zero attempts cannot become a false successful observation", func(t *testing.T) {
		t.Parallel()

		got, gotErr := observedAggregateResponse(
			attemptResponse{status: status},
			0,
		)
		if len(got.body) != 0 ||
			got.metadata.Status != (core.HTTPStatusCode{}) ||
			got.metadata.Attempts != 0 ||
			len(got.metadata.Headers.Values) != 0 ||
			!errors.Is(gotErr, core.ErrExchangeResponse) {
			t.Fatalf(
				"zero-attempt observation = (%v, %v), want (zero, %v)",
				got,
				gotErr,
				core.ErrExchangeResponse,
			)
		}
	})
}
