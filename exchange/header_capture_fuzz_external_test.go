package exchange_test

import (
	"bytes"
	"errors"
	"net/http"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func FuzzCapturedHeadersPreserveExactSelection(f *testing.F) {
	// A canonical seed comes from the owning production header value. The
	// oracle uses Go's installed HTTP grammar plus independent count and exact
	// string comparisons; it does not ask the capture validator for its wants.
	seed, err := exchange.NewHeaderValue("opaque-captured-value")
	if err != nil {
		f.Fatalf("typed seed = %v, want nil", err)
	}
	canonical, err := seed.Value()
	if err != nil {
		f.Fatalf("typed seed projection = %v, want nil", err)
	}
	f.Add(canonical, uint16(1), true)
	f.Add(canonical, uint16(exchange.HeaderValueMaximumCount), true)
	f.Add(canonical, uint16(exchange.HeaderValueMaximumCount+1), true)
	f.Add(canonical, uint16(0), true)
	f.Add("\r\n", uint16(1), true)
	f.Add("\r\n", uint16(1), false)
	f.Add("", uint16(1), true)
	f.Add(string([]byte{0x80, 0xff}), uint16(1), true)
	f.Fuzz(func(t *testing.T, wire string, count uint16, selected bool) {
		if len(wire) > exchange.HeaderValueMaximumBytes+1 || count > exchange.HeaderValueMaximumCount+1 {
			return
		}
		name := core.HTTPHeaderAccept()
		values := make([]string, int(count))
		for i := range values {
			values[i] = wire
		}
		headers := make(http.Header)
		headers[name.String()] = values
		body := &bindingObservedBody{reader: bytes.NewReader(nil)}
		calls := 0
		client := mustExchangeClient(t, &http.Client{Transport: bindingTransport(func(request *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: http.StatusOK, Header: headers, Body: body, Request: request}, nil
		})})
		selection := exchange.HeaderSelection{}
		if selected {
			selection.Names = []core.HTTPHeaderName{name}
		}
		got, gotErr := exchange.SendNoBodyBounded(exchange.NoBodyBoundedCall{
			Context: t.Context(), Client: client,
			Request: exchange.NoBodyBoundedRequest{Target: mustEndpoint(t, "http://capture-oracle.invalid/"), Semantics: exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt}, CaptureHeaders: selection, ExpectedStatus: core.HTTPStatusOK()},
			Policy:  exchange.NoBodyBoundedPolicy{Operation: singleAttemptOperationPolicy(t)},
		})
		grammarAccepted := goHeaderGrammarAccepts(t, wire)
		wantRefused := selected && count > 0 && (count > exchange.HeaderValueMaximumCount || len(wire) > exchange.HeaderValueMaximumBytes || !grammarAccepted)
		var wantErr error
		if wantRefused {
			wantErr = core.ErrExchangeResponse
		}
		if !errors.Is(gotErr, wantErr) {
			t.Fatalf("capture error = %v, want %v", gotErr, wantErr)
		}
		if calls != 1 || body.closes != 1 || body.reads != 0 {
			t.Fatalf("capture requests/closes/read bytes = (%d, %d, %d), want (1, 1, 0)", calls, body.closes, body.reads)
		}
		if wantRefused {
			if got.Body != nil || got.Metadata.Attempts != 0 || got.Metadata.Status != (core.HTTPStatusCode{}) || got.Metadata.Bytes.Uint64() != 0 || got.Metadata.Headers.Values != nil {
				t.Fatalf("refused response = %+v, want exact zero", got)
			}
			return
		}
		wantFields := 0
		if selected && count > 0 {
			wantFields = 1
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("response.Validate() = %v, want nil", err)
		}
		if got.Metadata.Status != core.HTTPStatusOK() || got.Metadata.Attempts != 1 || got.Metadata.Bytes.Uint64() != 0 || len(got.Body) != 0 || len(got.Metadata.Headers.Values) != wantFields {
			t.Fatalf("response = %+v, want OK, one attempt, no body, %d fields", got, wantFields)
		}
		for _, field := range got.Metadata.Headers.Values {
			if field.Name != name || len(field.Values) != int(count) {
				t.Fatalf("field name/count = (%v, %d), want (%v, %d)", field.Name, len(field.Values), name, count)
			}
			for _, value := range field.Values {
				gotWire, err := value.Value()
				if err != nil || gotWire != wire {
					t.Fatalf("captured value = (%q, %v), want (%q, nil)", gotWire, err, wire)
				}
			}
		}
	})
}
