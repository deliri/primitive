package runprotocol

import (
	"bytes"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"testing"
)

// Every row changes only native evidence or the claim it is meant to support;
// the compact bytes are honestly resealed, so a stale digest cannot reject it.
func TestCompactReceiptExecutedFactsConsistency(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		edit func(testing.TB, *RunReceipt, []RunFact) []RunFact
	}{
		{"native nonzero exit cannot support claimed success", func(_ testing.TB, _ *RunReceipt, f []RunFact) []RunFact { f[1].Execution.ExitCode = 1; return f }},
		{"completed execution cannot omit actual command", func(_ testing.TB, _ *RunReceipt, f []RunFact) []RunFact { return f[1:] }},
		{"empty test evidence cannot claim a passing target", func(_ testing.TB, r *RunReceipt, f []RunFact) []RunFact {
			r.Summary.Tests = RunCounts{}
			return append(f[:2], f[3:]...)
		}},
		{"missing benchmark measurements cannot claim measured pass", func(t testing.TB, r *RunReceipt, f []RunFact) []RunFact {
			r.Kind = RunKindBenchmark
			r.Summary.Tests = RunCounts{StartedCount: 1}
			r.Summary.Benchmarks = &RunBenchmarkCounts{Rows: 1}
			f[2].Result = &RunResult{Kind: RunResultBenchmark, Suite: receiptLabel(t, "example.org/project/pkg"), Benchmark: &GoBenchmarkRow{Kind: GoBenchmarkRowMetricsMissing}}
			return f
		}},
		{"zero fuzz executions cannot claim measured campaign pass", func(t testing.TB, r *RunReceipt, f []RunFact) []RunFact {
			r.Kind = RunKindFuzz
			zero := uint64(0)
			r.Summary.Fuzz = &RunFuzzCounts{Executions: &zero}
			f[2].Result.Kind = RunResultFuzz
			name := receiptLabel(t, "FuzzNative")
			f[2].Result.Name = &name
			return f
		}},
		{"unobserved fuzz executions cannot claim measured campaign pass", func(_ testing.TB, r *RunReceipt, f []RunFact) []RunFact {
			r.Kind = RunKindFuzz
			r.Summary.Fuzz = nil
			return f
		}},
		{"fuzz executions cannot borrow an ordinary test terminal", func(_ testing.TB, r *RunReceipt, f []RunFact) []RunFact {
			r.Kind = RunKindFuzz
			count := uint64(10)
			r.Summary.Fuzz = &RunFuzzCounts{Executions: &count}
			return f
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r, f, _ := compactReceiptFixture(t)
			f = tc.edit(t, &r, f)
			for i := range f {
				f[i].Ordinal = uint64(i + 1)
			}
			var body bytes.Buffer
			sequence, err := WriteRunFacts(t.Context(), RunFactWriteRequest{Destination: &body, Facts: receiptFacts(f)})
			if err != nil {
				t.Fatal(err)
			}
			r.Facts = sequence
			err = VerifyRunFacts(t.Context(), RunFactStreamRequest{Source: bytes.NewReader(body.Bytes()), Receipt: r})
			if !errors.Is(err, core.ErrRunProtocolConflict) {
				t.Fatalf("resealed contradictory execution = %v, want conflict", err)
			}
		})
	}
}

func TestCompactReceiptPartialExecutionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		edit    func(*RunReceipt, []RunFact) []RunFact
		wantErr error
	}{
		{"cancelled process retains command without aggregate exit", func(_ *RunReceipt, f []RunFact) []RunFact { return f }, nil},
		{"cancelled process cannot discard actual command", func(_ *RunReceipt, f []RunFact) []RunFact { return f[1:] }, core.ErrRunProtocolConflict},
		{"pre execution cancellation has no invented command", func(r *RunReceipt, _ []RunFact) []RunFact {
			r.Source.Captured = nil
			r.Worker = nil
			r.Budget = nil
			return nil
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r, facts, _ := compactReceiptFixture(t)
			r.Outcome = OutcomeCancelled
			r.Terminal = TerminalCancelled
			r.Summary = RunSummary{}
			facts = tc.edit(&r, facts[:2])
			for i := range facts {
				facts[i].Ordinal = uint64(i + 1)
			}
			var body bytes.Buffer
			sequence, err := WriteRunFacts(t.Context(), RunFactWriteRequest{Destination: &body, Facts: receiptFacts(facts)})
			if err != nil {
				t.Fatal(err)
			}
			r.Facts = sequence
			err = VerifyRunFacts(t.Context(), RunFactStreamRequest{Source: bytes.NewReader(body.Bytes()), Receipt: r})
			if !errors.Is(err, tc.wantErr) || r.Summary.ExitCode != nil {
				t.Fatalf("partial cancelled execution = (%v,%v), want %v/unknown aggregate exit", err, r.Summary.ExitCode, tc.wantErr)
			}
		})
	}
}
