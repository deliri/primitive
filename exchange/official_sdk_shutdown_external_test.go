package exchange_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"testing"

	"github.com/deliri/primitive/v2026/exchange"
)

// A real pooled connection is reused before close and replaced after close.
// This checks the transport effect, not merely the presence of a close method.
func TestOfficialSDKShutdownLayerTriadPreservesPoolOwnership(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	base := &http.Transport{}
	t.Cleanup(base.CloseIdleConnections)
	boundary, err := exchange.NewOfficialSDKMethodResponseBoundary(exchange.OfficialSDKMethodResponseBoundaryRequest{
		Method: exchange.MethodGet, Representation: exchange.OfficialSDKResponseRepresentationBinary,
	})
	if err != nil {
		t.Fatalf("NewOfficialSDKMethodResponseBoundary() error = %v, want nil", err)
	}
	wrapped, err := exchange.NewOfficialSDKResponseTransport(exchange.OfficialSDKResponseTransportRequest{Base: base, Boundary: boundary})
	if err != nil {
		t.Fatalf("NewOfficialSDKResponseTransport() error = %v, want nil", err)
	}
	// Two boundaries model the actual compositional adapter, not a direct base.
	wrapped, err = exchange.NewOfficialSDKResponseTransport(exchange.OfficialSDKResponseTransportRequest{Base: wrapped, Boundary: boundary})
	if err != nil {
		t.Fatalf("NewOfficialSDKResponseTransport(nested) error = %v, want nil", err)
	}
	client, err := exchange.NewOfficialSDKHTTPClient(wrapped)
	if err != nil {
		t.Fatalf("NewOfficialSDKHTTPClient() error = %v, want nil", err)
	}
	client.CloseIdleConnections() // Neutral: no connection exists yet.
	for _, step := range []struct {
		name        string
		closeBefore bool
		wantReused  bool
	}{
		{name: "empty pool opens first connection"},
		{name: "drained response remains reusable before shutdown", wantReused: true},
		{name: "shutdown reaches nested base and prevents stale reuse", closeBefore: true},
		{name: "repeated shutdown remains safe and new requests reconnect", closeBefore: true},
	} {
		if step.closeBefore {
			client.CloseIdleConnections()
			client.CloseIdleConnections()
		}
		connections := make(chan httptrace.GotConnInfo, 1)
		trace := &httptrace.ClientTrace{GotConn: func(info httptrace.GotConnInfo) { connections <- info }}
		ctx, cancel := context.WithTimeout(t.Context(), officialSDKTestTimeout)
		request, err := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, trace), http.MethodGet, server.URL, nil)
		if err != nil {
			cancel()
			t.Fatalf("%s NewRequestWithContext() error = %v, want nil", step.name, err)
		}
		response, err := client.Do(request)
		if err != nil {
			cancel()
			t.Fatalf("%s Do() error = %v, want nil", step.name, err)
		}
		count, readErr := io.Copy(io.Discard, response.Body)
		closeErr := response.Body.Close()
		cancel()
		if response.StatusCode != http.StatusNoContent || count != 0 || readErr != nil || closeErr != nil {
			t.Fatalf("%s status/bytes/read/close = %d/%d/%v/%v, want 204/0/nil/nil", step.name, response.StatusCode, count, readErr, closeErr)
		}
		select {
		case got := <-connections:
			if got.Reused != step.wantReused {
				t.Fatalf("%s connection reused = %t, want %t", step.name, got.Reused, step.wantReused)
			}
		default:
			t.Fatalf("%s observed connections = 0, want 1", step.name)
		}
	}
}
