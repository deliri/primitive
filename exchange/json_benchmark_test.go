package exchange_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// testJSONBenchmarkLimitBytes bounds every benchmarked JSON document. It is
// well above the largest body the benchmarks emit so the limit never becomes
// the thing under measurement.
const (
	testJSONBenchmarkLimitBytes  = 64 * 1024
	testJSONEnvelopeBytes        = len(`{"message":""}`)
	testJSONSmallDocumentBytes   = 128
	testJSONTypicalDocumentBytes = 1024
	testJSONPageDocumentBytes    = 8 * 1024
)

type jsonBenchmarkPayload struct {
	name  string
	bytes int
}

// jsonBenchmarkPayloads returns fresh request/response body-size cases for a
// minimal command, a typical record, and a small collection page.
func jsonBenchmarkPayloads() [3]jsonBenchmarkPayload {
	return [3]jsonBenchmarkPayload{
		{name: "128B", bytes: testJSONSmallDocumentBytes},
		{name: "1KiB", bytes: testJSONTypicalDocumentBytes},
		{name: "8KiB", bytes: testJSONPageDocumentBytes},
	}
}

func benchmarkJSONRoute() exchange.RouteSemantics {
	return exchange.RouteSemantics{
		Method: exchange.MethodPost,
		Replay: exchange.ReplaySingleAttempt,
	}
}

// newJSONBenchmarkRequest builds one POST carrying a strict JSON document.
// The body is a fresh reader per call because ReceiveJSON closes what it reads.
func newJSONBenchmarkRequest(b *testing.B, body []byte) *http.Request {
	b.Helper()
	request := httptest.NewRequest(
		http.MethodPost,
		"/",
		bytes.NewReader(body),
	)
	request.Header.Set(
		core.HTTPHeaderContentType().String(),
		mustHTTPMediaType(b, "application/json").String(),
	)
	return request
}

func mustJSONBenchmarkDocument(
	b *testing.B,
	documentBytes int,
) (transportDocument, []byte) {
	b.Helper()

	if documentBytes <= testJSONEnvelopeBytes {
		b.Fatalf(
			"JSON benchmark document bytes = %d, want greater than envelope bytes %d",
			documentBytes,
			testJSONEnvelopeBytes,
		)
	}
	document := transportDocument{
		Message: strings.Repeat("m", documentBytes-testJSONEnvelopeBytes),
	}
	encoded, encodeErr := document.MarshalJSON()
	if encodeErr != nil {
		b.Fatalf("MarshalJSON() setup error = %v, want nil", encodeErr)
	}
	if len(encoded) != documentBytes {
		b.Fatalf(
			"MarshalJSON() bytes = %d, want exact benchmark size %d",
			len(encoded),
			documentBytes,
		)
	}
	return document, encoded
}

// benchJSONWriter is a reusable http.ResponseWriter. It isolates Exchange's
// receive/write boundary from net/http's transport and retains the result so
// every benchmark can prove the measured path remained non-vacuous.
type benchJSONWriter struct {
	hdr  http.Header
	body []byte
	code int
}

func newBenchJSONWriter(bodyCapacity int) *benchJSONWriter {
	return &benchJSONWriter{
		hdr:  make(http.Header, 8),
		body: make([]byte, 0, bodyCapacity),
	}
}

func (w *benchJSONWriter) Header() http.Header { return w.hdr }

func (w *benchJSONWriter) WriteHeader(code int) { w.code = code }

func (w *benchJSONWriter) Write(b []byte) (int, error) {
	w.body = append(w.body, b...)
	return len(b), nil
}

func (w *benchJSONWriter) Reset() {
	for key := range w.hdr {
		delete(w.hdr, key)
	}
	w.body = w.body[:0]
	w.code = 0
}

func (w *benchJSONWriter) verifyResult(
	b *testing.B,
	wantBody []byte,
	wantStatus int,
) {
	b.Helper()

	if w.code != wantStatus {
		b.Fatalf("response status = %d, want %d", w.code, wantStatus)
	}
	if !bytes.Equal(w.body, wantBody) {
		b.Fatalf(
			"response body bytes = %d, want exact %d-byte document",
			len(w.body),
			len(wantBody),
		)
	}
}

