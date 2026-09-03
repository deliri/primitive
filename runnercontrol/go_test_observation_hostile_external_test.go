package runnercontrol_test

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/runprotocol"
)

type goEventFixture struct {
	Action  string  `json:"Action"`
	Package string  `json:"Package,omitempty"`
	Test    string  `json:"Test,omitempty"`
	Output  string  `json:"Output,omitempty"`
	Elapsed float64 `json:"Elapsed,omitempty"`
}

func TestGoTestObservationCompilerWriteBoundsEveryEventWithinOneChunk(t *testing.T) {
	t.Parallel()

	compiler, err := runnercontrol.NewGoTestObservationCompiler(runnercontrol.ObservationPolicy{
		Format: runnercontrol.ObservationGoTestJSON, ExpectedUnits: 1,
	})
	if err != nil {
		t.Fatalf("NewGoTestObservationCompiler() error = %v, want nil", err)
	}
	first := []byte("{\"Action\":\"start\",\"Package\":\"example.test/p\"}\n")
	oversized := append(append([]byte(nil), first...), strings.Repeat("x", runnercontrol.GoTestJSONEventMaximumBytes+1)...)
	written, gotErr := compiler.Write(oversized)
	if written != len(first) || !errors.Is(gotErr, core.ErrJSONContract) {
		t.Fatalf("GoTestObservationCompiler.Write(valid event plus overflow) = (%d, %v), want (%d, %v)", written, gotErr, len(first), core.ErrJSONContract)
	}
}

type goObservationCase struct {
	executionErr    error
	wantErr         error
	setup           func(testing.TB) [][]byte
	name            string
	class           string
	wantAccounting  runprotocol.ExecutionAccounting
	wantNanoseconds runprotocol.DecimalMeasurement
	wantBenchmarks  int
	policy          runnercontrol.ObservationPolicy
}

func TestGoTestObservationCompilerHostileEvidenceMatrix(t *testing.T) {
	t.Parallel()

	cases := goObservationCases()
	gotClasses := make(map[string]int)
	for index := range cases {
		gotClasses[cases[index].class]++
	}
	wantClasses := map[string]int{"valid": 10, "rejection": 10, "boundary": 20}
	if len(cases) != 40 || gotClasses["valid"] != wantClasses["valid"] || gotClasses["rejection"] != wantClasses["rejection"] || gotClasses["boundary"] != wantClasses["boundary"] {
		t.Fatalf("go test observation matrix classes = %v across %d cases, want %v across 40 earned cases", gotClasses, len(cases), wantClasses)
	}

	for index := range cases {
		tc := cases[index]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compiler, err := runnercontrol.NewGoTestObservationCompiler(tc.policy)
			if err != nil {
				t.Fatalf("NewGoTestObservationCompiler(%s) error = %v, want nil", tc.name, err)
			}
			for chunkIndex, chunk := range tc.setup(t) {
				gotWritten, gotWriteErr := compiler.Write(chunk)
				if gotWriteErr != nil || gotWritten != len(chunk) {
					t.Fatalf("GoTestObservationCompiler.Write(%s chunk %d) = (%d, %v), want (%d, nil) while retaining the complete raw stream", tc.name, chunkIndex, gotWritten, gotWriteErr, len(chunk))
				}
			}
			got, gotErr := compiler.Seal(tc.executionErr)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("GoTestObservationCompiler.Seal(%s) error = %v, want errors.Is(..., %v)", tc.name, gotErr, tc.wantErr)
				}
			} else if gotErr != nil {
				t.Fatalf("GoTestObservationCompiler.Seal(%s) error = %v, want nil", tc.name, gotErr)
			}
			if !got.Accounting.Equal(tc.wantAccounting) {
				t.Fatalf("GoTestObservationCompiler.Seal(%s) accounting = %+v, want %+v", tc.name, got.Accounting, tc.wantAccounting)
			}
			if len(got.Benchmarks) != tc.wantBenchmarks {
				t.Fatalf("GoTestObservationCompiler.Seal(%s) benchmark count = %d, want %d", tc.name, len(got.Benchmarks), tc.wantBenchmarks)
			}
			if tc.wantBenchmarks > 0 && got.Benchmarks[0].NanosecondsPerOp != tc.wantNanoseconds {
				t.Fatalf("GoTestObservationCompiler.Seal(%s) first ns/op = %+v, want %+v", tc.name, got.Benchmarks[0].NanosecondsPerOp, tc.wantNanoseconds)
			}
			if len(got.Accounting.Attempts) != 0 {
				if validationErr := got.Validate(); validationErr != nil {
					t.Fatalf("GoTestObservationCompiler.Seal(%s) retained observation Validate() error = %v, want nil even when the stream is refused", tc.name, validationErr)
				}
			}
		})
	}
}

