package controlwire_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlplanetest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type socketFixture struct {
	request           controlplane.RegistrationRequest
	response          controlplane.ResponseProjection[controlplane.RegistrationDocument]
	responseCanonical []byte
	support           controlwire.ProtocolSupport
}

type socketObservation struct {
	body       controlplane.RegistrationRequest
	assessment controlwire.ProtocolAssessment
	method     string
	path       string
	key        string
	err        error
}

func TestRoutedSocketExecutesProductionRequestAndAuthenticatedResponse(t *testing.T) {
	t.Parallel()

	fixture := productionSocketFixture(t)
	route, err := fixture.request.ControlRoute()
	if err != nil {
		t.Fatalf("RegistrationRequest.ControlRoute() error = %v, want nil", err)
	}
	observed := make(chan socketObservation, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		received, receiveErr := controlwire.ReceiveRoutedJSON[
			controlplane.RegistrationRequest,
			*controlplane.RegistrationRequest,
		](controlwire.AuthorityJSONReceiveCall{Request: request, Route: route, Support: fixture.support})
		observation := socketObservation{
			method: request.Method,
			path:   request.URL.Path,
			key:    request.Header.Get(core.HTTPHeaderIdempotencyKey().String()),
			err:    receiveErr,
		}
		if receiveErr == nil {
			observation.body = *received.Body
			observation.assessment = received.Assessment
		}
		observed <- observation
		if receiveErr != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		if writeErr := controlwire.WriteControlJSON(controlwire.ControlJSONWriteCall[controlplane.ResponseProjection[controlplane.RegistrationDocument]]{Writer: writer, Body: fixture.response}); writeErr != nil {
			t.Errorf("WriteControlJSON() error = %v, want nil", writeErr)
		}
	}))
	defer server.Close()

	authority, err := core.ParseHTTPEndpoint(server.URL)
	if err != nil {
		t.Fatalf("ParseHTTPEndpoint(server) error = %v, want nil", err)
	}
	client, err := exchange.NewClient(server.Client())
	if err != nil {
		t.Fatalf("exchange.NewClient() error = %v, want nil", err)
	}
	response, err := controlwire.SendRoutedJSON[
		controlplane.RegistrationRequest,
		controlplane.ResponseDocument[controlplane.RegistrationDocument, *controlplane.RegistrationDocument],
		*controlplane.ResponseDocument[controlplane.RegistrationDocument, *controlplane.RegistrationDocument],
	](controlwire.ClientJSONCall[controlplane.RegistrationRequest]{
		Context: context.Background(), Client: client,
		Authority: authority, Body: fixture.request,
	})
	if err != nil {
		t.Fatalf("SendRoutedJSON() error = %v, want nil", err)
	}
	if response.Body == nil || response.Body.Validate() != nil {
		t.Fatalf("response body = %+v, want authenticated production response", response.Body)
	}
	if response.Metadata.Bytes.Uint64() != uint64(len(fixture.responseCanonical)) ||
		response.Metadata.Attempts != 1 || response.Metadata.Status != core.HTTPStatusOK() {
		t.Fatalf("response metadata = %+v, want %d bytes, one attempt, and HTTP 200", response.Metadata, len(fixture.responseCanonical))
	}
	got := <-observed
	wantPath, err := route.Path()
	if err != nil {
		t.Fatalf("RouteContract.Path() error = %v, want nil", err)
	}
	gotRequestJSON, gotMarshalErr := got.body.MarshalJSON()
	wantRequestJSON, wantMarshalErr := fixture.request.MarshalJSON()
	if got.err != nil || gotMarshalErr != nil || wantMarshalErr != nil ||
		!bytes.Equal(gotRequestJSON, wantRequestJSON) || got.method != http.MethodPost ||
		got.path != wantPath || got.key != fixture.request.RequestNonce.String() {
		t.Fatalf("authority observation = %+v, want exact production request/method/path/nonce", got)
	}
	capability, err := route.ProtocolCapability(fixture.request.ControlRevision())
	if err != nil || got.assessment.Capability != capability ||
		got.assessment.Outcome != controlwire.ProtocolSupportOutcomeAccepted {
		t.Fatalf("authority protocol assessment = (%+v, %v), want exact accepted capability %+v", got.assessment, err, capability)
	}
}

