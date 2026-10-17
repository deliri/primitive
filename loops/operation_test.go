package loops

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

type operationCase struct {
	name   string
	path   string
	query  string
	route  Route
	method exchange.Method
	replay exchange.ReplayMode
}

func loopsContact(t testing.TB, kind ContactIdentifierKind, value string) ContactIdentifier {
	t.Helper()
	contact, err := ParseContactIdentifier(kind, value)
	if err != nil {
		t.Fatalf("ParseContactIdentifier(%v, %q) error = %v, want nil", kind, value, err)
	}
	return contact
}

func loopsCursor(t testing.TB, value string) Cursor {
	t.Helper()
	cursor, err := ParseCursor(value)
	if err != nil {
		t.Fatalf("ParseCursor(%q) error = %v, want nil", value, err)
	}
	return cursor
}

// documentedOperations is the OpenAPI operation table. Every Operation arm
// appears at least once; query rows cover each typed parameter spelling.
func documentedOperations(t testing.TB) []operationCase {
	t.Helper()
	email := loopsContact(t, ContactIdentifierEmail, "person+tag@example.com")
	user := loopsContact(t, ContactIdentifierUserID, "usr 1/2")
	cursor := loopsCursor(t, "clx+cursor/=")
	single, keyed := exchange.ReplaySingleAttempt, exchange.ReplaySingleAttemptWithIdempotencyKey
	return []operationCase{
		{"test API key", "/api/v1/api-key", "", Route{Operation: TestAPIKey}, exchange.MethodGet, single},
		{"create contact", "/api/v1/contacts/create", "", Route{Operation: CreateContact}, exchange.MethodPost, single},
		{"update contact upsert", "/api/v1/contacts/update", "", Route{Operation: UpdateContact}, exchange.MethodPut, single},
		{"find contact by email", "/api/v1/contacts/find", "email=person%2Btag%40example.com", Route{Operation: FindContact, Contact: email}, exchange.MethodGet, single},
		{"find contact by user", "/api/v1/contacts/find", "userId=usr+1%2F2", Route{Operation: FindContact, Contact: user}, exchange.MethodGet, single},
		{"delete contact", "/api/v1/contacts/delete", "", Route{Operation: DeleteContact}, exchange.MethodPost, single},
		{"read suppression", "/api/v1/contacts/suppression", "email=person%2Btag%40example.com", Route{Operation: ReadSuppression, Contact: email}, exchange.MethodGet, single},
		{"remove suppression", "/api/v1/contacts/suppression", "userId=usr+1%2F2", Route{Operation: RemoveSuppression, Contact: user}, exchange.MethodDelete, single},
		{"list all properties", "/api/v1/contacts/properties", "list=all", Route{Operation: ListContactProperties, PropertyList: PropertyListAll}, exchange.MethodGet, single},
		{"list custom properties", "/api/v1/contacts/properties", "list=custom", Route{Operation: ListContactProperties, PropertyList: PropertyListCustom}, exchange.MethodGet, single},
		{"create property", "/api/v1/contacts/properties", "", Route{Operation: CreateContactProperty}, exchange.MethodPost, single},
		{"list mailing lists", "/api/v1/lists", "", Route{Operation: ListMailingLists}, exchange.MethodGet, single},
		{"send event", "/api/v1/events/send", "", Route{Operation: SendEvent}, exchange.MethodPost, keyed},
		{"send transactional", "/api/v1/transactional", "", Route{Operation: SendTransactional}, exchange.MethodPost, keyed},
		{"list transactional default page", "/api/v1/transactional-emails", "", Route{Operation: ListTransactionalEmails}, exchange.MethodGet, single},
		{"list transactional minimum page", "/api/v1/transactional-emails", "perPage=10", Route{Operation: ListTransactionalEmails, Page: Page{Size: core.LoopsPageSizeMinimum}}, exchange.MethodGet, single},
		{"list transactional cursor page", "/api/v1/transactional-emails", "perPage=50&cursor=clx%2Bcursor%2F%3D", Route{Operation: ListTransactionalEmails, Page: Page{Size: core.LoopsPageSizeMaximum, Cursor: cursor}}, exchange.MethodGet, single},
		{"list transactional cursor only", "/api/v1/transactional-emails", "cursor=clx%2Bcursor%2F%3D", Route{Operation: ListTransactionalEmails, Page: Page{Cursor: cursor}}, exchange.MethodGet, single},
	}
}

