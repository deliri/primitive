package exchange_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestServerRuntimeLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive owned listener serves and shuts down cleanly", func(t *testing.T) {
		t.Parallel()

		address := availableLoopbackAddress(t)
		configuration := serverRuntimeConfiguration(t, address)
		runtime, err := exchange.NewServerRuntime(configuration, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusNoContent)
		}))
		if err != nil {
			t.Fatalf("exchange.NewServerRuntime() error = %v, want nil", err)
		}
		served := make(chan error, 1)
		go func() { served <- runtime.Serve() }()
		waitForRuntimeReady(t, runtime.Ready())

		response, requestErr := http.Get("http://" + address)
		if requestErr != nil {
			t.Fatalf("http.Get(owned listener) error = %v, want nil", requestErr)
		}
		_, readErr := io.Copy(io.Discard, response.Body)
		closeErr := response.Body.Close()
		if readErr != nil || closeErr != nil || response.StatusCode != http.StatusNoContent {
			t.Fatalf("owned listener response = (status %d, read %v, close %v), want (204, nil, nil)", response.StatusCode, readErr, closeErr)
		}
		if err := runtime.Shutdown(t.Context()); err != nil {
			t.Fatalf("ServerRuntime.Shutdown() error = %v, want nil", err)
		}
		if err := waitForRuntimeExit(t, served); err != nil {
			t.Fatalf("ServerRuntime.Serve() error = %v, want nil after shutdown", err)
		}
	})

	t.Run("positive preopened listener transfers exactly once and closes idempotently", func(t *testing.T) {
		t.Parallel()

		address := availableLoopbackAddress(t)
		configuration := serverRuntimeConfiguration(t, address)
		listener, listenErr := exchange.Listen(configuration.Address)
		if listenErr != nil {
			t.Fatalf("exchange.Listen() error = %v, want nil", listenErr)
		}
		runtime, runtimeErr := exchange.NewServerRuntime(configuration, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusNoContent)
		}))
		if runtimeErr != nil {
			t.Fatalf("exchange.NewServerRuntime() error = %v, want nil", runtimeErr)
		}
		served := make(chan error, 1)
		go func() { served <- runtime.ServeListener(listener) }()
		waitForRuntimeReady(t, runtime.Ready())
		response, requestErr := http.Get("http://" + address)
		if requestErr != nil {
			t.Fatalf("http.Get(preopened listener) error = %v, want nil", requestErr)
		}
		_, readErr := io.Copy(io.Discard, response.Body)
		closeErr := response.Body.Close()
		if readErr != nil || closeErr != nil || response.StatusCode != http.StatusNoContent {
			t.Fatalf("preopened listener response = (status %d, read %v, close %v), want (204, nil, nil)", response.StatusCode, readErr, closeErr)
		}
		if gotErr := runtime.Shutdown(t.Context()); gotErr != nil {
			t.Fatalf("ServerRuntime.Shutdown() error = %v, want nil", gotErr)
		}
		if gotErr := waitForRuntimeExit(t, served); gotErr != nil {
			t.Fatalf("ServerRuntime.ServeListener() error = %v, want nil", gotErr)
		}
		if gotErr := listener.Close(); gotErr != nil {
			t.Fatalf("ServerListener.Close(after shutdown) error = %v, want nil", gotErr)
		}
	})

	t.Run("negative invalid address creates no runtime", func(t *testing.T) {
		t.Parallel()

		address, addressErr := exchange.ParseListenAddress("0.0.0.0:8080")
		if !errors.Is(addressErr, core.ErrExchangeContract) || address != (exchange.ListenAddress{}) {
			t.Fatalf("exchange.ParseListenAddress(unspecified) = (%v, %v), want zero and %v", address, addressErr, core.ErrExchangeContract)
		}
		_, gotErr := exchange.NewServerRuntime(exchange.ServerRuntimeConfiguration{}, http.NotFoundHandler())
		if !errors.Is(gotErr, core.ErrExchangeContract) {
			t.Fatalf("exchange.NewServerRuntime(zero configuration) error = %v, want %v", gotErr, core.ErrExchangeContract)
		}
	})

	t.Run("negative invalid transferred listener publishes one refusal for each serve attempt", func(t *testing.T) {
		t.Parallel()

		runtime, runtimeErr := exchange.NewServerRuntime(
			serverRuntimeConfiguration(t, availableLoopbackAddress(t)),
			http.NotFoundHandler(),
		)
		if runtimeErr != nil {
			t.Fatalf("exchange.NewServerRuntime() error = %v, want nil", runtimeErr)
		}
		for attempt := 1; attempt <= 2; attempt++ {
			gotErr := runtime.ServeListener(nil)
			readyErr := waitForRuntimeStart(t, runtime.Ready())
			if !errors.Is(gotErr, core.ErrExchangeContract) || !errors.Is(readyErr, core.ErrExchangeContract) {
				t.Fatalf("ServeListener(nil) attempt %d = (serve %v, ready %v), want %v from both", attempt, gotErr, readyErr, core.ErrExchangeContract)
			}
			select {
			case extra := <-runtime.Ready():
				t.Fatalf("ServeListener(nil) attempt %d extra readiness = %v, want exactly one result", attempt, extra)
			default:
			}
		}
	})

	t.Run("neutral construction opens no listener and rejects shutdown before serve", func(t *testing.T) {
		t.Parallel()

		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("net.Listen(reservation) error = %v, want nil", err)
		}
		t.Cleanup(func() {
			if closeErr := listener.Close(); closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
				t.Errorf("reservation listener Close() error = %v, want nil or %v", closeErr, net.ErrClosed)
			}
		})
		address := listener.Addr().String()
		runtime, runtimeErr := exchange.NewServerRuntime(serverRuntimeConfiguration(t, address), http.NotFoundHandler())
		if runtimeErr != nil {
			t.Fatalf("exchange.NewServerRuntime(dormant) error = %v, want nil", runtimeErr)
		}
		if gotErr := runtime.Shutdown(t.Context()); !errors.Is(gotErr, core.ErrExchangeContract) {
			t.Fatalf("ServerRuntime.Shutdown(before Serve) error = %v, want %v", gotErr, core.ErrExchangeContract)
		}
	})
}

