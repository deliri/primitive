package tailnet

import (
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	tailscale "tailscale.com/client/tailscale/v2"
)

// BenchmarkAuthKeyLocalProvider measures the real API SDK and Exchange boundary
// against a local provider. It excludes tsnet startup and real provider latency.
// This entry point did not exist on the imported branch; it has no before claim.
func BenchmarkAuthKeyLocalProvider(b *testing.B) {
	configuration := fixtureConfiguration(b)
	token := fixtureIdentityToken(b)
	key := tailscale.Key{Key: fixtureAuthKey}
	key.Capabilities.Devices.Create.Ephemeral = true
	key.Capabilities.Devices.Create.Tags = []string{configuration.Tag.String()}
	keyBody, err := core.MarshalCanonicalJSONDocument(key)
	if err != nil {
		b.Fatalf("key response encoding = %v, want nil", err)
	}
	tokenBody, err := core.MarshalCanonicalJSONDocument(fixtureAccessResponse{AccessToken: fixtureAccessToken, TokenType: exchange.BearerAuthorizationScheme, ExpiresIn: 60})
	if err != nil {
		b.Fatalf("token response encoding = %v, want nil", err)
	}
	var tokenCalls, keyCalls atomic.Int64
	client := localEnrollmentClient(b, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body []byte
		switch r.URL.Path {
		case tokenExchangePath:
			tokenCalls.Add(1)
			body = tokenBody
		case createKeyPath:
			keyCalls.Add(1)
			body = keyBody
		default:
			b.Errorf("provider path = %q, want token or key operation", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(body); err != nil {
			b.Errorf("provider Write() = %v, want nil", err)
		}
	}))
	b.ReportAllocs()
	for b.Loop() {
		got, err := acquireAuthKey(b.Context(), client, configuration, token)
		if err != nil || got != fixtureAuthKey {
			b.Fatalf("acquireAuthKey() = (keyMatches=%t,%v), want exact key and nil", got == fixtureAuthKey, err)
		}
	}
	if tokenCalls.Load() != int64(b.N) || keyCalls.Load() != int64(b.N) {
		b.Fatalf("provider calls = token=%d key=%d, want %d each", tokenCalls.Load(), keyCalls.Load(), b.N)
	}
	b.ReportMetric(2, "requests/op")
}
