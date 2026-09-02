package exchange

import (
	"errors"
	"net/http"
	"net/netip"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// TrustedProxyMaximumCount bounds the fixed trusted-proxy prefix set.
	TrustedProxyMaximumCount = 64

	clientAddressAuthorityPeerText         = "peer"
	clientAddressAuthorityTrustedProxyText = "trusted proxy"
	clientAddressAuthorityGoogleCloudText  = "google cloud"
)

// ClientAddressAuthority is the closed caller decision for which HTTP
// transport fact may identify the upstream client. The zero value deliberately
// means the direct net/http peer so omitted policy cannot trust a header.
type ClientAddressAuthority uint8

const (
	// ClientAddressAuthorityPeer admits only Request.RemoteAddr.
	ClientAddressAuthorityPeer ClientAddressAuthority = iota
	// ClientAddressAuthorityTrustedProxy admits X-Forwarded-For only when the
	// direct peer belongs to the caller's fixed trusted-prefix set.
	ClientAddressAuthorityTrustedProxy
	// ClientAddressAuthorityGoogleCloud admits the documented Google Cloud
	// final X-Forwarded-For pair and selects its penultimate packet source.
	ClientAddressAuthorityGoogleCloud
	clientAddressAuthorityLimit
)

func clientAddressAuthorityFacts() [clientAddressAuthorityLimit]string {
	return [...]string{
		ClientAddressAuthorityPeer:         clientAddressAuthorityPeerText,
		ClientAddressAuthorityTrustedProxy: clientAddressAuthorityTrustedProxyText,
		ClientAddressAuthorityGoogleCloud:  clientAddressAuthorityGoogleCloudText,
	}
}

// Validate rejects values outside the closed authority domain.
func (a ClientAddressAuthority) Validate() error {
	if a >= clientAddressAuthorityLimit || clientAddressAuthorityFacts()[a] == "" {
		return core.ErrExchangeContract
	}
	return nil
}

// IsValid reports whether a belongs to the closed authority domain.
func (a ClientAddressAuthority) IsValid() bool { return a.Validate() == nil }

// OffWireEnum marks the authority as caller policy, not a wire value.
func (ClientAddressAuthority) OffWireEnum() {}

// String returns a diagnostic projection, not a wire value.
func (a ClientAddressAuthority) String() string {
	if err := a.Validate(); err != nil {
		return ""
	}
	return clientAddressAuthorityFacts()[a]
}

// TrustedProxyPrefixes is one fixed, canonical set of admitted direct and
// forwarded proxy networks. Its zero value is the valid trust-nothing set.
type TrustedProxyPrefixes struct {
	prefixes [TrustedProxyMaximumCount]netip.Prefix
	count    uint8
}

// ParseTrustedProxyPrefixes decodes one bounded comma-separated external
// configuration value into a canonical fixed-capacity prefix set.
func ParseTrustedProxyPrefixes(raw string) (TrustedProxyPrefixes, error) {
	if len(raw) > HeaderValueMaximumBytes {
		return TrustedProxyPrefixes{}, core.ErrExchangeContract
	}
	remaining := strings.TrimSpace(raw)
	if remaining == "" {
		return TrustedProxyPrefixes{}, nil
	}

	var prefixes TrustedProxyPrefixes
	for {
		if prefixes.count >= TrustedProxyMaximumCount {
			return TrustedProxyPrefixes{}, core.ErrExchangeContract
		}
		member, rest, found := strings.Cut(remaining, ",")
		prefix, err := parseTrustedProxyPrefix(strings.TrimSpace(member))
		if err != nil || prefixes.containsPrefix(prefix) {
			return TrustedProxyPrefixes{}, errors.Join(core.ErrExchangeContract, err)
		}
		prefixes.prefixes[prefixes.count] = prefix
		prefixes.count++
		if !found {
			break
		}
		remaining = rest
	}
	if err := prefixes.Validate(); err != nil {
		return TrustedProxyPrefixes{}, err
	}
	return prefixes, nil
}

