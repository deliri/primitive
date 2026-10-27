package cloudbilling

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/currency"
	"github.com/deliri/primitive/v2026/exchange"
)

func fixtureOptions(t testing.TB, endpoint string) ServerOptions {
	t.Helper()
	target, err := core.ParseHTTPEndpoint(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	return ServerOptions{Endpoint: target, Currency: currency.CodeUSD, PageSize: 5, Limits: fixtureLimits(t)}
}

type proofSink struct {
	bytes.Buffer
	failAt     int
	writes     int
	failure    error
	afterWrite context.CancelFunc
}

func (s *proofSink) Write(p []byte) (int, error) {
	s.writes++
	if s.writes == s.failAt {
		return 0, s.failure
	}
	n, err := s.Buffer.Write(p)
	if s.afterWrite != nil {
		s.afterWrite()
	}
	return n, err
}
func (s *proofSink) WriteString(value string) (int, error) { return s.Write([]byte(value)) }

func TestListSKUsOnePageAndOriginalSourceLayerTriad(t *testing.T) {
	t.Parallel()
	page := fixturePage()
	page.Next = "provider second page /+?"
	canonical := encodedFixture(t, page)
	// Noncanonical whitespace proves the sink and digest retain provider bytes.
	body := append(append([]byte(" \n"), canonical...), '\n')
	cases := []struct {
		name        string
		status      int
		data        []byte
		token       PageToken
		failAt      int
		failure     error
		short       bool
		bodyMaximum int
		truncate    bool
		cancel      bool
		nilProof    bool
		cancelProof bool
		wantErr     error
		wantProof   bool
		wantCalls   int
	}{
		{name: "complete page retains exact source and next token", status: 200, wantProof: true, wantCalls: 1},
		{name: "opaque token escaped exactly once", status: 200, token: "/+=?&", wantProof: true, wantCalls: 1},
		{name: "empty next token is no autopagination", status: 200, data: encodedFixture(t, fixturePage()), wantProof: true, wantCalls: 1},
		{name: "empty SKUs are neutral provider fact", status: 200, data: encodedFixture(t, Page{SKUs: []SKU{}}), wantProof: true, wantCalls: 1},
		{name: "malformed complete 200 retained before refusal", status: 200, data: []byte("malformed"), wantErr: core.ErrCloudBillingResponse, wantProof: true, wantCalls: 1},
		{name: "missing page structure retained before refusal", status: 200, data: []byte("null"), wantErr: core.ErrCloudBillingResponse, wantProof: true, wantCalls: 1},
		{name: "404 not an accepted page", status: 404, wantErr: core.ErrCloudBillingResponse, wantCalls: 1},
		{name: "429 no hidden retry", status: 429, wantErr: core.ErrCloudBillingResponse, wantCalls: 1},
		{name: "503 no hidden retry", status: 503, wantErr: core.ErrCloudBillingResponse, wantCalls: 1},
		{name: "206 not accepted as complete catalog", status: 206, wantErr: core.ErrCloudBillingResponse, wantCalls: 1},
		{name: "redirect never followed", status: 302, wantErr: core.ErrCloudBillingResponse, wantCalls: 1},
		{name: "declared extent truncated refuses partial proof", status: 200, truncate: true, wantErr: core.ErrCloudBillingResponse, wantCalls: 1},
		{name: "body below policy limit", status: 200, bodyMaximum: len(body) + 1, wantProof: true, wantCalls: 1},
		{name: "body exact policy limit", status: 200, bodyMaximum: len(body), wantProof: true, wantCalls: 1},
		{name: "body above policy limit", status: 200, bodyMaximum: len(body) - 1, wantErr: core.ErrCloudBillingResponse, wantCalls: 1},
		{name: "proof body sink failure preserved", status: 200, failAt: 1, failure: io.ErrClosedPipe, wantErr: io.ErrClosedPipe, wantCalls: 1},
		{name: "proof body short write refused", status: 200, failAt: 1, short: true, wantErr: io.ErrShortWrite, wantCalls: 1},
		{name: "proof separator sink failure preserved", status: 200, failAt: 2, failure: io.ErrClosedPipe, wantErr: io.ErrClosedPipe, wantCalls: 1},
		{name: "proof separator short write refused", status: 200, failAt: 2, short: true, wantErr: io.ErrShortWrite, wantCalls: 1},
		{name: "canceled context issues no request", status: 200, cancel: true, wantErr: context.Canceled},
		{name: "cancellation during proof write returns no receipt", status: 200, cancelProof: true, wantErr: context.Canceled, wantProof: true, wantCalls: 1},
		{name: "nil proof issues no request", status: 200, nilProof: true, wantErr: core.ErrCloudBillingContract},
		{name: "oversized input token issues no request", status: 200, token: PageToken(strings.Repeat("x", 65)), wantErr: core.ErrCloudBillingContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			data := tc.data
			if data == nil {
				data = body
			}
			var calls atomic.Int64
			seen := make(chan string, 1)
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) == 1 {
					seen <- r.Method + " " + r.URL.RequestURI()
				}
				if r.URL.Query().Get("currencyCode") != "USD" || r.URL.Query().Get("pageSize") != "5" || r.URL.Query().Get("pageToken") != string(tc.token) {
					t.Errorf("query=%q, want typed request facts", r.URL.RawQuery)
				}
				if tc.status == 302 {
					w.Header().Set("Location", "/must-not-follow")
				}
				if tc.truncate {
					w.Header().Set("Content-Length", strconv.Itoa(len(data)+1))
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write(data)
			}))
			defer provider.Close()
			client, err := exchange.NewClient(provider.Client())
			if err != nil {
				t.Fatal(err)
			}
			options := fixtureOptions(t, provider.URL+"/v1/services/service/skus")
			if tc.bodyMaximum != 0 {
				options.Limits.JSON.DocumentMaximumBytes, err = core.NewByteCount(uint64(tc.bodyMaximum))
				if err != nil {
					t.Fatal(err)
				}
			}
			server, err := NewServer(client, options)
			if err != nil {
				t.Fatal(err)
			}
			sink := &proofSink{failAt: tc.failAt, failure: tc.failure}
			var proof io.Writer = sink
			if tc.nilProof {
				proof = nil
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancelProof {
				sink.afterWrite = cancel
			}
			if tc.cancel {
				cancel()
			}
			got, err := server.ListSKUs(ctx, ListSKUsRequest{Token: tc.token, Proof: proof}, exchange.StreamPolicy{Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject}})
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || got.Page.SKUs != nil || got.Source.URL.String() != "" {
					t.Fatalf("ListSKUs()=(%+v,%v), want zero and %v", got, err, tc.wantErr)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if got.Source.SHA256 != core.SHA256Of(data) || got.Source.Bytes.Uint64() != uint64(len(data)) {
					t.Fatalf("source=(%v,%v), want exact body digest/extent", got.Source.SHA256, got.Source.Bytes)
				}
				if !strings.HasPrefix(<-seen, "GET /v1/services/service/skus?") {
					t.Fatal("provider request did not use owning ListSKUs path")
				}
				decoded, err := DecodePage(data, options.Limits)
				if err != nil {
					t.Fatal(err)
				}
				if got.Page.Next != decoded.Next || len(got.Page.SKUs) != len(decoded.SKUs) {
					t.Fatalf("page=%+v, want decoded provider facts %+v", got.Page, decoded)
				}
			}
			if int(calls.Load()) != tc.wantCalls {
				t.Fatalf("provider calls=%d, want %d", calls.Load(), tc.wantCalls)
			}
			if tc.wantProof {
				want := append(append([]byte{}, data...), '\n')
				if !bytes.Equal(sink.Bytes(), want) {
					t.Fatalf("proof=%q, want exact original plus separator %q", sink.Bytes(), want)
				}
			} else if tc.failAt == 0 && sink.Len() != 0 {
				t.Fatalf("proof bytes=%d, want zero for incomplete/refused transport", sink.Len())
			}
		})
	}
}
