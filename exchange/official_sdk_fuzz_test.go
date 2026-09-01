package exchange_test

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

const officialSDKFuzzBodyMaximum = 4097

type officialSDKFuzzFraming uint8

type officialSDKFuzzJSONDocument struct {
	Value string `json:"value"`
}

const (
	officialSDKFuzzFramingDeclaredExact officialSDKFuzzFraming = iota
	officialSDKFuzzFramingChunked
	officialSDKFuzzFramingDeclaredLonger
	officialSDKFuzzFramingLimitPlusOne
	officialSDKFuzzFramingLimit
)

func (f officialSDKFuzzFraming) declaredLength(bodyBytes int, limit uint64) int {
	switch f {
	case officialSDKFuzzFramingDeclaredExact:
		return bodyBytes
	case officialSDKFuzzFramingChunked:
		return -1
	case officialSDKFuzzFramingDeclaredLonger:
		return bodyBytes + 1
	case officialSDKFuzzFramingLimitPlusOne:
		if uint64(bodyBytes) > limit {
			return bodyBytes + 1
		}
		return int(limit + 1)
	default:
		return -2
	}
}

func FuzzOfficialSDKResponseTransportSemanticBoundary(f *testing.F) {
	seedLimit, seedLimitErr := core.NewByteCount(1)
	if seedLimitErr != nil {
		f.Fatalf("core.NewByteCount(seed) error = %v, want nil", seedLimitErr)
	}
	seedBoundary, seedBoundaryErr := exchange.NewOfficialSDKResponseBoundary(exchange.OfficialSDKResponseBoundaryRequest{
		Method: exchange.MethodGet, PathPrefix: "/selected/", PathSuffix: "/response",
		Representation: exchange.OfficialSDKResponseRepresentationJSON,
		MaximumBytes:   seedLimit,
	})
	if seedBoundaryErr != nil || seedBoundary.Validate() != nil {
		f.Fatalf("canonical SDK response boundary = (%v, %v), want validated boundary and nil", seedBoundary, seedBoundaryErr)
	}

	canonicalJSON, canonicalJSONErr := json.Marshal(officialSDKFuzzJSONDocument{Value: "canonical"})
	if canonicalJSONErr != nil {
		f.Fatalf("json.Marshal(canonical SDK response) error = %v, want nil", canonicalJSONErr)
	}
	for _, seed := range []struct {
		body           []byte
		limit          uint16
		selectedPath   bool
		selectedMethod bool
		framing        uint8
		statusClass    uint8
		cancelled      bool
		jsonResponse   bool
	}{
		{body: []byte(""), limit: 1, selectedPath: true, selectedMethod: true, framing: uint8(officialSDKFuzzFramingDeclaredExact)},
		{body: []byte("a"), limit: 1, selectedPath: true, selectedMethod: true, framing: uint8(officialSDKFuzzFramingDeclaredExact)},
		{body: []byte("ab"), limit: 1, selectedPath: true, selectedMethod: true, framing: uint8(officialSDKFuzzFramingDeclaredExact)},
		{body: []byte("chunked"), limit: 7, selectedPath: true, selectedMethod: true, framing: uint8(officialSDKFuzzFramingChunked)},
		{body: []byte("truncated"), limit: 32, selectedPath: true, selectedMethod: true, framing: uint8(officialSDKFuzzFramingDeclaredLonger)},
		{body: []byte("declared-over-limit"), limit: 8, selectedPath: true, selectedMethod: true, framing: uint8(officialSDKFuzzFramingLimitPlusOne)},
		{body: []byte("neutral-path"), limit: 1, selectedMethod: true, framing: uint8(officialSDKFuzzFramingDeclaredExact)},
		{body: []byte("neutral-method"), limit: 1, selectedPath: true, framing: uint8(officialSDKFuzzFramingDeclaredExact)},
		{body: []byte("cancelled"), limit: 32, selectedPath: true, selectedMethod: true, framing: uint8(officialSDKFuzzFramingDeclaredExact), cancelled: true},
		{body: make([]byte, officialSDKFuzzBodyMaximum), limit: officialSDKFuzzBodyMaximum - 1, selectedPath: true, selectedMethod: true, framing: uint8(officialSDKFuzzFramingChunked), statusClass: 3},
		{body: canonicalJSON, limit: uint16(len(canonicalJSON)), selectedPath: true, selectedMethod: true, framing: uint8(officialSDKFuzzFramingDeclaredExact), jsonResponse: true},
		{body: []byte("{\"value\":\"\xff\"}"), limit: 32, selectedPath: true, selectedMethod: true, framing: uint8(officialSDKFuzzFramingDeclaredExact), jsonResponse: true},
	} {
		f.Add(seed.body, seed.limit, seed.selectedPath, seed.selectedMethod, seed.framing, seed.statusClass, seed.cancelled, seed.jsonResponse)
	}

	f.Fuzz(func(
		t *testing.T,
		body []byte,
		limitInput uint16,
		selectedPath bool,
		selectedMethod bool,
		framingInput uint8,
		statusInput uint8,
		cancelled bool,
		jsonResponse bool,
	) {
		if len(body) > officialSDKFuzzBodyMaximum {
			body = body[:officialSDKFuzzBodyMaximum]
		}
		limitValue := uint64(limitInput)%uint64(officialSDKFuzzBodyMaximum-1) + 1
		limit, limitErr := core.NewByteCount(limitValue)
		if limitErr != nil {
			t.Fatalf("core.NewByteCount(%d) error = %v, want nil", limitValue, limitErr)
		}
		representation := exchange.OfficialSDKResponseRepresentationBinary
		if jsonResponse {
			representation = exchange.OfficialSDKResponseRepresentationJSON
		}
		boundary, boundaryErr := exchange.NewOfficialSDKResponseBoundary(exchange.OfficialSDKResponseBoundaryRequest{
			Method: exchange.MethodGet, PathPrefix: "/selected/", PathSuffix: "/response",
			Representation: representation, MaximumBytes: limit,
		})
		if boundaryErr != nil || boundary.Validate() != nil {
			t.Fatalf("exchange.NewOfficialSDKResponseBoundary() = (%v, %v), want validated boundary and nil", boundary, boundaryErr)
		}

		framing := officialSDKFuzzFraming(framingInput % uint8(officialSDKFuzzFramingLimit))
		declaredLength := framing.declaredLength(len(body), limitValue)
		if declaredLength < -1 {
			t.Fatalf("fuzz framing %d declared length = %d, want a closed framing", framing, declaredLength)
		}
		statusCodes := [...]int{
			http.StatusOK,
			http.StatusBadRequest,
			http.StatusTooManyRequests,
			http.StatusInternalServerError,
		}
		statusCode := statusCodes[int(statusInput)%len(statusCodes)]
		path := "/neutral/provider/response"
		if selectedPath {
			path = "/selected/provider/response"
		}
		method := http.MethodPost
		if selectedMethod {
			method = http.MethodGet
		}
		wantMatched := selectedPath && selectedMethod

		var calls atomic.Int64
		writeResults := make(chan error, 1)
		endpoint := officialSDKFuzzHTTPServer(t, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			if framing == officialSDKFuzzFramingChunked {
				writer.WriteHeader(statusCode)
				flusher, ok := writer.(http.Flusher)
				if !ok {
					writeResults <- core.ErrExchangeContract
					return
				}
				flusher.Flush()
			} else {
				writer.Header().Set("Content-Length", strconv.Itoa(declaredLength))
				writer.WriteHeader(statusCode)
			}
			_, writeErr := writer.Write(body)
			writeResults <- writeErr
		}))

		client := officialSDKLingerClient(t, boundary)
		ctx := context.Background()
		if cancelled {
			var cancel context.CancelFunc
			ctx, cancel = context.WithCancel(ctx)
			cancel()
		}
		request, requestErr := http.NewRequestWithContext(ctx, method, endpoint+path, nil)
		if requestErr != nil {
			t.Fatalf("http.NewRequestWithContext() error = %v, want nil", requestErr)
		}
		response, gotErr := client.Do(request)
		if cancelled {
			if response != nil {
				if closeErr := response.Body.Close(); closeErr != nil {
					t.Errorf("cancelled response close error = %v, want nil", closeErr)
				}
			}
			if response != nil || !errors.Is(gotErr, core.ErrExchangeCancelled) ||
				!errors.Is(gotErr, context.Canceled) || calls.Load() != 0 {
				t.Fatalf("pre-cancelled SDK exchange = (%v, %v, %d calls), want nil, cancellation identities, and zero calls", response, gotErr, calls.Load())
			}
			return
		}

		var gotWriteErr error
		select {
		case gotWriteErr = <-writeResults:
		case <-time.After(officialSDKTestTimeout):
			t.Fatal("provider response write completed = false, want true before timeout")
		}
		if gotCalls := calls.Load(); gotCalls != 1 {
			t.Fatalf("provider calls = %d, want 1", gotCalls)
		}
		wantDeclaredRejection := wantMatched && declaredLength >= 0 && uint64(declaredLength) > limitValue
		wantStreamRejection := wantMatched && framing == officialSDKFuzzFramingChunked && uint64(len(body)) > limitValue
		wantTruncation := declaredLength > len(body) && !wantDeclaredRejection
		if wantDeclaredRejection || wantStreamRejection {
			if response != nil {
				if closeErr := response.Body.Close(); closeErr != nil {
					t.Errorf("rejected response close error = %v, want nil", closeErr)
				}
			}
			if response != nil || !errors.Is(gotErr, core.ErrExchangeResponse) ||
				!errors.Is(gotErr, core.ErrExchangeBodyLimit) {
				t.Fatalf("oversized matched response = (%v, %v), want nil, %v, and %v", response, gotErr, core.ErrExchangeResponse, core.ErrExchangeBodyLimit)
			}
			return
		}
		if wantMatched && wantTruncation {
			if response != nil || !errors.Is(gotErr, core.ErrExchangeResponse) ||
				!errors.Is(gotErr, io.ErrUnexpectedEOF) {
				t.Fatalf("truncated matched response = (%v, %v), want nil, %v, and %v", response, gotErr, core.ErrExchangeResponse, io.ErrUnexpectedEOF)
			}
			return
		}
		wantJSONRejection := wantMatched && jsonResponse && len(body) != 0 &&
			!jsontext.Value(body).IsValid()
		if wantJSONRejection {
			if response != nil || !errors.Is(gotErr, core.ErrExchangeResponse) ||
				!errors.Is(gotErr, core.ErrJSONContract) {
				t.Fatalf("invalid matched JSON response = (%v, %v), want nil, %v, and %v", response, gotErr, core.ErrExchangeResponse, core.ErrJSONContract)
			}
			return
		}
		if gotErr != nil || response == nil || response.StatusCode != statusCode {
			t.Fatalf("admitted response = (%v, %v), want status %d and nil", response, gotErr, statusCode)
		}
		gotBody, readErr := io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		if wantTruncation {
			if !errors.Is(readErr, io.ErrUnexpectedEOF) || !bytes.Equal(gotBody, body) || closeErr != nil {
				t.Fatalf("neutral truncated stream = (%d bytes, %v, %v), want exact %d bytes, %v, and nil", len(gotBody), readErr, closeErr, len(body), io.ErrUnexpectedEOF)
			}
			return
		}
		if readErr != nil || closeErr != nil || !bytes.Equal(gotBody, body) {
			t.Fatalf("admitted response body = (%d bytes, %v, %v), want exact %d bytes and nil/nil", len(gotBody), readErr, closeErr, len(body))
		}
		if wantMatched && response.ContentLength != int64(len(body)) {
			t.Fatalf("buffered response ContentLength = %d, want %d", response.ContentLength, len(body))
		}
		if gotWriteErr != nil {
			t.Fatalf("provider response write error = %v, want nil", gotWriteErr)
		}
	})
}

