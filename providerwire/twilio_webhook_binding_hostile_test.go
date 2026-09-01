package providerwire

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestTwilioJSONSignedTargetHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	const digest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	tests := []struct {
		name       string
		configured string
		incoming   string
		mutate     func(*http.Request)
		wantURL    string
		wantErr    error
	}{
		{name: "provider digest is admitted on an otherwise queryless endpoint", configured: "https://example.test/callback", incoming: "https://internal.test/callback?bodySHA256=" + digest, wantURL: "https://example.test/callback?bodySHA256=" + digest},
		{name: "configured tenant query remains bound beside provider digest", configured: "https://example.test/callback?tenant=one", incoming: "https://internal.test/callback?tenant=one&bodySHA256=" + digest, wantURL: "https://example.test/callback?tenant=one&bodySHA256=" + digest},
		{name: "configured query key order is semantic while signed URL remains exact", configured: "https://example.test/callback?a=one&b=two", incoming: "https://internal.test/callback?b=two&bodySHA256=" + digest + "&a=one", wantURL: "https://example.test/callback?b=two&bodySHA256=" + digest + "&a=one"},
		{name: "configured duplicate values remain exact typed query facts", configured: "https://example.test/callback?tag=one&tag=two", incoming: "https://internal.test/callback?tag=one&tag=two&bodySHA256=" + digest, wantURL: "https://example.test/callback?tag=one&tag=two&bodySHA256=" + digest},
		{name: "missing provider digest is refused", configured: "https://example.test/callback", incoming: "https://internal.test/callback", wantErr: core.ErrProviderWireBinding},
		{name: "empty provider digest is refused", configured: "https://example.test/callback", incoming: "https://internal.test/callback?bodySHA256=", wantErr: core.ErrProviderWireBinding},
		{name: "duplicate provider digest is refused", configured: "https://example.test/callback", incoming: "https://internal.test/callback?bodySHA256=" + digest + "&bodySHA256=" + digest, wantErr: core.ErrProviderWireBinding},
		{name: "one byte below digest extent is refused", configured: "https://example.test/callback", incoming: "https://internal.test/callback?bodySHA256=" + digest[:len(digest)-1], wantErr: core.ErrProviderWireBinding},
		{name: "one byte above digest extent is refused", configured: "https://example.test/callback", incoming: "https://internal.test/callback?bodySHA256=" + digest + "0", wantErr: core.ErrProviderWireBinding},
		{name: "uppercase noncanonical digest is refused", configured: "https://example.test/callback", incoming: "https://internal.test/callback?bodySHA256=" + strings.ToUpper(digest), wantErr: core.ErrProviderWireBinding},
		{name: "nonhex digest member is refused", configured: "https://example.test/callback", incoming: "https://internal.test/callback?bodySHA256=g" + digest[1:], wantErr: core.ErrProviderWireBinding},
		{name: "foreign query fact is refused", configured: "https://example.test/callback?tenant=one", incoming: "https://internal.test/callback?tenant=one&foreign=true&bodySHA256=" + digest, wantErr: core.ErrProviderWireBinding},
		{name: "missing configured query fact is refused", configured: "https://example.test/callback?tenant=one", incoming: "https://internal.test/callback?bodySHA256=" + digest, wantErr: core.ErrProviderWireBinding},
		{name: "mutated callback path is refused", configured: "https://example.test/callback", incoming: "https://internal.test/foreign?bodySHA256=" + digest, wantErr: core.ErrProviderWireBinding},
		{name: "encoded callback raw path is refused", configured: "https://example.test/callback", incoming: "https://internal.test/callback?bodySHA256=" + digest, mutate: func(request *http.Request) { request.URL.RawPath = "/%63allback" }, wantErr: core.ErrProviderWireBinding},
		{name: "malformed query separator is refused", configured: "https://example.test/callback", incoming: "https://internal.test/callback?bodySHA256=" + digest + ";foreign=true", wantErr: core.ErrProviderWireBinding},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			endpoint, err := core.ParseHTTPEndpoint(testCase.configured)
			if err != nil {
				t.Fatalf("core.ParseHTTPEndpoint(configured) error = %v, want nil", err)
			}
			request := httptest.NewRequest(http.MethodPost, testCase.incoming, nil)
			if testCase.mutate != nil {
				testCase.mutate(request)
			}
			gotURL, gotErr := twilioSignedRequestURL(request, endpoint, TwilioWebhookRepresentationJSON)
			if gotURL != testCase.wantURL || !errors.Is(gotErr, testCase.wantErr) {
				t.Fatalf("twilioSignedRequestURL() = (%q, %v), want (%q, %v)", gotURL, gotErr, testCase.wantURL, testCase.wantErr)
			}
		})
	}
}

func TestTwilioBodySHA256ExhaustiveCanonicalDomain(t *testing.T) {
	t.Parallel()

	for length := 0; length <= core.SHA256DigestBytes*4; length++ {
		value := strings.Repeat("0", length)
		got := validTwilioBodySHA256(value)
		want := length == core.SHA256DigestBytes*2
		if got != want {
			t.Fatalf("validTwilioBodySHA256(%d zeroes) = %t, want %t", length, got, want)
		}
	}

	canonical := strings.Repeat("0", core.SHA256DigestBytes*2)
	for candidate := 0; candidate <= 255; candidate++ {
		mutated := []byte(canonical)
		mutated[0] = byte(candidate)
		got := validTwilioBodySHA256(string(mutated))
		want := candidate >= '0' && candidate <= '9' || candidate >= 'a' && candidate <= 'f'
		if got != want {
			t.Fatalf("validTwilioBodySHA256(first byte %#x) = %t, want %t", candidate, got, want)
		}
	}
}
