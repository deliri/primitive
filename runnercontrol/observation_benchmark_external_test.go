package runnercontrol_test

import (
	json "encoding/json/v2"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
)

const (
	benchmarkStreamPackages          = 4
	benchmarkStreamTestsPerPackage   = 16
	benchmarkStreamResultsPerPackage = 4
)

// goTestStreamBenchmarkFixture is a fixed go test -json stream shaped like real
// output: per-test run/output/pass events, benchmark result lines and one
// terminal pass per package.
func goTestStreamBenchmarkFixture(t testing.TB) []byte {
	t.Helper()
	fixtures := make([]goEventFixture, 0, benchmarkStreamPackages*(benchmarkStreamTestsPerPackage*3+benchmarkStreamResultsPerPackage+1))
	for packageIndex := range benchmarkStreamPackages {
		packageIdentity := "github.com/example/project/internal/package" + strconv.Itoa(packageIndex)
		for testIndex := range benchmarkStreamTestsPerPackage {
			name := "TestBehaviour" + strconv.Itoa(testIndex)
			fixtures = append(fixtures,
				event("run", packageIdentity, name, ""),
				event("output", packageIdentity, name, "=== RUN   "+name+"\n"),
				event("pass", packageIdentity, name, ""))
		}
		for resultIndex := range benchmarkStreamResultsPerPackage {
			line := "BenchmarkPath" + strconv.Itoa(packageIndex) + "x" + strconv.Itoa(resultIndex) + "-10   \t  123456\t      9876 ns/op\t     512 B/op\t       7 allocs/op\n"
			fixtures = append(fixtures, event("output", packageIdentity, "", line))
		}
		fixtures = append(fixtures, event("pass", packageIdentity, "", ""))
	}
	return events(fixtures...)(t)[0]
}

var benchmarkGoTestEventStreamSink runnercontrol.GoTestObservation

func BenchmarkGoTestObservationEventStream(b *testing.B) {
	data := goTestStreamBenchmarkFixture(b)
	policy := runnercontrol.ObservationPolicy{Format: runnercontrol.ObservationGoTestJSON, ExpectedUnits: benchmarkStreamPackages}
	wantBenchmarks := benchmarkStreamPackages * benchmarkStreamResultsPerPackage
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		compiler, err := runnercontrol.NewGoTestObservationCompiler(policy)
		if err != nil {
			b.Fatalf("NewGoTestObservationCompiler() error = %v, want nil", err)
		}
		if _, err := compiler.Write(data); err != nil {
			b.Fatalf("Write() error = %v, want nil", err)
		}
		got, err := compiler.Seal(nil)
		if err != nil {
			b.Fatalf("Seal() error = %v, want nil", err)
		}
		benchmarkGoTestEventStreamSink = got
	}
	got := benchmarkGoTestEventStreamSink
	if len(got.Benchmarks) != wantBenchmarks || got.Benchmarks[0].NanosecondsPerOp.Coefficient != 9876 || got.Benchmarks[0].AllocationsPerOp != 7 {
		b.Fatalf("Seal() benchmarks = %+v, want %d projected results", got.Benchmarks, wantBenchmarks)
	}
	attempt, ok := got.Accounting.Latest()
	if !ok || attempt.Passed != benchmarkStreamPackages {
		b.Fatalf("Seal() accounting = %+v, want %d passed packages", got.Accounting, benchmarkStreamPackages)
	}
}

var benchmarkJUnitObservationSink runnercontrol.JUnitObservation

func BenchmarkJUnitObservationStreaming(b *testing.B) {
	const cases = 64
	var document strings.Builder
	document.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n<testsuites><testsuite name=\"package\" tests=\"64\">")
	for index := range cases {
		document.WriteString(`<testcase classname="github.com/example/project" name="TestBehaviour` + strconv.Itoa(index) + `" time="0.001"><system-out>ok</system-out></testcase>`)
	}
	document.WriteString("</testsuite></testsuites>\n")
	data := []byte(document.String())
	policy := runnercontrol.ObservationPolicy{Format: runnercontrol.ObservationJUnitXML, ExpectedUnits: cases}
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		compiler, err := runnercontrol.NewJUnitObservationCompiler(policy)
		if err != nil {
			b.Fatalf("NewJUnitObservationCompiler() error = %v, want nil", err)
		}
		if _, err := compiler.Write(data); err != nil {
			compiler.Abort()
			b.Fatalf("Write() error = %v, want nil", err)
		}
		got, err := compiler.Seal(nil)
		if err != nil {
			b.Fatalf("Seal() error = %v, want nil", err)
		}
		benchmarkJUnitObservationSink = got
	}
	attempt, ok := benchmarkJUnitObservationSink.Accounting.Latest()
	if !ok || attempt.Passed != cases {
		b.Fatalf("Seal() accounting = %+v, want %d passed cases", benchmarkJUnitObservationSink.Accounting, cases)
	}
}

var benchmarkCapabilityDigestSink core.SHA256Digest

func BenchmarkExperimentCapabilityDigest(b *testing.B) {
	capability := experimentObservationRequestFixture(b).Capability
	if err := capability.Validate(); err != nil {
		b.Fatalf("ExperimentCapability.Validate(benchmark workload) error = %v, want nil", err)
	}
	b.ReportAllocs()
	for b.Loop() {
		digest, err := capability.Digest()
		if err != nil {
			b.Fatalf("ExperimentCapability.Digest() error = %v, want nil", err)
		}
		benchmarkCapabilityDigestSink = digest
	}
	if err := benchmarkCapabilityDigestSink.Validate(); err != nil {
		b.Fatalf("ExperimentCapability.Digest() result.Validate() error = %v, want nil", err)
	}
}

// The digest is the SHA-256 of exactly the document MarshalJSON emits.
func TestExperimentCapabilityDigestCoversMarshaledDocument(t *testing.T) {
	t.Parallel()
	capability := experimentObservationRequestFixture(t).Capability
	encoded, err := json.Marshal(capability)
	if err != nil {
		t.Fatalf("json.Marshal(ExperimentCapability) error = %v, want nil", err)
	}
	got, err := capability.Digest()
	if want := core.SHA256Of(encoded); err != nil || got != want {
		t.Fatalf("ExperimentCapability.Digest() = %v/%v, want %v/nil", got, err, want)
	}
	invalid := capability
	invalid.SchemaVersion++
	if _, err := invalid.Digest(); err == nil {
		t.Fatal("ExperimentCapability.Digest(invalid schema) error = nil, want refusal")
	}
}