func FuzzOfficialSDKStreamingSuccessResponseSemanticBoundary(f *testing.F) {
	canonicalJSON, err := json.Marshal(officialSDKFuzzJSONDocument{Value: "streaming-boundary"})
	if err != nil {
		f.Fatalf("json.Marshal(streaming boundary seed) error = %v, want nil", err)
	}
	for _, seed := range []struct {
		body        []byte
		queryClass  uint8
		statusClass uint8
	}{
		{body: []byte("media bytes are not JSON"), queryClass: 0, statusClass: 0},
		{body: make([]byte, 65), queryClass: 0, statusClass: 0},
		{body: canonicalJSON, queryClass: 1, statusClass: 0},
		{body: []byte("not-json"), queryClass: 1, statusClass: 0},
		{body: canonicalJSON, queryClass: 2, statusClass: 0},
		{body: canonicalJSON, queryClass: 3, statusClass: 0},
		{body: canonicalJSON, queryClass: 0, statusClass: 1},
		{body: make([]byte, 65), queryClass: 0, statusClass: 1},
	} {
		f.Add(seed.body, seed.queryClass, seed.statusClass)
	}

	f.Fuzz(func(t *testing.T, body []byte, queryInput uint8, statusInput uint8) {
		if len(body) > officialSDKFuzzBodyMaximum {
			body = body[:officialSDKFuzzBodyMaximum]
		}
		limit, limitErr := core.NewByteCount(64)
		if limitErr != nil {
			t.Fatalf("core.NewByteCount(64) error = %v, want nil", limitErr)
		}
		boundary, boundaryErr := exchange.NewOfficialSDKStreamingSuccessCeiling(
			exchange.OfficialSDKStreamingSuccessCeilingRequest{
				Method: exchange.MethodGet, StreamQueryName: "alt", StreamQueryValue: "media",
				AggregateRepresentation: exchange.OfficialSDKResponseRepresentationJSON,
				AggregateMaximumBytes:   limit,
			},
		)
		if boundaryErr != nil || boundary.Validate() != nil {
			t.Fatalf("streaming-success boundary = (%v, %v), want validated boundary and nil", boundary, boundaryErr)
		}
		queries := [...]string{"alt=media", "alt=json", "alt=media&alt=media", "projection=full"}
		queryClass := int(queryInput) % len(queries)
		statuses := [...]int{http.StatusOK, http.StatusInternalServerError}
		statusClass := int(statusInput) % len(statuses)
		status := statuses[statusClass]
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(status)
			_, _ = writer.Write(body)
		}))
		t.Cleanup(server.Close)
		client := officialSDKClient(t, boundary)
		response, gotErr := client.Get(server.URL + "/object?" + queries[queryClass])

		wantStreaming := queryClass == 0 && statusClass == 0
		wantBodyLimit := !wantStreaming && len(body) > 64
		wantJSONRejection := !wantStreaming && !wantBodyLimit && len(body) != 0 && !jsontext.Value(body).IsValid()
		if wantBodyLimit || wantJSONRejection {
			if response != nil {
				_ = response.Body.Close()
			}
			wantCause := error(core.ErrJSONContract)
			if wantBodyLimit {
				wantCause = core.ErrExchangeBodyLimit
			}
			if response != nil || !errors.Is(gotErr, core.ErrExchangeResponse) || !errors.Is(gotErr, wantCause) {
				t.Fatalf("conditional aggregate response = (%v, %v), want nil, %v, and %v", response, gotErr, core.ErrExchangeResponse, wantCause)
			}
			return
		}
		if gotErr != nil || response == nil || response.StatusCode != status {
			t.Fatalf("conditional streaming response = (%v, %v), want status %d and nil", response, gotErr, status)
		}
		gotBody, readErr := io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		if readErr != nil || closeErr != nil || !bytes.Equal(gotBody, body) {
			t.Fatalf("conditional streaming body = (%d bytes, %v, %v), want exact %d bytes and nil/nil", len(gotBody), readErr, closeErr, len(body))
		}
	})
}

