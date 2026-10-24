package exchange_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/netip"
	"syscall"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestTCPDialRequestConcretePeerLayerTriad(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, address string
		wantErr       error
	}{
		{name: "zero value has no peer", wantErr: core.ErrExchangeContract},
		{name: "listener allocation intent is not a peer", address: "127.0.0.1:0", wantErr: core.ErrExchangeContract},
		{name: "IPv4 wildcard is not a concrete peer", address: "0.0.0.0:80", wantErr: core.ErrExchangeContract},
		{name: "IPv6 wildcard is not a concrete peer", address: "[::]:80", wantErr: core.ErrExchangeContract},
		{name: "IPv4 mapping does not hide wildcard", address: "[::ffff:0.0.0.0]:80", wantErr: core.ErrExchangeContract},
		{name: "zone does not hide IPv6 wildcard", address: "[::%loopback]:80", wantErr: core.ErrExchangeContract},
		{name: "zone and mapping do not hide IPv4 wildcard", address: "[::ffff:0.0.0.0%loopback]:80", wantErr: core.ErrExchangeContract},
		{name: "lowest nonzero peer port is admitted", address: "127.0.0.1:1"},
		{name: "highest peer port is admitted", address: "127.0.0.1:65535"},
		{name: "IPv6 loopback remains concrete", address: "[::1]:443"},
		{name: "mapped concrete IPv4 is retained", address: "[::ffff:127.0.0.1]:443"},
		{name: "scoped link local address is retained", address: "[fe80::1%example]:443"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			var address netip.AddrPort
			if row.address != "" {
				address = netip.MustParseAddrPort(row.address)
			}
			request := exchange.TCPDialRequest{Address: address}
			if err := request.Validate(); !errors.Is(err, row.wantErr) || request.Address != address {
				t.Fatalf("Validate(%v) = %v, address %v, want %v and unchanged %v", address, err, request.Address, row.wantErr, address)
			}
		})
	}
}

// Acquisition cancellation must not become an undocumented connection lifetime.
// Both directions carry exact binary bytes after cancellation; no sleeps or
// asynchronous accept worker are needed for this native TCP observation.
func TestDialTCPNativeAcquisitionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, family := range []struct {
		name, network string
		loopback      netip.Addr
	}{
		{name: "positive IPv4 connection survives acquisition context cancellation", network: "tcp4", loopback: netip.AddrFrom4([4]byte{127, 0, 0, 1})},
		{name: "positive IPv6 connection survives acquisition context cancellation", network: "tcp6", loopback: netip.IPv6Loopback()},
	} {
		t.Run(family.name, func(t *testing.T) {
			t.Parallel()
			listener, err := net.ListenTCP(family.network, &net.TCPAddr{IP: family.loopback.AsSlice()})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = listener.Close() })
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			deadline, _ := ctx.Deadline()
			if err := listener.SetDeadline(deadline); err != nil {
				t.Fatal(err)
			}
			address, err := netip.ParseAddrPort(listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			connection, err := exchange.DialTCP(ctx, exchange.TCPDialRequest{Address: address})
			if err != nil || connection == nil {
				t.Fatalf("DialTCP(%v) = (%v, %v), want acquired connection", address, connection, err)
			}
			t.Cleanup(func() { _ = connection.Close() })
			peer, err := listener.AcceptTCP()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = peer.Close() })
			if err := errors.Join(connection.SetDeadline(deadline), peer.SetDeadline(deadline)); err != nil {
				t.Fatal(err)
			}
			cancel()
			payload := []byte{0, 1, 255, 127, '\n'}
			if n, err := connection.Write(payload); err != nil || n != len(payload) {
				t.Fatalf("client Write after cancel = (%d,%v), want %d bytes", n, err, len(payload))
			}
			got := make([]byte, len(payload))
			if n, err := io.ReadFull(peer, got); err != nil || n != len(payload) || !bytes.Equal(got, payload) {
				t.Fatalf("peer Read = (%x,%d,%v), want %x", got, n, err, payload)
			}
			if n, err := peer.Write(payload); err != nil || n != len(payload) {
				t.Fatalf("peer Write = (%d,%v), want %d bytes", n, err, len(payload))
			}
			if n, err := io.ReadFull(connection, got); err != nil || n != len(payload) || !bytes.Equal(got, payload) {
				t.Fatalf("client Read after cancel = (%x,%d,%v), want %x", got, n, err, payload)
			}
		})
	}
	t.Run("negative refused native peer preserves dial cause and no connection", func(t *testing.T) {
		t.Parallel()
		listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
		if err != nil {
			t.Fatal(err)
		}
		address, err := netip.ParseAddrPort(listener.Addr().String())
		if err := errors.Join(err, listener.Close()); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
		defer cancel()
		connection, err := exchange.DialTCP(ctx, exchange.TCPDialRequest{Address: address})
		var native *net.OpError
		if connection != nil || !errors.Is(err, core.ErrExchangeTransport) || !errors.Is(err, syscall.ECONNREFUSED) || !errors.As(err, &native) || native.Op != "dial" || native.Net != "tcp" {
			t.Fatalf("DialTCP refused peer = (%v,%v), native %+v, want nil with transport and native ECONNREFUSED", connection, err, native)
		}
	})
	t.Run("neutral absent peer is refused before acquisition", func(t *testing.T) {
		t.Parallel()
		connection, err := exchange.DialTCP(t.Context(), exchange.TCPDialRequest{})
		if connection != nil || !errors.Is(err, core.ErrExchangeContract) {
			t.Fatalf("DialTCP zero = (%v,%v), want nil and %v", connection, err, core.ErrExchangeContract)
		}
	})
}

func TestDialTCPStoppedContextReturnsNoConnection(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name    string
		context func(context.Context) (context.Context, context.CancelFunc)
		wantErr error
	}{
		{name: "nil context is a typed refusal", context: func(context.Context) (context.Context, context.CancelFunc) { return nil, func() {} }, wantErr: core.ErrNilContext},
		{name: "canceled context cannot acquire", context: func(parent context.Context) (context.Context, context.CancelFunc) {
			ctx, cancel := context.WithCancel(parent)
			cancel()
			return ctx, cancel
		}, wantErr: context.Canceled},
		{name: "expired context cannot acquire", context: func(parent context.Context) (context.Context, context.CancelFunc) {
			return context.WithDeadline(parent, time.Time{})
		}, wantErr: context.DeadlineExceeded},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := row.context(t.Context())
			defer cancel()
			connection, err := exchange.DialTCP(ctx, exchange.TCPDialRequest{Address: netip.MustParseAddrPort("127.0.0.1:1")})
			if connection != nil || !errors.Is(err, row.wantErr) {
				t.Fatalf("DialTCP stopped context = (%v,%v), want nil and %v", connection, err, row.wantErr)
			}
		})
	}
}
