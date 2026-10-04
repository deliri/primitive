package plunk

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestDocumentedResourceOperationLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		route   Route
		path    string
		method  exchange.Method
		wantErr error
	}{
		{"send email", Route{Operation: SendEmail}, "/v1/send", exchange.MethodPost, nil},
		{"upsert contact", Route{Operation: UpsertContact}, "/contacts", exchange.MethodPost, nil},
		{"list contacts", Route{Operation: ListContacts}, "/contacts", exchange.MethodGet, nil},
		{"read contact", Route{ReadContact, "cnt_abc"}, "/contacts/cnt_abc", exchange.MethodGet, nil},
		{"update contact", Route{UpdateContact, "cnt_abc"}, "/contacts/cnt_abc", exchange.MethodPatch, nil},
		{"delete contact", Route{DeleteContact, "cnt_abc"}, "/contacts/cnt_abc", exchange.MethodDelete, nil},
		{"create campaign", Route{Operation: CreateCampaign}, "/campaigns", exchange.MethodPost, nil},
		{"list campaigns", Route{Operation: ListCampaigns}, "/campaigns", exchange.MethodGet, nil},
		{"read campaign", Route{ReadCampaign, "campaign-1"}, "/campaigns/campaign-1", exchange.MethodGet, nil},
		{"update campaign", Route{UpdateCampaign, "campaign-1"}, "/campaigns/campaign-1", exchange.MethodPatch, nil},
		{"delete campaign", Route{DeleteCampaign, "campaign-1"}, "/campaigns/campaign-1", exchange.MethodDelete, nil},
		{"send campaign", Route{SendCampaign, "campaign-1"}, "/campaigns/campaign-1/send", exchange.MethodPost, nil},
		{"cancel campaign", Route{CancelCampaign, "campaign-1"}, "/campaigns/campaign-1/cancel", exchange.MethodPost, nil},
		{"campaign statistics", Route{ReadCampaignStats, "campaign-1"}, "/campaigns/campaign-1/stats", exchange.MethodGet, nil},
		{"campaign preview send", Route{TestCampaign, "campaign-1"}, "/campaigns/campaign-1/test", exchange.MethodPost, nil},
		{name: "absent operation", wantErr: core.ErrPlunkBinding},
		{name: "future operation", route: Route{Operation: 255}, wantErr: core.ErrPlunkBinding},
		{name: "missing resource", route: Route{Operation: SendCampaign}, wantErr: core.ErrPlunkBinding},
		{name: "unexpected resource", route: Route{SendEmail, "campaign-1"}, wantErr: core.ErrPlunkBinding},
		{name: "traversal resource", route: Route{SendCampaign, "../send"}, wantErr: core.ErrPlunkBinding},
		{name: "escaped delimiter", route: Route{ReadContact, "a%2fb"}, wantErr: core.ErrPlunkBinding},
		{name: "query injection", route: Route{ReadContact, "a?secret=b"}, wantErr: core.ErrPlunkBinding},
		{name: "fragment injection", route: Route{ReadCampaign, "a#b"}, wantErr: core.ErrPlunkBinding},
		{name: "unicode identity", route: Route{ReadCampaign, "你好"}, wantErr: core.ErrPlunkBinding},
		{name: "control identity", route: Route{ReadCampaign, "a\nb"}, wantErr: core.ErrPlunkBinding},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			target, e := tc.route.Endpoint()
			method, methodErr := tc.route.Method()
			if !errors.Is(e, tc.wantErr) || !errors.Is(methodErr, tc.wantErr) {
				t.Fatalf("route errors=%v,%v, want %v", e, methodErr, tc.wantErr)
			}
			if e != nil {
				if target != (core.HTTPEndpoint{}) || method != exchange.Method(0) {
					t.Fatalf("rejected route = (%v, %v), want zero endpoint and method", target, method)
				}
				return
			}
			got := target.HTTPURL()
			if got.Host != core.PlunkAPIHost || got.Path != tc.path || method != tc.method || !validOperationPath(got.Path, method) {
				t.Fatalf("route=%s,%v, want %s,%v", target.String(), method, tc.path, tc.method)
			}
		})
	}
}
func FuzzResourceRouteSemanticClosure(f *testing.F) {
	for op := SendEmail; op <= TestCampaign; op++ {
		r := Route{Operation: op}
		switch op {
		case SendEmail, UpsertContact, ListContacts, CreateCampaign, ListCampaigns:
		default:
			r.ResourceID = "provider-1"
		}
		if r.Validate() != nil {
			f.Fatalf("nominal route Validate() = %v, want nil", r.Validate())
		}
		f.Add(uint8(op), r.ResourceID)
	}
	f.Add(uint8(SendCampaign), "../send")
	f.Fuzz(func(t *testing.T, op uint8, id string) {
		r := Route{Operation: Operation(op), ResourceID: id}
		endpoint, e := r.Endpoint()
		method, me := r.Method()
		if e != nil {
			if !errors.Is(e, core.ErrPlunkBinding) || !errors.Is(me, core.ErrPlunkBinding) || endpoint != (core.HTTPEndpoint{}) {
				t.Fatalf("refusal=%+v,%v,%v, want zero bound refusal", endpoint, e, me)
			}
			return
		}
		if r.Validate() != nil || me != nil || endpoint.Validate() != nil {
			t.Fatalf("admitted route=%+v,%v, want valid", r, me)
		}
		u := endpoint.HTTPURL()
		if u.Host != core.PlunkAPIHost || u.Scheme != core.SchemeHTTPS || u.User != nil || u.RawQuery != "" || u.Fragment != "" || !validOperationPath(u.Path, method) {
			t.Fatalf("endpoint=%s, want single provider-bound operation", endpoint.String())
		}
		if id != "" && !strings.Contains(u.Path, "/"+id) {
			t.Fatalf("endpoint=%s, want exact resource %q", endpoint.String(), id)
		}
		again, e := r.Endpoint()
		if e != nil || again != endpoint {
			t.Fatalf("repeat=%+v,%v, want same nominal endpoint", again, e)
		}
	})
}

