package exchange

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"net/http"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestAdmittedBodyLengthExhaustsTransportBoundaries(t *testing.T) {
	t.Parallel()

	const limitBytes = 4096
	limit := mustInternalByteCount(t, limitBytes)
	cases := []struct {
		wantIdentity  error
		name          string
		contentLength int64
		limit         core.ByteCount
		wantLength    uint64
		wantPresent   bool
	}{
		{
			name:          "minimum integer is an unexpressible transport extent",
			contentLength: math.MinInt64,
			limit:         limit,
			wantIdentity:  core.ErrExchangeContract,
		},
		{
			name:          "one below absence is an unexpressible transport extent",
			contentLength: -2,
			limit:         limit,
			wantIdentity:  core.ErrExchangeContract,
		},
		{
			name:          "absence is admitted without an extent",
			contentLength: -1,
			limit:         limit,
		},
		{
			name:          "declared empty is distinct from absence",
			contentLength: 0,
			limit:         limit,
			wantPresent:   true,
		},
		{
			name:          "smallest nonempty extent is admitted",
			contentLength: 1,
			limit:         limit,
			wantPresent:   true,
			wantLength:    1,
		},
		{
			name:          "one below the limit is admitted",
			contentLength: limitBytes - 1,
			limit:         limit,
			wantPresent:   true,
			wantLength:    limitBytes - 1,
		},
		{
			name:          "exactly the limit is admitted",
			contentLength: limitBytes,
			limit:         limit,
			wantPresent:   true,
			wantLength:    limitBytes,
		},
		{
			name:          "one above the limit is refused",
			contentLength: limitBytes + 1,
			limit:         limit,
			wantIdentity:  core.ErrExchangeBodyLimit,
		},
		{
			name:          "maximum integer cannot inflate the authorized limit",
			contentLength: math.MaxInt64,
			limit:         limit,
			wantIdentity:  core.ErrExchangeBodyLimit,
		},
		{
			name:          "unset limit is a contract defect",
			contentLength: 1,
			wantIdentity:  core.ErrExchangeContract,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := admittedBodyLength(
				testCase.contentLength,
				testCase.limit,
			)
			if testCase.wantIdentity != nil {
				if !errors.Is(gotErr, testCase.wantIdentity) {
					t.Fatalf(
						"admittedBodyLength(%d) error = %v, want errors.Is %v",
						testCase.contentLength,
						gotErr,
						testCase.wantIdentity,
					)
				}
				if got != (core.DeclaredBodyLength{}) {
					t.Fatalf(
						"admittedBodyLength(%d) = %+v, want zero on refusal",
						testCase.contentLength,
						got,
					)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf(
					"admittedBodyLength(%d) error = %v, want nil",
					testCase.contentLength,
					gotErr,
				)
			}
			if got.Present() != testCase.wantPresent ||
				got.Length().Uint64() != testCase.wantLength {
				t.Fatalf(
					"admittedBodyLength(%d) = (present %t, length %d), want (present %t, length %d)",
					testCase.contentLength,
					got.Present(),
					got.Length().Uint64(),
					testCase.wantPresent,
					testCase.wantLength,
				)
			}
		})
	}
}