// TestRegistrationAuthorityRunsThroughTheRealControlWireReceiver proves the
// transactional verifier is wired to the authority HTTP boundary rather than
// existing only as an isolated pure helper.
func TestRegistrationAuthorityRunsThroughTheRealControlWireReceiver(t *testing.T) {
	t.Parallel()

	fixture := productionSocketFixture(t)
	canonical, err := fixture.request.MarshalJSON()
	if err != nil {
		t.Fatalf("RegistrationRequest.MarshalJSON() error = %v, want nil", err)
	}
	route, err := fixture.request.ControlRoute()
	if err != nil {
		t.Fatalf("RegistrationRequest.ControlRoute() error = %v, want nil", err)
	}
	path, err := route.Path()
	if err != nil {
		t.Fatalf("RouteContract.Path() error = %v, want nil", err)
	}
	key, err := fixture.request.RequestNonce.IdempotencyKey()
	if err != nil {
		t.Fatalf("RequestNonce.IdempotencyKey() error = %v, want nil", err)
	}
	httpRequest := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(canonical))
	httpRequest.Header.Set(core.HTTPHeaderContentType().String(), standardMediaType(t, exchange.StandardMediaTypeJSON).String())
	httpRequest.Header.Set(core.HTTPHeaderIdempotencyKey().String(), key.String())
	received, err := controlwire.ReceiveRoutedJSON[
		controlplane.RegistrationRequest,
		*controlplane.RegistrationRequest,
	](controlwire.AuthorityJSONReceiveCall{
		Request: httpRequest, Route: route, Support: fixture.support,
	})
	if err != nil {
		t.Fatalf("ReceiveRoutedJSON() error = %v, want nil", err)
	}
	verifier, err := received.Body.Token.Verifier()
	if err != nil {
		t.Fatalf("received RegistrationToken.Verifier() error = %v, want nil", err)
	}
	verified, err := controlplane.VerifyRegistrationAuthority(controlplane.RegistrationAuthorityVerification{
		Request: *received.Body, ExpectedVerifier: verifier,
	})
	if err != nil {
		t.Fatalf("VerifyRegistrationAuthority(received) error = %v, want nil", err)
	}
	replay, disposition, err := verified.Replay()
	if err != nil || disposition != controlwire.ReplayDispositionFresh || !replay.Equal(received.Replay) {
		t.Fatalf("authority replay = (%v, %v, %v), want received replay, %v, nil", replay, disposition, err, controlwire.ReplayDispositionFresh)
	}
}

func TestRoutedSocketReturnsUpgradeAssessmentBesideAnUnsupportedValidatedRequest(t *testing.T) {
	t.Parallel()

	fixture := productionSocketFixture(t)
	route, err := fixture.request.ControlRoute()
	if err != nil {
		t.Fatalf("RegistrationRequest.ControlRoute() error = %v, want nil", err)
	}
	encoded, err := fixture.request.MarshalJSON()
	if err != nil {
		t.Fatalf("RegistrationRequest.MarshalJSON() error = %v, want nil", err)
	}
	path, err := route.Path()
	if err != nil {
		t.Fatalf("RouteContract.Path() error = %v, want nil", err)
	}
	mediaType := standardMediaType(t, exchange.StandardMediaTypeJSON)
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(encoded))
	request.Header.Set(core.HTTPHeaderContentType().String(), mediaType.String())
	request.Header.Set(core.HTTPHeaderIdempotencyKey().String(), fixture.request.RequestNonce.String())
	support, err := controlwire.NewProtocolSupport(controlwire.ProtocolSupportRequest{
		Capabilities: []controlwire.ProtocolCapability{{
			Revision: controlwire.Revision2026V1, Family: controlwire.RouteFamilyCheckIns,
		}},
	})
	if err != nil {
		t.Fatalf("NewProtocolSupport(non-registration pair) error = %v, want nil", err)
	}
	received, err := controlwire.ReceiveRoutedJSON[
		controlplane.RegistrationRequest,
		*controlplane.RegistrationRequest,
	](controlwire.AuthorityJSONReceiveCall{Request: request, Route: route, Support: support})
	capability, capabilityErr := route.ProtocolCapability(fixture.request.ControlRevision())
	if err != nil || capabilityErr != nil || received.Validate() != nil || received.Body == nil ||
		received.Assessment.Capability != capability ||
		received.Assessment.Outcome != controlwire.ProtocolSupportOutcomeUpgradeRequired {
		t.Fatalf("ReceiveRoutedJSON(unsupported exact pair) = (%+v, %v/%v), want validated request beside exact upgrade-required assessment %+v", received, err, capabilityErr, capability)
	}
}

type clientResponseMode uint8

const (
	clientResponseAuthentic clientResponseMode = iota
	clientResponseWrongStatus
	clientResponseMalformedJSON
	clientResponseAbsentBody
	clientResponseWrongMediaType
	clientResponseRedirect
)

