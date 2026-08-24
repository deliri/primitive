package exchange_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestSessionClientRetainsRealHTTPCookies(t *testing.T) {
	t.Parallel()

	type cookieObservation struct {
		name       string
		setCookie  bool
		wantCookie bool
	}
	cases := []cookieObservation{
		{name: "positive response cookie is returned on the next request", setCookie: true, wantCookie: true},
		{name: "neutral response without a cookie leaves the next request clean", wantCookie: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var gotCookie bool
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/establish" && tc.setCookie {
					http.SetCookie(writer, &http.Cookie{Name: "session", Value: "established", Path: "/"})
				}
				if request.URL.Path == "/observe" {
					_, gotErr := request.Cookie("session")
					gotCookie = gotErr == nil
				}
				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(http.StatusOK)
				_, _ = writer.Write([]byte(`{"ok":true}`))
			}))
			defer server.Close()

			client, err := exchange.NewSessionClient()
			if err != nil {
				t.Fatalf("exchange.NewSessionClient() error = %v, want nil", err)
			}
			for _, path := range []string{"/establish", "/observe"} {
				target, targetErr := core.ParseHTTPEndpoint(server.URL + path)
				if targetErr != nil {
					t.Fatalf("core.ParseHTTPEndpoint(%q) error = %v, want nil", path, targetErr)
				}
				_, sendErr := exchange.SendNoBodyJSON[sessionClientResponse](exchange.NoBodyJSONCall{
					Context: t.Context(),
					Client:  client,
					Request: exchange.NoBodyRequest{
						Target: target,
						Semantics: exchange.RequestSemantics{
							Method: exchange.MethodGet,
							Replay: exchange.ReplaySafe,
						},
						ExpectedStatus: mustHTTPStatus(t, http.StatusOK),
					},
					Policy: exchange.NoBodyJSONPolicy{
						Operation:         singleAttemptOperationPolicy(t),
						ResponseBodyLimit: mustByteCount(t, 1024),
					},
				})
				if sendErr != nil {
					t.Fatalf("exchange.SendNoBodyJSON(%q) error = %v, want nil", path, sendErr)
				}
			}
			if gotCookie != tc.wantCookie {
				t.Fatalf("second request cookie present = %t, want %t", gotCookie, tc.wantCookie)
			}
		})
	}
}

type sessionClientResponse struct {
	OK bool `json:"ok"`
}

func (r sessionClientResponse) Validate() error {
	if !r.OK {
		return core.ErrExchangeResponse
	}
	return nil
}
