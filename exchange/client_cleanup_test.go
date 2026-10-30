package exchange_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type cleanupCountingTransport struct {
	closes atomic.Int32
	trips  atomic.Int32
}

func (t *cleanupCountingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	t.trips.Add(1)
	return nil, core.ErrExchangeTransport
}
func (t *cleanupCountingTransport) CloseIdleConnections() { t.closes.Add(1) }

type cleanupRequestOnlyTransport struct{ trips atomic.Int32 }

func (t *cleanupRequestOnlyTransport) RoundTrip(*http.Request) (*http.Response, error) {
	t.trips.Add(1)
	return nil, core.ErrExchangeTransport
}

func TestHTTPClientCleanupRequestLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		client        func() *http.Client
		wantErr       error
		admissionOnly bool
	}{
		{name: "missing native client refuses before dereference", client: func() *http.Client { return nil }, wantErr: core.ErrExchangeContract},
		{name: "typed nil standard transport refuses before native cleanup", client: func() *http.Client { return &http.Client{Transport: (*http.Transport)(nil)} }, wantErr: core.ErrExchangeContract},
		{name: "typed nil custom cleanup transport refuses before receiver call", client: func() *http.Client { return &http.Client{Transport: (*cleanupCountingTransport)(nil)} }, wantErr: core.ErrExchangeContract},
		{name: "nil transport admits Go shared default without exercising unowned pool", client: func() *http.Client { return &http.Client{} }, admissionOnly: true},
		{name: "finite timeout keeps exact custom transport and request policy", client: func() *http.Client { return &http.Client{Transport: &cleanupCountingTransport{}, Timeout: time.Second} }},
		{name: "zero timeout delegates optional custom cleanup", client: func() *http.Client { return &http.Client{Transport: &cleanupCountingTransport{}} }},
		{name: "custom transport without cleanup method remains native no effect", client: func() *http.Client {
			return &http.Client{Transport: &cleanupRequestOnlyTransport{}, Timeout: time.Second}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			native := tc.client()
			request := exchange.HTTPClientCleanupRequest{Client: native}
			if err := request.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("cleanup admission = %v, want %v", err, tc.wantErr)
			}
			if tc.admissionOnly {
				return // Native nil-transport pool semantics are source-qualified, not executed on a shared pool.
			}
			var timeout time.Duration
			var transport http.RoundTripper
			if native != nil {
				timeout, transport = native.Timeout, native.Transport
			}
			if err := exchange.CloseHTTPClientIdleConnections(request); !errors.Is(err, tc.wantErr) {
				t.Fatalf("cleanup execution = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				return
			}
			if native.Timeout != timeout || native.Transport != transport || request.Client != native {
				t.Fatalf("native timeout/transport/client identity = %v/%t/%t, want %v/true/true", native.Timeout, native.Transport == transport, request.Client == native, timeout)
			}
			switch owned := transport.(type) {
			case *cleanupCountingTransport:
				if owned.closes.Load() != 1 || owned.trips.Load() != 0 {
					t.Fatalf("native custom cleanup/request calls = %d/%d, want 1/0", owned.closes.Load(), owned.trips.Load())
				}
			case *cleanupRequestOnlyTransport:
				if owned.trips.Load() != 0 {
					t.Fatalf("native request calls during no-method cleanup = %d, want 0", owned.trips.Load())
				}
			}
			if timeout != 0 {
				if _, err := exchange.NewClient(native); !errors.Is(err, core.ErrExchangeContract) {
					t.Fatalf("unchanged request-client timeout admission = %v, want Exchange contract", err)
				}
			}
		})
	}
}

