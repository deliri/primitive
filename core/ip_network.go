package core

import (
	"errors"
	"net/netip"
)

// IPNetwork is a proven canonical exact-IP or masked CIDR identity. Exact
// addresses are represented internally as full-width prefixes so containment
// has one implementation for both shapes.
type IPNetwork struct {
	prefix netip.Prefix
}

// ParseIPNetwork accepts only canonical, unmapped IP addresses and masked CIDR
// prefixes. Alternate spellings are refused instead of being normalized across
// a trust boundary.
func ParseIPNetwork(value string) (IPNetwork, error) {
	if address, err := netip.ParseAddr(value); err == nil {
		return ipNetworkFromAddress(value, address)
	}
	return ipNetworkFromPrefix(value)
}

func ipNetworkFromAddress(value string, address netip.Addr) (IPNetwork, error) {
	if address.Is4In6() || address.Zone() != "" || address.String() != value {
		return IPNetwork{}, ipNetworkContractError("IP address is not canonical")
	}
	return IPNetwork{prefix: netip.PrefixFrom(address, address.BitLen())}, nil
}

func ipNetworkFromPrefix(value string) (IPNetwork, error) {
	prefix, err := netip.ParsePrefix(value)
	if err != nil || prefix.Addr().Is4In6() || prefix.Addr().Zone() != "" ||
		prefix.Masked().String() != value {
		return IPNetwork{}, ipNetworkContractError("IP prefix is not canonical and masked")
	}
	return IPNetwork{prefix: prefix}, nil
}

// Validate proves that n came through the canonical parser.
func (n IPNetwork) Validate() error {
	if !n.prefix.IsValid() || n.prefix.Addr().Is4In6() ||
		n.prefix.Addr().Zone() != "" || n.prefix != n.prefix.Masked() {
		return ipNetworkContractError("IP network is invalid")
	}
	return nil
}

// String returns the canonical address for an exact identity and canonical
// CIDR text for a range.
func (n IPNetwork) String() string {
	if n.Validate() != nil {
		return ""
	}
	if n.prefix.Bits() == n.prefix.Addr().BitLen() {
		return n.prefix.Addr().String()
	}
	return n.prefix.String()
}

// Prefix projects the validated network to the standard library value.
func (n IPNetwork) Prefix() (netip.Prefix, error) {
	if err := n.Validate(); err != nil {
		return netip.Prefix{}, err
	}
	return n.prefix, nil
}

// Contains reports whether canonicalAddress belongs to n. The candidate must
// itself be a canonical, unmapped address.
func (n IPNetwork) Contains(canonicalAddress string) (bool, error) {
	if err := n.Validate(); err != nil {
		return false, err
	}
	address, err := netip.ParseAddr(canonicalAddress)
	if err != nil || address.Is4In6() || address.Zone() != "" ||
		address.String() != canonicalAddress {
		return false, ipNetworkContractError("candidate IP address is not canonical")
	}
	return n.prefix.Contains(address), nil
}

func ipNetworkContractError(message string) error {
	return errors.Join(ErrPrimitiveContract, errors.New(message))
}
