package runprotocol

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
)

func TestRunFactMeasuredKindsLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		edit    func(testing.TB, *RunReceipt, []RunFact)
		wantErr error
	}{
		{"benchmark retains exact standard measurements", func(t testing.TB, r *RunReceipt, f []RunFact) {
			name := receiptLabel(t, "BenchmarkNative-1")
			nominal, err := NewName(name.String())
			if err != nil {
				t.Fatal(err)
			}
			r.Kind = RunKindBenchmark
			r.Seed = nil
			r.Summary.Tests = RunCounts{StartedCount: 1}
			r.Summary.Benchmarks = &RunBenchmarkCounts{Rows: 1, Measured: 1}
			f[2].Result = &RunResult{Kind: RunResultBenchmark, Suite: receiptLabel(t, "example.org/project/pkg"), Name: &name, Benchmark: &GoBenchmarkRow{Kind: GoBenchmarkRowMeasured, Measurement: &BenchmarkMeasurement{Name: nominal, Iterations: 10, NanosecondsPerOp: DecimalMeasurement{Coefficient: 1}, BytesPerOp: 0, AllocationsPerOp: 0}}}
		}, nil},
		{"benchmark row without metrics remains explicitly thin", func(t testing.TB, r *RunReceipt, f []RunFact) {
			r.Kind = RunKindBenchmark
			r.Summary.Tests = RunCounts{StartedCount: 1}
			r.Summary.Benchmarks = &RunBenchmarkCounts{Rows: 1}
			r.Outcome = OutcomeSkipped
			f[2].Result = &RunResult{Kind: RunResultBenchmark, Suite: receiptLabel(t, "example.org/project/pkg"), Benchmark: &GoBenchmarkRow{Kind: GoBenchmarkRowMetricsMissing}}
		}, nil},
		{"benchmark summary cannot invent measured rows", func(t testing.TB, r *RunReceipt, f []RunFact) {
			r.Kind = RunKindBenchmark
			r.Summary.Tests = RunCounts{StartedCount: 1}
			r.Summary.Benchmarks = &RunBenchmarkCounts{Rows: 1, Measured: 1}
			f[2].Result = &RunResult{Kind: RunResultBenchmark, Suite: receiptLabel(t, "example.org/project/pkg"), Benchmark: &GoBenchmarkRow{Kind: GoBenchmarkRowMetricsMissing}}
		}, core.ErrRunProtocolConflict},
		{"measured fuzz preserves a real observed zero", func(t testing.TB, r *RunReceipt, f []RunFact) {
			r.Kind = RunKindFuzz
			zero := uint64(0)
			r.Summary.Fuzz = &RunFuzzCounts{Executions: &zero}
			r.Outcome = OutcomeSkipped
			f[2].Result.Kind = RunResultFuzz
			name := receiptLabel(t, "FuzzNative")
			f[2].Result.Name = &name
		}, nil},
		{"unobserved fuzz counters remain absent", func(_ testing.TB, r *RunReceipt, f []RunFact) {
			r.Kind = RunKindFuzz
			r.Summary.Fuzz = nil
			r.Outcome = OutcomeSkipped
			f[2].Result.Kind = RunResultFuzz
			name := receiptLabel(t, "FuzzNative")
			f[2].Result.Name = &name
		}, nil},
		{"measured fuzz campaign retains actual executions", func(t testing.TB, r *RunReceipt, f []RunFact) {
			r.Kind = RunKindFuzz
			executions := uint64(10)
			r.Summary.Fuzz = &RunFuzzCounts{Executions: &executions}
			f[2].Result.Kind = RunResultFuzz
			name := receiptLabel(t, "FuzzNative")
			f[2].Result.Name = &name
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r, f, _ := compactReceiptFixture(t)
			tc.edit(t, &r, f)
			var body bytes.Buffer
			seq, err := WriteRunFacts(t.Context(), RunFactWriteRequest{Destination: &body, Facts: receiptFacts(f)})
			if err != nil {
				t.Fatal(err)
			}
			r.Facts = seq
			err = VerifyRunFacts(t.Context(), RunFactStreamRequest{Source: bytes.NewReader(body.Bytes()), Receipt: r})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("measured compact stream = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr == nil {
				header, err := jsonio.MarshalValidated(r, runReceiptCodec())
				if err != nil {
					t.Fatal(err)
				}
				round, err := jsonio.DecodeOwned[RunReceipt](header, runReceiptCodec())
				if err != nil {
					t.Fatal(err)
				}
				if r.Summary.Fuzz != nil {
					if round.Summary.Fuzz == nil {
						t.Fatalf("fuzz measurements after roundtrip = %+v, want original %+v", round.Summary.Fuzz, r.Summary.Fuzz)
					}
					if (round.Summary.Fuzz.Executions == nil) != (r.Summary.Fuzz.Executions == nil) {
						t.Fatalf("fuzz observation presence = %+v, want %+v", round.Summary.Fuzz, r.Summary.Fuzz)
					}
					if r.Summary.Fuzz.Executions != nil && *round.Summary.Fuzz.Executions != *r.Summary.Fuzz.Executions {
						t.Fatalf("fuzz execution count = %d, want %d", *round.Summary.Fuzz.Executions, *r.Summary.Fuzz.Executions)
					}
				}
			}
		})
	}
}

