package runprotocol

import (
	"bytes"
	"errors"
	"iter"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/temporal"
)

func runReceiptCodec() jsonio.OwnedCodecPolicy {
	return jsonio.OwnedCodecPolicy{Members: jsonio.MembersDeclaredOnly, Root: jsonio.RootDeclaredFields, PointerNulls: jsonio.PointerNullRefused}
}
func receiptLength(t testing.TB, n uint64) core.ByteLength {
	t.Helper()
	v, err := core.NewByteLength(n)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func receiptLabel(t testing.TB, s string) TestLabel {
	t.Helper()
	v, err := NewTestLabel(s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func receiptPath(t testing.TB, s string) core.RelativePath {
	t.Helper()
	v, err := core.ParseRelativePath(s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func receiptArgument(t testing.TB, s string) RunArgument {
	t.Helper()
	v, err := NewRunArgument(s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func receiptFacts(values []RunFact) iter.Seq2[RunFact, error] {
	return func(yield func(RunFact, error) bool) {
		for _, v := range values {
			if !yield(v, nil) {
				return
			}
		}
	}
}

// These are typed schema fixtures. They do not claim that a native command ran.
func compactReceiptFixture(t testing.TB) (RunReceipt, []RunFact, []byte) {
	t.Helper()
	digest := core.SHA256Of([]byte("captured source fixture"))
	tool, err := core.ParseAbsolutePath("/native/go")
	if err != nil {
		t.Fatal(err)
	}
	zero := int64(0)
	pass := OutcomePassed
	suite := receiptLabel(t, "example.org/project/pkg")
	name := receiptLabel(t, "TestNative/leaf")
	facts := []RunFact{
		{Ordinal: 1, Kind: RunFactInvocation, Invocation: &RunInvocation{Tool: tool, Arguments: []RunArgument{receiptArgument(t, "test"), receiptArgument(t, "-run=^TestNative/leaf$"), receiptArgument(t, "./pkg")}, Toolchain: fixtureText(t, "go version fixture"), Source: digest, Packages: 1, Threads: 1, MemoryPerProcess: receiptLength(t, 1<<20)}},
		{Ordinal: 2, Kind: RunFactExecution, Execution: &process.ResultObservation{ExitCode: 0, StdinBytes: receiptLength(t, 0), StdoutBytes: receiptLength(t, 100), StderrBytes: receiptLength(t, 0)}},
		{Ordinal: 3, Kind: RunFactResult, Result: &RunResult{Kind: RunResultTest, Suite: suite, Name: &name, Outcome: &pass}},
		{Ordinal: 4, Kind: RunFactResult, Result: &RunResult{Kind: RunResultPackage, Suite: suite, Outcome: &pass}},
	}
	var body bytes.Buffer
	sequence, err := WriteRunFacts(t.Context(), RunFactWriteRequest{Destination: &body, Facts: receiptFacts(facts)})
	if err != nil {
		t.Fatal(err)
	}
	seed, err := ParseRunShuffleSeed("0")
	if err != nil {
		t.Fatal(err)
	}
	receipt := RunReceipt{SchemaVersion: RunReceiptSchemaVersion, Run: fixtureIdentifier(t, "native-run"), Producer: fixtureIdentifier(t, "anvil"), Caller: fixtureIdentifier(t, "codex"), Kind: RunKindTest,
		Source: RunSource{Kind: RunSourceSnapshot, Repository: fixtureRepository(t, "github.com/example/project"), Commit: fixtureCommit(t, strings.Repeat("a", 40)), Captured: &RunSourceCapture{Digest: digest, Files: 1, Bytes: receiptLength(t, 100)}},
		Scope:  RunScope{Kind: RunScopePackage, Targets: []core.RelativePath{receiptPath(t, "pkg")}},
		Worker: &RunWorker{Name: fixtureIdentifier(t, "native-worker"), Platform: core.Platform{OperatingSystem: core.OperatingSystemLinux, Architecture: core.CPUArchitectureAMD64}, Executable: core.SHA256Of([]byte("producer")), ObservedAt: temporal.InstantFromNanoseconds(1), LogicalCPUs: 2, TotalMemoryBytes: receiptLength(t, 1<<30), ScratchTotalBytes: receiptLength(t, 1<<30)},
		Budget: &RunBudget{CPU: 1, MemoryBytes: receiptLength(t, 1<<20), ScratchBytes: receiptLength(t, 1<<20), Timeout: fixtureDuration(t, 1)}, Seed: &seed,
		StartedAt: temporal.InstantFromNanoseconds(1), CompletedAt: temporal.InstantFromNanoseconds(2), Summary: RunSummary{StreamDecoded: true, ExitCode: &zero, Tests: RunCounts{StartedCount: 1, Passed: 1}, Packages: RunCounts{StartedCount: 1, Passed: 1}}, Facts: sequence, Outcome: OutcomePassed, Terminal: TerminalCompleted}
	if err := receipt.Validate(); err != nil {
		t.Fatal(err)
	}
	return receipt, facts, body.Bytes()
}
func runReceiptFixture(t testing.TB) RunReceipt {
	t.Helper()
	r, _, _ := compactReceiptFixture(t)
	return r
}

func TestRunReceiptLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		edit    func(*RunReceipt)
		wantErr error
	}{
		{name: "snapshot retains actual captured tree", edit: func(*RunReceipt) {}},
		{name: "commit retains distinct source kind", edit: func(r *RunReceipt) { r.Source.Kind = RunSourceCommit }},
		{name: "maximum native seed remains exact", edit: func(r *RunReceipt) {
			v, err := ParseRunShuffleSeed("9223372036854775807")
			if err != nil {
				t.Fatal(err)
			}
			r.Seed = &v
		}},
		{name: "minimum native seed remains exact", edit: func(r *RunReceipt) {
			v, err := ParseRunShuffleSeed("-9223372036854775808")
			if err != nil {
				t.Fatal(err)
			}
			r.Seed = &v
		}},
		{name: "absent seed does not invent zero", edit: func(r *RunReceipt) { r.Seed = nil }},
		{name: "retry retains separate original run", edit: func(r *RunReceipt) {
			r.Retry = &RunRetry{Run: fixtureIdentifier(t, "failed-original"), PassedInIsolation: true}
		}},
		{name: "source refusal has no invented execution", edit: func(r *RunReceipt) {
			r.Source.Captured = nil
			r.Worker = nil
			r.Budget = nil
			r.Seed = nil
			r.Summary = RunSummary{}
			r.Facts = RunFactSequence{Bytes: receiptLength(t, 0), Digest: core.SHA256Of(nil)}
			r.Outcome = OutcomeNonAccepting
		}},
		{name: "cancellation before source remains cancellation", edit: func(r *RunReceipt) {
			r.Source.Captured = nil
			r.Worker = nil
			r.Budget = nil
			r.Summary = RunSummary{}
			r.Facts = RunFactSequence{Bytes: receiptLength(t, 0), Digest: core.SHA256Of(nil)}
			r.Outcome = OutcomeCancelled
			r.Terminal = TerminalCancelled
		}},
		{name: "unobserved phase stays absent", edit: func(r *RunReceipt) { r.Timing = RunTiming{} }},
		{name: "measured zero phase stays present", edit: func(r *RunReceipt) { zero := temporal.Duration{}; r.Timing.Preparation = &zero }},
		{name: "sampling absence does not claim peak", edit: func(r *RunReceipt) { r.Summary.Memory = nil }},
		{name: "unsupported future schema refuses", edit: func(r *RunReceipt) { r.SchemaVersion++ }, wantErr: core.ErrRunProtocolContract},
		{name: "zero schema cannot borrow current shape", edit: func(r *RunReceipt) { r.SchemaVersion = 0 }, wantErr: core.ErrRunProtocolContract},
		{name: "missing run attribution refuses", edit: func(r *RunReceipt) { r.Run = Identifier{} }, wantErr: core.ErrRunProtocolContract},
		{name: "missing producer attribution refuses", edit: func(r *RunReceipt) { r.Producer = Identifier{} }, wantErr: core.ErrRunProtocolContract},
		{name: "missing caller attribution refuses", edit: func(r *RunReceipt) { r.Caller = Identifier{} }, wantErr: core.ErrRunProtocolContract},
		{name: "unknown native workload refuses", edit: func(r *RunReceipt) { r.Kind = RunKindUnknown }, wantErr: core.ErrRunProtocolContract},
		{name: "future native workload refuses", edit: func(r *RunReceipt) { r.Kind = RunKind(255) }, wantErr: core.ErrRunProtocolContract},
		{name: "unknown source representation refuses", edit: func(r *RunReceipt) { r.Source.Kind = RunSourceUnknown }, wantErr: core.ErrRunProtocolContract},
		{name: "missing repository refuses source binding", edit: func(r *RunReceipt) { r.Source.Repository = RepositoryIdentity{} }, wantErr: core.ErrRunProtocolContract},
		{name: "missing committed revision refuses", edit: func(r *RunReceipt) { r.Source.Commit = core.BuildCommit{} }, wantErr: core.ErrRunProtocolContract},
		{name: "native execution cannot omit actual source", edit: func(r *RunReceipt) { r.Source.Captured = nil }, wantErr: core.ErrRunProtocolConflict},
		{name: "captured source needs actual digest", edit: func(r *RunReceipt) { r.Source.Captured.Digest = core.SHA256Digest{} }, wantErr: core.ErrRunProtocolContract},
		{name: "captured source cannot have no files", edit: func(r *RunReceipt) { r.Source.Captured.Files = 0 }, wantErr: core.ErrRunProtocolContract},
		{name: "empty scope cannot claim execution", edit: func(r *RunReceipt) { r.Scope.Targets = nil }, wantErr: core.ErrRunProtocolContract},
		{name: "single package scope cannot hide second target", edit: func(r *RunReceipt) { r.Scope.Targets = append(r.Scope.Targets, receiptPath(t, "other")) }, wantErr: core.ErrRunProtocolContract},
		{name: "selected scope refuses duplicate targets", edit: func(r *RunReceipt) {
			r.Scope.Kind = RunScopeSelected
			r.Scope.Targets = append(r.Scope.Targets, r.Scope.Targets[0])
		}, wantErr: core.ErrRunProtocolConflict},
		{name: "unknown selected scope refuses", edit: func(r *RunReceipt) { r.Scope.Kind = RunScopeUnknown }, wantErr: core.ErrRunProtocolContract},
		{name: "native execution requires actual worker", edit: func(r *RunReceipt) { r.Worker = nil }, wantErr: core.ErrRunProtocolConflict},
		{name: "worker cannot invent zero cpu capacity", edit: func(r *RunReceipt) { r.Worker.LogicalCPUs = 0 }, wantErr: core.ErrRunProtocolContract},
		{name: "worker cannot invent zero memory capacity", edit: func(r *RunReceipt) { r.Worker.TotalMemoryBytes = receiptLength(t, 0) }, wantErr: core.ErrRunProtocolContract},
		{name: "worker needs executable binding", edit: func(r *RunReceipt) { r.Worker.Executable = core.SHA256Digest{} }, wantErr: core.ErrRunProtocolContract},
		{name: "worker needs actual platform", edit: func(r *RunReceipt) { r.Worker.Platform = core.Platform{} }, wantErr: core.ErrRunProtocolContract},
		{name: "native execution requires effective budget", edit: func(r *RunReceipt) { r.Budget = nil }, wantErr: core.ErrRunProtocolConflict},
		{name: "effective cpu cannot be zero", edit: func(r *RunReceipt) { r.Budget.CPU = 0 }, wantErr: core.ErrRunProtocolContract},
		{name: "effective memory cannot be zero", edit: func(r *RunReceipt) { r.Budget.MemoryBytes = receiptLength(t, 0) }, wantErr: core.ErrRunProtocolContract},
		{name: "effective deadline cannot be zero", edit: func(r *RunReceipt) { r.Budget.Timeout = temporal.Duration{} }, wantErr: core.ErrRunProtocolContract},
		{name: "unset seed is not observed zero", edit: func(r *RunReceipt) { r.Seed = &RunShuffleSeed{} }, wantErr: core.ErrRunProtocolContract},
		{name: "completion cannot precede start", edit: func(r *RunReceipt) { r.CompletedAt = temporal.InstantFromNanoseconds(0) }, wantErr: core.ErrRunProtocolConflict},
		{name: "admission cannot exceed preparation", edit: func(r *RunReceipt) {
			a := fixtureDuration(t, 2)
			p := fixtureDuration(t, 1)
			r.Timing.Admission = &a
			r.Timing.Preparation = &p
		}, wantErr: core.ErrRunProtocolConflict},
		{name: "native build timing needs observed preparation", edit: func(r *RunReceipt) { v := fixtureDuration(t, 1); r.Timing.BuildAndRun = &v }, wantErr: core.ErrRunProtocolConflict},
		{name: "test terminal counts cannot exceed starts", edit: func(r *RunReceipt) { r.Summary.Tests.Failed = 1 }, wantErr: core.ErrRunProtocolConflict},
		{name: "package terminal count cannot overflow", edit: func(r *RunReceipt) { r.Summary.Packages.Passed = ^uint64(0) }, wantErr: core.ErrRunProtocolConflict},
		{name: "passed outcome cannot hide failed test", edit: func(r *RunReceipt) { r.Summary.Tests.Passed = 0; r.Summary.Tests.Failed = 1 }, wantErr: core.ErrRunProtocolConflict},
		{name: "passed outcome cannot hide build refusal", edit: func(r *RunReceipt) { r.Summary.BuildFailed = true }, wantErr: core.ErrRunProtocolConflict},
		{name: "passed outcome requires complete stream", edit: func(r *RunReceipt) { r.Summary.StreamDecoded = false }, wantErr: core.ErrRunProtocolConflict},
		{name: "passed outcome requires observed native exit", edit: func(r *RunReceipt) { r.Summary.ExitCode = nil }, wantErr: core.ErrRunProtocolConflict},
		{name: "passed outcome cannot claim failed exit", edit: func(r *RunReceipt) { v := int64(1); r.Summary.ExitCode = &v }, wantErr: core.ErrRunProtocolConflict},
		{name: "wrong terminal cannot conceal passed outcome", edit: func(r *RunReceipt) { r.Terminal = TerminalCancelled }, wantErr: core.ErrRunProtocolConflict},
		{name: "test run cannot invent benchmark phase", edit: func(r *RunReceipt) { r.Summary.Benchmarks = &RunBenchmarkCounts{} }, wantErr: core.ErrRunProtocolConflict},
		{name: "test run cannot invent mutation fuzz phase", edit: func(r *RunReceipt) { v := uint64(0); r.Summary.Fuzz = &RunFuzzCounts{Executions: &v} }, wantErr: core.ErrRunProtocolConflict},
		{name: "same run cannot be its own retry", edit: func(r *RunReceipt) { r.Retry = &RunRetry{Run: r.Run} }, wantErr: core.ErrRunProtocolConflict},
		{name: "external empty fuzz group refuses instead of normalizing unknown", edit: func(r *RunReceipt) {
			r.Kind = RunKindFuzz
			r.Outcome = OutcomeSkipped
			r.Summary.Fuzz = &RunFuzzCounts{}
		}, wantErr: core.ErrRunProtocolContract},
		{name: "empty sequence cannot retain bytes", edit: func(r *RunReceipt) { r.Facts.Count = 0 }, wantErr: core.ErrRunProtocolConflict},
		{name: "nonempty sequence needs content digest", edit: func(r *RunReceipt) { r.Facts.Digest = core.SHA256Digest{} }, wantErr: core.ErrRunProtocolContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := runReceiptFixture(t)
			tc.edit(&r)
			err := r.Validate()
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("RunReceipt.Validate = %v, want %v", err, tc.wantErr)
			}
			encoded, encodeErr := jsonio.MarshalValidated(r, runReceiptCodec())
			if tc.wantErr != nil {
				if !errors.Is(encodeErr, tc.wantErr) || len(encoded) != 0 {
					t.Fatalf("refused header encoding = (%d,%v), want zero/%v", len(encoded), encodeErr, tc.wantErr)
				}
				return
			}
			if encodeErr != nil {
				t.Fatal(encodeErr)
			}
			round, decodeErr := jsonio.DecodeOwned[RunReceipt](encoded, runReceiptCodec())
			if decodeErr != nil {
				t.Fatal(decodeErr)
			}
			second, err := jsonio.MarshalValidated(round, runReceiptCodec())
			if err != nil || !bytes.Equal(encoded, second) {
				t.Fatalf("canonical header roundtrip = (%d,%v), want %d identical bytes", len(second), err, len(encoded))
			}
		})
	}
}
