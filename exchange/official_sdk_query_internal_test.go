package exchange

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

const (
	sdkQueryFixtureName  = "alt"
	sdkQueryFixtureValue = "media"
)

type sdkQueryObservedBody struct {
	reader    *bytes.Reader
	readBytes int
	closes    int
}

func (b *sdkQueryObservedBody) Read(p []byte) (int, error) {
	n, err := b.reader.Read(p)
	b.readBytes += n
	return n, err
}
func (b *sdkQueryObservedBody) Close() error { b.closes++; return nil }

type sdkQueryObservedTransport struct {
	body   *sdkQueryObservedBody
	calls  int
	status int
}

func (r *sdkQueryObservedTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	r.calls++
	return &http.Response{StatusCode: r.status, Body: r.body, ContentLength: -1, Request: request}, nil
}

func sdkQueryBoundary(t testing.TB) OfficialSDKResponseBoundary {
	t.Helper()
	limit, err := core.NewByteCount(1)
	if err != nil {
		t.Fatal(err)
	}
	boundary, err := NewOfficialSDKStreamingSuccessCeiling(OfficialSDKStreamingSuccessCeilingRequest{
		StreamQueryName: sdkQueryFixtureName, StreamQueryValue: sdkQueryFixtureValue,
		AggregateMaximumBytes: limit, Method: MethodGet,
		AggregateRepresentation: OfficialSDKResponseRepresentationBinary,
	})
	if err != nil {
		t.Fatal(err)
	}
	return boundary
}

type sdkQueryExecution struct {
	response      *http.Response
	err           error
	body          *sdkQueryObservedBody
	providerCalls int
}

// executeSDKQueryFixture only constructs the standard RoundTripper seam and
// returns observed facts. The table and fuzz callback own all comparisons.
func executeSDKQueryFixture(t *testing.T, query string, status int, payload []byte) sdkQueryExecution {
	t.Helper()
	body := &sdkQueryObservedBody{reader: bytes.NewReader(payload)}
	base := &sdkQueryObservedTransport{body: body, status: status}
	transport, err := NewOfficialSDKResponseTransport(OfficialSDKResponseTransportRequest{Base: base, Boundary: sdkQueryBoundary(t)})
	if err != nil {
		t.Fatalf("NewOfficialSDKResponseTransport() setup error = %v, want nil", err)
	}
	request := httptest.NewRequest(http.MethodGet, "https://provider.example.test/object", nil)
	request.URL.RawQuery = query
	response, err := transport.RoundTrip(request)
	return sdkQueryExecution{response: response, err: err, body: body, providerCalls: base.calls}
}

