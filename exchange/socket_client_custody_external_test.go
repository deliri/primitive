package exchange_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type socketMutableTarget struct {
	value       url.URL
	later       *url.URL
	projections int
}

func (*socketMutableTarget) Validate() error { return nil }
func (s *socketMutableTarget) HTTPURL() url.URL {
	s.projections++
	if s.later != nil && s.projections > 1 {
		return *s.later
	}
	return s.value
}

func TestClientSocketCustodyLayerTriad(t *testing.T) {
	t.Parallel()
	type disturbance uint8
	const (
		undisturbed disturbance = iota
		replaceHeader
		replaceHeaderValue
		clearHeaderValues
		replaceCapture
		changeAuthority
		changePath
		reprojectAuthority
	)
	// Two distinct documents pressure per-call body custody independently of
	// the expected number of transport effects below. This fixture is read-only.
	requests := []transportDocument{
		{Message: "first request\x00\"\\\n"},
		{Message: "second request \u03bb\U0001f50c"},
	}
	cases := []struct {
		name                string
		disturbance         disturbance
		header              string
		capture             string
		omitHeaders         bool
		wantErr             error
		wantCalls           int
		wantProjections     int
		wantHeader          string
		wantCapture         string
		wantCapturedHeaders int
	}{
		{name: "positive original configuration crosses twice unchanged", wantCalls: 2, wantProjections: 1, header: "sealed-1", wantHeader: "sealed-1", capture: "observed-1", wantCapture: "observed-1", wantCapturedHeaders: 1},
		{name: "negative caller replacement cannot rename an owned header", disturbance: replaceHeader, wantCalls: 2, wantProjections: 1, header: "sealed-2", wantHeader: "sealed-2", capture: "observed-2", wantCapture: "observed-2", wantCapturedHeaders: 1},
		{name: "negative nested caller replacement cannot alter owned value", disturbance: replaceHeaderValue, wantCalls: 2, wantProjections: 1, header: "sealed-3", wantHeader: "sealed-3", capture: "observed-3", wantCapture: "observed-3", wantCapturedHeaders: 1},
		{name: "negative clearing borrowed input cannot invalidate the socket", disturbance: clearHeaderValues, wantCalls: 2, wantProjections: 1, header: "sealed-4", wantHeader: "sealed-4", capture: "observed-4", wantCapture: "observed-4", wantCapturedHeaders: 1},
		{name: "negative caller selection mutation cannot change observed fields", disturbance: replaceCapture, wantCalls: 2, wantProjections: 1, header: "sealed-5", wantHeader: "sealed-5", capture: "observed-5", wantCapture: "observed-5", wantCapturedHeaders: 1},
		{name: "negative same-path authority mutation cannot redirect credentials", disturbance: changeAuthority, wantCalls: 2, wantProjections: 1, header: "sealed-6", wantHeader: "sealed-6", capture: "observed-6", wantCapture: "observed-6", wantCapturedHeaders: 1},
		{name: "negative caller route mutation cannot invalidate sealed route", disturbance: changePath, wantCalls: 2, wantProjections: 1, header: "sealed-7", wantHeader: "sealed-7", capture: "observed-7", wantCapture: "observed-7", wantCapturedHeaders: 1},
		{name: "negative repeated projection cannot swap authority after admission", disturbance: reprojectAuthority, wantCalls: 2, wantProjections: 1, header: "sealed-8", wantHeader: "sealed-8", capture: "observed-8", wantCapture: "observed-8", wantCapturedHeaders: 1},
		{name: "neutral absent headers and capture remain absent", omitHeaders: true, wantCalls: 2, wantProjections: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			endpoint := mustEndpoint(t, "https://original.example.test/socket")
			target := &socketMutableTarget{value: endpoint.HTTPURL()}
			other := mustEndpoint(t, "https://other.example.test/socket").HTTPURL()
			if tc.disturbance == reprojectAuthority {
				target.later = &other
			}
			name, err := exchange.StandardHeaderAuthorization.Name()
			if err != nil {
				t.Fatalf("authorization name setup error = %v, want nil", err)
			}
			foreignName, err := exchange.StandardHeaderCacheControl.Name()
			if err != nil {
				t.Fatalf("cache control name setup error = %v, want nil", err)
			}
			headerValue, err := exchange.NewHeaderValue(tc.header)
			if err != nil {
				t.Fatalf("header fixture admission error = %v, want nil", err)
			}
			headers := []exchange.Header{{Name: name, Values: []exchange.HeaderValue{headerValue}}}
			selection := []core.HTTPHeaderName{name}
			if tc.omitHeaders {
				headers = nil
				selection = nil
			}
			wire, err := (transportDocument{Message: "exact response"}).MarshalJSON()
			if err != nil {
				t.Fatalf("response fixture encoding error = %v, want nil", err)
			}
			requestWires := make([][]byte, len(requests))
			for index, request := range requests {
				if err := request.Validate(); err != nil {
					t.Fatalf("request %d fixture validation = %v, want nil", index, err)
				}
				requestWires[index], err = request.MarshalJSON()
				if err != nil {
					t.Fatalf("request %d fixture encoding = %v, want nil", index, err)
				}
			}
			var calls int
			transport := bindingTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				if calls > len(requestWires) {
					t.Fatalf("transport call index = %d, want at most %d input documents", calls, len(requestWires))
				}
				if r.URL.String() != endpoint.String() || r.Header.Get(name.String()) != tc.wantHeader || r.Header.Get(foreignName.String()) != "" {
					t.Errorf("wire URL/header/foreign header = (%q, %q, %q), want (%q, %q, empty)", r.URL.String(), r.Header.Get(name.String()), r.Header.Get(foreignName.String()), endpoint.String(), tc.wantHeader)
				}
				wantWire := requestWires[calls-1]
				gotWire, readErr := io.ReadAll(io.LimitReader(r.Body, int64(len(wantWire))+1))
				gotReadErr := errors.Join(readErr, r.Body.Close())
				if gotReadErr != nil || !bytes.Equal(gotWire, wantWire) {
					t.Fatalf("request %d wire/error = (%q, %v), want (%q, nil)", calls, gotWire, gotReadErr, wantWire)
				}
				if r.Method != http.MethodPost || r.ContentLength != int64(len(wantWire)) || r.Header.Get(core.HTTPHeaderContentType().String()) != core.HTTPMediaTypeJSON().String() {
					t.Fatalf("request %d method/length/content type = (%q, %d, %q), want (%q, %d, %q)", calls, r.Method, r.ContentLength, r.Header.Get(core.HTTPHeaderContentType().String()), http.MethodPost, len(wantWire), core.HTTPMediaTypeJSON().String())
				}
				responseHeaders := make(http.Header)
				responseHeaders.Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
				responseHeaders.Set(name.String(), tc.capture)
				responseHeaders.Set(foreignName.String(), "foreign")
				return &http.Response{StatusCode: http.StatusAccepted, Header: responseHeaders, Body: io.NopCloser(bytes.NewReader(wire)), ContentLength: int64(len(wire)), Request: r}, nil
			})
			socket, err := exchange.NewClientSocket(exchange.ClientSocketConfiguration{
				Target: target, Client: mustExchangeClient(t, &http.Client{Transport: transport}),
				Headers: exchange.Headers{Values: headers}, CaptureHeaders: exchange.HeaderSelection{Names: selection},
				Contract: socketPairContract(t, "/socket", exchange.ReplaySingleAttempt), Operation: singleAttemptOperationPolicy(t),
			})
			if err != nil {
				t.Fatalf("NewClientSocket() error = %v, want nil", err)
			}
			switch tc.disturbance {
			case replaceHeader:
				headers[0] = exchange.Header{Name: foreignName, Values: headers[0].Values}
			case replaceHeaderValue:
				headers[0].Values[0], err = exchange.NewHeaderValue("mutated")
			case clearHeaderValues:
				clear(headers[0].Values)
			case replaceCapture:
				selection[0] = foreignName
			case changeAuthority:
				target.value = other
			case changePath:
				target.value.Path = "/other"
			}
			if err != nil {
				t.Fatalf("mutation fixture admission error = %v, want nil", err)
			}
			for attempt, request := range requests {
				got, gotErr := exchange.SendSocketJSON[transportDocument, transportDocument](t.Context(), socket, request)
				if !errors.Is(gotErr, tc.wantErr) || got.Body.Message != "exact response" {
					t.Fatalf("send %d response/error = (%+v, %v), want exact response and %v", attempt, got, gotErr, tc.wantErr)
				}
				if got.Metadata.Attempts != 1 || got.Metadata.Status != mustHTTPStatus(t, http.StatusAccepted) || got.Metadata.Bytes.Uint64() != uint64(len(wire)) || len(got.Metadata.Headers.Values) != tc.wantCapturedHeaders {
					t.Fatalf("send %d metadata = %+v, want one accepted attempt, %d bytes, %d captured header", attempt, got.Metadata, len(wire), tc.wantCapturedHeaders)
				}
				if tc.wantCapturedHeaders == 0 {
					continue
				}
				captured := got.Metadata.Headers.Values[0]
				if captured.Name != name || len(captured.Values) != 1 {
					t.Fatalf("captured header = %+v, want original name and one value", captured)
				}
				value, err := captured.Values[0].Value()
				if err != nil || value != tc.wantCapture {
					t.Fatalf("captured value/error = (%q, %v), want (%q, nil)", value, err, tc.wantCapture)
				}
			}
			if calls != tc.wantCalls || target.projections != tc.wantProjections {
				t.Fatalf("transport calls/target projections = (%d, %d), want (%d, %d)", calls, target.projections, tc.wantCalls, tc.wantProjections)
			}
		})
	}
}
