package exchange_test

import (
	"errors"
	"net"
	"net/http"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type listenAddressCaseClass uint8

const (
	listenAddressClassUnknown listenAddressCaseClass = iota
	listenAddressClassValid
	listenAddressClassReject
	listenAddressClassBoundary
)

func TestParseListenAddressHostileBoundaries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		text      string
		wantText  string
		wantErr   error
		caseClass listenAddressCaseClass
	}{
		{name: "valid IPv4 loopback service port", text: "127.0.0.1:8080", wantText: "127.0.0.1:8080", caseClass: listenAddressClassValid},
		{name: "valid IPv4 private ten network", text: "10.42.0.4:8080", wantText: "10.42.0.4:8080", caseClass: listenAddressClassValid},
		{name: "valid IPv4 private one-seven-two network", text: "172.16.0.1:443", wantText: "172.16.0.1:443", caseClass: listenAddressClassValid},
		{name: "valid IPv4 private one-nine-two network", text: "192.168.1.1:8443", wantText: "192.168.1.1:8443", caseClass: listenAddressClassValid},
		{name: "valid IPv4 public address", text: "203.0.113.10:9000", wantText: "203.0.113.10:9000", caseClass: listenAddressClassValid},
		{name: "valid IPv4 link local address", text: "169.254.1.1:9090", wantText: "169.254.1.1:9090", caseClass: listenAddressClassValid},
		{name: "valid IPv6 loopback address", text: "[::1]:8080", wantText: "[::1]:8080", caseClass: listenAddressClassValid},
		{name: "valid IPv6 global address", text: "[2001:db8::1]:443", wantText: "[2001:db8::1]:443", caseClass: listenAddressClassValid},
		{name: "valid IPv6 unique local address", text: "[fd00::1]:8443", wantText: "[fd00::1]:8443", caseClass: listenAddressClassValid},
		{name: "valid IPv6 mapped IPv4 address", text: "[::ffff:192.0.2.1]:8080", wantText: "[::ffff:192.0.2.1]:8080", caseClass: listenAddressClassValid},

		{name: "reject empty input", text: "", wantErr: core.ErrExchangeContract, caseClass: listenAddressClassReject},
		{name: "reject hostname instead of literal address", text: "localhost:8080", wantErr: core.ErrExchangeContract, caseClass: listenAddressClassReject},
		{name: "reject IPv4 without port", text: "127.0.0.1", wantErr: core.ErrExchangeContract, caseClass: listenAddressClassReject},
		{name: "reject IPv6 without brackets", text: "::1:8080", wantErr: core.ErrExchangeContract, caseClass: listenAddressClassReject},
		{name: "reject negative port", text: "127.0.0.1:-1", wantErr: core.ErrExchangeContract, caseClass: listenAddressClassReject},
		{name: "reject port beyond uint sixteen", text: "127.0.0.1:65536", wantErr: core.ErrExchangeContract, caseClass: listenAddressClassReject},
		{name: "reject alphabetic port", text: "127.0.0.1:http", wantErr: core.ErrExchangeContract, caseClass: listenAddressClassReject},
		{name: "reject truncated IPv6 bracket", text: "[::1:8080", wantErr: core.ErrExchangeContract, caseClass: listenAddressClassReject},
		{name: "reject leading whitespace", text: " 127.0.0.1:8080", wantErr: core.ErrExchangeContract, caseClass: listenAddressClassReject},
		{name: "reject trailing material", text: "127.0.0.1:8080/path", wantErr: core.ErrExchangeContract, caseClass: listenAddressClassReject},

		{name: "boundary minimum port is admitted", text: "127.0.0.1:1", wantText: "127.0.0.1:1", caseClass: listenAddressClassBoundary},
		{name: "boundary one above minimum port is admitted", text: "127.0.0.1:2", wantText: "127.0.0.1:2", caseClass: listenAddressClassBoundary},
		{name: "boundary one below maximum port is admitted", text: "127.0.0.1:65534", wantText: "127.0.0.1:65534", caseClass: listenAddressClassBoundary},
		{name: "boundary maximum port is admitted", text: "127.0.0.1:65535", wantText: "127.0.0.1:65535", caseClass: listenAddressClassBoundary},
		{name: "boundary zero port is rejected", text: "127.0.0.1:0", wantErr: core.ErrExchangeContract, caseClass: listenAddressClassBoundary},
		{name: "boundary unspecified IPv4 is rejected", text: "0.0.0.0:8080", wantErr: core.ErrExchangeContract, caseClass: listenAddressClassBoundary},
		{name: "boundary unspecified IPv6 is rejected", text: "[::]:8080", wantErr: core.ErrExchangeContract, caseClass: listenAddressClassBoundary},
		{name: "boundary lowest nonzero IPv4 literal is admitted", text: "0.0.0.1:8080", wantText: "0.0.0.1:8080", caseClass: listenAddressClassBoundary},
		{name: "boundary highest IPv4 literal is admitted", text: "255.255.255.255:8080", wantText: "255.255.255.255:8080", caseClass: listenAddressClassBoundary},
		{name: "boundary IPv4 octet overflow is rejected", text: "256.0.0.1:8080", wantErr: core.ErrExchangeContract, caseClass: listenAddressClassBoundary},
		{name: "boundary shortest canonical IPv6 loopback is admitted", text: "[::1]:1", wantText: "[::1]:1", caseClass: listenAddressClassBoundary},
		{name: "boundary expanded IPv6 canonicalizes", text: "[0:0:0:0:0:0:0:1]:8080", wantText: "[::1]:8080", caseClass: listenAddressClassBoundary},
		{name: "boundary highest IPv6 literal is admitted", text: "[ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff]:65535", wantText: "[ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff]:65535", caseClass: listenAddressClassBoundary},
		{name: "boundary IPv6 group overflow is rejected", text: "[10000::1]:8080", wantErr: core.ErrExchangeContract, caseClass: listenAddressClassBoundary},
		{name: "boundary IPv6 zone is retained", text: "[fe80::1%eth0]:8080", wantText: "[fe80::1%eth0]:8080", caseClass: listenAddressClassBoundary},
		{name: "boundary empty IPv6 zone is rejected", text: "[fe80::1%]:8080", wantErr: core.ErrExchangeContract, caseClass: listenAddressClassBoundary},
		{name: "boundary embedded IPv4 zero host remains concrete", text: "[::ffff:0.0.0.0]:8080", wantText: "[::ffff:0.0.0.0]:8080", caseClass: listenAddressClassBoundary},
		{name: "boundary embedded IPv4 maximum host is admitted", text: "[::ffff:255.255.255.255]:8080", wantText: "[::ffff:255.255.255.255]:8080", caseClass: listenAddressClassBoundary},
		{name: "boundary plus-prefixed port is rejected", text: "127.0.0.1:+8080", wantErr: core.ErrExchangeContract, caseClass: listenAddressClassBoundary},
		{name: "boundary leading-zero port canonicalizes", text: "127.0.0.1:08080", wantText: "127.0.0.1:8080", caseClass: listenAddressClassBoundary},
	}

	counts := [4]int{}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := exchange.ParseListenAddress(testCase.text)
			if !errors.Is(gotErr, testCase.wantErr) {
				t.Fatalf("exchange.ParseListenAddress(%q) error = %v, want %v", testCase.text, gotErr, testCase.wantErr)
			}
			if testCase.wantErr != nil {
				if got != (exchange.ListenAddress{}) {
					t.Fatalf("exchange.ParseListenAddress(%q) = %v, want zero after rejection", testCase.text, got)
				}
				return
			}
			if got.String() != testCase.wantText {
				t.Fatalf("exchange.ParseListenAddress(%q).String() = %q, want %q", testCase.text, got.String(), testCase.wantText)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("exchange.ParseListenAddress(%q).Validate() error = %v, want nil", testCase.text, err)
			}
		})
		counts[testCase.caseClass]++
	}
	if counts[listenAddressClassValid] != 10 || counts[listenAddressClassReject] != 10 || counts[listenAddressClassBoundary] != 20 {
		t.Fatalf("listen-address hostile classes = valid %d reject %d boundary %d, want 10/10/20", counts[listenAddressClassValid], counts[listenAddressClassReject], counts[listenAddressClassBoundary])
	}
}

