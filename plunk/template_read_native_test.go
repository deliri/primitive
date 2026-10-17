package plunk

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestTemplateReadNativeProviderDoor(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodGet || r.URL.Path != "/templates/template-1" || r.Header.Get("Authorization") != "Bearer sk_local_fixture" || r.Header.Get("Idempotency-Key") != "" {
			t.Errorf("template read method/path/auth/key = %s %s %t %q, want owned GET with no replay key", r.Method, r.URL.Path, r.Header.Get("Authorization") == "Bearer sk_local_fixture", r.Header.Get("Idempotency-Key"))
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := io.WriteString(w, "template metadata stream"); err != nil {
			t.Errorf("native response write error = %v, want nil", err)
		}
	}))
	defer peer.Close()
	redirect, err := url.Parse(peer.URL)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := exchange.NewClient(&http.Client{Transport: resourceRedirect{target: redirect, transport: peer.Client().Transport}})
	if err != nil {
		t.Fatal(err)
	}
	credential, err := ParseCredential([]byte("sk_local_fixture"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := credential.Close(); err != nil {
			t.Errorf("credential close error = %v, want nil", err)
		}
	}()
	client, err := NewClient(wire, credential)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("client close error = %v, want nil", err)
		}
	}()
	// The typed endpoint constructor admits a documented provider URL even on
	// the baseline SDK, whose route gate currently refuses template reads.
	target, err := core.ParseHTTPEndpoint("https://" + core.PlunkAPIHost + "/templates/template-1")
	if err != nil {
		t.Fatal(err)
	}
	var destination bytes.Buffer
	got, err := client.Download(t.Context(), DownloadRequest{Stream: exchange.DownloadRequest{Target: target, Destination: &destination, Semantics: exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt}, ExpectedResponseContentType: core.HTTPMediaTypeJSON(), ExpectedStatus: core.HTTPStatusOK()}}, plunkStreamPolicy(t))
	if err != nil || got.Validate() != nil || destination.String() != "template metadata stream" || calls.Load() != 1 {
		t.Fatalf("native template read = (%q,%v,%d calls), want exact stream and one authenticated request", destination.String(), err, calls.Load())
	}
}

func TestTemplateReadOnlyRouteLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, path string
		method     exchange.Method
		want       bool
	}{
		{name: "owned resource read", path: "/templates/template-1", method: exchange.MethodGet, want: true},
		{name: "minimum opaque identifier", path: "/templates/a", method: exchange.MethodGet, want: true},
		{name: "opaque resource grammar", path: "/templates/A_9-z", method: exchange.MethodGet, want: true},
		{name: "resource missing", path: "/templates/", method: exchange.MethodGet},
		{name: "collection listing is not a resource read", path: "/templates", method: exchange.MethodGet},
		{name: "nested action cannot escape read door", path: "/templates/template-1/send", method: exchange.MethodGet},
		{name: "escaped delimiter cannot name a template", path: "/templates/template%2f1", method: exchange.MethodGet},
		{name: "create is not admitted", path: "/templates/template-1", method: exchange.MethodPost},
		{name: "update is not admitted", path: "/templates/template-1", method: exchange.MethodPatch},
		{name: "delete is not admitted", path: "/templates/template-1", method: exchange.MethodDelete},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := validOperationPath(tc.path, tc.method); got != tc.want {
				t.Fatalf("template path/method admission = %t, want %t", got, tc.want)
			}
		})
	}
	for _, id := range []string{"", "../template", "template/other", "x?secret=y", "x#fragment", "x\nnext"} {
		route := Route{Operation: ReadTemplate, ResourceID: id}
		got, err := route.Endpoint()
		method, methodErr := route.Method()
		if !errors.Is(err, core.ErrPlunkBinding) || !errors.Is(methodErr, core.ErrPlunkBinding) || got != (core.HTTPEndpoint{}) || method != 0 {
			t.Fatalf("rejected template resource %q = (%v,%v,%v,%v), want zero endpoint/method and binding refusal", id, got, method, err, methodErr)
		}
	}
}
