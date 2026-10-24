package exchange

import (
	"context"
	"net"
	"net/netip"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// TCPDialRequest selects one concrete TCP peer. Address is Go's nominal IP
// and port value, so dialing performs no hostname lookup. Unlike a listener,
// a peer cannot request an unspecified address or a zero allocated port.
type TCPDialRequest struct {
	Address netip.AddrPort
}

func (r TCPDialRequest) Validate() error {
	if !r.Address.IsValid() || r.Address.Port() == 0 || r.Address.Addr().WithZone("").Unmap().IsUnspecified() {
		return core.ErrExchangeContract
	}
	return nil
}

// DialTCP acquires one native TCP connection through Go's typed dial API.
// The caller owns the returned connection, including its eventual close;
// net/http may take that ownership through Transport.DialContext. Cancellation
// governs acquisition only, exactly as net.Dialer does, and does not close an
// acquired connection. TLS, HTTP authority and request signing stay with their
// existing owners. Native dial errors retain their net.OpError and cause.
func DialTCP(ctx context.Context, request TCPDialRequest) (net.Conn, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return nil, err
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	connection, err := (&net.Dialer{}).DialTCP(ctx, "tcp", netip.AddrPort{}, request.Address)
	if err != nil {
		return nil, transportError(err)
	}
	return connection, nil
}

var _ core.Validatable = TCPDialRequest{}
