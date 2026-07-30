package exchange_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// benchmarkDeclaredBodyBytes is the extent a webhook-class ingress admits. It is
// the size at which a doubling buffer costs the most: every intermediate power of
// two is allocated and copied on the way to the real length.
const benchmarkDeclaredBodyBytes = 512 * 1024

// undeclaredContentLength is the extent net/http reports for a message that
// declares none, which is the state a doubling buffer is still required for.
const undeclaredContentLength = -1

// BenchmarkBoundedReceiveByDeclaredExtent measures one server-side bounded
// receive of the same body twice: once with the extent the request declares, and
// once with no declared extent.
//
// The declared case reserves the extent once, so its allocation should track the
// body itself. The undeclared case is the control: it still has to grow, so the
// pair measures what the declaration buys rather than a cost that vanished
// everywhere. Request construction is inside the timed region for both cases, so
// the difference between them is the buffering and nothing else.
func BenchmarkBoundedReceiveByDeclaredExtent(b *testing.B) {
	b.ReportAllocs()

	body := bytes.Repeat([]byte{0x5a}, benchmarkDeclaredBodyBytes)
	limit := mustBenchmarkByteCount(b, uint64(len(body)))
	policy := exchange.ServerBoundedPolicy{RequestBodyLimit: limit}
	route := exchange.RouteSemantics{
		Method: core.HTTPMethodPost,
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
						Request:             request,
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
