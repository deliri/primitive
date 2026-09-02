package exchange

import (
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type opaqueRoundTripper func(*http.Request) (*http.Response, error)

func (f opaqueRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestClientWithoutProxyLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive standard transport is cloned with proxy routing removed", func(t *testing.T) {
		t.Parallel()

		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = func(*http.Request) (*url.URL, error) {
			return nil, core.ErrExchangeTransport
		}
		original := &http.Client{Transport: transport}
		client, clientErr := NewClient(original)
		if clientErr != nil {
			t.Fatalf("NewClient() error = %v, want nil", clientErr)
		}
		direct, gotErr := client.WithoutProxy()
		if gotErr != nil {
			t.Fatalf("Client.WithoutProxy() error = %v, want nil", gotErr)
		}
		gotTransport, ok := direct.http.Transport.(*http.Transport)
		if !ok || gotTransport == transport || gotTransport.Proxy != nil {
			t.Fatalf("Client.WithoutProxy() transport = (%T, cloned %t, proxy nil %t), want (*http.Transport, true, true)", direct.http.Transport, gotTransport != transport, gotTransport != nil && gotTransport.Proxy == nil)
		}
		if original.Transport != transport || transport.Proxy == nil {
			t.Fatalf("caller transport after derivation = (same %t, proxy present %t), want true, true", original.Transport == transport, transport.Proxy != nil)
		}
	})

	t.Run("negative opaque transport is refused because proxy bypass cannot be proved", func(t *testing.T) {
		t.Parallel()

		client, clientErr := NewClient(&http.Client{Transport: opaqueRoundTripper(func(*http.Request) (*http.Response, error) {
			return nil, core.ErrExchangeTransport
		})})
		if clientErr != nil {
			t.Fatalf("NewClient(opaque transport) error = %v, want nil", clientErr)
		}
		got, gotErr := client.WithoutProxy()
		if got != (Client{}) || !errors.Is(gotErr, core.ErrExchangeContract) {
			t.Fatalf("Client.WithoutProxy(opaque) = (%v, %v), want zero and %v", got, gotErr, core.ErrExchangeContract)
		}
	})

	t.Run("neutral standard client derives direct custody without changing its validity", func(t *testing.T) {
		t.Parallel()

		client, clientErr := NewStandardClient()
		if clientErr != nil {
			t.Fatalf("NewStandardClient() error = %v, want nil", clientErr)
		}
		direct, gotErr := client.WithoutProxy()
		if gotErr != nil || direct.Validate() != nil || client.Validate() != nil {
			t.Fatalf("standard Client.WithoutProxy() = (direct %v, error %v, source validation %v), want both valid and nil", direct.Validate(), gotErr, client.Validate())
		}
	})
}
