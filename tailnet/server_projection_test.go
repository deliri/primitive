package tailnet

import (
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/tailnetconfig"
	"tailscale.com/ipn"
)

func TestServerProjectionOwnsExactEnrollment(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		key      string
		hostname tailnetconfig.Hostname
		tag      tailnetconfig.Tag
	}{
		{"minimum explicit names and key", authKeyPrefix + "a", "a", "tag:a"},
		{"maximum host and credential extent", authKeyPrefix + strings.Repeat("a", authKeyMaximumBytes-len(authKeyPrefix)), tailnetconfig.Hostname(strings.Repeat("a", tailnetconfig.HostnameMaximumBytes)), tailnetconfig.Tag(tailnetconfig.TagPrefix + strings.Repeat("a", tailnetconfig.HostnameMaximumBytes))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			configuration := fixtureConfiguration(t)
			configuration.Hostname = tc.hostname
			configuration.Tag = tc.tag
			if err := configuration.Validate(); err != nil {
				t.Fatalf("configuration.Validate() = %v, want nil", err)
			}
			server := newServer(configuration, tc.key)
			if server.AuthKey != tc.key || server.ControlURL != ipn.DefaultControlURL || server.Dir != configuration.StateDirectory.String() || server.Hostname != configuration.Hostname.String() || !server.Ephemeral {
				t.Fatalf("server projection = keyMatches=%t, control=%q, dir=%q, hostname=%q, ephemeral=%t, want exact authored coordinates", server.AuthKey == tc.key, server.ControlURL, server.Dir, server.Hostname, server.Ephemeral)
			}
			if len(server.AdvertiseTags) != 1 || server.AdvertiseTags[0] != configuration.Tag.String() || server.ClientID != "" || server.IDToken != "" || server.Audience != "" || server.RunWebClient {
				t.Fatalf("server authority = tags=%v, secondaryFederation=%t, webClient=%t, want one exact tag and no secondary enrollment", server.AdvertiseTags, server.ClientID != "" || server.IDToken != "" || server.Audience != "", server.RunWebClient)
			}
		})
	}
}