// BenchmarkServerJSONBoundary measures one complete server-side API turn —
// strict decode plus contract validation on ingress, validated encode plus
// framed write on egress — with net/http's transport removed. This is the
// per-request work an `api` instance must do beyond the network itself.
func BenchmarkServerJSONBoundary(b *testing.B) {
	b.ReportAllocs()

	route := benchmarkJSONRoute()
	readPolicy := exchange.ServerPolicy{
		RequestBodyLimit: mustByteCount(b, testJSONBenchmarkLimitBytes),
	}
	writePolicy := exchange.JSONWritePolicy{
		ResponseBodyLimit: mustByteCount(b, testJSONBenchmarkLimitBytes),
	}
	ok := mustHTTPStatus(b, http.StatusOK)

	for _, payload := range jsonBenchmarkPayloads() {
		_, encoded := mustJSONBenchmarkDocument(b, payload.bytes)

		b.Run(payload.name, func(b *testing.B) {
			writer := newBenchJSONWriter(len(encoded))
			b.ReportAllocs()
			b.SetBytes(int64(2 * len(encoded)))
			b.ResetTimer()

			for range b.N {
				writer.Reset()
				serverCall := socketServerCallFrom(b, writer, newJSONBenchmarkRequest(b, encoded))
				received, receiveErr := exchange.ReceiveJSON[
					transportDocument,
					*transportDocument,
				](exchange.JSONReceiveCall{
					Call:   serverCall,
					Route:  route,
					Policy: readPolicy,
				})
				if receiveErr != nil {
					b.Fatalf("ReceiveJSON() error = %v, want nil", receiveErr)
				}
				writeErr := exchange.WriteJSON(
					exchange.JSONWriteCall[transportDocument]{
						Call: serverCall,
						Response: exchange.ServerJSONResponse[transportDocument]{
							Body:   *received.Body,
							Status: ok,
						},
						Policy: writePolicy,
					},
				)
				if writeErr != nil {
					b.Fatalf("WriteJSON() error = %v, want nil", writeErr)
				}
			}
			b.StopTimer()
			writer.verifyResult(b, encoded, http.StatusOK)
		})
	}
}

// BenchmarkRequestConstructionControl is the control for
// BenchmarkServerJSONBoundary. The boundary benchmark must build a fresh
// *http.Request per iteration because ReceiveJSON closes the body it reads;
// this measures that setup alone so it can be subtracted from the boundary
// cost rather than silently attributed to Exchange.
func BenchmarkRequestConstructionControl(b *testing.B) {
	_, encoded := mustJSONBenchmarkDocument(
		b,
		testJSONTypicalDocumentBytes,
	)

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		if request := newJSONBenchmarkRequest(b, encoded); request == nil {
			b.Fatal("newJSONBenchmarkRequest() = nil, want non-nil")
		}
	}
}

// BenchmarkServerJSONBoundaryByLimit holds the document fixed and varies only
// the configured RequestBodyLimit. Per-request cost must track the bytes
// actually received; if it tracks the limit instead, every route pays for its
// ceiling on every call and instance sizing cannot be derived from traffic.
func BenchmarkServerJSONBoundaryByLimit(b *testing.B) {
	b.ReportAllocs()

	route := benchmarkJSONRoute()
	writePolicy := exchange.JSONWritePolicy{
		ResponseBodyLimit: mustByteCount(b, testJSONBenchmarkLimitBytes),
	}
	ok := mustHTTPStatus(b, http.StatusOK)
	_, encoded := mustJSONBenchmarkDocument(b, testJSONSmallDocumentBytes)

	limits := [...]struct {
		name  string
		bytes uint64
	}{
		{name: "limit1KiB", bytes: 1024},
		{name: "limit64KiB", bytes: 64 * 1024},
		{name: "limit1MiB", bytes: 1024 * 1024},
	}
	for _, limit := range limits {
		readPolicy := exchange.ServerPolicy{
			RequestBodyLimit: mustByteCount(b, limit.bytes),
		}
		b.Run(limit.name, func(b *testing.B) {
			writer := newBenchJSONWriter(len(encoded))
			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				writer.Reset()
				serverCall := socketServerCallFrom(b, writer, newJSONBenchmarkRequest(b, encoded))
				received, receiveErr := exchange.ReceiveJSON[
					transportDocument,
					*transportDocument,
				](exchange.JSONReceiveCall{
					Call:   serverCall,
					Route:  route,
					Policy: readPolicy,
				})
				if receiveErr != nil {
					b.Fatalf("ReceiveJSON() error = %v, want nil", receiveErr)
				}
				writeErr := exchange.WriteJSON(
					exchange.JSONWriteCall[transportDocument]{
						Call: serverCall,
						Response: exchange.ServerJSONResponse[transportDocument]{
							Body:   *received.Body,
							Status: ok,
						},
						Policy: writePolicy,
					},
				)
				if writeErr != nil {
					b.Fatalf("WriteJSON() error = %v, want nil", writeErr)
				}
			}
			b.StopTimer()
			writer.verifyResult(b, encoded, http.StatusOK)
		})
	}
}

