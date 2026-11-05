package timematch

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

// This is an explicit local HTTP provider double, not a provider approval.
// Real sockets, TLS and Exchange execute every request; the dialer routes
// only the documented official provider hosts to the local server, and any
// other destination fails, so a credential sent elsewhere is observable.
func fixtureHost(address string) bool {
	switch address {
	case "accounts.google.com:443", "oauth2.googleapis.com:443", "www.googleapis.com:443", "people.googleapis.com:443",
		"login.microsoftonline.com:443", "graph.microsoft.com:443",
		"caldav.icloud.com:443", "p42-caldav.icloud.com:443":
		return true
	default:
		return false
	}
}

const privateMarker = "PRIVATE-EVENT-TITLE-attendee@example.com"

type recorded struct {
	host, method, path, query string
	header                    http.Header
	body                      []byte
}

type provider struct {
	mutex    sync.Mutex
	requests []recorded
	refused  []string
	handle   func(w http.ResponseWriter, r *http.Request, body []byte)
}

func (p *provider) record() []recorded {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return append([]recorded(nil), p.requests...)
}

func (p *provider) refusedDials() []string {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return append([]string(nil), p.refused...)
}

func newNative(t *testing.T, handle func(w http.ResponseWriter, r *http.Request, body []byte)) (Client, *provider) {
	t.Helper()
	p := &provider{handle: handle}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		p.mutex.Lock()
		p.requests = append(p.requests, recorded{host: r.Host, method: r.Method, path: r.URL.EscapedPath(), query: r.URL.RawQuery, header: r.Header.Clone(), body: body})
		p.mutex.Unlock()
		p.handle(w, r, body)
	}))
	t.Cleanup(server.Close)
	httpClient := server.Client()
	transport := httpClient.Transport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.TLSClientConfig.ServerName = "example.com"
	local := strings.TrimPrefix(server.URL, "https://")
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		if !fixtureHost(address) {
			p.mutex.Lock()
			p.refused = append(p.refused, address)
			p.mutex.Unlock()
			return nil, core.ErrTimeMatchContract
		}
		return (&net.Dialer{}).DialContext(ctx, network, local)
	}
	httpClient.Transport = transport
	t.Cleanup(transport.CloseIdleConnections)
	network, err := exchange.NewClient(httpClient)
	if err != nil {
		t.Fatalf("exchange.NewClient() error = %v, want nil", err)
	}
	client, err := NewClient(network)
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}
	return client, p
}

func fixturePolicy(t testing.TB) exchange.OperationPolicy {
	t.Helper()
	timeout, err := temporal.DurationFromSeconds(20)
	if err != nil {
		t.Fatalf("DurationFromSeconds() error = %v, want nil", err)
	}
	return exchange.OperationPolicy{OperationTimeout: timeout, AttemptTimeout: timeout, Retry: exchange.RetryPolicy{MaximumAttempts: 1}, Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject}}
}

func at(t testing.TB, text string) temporal.Instant {
	t.Helper()
	instant, err := temporal.ParseRFC3339(text)
	if err != nil {
		t.Fatalf("ParseRFC3339(%q) error = %v, want nil", text, err)
	}
	return instant
}

func window(t testing.TB, start, end string) temporal.IntervalBounds {
	t.Helper()
	return temporal.IntervalBounds{Start: at(t, start), End: at(t, end)}
}

func secret(t testing.TB, value string) Secret {
	t.Helper()
	parsed, err := ParseSecret([]byte(value))
	if err != nil {
		t.Fatalf("ParseSecret() error = %v, want nil", err)
	}
	return parsed
}

func calendarID(t testing.TB, value string) CalendarID {
	t.Helper()
	parsed, err := ParseCalendarID(value)
	if err != nil {
		t.Fatalf("ParseCalendarID(%q) error = %v, want nil", value, err)
	}
	return parsed
}

func oauthConnection(t testing.TB, provider Provider) Connection {
	t.Helper()
	return Connection{Provider: provider, AccessToken: secret(t, "access-token-fixture")}
}

func appleConnection(t testing.TB) Connection {
	t.Helper()
	account, err := ParseAppleAccount("visitor@icloud.com")
	if err != nil {
		t.Fatalf("ParseAppleAccount() error = %v, want nil", err)
	}
	return Connection{Provider: ProviderApple, AppleAccount: account, ApplePassword: secret(t, "abcd-efgh-ijkl-mnop")}
}

func application(t testing.TB, provider Provider, tenant MicrosoftTenant) Application {
	t.Helper()
	id, idErr := core.ParseOAuthClientID("client-fixture.apps.example")
	redirect, redirectErr := core.ParseOAuthRedirectURI("https://book.example.com/timematch/return")
	if idErr != nil || redirectErr != nil {
		t.Fatalf("OAuth fixture errors = (%v, %v), want nil", idErr, redirectErr)
	}
	return Application{Provider: provider, ClientID: id, ClientSecret: secret(t, "client-secret-fixture"), RedirectURI: redirect, Tenant: tenant}
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

// noLeak proves an error never carries a credential or provider-private body.
func noLeak(t *testing.T, err error, secrets ...string) {
	t.Helper()
	if err == nil {
		return
	}
	if !errors.Is(err, core.ErrTimeMatchContract) {
		t.Fatalf("error = %v, want a TimeMatch identity before inspecting its text", err)
	}
	text := err.Error()
	for _, value := range append(secrets, privateMarker) {
		if strings.Contains(text, value) {
			t.Fatalf("error %q contains secret or private provider text %q, want neither", text, value)
		}
	}
}

const fixtureBackstop = 30 * time.Second