func TestRoutedSocketClientRejectsEveryIndependentTransportBoundary(t *testing.T) {
	t.Parallel()

	fixture := productionSocketFixture(t)
	jsonMediaType := standardMediaType(t, exchange.StandardMediaTypeJSON)
	plainMediaType := standardMediaType(t, exchange.StandardMediaTypePlainText)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	cases := []struct {
		name   string
		mode   clientResponseMode
		mutate func(*controlwire.ClientJSONCall[controlplane.RegistrationRequest])
		want   []error
	}{
		{name: "nil context is rejected before transport", mutate: func(call *controlwire.ClientJSONCall[controlplane.RegistrationRequest]) { call.Context = nil }, want: []error{core.ErrExchangeContract}},
		{name: "cancelled context retains cancellation identity", mutate: func(call *controlwire.ClientJSONCall[controlplane.RegistrationRequest]) { call.Context = cancelled }, want: []error{core.ErrExchangeRequest, core.ErrExchangeCancelled}},
		{name: "zero exchange client is rejected before transport", mutate: func(call *controlwire.ClientJSONCall[controlplane.RegistrationRequest]) {
			call.Client = exchange.Client{}
		}, want: []error{core.ErrExchangeContract}},
		{name: "zero authority is rejected before transport", mutate: func(call *controlwire.ClientJSONCall[controlplane.RegistrationRequest]) {
			call.Authority = core.HTTPEndpoint{}
		}, want: []error{core.ErrControlWireRoute}},
		{name: "authority carrying a path cannot steal route ownership", mutate: func(call *controlwire.ClientJSONCall[controlplane.RegistrationRequest]) {
			call.Authority = authorityWithSuffix(t, call.Authority, "/base")
		}, want: []error{core.ErrControlWireRoute, core.ErrExchangeContract}},
		{name: "authority carrying a query cannot alter route ownership", mutate: func(call *controlwire.ClientJSONCall[controlplane.RegistrationRequest]) {
			call.Authority = authorityWithSuffix(t, call.Authority, "?page=1")
		}, want: []error{core.ErrControlWireRoute, core.ErrExchangeContract}},
		{name: "zero production request is rejected before transport", mutate: func(call *controlwire.ClientJSONCall[controlplane.RegistrationRequest]) {
			call.Body = controlplane.RegistrationRequest{}
		}, want: []error{core.ErrControlPlaneRegistration}},
		{name: "unexpected success status is rejected", mode: clientResponseWrongStatus, want: []error{core.ErrExchangeResponse}},
		{name: "malformed authenticated response is rejected", mode: clientResponseMalformedJSON, want: []error{core.ErrExchangeResponse, core.ErrJSONContract}},
		{name: "absent authenticated response is rejected", mode: clientResponseAbsentBody, want: []error{core.ErrExchangeResponse, core.ErrJSONContract}},
		{name: "wrong response media type is rejected", mode: clientResponseWrongMediaType, want: []error{core.ErrExchangeResponse, core.ErrExchangeContentType}},
		{name: "redirect cannot move a control request", mode: clientResponseRedirect, want: []error{core.ErrExchangeRedirect, core.ErrExchangeContract}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writeClientResponse(writer, request, clientResponse{
					mode: tc.mode, body: fixture.responseCanonical,
					jsonMediaType: jsonMediaType, plainMediaType: plainMediaType,
				})
			}))
			defer server.Close()
			authority, endpointErr := core.ParseHTTPEndpoint(server.URL)
			if endpointErr != nil {
				t.Fatalf("ParseHTTPEndpoint(server) error = %v, want nil", endpointErr)
			}
			client, clientErr := exchange.NewClient(server.Client())
			if clientErr != nil {
				t.Fatalf("exchange.NewClient() error = %v, want nil", clientErr)
			}
			call := controlwire.ClientJSONCall[controlplane.RegistrationRequest]{
				Context: context.Background(), Client: client,
				Authority: authority, Body: fixture.request,
			}
			if tc.mutate != nil {
				tc.mutate(&call)
			}
			got, sendErr := controlwire.SendRoutedJSON[
				controlplane.RegistrationRequest,
				controlplane.ResponseDocument[controlplane.RegistrationDocument, *controlplane.RegistrationDocument],
				*controlplane.ResponseDocument[controlplane.RegistrationDocument, *controlplane.RegistrationDocument],
			](call)
			if got.Body != nil {
				t.Fatalf("rejected response body = %+v, want nil", got.Body)
			}
			for _, want := range tc.want {
				if !errors.Is(sendErr, want) {
					t.Fatalf("SendRoutedJSON() error = %v, want errors.Is %v", sendErr, want)
				}
			}
		})
	}
}

type clientResponse struct {
	mode           clientResponseMode
	body           []byte
	jsonMediaType  core.HTTPMediaType
	plainMediaType core.HTTPMediaType
}

func TestRoutedSocketAuthorityAcceptsTenProductionRequestRepresentations(t *testing.T) {
	t.Parallel()

	fixture := productionSocketFixture(t)
	route, err := fixture.request.ControlRoute()
	if err != nil {
		t.Fatalf("RegistrationRequest.ControlRoute() error = %v, want nil", err)
	}
	path, err := route.Path()
	if err != nil {
		t.Fatalf("RouteContract.Path() error = %v, want nil", err)
	}
	mediaType := standardMediaType(t, exchange.StandardMediaTypeJSON)
	wantKey, err := fixture.request.RequestNonce.IdempotencyKey()
	if err != nil {
		t.Fatalf("RequestNonce.IdempotencyKey() error = %v, want nil", err)
	}
	cases := validSocketRequestBodies(t, fixture.request)
	if len(cases) != 10 {
		t.Fatalf("authority valid representation inventory = %d, want exactly 10", len(cases))
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(tc.body))
			request.Header.Set(core.HTTPHeaderContentType().String(), mediaType.String())
			request.Header.Set(core.HTTPHeaderIdempotencyKey().String(), wantKey.String())
			got, gotErr := controlwire.ReceiveRoutedJSON[
				controlplane.RegistrationRequest,
				*controlplane.RegistrationRequest,
			](controlwire.AuthorityJSONReceiveCall{Request: request, Route: route, Support: fixture.support})
			if gotErr != nil || got.Body == nil || got.Body.Validate() != nil ||
				got.IdempotencyKey.String() != wantKey.String() ||
				got.Replay.Validate() != nil ||
				got.Assessment.Outcome != controlwire.ProtocolSupportOutcomeAccepted ||
				got.Assessment.Capability.Revision != fixture.request.ControlRevision() ||
				got.Assessment.Capability.Family != route.Family() {
				t.Fatalf("ReceiveRoutedJSON(%s) = (%+v, %v), want exact validated body and idempotency %q", tc.name, got, gotErr, wantKey.String())
			}
			canonical, marshalErr := got.Body.MarshalJSON()
			wantCanonical, wantErr := fixture.request.MarshalJSON()
			if marshalErr != nil || wantErr != nil || !bytes.Equal(canonical, wantCanonical) {
				t.Fatalf("received canonical request = (%d bytes, %v), want exact %d bytes and nil", len(canonical), marshalErr, len(wantCanonical))
			}
		})
	}
}

