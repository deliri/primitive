package runnercontrol_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
)

var benchmarkGoJSONStreamSink runnercontrol.GoTestObservation

func BenchmarkGoJSONDiagnosticStreaming(b *testing.B) {
	b.ReportAllocs()
	for _, size := range []int{1024, 65536} {
		name := "one KiB diagnostic"
		if size == 65536 {
			name = "64 KiB diagnostic"
		}
		b.Run(name, func(b *testing.B) {
			data := events(event("output", "selected", "", strings.Repeat("x", size)), event("pass", "selected", "", ""))(b)[0]
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for b.Loop() {
				compiler, err := runnercontrol.NewGoTestObservationCompiler(runnercontrol.ObservationPolicy{Format: runnercontrol.ObservationGoTestJSON, ExpectedUnits: 1})
				if err != nil {
					b.Fatalf("NewGoTestObservationCompiler() error = %v, want nil", err)
				}
				if _, err := compiler.Write(data); err != nil {
					b.Fatalf("Write() error = %v, want nil", err)
				}
				got, err := compiler.Seal(nil)
				if err != nil || len(got.Accounting.Attempts) != 1 || got.Accounting.Attempts[0].Passed != 1 {
					b.Fatalf("Seal() = %+v/%v, want one passed package/nil", got, err)
				}
				benchmarkGoJSONStreamSink = got
			}
		})
	}
}

func TestGoJSONStreamingDiscardsArbitraryDiagnosticExtent(t *testing.T) {
	t.Parallel()
	compiler, err := runnercontrol.NewGoTestObservationCompiler(runnercontrol.ObservationPolicy{Format: runnercontrol.ObservationGoTestJSON, ExpectedUnits: 1})
	if err != nil {
		t.Fatalf("NewGoTestObservationCompiler() error = %v, want nil", err)
	}
	if _, err := compiler.Write([]byte(`{"Action":"output","Output":"`)); err != nil {
		t.Fatalf("Write(prefix) error = %v, want nil", err)
	}
	chunk := bytes.Repeat([]byte("diagnostic "), 1024)
	for range 2048 {
		if n, err := compiler.Write(chunk); n != len(chunk) || err != nil {
			t.Fatalf("Write(diagnostic) = %d/%v, want %d/nil", n, err, len(chunk))
		}
	}
	suffix := "\",\"Package\":\"selected\"}\n{\"Action\":\"pass\",\"Package\":\"selected\"}\n"
	if _, err := compiler.Write([]byte(suffix)); err != nil {
		t.Fatalf("Write(suffix) error = %v, want nil", err)
	}
	got, err := compiler.Seal(nil)
	if err != nil || len(got.Accounting.Attempts) != 1 || got.Accounting.Attempts[0].Passed != 1 || len(got.Benchmarks) != 0 {
		t.Fatalf("Seal() = %+v/%v, want one passed package and no measurements/nil", got, err)
	}
}

func TestGoJSONStreamingEscapesAndDuplicateMetrics(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, data string
		wantErr    error
	}{
		{name: "escaped package identity equals UTF8 identity", data: "{\"Action\":\"start\",\"Package\":\"\\uD83D\\uDE00\"}\n{\"Action\":\"pass\",\"Package\":\"😀\"}\n"},
		{name: "duplicate benchmark unit cannot choose first value", data: "{\"Action\":\"output\",\"Package\":\"p\",\"Output\":\"BenchmarkX 1 1 ns/op 2 ns/op 0 B/op 0 allocs/op\"}\n", wantErr: core.ErrJSONContract},
		{name: "duplicate escaped member remains duplicate", data: `{"Action":"pass","Package":"p","\u0050ackage":"q"}`, wantErr: core.ErrJSONContract},
		{name: "lone high surrogate is refused", data: `{"Action":"pass","Package":"\uD800"}`, wantErr: core.ErrJSONContract},
		{name: "split UTF8 after fragment window", data: "{\"Action\":\"pass\",\"Package\":\"" + strings.Repeat("a", 191) + "😀\"}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compiler, err := runnercontrol.NewGoTestObservationCompiler(runnercontrol.ObservationPolicy{Format: runnercontrol.ObservationGoTestJSON, ExpectedUnits: 1})
			if err != nil {
				t.Fatalf("NewGoTestObservationCompiler() error = %v, want nil", err)
			}
			for _, value := range []byte(tc.data) {
				if _, err := compiler.Write([]byte{value}); err != nil {
					if !errors.Is(err, tc.wantErr) {
						t.Fatalf("Write() error = %v, want %v", err, tc.wantErr)
					}
					break
				}
			}
			got, err := compiler.Seal(nil)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("Seal() error = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil || len(got.Accounting.Attempts) != 1 || got.Accounting.Attempts[0].Passed != 1 {
				t.Fatalf("Seal() = %+v/%v, want one passed package/nil", got, err)
			}
		})
	}
}