func TestOfficialSDKQueryCustodyLayerTriad(t *testing.T) {
	t.Parallel()
	exact := url.QueryEscape(sdkQueryFixtureName) + "=" + url.QueryEscape(sdkQueryFixtureValue)
	cases := []struct {
		name, query   string
		status        int
		wantStream    bool
		wantErr       error
		wantReadBytes int
		wantCloses    int
	}{
		{name: "positive exact coordinate transfers unread custody", query: exact, status: http.StatusOK, wantStream: true},
		{name: "positive escaped coordinate uses Go query semantics", query: "%61lt=m%65dia", status: http.StatusOK, wantStream: true},
		{name: "neutral unrelated valid field leaves selection intact", query: "projection=full&" + exact, status: http.StatusOK, wantStream: true},
		{name: "neutral empty separators use Go query semantics", query: "&&" + exact + "&&", status: http.StatusOK, wantStream: true},
		{name: "negative malformed unrelated value cannot disappear", query: exact + "&broken=%zz", status: http.StatusOK, wantErr: core.ErrExchangeBodyLimit, wantReadBytes: 2, wantCloses: 1},
		{name: "negative malformed unrelated name cannot disappear", query: exact + "&%zz=broken", status: http.StatusOK, wantErr: core.ErrExchangeBodyLimit, wantReadBytes: 2, wantCloses: 1},
		{name: "negative malformed second selected value cannot disappear", query: exact + "&alt=%zz", status: http.StatusOK, wantErr: core.ErrExchangeBodyLimit, wantReadBytes: 2, wantCloses: 1},
		{name: "negative semicolon field cannot disappear", query: exact + "&broken=a;b", status: http.StatusOK, wantErr: core.ErrExchangeBodyLimit, wantReadBytes: 2, wantCloses: 1},
		{name: "negative incomplete escape cannot disappear", query: "broken=%&" + exact, status: http.StatusOK, wantErr: core.ErrExchangeBodyLimit, wantReadBytes: 2, wantCloses: 1},
		{name: "negative duplicate exact coordinate is ambiguous", query: exact + "&" + exact, status: http.StatusOK, wantErr: core.ErrExchangeBodyLimit, wantReadBytes: 2, wantCloses: 1},
		{name: "negative conflicting duplicate is ambiguous", query: exact + "&alt=json", status: http.StatusOK, wantErr: core.ErrExchangeBodyLimit, wantReadBytes: 2, wantCloses: 1},
		{name: "negative missing equals adds empty duplicate", query: exact + "&alt", status: http.StatusOK, wantErr: core.ErrExchangeBodyLimit, wantReadBytes: 2, wantCloses: 1},
		{name: "negative encoded duplicate is ambiguous", query: exact + "&%61lt=media", status: http.StatusOK, wantErr: core.ErrExchangeBodyLimit, wantReadBytes: 2, wantCloses: 1},
		{name: "boundary empty query aggregates", status: http.StatusOK, wantErr: core.ErrExchangeBodyLimit, wantReadBytes: 2, wantCloses: 1},
		{name: "boundary absent selected value aggregates", query: "alt=", status: http.StatusOK, wantErr: core.ErrExchangeBodyLimit, wantReadBytes: 2, wantCloses: 1},
		{name: "boundary plus means space not exact coordinate", query: "alt=media+", status: http.StatusOK, wantErr: core.ErrExchangeBodyLimit, wantReadBytes: 2, wantCloses: 1},
		{name: "boundary case-sensitive name aggregates", query: "ALT=media", status: http.StatusOK, wantErr: core.ErrExchangeBodyLimit, wantReadBytes: 2, wantCloses: 1},
		{name: "boundary case-sensitive value aggregates", query: "alt=MEDIA", status: http.StatusOK, wantErr: core.ErrExchangeBodyLimit, wantReadBytes: 2, wantCloses: 1},
		{name: "boundary escaped separator is part of value", query: "alt=media%26other=value", status: http.StatusOK, wantErr: core.ErrExchangeBodyLimit, wantReadBytes: 2, wantCloses: 1},
		{name: "boundary last successful status keeps streaming custody", query: exact, status: http.StatusMultipleChoices - 1, wantStream: true},
		{name: "boundary first redirect status cannot bypass aggregation", query: exact, status: http.StatusMultipleChoices, wantErr: core.ErrExchangeBodyLimit, wantReadBytes: 2, wantCloses: 1},
		{name: "boundary status immediately below success cannot bypass aggregation", query: exact, status: http.StatusOK - 1, wantErr: core.ErrExchangeBodyLimit, wantReadBytes: 2, wantCloses: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payload := []byte{0x00, 0xff}
			got := executeSDKQueryFixture(t, tc.query, tc.status, payload)
			if got.providerCalls != 1 {
				t.Fatalf("provider calls = %d, want 1", got.providerCalls)
			}
			if !errors.Is(got.err, tc.wantErr) {
				t.Fatalf("response error = %v, want %v", got.err, tc.wantErr)
			}
			if (got.response != nil) != tc.wantStream {
				t.Fatalf("response present = %t, want %t", got.response != nil, tc.wantStream)
			}
			if got.body.readBytes != tc.wantReadBytes || got.body.closes != tc.wantCloses {
				t.Fatalf("body read/close = %d/%d, want %d/%d", got.body.readBytes, got.body.closes, tc.wantReadBytes, tc.wantCloses)
			}
			if tc.wantErr != nil && !errors.Is(got.err, core.ErrExchangeResponse) {
				t.Fatalf("boundary error = %v, want %v", got.err, core.ErrExchangeResponse)
			}
			if got.response == nil {
				return
			}
			if got.response.Body != got.body || got.response.StatusCode != tc.status || got.response.ContentLength != -1 {
				t.Fatalf("stream response = %v, want original body, status %d and absent extent", got.response, tc.status)
			}
			gotBody, gotReadErr := io.ReadAll(got.response.Body)
			gotCloseErr := got.response.Body.Close()
			if gotReadErr != nil || gotCloseErr != nil || !bytes.Equal(gotBody, payload) || got.body.closes != 1 {
				t.Fatalf("caller read/close = %x/%v/%v/%d closes, want %x/nil/nil/1", gotBody, gotReadErr, gotCloseErr, got.body.closes, payload)
			}
		})
	}
}

