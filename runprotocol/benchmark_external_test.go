package runprotocol_test

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/runprotocol"
)

func BenchmarkParseSourcePath(b *testing.B) {
	const value = "filestore/walk.go"
	var wantErr error
	b.ReportAllocs()
	var last runprotocol.SourcePath
	for b.Loop() {
		got, err := runprotocol.ParseSourcePath(value)
		if !errors.Is(err, wantErr) {
			b.Fatalf("runprotocol.ParseSourcePath() error = %v, want %v", err, wantErr)
		}
		last = got
	}
	if last.String() != value {
		b.Fatalf("runprotocol.ParseSourcePath() = %q, want %q", last, value)
	}
}

// benchmarkFuzzTranscript is a fixed native fuzz transcript: many short
// progress records, as `go test -fuzz` prints them, ending in the terminal
// frame for FuzzChosen.
func benchmarkFuzzTranscript(b *testing.B) *strings.Reader {
	b.Helper()
	const progressRecords = 2048
	var transcript strings.Builder
	for index := range progressRecords {
		fmt.Fprintf(&transcript, "fuzz: elapsed: %ds, execs: %d (411/sec), new interesting: 12 (total: 34)\n", index, 1000+index)
	}
	transcript.WriteString("--- PASS: FuzzChosen (12.34s)\n")
	if transcript.Len() < progressRecords*64 {
		b.Fatalf("transcript bytes = %d, want a non-vacuous fixed transcript", transcript.Len())
	}
	return strings.NewReader(transcript.String())
}

func BenchmarkObserveGoFuzzCompletionTranscript(b *testing.B) {
	source := benchmarkFuzzTranscript(b)
	target := fuzzCompletionTarget(b, "FuzzChosen")
	b.SetBytes(source.Size())
	b.ReportAllocs()
	var got runprotocol.GoFuzzCompletionObservation
	for b.Loop() {
		if _, err := source.Seek(0, io.SeekStart); err != nil {
			b.Fatal(err)
		}
		var err error
		got, err = runprotocol.ObserveGoFuzzCompletion(b.Context(), runprotocol.GoFuzzCompletionRequest{Source: source, Target: target})
		if err != nil {
			b.Fatal(err)
		}
	}
	if got.Completion() != runprotocol.GoFuzzCompletionPassed {
		b.Fatalf("completion = %v, want pass", got.Completion())
	}
}

func BenchmarkGoFuzzMetricRecordsTranscript(b *testing.B) {
	source := benchmarkFuzzTranscript(b)
	b.SetBytes(source.Size())
	b.ReportAllocs()
	var last int64
	records := 0
	for b.Loop() {
		if _, err := source.Seek(0, io.SeekStart); err != nil {
			b.Fatal(err)
		}
		records = 0
		for record, err := range runprotocol.GoFuzzMetricRecords(b.Context(), runprotocol.GoFuzzMetricSourceRequest{Source: source}) {
			if err != nil {
				b.Fatal(err)
			}
			records++
			field, err := record.Field(runprotocol.GoFuzzMetricExecs)
			if err != nil {
				b.Fatal(err)
			}
			if value, present := field.Value(); present {
				last = value
			}
		}
	}
	if records != 2049 || last != 1000+2047 {
		b.Fatalf("records/last execs = %d/%d, want 2049/%d", records, last, 1000+2047)
	}
}