func TestDocumentedOperationRouteLayerTriad(t *testing.T) {
	t.Parallel()
	covered := [operationLimit]bool{}
	for _, tc := range documentedOperations(t) {
		covered[tc.route.Operation] = true
		t.Run("positive "+tc.name, func(t *testing.T) {
			t.Parallel()
			target, err := tc.route.Endpoint()
			method, methodErr := tc.route.Method()
			if err != nil || methodErr != nil {
				t.Fatalf("route errors = %v, %v, want nil", err, methodErr)
			}
			got := target.HTTPURL()
			if got.Scheme != core.SchemeHTTPS || got.Host != core.LoopsAPIHost || got.Path != tc.path || got.RawQuery != tc.query || method != tc.method {
				t.Fatalf("route = %s %s, want %s https://%s%s?%s", method, target.String(), tc.method, core.LoopsAPIHost, tc.path, tc.query)
			}
			recovered, err := routeFromTarget(target, method)
			if err != nil || recovered != tc.route {
				t.Fatalf("routeFromTarget() = (%+v, %v), want exact typed route %+v", recovered, err, tc.route)
			}
			if tc.route.Operation.admitsIdempotencyKey() != (tc.replay == exchange.ReplaySingleAttemptWithIdempotencyKey) {
				t.Fatalf("idempotency admission = %t, want replay %v", tc.route.Operation.admitsIdempotencyKey(), tc.replay)
			}
		})
	}
	for operation := TestAPIKey; operation < operationLimit; operation++ {
		if !covered[operation] {
			t.Fatalf("operation %d has no documented route row", operation)
		}
	}
	email := loopsContact(t, ContactIdentifierEmail, "a@example.com")
	for _, tc := range []struct {
		name  string
		route Route
	}{
		{name: "neutral zero route", route: Route{}},
		{name: "future operation", route: Route{Operation: operationLimit}},
		{name: "missing contact", route: Route{Operation: FindContact}},
		{name: "forged contact without kind", route: Route{Operation: ReadSuppression, Contact: ContactIdentifier{value: "a"}}},
		{name: "foreign contact on body door", route: Route{Operation: SendEvent, Contact: email}},
		{name: "foreign page on contact door", route: Route{Operation: FindContact, Contact: email, Page: Page{Size: core.LoopsPageSizeMinimum}}},
		{name: "missing property filter", route: Route{Operation: ListContactProperties}},
		{name: "unknown property filter", route: Route{Operation: ListContactProperties, PropertyList: propertyListLimit}},
		{name: "foreign property filter", route: Route{Operation: ListMailingLists, PropertyList: PropertyListAll}},
		{name: "page below window", route: Route{Operation: ListTransactionalEmails, Page: Page{Size: core.LoopsPageSizeMinimum - 1}}},
		{name: "page above window", route: Route{Operation: ListTransactionalEmails, Page: Page{Size: core.LoopsPageSizeMaximum + 1}}},
		{name: "forged cursor", route: Route{Operation: ListTransactionalEmails, Page: Page{Cursor: Cursor{value: "a b"}}}},
	} {
		t.Run("negative "+tc.name, func(t *testing.T) {
			t.Parallel()
			target, err := tc.route.Endpoint()
			method, methodErr := tc.route.Method()
			if !errors.Is(err, core.ErrLoopsBinding) || !errors.Is(methodErr, core.ErrLoopsBinding) || target != (core.HTTPEndpoint{}) || method != exchange.Method(0) {
				t.Fatalf("rejected route = (%v, %v, %v, %v), want zero endpoint/method and binding refusal", target, method, err, methodErr)
			}
		})
	}
}

