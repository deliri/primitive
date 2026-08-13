package controlwire

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type socketRequest struct {
	Offering core.Offering `json:"offering"`
	Nonce    RequestNonce  `json:"request_nonce"`
}

type socketRequestWire socketRequest

type socketResponse struct {
	Nonce RequestNonce `json:"request_nonce"`
}

type socketResponseWire socketResponse

type socketObservation struct {
	Body   socketRequest
	Method string
	Path   string
	Key    string
	Err    error
}

func (r socketRequest) Validate() error {
	if err := r.Offering.Validate(); err != nil {
		return errors.Join(core.ErrControlWireContract, err)
	}
	if err := r.Nonce.Validate(); err != nil {
		return errors.Join(core.ErrControlWireContract, err)
	}
	return nil
}

func (r socketRequest) ControlRoute() (RouteContract, error) {
	return NewRouteContract(r.Offering, RouteFamilyRegistrations)
}

func (r socketRequest) ControlNonce() RequestNonce { return r.Nonce }

func (r socketRequest) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONDocument(socketRequestWire(r))
}

func (r *socketRequest) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.Join(core.ErrJSONContract, core.ErrControlWireContract)
	}
	wire, err := core.DecodeStrictJSONStructure[socketRequestWire](
		data, core.DefaultStrictJSONLimits(),
	)
	if err != nil {
		return errors.Join(core.ErrJSONContract, core.ErrControlWireContract, err)
	}
	candidate := socketRequest(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*r = candidate
	return nil
}

func (r socketResponse) Validate() error { return r.Nonce.Validate() }

func (r socketResponse) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONDocument(socketResponseWire(r))
}

func (r *socketResponse) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.Join(core.ErrJSONContract, core.ErrControlWireContract)
	}
	wire, err := core.DecodeStrictJSONStructure[socketResponseWire](
		data, core.DefaultStrictJSONLimits(),
	)
	if err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	candidate := socketResponse(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*r = candidate
	return nil
}

