package exchange_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestCloseIdleConnectionsRevalidatesNativeClient(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name            string
		zero            bool
		competingTimeout bool
	}{
		{name: "zero client refuses without native dereference", zero: true},
		{name: "borrowed native client with changed timeout refuses", competingTimeout: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var client exchange.Client
			if !tc.zero {
				network := http.DefaultTransport.(*http.Transport).Clone()
				t.Cleanup(network.CloseIdleConnections)
				native := &http.Client{Transport: network}
				var err error
				client, err = exchange.NewClient(native)
				if err != nil {
					t.Fatal(err)
				}
				if tc.competingTimeout {
					native.Timeout = time.Second
				}
			}
			if err := client.CloseIdleConnections(); !errors.Is(err, core.ErrExchangeContract) {
				t.Fatalf("invalid cleanup = %v, want Exchange contract", err)
			}
		})
	}
}

func TestCloseIdleConnectionsPreservesActiveNativeCall(t *testing.T) {
	t.Parallel()
	entered := make(chan struct{})
	release := make(chan struct{})
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		select {
		case <-release:
			if _, err := io.WriteString(w, "active"); err != nil {
				t.Error(err)
			}
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(peer.Close)
	network := http.DefaultTransport.(*http.Transport).Clone()
	network.Proxy = nil
	t.Cleanup(network.CloseIdleConnections)
	native := &http.Client{Transport: network}
	client, err := exchange.NewClient(native)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	joined := make(chan struct{})
	go func() {
		defer close(joined)
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, peer.URL, nil)
		if err != nil {
			done <- err
			return
		}
		response, err := native.Do(request)
		if err != nil {
			done <- err
			return
		}
		body, readErr := io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		if string(body) != "active" {
			done <- errors.Join(core.ErrExchangeResponse, readErr, closeErr)
			return
		}
		done <- errors.Join(readErr, closeErr)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-joined:
		case <-time.After(10 * time.Second):
			t.Error("owned native call did not join after cancellation")
		}
	})
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("native call never entered provider")
	}
	if err := client.CloseIdleConnections(); err != nil {
		t.Fatal(err)
	}
	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("active native call after cleanup = %v, want exact completed response", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("active native call failed to complete after provider release")
	}
}