func TestRouteFromTargetRefusesNonCanonicalSpellingsLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		target string
		method exchange.Method
		want   Operation
	}{
		{name: "canonical suppression read", target: "https://app.loops.so/api/v1/contacts/suppression?email=a%40example.com", method: exchange.MethodGet, want: ReadSuppression},
		{name: "same path selects delete by method", target: "https://app.loops.so/api/v1/contacts/suppression?email=a%40example.com", method: exchange.MethodDelete, want: RemoveSuppression},
		{name: "same path selects property creation by method", target: "https://app.loops.so/api/v1/contacts/properties", method: exchange.MethodPost, want: CreateContactProperty},
		{name: "unescaped at sign is not canonical", target: "https://app.loops.so/api/v1/contacts/find?email=a@example.com", method: exchange.MethodGet},
		{name: "percent space is not canonical plus", target: "https://app.loops.so/api/v1/contacts/find?userId=a%20b", method: exchange.MethodGet},
		{name: "both identities", target: "https://app.loops.so/api/v1/contacts/find?email=a%40example.com&userId=u", method: exchange.MethodGet},
		{name: "duplicate identity", target: "https://app.loops.so/api/v1/contacts/find?email=a&email=b", method: exchange.MethodGet},
		{name: "empty identity", target: "https://app.loops.so/api/v1/contacts/find?email=", method: exchange.MethodGet},
		{name: "missing identity", target: "https://app.loops.so/api/v1/contacts/find", method: exchange.MethodGet},
		{name: "reordered page fields", target: "https://app.loops.so/api/v1/transactional-emails?cursor=c&perPage=10", method: exchange.MethodGet},
		{name: "leading zero page size", target: "https://app.loops.so/api/v1/transactional-emails?perPage=010", method: exchange.MethodGet},
		{name: "page below window", target: "https://app.loops.so/api/v1/transactional-emails?perPage=9", method: exchange.MethodGet},
		{name: "page above window", target: "https://app.loops.so/api/v1/transactional-emails?perPage=51", method: exchange.MethodGet},
		{name: "unknown property filter", target: "https://app.loops.so/api/v1/contacts/properties?list=hidden", method: exchange.MethodGet},
		{name: "query on body door", target: "https://app.loops.so/api/v1/events/send?email=a", method: exchange.MethodPost},
		{name: "trailing separator", target: "https://app.loops.so/api/v1/contacts/properties?list=all&", method: exchange.MethodGet},
		{name: "bare query marker", target: "https://app.loops.so/api/v1/lists?", method: exchange.MethodGet},
		{name: "foreign host", target: "https://loops.so/api/v1/lists", method: exchange.MethodGet},
		{name: "explicit port", target: "https://app.loops.so:443/api/v1/lists", method: exchange.MethodGet},
		{name: "plain HTTP", target: "http://app.loops.so/api/v1/lists", method: exchange.MethodGet},
		{name: "missing api prefix", target: "https://app.loops.so/v1/lists", method: exchange.MethodGet},
		{name: "legacy transactional listing is not admitted", target: "https://app.loops.so/api/v1/transactional", method: exchange.MethodGet},
		{name: "escaped path delimiter", target: "https://app.loops.so/api/v1/contacts%2Ffind?email=a", method: exchange.MethodGet},
		{name: "dot segment", target: "https://app.loops.so/api/v1/contacts/../lists", method: exchange.MethodGet},
		{name: "wrong method for door", target: "https://app.loops.so/api/v1/contacts/update", method: exchange.MethodPatch},
		{name: "neutral unknown resource", target: "https://app.loops.so/api/v1/campaigns", method: exchange.MethodGet},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			target, err := core.ParseHTTPEndpoint(tc.target)
			if err != nil {
				t.Fatalf("core.ParseHTTPEndpoint(%q) error = %v, want typed endpoint", tc.target, err)
			}
			got, err := routeFromTarget(target, tc.method)
			if tc.want == OperationInvalid {
				if !errors.Is(err, core.ErrLoopsBinding) || got != (Route{}) {
					t.Fatalf("routeFromTarget() = (%+v, %v), want zero route and binding refusal", got, err)
				}
				return
			}
			if err != nil || got.Operation != tc.want {
				t.Fatalf("routeFromTarget() = (%+v, %v), want %v", got, err, tc.want)
			}
		})
	}
	if got, err := routeFromTarget(nil, exchange.MethodGet); !errors.Is(err, core.ErrLoopsBinding) || got != (Route{}) {
		t.Fatalf("routeFromTarget(nil) = (%+v, %v), want binding refusal", got, err)
	}
}

