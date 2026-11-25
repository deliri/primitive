package objectstore

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func redirectCall(t *testing.T) (exchange.SocketServerCall, *httptest.ResponseRecorder) {
	t.Helper()

	recorder := httptest.NewRecorder()
	call, err := exchange.NewSocketServerCall(recorder, httptest.NewRequest(http.MethodGet, "https://example.com/download", nil))
	if err != nil {
		t.Fatalf("exchange.NewSocketServerCall() setup error = %v, want nil", err)
	}
	return call, recorder
}

func TestDownloadCapabilityProjectionWriteRedirectLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive host-only bearer becomes one no-store 303 Location", func(t *testing.T) {
		t.Parallel()

		for _, extent := range []int{0, CapabilityURLMaximumBytes - 1, CapabilityURLMaximumBytes} {
			projection := downloadCapabilityProjectionRequestFixture(t, downloadCapabilityFixtureRequest{
				Provider: ProviderGoogleCloudStorage, Extent: extent,
			})
			want := projection.target.URL.value.String()
			call, recorder := redirectCall(t)
			if err := projection.WriteRedirect(call); err != nil {
				t.Fatalf("WriteRedirect(%d-byte bearer) error = %v, want nil", len(want), err)
			}
			if recorder.Code != http.StatusSeeOther || recorder.Body.Len() != 0 {
				t.Fatalf("WriteRedirect() response = (%d, %d body bytes), want (303, 0)", recorder.Code, recorder.Body.Len())
			}
			header := recorder.Header()
			if got := header.Values("Location"); len(got) != 1 || got[0] != want {
				t.Fatalf("WriteRedirect() Location count = %d, want exactly the %d-byte issued bearer", len(got), len(want))
			}
			if got := header.Values("Cache-Control"); len(got) != 1 || got[0] != "no-store" {
				t.Fatalf("WriteRedirect() Cache-Control = %q, want [no-store]", got)
			}
			if got := header.Get("Content-Length"); got != "0" {
				t.Fatalf("WriteRedirect() Content-Length = %q, want 0", got)
			}
		}
	})

	t.Run("negative unspendable bearers refuse before any response byte", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name       string
			projection DownloadCapabilityProjection
		}{
			{name: "unset projection", projection: DownloadCapabilityProjection{}},
			{name: "amazon bearer signs checksum mode a browser cannot send", projection: downloadCapabilityProjectionRequestFixture(t, downloadCapabilityFixtureRequest{Provider: ProviderAmazonS3})},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				call, recorder := redirectCall(t)
				if err := tc.projection.WriteRedirect(call); !errors.Is(err, core.ErrObjectStoreContract) {
					t.Fatalf("WriteRedirect() error = %v, want errors.Is %v", err, core.ErrObjectStoreContract)
				}
				if recorder.Code != http.StatusOK || recorder.Body.Len() != 0 || len(recorder.Header()) != 0 || recorder.Flushed {
					t.Fatalf("WriteRedirect() refusal wrote status %d, %d body bytes, headers %v; want nothing", recorder.Code, recorder.Body.Len(), recorder.Header())
				}
			})
		}
	})

	t.Run("hostile invalid server call refuses without disclosure", func(t *testing.T) {
		t.Parallel()

		projection := downloadCapabilityProjectionRequestFixture(t, downloadCapabilityFixtureRequest{Provider: ProviderGoogleCloudStorage})
		err := projection.WriteRedirect(exchange.SocketServerCall{})
		if err == nil || strings.Contains(err.Error(), "X-Goog-Signature") {
			t.Fatalf("WriteRedirect(zero call) error = %v, want a refusal that does not carry the bearer", err)
		}
	})
}