type validSocketRequestBody struct {
	name string
	body []byte
}

func validSocketRequestBodies(t testing.TB, request controlplane.RegistrationRequest) []validSocketRequestBody {
	t.Helper()

	canonical, err := request.MarshalJSON()
	if err != nil {
		t.Fatalf("RegistrationRequest.MarshalJSON() error = %v, want nil", err)
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, canonical, "", "  "); err != nil {
		t.Fatalf("json.Indent(compiler-produced request) error = %v, want nil", err)
	}
	return []validSocketRequestBody{
		{name: "canonical producer bytes", body: canonical},
		{name: "one leading space", body: append([]byte{' '}, canonical...)},
		{name: "one trailing newline", body: append(bytes.Clone(canonical), '\n')},
		{name: "mixed outer whitespace", body: append(append([]byte{'\t', '\r'}, canonical...), '\n', ' ')},
		{name: "indented typed document", body: indented.Bytes()},
		{name: "one below request ceiling", body: padSocketRequest(canonical, controlplane.RegistrationRequestJSONMaximumBytes-1)},
		{name: "exact request ceiling", body: padSocketRequest(canonical, controlplane.RegistrationRequestJSONMaximumBytes)},
		{name: "half request ceiling", body: padSocketRequest(canonical, controlplane.RegistrationRequestJSONMaximumBytes/2)},
		{name: "quarter request ceiling", body: padSocketRequest(canonical, controlplane.RegistrationRequestJSONMaximumBytes/4)},
		{name: "three-quarter request ceiling", body: padSocketRequest(canonical, 3*controlplane.RegistrationRequestJSONMaximumBytes/4)},
	}
}

func padSocketRequest(canonical []byte, size int) []byte {
	padded := make([]byte, size)
	copy(padded[size-len(canonical):], canonical)
	for index := range size - len(canonical) {
		padded[index] = ' '
	}
	return padded
}

func writeClientResponse(writer http.ResponseWriter, request *http.Request, response clientResponse) {
	if response.mode == clientResponseRedirect {
		http.Redirect(writer, request, "/", http.StatusTemporaryRedirect)
		return
	}
	mediaType := response.jsonMediaType
	if response.mode == clientResponseWrongMediaType {
		mediaType = response.plainMediaType
	}
	writer.Header().Set(core.HTTPHeaderContentType().String(), mediaType.String())
	if response.mode == clientResponseWrongStatus {
		writer.WriteHeader(http.StatusCreated)
		return
	}
	writer.WriteHeader(http.StatusOK)
	if response.mode == clientResponseAbsentBody {
		return
	}
	if response.mode == clientResponseMalformedJSON {
		_, _ = writer.Write([]byte{'{'})
		return
	}
	_, _ = writer.Write(response.body)
}