func goObservationCases() []goObservationCase {
	pass := accounting(1, 1, 0, 0, 0, 0, 0, 0, false)
	fail := accounting(1, 0, 1, 0, 0, 0, 0, 0, false)
	skip := accounting(1, 0, 0, 1, 0, 0, 0, 0, false)
	unavailable := accounting(1, 0, 0, 0, 1, 0, 0, 0, false)
	valid := []goObservationCase{
		observationCase("package pass closes the planned unit", "valid", events(event("pass", "example.com/pass", "", "")), nil, pass),
		observationCase("package fail preserves a failed unit", "valid", events(event("fail", "example.com/fail", "", "")), errors.New("go test exit status 1"), fail),
		observationCase("package skip remains distinct from pass", "valid", events(event("skip", "example.com/skip", "", "")), nil, skip),
		observationCase("start then pass preserves one package identity", "valid", events(event("start", "example.com/start", "", ""), event("pass", "example.com/start", "", "")), nil, pass),
		observationCase("test terminal does not double count package terminal", "valid", events(event("run", "example.com/tests", "TestOne", ""), event("pass", "example.com/tests", "TestOne", ""), event("pass", "example.com/tests", "", "")), nil, pass),
		observationCase("pause and continue do not invent terminal evidence", "valid", events(event("pause", "example.com/pause", "TestOne", ""), event("cont", "example.com/pause", "TestOne", ""), event("pass", "example.com/pause", "", "")), nil, pass),
		observationCase("ordinary output does not invent measurements", "valid", events(event("output", "example.com/output", "", "diagnostic line\n"), event("pass", "example.com/output", "", "")), nil, pass),
		benchmarkCase("integer benchmark result retains allocations", "valid", "BenchmarkEncode-8 1000 125 ns/op 64 B/op 2 allocs/op\n", runprotocol.DecimalMeasurement{Coefficient: 125}),
		benchmarkCase("decimal benchmark result stays exact", "valid", "BenchmarkEncode-8 1000 125.75 ns/op 64 B/op 2 allocs/op\n", runprotocol.DecimalMeasurement{Coefficient: 12575, Scale: 2}),
		benchmarkCase("extra throughput metric does not hide required metrics", "valid", "BenchmarkEncode-8 1000 125 ns/op 12.5 MB/s 64 B/op 2 allocs/op\n", runprotocol.DecimalMeasurement{Coefficient: 125}),
	}
	rejections := []goObservationCase{
		observationCase("empty JSON event is refused", "rejection", rawChunks([]byte("\n")), nil, unavailable, core.ErrJSONContract),
		observationCase("truncated JSON event is refused", "rejection", rawChunks([]byte("{\n")), nil, unavailable, core.ErrJSONContract),
		observationCase("unknown JSON member is refused", "rejection", rawChunks([]byte(`{"Action":"pass","Package":"example.com/p","Future":true}`+"\n")), nil, unavailable, core.ErrJSONContract),
		observationCase("duplicate JSON member is refused", "rejection", rawChunks([]byte(`{"Action":"pass","Action":"fail","Package":"example.com/p"}`+"\n")), nil, unavailable, core.ErrJSONContract),
		observationCase("unknown action is refused", "rejection", events(event("future", "example.com/p", "", "")), nil, unavailable, core.ErrJSONContract),
		observationCase("missing package identity is refused", "rejection", events(event("pass", "", "", "")), nil, unavailable, core.ErrJSONContract),
		observationCase("event after package terminal is contradictory", "rejection", events(event("pass", "example.com/p", "", ""), event("output", "example.com/p", "", "late\n")), nil, pass, core.ErrJSONContract),
		observationCase("more package identities than planned are refused", "rejection", events(event("start", "example.com/a", "", ""), event("start", "example.com/b", "", "")), nil, unavailable, core.ErrPrimitiveContract),
		observationCase("incomplete benchmark result is refused", "rejection", events(event("output", "example.com/p", "BenchmarkX", "BenchmarkX-8 1 20 ns/op\n")), nil, unavailable, core.ErrJSONContract),
		observationCase("successful process without package terminal is refused", "rejection", events(event("start", "example.com/p", "", "")), nil, unavailable, core.ErrPrimitiveContract),
	}
	boundaries := boundaryObservationCases()
	return append(append(valid, rejections...), boundaries...)
}