func FuzzOfficialSDKQueryCustodySemanticBoundary(f *testing.F) {
	boundary := sdkQueryBoundary(f)
	query := url.QueryEscape(boundary.streamQueryName) + "=" + url.QueryEscape(boundary.streamQueryValue)
	f.Add(query, uint8(0))
	f.Add(query+"&bad=%zz", uint8(0))
	f.Add(query+"&"+query, uint8(0))
	f.Add(query, uint8(1))
	f.Add("", uint8(0))
	f.Fuzz(func(t *testing.T, query string, statusInput uint8) {
		if len(query) > SocketRequestTargetMaximumBytes {
			query = query[:SocketRequestTargetMaximumBytes]
		}
		statuses := [...]int{http.StatusOK, http.StatusMultipleChoices, http.StatusBadRequest, http.StatusServiceUnavailable}
		status := statuses[int(statusInput)%len(statuses)]
		parsed, parseErr := url.ParseQuery(query)
		values := parsed[sdkQueryFixtureName]
		want := parseErr == nil && len(values) == 1 && values[0] == sdkQueryFixtureValue && status == http.StatusOK
		payload := []byte{0x00, 0xff}
		got := executeSDKQueryFixture(t, query, status, payload)
		if got.providerCalls != 1 {
			t.Fatalf("provider calls = %d, want 1", got.providerCalls)
		}
		if !want {
			if got.response != nil || !errors.Is(got.err, core.ErrExchangeResponse) || !errors.Is(got.err, core.ErrExchangeBodyLimit) {
				t.Fatalf("aggregate response/error = %v/%v, want nil and response/body-limit identities", got.response, got.err)
			}
			if got.body.readBytes != len(payload) || got.body.closes != 1 {
				t.Fatalf("aggregate read/close = %d/%d, want %d/1", got.body.readBytes, got.body.closes, len(payload))
			}
			return
		}
		if got.err != nil || got.response == nil {
			t.Fatalf("stream response/error = %v/%v, want nonnil/nil", got.response, got.err)
		}
		if got.body.readBytes != 0 || got.body.closes != 0 {
			t.Fatalf("premature stream read/close = %d/%d, want 0/0", got.body.readBytes, got.body.closes)
		}
		if got.response.Body != got.body || got.response.StatusCode != status || got.response.ContentLength != -1 {
			t.Fatalf("stream response = %v, want original body, status %d and absent extent", got.response, status)
		}
		gotBody, gotReadErr := io.ReadAll(got.response.Body)
		gotCloseErr := got.response.Body.Close()
		if gotReadErr != nil || gotCloseErr != nil || !bytes.Equal(gotBody, payload) || got.body.closes != 1 {
			t.Fatalf("caller read/close = %x/%v/%v/%d closes, want %x/nil/nil/1", gotBody, gotReadErr, gotCloseErr, got.body.closes, payload)
		}
	})
}