func TestRoutedSocketAuthorityRejectsThirtyThreeExternalRequestBoundaries(t *testing.T) {
	t.Parallel()

	fixture := productionSocketFixture(t)
	encoded, err := fixture.request.MarshalJSON()
	if err != nil {
		t.Fatalf("RegistrationRequest.MarshalJSON() error = %v, want nil", err)
	}
	route, err := fixture.request.ControlRoute()
	if err != nil {
		t.Fatalf("RegistrationRequest.ControlRoute() error = %v, want nil", err)
	}
	path, err := route.Path()
	if err != nil {
		t.Fatalf("RouteContract.Path() error = %v, want nil", err)
	}
	jsonMediaType := standardMediaType(t, exchange.StandardMediaTypeJSON)
	plainMediaType := standardMediaType(t, exchange.StandardMediaTypePlainText)
	otherNonce, err := controlwire.GenerateRequestNonce()
	if err != nil {
		t.Fatalf("GenerateRequestNonce() error = %v, want nil", err)
	}
	base := func(document []byte) *http.Request {
		request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(document))
		request.Header.Set(core.HTTPHeaderContentType().String(), jsonMediaType.String())
		request.Header.Set(core.HTTPHeaderIdempotencyKey().String(), fixture.request.RequestNonce.String())
		return request
	}
	type hostileCase struct {
		name  string
		build func() *http.Request
		want  []error
	}
	cases := []hostileCase{
		{name: "nil HTTP request is rejected", build: func() *http.Request { return nil }, want: []error{core.ErrControlWireRoute, core.ErrExchangeContract}},
		{name: "nil URL is rejected", build: func() *http.Request { request := base(encoded); request.URL = nil; return request }, want: []error{core.ErrControlWireRoute, core.ErrExchangeContract}},
		{name: "root path is rejected", build: func() *http.Request { return httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(encoded)) }, want: []error{core.ErrControlWireRoute}},
		{name: "trailing slash is rejected", build: func() *http.Request { request := base(encoded); request.URL.Path += "/"; return request }, want: []error{core.ErrControlWireRoute}},
		{name: "explicit raw path is rejected", build: func() *http.Request { request := base(encoded); request.URL.RawPath = request.URL.Path; return request }, want: []error{core.ErrControlWireRoute}},
		{name: "query is rejected", build: func() *http.Request { request := base(encoded); request.URL.RawQuery = "page=1"; return request }, want: []error{core.ErrControlWireRoute}},
		{name: "empty query marker is rejected", build: func() *http.Request { request := base(encoded); request.URL.ForceQuery = true; return request }, want: []error{core.ErrControlWireRoute}},
		{name: "GET method is rejected", build: func() *http.Request { request := base(encoded); request.Method = http.MethodGet; return request }, want: []error{core.ErrExchangeRequest, core.ErrExchangeContract}},
		{name: "lowercase POST is rejected", build: func() *http.Request { request := base(encoded); request.Method = "post"; return request }, want: []error{core.ErrExchangeRequest, core.ErrExchangeContract}},
		{name: "missing content type is rejected", build: func() *http.Request {
			request := base(encoded)
			request.Header.Del(core.HTTPHeaderContentType().String())
			return request
		}, want: []error{core.ErrExchangeRequest, core.ErrExchangeContentType}},
		{name: "plain text content type is rejected", build: func() *http.Request {
			request := base(encoded)
			request.Header.Set(core.HTTPHeaderContentType().String(), plainMediaType.String())
			return request
		}, want: []error{core.ErrExchangeRequest, core.ErrExchangeContentType}},
		{name: "duplicate content type is rejected", build: func() *http.Request {
			request := base(encoded)
			request.Header.Add(core.HTTPHeaderContentType().String(), jsonMediaType.String())
			return request
		}, want: []error{core.ErrExchangeRequest, core.ErrExchangeContentType}},
		{name: "content coding is rejected", build: func() *http.Request {
			request := base(encoded)
			request.Header.Set(core.HTTPHeaderContentEncoding().String(), "gzip")
			return request
		}, want: []error{core.ErrExchangeRequest, core.ErrExchangeContentType}},
		{name: "missing idempotency is rejected", build: func() *http.Request {
			request := base(encoded)
			request.Header.Del(core.HTTPHeaderIdempotencyKey().String())
			return request
		}, want: []error{core.ErrExchangeRequest, core.ErrExchangeContract}},
		{name: "duplicate idempotency is rejected", build: func() *http.Request {
			request := base(encoded)
			request.Header.Add(core.HTTPHeaderIdempotencyKey().String(), fixture.request.RequestNonce.String())
			return request
		}, want: []error{core.ErrExchangeRequest, core.ErrExchangeContract}},
		{name: "malformed idempotency is rejected", build: func() *http.Request {
			request := base(encoded)
			request.Header.Set(core.HTTPHeaderIdempotencyKey().String(), "invalid key")
			return request
		}, want: []error{core.ErrExchangeRequest, core.ErrExchangeContract}},
		{name: "different idempotency is rejected", build: func() *http.Request {
			request := base(encoded)
			request.Header.Set(core.HTTPHeaderIdempotencyKey().String(), otherNonce.String())
			return request
		}, want: []error{core.ErrControlWireNonce, core.ErrExchangeContract}},
		{name: "one byte above request document ceiling is rejected", build: func() *http.Request {
			return base(padSocketRequest(encoded, controlplane.RegistrationRequestJSONMaximumBytes+1))
		}, want: []error{core.ErrExchangeRequest, core.ErrExchangeBodyLimit}},
		{name: "one byte below shared ceiling is stopped by tighter product limit", build: func() *http.Request {
			return base(bytes.Repeat([]byte{' '}, core.JSONDocumentMaximumBytes-1))
		}, want: []error{core.ErrExchangeRequest, core.ErrExchangeBodyLimit}},
		{name: "exact shared ceiling is stopped by tighter product limit", build: func() *http.Request {
			return base(bytes.Repeat([]byte{' '}, core.JSONDocumentMaximumBytes))
		}, want: []error{core.ErrExchangeRequest, core.ErrExchangeBodyLimit}},
		{name: "one byte above transport ceiling is rejected before typed decoding", build: func() *http.Request {
			return base(bytes.Repeat([]byte{' '}, core.JSONDocumentMaximumBytes+1))
		}, want: []error{core.ErrExchangeRequest, core.ErrExchangeBodyLimit}},
	}
	malformed := []struct {
		name string
		body []byte
	}{
		{name: "empty body", body: nil},
		{name: "zero length body", body: []byte{}},
		{name: "whitespace body", body: []byte{' ', '\n'}},
		{name: "null body", body: []byte("null")},
		{name: "array body", body: []byte("[]")},
		{name: "empty object body", body: []byte("{}")},
		{name: "opening object only", body: []byte{'{'}},
		{name: "first byte truncation", body: bytes.Clone(encoded[:1])},
		{name: "midpoint truncation", body: bytes.Clone(encoded[:len(encoded)/2])},
		{name: "one byte truncation", body: bytes.Clone(encoded[:len(encoded)-1])},
		{name: "trailing scalar", body: append(bytes.Clone(encoded), '0')},
		{name: "trailing object", body: append(bytes.Clone(encoded), '{', '}')},
	}
	for _, malformedCase := range malformed {
		current := malformedCase
		cases = append(cases, hostileCase{
			name:  current.name + " is rejected",
			build: func() *http.Request { return base(current.body) },
			want:  []error{core.ErrExchangeRequest, core.ErrJSONContract},
		})
	}
	if len(cases) != 33 {
		t.Fatalf("authority hostile inventory = %d cases, want exactly 33", len(cases))
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, receiveErr := controlwire.ReceiveRoutedJSON[
				controlplane.RegistrationRequest,
				*controlplane.RegistrationRequest,
			](controlwire.AuthorityJSONReceiveCall{Request: tc.build(), Route: route, Support: fixture.support})
			if got.Body != nil || !got.IdempotencyKey.IsZero() || got.Replay != (controlwire.ReplayIdentity{}) {
				t.Fatalf("rejected receive = %+v, want zero result", got)
			}
			for _, want := range tc.want {
				if !errors.Is(receiveErr, want) {
					t.Fatalf("ReceiveRoutedJSON() error = %v, want errors.Is %v", receiveErr, want)
				}
			}
		})
	}
}