func boundaryObservationCases() []goObservationCase {
	pass := accounting(1, 1, 0, 0, 0, 0, 0, 0, false)
	filteredPass := accounting(1, 1, 0, 0, 0, 0, 0, 0, true)
	cancelled := accounting(1, 0, 0, 0, 0, 0, 1, 0, false)
	timedOut := accounting(1, 0, 0, 0, 0, 1, 0, 0, false)
	failed := accounting(1, 0, 1, 0, 0, 0, 0, 0, false)
	return []goObservationCase{
		observationCase("final canonical event may omit trailing newline", "boundary", withoutFinalNewline(events(event("pass", "example.com/final", "", ""))), nil, pass),
		observationCase("one-byte writes preserve event framing", "boundary", oneByteChunks(events(event("pass", "example.com/bytes", "", ""))), nil, pass),
		observationCaseWithPolicy("filtered posture survives accounting", "boundary", policy(1, true), events(event("pass", "example.com/filter", "", "")), nil, filteredPass),
		observationCaseWithPolicy("two packages close independently", "boundary", policy(2, false), events(event("pass", "example.com/a", "", ""), event("fail", "example.com/b", "", "")), errors.New("go test exit status 1"), accounting(2, 1, 1, 0, 0, 0, 0, 0, false)),
		observationCaseWithPolicy("three terminal classes remain exclusive", "boundary", policy(3, false), events(event("pass", "example.com/a", "", ""), event("fail", "example.com/b", "", ""), event("skip", "example.com/c", "", "")), errors.New("go test exit status 1"), accounting(3, 1, 1, 1, 0, 0, 0, 0, false)),
		observationCase("active package cancellation is not failure", "boundary", events(event("start", "example.com/cancel", "", "")), context.Canceled, cancelled),
		observationCase("active package deadline is not cancellation", "boundary", events(event("start", "example.com/timeout", "", "")), context.DeadlineExceeded, timedOut),
		observationCase("active package process failure remains failure", "boundary", events(event("start", "example.com/fail", "", "")), errors.New("process failed"), failed),
		observationCase("cancellation before first event retains one cancelled unit", "boundary", rawChunks(nil), context.Canceled, cancelled),
		observationCase("deadline before first event retains one timed-out unit", "boundary", rawChunks(nil), context.DeadlineExceeded, timedOut),
		observationCase("setup failure before first event retains one failed unit", "boundary", rawChunks(nil), errors.New("setup failed"), failed),
		observationCaseWithPolicy("unstarted units remain not-run after cancellation", "boundary", policy(2, false), events(event("start", "example.com/a", "", "")), context.Canceled, accounting(2, 0, 0, 0, 0, 0, 1, 1, false)),
		observationCase("test skip does not replace package pass", "boundary", events(event("skip", "example.com/p", "TestOptional", ""), event("pass", "example.com/p", "", "")), nil, pass),
		benchmarkCase("zero bytes per operation is valid evidence", "boundary", "BenchmarkZeroBytes-8 1 1 ns/op 0 B/op 1 allocs/op\n", runprotocol.DecimalMeasurement{Coefficient: 1}),
		benchmarkCase("zero allocations per operation is valid evidence", "boundary", "BenchmarkZeroAllocs-8 1 1 ns/op 8 B/op 0 allocs/op\n", runprotocol.DecimalMeasurement{Coefficient: 1}),
		benchmarkCase("maximum iteration count is retained without overflow", "boundary", fmt.Sprintf("BenchmarkMaximum-8 %d 1 ns/op 0 B/op 0 allocs/op\n", uint64(math.MaxUint64)), runprotocol.DecimalMeasurement{Coefficient: 1}),
		benchmarkCase("nine decimal places meet the precision ceiling", "boundary", "BenchmarkPrecision-8 1 1.123456789 ns/op 0 B/op 0 allocs/op\n", runprotocol.DecimalMeasurement{Coefficient: 1123456789, Scale: 9}),
		benchmarkCase("decimal trailing zeroes normalize canonically", "boundary", "BenchmarkCanonical-8 1 1.2300 ns/op 0 B/op 0 allocs/op\n", runprotocol.DecimalMeasurement{Coefficient: 123, Scale: 2}),
		multiBenchmarkCase(),
		observationCase("malformed suffix cannot erase an earlier terminal", "boundary", appendSuffix(events(event("pass", "example.com/p", "", "")), []byte("{\n")), nil, pass, core.ErrJSONContract),
	}
}