func TestHTTPClientCleanupClosesNativeIdleAfterOperationCancellation(t *testing.T) {
	t.Parallel()
	idle, closed := make(chan struct{}), make(chan struct{})
	var idleOnce, closedOnce sync.Once
	peer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := io.WriteString(w, "idle"); err != nil {
			t.Errorf("loopback response write = %v, want nil", err)
		}
	}))
	peer.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		switch state {
		case http.StateIdle:
			idleOnce.Do(func() { close(idle) })
		case http.StateClosed:
			closedOnce.Do(func() { close(closed) })
		}
	}
	peer.Start()
	t.Cleanup(peer.Close)
	network := &http.Transport{}
	t.Cleanup(network.CloseIdleConnections)
	native := &http.Client{Transport: network, Timeout: 5 * time.Second}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, peer.URL, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	response, err := native.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	var body [4]byte
	_, readErr := io.ReadFull(response.Body, body[:])
	_, drainErr := io.Copy(io.Discard, response.Body)
	closeErr := response.Body.Close()
	if readErr != nil || drainErr != nil || closeErr != nil || string(body[:]) != "idle" {
		t.Fatalf("native response = %q/%v/%v/%v, want idle/nil/nil/nil", body, readErr, drainErr, closeErr)
	}
	select {
	case <-idle:
	case <-time.After(10 * time.Second):
		t.Fatal("native connection idle = false, want observed idle state")
	}
	cancel()
	select {
	case <-closed:
		t.Fatal("native connection closed before cleanup = true, want false")
	default:
	}
	if err := exchange.CloseHTTPClientIdleConnections(exchange.HTTPClientCleanupRequest{Client: native}); err != nil {
		t.Fatalf("finite timeout cleanup after operation cancellation = %v, want nil", err)
	}
	if native.Timeout != 5*time.Second || native.Transport != network {
		t.Fatalf("native timeout/transport identity = %v/%t, want 5s/true", native.Timeout, native.Transport == network)
	}
	select {
	case <-closed:
	case <-time.After(10 * time.Second):
		t.Fatal("native connection closed after cleanup = false, want server-observed closure")
	}
	if err := exchange.CloseHTTPClientIdleConnections(exchange.HTTPClientCleanupRequest{Client: native}); err != nil {
		t.Fatalf("repeated cleanup of closed idle pool = %v, want nil", err)
	}
}

func TestHTTPClientCleanupPreservesNativeActiveResponse(t *testing.T) {
	t.Parallel()
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		select {
		case <-release:
			if _, err := io.WriteString(w, "active"); err != nil {
				t.Errorf("active loopback write = %v, want nil", err)
			}
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(peer.Close)
	network := &http.Transport{}
	t.Cleanup(network.CloseIdleConnections)
	native := &http.Client{Transport: network, Timeout: 5 * time.Second}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	defer releaseOnce.Do(func() { close(release) })
	done, joined := make(chan error, 1), make(chan struct{})
	go func() {
		defer close(joined)
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, peer.URL, http.NoBody)
		if err != nil {
			done <- err
			return
		}
		response, err := native.Do(request)
		if err != nil {
			done <- err
			return
		}
		var body [6]byte
		_, readErr := io.ReadFull(response.Body, body[:])
		_, drainErr := io.Copy(io.Discard, response.Body)
		closeErr := response.Body.Close()
		if string(body[:]) != "active" {
			done <- errors.Join(core.ErrExchangeResponse, readErr, drainErr, closeErr)
			return
		}
		done <- errors.Join(readErr, drainErr, closeErr)
	}()
	t.Cleanup(func() {
		cancel()
		releaseOnce.Do(func() { close(release) })
		select {
		case <-joined:
		case <-time.After(10 * time.Second):
			t.Error("native active caller joined = false, want true after cancellation")
		}
	})
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("active handler entered = false, want observed barrier")
	}
	if err := exchange.CloseHTTPClientIdleConnections(exchange.HTTPClientCleanupRequest{Client: native}); err != nil {
		t.Fatalf("native active cleanup = %v, want nil", err)
	}
	select {
	case err := <-done:
		t.Fatalf("active call completed before release = %v, want blocked handler", err)
	default:
	}
	releaseOnce.Do(func() { close(release) })
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("native active response after cleanup = %v, want exact response and nil", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("native response completed = false, want completion after handler release")
	}
}
