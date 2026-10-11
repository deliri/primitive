package exchange

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"golang.org/x/net/http2"
)

func TestServerRuntimeHTTP2PolicyLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		allow bool
	}{
		{name: "cleartext HTTP2 absent by default"},
		{name: "explicit cleartext HTTP2 opt in", allow: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			policy := runtimeAgreementPolicy(t)
			policy.AllowUnencryptedHTTP2 = tc.allow
			server, err := newHTTPServer(policy, http.NotFoundHandler())
			if err != nil {
				t.Fatalf("newHTTPServer() error = %v, want nil", err)
			}
			if tc.allow {
				if server.Protocols == nil || !server.Protocols.HTTP1() || !server.Protocols.UnencryptedHTTP2() || server.Protocols.HTTP2() {
					t.Fatalf("explicit native protocols = %v, want HTTP1 and unencrypted HTTP2 only", server.Protocols)
				}
			} else if server.Protocols != nil {
				t.Fatalf("default native protocols = %v, want Go default nil", server.Protocols)
			}
		})
	}
	address, err := ParseListenAddress("127.0.0.1:0")
	if err != nil {
		t.Fatalf("ParseListenAddress() error = %v, want nil", err)
	}
	refused, err := NewServerRuntime(ServerRuntimeConfiguration{Address: address, Policy: ServerRuntimePolicy{AllowUnencryptedHTTP2: true}}, http.NotFoundHandler())
	if !errors.Is(err, core.ErrExchangeContract) || refused != nil {
		t.Fatalf("unset timeout/header policy = (%v, %v), want nil runtime and typed refusal", refused, err)
	}
}

func TestServerRuntimeNativeHTTP2OwnsConnectionExit(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	address, err := ParseListenAddress("127.0.0.1:0")
	if err != nil {
		t.Fatalf("ParseListenAddress() error = %v, want nil", err)
	}
	policy := runtimeAgreementPolicy(t)
	policy.AllowUnencryptedHTTP2 = true
	runtime, err := NewServerRuntime(ServerRuntimeConfiguration{Address: address, Policy: policy}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, "owned HTTP2 response")
	}))
	if err != nil {
		t.Fatalf("NewServerRuntime() error = %v, want nil", err)
	}
	done := make(chan struct{})
	var serveErr error
	go func() {
		defer close(done)
		serveErr = runtime.Serve()
	}()
	t.Cleanup(func() {
		if closeErr := runtime.Close(); closeErr != nil {
			t.Errorf("runtime.Close() error = %v, want nil", closeErr)
		}
		select {
		case <-done:
			if serveErr != nil {
				t.Errorf("owned HTTP2 serve exit = %v, want nil", serveErr)
			}
		case <-time.After(10 * time.Second):
			t.Error("owned HTTP2 serve exit = timeout, want terminal join")
		}
	})
	select {
	case readyErr := <-runtime.Ready():
		if readyErr != nil {
			t.Fatalf("listener readiness = %v, want nil", readyErr)
		}
	case <-ctx.Done():
		t.Fatalf("listener readiness = %v, want owned acquisition", ctx.Err())
	}
	bound, err := runtime.Address()
	if err != nil {
		t.Fatalf("runtime.Address() error = %v, want nil", err)
	}
	transport := &http2.Transport{AllowHTTP: true, DialTLSContext: func(ctx context.Context, network, address string, _ *tls.Config) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, address)
	}}
	t.Cleanup(transport.CloseIdleConnections)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+bound.String()+"/", nil)
	if err != nil {
		t.Fatalf("request construction error = %v, want nil", err)
	}
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatalf("native cleartext HTTP2 request error = %v, want nil", err)
	}
	var body bytes.Buffer
	_, readErr := io.Copy(&body, response.Body)
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil || response.ProtoMajor != 2 || response.StatusCode != http.StatusAccepted || body.String() != "owned HTTP2 response" {
		t.Fatalf("native HTTP2 response = (protocol %d, status %d, body %q, read %v, close %v), want (2, 202, exact owned body, nil, nil)", response.ProtoMajor, response.StatusCode, body.String(), readErr, closeErr)
	}
	if err := runtime.Close(); err != nil {
		t.Fatalf("native HTTP2 owner close = %v, want nil", err)
	}
	select {
	case <-done:
		if serveErr != nil {
			t.Fatalf("native HTTP2 owner exit = %v, want nil", serveErr)
		}
	case <-ctx.Done():
		t.Fatalf("native HTTP2 owner exit = %v, want terminal join", ctx.Err())
	}
	// The client remains open here: owner shutdown must close its existing
	// HTTP2 connection as well as the listener, rather than relying on cleanup.
	response, err = transport.RoundTrip(request)
	if response != nil {
		_ = response.Body.Close()
	}
	// witness:waiver test/errors -- HTTP2 may report EOF or a refused reconnect depending on when Go observes the closed native connection.
	if err == nil || response != nil {
		t.Fatalf("HTTP2 after owner exit = (%v, %v), want no response and native transport failure", response, err)
	}
}