func TestRoutedSocketLayerTriadExecutesBothRealHTTPBoundaries(t *testing.T) {
	t.Parallel()

	nonce, err := GenerateRequestNonce()
	if err != nil {
		t.Fatalf("GenerateRequestNonce() error = %v, want nil", err)
	}
	body := socketRequest{Offering: core.OfferingPeachfuzz, Nonce: nonce}
	route, err := body.ControlRoute()
	if err != nil {
		t.Fatalf("ControlRoute() error = %v, want nil", err)
	}
	observed := make(chan socketObservation, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		received, receiveErr := ReceiveRoutedJSON[socketRequest, *socketRequest](
			AuthorityJSONReceiveCall{Request: request, Route: route},
		)
		observation := socketObservation{
			Method: request.Method, Path: request.URL.Path,
			Key: request.Header.Get(core.HTTPHeaderIdempotencyKey().String()),
			Err: receiveErr,
		}
		if receiveErr == nil {
			observation.Body = *received.Body
		}
		observed <- observation
		if receiveErr != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		if writeErr := WriteControlJSON(ControlJSONWriteCall[socketResponse]{
			Writer: writer, Body: socketResponse{Nonce: received.Body.Nonce},
		}); writeErr != nil {
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
	response, err := SendRoutedJSON[socketRequest, socketResponse, *socketResponse](
		ClientJSONCall[socketRequest]{
			Context: context.Background(), Client: client,
			Authority: authority, Body: body,
		},
	)
	if err != nil {
		t.Fatalf("SendRoutedJSON() error = %v, want nil", err)
	}
	if response.Body == nil || response.Body.Nonce != nonce {
		t.Fatalf("response body = %+v, want nonce %v", response.Body, nonce)
	}
	if err := response.Validate(); err != nil {
		t.Fatalf("response Validate() error = %v, want nil", err)
	}
	wantResponse, err := (socketResponse{Nonce: nonce}).MarshalJSON()
	if err != nil {
		t.Fatalf("socket response MarshalJSON() error = %v, want nil", err)
	}
	if response.Metadata.Bytes.Uint64() != uint64(len(wantResponse)) ||
		response.Metadata.Attempts != 1 || response.Metadata.Status != core.HTTPStatusOK() {
		t.Fatalf("response metadata = %+v, want %d bytes, one attempt, and HTTP 200",
			response.Metadata, len(wantResponse))
	}
	got := <-observed
	wantPath, err := route.Path()
	if err != nil {
		t.Fatalf("route Path() error = %v, want nil", err)
	}
	if got.Err != nil || got.Body != body || got.Method != http.MethodPost ||
		got.Path != wantPath || got.Key != nonce.String() {
		t.Fatalf("authority observation = %+v, want exact body/method/path/nonce", got)
	}
}

type socketClientResponseMode uint8

const (
	socketClientResponseValid socketClientResponseMode = iota
	socketClientResponseWrongStatus
	socketClientResponseMalformedJSON
	socketClientResponseAbsentBody
	socketClientResponseWrongMediaType
	socketClientResponseRedirect
)

func TestRoutedSocketClientLayerTriadRejectsEveryIndependentBoundarySubstitution(t *testing.T) {
	t.Parallel()

	nonce, err := GenerateRequestNonce()
	if err != nil {
		t.Fatalf("GenerateRequestNonce() error = %v, want nil", err)
	}
	body := socketRequest{Offering: core.OfferingWitness, Nonce: nonce}
	jsonMediaType, err := exchange.StandardMediaTypeJSON.HTTPMediaType()
	if err != nil {
		t.Fatalf("JSON HTTPMediaType() error = %v, want nil", err)
	}
	plainMediaType, err := exchange.StandardMediaTypePlainText.HTTPMediaType()
	if err != nil {
		t.Fatalf("plain text HTTPMediaType() error = %v, want nil", err)
	}
	validResponse, err := (socketResponse{Nonce: nonce}).MarshalJSON()
	if err != nil {
		t.Fatalf("socket response MarshalJSON() error = %v, want nil", err)
	}

	type hostileCase struct {
		name   string
		mode   socketClientResponseMode
		mutate func(*ClientJSONCall[socketRequest])
		want   []error
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	cases := []hostileCase{
		{name: "nil context", mutate: func(call *ClientJSONCall[socketRequest]) { call.Context = nil }, want: []error{core.ErrExchangeContract}},
		{name: "cancelled context", mutate: func(call *ClientJSONCall[socketRequest]) { call.Context = cancelled }, want: []error{core.ErrExchangeRequest, core.ErrExchangeCancelled}},
		{name: "zero client", mutate: func(call *ClientJSONCall[socketRequest]) { call.Client = exchange.Client{} }, want: []error{core.ErrExchangeContract}},
		{name: "zero authority", mutate: func(call *ClientJSONCall[socketRequest]) { call.Authority = core.HTTPEndpoint{} }, want: []error{core.ErrControlWireRoute}},
		{name: "authority owns a path", mutate: func(call *ClientJSONCall[socketRequest]) {
			call.Authority = socketAuthorityWithSuffix(t, call.Authority, "/base")
		}, want: []error{core.ErrControlWireRoute, core.ErrExchangeContract}},
		{name: "authority owns a query", mutate: func(call *ClientJSONCall[socketRequest]) {
			call.Authority = socketAuthorityWithSuffix(t, call.Authority, "?page=1")
		}, want: []error{core.ErrControlWireRoute, core.ErrExchangeContract}},
		{name: "zero request body", mutate: func(call *ClientJSONCall[socketRequest]) { call.Body = socketRequest{} }, want: []error{core.ErrControlWireContract}},
		{name: "unexpected success status", mode: socketClientResponseWrongStatus, want: []error{core.ErrExchangeResponse}},
		{name: "malformed response JSON", mode: socketClientResponseMalformedJSON, want: []error{core.ErrExchangeResponse, core.ErrJSONContract}},
		{name: "absent response body", mode: socketClientResponseAbsentBody, want: []error{core.ErrExchangeResponse, core.ErrJSONContract}},
		{name: "wrong response media type", mode: socketClientResponseWrongMediaType, want: []error{core.ErrExchangeResponse, core.ErrExchangeContentType}},
		{name: "redirect response", mode: socketClientResponseRedirect, want: []error{core.ErrExchangeRedirect, core.ErrExchangeContract}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				socketWriteHostileResponse(writer, request, socketHostileResponse{
					mode: testCase.mode, body: validResponse,
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
			call := ClientJSONCall[socketRequest]{
				Context: context.Background(), Client: client,
				Authority: authority, Body: body,
			}
			if testCase.mutate != nil {
				testCase.mutate(&call)
			}
			got, sendErr := SendRoutedJSON[socketRequest, socketResponse, *socketResponse](call)
			if got.Body != nil {
				t.Fatalf("rejected response body = %+v, want nil", got.Body)
			}
			for _, want := range testCase.want {
				if !errors.Is(sendErr, want) {
					t.Fatalf("SendRoutedJSON() error = %v, want errors.Is %v", sendErr, want)
				}
			}
		})
	}
}

type socketHostileResponse struct {
	mode           socketClientResponseMode
	body           []byte
	jsonMediaType  core.HTTPMediaType
	plainMediaType core.HTTPMediaType
}

func socketWriteHostileResponse(
	writer http.ResponseWriter,
	request *http.Request,
	response socketHostileResponse,
) {
	if response.mode == socketClientResponseRedirect {
		http.Redirect(writer, request, "/", http.StatusTemporaryRedirect)
		return
	}
	if response.mode == socketClientResponseWrongMediaType {
		writer.Header().Set(core.HTTPHeaderContentType().String(), response.plainMediaType.String())
	} else {
		writer.Header().Set(core.HTTPHeaderContentType().String(), response.jsonMediaType.String())
	}
	if response.mode == socketClientResponseWrongStatus {
		writer.WriteHeader(http.StatusCreated)
		return
	}
	writer.WriteHeader(http.StatusOK)
	if response.mode == socketClientResponseAbsentBody {
		return
	}
	if response.mode == socketClientResponseMalformedJSON {
		_, _ = writer.Write([]byte{'{'})
		return
	}
	_, _ = writer.Write(response.body)
}

func socketAuthorityWithSuffix(
	t testing.TB,
	authority core.HTTPEndpoint,
	suffix string,
) core.HTTPEndpoint {
	endpoint, err := core.ParseHTTPEndpoint(authority.String() + suffix)
	if err != nil {
		t.Fatalf("ParseHTTPEndpoint(authority suffix) error = %v, want nil", err)
	}
	return endpoint
}

func TestRoutedSocketLayerTriadRejectsEveryIndependentBoundarySubstitution(t *testing.T) {
	t.Parallel()

	nonce, err := GenerateRequestNonce()
	if err != nil {
		t.Fatalf("GenerateRequestNonce() error = %v, want nil", err)
	}
	body := socketRequest{Offering: core.OfferingBug, Nonce: nonce}
	encoded, err := body.MarshalJSON()
	if err != nil {
		t.Fatalf("socket request MarshalJSON() error = %v, want nil", err)
	}
	route, err := body.ControlRoute()
	if err != nil {
		t.Fatalf("ControlRoute() error = %v, want nil", err)
	}
	path, err := route.Path()
	if err != nil {
		t.Fatalf("route Path() error = %v, want nil", err)
	}
	jsonMediaType, err := exchange.StandardMediaTypeJSON.HTTPMediaType()
	if err != nil {
		t.Fatalf("JSON HTTPMediaType() error = %v, want nil", err)
	}
	plainMediaType, err := exchange.StandardMediaTypePlainText.HTTPMediaType()
	if err != nil {
		t.Fatalf("plain text HTTPMediaType() error = %v, want nil", err)
	}
	otherNonce, err := GenerateRequestNonce()
	if err != nil {
		t.Fatalf("GenerateRequestNonce(other) error = %v, want nil", err)
	}
	otherBody, err := (socketRequest{Offering: core.OfferingWitness, Nonce: nonce}).MarshalJSON()
	if err != nil {
		t.Fatalf("other socket request MarshalJSON() error = %v, want nil", err)
	}

	type hostileCase struct {
		name   string
		build  func() *http.Request
		mutate func(*http.Request)
		want   []error
	}
	base := func(document []byte) *http.Request {
		request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(document))
		request.Header.Set(core.HTTPHeaderContentType().String(), jsonMediaType.String())
		request.Header.Set(core.HTTPHeaderIdempotencyKey().String(), nonce.String())
		return request
	}
	malformed := [][]byte{
		nil, {}, []byte(`null`), []byte(`[]`), []byte(`{`),
		[]byte(`{"offering":"bug"}`),
		[]byte(`{"offering":"bug","request_nonce":null}`),
		[]byte(`{"offering":"bug","request_nonce":"not-a-nonce"}`),
		[]byte(`{"offering":"bug","request_nonce":"0198f5ef-81a6-7abc-8123-0123456789ab","extra":true}`),
		[]byte(`{"offering":"bug","offering":"bug","request_nonce":"0198f5ef-81a6-7abc-8123-0123456789ab"}`),
	}
	cases := []hostileCase{
		{name: "nil request", build: func() *http.Request { return nil }, want: []error{core.ErrControlWireRoute, core.ErrExchangeContract}},
		{name: "nil URL", build: func() *http.Request { request := base(encoded); request.URL = nil; return request }, want: []error{core.ErrControlWireRoute, core.ErrExchangeContract}},
		{name: "wrong path", build: func() *http.Request { return httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(encoded)) }, want: []error{core.ErrControlWireRoute}},
		{name: "trailing slash", build: func() *http.Request { request := base(encoded); request.URL.Path += "/"; return request }, want: []error{core.ErrControlWireRoute}},
		{name: "raw path", build: func() *http.Request { request := base(encoded); request.URL.RawPath = request.URL.Path; return request }, want: []error{core.ErrControlWireRoute}},
		{name: "query", build: func() *http.Request { request := base(encoded); request.URL.RawQuery = "page=1"; return request }, want: []error{core.ErrControlWireRoute}},
		{name: "empty query marker", build: func() *http.Request { request := base(encoded); request.URL.ForceQuery = true; return request }, want: []error{core.ErrControlWireRoute}},
		{name: "wrong method", build: func() *http.Request { request := base(encoded); request.Method = http.MethodGet; return request }, want: []error{core.ErrExchangeRequest, core.ErrExchangeContract}},
		{name: "lowercase method", build: func() *http.Request { request := base(encoded); request.Method = "post"; return request }, want: []error{core.ErrExchangeRequest, core.ErrExchangeContract}},
		{name: "missing content type", build: func() *http.Request {
			request := base(encoded)
			request.Header.Del(core.HTTPHeaderContentType().String())
			return request
		}, want: []error{core.ErrExchangeRequest, core.ErrExchangeContentType}},
		{name: "wrong content type", build: func() *http.Request {
			request := base(encoded)
			request.Header.Set(core.HTTPHeaderContentType().String(), plainMediaType.String())
			return request
		}, want: []error{core.ErrExchangeRequest, core.ErrExchangeContentType}},
		{name: "duplicate content type", build: func() *http.Request {
			request := base(encoded)
			request.Header.Add(core.HTTPHeaderContentType().String(), jsonMediaType.String())
			return request
		}, want: []error{core.ErrExchangeRequest, core.ErrExchangeContentType}},
		{name: "unsupported content coding", build: func() *http.Request {
			request := base(encoded)
			request.Header.Set(core.HTTPHeaderContentEncoding().String(), "gzip")
			return request
		}, want: []error{core.ErrExchangeRequest, core.ErrExchangeContentType}},
		{name: "missing idempotency", build: func() *http.Request {
			request := base(encoded)
			request.Header.Del(core.HTTPHeaderIdempotencyKey().String())
			return request
		}, want: []error{core.ErrExchangeRequest, core.ErrExchangeContract}},
		{name: "duplicate idempotency", build: func() *http.Request {
			request := base(encoded)
			request.Header.Add(core.HTTPHeaderIdempotencyKey().String(), nonce.String())
			return request
		}, want: []error{core.ErrExchangeRequest, core.ErrExchangeContract}},
		{name: "invalid idempotency", build: func() *http.Request {
			request := base(encoded)
			request.Header.Set(core.HTTPHeaderIdempotencyKey().String(), "invalid key")
			return request
		}, want: []error{core.ErrExchangeRequest, core.ErrExchangeContract}},
		{name: "different idempotency", build: func() *http.Request {
			request := base(encoded)
			request.Header.Set(core.HTTPHeaderIdempotencyKey().String(), otherNonce.String())
			return request
		}, want: []error{core.ErrControlWireNonce, core.ErrExchangeContract}},
		{name: "different offering", build: func() *http.Request { return base(otherBody) }, want: []error{core.ErrControlWireRoute, core.ErrExchangeContract}},
		{name: "above document ceiling", build: func() *http.Request { return base(bytes.Repeat([]byte{' '}, core.JSONDocumentMaximumBytes+1)) }, want: []error{core.ErrExchangeRequest, core.ErrExchangeBodyLimit}},
	}
	for index, document := range malformed {
		cases = append(cases, hostileCase{
			name:  "malformed document " + string(rune('a'+index)),
			build: func() *http.Request { return base(document) },
			want:  []error{core.ErrExchangeRequest, core.ErrJSONContract},
		})
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			request := testCase.build()
			if request != nil && testCase.mutate != nil {
				testCase.mutate(request)
			}
			got, receiveErr := ReceiveRoutedJSON[socketRequest, *socketRequest](
				AuthorityJSONReceiveCall{Request: request, Route: route},
			)
			if got.Body != nil || !got.IdempotencyKey.IsZero() {
				t.Fatalf("rejected receive = %+v, want zero result", got)
			}
			for _, want := range testCase.want {
				if !errors.Is(receiveErr, want) {
					t.Fatalf("ReceiveRoutedJSON() error = %v, want errors.Is %v", receiveErr, want)
				}
			}
		})
	}
}

