package exchange_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestClientOfficialSDKTransportLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, body string
		wantErr    error
	}{
		{name: "owned TLS authority delivers exact bytes below ceiling", body: strings.Repeat("x", 127)},
		{name: "owned TLS authority delivers exact bytes at ceiling", body: strings.Repeat("x", 128)},
		{name: "owned TLS authority cannot release partial bytes above ceiling", body: strings.Repeat("x", 129), wantErr: core.ErrExchangeBodyLimit},
		{name: "empty response remains empty after owned transport projection"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Uint64
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				if _, err := io.WriteString(w, tc.body); err != nil {
					t.Errorf("provider write error = %v, want nil", err)
				}
			}))
			t.Cleanup(server.Close)
			original := server.Client()
			base := original.Transport
			owned, err := exchange.NewClient(original)
			if err != nil {
				t.Fatalf("NewClient() error = %v, want nil", err)
			}
			limit, err := core.NewByteCount(128)
			if err != nil {
				t.Fatalf("NewByteCount() error = %v, want nil", err)
			}
			boundary, err := exchange.NewOfficialSDKResponseCeiling(exchange.OfficialSDKResponseCeilingRequest{Method: exchange.MethodGet, Representation: exchange.OfficialSDKResponseRepresentationBinary, MaximumBytes: limit})
			if err != nil {
				t.Fatalf("NewOfficialSDKResponseCeiling() error = %v, want nil", err)
			}
			transport, err := owned.OfficialSDKResponseTransport(boundary)
			if err != nil {
				t.Fatalf("OfficialSDKResponseTransport() error = %v, want nil", err)
			}
			client, err := exchange.NewOfficialSDKHTTPClient(transport)
			if err != nil {
				t.Fatalf("NewOfficialSDKHTTPClient() error = %v, want nil", err)
			}
			request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, nil)
			if err != nil {
				t.Fatalf("NewRequestWithContext() error = %v, want nil", err)
			}
			response, err := client.Do(request)
			if response != nil {
				defer func() {
					if err := response.Body.Close(); err != nil {
						t.Errorf("certificate response close error = %v, want nil", err)
					}
				}()
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("SDK Do() error = %v, want %v", err, tc.wantErr)
			}
			if original.Transport != base {
				t.Fatal("admitted client transport = changed, want unchanged")
			}
			if got := calls.Load(); got != 1 {
				t.Fatalf("provider calls = %d, want 1", got)
			}
			if tc.wantErr != nil {
				if response != nil {
					t.Fatalf("refused response = %v, want nil", response)
				}
				return
			}
			body, err := io.ReadAll(io.LimitReader(response.Body, 129))
			if err != nil || string(body) != tc.body {
				t.Fatalf("SDK body = (%q, %v), want (%q, nil)", body, err, tc.body)
			}
		})
	}
}

func TestClientOfficialSDKTransportRejectsUnadmittedConstruction(t *testing.T) {
	t.Parallel()
	client, err := exchange.NewStandardClient()
	if err != nil {
		t.Fatalf("NewStandardClient() error = %v, want nil", err)
	}
	for _, tc := range []struct {
		name   string
		client exchange.Client
	}{
		{name: "zero client cannot create transport"},
		{name: "admitted default transport cannot bypass zero boundary", client: client},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.client.OfficialSDKResponseTransport(exchange.OfficialSDKResponseBoundary{})
			if got != nil || !errors.Is(err, core.ErrExchangeContract) {
				t.Fatalf("OfficialSDKResponseTransport() = (%v, %v), want nil and %v", got, err, core.ErrExchangeContract)
			}
		})
	}
}
