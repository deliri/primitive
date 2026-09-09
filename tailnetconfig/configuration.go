package tailnetconfig

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/googleidentity"
	"github.com/deliri/primitive/v2026/temporal"
	"net/netip"
	"strings"
)

const (
	MaximumStartupNanoseconds int64 = 60_000_000_000
	ClientIDMaximumBytes            = 128
	HostnameMaximumBytes            = 63
	TagNameMaximumBytes             = 63
	TagPrefix                       = "tag:"
)

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
	if !identifier(string(v), ClientIDMaximumBytes) {
		return core.ErrTailnetContract
	}
	return nil
}
func (v Hostname) Validate() error {
	text := string(v)
	if !identifier(text, HostnameMaximumBytes) || strings.Contains(text, "_") || text != strings.ToLower(text) || text[0] == '-' || text[len(text)-1] == '-' {
		return core.ErrTailnetContract
	}
	return nil
}
func (v Tag) Validate() error {
	text, ok := strings.CutPrefix(string(v), TagPrefix)
	if !ok || !identifier(text, TagNameMaximumBytes) || !tagInitialLetter(text[0]) || strings.Contains(text, "_") {
		return core.ErrTailnetContract
	}
	return nil
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
		return errors.Join(core.ErrTailnetContract, err)
	}
	if !IsAddress(c.Destination.Addr()) || c.Destination.Port() == 0 || c.StartupTimeout.IsZero() || c.StartupTimeout.Nanoseconds() > MaximumStartupNanoseconds {
		return core.ErrTailnetContract
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

// ParseClientID admits the exact provider identifier without canonical repair.
func ParseClientID(value string) (ClientID, error) {
	result := ClientID(value)
	if err := result.Validate(); err != nil {
		return "", err
	}
	return result, nil
}
func (v ClientID) String() string { return string(v) }

// ParseHostname admits one canonical DNS label without guessing a hostname.
func ParseHostname(value string) (Hostname, error) {
	result := Hostname(value)
	if err := result.Validate(); err != nil {
		return "", err
	}
	return result, nil
}
func (v Hostname) String() string { return string(v) }

// ParseTag admits one explicit provider tag, never a list of authorities.
func ParseTag(value string) (Tag, error) {
	result := Tag(value)
	if err := result.Validate(); err != nil {
		return "", err
	}
	return result, nil
}
func (v Tag) String() string { return string(v) }

// The provider tag alphabet differs from DNS labels: case and trailing hyphens
// are significant, and the initial byte must be a letter. The local byte ceiling
// is a separate Primitive bound; it must not track hostname changes implicitly.
func tagInitialLetter(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}