func FuzzRoutedSocketAuthoritySemanticClosure(f *testing.F) {
	nonce, err := GenerateRequestNonce()
	if err != nil {
		f.Fatalf("GenerateRequestNonce() error = %v, want nil", err)
	}
	seed := socketRequest{Offering: core.OfferingPeachfuzz, Nonce: nonce}
	canonical, err := seed.MarshalJSON()
	if err != nil {
		f.Fatalf("socket request MarshalJSON() error = %v, want nil", err)
	}
	jsonMediaType, err := exchange.StandardMediaTypeJSON.HTTPMediaType()
	if err != nil {
		f.Fatalf("JSON HTTPMediaType() error = %v, want nil", err)
	}
	plainMediaType, err := exchange.StandardMediaTypePlainText.HTTPMediaType()
	if err != nil {
		f.Fatalf("plain text HTTPMediaType() error = %v, want nil", err)
	}
	validVector := append([]byte{0, 0, 0}, []byte(nonce.String())...)
	f.Add(canonical, validVector)
	for _, document := range [][]byte{
		nil, {}, []byte(`null`), []byte(`{}`), []byte(`[]`), []byte(`{`),
		bytes.Repeat([]byte{' '}, core.JSONDocumentMaximumBytes+1),
	} {
		f.Add(document, validVector)
	}
	f.Add(canonical, append([]byte{0, 0, 0}, []byte("invalid key")...))
	f.Add(canonical, append([]byte{1, 0, 0}, []byte(nonce.String())...))
	f.Add(canonical, append([]byte{0, 1, 0}, []byte(nonce.String())...))
	f.Add(canonical, append([]byte{0, 0, 1}, []byte(nonce.String())...))

	f.Fuzz(func(t *testing.T, document, vector []byte) {
		route, routeErr := seed.ControlRoute()
		if routeErr != nil {
			t.Fatalf("seed ControlRoute() error = %v, want nil", routeErr)
		}
		path, pathErr := route.Path()
		if pathErr != nil {
			t.Fatalf("seed route Path() error = %v, want nil", pathErr)
		}
		modes, key := socketFuzzModes(vector)
		input := socketFuzzInput{
			document: document, key: key, path: path,
			pathMode: modes[0], methodMode: modes[1], contentMode: modes[2],
			jsonMediaType: jsonMediaType, plainMediaType: plainMediaType,
		}
		request := socketFuzzRequest(input)
		got, receiveErr := ReceiveRoutedJSON[socketRequest, *socketRequest](
			AuthorityJSONReceiveCall{Request: request, Route: route},
		)
		oracle := socketReceiveOracle(socketOracleInput{
			document: document, key: key, pathMode: modes[0],
			methodMode: modes[1], contentMode: modes[2], route: route,
		})
		if oracle.err != nil {
			if got.Body != nil || !got.IdempotencyKey.IsZero() {
				t.Fatalf("rejected receive = %+v, want zero result", got)
			}
			for _, want := range oracle.want {
				if !errors.Is(receiveErr, want) {
					t.Fatalf("ReceiveRoutedJSON() error = %v, want errors.Is %v", receiveErr, want)
				}
			}
			return
		}
		if receiveErr != nil || got.Body == nil || *got.Body != oracle.body {
			t.Fatalf("ReceiveRoutedJSON() = (%+v, %v), want body %+v and nil", got, receiveErr, oracle.body)
		}
		wantKey, keyErr := oracle.body.Nonce.IdempotencyKey()
		if keyErr != nil || got.IdempotencyKey.String() != wantKey.String() {
			t.Fatalf("received idempotency = %q, want %q; oracle error %v",
				got.IdempotencyKey.String(), wantKey.String(), keyErr)
		}
	})
}