// Every external compact document and stream door is reached from a canonical
// typed producer seed. Accepted bytes must preserve exact typed facts, binding,
// counts and canonical representation; rejection returns a stable identity.
func FuzzCompactReceiptJSONAndStreamSemanticClosure(f *testing.F) {
	receipt, facts, body := compactReceiptFixture(f)
	header, err := jsonio.MarshalValidated(receipt, runReceiptCodec())
	if err != nil {
		f.Fatal(err)
	}
	fact, err := jsonio.MarshalValidated(facts[2], runReceiptCodec())
	if err != nil {
		f.Fatal(err)
	}
	result, err := jsonio.MarshalValidated(*facts[2].Result, runReceiptCodec())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(header, uint8(0))
	f.Add(fact, uint8(1))
	f.Add(result, uint8(2))
	f.Add(body, uint8(3))
	reordered := append([]RunFact(nil), facts...)
	reordered[0], reordered[1] = reordered[1], reordered[0]
	for i := range reordered {
		reordered[i].Ordinal = uint64(i + 1)
	}
	var reorderedBody bytes.Buffer
	if _, err := WriteRunFacts(f.Context(), RunFactWriteRequest{Destination: &reorderedBody, Facts: receiptFacts(reordered)}); err != nil {
		f.Fatal(err)
	}
	f.Add(reorderedBody.Bytes(), uint8(3))
	f.Add([]byte("null"), uint8(0))
	f.Add([]byte{}, uint8(3))
	f.Fuzz(func(t *testing.T, data []byte, door uint8) {
		switch door % 4 {
		case 0:
			got, err := jsonio.DecodeOwned[RunReceipt](data, runReceiptCodec())
			if err != nil {
				if !errors.Is(err, core.ErrJSONContract) || got.SchemaVersion != 0 || got.Run != (Identifier{}) || len(got.Scope.Targets) != 0 {
					t.Fatalf("rejected header = (%+v,%v), want zero/JSON contract", got, err)
				}
				return
			}
			first, err := jsonio.MarshalValidated(got, runReceiptCodec())
			if err != nil {
				t.Fatal(err)
			}
			second, err := jsonio.DecodeOwned[RunReceipt](first, runReceiptCodec())
			if err != nil {
				t.Fatal(err)
			}
			last, err := jsonio.MarshalValidated(second, runReceiptCodec())
			if err != nil || !bytes.Equal(first, last) {
				t.Fatalf("header canonical closure = (%d,%v), want %d identical bytes", len(last), err, len(first))
			}
		case 1:
			got, err := jsonio.DecodeOwned[RunFact](data, runReceiptCodec())
			if err != nil {
				if !errors.Is(err, core.ErrJSONContract) || got != (RunFact{}) {
					t.Fatalf("rejected fact = (%+v,%v), want zero/JSON contract", got, err)
				}
				return
			}
			first, err := jsonio.MarshalValidated(got, runReceiptCodec())
			if err != nil {
				t.Fatal(err)
			}
			second, err := jsonio.DecodeOwned[RunFact](first, runReceiptCodec())
			if err != nil {
				t.Fatal(err)
			}
			last, err := jsonio.MarshalValidated(second, runReceiptCodec())
			if err != nil || !bytes.Equal(first, last) {
				t.Fatalf("fact canonical closure = (%d,%v), want %d identical bytes", len(last), err, len(first))
			}
		case 2:
			got, err := jsonio.DecodeOwned[RunResult](data, runReceiptCodec())
			if err != nil {
				if !errors.Is(err, core.ErrJSONContract) || got != (RunResult{}) {
					t.Fatalf("rejected result = (%+v,%v), want zero/JSON contract", got, err)
				}
				return
			}
			first, err := jsonio.MarshalValidated(got, runReceiptCodec())
			if err != nil {
				t.Fatal(err)
			}
			second, err := jsonio.DecodeOwned[RunResult](first, runReceiptCodec())
			if err != nil {
				t.Fatal(err)
			}
			last, err := jsonio.MarshalValidated(second, runReceiptCodec())
			if err != nil || !bytes.Equal(first, last) {
				t.Fatalf("result canonical closure = (%d,%v), want %d identical bytes", len(last), err, len(first))
			}
		case 3:
			r := runReceiptFixture(t)
			r.Facts.Bytes = receiptLength(t, uint64(len(data)))
			r.Facts.Digest = core.SHA256Of(data)
			err := VerifyRunFacts(t.Context(), RunFactStreamRequest{Source: bytes.NewReader(data), Receipt: r})
			if err != nil {
				if !errors.Is(err, core.ErrRunProtocolContract) && !errors.Is(err, core.ErrRunProtocolConflict) {
					t.Fatalf("refused compact stream = %v, want contract/conflict", err)
				}
				return
			}
			var decoded []RunFact
			for fact, err := range RunFacts(t.Context(), RunFactStreamRequest{Source: bytes.NewReader(data), Receipt: r}) {
				if err != nil {
					t.Fatal(err)
				}
				if len(decoded) >= 4 {
					t.Fatalf("accepted compact stream count exceeds %d, want exactly four facts", len(decoded))
				}
				decoded = append(decoded, fact)
			}
			// Accepted input has exactly the fixed four-fact seed denominator, so this
			// oracle is bounded independently of arbitrary rejected input length.
			invocationObserved := false
			for _, fact := range decoded {
				if fact.Invocation != nil {
					invocationObserved = true
					if fact.Invocation.Source != r.Source.Captured.Digest {
						t.Fatalf("accepted compact invocation source = %v, want %v", fact.Invocation.Source, r.Source.Captured.Digest)
					}
				}
			}
			if len(decoded) != 4 || !invocationObserved {
				t.Fatalf("accepted compact source = (%d facts,%t invocation), want four with observed source binding", len(decoded), invocationObserved)
			}
			var canonical bytes.Buffer
			seq, err := WriteRunFacts(t.Context(), RunFactWriteRequest{Destination: &canonical, Facts: receiptFacts(decoded)})
			if err != nil || seq != r.Facts || !bytes.Equal(canonical.Bytes(), data) {
				t.Fatalf("accepted compact canonical binding = (%+v,%v), want exact declared sequence", seq, err)
			}
		}
	})
}
