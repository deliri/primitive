package googleidentity_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/googleidentity"
)

func TestMetadataIdleCleanupOwnsDerivedNativePool(t *testing.T) {
	t.Parallel()
	closed := make(chan string, 8)
	metadataConnections := make(chan string, 4)
	peer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Native-Connection", r.RemoteAddr)
		if r.URL.Path == "/caller" {
			_, err := io.WriteString(w, "caller")
			if err != nil {
				t.Error(err)
			}
			return
		}
		if r.URL.Path != "/computeMetadata/v1/instance/service-accounts/default/identity" || r.URL.Query().Get("audience") != "https://receiver.invalid" || r.Header.Get("Metadata-Flavor") != "Google" {
			t.Errorf("metadata wire = %q/%q, want SDK identity acquisition", r.URL.String(), r.Header.Get("Metadata-Flavor"))
		}
		metadataConnections <- r.RemoteAddr
		w.Header().Set("Metadata-Flavor", "Google")
		if _, err := io.WriteString(w, "synthetic.metadata-token"); err != nil {
			t.Error(err)
		}
	}))
	peer.Config.ConnState = func(connection net.Conn, state http.ConnState) {
		if state == http.StateClosed {
			closed <- connection.RemoteAddr().String()
		}
	}
	peer.Start()
	t.Cleanup(peer.Close)
	network := http.DefaultTransport.(*http.Transport).Clone()
	network.IdleConnTimeout = 0
	var proxyCalls atomic.Int32
	network.Proxy = func(*http.Request) (*url.URL, error) {
		proxyCalls.Add(1)
		return nil, nil
	}
	dialer := &net.Dialer{}
	network.DialContext = func(ctx context.Context, kind, address string) (net.Conn, error) {
		if address == "metadata.google.internal:80" {
			address = strings.TrimPrefix(peer.URL, "http://")
		}
		return dialer.DialContext(ctx, kind, address)
	}
	t.Cleanup(network.CloseIdleConnections)
	native := &http.Client{Transport: network}
	caller, err := exchange.NewClient(native)
	if err != nil {
		t.Fatal(err)
	}
	readCaller := func() string {
		t.Helper()
		response, err := native.Get(peer.URL + "/caller")
		if err != nil {
			t.Fatal(err)
		}
		body, readErr := io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		if err := errors.Join(readErr, closeErr); err != nil || string(body) != "caller" {
			t.Fatalf("caller response = (%q,%v), want exact caller body", body, err)
		}
		return response.Header.Get("X-Native-Connection")
	}
	callerConnection := readCaller()
	client, err := googleidentity.NewClient(caller)
	if err != nil {
		t.Fatal(err)
	}
	copyOfClient := client
	audience, err := googleidentity.ParseAudience("https://receiver.invalid")
	if err != nil {
		t.Fatal(err)
	}
	policy, err := googleidentity.DefaultPolicy()
	if err != nil {
		t.Fatal(err)
	}
	request := googleidentity.IdentityTokenRequest{Audience: audience, Policy: policy}
	acquire := func(source googleidentity.Client) string {
		t.Helper()
		idle := make(chan error, 1)
		connections := make(chan bool, 1)
		ctx := httptrace.WithClientTrace(t.Context(), &httptrace.ClientTrace{GotConn: func(info httptrace.GotConnInfo) { connections <- info.Reused }, PutIdleConn: func(err error) { idle <- err }})
		got, err := googleidentity.AcquireGoogleCloud(ctx, source, request)
		if err != nil || got.Validate() != nil {
			t.Fatalf("real SDK acquisition = (%v,%v), want valid token", got, err)
		}
		select {
		case err := <-idle:
			if err != nil {
				t.Fatalf("native metadata idle admission = %v, want nil", err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("native metadata connection never became idle")
		}
		if reused := <-connections; reused {
			t.Fatal("metadata connection reused after explicit cleanup, want fresh native connection")
		}
		return <-metadataConnections
	}
	derivedConnection := acquire(client)
	if derivedConnection == callerConnection || proxyCalls.Load() != 1 || network.Proxy == nil {
		t.Fatalf("derived ownership = (same connection %t,proxy calls %d,proxy present %t), want separate pool and preserved caller proxy", derivedConnection == callerConnection, proxyCalls.Load(), network.Proxy != nil)
	}
	if err := client.CloseIdleConnections(); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-closed:
		if got != derivedConnection {
			t.Fatalf("native closed connection = %q, want derived %q; caller %q must stay open", got, derivedConnection, callerConnection)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("derived metadata idle connection remained open after cleanup")
	}
	if got := readCaller(); got != callerConnection || proxyCalls.Load() != 2 {
		t.Fatalf("caller pool after derived cleanup = (%q,%d proxy calls), want original %q and2", got, proxyCalls.Load(), callerConnection)
	}
	if err := client.CloseIdleConnections(); err != nil || client.Validate() != nil {
		t.Fatalf("idle cleanup replay = %v, capability validation = %v, want nil without copy invalidation", err, client.Validate())
	}
	_ = acquire(copyOfClient)
	if err := copyOfClient.CloseIdleConnections(); err != nil {
		t.Fatal(err)
	}
}

func TestMetadataIdleCleanupZeroCapabilityRefuses(t *testing.T) {
	t.Parallel()
	if err := (googleidentity.Client{}).CloseIdleConnections(); !errors.Is(err, core.ErrGoogleIdentityContract) || !errors.Is(err, core.ErrExchangeContract) {
		t.Fatalf("zero metadata cleanup = %v, want identity and Exchange contract causes", err)
	}
}