type socketOracle struct {
	body socketRequest
	want []error
	err  error
}

type socketOracleInput struct {
	document    []byte
	key         string
	pathMode    uint8
	methodMode  uint8
	contentMode uint8
	route       RouteContract
}

func socketReceiveOracle(input socketOracleInput) socketOracle {
	if input.pathMode%4 != 0 {
		return socketOracle{err: core.ErrControlWireRoute, want: []error{core.ErrControlWireRoute}}
	}
	if input.methodMode%4 != 0 {
		return socketOracle{err: core.ErrExchangeRequest, want: []error{core.ErrExchangeRequest, core.ErrExchangeContract}}
	}
	if input.contentMode%4 != 0 {
		return socketOracle{err: core.ErrExchangeContentType, want: []error{core.ErrExchangeRequest, core.ErrExchangeContentType}}
	}
	parsedKey, keyErr := exchange.ParseIdempotencyKey(input.key)
	if keyErr != nil {
		return socketOracle{err: keyErr, want: []error{core.ErrExchangeRequest, core.ErrExchangeContract}}
	}
	maximum, maximumErr := core.NewByteCount(core.JSONDocumentMaximumBytes)
	if maximumErr != nil {
		return socketOracle{err: maximumErr, want: []error{core.ErrControlWireContract}}
	}
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = maximum
	body, decodeErr := core.DecodeStrictJSON[*socketRequest](input.document, limits)
	if decodeErr != nil {
		want := []error{core.ErrExchangeRequest, core.ErrJSONContract}
		if len(input.document) > core.JSONDocumentMaximumBytes {
			want = []error{core.ErrExchangeRequest, core.ErrExchangeBodyLimit}
		}
		return socketOracle{err: decodeErr, want: want}
	}
	actualRoute, actualRouteErr := body.ControlRoute()
	if actualRouteErr != nil || actualRoute.Offering() != input.route.Offering() ||
		actualRoute.Family() != input.route.Family() {
		return socketOracle{err: core.ErrControlWireRoute, want: []error{core.ErrControlWireRoute}}
	}
	wantKey, wantKeyErr := body.Nonce.IdempotencyKey()
	if wantKeyErr != nil || parsedKey.String() != wantKey.String() {
		return socketOracle{err: core.ErrControlWireNonce, want: []error{core.ErrControlWireNonce, core.ErrExchangeContract}}
	}
	return socketOracle{body: *body}
}

type socketFuzzInput struct {
	document       []byte
	key            string
	path           string
	pathMode       uint8
	methodMode     uint8
	contentMode    uint8
	jsonMediaType  core.HTTPMediaType
	plainMediaType core.HTTPMediaType
}

func socketFuzzRequest(input socketFuzzInput) *http.Request {
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

func socketFuzzModes(vector []byte) ([3]uint8, string) {
	var modes [3]uint8
	if len(vector) < len(modes) {
		copy(modes[:], vector)
		return modes, ""
	}
	copy(modes[:], vector[:len(modes)])
	return modes, string(vector[len(modes):])
}

var (
	_ RoutedJSONRequest = socketRequest{}
	_ core.Validatable  = socketResponse{}
)
