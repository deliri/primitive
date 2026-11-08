package exchange

import (
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
)

func TestClientCloneStandardTransportLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                                              string
		wantErr                                           error
		implicit, direct, zero, opaque, typedNil, timeout bool
	}{
		{name: "configured proxy and client policy survive separate transport custody"},
		{name: "explicit direct transport remains direct without aliasing", direct: true},
		{name: "implicit Go transport receives private custody", implicit: true},
		{name: "zero client cannot acquire transport authority", zero: true, wantErr: core.ErrExchangeContract},
		{name: "opaque transport cannot invent cloning semantics", opaque: true, wantErr: core.ErrExchangeContract},
		{name: "typed nil transport is refused before native clone", typedNil: true, wantErr: core.ErrExchangeContract},
		{name: "mutated client timeout is refused before derivation", timeout: true, wantErr: core.ErrExchangeContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			base := http.DefaultTransport.(*http.Transport).Clone()
			defer base.CloseIdleConnections()
			base.MaxIdleConnsPerHost = 7
			base.DisableCompression = true
			base.Proxy = func(*http.Request) (*url.URL, error) { return nil, core.ErrExchangeTransport }
			if tc.direct {
				base.Proxy = nil
			}
			jar, err := cookiejar.New(nil)
			if err != nil {
				t.Fatal(err)
			}
			original := &http.Client{Transport: base, Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return core.ErrExchangeTransport }}
			if tc.implicit {
				original.Transport = nil
			}
			if tc.opaque {
				original.Transport = opaqueRoundTripper(func(*http.Request) (*http.Response, error) { return nil, core.ErrExchangeTransport })
			}
			if tc.typedNil {
				original.Transport = (*http.Transport)(nil)
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
			got, gotErr := client.CloneStandardTransport()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("CloneStandardTransport() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (Client{}) {
					t.Fatalf("refused clone = %+v, want zero capability", got)
				}
				return
			}
			owned, ok := got.http.Transport.(*http.Transport)
			if !ok || owned == nil {
				t.Fatalf("cloned transport = %T, want nonnil standard transport", got.http.Transport)
			}
			defer owned.CloseIdleConnections()
			selected := base
			if tc.implicit {
				selected = http.DefaultTransport.(*http.Transport)
			}
			if got.http == original || owned == selected || got.http.Jar != jar || got.http.CheckRedirect == nil || got.Validate() != nil {
				t.Fatalf("clone aliases client/transport = %t/%t, jar preserved = %t, redirect present = %t, want false/false/true/true", got.http == original, owned == selected, got.http.Jar == jar, got.http.CheckRedirect != nil)
			}
			if err := got.http.CheckRedirect(nil, nil); !errors.Is(err, core.ErrExchangeTransport) {
				t.Fatalf("cloned redirect policy = %v, want original cause %v", err, core.ErrExchangeTransport)
			}
			if (owned.Proxy == nil) != (selected.Proxy == nil) || owned.MaxIdleConnsPerHost != selected.MaxIdleConnsPerHost || owned.DisableCompression != selected.DisableCompression {
				t.Fatalf("cloned proxy/idle/compression policy = %t/%d/%t, want %t/%d/%t", owned.Proxy != nil, owned.MaxIdleConnsPerHost, owned.DisableCompression, selected.Proxy != nil, selected.MaxIdleConnsPerHost, selected.DisableCompression)
			}
			if !tc.implicit && !tc.direct {
				if _, err := owned.Proxy(nil); !errors.Is(err, core.ErrExchangeTransport) {
					t.Fatalf("cloned proxy policy cause = %v, want %v", err, core.ErrExchangeTransport)
				}
			}
			owned.MaxIdleConnsPerHost++
			if owned.MaxIdleConnsPerHost == selected.MaxIdleConnsPerHost || (tc.implicit && original.Transport != nil) || (!tc.implicit && original.Transport != base) {
				t.Fatalf("source transport = %T and idle limits = %d/%d, want unchanged source and separate derived limit", original.Transport, selected.MaxIdleConnsPerHost, owned.MaxIdleConnsPerHost)
			}
		})
	}
}