// BenchmarkServerJSONBoundaryParallel is BenchmarkServerJSONBoundary across
// every core, which is how a real instance is loaded.
func BenchmarkServerJSONBoundaryParallel(b *testing.B) {
	route := benchmarkJSONRoute()
	readPolicy := exchange.ServerPolicy{
		RequestBodyLimit: mustByteCount(b, testJSONBenchmarkLimitBytes),
	}
	writePolicy := exchange.JSONWritePolicy{
		ResponseBodyLimit: mustByteCount(b, testJSONBenchmarkLimitBytes),
	}
	ok := mustHTTPStatus(b, http.StatusOK)
	_, encoded := mustJSONBenchmarkDocument(
		b,
		testJSONTypicalDocumentBytes,
	)

	b.ReportAllocs()
	b.SetBytes(int64(2 * len(encoded)))
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		writer := newBenchJSONWriter(len(encoded))
		didRun := false
		for pb.Next() {
			didRun = true
			writer.Reset()
			serverCall := socketServerCallFrom(b, writer, newJSONBenchmarkRequest(b, encoded))
			received, receiveErr := exchange.ReceiveJSON[
				transportDocument,
				*transportDocument,
			](exchange.JSONReceiveCall{
				Call:   serverCall,
				Route:  route,
				Policy: readPolicy,
			})
			if receiveErr != nil {
				b.Fatalf("ReceiveJSON() error = %v, want nil", receiveErr)
			}
			writeErr := exchange.WriteJSON(
				exchange.JSONWriteCall[transportDocument]{
					Call: serverCall,
					Response: exchange.ServerJSONResponse[transportDocument]{
						Body:   *received.Body,
						Status: ok,
					},
					Policy: writePolicy,
				},
			)
			if writeErr != nil {
				b.Fatalf("WriteJSON() error = %v, want nil", writeErr)
			}
		}
		if didRun {
			writer.verifyResult(b, encoded, http.StatusOK)
		}
	})
}

// BenchmarkJSONRoundTripOverLoopbackParallel measures one complete typed JSON
// API turn over a real TCP listener through Exchange's client and server entry
// points. Its explicit parallel shape measures instance throughput under load.
func BenchmarkJSONRoundTripOverLoopbackParallel(b *testing.B) {
	route := benchmarkJSONRoute()
	readPolicy := exchange.ServerPolicy{
		RequestBodyLimit: mustByteCount(b, testJSONBenchmarkLimitBytes),
	}
	writePolicy := exchange.JSONWritePolicy{
		ResponseBodyLimit: mustByteCount(b, testJSONBenchmarkLimitBytes),
	}
	ok := mustHTTPStatus(b, http.StatusOK)
	document, encoded := mustJSONBenchmarkDocument(
		b,
		testJSONTypicalDocumentBytes,
	)

	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		serverCall := socketServerCallFrom(b, writer, request)
		received, receiveErr := exchange.ReceiveJSON[
			transportDocument,
			*transportDocument,
		](exchange.JSONReceiveCall{
			Call:   serverCall,
			Route:  route,
			Policy: readPolicy,
		})
		if receiveErr != nil {
			http.Error(writer, "receive", http.StatusBadRequest)
			return
		}
		if writeErr := exchange.WriteJSON(
			exchange.JSONWriteCall[transportDocument]{
				Call: serverCall,
				Response: exchange.ServerJSONResponse[transportDocument]{
					Body:   *received.Body,
					Status: ok,
				},
				Policy: writePolicy,
			},
		); writeErr != nil {
			b.Errorf("WriteJSON() error = %v, want nil", writeErr)
		}
	}))
	defer server.Close()

	client := mustExchangeClient(b, server.Client())
	target := mustEndpoint(b, server.URL)
	policy := exchange.JSONPolicy{
		Operation:         singleAttemptOperationPolicy(b),
		RequestBodyLimit:  mustByteCount(b, testJSONBenchmarkLimitBytes),
		ResponseBodyLimit: mustByteCount(b, testJSONBenchmarkLimitBytes),
	}
	request := exchange.JSONRequest[transportDocument]{
		Target: target,
		Body:   document,
		Semantics: exchange.RequestSemantics{
			Method: exchange.MethodPost,
			Replay: exchange.ReplaySingleAttempt,
		},
		ExpectedStatus: ok,
	}
	ctx := context.Background()
	b.ReportAllocs()
	b.SetBytes(int64(2 * len(encoded)))
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			got, gotErr := exchange.SendJSON[
				transportDocument,
				transportDocument,
			](exchange.JSONCall[transportDocument]{
				Context: ctx,
				Client:  client,
				Request: request,
				Policy:  policy,
			})
			if gotErr != nil {
				b.Fatalf("exchange.SendJSON() error = %v, want nil", gotErr)
			}
			if got.Body != document ||
				got.Metadata.Status != ok ||
				got.Metadata.Attempts != 1 {
				b.Fatalf(
					"exchange.SendJSON() = %+v, want echoed body, status %v, and one attempt",
					got,
					ok,
				)
			}
		}
	})
}
