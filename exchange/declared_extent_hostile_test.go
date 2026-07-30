package exchange_test

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// TestDeclaredExtentNeverReplacesTheBodyLimit is the safety property of reserving
// from a declared extent: the declaration is a hint about how much to allocate,
// never a statement of how much may be read.
//
// A sender that understates its extent must still be cut off at the limit. A
// sender that declares more than the authorized limit must be refused before the
// body is read, even if the test transport presents fewer bytes than it declared.
// If the reservation ever became the bound, an ingress could admit an unbounded
// body behind a small declaration.
func TestDeclaredExtentNeverReplacesTheBodyLimit(t *testing.T) {
	t.Parallel()

	const limitBytes = 4096
	limit := mustByteCount(t, limitBytes)
	policy := exchange.ServerBoundedPolicy{RequestBodyLimit: limit}
	route := exchange.RouteSemantics{
		Method: core.HTTPMethodPost,
		Replay: exchange.ReplaySingleAttempt,
	}
	cases := []struct {
		wantIdentity  error
		name          string
		sentBytes     int
		declaredBytes int64
		wantBytes     int
	}{
		{
			name:          "an understated extent does not raise the limit",
			sentBytes:     limitBytes + 1,
			declaredBytes: 1,
			wantIdentity:  core.ErrExchangeBodyLimit,
		},
		{
			name:          "an understated extent still admits a body within the limit",
			sentBytes:     limitBytes,
			declaredBytes: 1,
			wantBytes:     limitBytes,
		},
		{
			name:          "an overstated extent within the limit admits what was sent",
			sentBytes:     1,
			declaredBytes: limitBytes,
			wantBytes:     1,
		},
		{
			name:          "an overstated extent above the limit is refused before reading",
			sentBytes:     1,
			declaredBytes: limitBytes + 1,
			wantIdentity:  core.ErrExchangeBodyLimit,
		},
		{
			name:          "an undeclared extent is bounded while reading",
			sentBytes:     limitBytes + 1,
			declaredBytes: -1,
			wantIdentity:  core.ErrExchangeBodyLimit,
		},
		{
			name:          "an unexpressible extent is a contract defect",
			sentBytes:     1,
			declaredBytes: -2,
			wantIdentity:  core.ErrExchangeContract,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			body := bytes.Repeat([]byte{0x2f}, testCase.sentBytes)
			request := httptest.NewRequest(
				http.MethodPost,
				"/ingest",
				bytes.NewReader(body),
			)
			request.Header.Set(
				core.HTTPHeaderContentType().String(),
				core.HTTPMediaTypeOctetStream().String(),
			)
			request.ContentLength = testCase.declaredBytes

			got, gotErr := exchange.ReceiveBounded(exchange.BoundedReceiveCall{
				Request:             request,
				Route:               route,
				Policy:              policy,
				ExpectedContentType: core.HTTPMediaTypeOctetStream(),
			})
			if testCase.wantIdentity != nil {
				if !errors.Is(gotErr, testCase.wantIdentity) {
					t.Fatalf(
						"ReceiveBounded() error = %v, want errors.Is %v",
						gotErr,
						testCase.wantIdentity,
					)
				}
				if len(got.Body) != 0 {
					t.Fatalf(
						"ReceiveBounded() returned %d refused bytes, want none",
						len(got.Body),
					)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("ReceiveBounded() error = %v, want nil", gotErr)
			}
			if len(got.Body) != testCase.wantBytes {
				t.Fatalf(
					"len(ReceiveBounded().Body) = %d, want %d",
					len(got.Body),
					testCase.wantBytes,
				)
			}
			if !bytes.Equal(got.Body, body[:testCase.wantBytes]) {
				t.Fatalf(
					"bytes.Equal(ReceiveBounded().Body, sent[:%d]) = false, want true",
					testCase.wantBytes,
				)
			}
		})
	}
}
