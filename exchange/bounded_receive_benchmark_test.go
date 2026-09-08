package exchange_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// benchmarkDeclaredBodyBytes exercises growth well above the bounded initial
// reservation. The same caller-owned bytes are supplied to both receipt paths.
const benchmarkDeclaredBodyBytes = 512 * 1024

// undeclaredContentLength is the extent net/http reports for a message that
// declares none, which is the state a doubling buffer is still required for.
const undeclaredContentLength = -1

// BenchmarkBoundedReceiveByDeclaredExtent measures one server-side bounded
// receive of the same body twice: once with the extent the request declares, and
// once with no declared extent.
//
// Both cases include request construction and complete bounded receipt. Initial
// reservation is capped below this 512 KiB payload: neither case reserves the
// full declaration. This is a large-body control, not a claimed allocation win
// for declared length. The reported allocations include fixture construction.
func BenchmarkBoundedReceiveByDeclaredExtent(b *testing.B) {
	b.ReportAllocs()

	body := bytes.Repeat([]byte{0x5a}, benchmarkDeclaredBodyBytes)
	limit := mustBenchmarkByteCount(b, uint64(len(body)))
	policy := exchange.ServerBoundedPolicy{RequestBodyLimit: limit}
	route := exchange.RouteSemantics{
		Method: exchange.MethodPost,
		Replay: exchange.ReplaySingleAttempt,
	}
	cases := []struct {
		name     string
		declared bool
	}{
		{name: "declared extent", declared: true},
		{name: "undeclared extent", declared: false},
	}

	for _, testCase := range cases {
		b.Run(testCase.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(body)))
			for b.Loop() {
				request := httptest.NewRequest(
					http.MethodPost,
					"/ingest",
					bytes.NewReader(body),
				)
				request.Header.Set(
					core.HTTPHeaderContentType().String(),
					core.HTTPMediaTypeOctetStream().String(),
				)
				if !testCase.declared {
					request.ContentLength = undeclaredContentLength
				}
				received, err := exchange.ReceiveBounded(
					exchange.BoundedReceiveCall{
						Call:                socketServerCall(b, request),
						Route:               route,
						Policy:              policy,
						ExpectedContentType: core.HTTPMediaTypeOctetStream(),
					},
				)
				if err != nil {
					b.Fatalf("ReceiveBounded() error = %v, want nil", err)
				}
				if len(received.Body) != len(body) {
					b.Fatalf(
						"len(received.Body) = %d, want %d",
						len(received.Body),
						len(body),
					)
				}
			}
		})
	}
}

func mustBenchmarkByteCount(b *testing.B, value uint64) core.ByteCount {
	b.Helper()
	count, err := core.NewByteCount(value)
	if err != nil {
		b.Fatalf("core.NewByteCount(%d) error = %v, want nil", value, err)
	}
	return count
}
