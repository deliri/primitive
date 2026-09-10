package exchange

import (
	"errors"
	"net/http"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// BenchmarkAggregateHeaderValueAdmission measures the real response-ingress
// producer, including exact body custody. A reusable Go response fixture keeps
// its already-received header allocation outside the timer. The caller's fixed
// selection is one field; admitted conversion work is bounded at 64 values.
// Both revisions run this identical harness. Every iteration checks refusal,
// status, counts, and custody; the final admitted values are checked exactly.
func BenchmarkAggregateHeaderValueAdmission(b *testing.B) {
	b.ReportAllocs()
	cases := []struct {
		name       string
		values     int
		wantErr    error
		wantFields int
		wantValues int
		wantReads  int
		wantStatus core.HTTPStatusCode
	}{
		{name: "at_limit", values: HeaderValueMaximumCount, wantFields: 1, wantValues: HeaderValueMaximumCount, wantReads: 1, wantStatus: core.HTTPStatusOK()},
		{name: "above_limit", values: HeaderValueMaximumCount + 1, wantErr: core.ErrExchangeResponse},
		{name: "excess_4096", values: 4096, wantErr: core.ErrExchangeResponse},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			name := core.HTTPHeaderAccept()
			wire := captureFixtureValues(tc.values)
			headers := make(http.Header)
			headers[name.String()] = wire
			body := &captureFixtureBody{}
			input := aggregateReadRequest{context: b.Context(), response: &http.Response{StatusCode: http.StatusOK, Header: headers, Body: body}, capture: HeaderSelection{Names: []core.HTTPHeaderName{name}}, expectedStatus: core.HTTPStatusOK()}
			var got attemptResponse
			var err error
			b.ReportAllocs()
			for b.Loop() {
				body.reads, body.closes = 0, 0
				body.source.Reset(nil)
				got, err = readAggregateHTTPResponse(input)
				if !errors.Is(err, tc.wantErr) || got.status != tc.wantStatus || len(got.headers.Values) != tc.wantFields || len(got.body) != 0 || got.retryAfter != "" || body.reads != tc.wantReads || body.closes != 1 {
					b.Fatalf("ingress error/status/fields/body/retry/reads/closes = (%v, %v, %d, %d, %q, %d, %d), want (%v, %v, %d, 0, empty, %d, 1)", err, got.status, len(got.headers.Values), len(got.body), got.retryAfter, body.reads, body.closes, tc.wantErr, tc.wantStatus, tc.wantFields, tc.wantReads)
				}
				if tc.wantErr != nil && (got.headers.Values != nil || got.body != nil) {
					b.Fatalf("refused headers/body=%+v/%x, want nil/nil", got.headers.Values, got.body)
				}
			}
			for _, field := range got.headers.Values {
				if field.Name != name || len(field.Values) != tc.wantValues {
					b.Fatalf("final field = %v, want exact admitted name and %d values", field, tc.wantValues)
				}
				for i, value := range field.Values {
					gotWire, err := value.Value()
					if err != nil || gotWire != wire[i] {
						b.Fatalf("final value[%d] = (%q, %v), want (%q, nil)", i, gotWire, err, wire[i])
					}
				}
			}
		})
	}
}
