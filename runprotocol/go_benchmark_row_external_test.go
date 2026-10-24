package runprotocol_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runprotocol"
)

// A row's kind and measurement agree on the wire: only a measured row carries
// a measurement, every kind round-trips by its closed label, and an unknown
// label or a disagreeing measurement is refused.
func TestGoBenchmarkRowWireLayerTriad(t *testing.T) {
	t.Parallel()
	name, err := runprotocol.NewName("BenchmarkB")
	if err != nil {
		t.Fatal(err)
	}
	measurement := runprotocol.BenchmarkMeasurement{Name: name, Iterations: 1, NanosecondsPerOp: runprotocol.DecimalMeasurement{Coefficient: 83}}
	for _, tc := range []struct {
		layer, name string
		row         runprotocol.GoBenchmarkRow
		wire        string
		wantErr     bool
	}{
		{layer: "positive", name: "measured with its measurement", row: runprotocol.GoBenchmarkRow{Kind: runprotocol.GoBenchmarkRowMeasured, Measurement: &measurement}},
		{layer: "positive", name: "metrics missing without one", row: runprotocol.GoBenchmarkRow{Kind: runprotocol.GoBenchmarkRowMetricsMissing}, wire: `{"kind":"metrics_missing"}`},
		{layer: "positive", name: "unattributed without one", row: runprotocol.GoBenchmarkRow{Kind: runprotocol.GoBenchmarkRowUnattributed}, wire: `{"kind":"unattributed"}`},
		{layer: "negative", name: "unknown kind", row: runprotocol.GoBenchmarkRow{}, wantErr: true},
		{layer: "negative", name: "measured without a measurement", row: runprotocol.GoBenchmarkRow{Kind: runprotocol.GoBenchmarkRowMeasured}, wantErr: true},
		{layer: "negative", name: "a refusal carrying a measurement", row: runprotocol.GoBenchmarkRow{Kind: runprotocol.GoBenchmarkRowInterrupted, Measurement: &measurement}, wantErr: true},
		{layer: "neutral", name: "past the last kind", row: runprotocol.GoBenchmarkRow{Kind: runprotocol.GoBenchmarkRowUnattributed + 1}, wantErr: true},
	} {
		t.Run(tc.layer+"/"+tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.row.Validate()
			if (err != nil) != tc.wantErr || tc.wantErr && !errors.Is(err, core.ErrJSONContract) && !errors.Is(err, core.ErrPrimitiveContract) {
				t.Fatalf("Validate() = %v, want error=%t", err, tc.wantErr)
			}
			if tc.wantErr {
				if _, err := json.Marshal(tc.row.Kind); tc.row.Kind.IsValid() == (err != nil) {
					t.Fatalf("kind %d marshal error = %v, want it to agree with IsValid", tc.row.Kind, err)
				}
				return
			}
			data, err := json.Marshal(tc.row)
			if err != nil {
				t.Fatal(err)
			}
			if tc.wire != "" && string(data) != tc.wire {
				t.Fatalf("wire = %s, want %s", data, tc.wire)
			}
			var got runprotocol.GoBenchmarkRow
			if err := json.Unmarshal(data, &got); err != nil || got.Validate() != nil || got.Kind != tc.row.Kind || (got.Measurement == nil) != (tc.row.Measurement == nil) || got.Measurement != nil && *got.Measurement != *tc.row.Measurement {
				t.Fatalf("roundtrip = (%+v, %v), want %+v", got, err, tc.row)
			}
		})
	}
	var kind runprotocol.GoBenchmarkRowKind
	if err := json.Unmarshal([]byte(`"fabricated"`), &kind); err == nil {
		t.Fatalf("unknown label decoded as %v, want refused", kind)
	}
}
