package tailnetconfig_test

import (
	"context"
	"errors"
	"net/netip"
	"regexp"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/googleidentity"
	"github.com/deliri/primitive/v2026/tailnet"
	"github.com/deliri/primitive/v2026/tailnetconfig"
	"github.com/deliri/primitive/v2026/temporal"
	"tailscale.com/tailcfg"
)

type noEnrollmentIdentity struct{ calls int }

func (s *noEnrollmentIdentity) Identity(context.Context, googleidentity.Audience) (googleidentity.Token, error) {
	s.calls++
	return googleidentity.Token{}, core.ErrTailnetEnrollment
}

// Inventory: the three Parse doors, their Validate/String contracts, IsAddress,
// Configuration.Validate, and Tailnet.NewClient consume the same generated facts.
// The independent oracle uses regex grammars and address byte ranges rather than
// production's shared identifier helper and prefix parsing.
func FuzzConfigurationIngress(f *testing.F) {
	seed := fixtureConfiguration(f)
	if err := seed.Validate(); err != nil {
		f.Fatalf("seed.Validate() = %v, want nil", err)
	}
	minimum := seed
	minimum.ClientID = "a"
	minimum.Hostname = "a"
	minimum.Tag = "tag:A"
	minimum.Destination = netip.MustParseAddrPort("100.64.0.0:1")
	var err error
	minimum.StartupTimeout, err = temporal.DurationFromNanoseconds(1)
	if err != nil {
		f.Fatalf("minimum duration = %v, want nil", err)
	}
	maximum := seed
	maximum.ClientID = tailnetconfig.ClientID(strings.Repeat("a", tailnetconfig.ClientIDMaximumBytes))
	maximum.Hostname = tailnetconfig.Hostname(strings.Repeat("a", tailnetconfig.HostnameMaximumBytes))
	maximum.Tag = tailnetconfig.Tag(tailnetconfig.TagPrefix + strings.Repeat("A", tailnetconfig.TagNameMaximumBytes))
	maximum.Destination = netip.MustParseAddrPort("[fd7a:115c:a1e0:ffff:ffff:ffff:ffff:ffff]:65535")
	maximum.StartupTimeout, err = temporal.DurationFromNanoseconds(tailnetconfig.MaximumStartupNanoseconds)
	if err != nil {
		f.Fatalf("maximum duration = %v, want nil", err)
	}
	for _, canonical := range []tailnetconfig.Configuration{minimum, seed, maximum} {
		if err := canonical.Validate(); err != nil {
			f.Fatalf("canonical seed = %v, want nil", err)
		}
		f.Add(canonical.ClientID.String(), canonical.Hostname.String(), canonical.Tag.String(), canonical.Destination.String(), canonical.StartupTimeout.Nanoseconds())
	}
	f.Add(seed.ClientID.String(), seed.Hostname.String(), "tag:0", seed.Destination.String(), seed.StartupTimeout.Nanoseconds())
	f.Add("", "", "", "", int64(0))
	f.Add(seed.ClientID.String(), seed.Hostname.String(), seed.Tag.String(), "100.128.0.0:1", tailnetconfig.MaximumStartupNanoseconds+1)
	clientGrammar := regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	tagGrammar := regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]*$`)
	hostnameGrammar := regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`)
	f.Fuzz(func(t *testing.T, clientText, hostnameText, tagText, addressText string, nanoseconds int64) {
		client, clientErr := tailnetconfig.ParseClientID(clientText)
		hostname, hostnameErr := tailnetconfig.ParseHostname(hostnameText)
		tag, tagErr := tailnetconfig.ParseTag(tagText)
		wantClient := len(clientText) <= tailnetconfig.ClientIDMaximumBytes && clientGrammar.MatchString(clientText)
		wantHostname := len(hostnameText) <= tailnetconfig.HostnameMaximumBytes && hostnameGrammar.MatchString(hostnameText)
		wantTag := len(tagText) > len(tailnetconfig.TagPrefix) && len(tagText) <= len(tailnetconfig.TagPrefix)+tailnetconfig.TagNameMaximumBytes && tagText[:len(tailnetconfig.TagPrefix)] == tailnetconfig.TagPrefix && tagGrammar.MatchString(tagText[len(tailnetconfig.TagPrefix):])
		providerAllowsTag := tailcfg.CheckTag(tagText) == nil && len(tagText) <= len(tailnetconfig.TagPrefix)+tailnetconfig.TagNameMaximumBytes
		if providerAllowsTag != wantTag {
			t.Fatalf("tag oracle disagreement = provider:%t regex:%t, want agreement", providerAllowsTag, wantTag)
		}
		for _, row := range []struct {
			name              string
			gotErr            error
			want              bool
			gotText, wantText string
		}{
			{"client identifier", clientErr, wantClient, client.String(), clientText},
			{"hostname", hostnameErr, wantHostname, hostname.String(), hostnameText},
			{"tag", tagErr, wantTag, tag.String(), tagText},
		} {
			if (row.gotErr == nil) != row.want {
				t.Fatalf("%s admission = %v, want valid=%t", row.name, row.gotErr, row.want)
			}
			if row.want && row.gotText != row.wantText {
				t.Fatalf("%s spelling = %q, want %q", row.name, row.gotText, row.wantText)
			}
			if !row.want && (!errors.Is(row.gotErr, core.ErrTailnetContract) || row.gotText != "") {
				t.Fatalf("%s refusal = (%q,%v), want zero and Tailnet contract", row.name, row.gotText, row.gotErr)
			}
		}
		address, addressErr := netip.ParseAddrPort(addressText)
		wantAddress := false
		if addressErr == nil && address.Addr().Zone() == "" {
			unmapped := address.Addr().Unmap()
			if unmapped.Is4() {
				value := unmapped.As4()
				wantAddress = value[0] == 100 && value[1] >= 64 && value[1] <= 127
			}
			if unmapped.Is6() {
				value := unmapped.As16()
				wantAddress = value[0] == 0xfd && value[1] == 0x7a && value[2] == 0x11 && value[3] == 0x5c && value[4] == 0xa1 && value[5] == 0xe0
			}
		}
		if got := tailnetconfig.IsAddress(address.Addr()); got != wantAddress {
			t.Fatalf("IsAddress(%v) = %t, want %t", address.Addr(), got, wantAddress)
		}
		duration, durationErr := temporal.DurationFromNanoseconds(nanoseconds)
		configuration := seed
		configuration.ClientID = tailnetconfig.ClientID(clientText)
		configuration.Hostname = tailnetconfig.Hostname(hostnameText)
		configuration.Tag = tailnetconfig.Tag(tagText)
		configuration.Destination = address
		configuration.StartupTimeout = duration
		wantValid := wantClient && wantHostname && wantTag && wantAddress && address.Port() != 0 && durationErr == nil && nanoseconds > 0 && nanoseconds <= tailnetconfig.MaximumStartupNanoseconds
		gotErr := configuration.Validate()
		if (gotErr == nil) != wantValid {
			t.Fatalf("Configuration.Validate() = %v, want valid=%t", gotErr, wantValid)
		}
		source := &noEnrollmentIdentity{}
		capability, err := tailnet.NewClient(configuration, source)
		if capability != nil {
			t.Cleanup(func() {
				if err := capability.Close(); err != nil {
					t.Errorf("Close() = %v, want nil", err)
				}
			})
		}
		if (err == nil) != wantValid || (capability != nil) != wantValid || source.calls != 0 {
			t.Fatalf("NewClient() = (%p,%v,%d identity calls), want valid=%t and no effects", capability, err, source.calls, wantValid)
		}
		if !wantValid && !errors.Is(err, core.ErrTailnetContract) {
			t.Fatalf("refusal = %v, want Tailnet contract", err)
		}
	})
}