func parseTrustedProxyPrefix(raw string) (netip.Prefix, error) {
	if raw == "" {
		return netip.Prefix{}, core.ErrExchangeContract
	}
	prefix, err := netip.ParsePrefix(raw)
	if err != nil {
		return netip.Prefix{}, err
	}
	if prefix.Addr().Zone() != "" {
		return netip.Prefix{}, core.ErrExchangeContract
	}
	return canonicalTrustedProxyPrefix(prefix)
}

func canonicalTrustedProxyPrefix(prefix netip.Prefix) (netip.Prefix, error) {
	address := prefix.Addr()
	bits := prefix.Bits()
	if address.Is4In6() {
		if bits < 96 {
			return netip.Prefix{}, core.ErrExchangeContract
		}
		unmappedBits := bits - 96
		if unmappedBits == 0 {
			return netip.Prefix{}, core.ErrExchangeContract
		}
		prefix = netip.PrefixFrom(address.Unmap(), unmappedBits)
	}
	prefix = prefix.Masked()
	if !prefix.IsValid() || prefix.Addr().Zone() != "" {
		return netip.Prefix{}, core.ErrExchangeContract
	}
	return prefix, nil
}

// Validate rejects noncanonical, duplicate, hidden, or over-capacity state.
func (p TrustedProxyPrefixes) Validate() error {
	if p.count > TrustedProxyMaximumCount {
		return core.ErrExchangeContract
	}
	for index := range int(p.count) {
		if err := p.validatePrefix(index); err != nil {
			return err
		}
	}
	for index := int(p.count); index < len(p.prefixes); index++ {
		if p.prefixes[index].IsValid() {
			return core.ErrExchangeContract
		}
	}
	return nil
}

func (p TrustedProxyPrefixes) validatePrefix(index int) error {
	prefix := p.prefixes[index]
	canonical, err := canonicalTrustedProxyPrefix(prefix)
	if err != nil || canonical != prefix {
		return core.ErrExchangeContract
	}
	for prior := range index {
		if p.prefixes[prior] == prefix {
			return core.ErrExchangeContract
		}
	}
	return nil
}

func (p TrustedProxyPrefixes) containsPrefix(candidate netip.Prefix) bool {
	for index := range int(p.count) {
		if p.prefixes[index] == candidate {
			return true
		}
	}
	return false
}

func (p TrustedProxyPrefixes) containsAddress(address netip.Addr) bool {
	for index := range int(p.count) {
		if p.prefixes[index].Contains(address) {
			return true
		}
	}
	return false
}

// Count reports the admitted prefix count.
func (p TrustedProxyPrefixes) Count() uint8 { return p.count }

// String returns the canonical comma-separated external configuration value.
func (p TrustedProxyPrefixes) String() string {
	if err := p.Validate(); err != nil {
		return ""
	}
	var projection strings.Builder
	for index := range int(p.count) {
		if index > 0 {
			projection.WriteByte(',')
		}
		projection.WriteString(p.prefixes[index].String())
	}
	return projection.String()
}

// ClientAddressRequest binds one Exchange-owned HTTP call to one explicit
// authority decision. Trusted prefixes are required only in trusted-proxy
// mode and forbidden in every other mode.
type ClientAddressRequest struct {
	Call           SocketServerCall
	TrustedProxies TrustedProxyPrefixes
	Authority      ClientAddressAuthority
}

// Validate rejects incomplete and contradictory address-resolution policy.
func (r ClientAddressRequest) Validate() error {
	if err := r.Call.Validate(); err != nil {
		return requestError(err)
	}
	if err := r.Authority.Validate(); err != nil {
		return requestError(err)
	}
	if err := r.TrustedProxies.Validate(); err != nil {
		return requestError(err)
	}
	trustedMode := r.Authority == ClientAddressAuthorityTrustedProxy
	if trustedMode != (r.TrustedProxies.Count() > 0) {
		return requestError(core.ErrExchangeContract)
	}
	return nil
}

// ClientAddress is one canonical address projected from a validated real HTTP
// request under its caller-owned authority policy.
type ClientAddress struct {
	Address netip.Addr
}