func observationCase(name, class string, setup func(testing.TB) [][]byte, executionErr error, want runprotocol.ExecutionAccounting, wantErr ...error) goObservationCase {
	var expectedErr error
	if len(wantErr) == 1 {
		expectedErr = wantErr[0]
	}
	return goObservationCase{name: name, class: class, policy: policy(1, false), setup: setup, executionErr: executionErr, wantAccounting: want, wantErr: expectedErr}
}

func observationCaseWithPolicy(name, class string, observationPolicy runnercontrol.ObservationPolicy, setup func(testing.TB) [][]byte, executionErr error, want runprotocol.ExecutionAccounting) goObservationCase {
	return goObservationCase{name: name, class: class, policy: observationPolicy, setup: setup, executionErr: executionErr, wantAccounting: want}
}

func benchmarkCase(name, class, output string, nanoseconds runprotocol.DecimalMeasurement) goObservationCase {
	result := observationCase(name, class, events(event("output", "example.com/bench", "BenchmarkEvidence", output), event("pass", "example.com/bench", "", "")), nil, accounting(1, 1, 0, 0, 0, 0, 0, 0, false))
	result.wantBenchmarks = 1
	result.wantNanoseconds = nanoseconds
	return result
}

func multiBenchmarkCase() goObservationCase {
	result := observationCase("two distinct benchmark samples are both retained", "boundary", events(
		event("output", "example.com/bench", "BenchmarkA", "BenchmarkA-8 1 1 ns/op 0 B/op 0 allocs/op\n"),
		event("output", "example.com/bench", "BenchmarkB", "BenchmarkB-8 1 2 ns/op 0 B/op 0 allocs/op\n"),
		event("pass", "example.com/bench", "", ""),
	), nil, accounting(1, 1, 0, 0, 0, 0, 0, 0, false))
	result.wantBenchmarks = 2
	result.wantNanoseconds = runprotocol.DecimalMeasurement{Coefficient: 1}
	return result
}

func accounting(planned, passed, failed, skipped, unavailable, timedOut, cancelled, notRun uint32, filtered bool) runprotocol.ExecutionAccounting {
	attempt := runprotocol.ExecutionAttempt{Sequence: 1, Planned: planned, Passed: passed, Failed: failed, Skipped: skipped, Unavailable: unavailable, Expired: timedOut, Cancelled: cancelled, NotRun: notRun, Cache: runprotocol.CacheDisabled, Filtered: filtered}
	return runprotocol.ExecutionAccounting{Attempts: []runprotocol.ExecutionAttempt{attempt}}
}

