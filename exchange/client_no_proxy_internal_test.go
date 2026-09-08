package exchange

import (
	"errors"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
)

type opaqueRoundTripper func(*http.Request) (*http.Response, error)

func (f opaqueRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestClientWithoutProxyLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                                                 string
		zero, opaque, nilTransport, typedNil, proxy, timeout bool
		wantErr                                              error
	}{
		{name: "proxy removal clones caller transport", proxy: true},
		{name: "already direct transport still receives separate custody"},
		{name: "implicit Go default is cloned without changing global transport", nilTransport: true},
		{name: "zero client cannot derive an executable capability", zero: true, wantErr: core.ErrExchangeContract},
		{name: "opaque round tripper cannot claim proven direct dialing", opaque: true, wantErr: core.ErrExchangeContract},
		{name: "typed nil transport cannot panic inside Go Clone", typedNil: true, wantErr: core.ErrExchangeContract},
		{name: "mutated client timeout must be revalidated at derivation", timeout: true, wantErr: core.ErrExchangeContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			base := http.DefaultTransport.(*http.Transport).Clone()
			defer base.CloseIdleConnections()
			base.MaxIdleConnsPerHost = 7
			base.DisableCompression = true
			if tc.proxy {
				base.Proxy = func(*http.Request) (*url.URL, error) { return nil, core.ErrExchangeTransport }
			} else {
				base.Proxy = nil
			}
			original := &http.Client{Transport: base}
			if tc.opaque {
				original.Transport = opaqueRoundTripper(func(*http.Request) (*http.Response, error) { return nil, core.ErrExchangeTransport })
			}
			if tc.typedNil {
				original.Transport = (*http.Transport)(nil)
			}
			if tc.nilTransport {
				original.Transport = nil
			}
			client, err := NewClient(original)
			if err != nil {
				t.Fatal(err)
			}
			if tc.zero {
				client = Client{}
			}
			if tc.timeout {
				original.Timeout = time.Nanosecond
			}
			got, gotErr := client.WithoutProxy()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("direct derivation error=%v,want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (Client{}) {
					t.Fatalf("refused derivation leaked capability %+v", got)
				}
				return
			}
			transport, ok := got.http.Transport.(*http.Transport)
			if !ok || transport == nil {
				t.Fatalf("derived transport=%T,want Go transport", got.http.Transport)
			}
			defer transport.CloseIdleConnections()
			selected := base
			if tc.nilTransport {
				selected = http.DefaultTransport.(*http.Transport)
			}
			if got.http == original || transport == selected || transport.Proxy != nil || got.Validate() != nil || client.Validate() != nil {
				t.Fatalf("direct custody=(same client %t,same transport %t,proxy %t),want separate valid direct capability", got.http == original, transport == selected, transport.Proxy != nil)
			}
			if transport.MaxIdleConnsPerHost != selected.MaxIdleConnsPerHost || transport.DisableCompression != selected.DisableCompression || got.http.Jar != original.Jar {
				t.Fatalf("idle/compression/jar preserved=%t/%t/%t, want true/true/true", transport.MaxIdleConnsPerHost == selected.MaxIdleConnsPerHost, transport.DisableCompression == selected.DisableCompression, got.http.Jar == original.Jar)
			}
			transport.MaxIdleConnsPerHost++
			if selected.MaxIdleConnsPerHost == transport.MaxIdleConnsPerHost {
				t.Fatalf("source/derived idle limits=%d/%d, want distinct", selected.MaxIdleConnsPerHost, transport.MaxIdleConnsPerHost)
			}
			if tc.nilTransport {
				if original.Transport != nil || selected.Proxy == nil {
					t.Fatalf("original transport/default proxy=%T/%t, want nil/true", original.Transport, selected.Proxy != nil)
				}
			} else if original.Transport != base || (base.Proxy != nil) != tc.proxy {
				t.Fatalf("original transport/base match and proxy present=%t/%t, want true/%t", original.Transport == base, base.Proxy != nil, tc.proxy)
			}
		})
	}
}
