package loops

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type idempotencyRoundTripFunc func(*http.Request) (*http.Response, error)

func (function idempotencyRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func loopsSendRequest(t testing.TB, route Route, method exchange.Method, body []byte, semantics exchange.RequestSemantics) exchange.StreamRoundTripRequest {
	t.Helper()
	target, err := route.Endpoint()
	if err != nil {
		t.Fatalf("Route.Endpoint() error = %v, want nil", err)
	}
	semantics.Method = method
	return exchange.StreamRoundTripRequest{
		Target: target, Source: bytes.NewReader(body), Destination: io.Discard,
		Semantics: semantics, RequestContentType: core.HTTPMediaTypeJSON(), ExpectedResponseContentType: core.HTTPMediaTypeJSON(),
		RequestContentLength: loopsByteLength(t, len(body)), ExpectedStatus: core.HTTPStatusOK(),
	}
}

func TestIdempotencyHeaderCrossesLoopsWallAtPublishedMaximum(t *testing.T) {
	t.Parallel()
	keyed := func(key string) exchange.RequestSemantics {
		return exchange.RequestSemantics{Replay: exchange.ReplaySingleAttemptWithIdempotencyKey, IdempotencyKey: loopsIdempotencyKey(t, key)}
	}
	single := exchange.RequestSemantics{Replay: exchange.ReplaySingleAttempt}
	body := []byte(`{"email":"person@example.com"}`)
	for _, tc := range []struct {
		wantErr   error
		name      string
		wantKey   string
		semantics exchange.RequestSemantics
		body      []byte
		route     Route
		wantCalls int
		method    exchange.Method
	}{
		{name: "event key at published maximum", route: Route{Operation: SendEvent}, method: exchange.MethodPost, semantics: keyed(strings.Repeat("e", core.LoopsIdempotencyKeyMaximumBytes)), body: body, wantKey: strings.Repeat("e", core.LoopsIdempotencyKeyMaximumBytes), wantCalls: 1},
		{name: "transactional key at published maximum", route: Route{Operation: SendTransactional}, method: exchange.MethodPost, semantics: keyed(strings.Repeat("t", core.LoopsIdempotencyKeyMaximumBytes)), body: body, wantKey: strings.Repeat("t", core.LoopsIdempotencyKeyMaximumBytes), wantCalls: 1},
		{name: "neutral event without key", route: Route{Operation: SendEvent}, method: exchange.MethodPost, semantics: single, body: body, wantCalls: 1},
		{name: "event key one above maximum", route: Route{Operation: SendEvent}, method: exchange.MethodPost, semantics: keyed(strings.Repeat("e", core.LoopsIdempotencyKeyMaximumBytes+1)), body: body, wantErr: core.ErrLoopsBinding},
		{name: "contact create has no documented key", route: Route{Operation: CreateContact}, method: exchange.MethodPost, semantics: keyed("k"), body: body, wantErr: core.ErrLoopsBinding},
		{name: "contact upsert has no documented key", route: Route{Operation: UpdateContact}, method: exchange.MethodPut, semantics: keyed("k"), body: body, wantErr: core.ErrLoopsBinding},
		{name: "suppression delete has no body", route: Route{Operation: RemoveSuppression, Contact: loopsContact(t, ContactIdentifierEmail, "a@example.com")}, method: exchange.MethodDelete, semantics: single, body: body, wantErr: core.ErrLoopsBinding},
		{name: "suppression delete at zero length", route: Route{Operation: RemoveSuppression, Contact: loopsContact(t, ContactIdentifierEmail, "a@example.com")}, method: exchange.MethodDelete, semantics: single, wantCalls: 1},
		{name: "unknown replay mode", route: Route{Operation: SendEvent}, method: exchange.MethodPost, semantics: exchange.RequestSemantics{Replay: exchange.ReplayMode(255)}, body: body, wantErr: core.ErrLoopsBinding},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls int
			var gotKey string
			native, err := exchange.NewClient(&http.Client{Transport: idempotencyRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				gotKey = request.Header.Get(core.HTTPHeaderIdempotencyKey().String())
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"success":true}`)), Request: request}, nil
			})})
			if err != nil {
				t.Fatal(err)
			}
			credential, err := ParseCredential([]byte("loops_test_key"))
			if err != nil {
				t.Fatal(err)
			}
			provider, err := NewClient(native, credential)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := errors.Join(provider.Close(), credential.Close()); err != nil {
					t.Errorf("cleanup error = %v, want nil", err)
				}
			}()
			_, gotErr := provider.RoundTrip(context.Background(), Request{Stream: loopsSendRequest(t, tc.route, tc.method, tc.body, tc.semantics)}, loopsStreamPolicy(t))
			if !errors.Is(gotErr, tc.wantErr) || calls != tc.wantCalls || gotKey != tc.wantKey {
				t.Fatalf("RoundTrip() = error:%v calls:%d key:%q, want %v, %d, %q", gotErr, calls, gotKey, tc.wantErr, tc.wantCalls, tc.wantKey)
			}
		})
	}
}

func TestProviderStatusIsReturnedWithoutSDKRetry(t *testing.T) {
	t.Parallel()
	for _, status := range []int{http.StatusTooManyRequests, http.StatusConflict} {
		client, calls := loopsNativeClient(t, "loops_native_key", func(loopsNativeRequest) {}, status)
		request := loopsSendRequest(t, Route{Operation: SendEvent}, exchange.MethodPost, []byte(`{}`), exchange.RequestSemantics{Replay: exchange.ReplaySingleAttemptWithIdempotencyKey, IdempotencyKey: loopsIdempotencyKey(t, "evt-1")})
		var want core.HTTPStatusCode
		if err := want.AdmitInt(status); err != nil {
			t.Fatal(err)
		}
		response, err := client.RoundTrip(t.Context(), Request{Stream: request}, loopsStreamPolicy(t))
		if err == nil || calls.Load() != 1 || response.Stream.Metadata.Attempts != 1 || response.Stream.Metadata.Status != want {
			t.Fatalf("status %d = (%+v, %v) calls %d, want one attempt with the exact provider status", status, response.Stream.Metadata, err, calls.Load())
		}
	}
}

func TestClientRefusesCallerAuthorityAndForeignShapes(t *testing.T) {
	t.Parallel()
	client, calls := loopsNativeClient(t, "loops_native_key", func(loopsNativeRequest) {}, http.StatusOK)
	authorization, err := exchange.NewBearerAuthorizationHeader(exchange.BearerAuthorization{Token: []byte("foreign")})
	if err != nil {
		t.Fatal(err)
	}
	base := func() exchange.StreamRoundTripRequest {
		return loopsSendRequest(t, Route{Operation: CreateContact}, exchange.MethodPost, []byte(`{}`), exchange.RequestSemantics{Replay: exchange.ReplaySingleAttempt})
	}
	foreignHeader, err := core.ParseHTTPHeaderName("X-Loops-Foreign")
	if err != nil {
		t.Fatal(err)
	}
	foreignHost, err := core.ParseHTTPEndpoint("https://next-api.useplunk.com/api/v1/contacts/create")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		mutate func(*exchange.StreamRoundTripRequest)
		name   string
	}{
		{name: "caller authorization header", mutate: func(r *exchange.StreamRoundTripRequest) {
			r.Headers = exchange.Headers{Values: []exchange.Header{authorization}}
		}},
		{name: "caller secret header", mutate: func(r *exchange.StreamRoundTripRequest) {
			r.SecretHeaders = []exchange.SecretHeader{{Name: foreignHeader, Value: []byte("secret"), Scheme: exchange.SecretHeaderLiteral}}
		}},
		{name: "form request media", mutate: func(r *exchange.StreamRoundTripRequest) { r.RequestContentType = core.HTTPMediaTypeFormURLEncoded() }},
		{name: "omitted response media", mutate: func(r *exchange.StreamRoundTripRequest) { r.ExpectedResponseContentType = core.HTTPMediaType{} }},
		{name: "foreign provider host", mutate: func(r *exchange.StreamRoundTripRequest) { r.Target = foreignHost }},
		{name: "neutral zero request", mutate: func(r *exchange.StreamRoundTripRequest) { *r = exchange.StreamRoundTripRequest{} }},
	} {
		request := base()
		tc.mutate(&request)
		response, err := client.RoundTrip(t.Context(), Request{Stream: request}, loopsStreamPolicy(t))
		if !errors.Is(err, core.ErrLoopsBinding) || calls.Load() != 0 || response.Stream.Metadata.Attempts != 0 {
			t.Fatalf("%s: RoundTrip() = %v calls %d, want binding refusal before transport", tc.name, err, calls.Load())
		}
	}
	var closed Client
	if validateErr, closeErr := closed.Validate(), closed.Close(); !errors.Is(validateErr, core.ErrLoopsContract) || !errors.Is(closeErr, core.ErrLoopsContract) {
		t.Fatalf("zero client Validate/Close = %v/%v, want %v", validateErr, closeErr, core.ErrLoopsContract)
	}
}
