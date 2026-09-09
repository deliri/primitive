package tailnetconfig

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/googleidentity"
	"github.com/deliri/primitive/v2026/temporal"
	"net/netip"
	"strings"
)

var ErrContract = errors.New("tailnet contract")

const MaximumStartupNanoseconds int64 = 60_000_000_000

type ClientID string
type Hostname string
type Tag string

func identifier(value string, maximum int) bool {
	if len(value) == 0 || len(value) > maximum {
		return false
	}
	for _, char := range value {
		if !identifierRune(char) {
			return false
		}
	}
	return true
}

func (v ClientID) Validate() error {
	if !identifier(string(v), 128) {
		return ErrContract
	}
	return nil
}
func (v Hostname) Validate() error {
	text := string(v)
	if !identifier(text, 63) || strings.Contains(text, "_") || text != strings.ToLower(text) || text[0] == '-' || text[len(text)-1] == '-' {
		return ErrContract
	}
	return nil
}
func (v Tag) Validate() error {
	text, ok := strings.CutPrefix(string(v), "tag:")
	if !ok {
		return ErrContract
	}
	return Hostname(text).Validate()
}

// Configuration is an authored capability intent, not an observation or receipt.
type Configuration struct {
	ClientID       ClientID
	Hostname       Hostname
	Tag            Tag
	Audience       googleidentity.Audience
	StateDirectory core.AbsolutePath
	Destination    netip.AddrPort
	StartupTimeout temporal.Duration
}

func (c Configuration) Validate() error {
	if err := errors.Join(c.ClientID.Validate(), c.Hostname.Validate(), c.Tag.Validate(), c.Audience.Validate(), c.StateDirectory.Validate(), c.StartupTimeout.Validate()); err != nil {
		return errors.Join(ErrContract, err)
	}
	if !IsAddress(c.Destination.Addr()) || c.Destination.Port() == 0 || c.StartupTimeout.IsZero() || c.StartupTimeout.Nanoseconds() > MaximumStartupNanoseconds {
		return ErrContract
	}
	return nil
}

func identifierRune(char rune) bool {
	return char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-' || char == '_'
}

// IsAddress recognizes Tailscale's assigned ranges, never generic public CGNAT
// reachability or caller authorization. Authorization remains separately required.
func IsAddress(address netip.Addr) bool {
	if address.Zone() != "" {
		return false
	}
	address = address.Unmap()
	return netip.MustParsePrefix("100.64.0.0/10").Contains(address) || netip.MustParsePrefix("fd7a:115c:a1e0::/48").Contains(address)
}
