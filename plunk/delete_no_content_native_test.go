package plunk

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// Real SDK -> Exchange -> net/http -> local TLS peer. The peer emits the
// documented204 without Content-Type or a body, not the old200 JSON fixture.
// https://docs.useplunk.com/api-reference/contacts/deleteContact
func TestDeleteNoContentNativeLayerTriad(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		route     Route
		method    exchange.Method
		status    int
		wantCalls int64
		wantErr   error
	}{
		{name: "contact204 with no representation reaches actual peer", route: Route{Operation: DeleteContact, ResourceID: "contact_1"}, method: exchange.MethodDelete, status: http.StatusNoContent, wantCalls: 1},
		{name: "campaign204 shares no representation agreement", route: Route{Operation: DeleteCampaign, ResourceID: "campaign_1"}, method: exchange.MethodDelete, status: http.StatusNoContent, wantCalls: 1},
		{name: "body-bearing success cannot omit representation policy", route: Route{Operation: DeleteContact, ResourceID: "contact_1"}, method: exchange.MethodDelete, status: http.StatusOK, wantErr: core.ErrPlunkBinding},
		{name: "404 cannot be declared empty success", route: Route{Operation: DeleteContact, ResourceID: "contact_1"}, method: exchange.MethodDelete, status: http.StatusNotFound, wantErr: core.ErrPlunkBinding},
		{name: "send204 cannot bypass JSON response policy", route: Route{Operation: SendEmail}, method: exchange.MethodPost, status: http.StatusNoContent, wantErr: core.ErrPlunkBinding},
		{name: "patch204 cannot bypass JSON response policy", route: Route{Operation: UpdateContact, ResourceID: "contact_1"}, method: exchange.MethodPatch, status: http.StatusNoContent, wantErr: core.ErrPlunkBinding},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int64
			peer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != test.method.String() || r.URL.RawQuery != "" || r.Header.Get("Authorization") != "Bearer sk_native_contact_delete" {
					t.Errorf("native request method/query/auth = %q/%q/%t, want owned method/empty/true", r.Method, r.URL.RawQuery, r.Header.Get("Authorization") == "Bearer sk_native_contact_delete")
				}
				if length, err := io.Copy(io.Discard, r.Body); err != nil || length != 0 {
					t.Errorf("native request bytes/error = %d/%v, want0/nil", length, err)
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer peer.Close()
			transport := peer.Client().Transport.(*http.Transport).Clone()
			transport.TLSClientConfig.ServerName = "example.com"
			transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, peer.Listener.Addr().String())
			}
			defer transport.CloseIdleConnections()
			native, err := exchange.NewClient(&http.Client{Transport: transport})
			if err != nil {
				t.Fatal(err)
			}
			credential, err := ParseCredential([]byte("sk_native_contact_delete"))
			if err != nil {
				t.Fatal(err)
			}
			client, err := NewClient(native, credential)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := errors.Join(client.Close(), credential.Close()); err != nil {
					t.Errorf("owned credential close = %v, want nil", err)
				}
			})
			target, err := test.route.Endpoint()
			if err != nil {
				t.Fatal(err)
			}
			var status core.HTTPStatusCode
			if err := status.AdmitInt(test.status); err != nil {
				t.Fatal(err)
			}
			request := Request{Stream: exchange.StreamRoundTripRequest{Target: target, Source: http.NoBody, Destination: io.Discard, RequestContentType: core.HTTPMediaTypeJSON(), ExpectedStatus: status, Semantics: exchange.RequestSemantics{Method: test.method, Replay: exchange.ReplaySingleAttempt}}}
			response, gotErr := client.RoundTrip(t.Context(), request, plunkStreamPolicy(t))
			if !errors.Is(gotErr, test.wantErr) || calls.Load() != test.wantCalls {
				t.Fatalf("RoundTrip error/calls=%v/%d, want%v/%d", gotErr, calls.Load(), test.wantErr, test.wantCalls)
			}
			if gotErr == nil && (response.Stream.Metadata.Status != status || response.Stream.Metadata.Attempts != 1 || response.Stream.Metadata.Bytes.Uint64() != 0) {
				t.Fatalf("native204 metadata=%+v, want exact204/oneattempt/zero bytes", response.Stream.Metadata)
			}
		})
	}
}

func TestNoRepresentationPolicyExhaustsHTTPStatusDomain(t *testing.T) {
	t.Parallel()
	target, err := (Route{Operation: DeleteContact, ResourceID: "contact_1"}).Endpoint()
	if err != nil {
		t.Fatal(err)
	}
	for status := 100; status <= 599; status++ {
		var code core.HTTPStatusCode
		if err := code.AdmitInt(status); err != nil {
			t.Fatal(err)
		}
		request := Request{Stream: exchange.StreamRoundTripRequest{Target: target, Source: http.NoBody, Destination: io.Discard, RequestContentType: core.HTTPMediaTypeJSON(), ExpectedStatus: code, Semantics: exchange.RequestSemantics{Method: exchange.MethodDelete, Replay: exchange.ReplaySingleAttempt}}}
		gotErr := request.Validate()
		if status == http.StatusNoContent {
			if gotErr != nil {
				t.Fatalf("204 no representation policy=%v, want nil", gotErr)
			}
		} else if !errors.Is(gotErr, core.ErrPlunkBinding) {
			t.Fatalf("status%d empty representation policy=%v, want ErrPlunkBinding", status, gotErr)
		}
	}
}
