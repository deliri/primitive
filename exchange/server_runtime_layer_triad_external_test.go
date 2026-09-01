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

	t.Run("neutral construction opens no listener and rejects shutdown before serve", func(t *testing.T) {
		t.Parallel()

		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("net.Listen(reservation) error = %v, want nil", err)
		}
		defer listener.Close()
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

func waitForRuntimeReady(t testing.TB, ready <-chan struct{}) {
	t.Helper()
	select {
	case <-ready:
	case <-time.After(10 * time.Second):
		t.Fatal("ServerRuntime readiness fact did not arrive")
	}
}

func waitForRuntimeExit(t testing.TB, served <-chan error) error {
	t.Helper()
	select {
	case err := <-served:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("ServerRuntime did not exit after shutdown")
		return context.DeadlineExceeded
	}
}
