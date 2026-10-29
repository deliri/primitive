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
			if row.wantErr == nil {
				native, nativeErr := json.Marshal(row.result)
				if nativeErr != nil {
					t.Fatalf("native encoder before codec validation = %s/%v, want suite omission supported by jsonv2 itself", native, nativeErr)
				}
				var presence struct {
					Suite *TestLabel `json:"suite,omitempty"`
				}
				if err := json.Unmarshal(native, &presence); err != nil || (presence.Suite != nil) != row.wantSuite {
					t.Fatalf("native suite presence before codec validation = %s/%v, want present=%v", native, err, row.wantSuite)
				}
			}
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
				scalar, err := known.MarshalJSON()
				if err != nil || !bytes.Contains(encoded, append([]byte(`"suite":`), scalar...)) {
					t.Fatalf("known suite scalar wire = %s/%v, want original suite scalar %s", encoded, err, scalar)
				}
			}

		})
	}
}

// These are compiler-owned wire-policy fixtures, not a claim that a native
// benchmark ran. Actual producer attribution remains Anvil's owning contract.
func TestBenchmarkUnobservedSuiteStreamRoundTrip(t *testing.T) {
	t.Parallel()
	receipt, facts, _ := compactReceiptFixture(t)
	facts = facts[:3]
	facts[2].Result = &RunResult{Kind: RunResultBenchmark, Benchmark: &GoBenchmarkRow{Kind: GoBenchmarkRowUnattributed}}
	receipt.Kind = RunKindBenchmark
	receipt.Outcome = OutcomeSkipped
	receipt.Seed = nil
	receipt.Summary.Tests = RunCounts{}
	receipt.Summary.Packages = RunCounts{}
	receipt.Summary.Benchmarks = &RunBenchmarkCounts{Rows: 1}
	var stream bytes.Buffer
	sequence, err := WriteRunFacts(t.Context(), RunFactWriteRequest{Destination: &stream, Facts: receiptFacts(facts)})
	if err != nil {
		t.Fatal(err)
	}
	receipt.Facts = sequence
	if err := VerifyRunFacts(t.Context(), RunFactStreamRequest{Source: bytes.NewReader(stream.Bytes()), Receipt: receipt}); err != nil {
		t.Fatalf("unobserved benchmark stream = %v, want valid native facts without fabricated suite", err)
	}
	count := 0
	for fact, err := range RunFacts(t.Context(), RunFactStreamRequest{Source: bytes.NewReader(stream.Bytes()), Receipt: receipt}) {
		if err != nil {
			t.Fatal(err)
		}
		count++
		if fact.Result != nil && (fact.Result.Suite != (TestLabel{}) || fact.Result.PackagePath != nil || fact.Result.Benchmark.Kind != GoBenchmarkRowUnattributed) {
			t.Fatalf("stream attribution = %+v, want honest absence", fact.Result)
		}
	}
	if count != 3 {
		t.Fatalf("stream fact count = %d, want 3", count)
	}
}
