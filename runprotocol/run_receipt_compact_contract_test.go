package runprotocol

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"testing"

	"github.com/deliri/primitive/v2026/jsonio"
)

// The shared durable record must remain usable after native scratch is gone.
// This observes the actual typed encoder, not a second receipt constructor.
func TestRunReceiptExcludesRawArtifactDependencies(t *testing.T) {
	t.Parallel()
	receipt := runReceiptFixture(t)
	encoded, err := jsonio.MarshalValidated(receipt, runReceiptCodec())
	if err != nil {
		t.Fatal(err)
	}
	var dependencies struct {
		Invocation *ArtifactReference `json:"invocation,omitempty"`
		Execution  *ArtifactReference `json:"execution,omitempty"`
		Results    *ArtifactReference `json:"results,omitempty"`
	}
	if err := json.Unmarshal(encoded, &dependencies); err != nil {
		t.Fatal(err)
	}
	if dependencies.Invocation != nil || dependencies.Execution != nil || dependencies.Results != nil {
		t.Fatalf("encoded receipt raw dependencies = %+v, want none after joined scratch discard", dependencies)
	}
}

func TestBenchmarkResultPreservesUnobservedSuite(t *testing.T) {
	t.Parallel()
	name := receiptLabel(t, "BenchmarkNative-1")
	nominal, err := NewName(name.String())
	if err != nil {
		t.Fatal(err)
	}
	measurement := BenchmarkMeasurement{Name: nominal, Iterations: 10, NanosecondsPerOp: DecimalMeasurement{Coefficient: 1}}
	known := receiptLabel(t, "example.org/project/pkg")
	path := receiptPath(t, "pkg")
	pass := OutcomePassed
	for _, row := range []struct {
		name      string
		result    RunResult
		wantErr   error
		wantSuite bool
	}{
		{name: "native unattributed row omits unobserved suite", result: RunResult{Kind: RunResultBenchmark, Benchmark: &GoBenchmarkRow{Kind: GoBenchmarkRowUnattributed}}},
		{name: "missing metrics retains honest unobserved suite", result: RunResult{Kind: RunResultBenchmark, Benchmark: &GoBenchmarkRow{Kind: GoBenchmarkRowMetricsMissing}}},
		{name: "measured row can lack suite without inventing one", result: RunResult{Kind: RunResultBenchmark, Name: &name, Benchmark: &GoBenchmarkRow{Kind: GoBenchmarkRowMeasured, Measurement: &measurement}}},
		{name: "known suite retains existing exact wire", result: RunResult{Kind: RunResultBenchmark, Suite: known, Name: &name, Benchmark: &GoBenchmarkRow{Kind: GoBenchmarkRowMeasured, Measurement: &measurement}}, wantSuite: true},
		{name: "package attribution cannot accompany unobserved suite", result: RunResult{Kind: RunResultBenchmark, PackagePath: &path, Benchmark: &GoBenchmarkRow{Kind: GoBenchmarkRowUnattributed}}, wantErr: core.ErrRunProtocolContract},
		{name: "test still requires actual suite", result: RunResult{Kind: RunResultTest, Name: &name, Outcome: &pass}, wantErr: core.ErrRunProtocolContract},
		{name: "package still requires actual suite", result: RunResult{Kind: RunResultPackage, Outcome: &pass}, wantErr: core.ErrRunProtocolContract},
		{name: "fuzz still requires actual suite", result: RunResult{Kind: RunResultFuzz, Name: &name, Outcome: &pass}, wantErr: core.ErrRunProtocolContract},
		{name: "nonzero malformed suite is never absence", result: RunResult{Kind: RunResultBenchmark, Suite: TestLabel{value: string([]byte{255})}, Benchmark: &GoBenchmarkRow{Kind: GoBenchmarkRowUnattributed}}, wantErr: core.ErrRunProtocolContract},
		{name: "missing benchmark row remains contradictory", result: RunResult{Kind: RunResultBenchmark}, wantErr: core.ErrRunProtocolConflict},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			err := row.result.Validate()
			if !errors.Is(err, row.wantErr) {
				t.Fatalf("benchmark attribution validation = %v, want %v", err, row.wantErr)
			}
			encoded, encodeErr := jsonio.MarshalValidated(row.result, runReceiptCodec())
			if row.wantErr != nil {
				if !errors.Is(encodeErr, row.wantErr) || len(encoded) != 0 {
					t.Fatalf("refused benchmark encoding = %q/%v, want empty and %v", encoded, encodeErr, row.wantErr)
				}
				return
			}
			if encodeErr != nil {
				t.Fatal(encodeErr)
			}
			var wire struct {
				Suite *TestLabel `json:"suite,omitempty"`
			}
			if err := json.Unmarshal(encoded, &wire); err != nil {
				t.Fatal(err)
			}
			if (wire.Suite != nil) != row.wantSuite {
				t.Fatalf("observed suite wire = %s, want presence=%v", encoded, row.wantSuite)
			}
			var direct RunResult
			if err := json.Unmarshal(encoded, &direct); err != nil || direct.Suite != row.result.Suite || direct.Kind != row.result.Kind {
				t.Fatalf("native JSON attribution = %+v/%v, want suite=%+v kind=%v", direct, err, row.result.Suite, row.result.Kind)
			}
			round, err := jsonio.DecodeOwned[RunResult](encoded, runReceiptCodec())
			if err != nil || round.Suite != row.result.Suite {
				t.Fatalf("canonical benchmark attribution = %+v/%v, want original suite", round, err)
			}
			second, err := jsonio.MarshalValidated(round, runReceiptCodec())
			if err != nil || !bytes.Equal(encoded, second) {
				t.Fatalf("canonical benchmark bytes = %q/%v, want %q/nil", second, err, encoded)
			}
			if row.wantSuite {
				const want = `{"kind":"benchmark","suite":"example.org/project/pkg","name":"BenchmarkNative-1","benchmark":{"kind":"measured","measurement":{"name":"BenchmarkNative-1","iterations":10,"ns_per_op":{"coefficient":1,"exponent":0},"bytes_per_op":0,"allocations_per_op":0}}}`
				// Decode an independent known-suite wire to compare its exact native field
				// presence; canonical bytes are separately stable across the round trip.
				var expected RunResult
				if err := json.Unmarshal([]byte(want), &expected); err != nil || expected.Suite != known {
					t.Fatalf("existing suite wire oracle = %+v/%v, want actual suite", expected, err)
				}
			}
		})
	}
}