func FuzzRoutedSocketAuthoritySemanticClosure(f *testing.F) {
	fixture := productionSocketFixture(f)
	canonical, err := fixture.request.MarshalJSON()
	if err != nil {
		f.Fatalf("RegistrationRequest.MarshalJSON() error = %v, want nil", err)
	}
	jsonMediaType := standardMediaType(f, exchange.StandardMediaTypeJSON)
	plainMediaType := standardMediaType(f, exchange.StandardMediaTypePlainText)
	bodyLimitFact, err := fixture.request.ControlRequestBodyLimit()
	if err != nil {
		f.Fatalf("RegistrationRequest.ControlRequestBodyLimit() error = %v, want nil", err)
	}
	bodyLimit, err := bodyLimitFact.Uint64()
	if err != nil {
		f.Fatalf("registration request body limit Uint64() error = %v, want nil", err)
	}
	validVector := append([]byte{0, 0, 0}, []byte(fixture.request.RequestNonce.String())...)
	for _, tc := range validSocketRequestBodies(f, fixture.request) {
		f.Add(tc.body, validVector)
	}
	for _, document := range [][]byte{
		nil, {}, []byte("null"), []byte("{}"), []byte("[]"), []byte{'{'},
		padSocketRequest(canonical, controlplane.RegistrationRequestJSONMaximumBytes+1),
		bytes.Repeat([]byte{' '}, core.JSONDocumentMaximumBytes-1),
		bytes.Repeat([]byte{' '}, core.JSONDocumentMaximumBytes),
		bytes.Repeat([]byte{' '}, core.JSONDocumentMaximumBytes+1),
	} {
		f.Add(document, validVector)
	}
	f.Add(canonical, append([]byte{0, 0, 0}, []byte("invalid key")...))
	f.Add(canonical, append([]byte{1, 0, 0}, []byte(fixture.request.RequestNonce.String())...))
	f.Add(canonical, append([]byte{0, 1, 0}, []byte(fixture.request.RequestNonce.String())...))
	f.Add(canonical, append([]byte{0, 0, 1}, []byte(fixture.request.RequestNonce.String())...))

	f.Fuzz(func(t *testing.T, document, vector []byte) {
		route, routeErr := fixture.request.ControlRoute()
		if routeErr != nil {
			t.Fatalf("RegistrationRequest.ControlRoute() error = %v, want nil", routeErr)
		}
		path, pathErr := route.Path()
		if pathErr != nil {
			t.Fatalf("RouteContract.Path() error = %v, want nil", pathErr)
		}
		modes, key := fuzzModes(vector)
		input := fuzzRequestInput{
			document: document, key: key, path: path,
			pathMode: modes[0], methodMode: modes[1], contentMode: modes[2],
			jsonMediaType: jsonMediaType, plainMediaType: plainMediaType,
		}
		got, receiveErr := controlwire.ReceiveRoutedJSON[
			controlplane.RegistrationRequest,
			*controlplane.RegistrationRequest,
		](controlwire.AuthorityJSONReceiveCall{Request: fuzzRequest(input), Route: route, Support: fixture.support})
		oracle := receiveOracle(receiveOracleInput{
			document: document, key: key, pathMode: modes[0],
			methodMode: modes[1], contentMode: modes[2], route: route, bodyLimit: bodyLimit,
		})
		if oracle.err != nil {
			if got.Body != nil || !got.IdempotencyKey.IsZero() || got.Replay != (controlwire.ReplayIdentity{}) {
				t.Fatalf("rejected receive = %+v, want zero result", got)
			}
			for _, want := range oracle.want {
				if !errors.Is(receiveErr, want) {
					t.Fatalf("ReceiveRoutedJSON() error = %v, want errors.Is %v", receiveErr, want)
				}
			}
			return
		}
		if receiveErr != nil || got.Body == nil || got.Body.Validate() != nil {
			t.Fatalf("ReceiveRoutedJSON() = (%+v, %v), want exact validated production request", got, receiveErr)
		}
		if got.Assessment.Outcome != controlwire.ProtocolSupportOutcomeAccepted ||
			got.Assessment.Capability.Revision != got.Body.ControlRevision() ||
			got.Assessment.Capability.Family != route.Family() {
			t.Fatalf("received protocol assessment = %+v, want exact accepted request pair", got.Assessment)
		}
		canonicalGot, marshalErr := got.Body.MarshalJSON()
		if marshalErr != nil || !bytes.Equal(canonicalGot, oracle.canonical) {
			t.Fatalf("received canonical body = (%d bytes, %v), want exact %d bytes", len(canonicalGot), marshalErr, len(oracle.canonical))
		}
		wantKey, keyErr := got.Body.RequestNonce.IdempotencyKey()
		if keyErr != nil || got.IdempotencyKey.String() != wantKey.String() {
			t.Fatalf("received idempotency = %q, want %q; oracle error %v", got.IdempotencyKey.String(), wantKey.String(), keyErr)
		}
	})
}