func TestAggregateResponseDeclaredExtentCannotWeakenTheBodyLimit(t *testing.T) {
	t.Parallel()

	const limitBytes = 4096
	limit := mustInternalByteCount(t, limitBytes)
	cases := []struct {
		wantIdentity  error
		name          string
		bodyBytes     int
		declaredBytes int64
		wantBytes     int
		wantUnread    int
	}{
		{
			name:          "exact declaration admits the exact bounded response",
			bodyBytes:     limitBytes,
			declaredBytes: limitBytes,
			wantBytes:     limitBytes,
		},
		{
			name:          "absent declaration remains bounded while reading",
			bodyBytes:     limitBytes + 1,
			declaredBytes: -1,
			wantIdentity:  core.ErrExchangeBodyLimit,
		},
		{
			name:          "understated declaration does not raise the read limit",
			bodyBytes:     limitBytes + 1,
			declaredBytes: 1,
			wantIdentity:  core.ErrExchangeBodyLimit,
		},
		{
			name:          "understated declaration admits bytes within the limit",
			bodyBytes:     limitBytes,
			declaredBytes: 1,
			wantBytes:     limitBytes,
		},
		{
			name:          "declared empty cannot conceal one byte over the limit",
			bodyBytes:     limitBytes + 1,
			declaredBytes: 0,
			wantIdentity:  core.ErrExchangeBodyLimit,
		},
		{
			name:          "one over declared limit is refused before reading",
			bodyBytes:     1,
			declaredBytes: limitBytes + 1,
			wantUnread:    1,
			wantIdentity:  core.ErrExchangeBodyLimit,
		},
		{
			name:          "maximum declaration is refused before reading",
			bodyBytes:     1,
			declaredBytes: math.MaxInt64,
			wantUnread:    1,
			wantIdentity:  core.ErrExchangeBodyLimit,
		},
		{
			name:          "one below absence is a response contract defect",
			bodyBytes:     1,
			declaredBytes: -2,
			wantUnread:    1,
			wantIdentity:  core.ErrExchangeContract,
		},
		{
			name:          "minimum integer is a response contract defect",
			bodyBytes:     1,
			declaredBytes: math.MinInt64,
			wantUnread:    1,
			wantIdentity:  core.ErrExchangeContract,
		},
		{
			name:          "declared and actual empty response stays empty",
			declaredBytes: 0,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			source := bytes.NewReader(bytes.Repeat(
				[]byte{0x7d},
				testCase.bodyBytes,
			))
			got, gotErr := readAggregateResponseBody(aggregateReadRequest{
				context: context.Background(),
				response: &http.Response{
					Body:          io.NopCloser(source),
					ContentLength: testCase.declaredBytes,
				},
				limit: limit,
			})
			if testCase.wantIdentity != nil {
				if !errors.Is(gotErr, testCase.wantIdentity) {
					t.Fatalf(
						"readAggregateResponseBody() error = %v, want errors.Is %v",
						gotErr,
						testCase.wantIdentity,
					)
				}
				if len(got) != 0 {
					t.Fatalf(
						"readAggregateResponseBody() returned %d refused bytes, want none",
						len(got),
					)
				}
			} else {
				if gotErr != nil {
					t.Fatalf(
						"readAggregateResponseBody() error = %v, want nil",
						gotErr,
					)
				}
				if len(got) != testCase.wantBytes {
					t.Fatalf(
						"len(readAggregateResponseBody()) = %d, want %d",
						len(got),
						testCase.wantBytes,
					)
				}
			}
			if gotUnread := source.Len(); gotUnread != testCase.wantUnread {
				t.Fatalf(
					"source bytes unread = %d, want %d",
					gotUnread,
					testCase.wantUnread,
				)
			}
		})
	}
}

func TestDeclaredReservationDoesNotDoubleBeforeEOF(t *testing.T) {
	t.Parallel()

	const bodyBytes = 512 * 1024
	body := bytes.Repeat([]byte{0x3c}, bodyBytes)
	declared, err := core.ParseDeclaredBodyLength(bodyBytes)
	if err != nil {
		t.Fatalf(
			"core.ParseDeclaredBodyLength(%d) error = %v, want nil",
			bodyBytes,
			err,
		)
	}
	got, gotErr := readBoundedBody(boundedBodyRead{
		context:  context.Background(),
		source:   bytes.NewReader(body),
		declared: declared,
		limit:    mustInternalByteCount(t, bodyBytes),
	})
	if gotErr != nil {
		t.Fatalf("readBoundedBody() error = %v, want nil", gotErr)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf(
			"bytes.Equal(readBoundedBody(), source) = false for %d bytes, want true",
			bodyBytes,
		)
	}
	if gotCapacity := cap(got); gotCapacity != bodyBytes {
		t.Fatalf(
			"cap(readBoundedBody()) = %d, want exact declared reservation %d",
			gotCapacity,
			bodyBytes,
		)
	}
}

func mustInternalByteCount(t *testing.T, value uint64) core.ByteCount {
	t.Helper()
	got, err := core.NewByteCount(value)
	if err != nil {
		t.Fatalf("core.NewByteCount(%d) error = %v, want nil", value, err)
	}
	return got
}

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