func FuzzRouteSemanticClosure(f *testing.F) {
	for _, tc := range documentedOperations(f) {
		page := tc.route.Page
		f.Add(uint8(tc.route.Operation), uint8(tc.route.Contact.Kind()), tc.route.Contact.String(), uint8(tc.route.PropertyList), uint8(page.Size), page.Cursor.String())
	}
	f.Add(uint8(FindContact), uint8(ContactIdentifierEmail), "a&email=b", uint8(0), uint8(0), "")
	f.Add(uint8(ListTransactionalEmails), uint8(0), "", uint8(0), uint8(9), "c#d")
	f.Fuzz(func(t *testing.T, op, kind uint8, contact string, list, size uint8, cursorText string) {
		route := Route{Operation: Operation(op), PropertyList: PropertyListFilter(list), Page: Page{Size: PageSize(size)}}
		if identity, err := ParseContactIdentifier(ContactIdentifierKind(kind), contact); err == nil {
			route.Contact = identity
		}
		if cursor, err := ParseCursor(cursorText); err == nil {
			route.Page.Cursor = cursor
		}
		endpoint, err := route.Endpoint()
		method, methodErr := route.Method()
		if err != nil {
			if !errors.Is(err, core.ErrLoopsBinding) || !errors.Is(methodErr, core.ErrLoopsBinding) || endpoint != (core.HTTPEndpoint{}) || route.Validate() == nil {
				t.Fatalf("refusal = %v/%v/%v, want zero binding refusal of an invalid route", endpoint, err, methodErr)
			}
			return
		}
		u := endpoint.HTTPURL()
		if route.Validate() != nil || methodErr != nil || u.Host != core.LoopsAPIHost || u.Scheme != core.SchemeHTTPS || u.User != nil || u.Fragment != "" {
			t.Fatalf("admitted endpoint = %s, want one provider-bound operation", endpoint.String())
		}
		recovered, err := routeFromTarget(endpoint, method)
		if err != nil || recovered != route {
			t.Fatalf("routeFromTarget(%s) = (%+v, %v), want exact round trip %+v", endpoint.String(), recovered, err, route)
		}
		again, err := recovered.Endpoint()
		if err != nil || again.String() != endpoint.String() {
			t.Fatalf("second projection = %s/%v, want byte-identical %s", again.String(), err, endpoint.String())
		}
	})
}

func FuzzRouteFromTargetSemanticClosure(f *testing.F) {
	for _, tc := range documentedOperations(f) {
		target, err := tc.route.Endpoint()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(target.String(), uint8(tc.method))
	}
	f.Add("https://app.loops.so/api/v1/contacts/find?email=a@b", uint8(exchange.MethodGet))
	f.Add("https://app.loops.so/api/v1/transactional-emails?cursor=a&perPage=10", uint8(exchange.MethodGet))
	f.Fuzz(func(t *testing.T, text string, methodValue uint8) {
		target, err := core.ParseHTTPEndpoint(text)
		if err != nil {
			return // Core owns URL admission; its own fuzz target pins that door.
		}
		method := exchange.Method(methodValue)
		route, err := routeFromTarget(target, method)
		if err != nil {
			if !errors.Is(err, core.ErrLoopsBinding) || route != (Route{}) {
				t.Fatalf("refusal = (%+v, %v), want zero binding refusal", route, err)
			}
			return
		}
		canonical, err := route.Endpoint()
		routeMethod, methodErr := route.Method()
		if err != nil || methodErr != nil || canonical.String() != target.String() || routeMethod != method {
			t.Fatalf("admitted %s %s, want exact canonical %s %s", method, target.String(), routeMethod, canonical.String())
		}
	})
}

type loopsNativeRequest struct {
	method, path, query, authorization, idempotency string
	body                                            []byte
}

// loopsNativeClient dials a local TLS peer for app.loops.so through real
// Exchange, net/http and crypto/tls; the peer records each observed request.
func loopsNativeClient(t testing.TB, credentialText string, observe func(loopsNativeRequest), status int) (Client, *atomic.Int64) {
	t.Helper()
	var calls atomic.Int64
	peer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("native request body read error = %v, want nil", err)
		}
		observe(loopsNativeRequest{method: r.Method, path: r.URL.EscapedPath(), query: r.URL.RawQuery, authorization: r.Header.Get("Authorization"), idempotency: r.Header.Get("Idempotency-Key"), body: body})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, `{"success":true}`)
	}))
	t.Cleanup(peer.Close)
	transport := peer.Client().Transport.(*http.Transport).Clone()
	transport.TLSClientConfig.ServerName = "example.com"
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, peer.Listener.Addr().String())
	}
	t.Cleanup(transport.CloseIdleConnections)
	native, err := exchange.NewClient(&http.Client{Transport: transport})
	if err != nil {
		t.Fatal(err)
	}
	credential, err := ParseCredential([]byte(credentialText))
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(native, credential)
	if err != nil {
		t.Fatal(err)
	}
	if err := credential.Close(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("client.Close() = %v, want nil", err)
		}
	})
	return client, &calls
}

func loopsStreamPolicy(t testing.TB) exchange.StreamPolicy {
	t.Helper()
	operation, err := temporal.DurationFromSeconds(5)
	if err != nil {
		t.Fatalf("temporal.DurationFromSeconds(operation) error = %v, want nil", err)
	}
	attempt, err := temporal.DurationFromSeconds(4)
	if err != nil {
		t.Fatalf("temporal.DurationFromSeconds(attempt) error = %v, want nil", err)
	}
	return exchange.StreamPolicy{OperationTimeout: operation, AttemptTimeout: attempt, Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject}}
}