func policy(expected uint32, filtered bool) runnercontrol.ObservationPolicy {
	return runnercontrol.ObservationPolicy{Format: runnercontrol.ObservationGoTestJSON, ExpectedUnits: expected, Filtered: filtered}
}

func event(action, packageIdentity, test, output string) goEventFixture {
	return goEventFixture{Action: action, Package: packageIdentity, Test: test, Output: output}
}

func events(values ...goEventFixture) func(testing.TB) [][]byte {
	return func(t testing.TB) [][]byte {
		t.Helper()
		var stream []byte
		for index := range values {
			encoded, err := json.Marshal(values[index])
			if err != nil {
				t.Fatalf("json.Marshal(go event fixture %d) error = %v, want nil", index, err)
			}
			stream = append(stream, encoded...)
			stream = append(stream, '\n')
		}
		return [][]byte{stream}
	}
}

func rawChunks(chunks ...[]byte) func(testing.TB) [][]byte {
	return func(testing.TB) [][]byte {
		result := make([][]byte, len(chunks))
		for index := range chunks {
			result[index] = append([]byte(nil), chunks[index]...)
		}
		return result
	}
}

func oneByteChunks(base func(testing.TB) [][]byte) func(testing.TB) [][]byte {
	return func(t testing.TB) [][]byte {
		stream := base(t)[0]
		chunks := make([][]byte, len(stream))
		for index := range stream {
			chunks[index] = []byte{stream[index]}
		}
		return chunks
	}
}

func withoutFinalNewline(base func(testing.TB) [][]byte) func(testing.TB) [][]byte {
	return func(t testing.TB) [][]byte {
		stream := base(t)[0]
		return [][]byte{slices.Clone(stream[:len(stream)-1])}
	}
}

func appendSuffix(base func(testing.TB) [][]byte, suffix []byte) func(testing.TB) [][]byte {
	return func(t testing.TB) [][]byte {
		stream := base(t)[0]
		return [][]byte{append(slices.Clone(stream), suffix...)}
	}
}

func FuzzGoTestObservationCompilerSemanticClosure(f *testing.F) {
	canonical := events(event("pass", "example.com/fuzz", "", ""))(f)[0]
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte("{\n"))
	f.Add([]byte(`{"Action":"future","Package":"example.com/fuzz"}` + "\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		compiler, err := runnercontrol.NewGoTestObservationCompiler(policy(1, false))
		if err != nil {
			t.Fatalf("NewGoTestObservationCompiler(fuzz policy) error = %v, want nil", err)
		}
		gotWritten, gotWriteErr := compiler.Write(data)
		if gotWriteErr != nil || gotWritten != len(data) {
			t.Fatalf("GoTestObservationCompiler.Write(fuzz input) = (%d, %v), want (%d, nil) so raw evidence remains retainable", gotWritten, gotWriteErr, len(data))
		}
		got, gotErr := compiler.Seal(nil)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrPrimitiveContract) {
				t.Fatalf("GoTestObservationCompiler.Seal(rejected fuzz input) error = %v, want errors.Is(..., %v)", gotErr, core.ErrPrimitiveContract)
			}
			if len(got.Accounting.Attempts) != 0 {
				if validationErr := got.Validate(); validationErr != nil {
					t.Fatalf("GoTestObservationCompiler.Seal(rejected fuzz input) retained observation Validate() error = %v, want nil", validationErr)
				}
			}
			return
		}
		if validationErr := got.Validate(); validationErr != nil {
			t.Fatalf("GoTestObservationCompiler.Seal(accepted fuzz input) Validate() error = %v, want nil", validationErr)
		}
		latest, ok := got.Accounting.Latest()
		if !ok || latest.Planned != 1 || latest.Passed+latest.Failed+latest.Skipped != 1 {
			t.Fatalf("GoTestObservationCompiler.Seal(accepted fuzz input) accounting = %+v, want exactly one terminal package unit", got.Accounting)
		}
	})
}
