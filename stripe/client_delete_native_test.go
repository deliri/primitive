package stripe

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type stripeDeleteFixtureMode uint8

const (
	stripeDeleteFixtureComplete stripeDeleteFixtureMode = iota
	stripeDeleteFixtureLostReply
	stripeDeleteFixtureUnavailable
	stripeDeleteFixtureCanceled
)

type stripeDeleteFixtureDocument struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// Actual local TLS, native HTTP and Primitive authenticated transport. The
// subscription is a test-owned provider fixture, not a live Stripe object.
func TestDeleteNativeLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		mode       stripeDeleteFixtureMode
		wantDelete int64
		wantErr    error
		readBack   bool
	}{
		{name: "single delete sends exact form without idempotency header", mode: stripeDeleteFixtureComplete, wantDelete: 1},
		{name: "lost delete reply preserves native EOF then readback proves committed effect", mode: stripeDeleteFixtureLostReply, wantDelete: 1, wantErr: io.ErrUnexpectedEOF, readBack: true},
		{name: "provider503 preserves status and never retries deletion", mode: stripeDeleteFixtureUnavailable, wantDelete: 1, wantErr: core.ErrExchangeUnavailable},
		{name: "canceled caller emits no provider mutation", mode: stripeDeleteFixtureCanceled, wantErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var deletes, reads atomic.Int64
			var ended atomic.Bool
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Host != core.StripeAPIHost || r.URL.Path != "/v1/subscriptions/sub_owned" || r.Header.Get("Authorization") != "Bearer rk_test_credential" || r.Header.Get(core.StripeVersionHeaderName) != core.StripeAPIVersion || r.Header.Get(core.HTTPHeaderIdempotencyKey().String()) != "" {
					t.Errorf("provider request host/path/auth/version/key binding refused")
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				switch r.Method {
				case http.MethodDelete:
					deletes.Add(1)
					if err := r.ParseForm(); err != nil {
						t.Errorf("ParseForm()=%v, want nil", err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					// net/http ParseForm does not decode DELETE bodies; use native URL codec.
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Errorf("read form=%v, want nil", err)
						return
					}
					form, err := url.ParseQuery(string(body))
					if err != nil || form.Get("invoice_now") != "false" || form.Get("prorate") != "false" || len(form) != 2 {
						t.Errorf("delete form=%v/%v, want exact false flags", form, err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					if tc.mode == stripeDeleteFixtureUnavailable {
						w.WriteHeader(http.StatusServiceUnavailable)
						return
					}
					ended.Store(true)
					if tc.mode == stripeDeleteFixtureLostReply {
						w.Header().Set("Content-Length", "100")
						_, _ = io.WriteString(w, `{"id":`)
						return
					}
				case http.MethodGet:
					reads.Add(1)
				default:
					t.Errorf("provider method=%s, want DELETE or GET", r.Method)
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				status := "active"
				if ended.Load() {
					status = "canceled"
				}
				raw, err := json.Marshal(stripeDeleteFixtureDocument{ID: "sub_owned", Status: status})
				if err != nil {
					t.Errorf("provider fixture encode=%v, want nil", err)
					return
				}
				_, _ = w.Write(raw)
			}))
			t.Cleanup(server.Close)
			endpoint, err := url.Parse(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			native := server.Client()
			transport, ok := native.Transport.(*http.Transport)
			if !ok {
				t.Fatal("native TLS client transport missing")
			}
			transport.TLSClientConfig = transport.TLSClientConfig.Clone()
			transport.TLSClientConfig.ServerName = endpoint.Hostname()
			transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, endpoint.Host)
			}
			t.Cleanup(transport.CloseIdleConnections)
			client, err := exchange.NewClient(native)
			if err != nil {
				t.Fatal(err)
			}
			credential, err := ParseCredential([]byte("rk_test_credential"))
			if err != nil {
				t.Fatal(err)
			}
			provider, err := NewClient(client, credential)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := errors.Join(provider.Close(), credential.Close()); err != nil {
					t.Errorf("credential cleanup=%v, want nil", err)
				}
			})
			target, err := core.ParseHTTPEndpoint("https://" + core.StripeAPIHost + "/v1/subscriptions/sub_owned")
			if err != nil {
				t.Fatal(err)
			}
			var reply bytes.Buffer
			request := Request{Stream: exchange.StreamRoundTripRequest{Target: target, Source: bytes.NewBufferString("invoice_now=false&prorate=false"), Destination: &reply, Semantics: exchange.RequestSemantics{Method: exchange.MethodDelete, Replay: exchange.ReplaySingleAttempt}, RequestContentType: core.HTTPMediaTypeFormURLEncoded(), ExpectedResponseContentType: core.HTTPMediaTypeJSON(), ExpectedStatus: core.HTTPStatusOK()}}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.mode == stripeDeleteFixtureCanceled {
				cancel()
			}
			_, gotErr := provider.RoundTrip(ctx, request, stripeStreamPolicy(t))
			if !errors.Is(gotErr, tc.wantErr) || deletes.Load() != tc.wantDelete {
				t.Fatalf("delete error=%v calls=%d, want %v/%d", gotErr, deletes.Load(), tc.wantErr, tc.wantDelete)
			}
			if tc.mode == stripeDeleteFixtureUnavailable {
				var nativeStatus exchange.StatusError
				if !errors.As(gotErr, &nativeStatus) || func() bool {
					value, err := nativeStatus.Status().Int()
					return err != nil || value != http.StatusServiceUnavailable
				}() {
					t.Fatalf("delete status cause=%v, want native503", gotErr)
				}
			}
			if tc.readBack {
				reply.Reset()
				_, err = provider.Download(t.Context(), DownloadRequest{Stream: exchange.DownloadRequest{Target: target, Destination: &reply, Semantics: exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt}, ExpectedResponseContentType: core.HTTPMediaTypeJSON(), ExpectedStatus: core.HTTPStatusOK()}}, stripeStreamPolicy(t))
				if err != nil {
					t.Fatalf("readback=%v, want nil", err)
				}
			}
			if tc.mode == stripeDeleteFixtureComplete || tc.readBack {
				var document stripeDeleteFixtureDocument
				if err := json.Unmarshal(reply.Bytes(), &document); err != nil || document.ID != "sub_owned" || document.Status != "canceled" {
					t.Fatalf("observed effect=%+v/%v, want exact canceled owned subscription", document, err)
				}
			}
			wantReads := int64(0)
			if tc.readBack {
				wantReads = 1
			}
			if reads.Load() != wantReads || deletes.Load() != tc.wantDelete {
				t.Fatalf("read/delete counts=%d/%d, want %d/%d without mutation replay", reads.Load(), deletes.Load(), wantReads, tc.wantDelete)
			}
		})
	}
}

func TestDeleteRequestMethodReplayLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		mutate  func(*Request)
		wantErr error
	}{
		{name: "v1 form delete admits one native attempt"},
		{name: "delete refuses generic idempotent replay", mutate: func(r *Request) { r.Stream.Semantics.Replay = exchange.ReplayIdempotent }, wantErr: core.ErrStripeBinding},
		{name: "delete refuses idempotency header policy", mutate: func(r *Request) {
			key, err := exchange.ParseIdempotencyKey("existing-owned-operation")
			if err != nil {
				panic(err)
			}
			r.Stream.Semantics.Replay = exchange.ReplaySingleAttemptWithIdempotencyKey
			r.Stream.Semantics.IdempotencyKey = key
		}, wantErr: core.ErrStripeBinding},
		{name: "v2 json delete remains unsupported", mutate: func(r *Request) {
			target, err := core.ParseHTTPEndpoint("https://" + core.StripeAPIHost + "/v2/core/accounts/acct_owned")
			if err != nil {
				panic(err)
			}
			r.Stream.Target = target
			r.Stream.RequestContentType = core.HTTPMediaTypeJSON()
		}, wantErr: core.ErrStripeBinding},
		{name: "v1 delete refuses JSON form mismatch", mutate: func(r *Request) { r.Stream.RequestContentType = core.HTTPMediaTypeJSON() }, wantErr: core.ErrStripeBinding},
		{name: "delete refuses nonJSON response contract", mutate: func(r *Request) { r.Stream.ExpectedResponseContentType = core.HTTPMediaTypeFormURLEncoded() }, wantErr: core.ErrStripeBinding},
		{name: "patch cannot borrow delete admission", mutate: func(r *Request) { r.Stream.Semantics.Method = exchange.MethodPatch }, wantErr: core.ErrStripeBinding},
		{name: "existing post single attempt remains admitted", mutate: func(r *Request) { r.Stream.Semantics.Method = exchange.MethodPost }},
		{name: "future method cannot borrow mutation admission", mutate: func(r *Request) { r.Stream.Semantics.Method = exchange.Method(255) }, wantErr: core.ErrStripeBinding},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			stream := stripeIdempotencyRequest(t, "owned-operation")
			stream.Semantics = exchange.RequestSemantics{Method: exchange.MethodDelete, Replay: exchange.ReplaySingleAttempt}
			request := Request{Stream: stream}
			if tc.mutate != nil {
				tc.mutate(&request)
			}
			if got := request.Validate(); !errors.Is(got, tc.wantErr) {
				t.Fatalf("Request.Validate()=%v, want %v", got, tc.wantErr)
			}
		})
	}
}