type resourceRedirect struct {
	target    *url.URL
	transport http.RoundTripper
}

func (r resourceRedirect) RoundTrip(req *http.Request) (*http.Response, error) {
	copy := req.Clone(req.Context())
	u := *req.URL
	u.Scheme = r.target.Scheme
	u.Host = r.target.Host
	copy.URL = &u
	return r.transport.RoundTrip(copy)
}
func TestResourceClientUsesExchangeAndSingleAttemptLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		route     Route
		method    exchange.Method
		wantCalls int64
		wantErr   error
	}{
		{"create campaign reaches provider", Route{Operation: CreateCampaign}, exchange.MethodPost, 1, nil},
		{"send campaign reaches provider", Route{SendCampaign, "campaign-1"}, exchange.MethodPost, 1, nil},
		{"cancel campaign reaches provider", Route{CancelCampaign, "campaign-1"}, exchange.MethodPost, 1, nil},
		{"contact patch reaches provider", Route{UpdateContact, "contact-1"}, exchange.MethodPatch, 1, nil},
		{"contact delete reaches provider", Route{DeleteContact, "contact-1"}, exchange.MethodDelete, 1, nil},
		{"read cannot cross write door", Route{ReadCampaign, "campaign-1"}, exchange.MethodGet, 0, core.ErrPlunkBinding},
		{"wrong operation method emits no request", Route{SendCampaign, "campaign-1"}, exchange.MethodPatch, 0, core.ErrPlunkBinding},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int64
			wantKey := "resource-fixture"
			if tc.method == exchange.MethodDelete {
				wantKey = ""
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Header.Get("Authorization") != "Bearer sk_local_fixture" || r.Header.Get("Idempotency-Key") != wantKey || r.Method != tc.method.String() {
					t.Errorf("HTTP method/auth/key did not bind to request")
				}
				n, e := io.Copy(io.Discard, r.Body)
				if e != nil || n == 0 {
					t.Errorf("request body bytes/error=%d/%v, want nonempty body", n, e)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, "{}")
			}))
			defer server.Close()
			target, e := url.Parse(server.URL)
			if e != nil {
				t.Fatal(e)
			}
			httpClient, e := exchange.NewClient(&http.Client{Transport: resourceRedirect{target, server.Client().Transport}})
			if e != nil {
				t.Fatal(e)
			}
			credential, e := ParseCredential([]byte("sk_local_fixture"))
			if e != nil {
				t.Fatal(e)
			}
			defer func() {
				if err := credential.Close(); err != nil {
					t.Errorf("credential.Close() = %v, want nil", err)
				}
			}()
			client, e := NewClient(httpClient, credential)
			if e != nil {
				t.Fatal(e)
			}
			defer func() {
				if err := client.Close(); err != nil {
					t.Errorf("client.Close() = %v, want nil", err)
				}
			}()
			request := plunkIdempotencyRequest(t, "resource-fixture")
			request.Target, e = tc.route.Endpoint()
			if e != nil {
				t.Fatal(e)
			}
			request.Semantics.Method = tc.method
			if tc.method == exchange.MethodDelete {
				request.Semantics.Replay = exchange.ReplaySingleAttempt
				request.Semantics.IdempotencyKey = exchange.IdempotencyKey{}
			}
			_, gotErr := client.RoundTrip(t.Context(), Request{Stream: request}, plunkStreamPolicy(t))
			if !errors.Is(gotErr, tc.wantErr) || calls.Load() != tc.wantCalls {
				t.Fatalf("RoundTrip()=%v calls=%d, want %v/%d", gotErr, calls.Load(), tc.wantErr, tc.wantCalls)
			}
		})
	}
}