func officialSDKFuzzHTTPServer(t testing.TB, handler http.Handler) string {
	t.Helper()

	listener, listenErr := net.Listen("tcp", "127.0.0.1:0")
	if listenErr != nil {
		t.Fatalf("net.Listen(loopback SDK provider) error = %v, want nil", listenErr)
	}
	server := &http.Server{Handler: handler}
	served := make(chan error, 1)
	go func() {
		served <- server.Serve(listener)
	}()
	t.Cleanup(func() {
		closeErr := server.Close()
		select {
		case serveErr := <-served:
			if closeErr != nil || (!errors.Is(serveErr, http.ErrServerClosed) && !errors.Is(serveErr, net.ErrClosed)) {
				t.Errorf("loopback SDK provider shutdown = (%v, %v), want nil and closed-server identity", closeErr, serveErr)
			}
		case <-time.After(officialSDKTestTimeout):
			t.Errorf("loopback SDK provider shutdown completed = false, want true before timeout")
		}
	})
	return "http://" + listener.Addr().String()
}

func officialSDKLingerClient(
	t testing.TB,
	boundary exchange.OfficialSDKResponseBoundary,
) *http.Client {
	t.Helper()

	base := &http.Transport{DialContext: officialSDKLingerDialContext}
	t.Cleanup(base.CloseIdleConnections)
	transport, transportErr := exchange.NewOfficialSDKResponseTransport(
		exchange.OfficialSDKResponseTransportRequest{Base: base, Boundary: boundary},
	)
	if transportErr != nil {
		t.Fatalf("exchange.NewOfficialSDKResponseTransport(fuzz) error = %v, want nil", transportErr)
	}
	client, clientErr := exchange.NewOfficialSDKHTTPClient(transport)
	if clientErr != nil {
		t.Fatalf("exchange.NewOfficialSDKHTTPClient(fuzz) error = %v, want nil", clientErr)
	}
	return client
}