func availableLoopbackAddress(t testing.TB) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen(loopback allocation) error = %v, want nil", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("listener.Close(loopback allocation) error = %v, want nil", err)
	}
	return address
}

func serverRuntimeConfiguration(t testing.TB, address string) exchange.ServerRuntimeConfiguration {
	t.Helper()
	parsed, addressErr := exchange.ParseListenAddress(address)
	readHeader, readHeaderErr := temporal.DurationFromSeconds(5)
	read, readErr := temporal.DurationFromSeconds(10)
	write, writeErr := temporal.DurationFromSeconds(10)
	idle, idleErr := temporal.DurationFromSeconds(30)
	headerBytes, headerErr := core.NewByteCount(32 * 1024)
	if err := errors.Join(addressErr, readHeaderErr, readErr, writeErr, idleErr, headerErr); err != nil {
		t.Fatalf("server runtime configuration error = %v, want nil", err)
	}
	configuration := exchange.ServerRuntimeConfiguration{
		Address: parsed,
		Policy: exchange.ServerRuntimePolicy{
			ReadHeaderTimeout:  readHeader,
			ReadTimeout:        read,
			WriteTimeout:       write,
			IdleTimeout:        idle,
			MaximumHeaderBytes: headerBytes,
		},
	}
	if err := configuration.Validate(); err != nil {
		t.Fatalf("ServerRuntimeConfiguration.Validate() error = %v, want nil", err)
	}
	return configuration
}

func waitForRuntimeReady(t testing.TB, ready <-chan error) {
	t.Helper()
	if err := waitForRuntimeStart(t, ready); err != nil {
		t.Fatalf("ServerRuntime listener acquisition error = %v, want nil", err)
	}
}

func waitForRuntimeStart(t testing.TB, ready <-chan error) error {
	t.Helper()
	select {
	case err := <-ready:
		return err
	case <-time.After(10 * time.Second):
		t.Fatalf("ServerRuntime readiness facts received = %d after %v, want 1", 0, 10*time.Second)
		return context.DeadlineExceeded
	}
}

func waitForRuntimeExit(t testing.TB, served <-chan error) error {
	t.Helper()
	select {
	case err := <-served:
		return err
	case <-time.After(10 * time.Second):
		t.Fatalf("ServerRuntime exit facts received = %d after %v, want 1", 0, 10*time.Second)
		return context.DeadlineExceeded
	}
}