type receiveOracleResult struct {
	body      controlplane.RegistrationRequest
	canonical []byte
	want      []error
	err       error
}

type receiveOracleInput struct {
	document    []byte
	key         string
	pathMode    uint8
	methodMode  uint8
	contentMode uint8
	route       controlwire.RouteContract
	bodyLimit   uint64
}

func receiveOracle(input receiveOracleInput) receiveOracleResult {
	if input.pathMode%4 != 0 {
		return receiveOracleResult{err: core.ErrControlWireRoute, want: []error{core.ErrControlWireRoute}}
	}
	if input.methodMode%4 != 0 {
		return receiveOracleResult{err: core.ErrExchangeRequest, want: []error{core.ErrExchangeRequest, core.ErrExchangeContract}}
	}
	if input.contentMode%4 != 0 {
		return receiveOracleResult{err: core.ErrExchangeContentType, want: []error{core.ErrExchangeRequest, core.ErrExchangeContentType}}
	}
	parsedKey, keyErr := exchange.ParseIdempotencyKey(input.key)
	if keyErr != nil {
		return receiveOracleResult{err: keyErr, want: []error{core.ErrExchangeRequest, core.ErrExchangeContract}}
	}
	if uint64(len(input.document)) > input.bodyLimit {
		return receiveOracleResult{err: core.ErrExchangeBodyLimit, want: []error{core.ErrExchangeRequest, core.ErrExchangeBodyLimit}}
	}
	var body controlplane.RegistrationRequest
	if decodeErr := body.UnmarshalJSON(input.document); decodeErr != nil {
		return receiveOracleResult{err: decodeErr, want: []error{core.ErrExchangeRequest, core.ErrJSONContract}}
	}
	actualRoute, routeErr := body.ControlRoute()
	if routeErr != nil || actualRoute != input.route {
		return receiveOracleResult{err: core.ErrControlWireRoute, want: []error{core.ErrControlWireRoute}}
	}
	wantKey, wantKeyErr := body.RequestNonce.IdempotencyKey()
	if wantKeyErr != nil || parsedKey.String() != wantKey.String() {
		return receiveOracleResult{err: core.ErrControlWireNonce, want: []error{core.ErrControlWireNonce, core.ErrExchangeContract}}
	}
	canonical, marshalErr := body.MarshalJSON()
	if marshalErr != nil {
		return receiveOracleResult{err: marshalErr, want: []error{core.ErrControlPlaneRegistration}}
	}
	return receiveOracleResult{body: body, canonical: canonical}
}

type fuzzRequestInput struct {
	document       []byte
	key            string
	path           string
	pathMode       uint8
	methodMode     uint8
	contentMode    uint8
	jsonMediaType  core.HTTPMediaType
	plainMediaType core.HTTPMediaType
}

func fuzzRequest(input fuzzRequestInput) *http.Request {
	paths := [...]string{input.path, input.path + "/", "/", input.path + "?page=1"}
	methods := [...]string{http.MethodPost, http.MethodGet, http.MethodPut, "post"}
	request := httptest.NewRequest(
		methods[input.methodMode%uint8(len(methods))],
		paths[input.pathMode%uint8(len(paths))], bytes.NewReader(input.document),
	)
	request.Header.Set(core.HTTPHeaderIdempotencyKey().String(), input.key)
	switch input.contentMode % 4 {
	case 0:
		request.Header.Set(core.HTTPHeaderContentType().String(), input.jsonMediaType.String())
	case 1:
		request.Header.Del(core.HTTPHeaderContentType().String())
	case 2:
		request.Header.Set(core.HTTPHeaderContentType().String(), input.plainMediaType.String())
	case 3:
		request.Header.Set(core.HTTPHeaderContentEncoding().String(), "gzip")
	}
	return request
}