func officialSDKLingerDialContext(
	ctx context.Context,
	network string,
	address string,
) (net.Conn, error) {
	var dialer net.Dialer
	connection, dialErr := dialer.DialContext(ctx, network, address)
	if dialErr != nil {
		return nil, dialErr
	}
	if lingerErr := setOfficialSDKLinger(connection); lingerErr != nil {
		_ = connection.Close()
		return nil, lingerErr
	}
	return connection, nil
}

func setOfficialSDKLinger(connection net.Conn) error {
	tcpConnection, ok := connection.(*net.TCPConn)
	if !ok {
		return nil
	}
	return tcpConnection.SetLinger(0)
}

func FuzzOfficialSDKResponseRepresentationJSONSemanticClosure(f *testing.F) {
	for _, representation := range []exchange.OfficialSDKResponseRepresentation{
		exchange.OfficialSDKResponseRepresentationBinary,
		exchange.OfficialSDKResponseRepresentationJSON,
	} {
		if err := representation.Validate(); err != nil {
			f.Fatalf("representation seed Validate() error = %v, want nil", err)
		}
		canonical, err := representation.MarshalJSON()
		if err != nil {
			f.Fatalf("representation seed MarshalJSON() error = %v, want nil", err)
		}
		f.Add(canonical)
	}
	for _, hostile := range [][]byte{
		{},
		[]byte(`null`),
		[]byte(`""`),
		[]byte(`"future"`),
		[]byte(`"JSON"`),
		[]byte(`{"representation":"json"}`),
		[]byte(`"json" trailing`),
		[]byte{0xff},
	} {
		f.Add(hostile)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		before := exchange.OfficialSDKResponseRepresentationBinary
		got := before
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) ||
				!errors.Is(gotErr, core.ErrExchangeContract) || got != before {
				t.Fatalf("representation UnmarshalJSON(rejected) = (%v, %v), want preserved, %v, and %v", got, gotErr, core.ErrJSONContract, core.ErrExchangeContract)
			}
			return
		}

		if err := got.Validate(); err != nil {
			t.Fatalf("representation UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		var token string
		oracleErr := json.Unmarshal(data, &token)
		if oracleErr != nil || token != got.String() {
			t.Fatalf("independent JSON token oracle = (%q, %v), want (%q, nil)", token, oracleErr, got.String())
		}
		canonical, marshalErr := got.MarshalJSON()
		if marshalErr != nil {
			t.Fatalf("accepted representation MarshalJSON() error = %v, want nil", marshalErr)
		}
		var roundTrip exchange.OfficialSDKResponseRepresentation
		roundTripErr := roundTrip.UnmarshalJSON(canonical)
		if roundTripErr != nil || roundTrip != got {
			t.Fatalf("representation canonical round trip = (%v, %v), want (%v, nil)", roundTrip, roundTripErr, got)
		}
		second, secondErr := roundTrip.MarshalJSON()
		if secondErr != nil || !bytes.Equal(second, canonical) {
			t.Fatalf("representation second canonical projection = (%q, %v), want (%q, nil)", second, secondErr, canonical)
		}
	})
}
