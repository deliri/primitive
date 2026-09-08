package exchange

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"net/http"
	"slices"
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
				if got != (declaredBodyLength{}) {
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
			if got.present != testCase.wantPresent ||
				got.length.Uint64() != testCase.wantLength {
				t.Fatalf(
					"admittedBodyLength(%d) = (present %t, length %d), want (present %t, length %d)",
					testCase.contentLength,
					got.present,
					got.length.Uint64(),
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
	declared, err := parseDeclaredBodyLength(bodyBytes)
	if err != nil {
		t.Fatalf(
			"parseDeclaredBodyLength(%d) error = %v, want nil",
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

// This projection has no policy choices: status, attempt count and captured
// fields are independent admission axes; body extent is exact byte conservation.
func TestObservedAggregateResponseLayerTriad(t *testing.T) {
	t.Parallel()
	value, err := NewHeaderValue("opaque")
	if err != nil {
		t.Fatal(err)
	}
	header := Header{Name: core.HTTPHeaderAccept(), Values: []HeaderValue{value}}
	cases := []struct {
		name     string
		status   core.HTTPStatusCode
		attempts uint64
		body     []byte
		headers  CapturedHeaders
		wantErr  error
	}{
		{name: "binary observation cannot normalize or truncate bytes", status: core.HTTPStatusOK(), attempts: 2, body: []byte{0, 0xff}},
		{name: "empty completed attempt retains status without inventing bytes", status: core.HTTPStatusOK(), attempts: 1},
		{name: "one observed byte cannot round down to absent", status: core.HTTPStatusOK(), attempts: 1, body: []byte{0xff}},
		{name: "maximum attempt counter cannot wrap", status: core.HTTPStatusOK(), attempts: math.MaxUint64},
		{name: "captured field retains its exact nominal value", status: core.HTTPStatusOK(), attempts: 1, headers: CapturedHeaders{Values: []Header{header}}},
		{name: "absent status cannot publish existing bytes", attempts: 1, body: []byte{0xff}, wantErr: core.ErrExchangeResponse},
		{name: "absent attempts cannot publish existing status", status: core.HTTPStatusOK(), wantErr: core.ErrExchangeResponse},
		{name: "invalid captured field cannot publish a completed attempt", status: core.HTTPStatusOK(), attempts: 1, headers: CapturedHeaders{Values: []Header{{}}}, wantErr: core.ErrExchangeResponse},
		{name: "duplicated captured field cannot count as two observations", status: core.HTTPStatusOK(), attempts: 1, headers: CapturedHeaders{Values: []Header{header, header}}, wantErr: core.ErrExchangeResponse},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			headers := CapturedHeaders{Values: slices.Clone(tc.headers.Values)}
			for i := range headers.Values {
				headers.Values[i].Values = slices.Clone(headers.Values[i].Values)
			}
			response := attemptResponse{status: tc.status, body: bytes.Clone(tc.body), headers: headers}
			got, gotErr := observedAggregateResponse(response, tc.attempts)
			if !errors.Is(gotErr, tc.wantErr) || (tc.wantErr != nil && !errors.Is(gotErr, core.ErrExchangeContract)) {
				t.Fatalf("observation error = %v, want %v with owning contract identity", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got.body != nil || got.metadata.Status != (core.HTTPStatusCode{}) || got.metadata.Attempts != 0 || got.metadata.Bytes != (core.ByteLength{}) || got.metadata.Headers.Values != nil {
					t.Fatalf("refused observation = %+v, want exact zero", got)
				}
			} else {
				if got.metadata.Status != tc.status || got.metadata.Attempts != tc.attempts || got.metadata.Bytes.Uint64() != uint64(len(tc.body)) || !bytes.Equal(got.body, tc.body) || !((got.metadata.Headers.Values == nil) == (tc.headers.Values == nil) && slices.EqualFunc(got.metadata.Headers.Values, tc.headers.Values, func(got, want Header) bool {
					return got.Name == want.Name && (got.Values == nil) == (want.Values == nil) && slices.EqualFunc(got.Values, want.Values, func(got, want HeaderValue) bool {
						return (got.value == nil && want.value == nil) || (got.value != nil && want.value != nil && *got.value == *want.value)
					})
				})) {
					t.Fatalf("observation = %+v, want exact status, attempts, byte extent and captured fields", got)
				}
				if err := got.metadata.Validate(); err != nil {
					t.Fatalf("observation validation = %v, want nil", err)
				}
			}
			if response.status != tc.status || !bytes.Equal(response.body, tc.body) || !((response.headers.Values == nil) == (tc.headers.Values == nil) && slices.EqualFunc(response.headers.Values, tc.headers.Values, func(got, want Header) bool {
				return got.Name == want.Name && (got.Values == nil) == (want.Values == nil) && slices.EqualFunc(got.Values, want.Values, func(got, want HeaderValue) bool {
					return (got.value == nil && want.value == nil) || (got.value != nil && want.value != nil && *got.value == *want.value)
				})
			})) {
				t.Fatalf("source status/body/headers=%v/%x/%+v, want %v/%x/%+v", response.status, response.body, response.headers, tc.status, tc.body, tc.headers)
			}
		})
	}
}