func TestServerRuntimeRefusesDuplicateServeAndOccupiedAddress(t *testing.T) {
	t.Parallel()

	t.Run("duplicate serve is a typed contract rejection", func(t *testing.T) {
		t.Parallel()

		address := availableLoopbackAddress(t)
		runtime, err := exchange.NewServerRuntime(serverRuntimeConfiguration(t, address), http.NotFoundHandler())
		if err != nil {
			t.Fatalf("exchange.NewServerRuntime() error = %v, want nil", err)
		}
		served := make(chan error, 1)
		go func() { served <- runtime.Serve() }()
		waitForRuntimeReady(t, runtime.Ready())
		if gotErr := runtime.Serve(); !errors.Is(gotErr, core.ErrExchangeContract) {
			t.Fatalf("ServerRuntime.Serve(second call) error = %v, want %v", gotErr, core.ErrExchangeContract)
		}
		if err := runtime.Shutdown(t.Context()); err != nil {
			t.Fatalf("ServerRuntime.Shutdown() error = %v, want nil", err)
		}
		if err := waitForRuntimeExit(t, served); err != nil {
			t.Fatalf("ServerRuntime.Serve(first call) error = %v, want nil", err)
		}
	})

	t.Run("occupied address retains the transport failure", func(t *testing.T) {
		t.Parallel()

		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("net.Listen(occupied fixture) error = %v, want nil", err)
		}
		defer listener.Close()
		runtime, runtimeErr := exchange.NewServerRuntime(serverRuntimeConfiguration(t, listener.Addr().String()), http.NotFoundHandler())
		if runtimeErr != nil {
			t.Fatalf("exchange.NewServerRuntime(occupied address) error = %v, want nil before effect", runtimeErr)
		}
		if gotErr := runtime.Serve(); !errors.Is(gotErr, core.ErrExchangeTransport) {
			t.Fatalf("ServerRuntime.Serve(occupied address) error = %v, want %v", gotErr, core.ErrExchangeTransport)
		}
	})
}

func FuzzParseListenAddressSemanticClosure(f *testing.F) {
	valid, err := exchange.ParseListenAddress("127.0.0.1:8080")
	if err != nil {
		f.Fatalf("exchange.ParseListenAddress(seed) error = %v, want nil", err)
	}
	f.Add(valid.String())
	f.Add("[::1]:65535")
	f.Add("")
	f.Add("0.0.0.0:0")
	f.Add("localhost:8080")

	f.Fuzz(func(t *testing.T, text string) {
		got, gotErr := exchange.ParseListenAddress(text)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrExchangeContract) || got != (exchange.ListenAddress{}) {
				t.Fatalf("exchange.ParseListenAddress(%q) = (%v, %v), want zero and %v", text, got, gotErr, core.ErrExchangeContract)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("exchange.ParseListenAddress(%q).Validate() error = %v, want nil", text, err)
		}
		canonical := got.String()
		roundTrip, roundTripErr := exchange.ParseListenAddress(canonical)
		if roundTripErr != nil || roundTrip != got {
			t.Fatalf("listen address canonical closure = (%v, %v), want (%v, nil)", roundTrip, roundTripErr, got)
		}
	})
}