// Validate rejects absent, zoned, and noncanonical mapped addresses.
func (a ClientAddress) Validate() error {
	if !a.Address.IsValid() || a.Address.Zone() != "" || a.Address != a.Address.Unmap() {
		return core.ErrExchangeContract
	}
	return nil
}

// ResolveClientAddress projects one request to its canonical upstream address.
func ResolveClientAddress(call ClientAddressRequest) (ClientAddress, error) {
	if err := call.Validate(); err != nil {
		return ClientAddress{}, err
	}
	request := call.Call.request
	peer, err := parseClientAddress(request.RemoteAddr)
	if err != nil {
		return ClientAddress{}, requestError(err)
	}
	address := peer
	switch call.Authority {
	case ClientAddressAuthorityTrustedProxy:
		address = resolveTrustedProxyAddress(request, peer, call.TrustedProxies)
	case ClientAddressAuthorityGoogleCloud:
		address = resolveGoogleCloudAddress(request, peer)
	case ClientAddressAuthorityPeer:
	default:
		return ClientAddress{}, requestError(core.ErrExchangeContract)
	}
	result := ClientAddress{Address: address}
	if err := result.Validate(); err != nil {
		return ClientAddress{}, requestError(err)
	}
	return result, nil
}

func parseClientAddress(raw string) (netip.Addr, error) {
	addressPort, portErr := netip.ParseAddrPort(raw)
	if portErr == nil {
		return canonicalClientAddress(addressPort.Addr())
	}
	address, addressErr := netip.ParseAddr(raw)
	if addressErr != nil {
		return netip.Addr{}, errors.Join(portErr, addressErr)
	}
	return canonicalClientAddress(address)
}

func canonicalClientAddress(address netip.Addr) (netip.Addr, error) {
	if !address.IsValid() || address.Zone() != "" {
		return netip.Addr{}, core.ErrExchangeContract
	}
	return address.Unmap(), nil
}

func resolveTrustedProxyAddress(request *http.Request, peer netip.Addr, trusted TrustedProxyPrefixes) netip.Addr {
	if !trusted.containsAddress(peer) {
		return peer
	}
	cursor, ok := newForwardedMemberCursor(request)
	if !ok {
		return peer
	}
	for {
		member, present := cursor.next()
		if !present {
			return peer
		}
		address, err := parseClientAddress(member)
		if err != nil {
			return peer
		}
		if !trusted.containsAddress(address) {
			return address
		}
	}
}

func resolveGoogleCloudAddress(request *http.Request, peer netip.Addr) netip.Addr {
	cursor, ok := newForwardedMemberCursor(request)
	if !ok {
		return peer
	}
	forwardingRule, present := cursor.next()
	if !present {
		return peer
	}
	if _, err := parseClientAddress(forwardingRule); err != nil {
		return peer
	}
	packetSource, present := cursor.next()
	if !present {
		return peer
	}
	address, err := parseClientAddress(packetSource)
	if err != nil {
		return peer
	}
	return address
}

type forwardedMemberCursor struct {
	values     []string
	valueIndex int
	end        int
}

func newForwardedMemberCursor(request *http.Request) (forwardedMemberCursor, bool) {
	values := request.Header.Values(StandardHeaderForwardedFor.String())
	if len(values) == 0 || len(values) > HeaderValueMaximumCount {
		return forwardedMemberCursor{}, false
	}
	for _, value := range values {
		if len(value) > HeaderValueMaximumBytes {
			return forwardedMemberCursor{}, false
		}
	}
	last := len(values) - 1
	return forwardedMemberCursor{values: values, valueIndex: last, end: len(values[last])}, true
}

func (c *forwardedMemberCursor) next() (string, bool) {
	if c.valueIndex < 0 {
		return "", false
	}
	value := c.values[c.valueIndex]
	comma := strings.LastIndexByte(value[:c.end], ',')
	member := strings.TrimSpace(value[comma+1 : c.end])
	if comma < 0 {
		c.valueIndex--
		if c.valueIndex >= 0 {
			c.end = len(c.values[c.valueIndex])
		}
	} else {
		c.end = comma
	}
	return member, true
}

var _ core.OffWireEnum = ClientAddressAuthorityPeer