func fuzzModes(vector []byte) ([3]uint8, string) {
	var modes [3]uint8
	if len(vector) < len(modes) {
		copy(modes[:], vector)
		return modes, ""
	}
	copy(modes[:], vector[:len(modes)])
	return modes, string(vector[len(modes):])
}

func productionSocketFixture(t testing.TB) socketFixture {
	t.Helper()

	authoritySeed := [ed25519.SeedSize]byte{}
	deviceSeed := [ed25519.SeedSize]byte{}
	for index := range authoritySeed {
		authoritySeed[index] = 0x41
		deviceSeed[index] = 0x51
	}
	installation, err := controlplanetest.IssueInstallation(controlplanetest.InstallationRequest{
		AuthoritySeed: authoritySeed, DeviceSeed: deviceSeed, Offering: core.OfferingWitness,
	})
	if err != nil {
		t.Fatalf("controlplanetest.IssueInstallation() error = %v, want nil", err)
	}
	tokenBytes := [controlwire.RegistrationTokenBytes]byte{1}
	token, err := controlwire.NewRegistrationToken(tokenBytes)
	if err != nil {
		t.Fatalf("controlwire.NewRegistrationToken() error = %v, want nil", err)
	}
	nonceBytes := [controlwire.NonceBytes]byte{1}
	nonce, err := controlwire.NewRequestNonce(nonceBytes)
	if err != nil {
		t.Fatalf("controlwire.NewRequestNonce() error = %v, want nil", err)
	}
	request := controlplane.RegistrationRequest{
		Token: token, Build: installation.Build, RequestNonce: nonce,
		DeviceKey:    installation.DevicePublic,
		Installation: installation.Certificate.Body.Subject.DeviceID,
		Revision:     controlwire.Revision2026V1,
	}
	if err := request.Validate(); err != nil {
		t.Fatalf("compiler-built RegistrationRequest.Validate() error = %v, want nil", err)
	}
	responseBytes, err := os.ReadFile("../controlplane/testdata/registration_response.json")
	if err != nil {
		t.Fatalf("reading registration response fixture error = %v, want nil", err)
	}
	var body controlplane.RegistrationDocument
	if err := body.UnmarshalJSON(responseBytes); err != nil {
		t.Fatalf("RegistrationDocument.UnmarshalJSON() error = %v, want nil", err)
	}
	material := bytes.Repeat([]byte{0x73}, ed25519.SeedSize)
	support, err := controlwire.PublishedProtocolSupport()
	if err != nil {
		t.Fatalf("PublishedProtocolSupport() error = %v, want nil", err)
	}
	assessment, err := controlwire.AssessProtocol(controlwire.ProtocolAssessmentRequest{
		Support: support,
		Capability: controlwire.ProtocolCapability{
			Revision: body.Payload.Header.Revision,
			Family:   body.Payload.Header.Family,
		},
	})
	if err != nil || assessment.Outcome != controlwire.ProtocolSupportOutcomeAccepted {
		t.Fatalf("AssessProtocol(response pair) = (%+v, %v), want accepted and nil", assessment, err)
	}
	projection, err := controlplane.IssueResponse(controlplane.ResponseIssuance[controlplane.RegistrationDocument]{
		Signer: ed25519.NewKeyFromSeed(material), Header: body.Payload.Header, Body: body,
		Assessment: assessment,
	})
	if err != nil {
		t.Fatalf("IssueResponse() error = %v, want nil", err)
	}
	canonical, err := projection.MarshalJSON()
	if err != nil {
		t.Fatalf("ResponseProjection.MarshalJSON() error = %v, want nil", err)
	}
	return socketFixture{request: request, response: projection, responseCanonical: canonical, support: support}
}

func standardMediaType(t testing.TB, value exchange.StandardMediaType) core.HTTPMediaType {
	t.Helper()

	mediaType, err := value.HTTPMediaType()
	if err != nil {
		t.Fatalf("StandardMediaType.HTTPMediaType() error = %v, want nil", err)
	}
	return mediaType
}

func authorityWithSuffix(t testing.TB, authority core.HTTPEndpoint, suffix string) core.HTTPEndpoint {
	t.Helper()

	endpoint, err := core.ParseHTTPEndpoint(authority.String() + suffix)
	if err != nil {
		t.Fatalf("ParseHTTPEndpoint(authority suffix) error = %v, want nil", err)
	}
	return endpoint
}

var (
	_ controlwire.RoutedJSONRequest             = controlplane.RegistrationRequest{}
	_ controlwire.AuthenticatedResponseDocument = (*controlplane.ResponseDocument[controlplane.RegistrationDocument, *controlplane.RegistrationDocument])(nil)
)