func loopsByteLength(t testing.TB, count int) *core.ByteLength {
	t.Helper()
	length, err := core.NewByteLength(uint64(count))
	if err != nil {
		t.Fatal(err)
	}
	return &length
}

func loopsIdempotencyKey(t testing.TB, value string) exchange.IdempotencyKey {
	t.Helper()
	key, err := exchange.ParseIdempotencyKey(value)
	if err != nil {
		t.Fatalf("exchange.ParseIdempotencyKey() error = %v, want nil", err)
	}
	return key
}

// loopsOperationCall issues one documented operation through its owning door.
func loopsOperationCall(t testing.TB, client Client, tc operationCase, key string, destination io.Writer) error {
	t.Helper()
	target, err := tc.route.Endpoint()
	if err != nil {
		t.Fatal(err)
	}
	if tc.method == exchange.MethodGet {
		_, err := client.Download(context.Background(), DownloadRequest{Stream: exchange.DownloadRequest{
			Target: target, Destination: destination, Semantics: exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt},
			ExpectedResponseContentType: core.HTTPMediaTypeJSON(), ExpectedStatus: core.HTTPStatusOK(),
		}}, loopsStreamPolicy(t))
		return err
	}
	body := []byte(`{"email":"person@example.com"}`)
	if tc.method == exchange.MethodDelete {
		body = nil
	}
	semantics := exchange.RequestSemantics{Method: tc.method, Replay: exchange.ReplaySingleAttempt}
	if key != "" {
		semantics = exchange.RequestSemantics{Method: tc.method, Replay: exchange.ReplaySingleAttemptWithIdempotencyKey, IdempotencyKey: loopsIdempotencyKey(t, key)}
	}
	_, err = client.RoundTrip(context.Background(), Request{Stream: exchange.StreamRoundTripRequest{
		Target: target, Source: bytes.NewReader(body), Destination: destination, RequestContentLength: loopsByteLength(t, len(body)),
		RequestContentType: core.HTTPMediaTypeJSON(), ExpectedResponseContentType: core.HTTPMediaTypeJSON(), ExpectedStatus: core.HTTPStatusOK(), Semantics: semantics,
	}}, loopsStreamPolicy(t))
	return err
}

// Real SDK -> Exchange -> net/http -> TLS peer for every documented operation.
func TestEveryOperationCrossesNativeProviderDoorLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range documentedOperations(t) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			key := ""
			if tc.replay == exchange.ReplaySingleAttemptWithIdempotencyKey {
				key = "evt-" + strings.Repeat("k", core.LoopsIdempotencyKeyMaximumBytes-4)
			}
			var observed loopsNativeRequest
			client, calls := loopsNativeClient(t, "loops_native_key", func(request loopsNativeRequest) { observed = request }, http.StatusOK)
			var destination bytes.Buffer
			if err := loopsOperationCall(t, client, tc, key, &destination); err != nil {
				t.Fatalf("operation call error = %v, want nil", err)
			}
			wantBody := tc.method == exchange.MethodPost || tc.method == exchange.MethodPut
			if calls.Load() != 1 || observed.method != tc.method.String() || observed.path != tc.path || observed.query != tc.query ||
				observed.authorization != "Bearer loops_native_key" || observed.idempotency != key || (len(observed.body) != 0) != wantBody {
				t.Fatalf("native request = %+v calls %d, want %s %s?%s with owned bearer, key %q, body %t", observed, calls.Load(), tc.method, tc.path, tc.query, key, wantBody)
			}
			if destination.String() != `{"success":true}` {
				t.Fatalf("response stream = %q, want exact provider JSON", destination.String())
			}
		})
	}
}

func TestOperationDoorsRefuseCrossedMethodsBeforeTransport(t *testing.T) {
	t.Parallel()
	for _, tc := range documentedOperations(t) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, calls := loopsNativeClient(t, "loops_native_key", func(loopsNativeRequest) {}, http.StatusOK)
			crossed := tc
			switch tc.method {
			case exchange.MethodGet:
				crossed.method = exchange.MethodPost
			case exchange.MethodDelete:
				// GET on the suppression path is a different documented door.
				crossed.method = exchange.MethodPut
			default:
				crossed.method = exchange.MethodGet
			}
			err := loopsOperationCall(t, client, crossed, "", io.Discard)
			if !errors.Is(err, core.ErrLoopsBinding) || calls.Load() != 0 {
				t.Fatalf("crossed method call = %v calls %d, want binding refusal and no request", err, calls.Load())
			}
		})
	}
}
