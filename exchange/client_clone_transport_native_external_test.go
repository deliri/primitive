package exchange_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/exchange"
)

// The official-SDK path is the credential caller's real handoff. Distinct
// remote addresses prove distinct native pools; reuse after closing the clone
// proves that the original pool stayed open. The origin must never be reached.
func TestClientCloneStandardTransportProxyAndPoolCustodyNative(t *testing.T) {
	t.Parallel()
	var originCalls, proxyCalls atomic.Uint64
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		originCalls.Add(1)
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "proxy bypassed")
	}))
	defer origin.Close()
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyCalls.Add(1)
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, r.RemoteAddr)
	}))
	defer proxy.Close()
	proxyURL, err := url.Parse(proxy.URL)
	if err != nil {
		t.Fatal(err)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = http.ProxyURL(proxyURL)
	defer transport.CloseIdleConnections()
	original, err := exchange.NewClient(&http.Client{Transport: transport})
	if err != nil {
		t.Fatal(err)
	}
	clone, err := original.CloneStandardTransport()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := clone.CloseIdleConnections(); err != nil {
			t.Errorf("clone close error = %v, want nil", err)
		}
	}()
	firstOriginal := sdkCloneConnection(t, original, origin.URL)
	firstClone := sdkCloneConnection(t, clone, origin.URL)
	if firstClone == firstOriginal {
		t.Fatalf("original/clone native connection = %q/%q, want separate pools", firstOriginal, firstClone)
	}
	if err := clone.CloseIdleConnections(); err != nil {
		t.Fatalf("clone close error = %v, want nil", err)
	}
	secondOriginal := sdkCloneConnection(t, original, origin.URL)
	secondClone := sdkCloneConnection(t, clone, origin.URL)
	if secondOriginal != firstOriginal || secondClone == firstClone || secondClone == firstOriginal || originCalls.Load() != 0 || proxyCalls.Load() != 4 {
		t.Fatalf("native connections original = %q/%q clone = %q/%q origin/proxy calls = %d/%d, want original reuse, clone replacement, distinct pools and 0/4 calls", firstOriginal, secondOriginal, firstClone, secondClone, originCalls.Load(), proxyCalls.Load())
	}
}

func sdkCloneConnection(t *testing.T, client exchange.Client, target string) string {
	t.Helper()
	boundary, err := exchange.NewOfficialSDKMethodResponseBoundary(exchange.OfficialSDKMethodResponseBoundaryRequest{Method: exchange.MethodGet, Representation: exchange.OfficialSDKResponseRepresentationBinary})
	if err != nil {
		t.Fatal(err)
	}
	transport, err := client.OfficialSDKResponseTransport(boundary)
	if err != nil {
		t.Fatal(err)
	}
	native, err := exchange.NewOfficialSDKHTTPClient(transport)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := native.Do(request)
	if err != nil {
		t.Fatalf("SDK request error = %v, want nil through the configured proxy", err)
	}
	body, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		t.Fatalf("SDK body close/read = %v, want nil", err)
	}
	if response.StatusCode != http.StatusOK || len(body) == 0 {
		t.Fatalf("SDK response = (%d, %q), want nonempty native connection identity", response.StatusCode, body)
	}
	return string(body)
}
