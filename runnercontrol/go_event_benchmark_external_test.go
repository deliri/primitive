package runnercontrol_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/runprotocol"
)

// benchmarkConsumerRefusal is a consumer's own typed refusal, returned unchanged.
type benchmarkConsumerRefusal struct{}

func (benchmarkConsumerRefusal) Error() string { return "benchmark consumer refused" }

// benchmarkRowAt is one delivered row and how many events had closed before it.
type benchmarkRowAt struct {
	row    runprotocol.GoBenchmarkRow
	events int
}

func readBenchmarkRows(t *testing.T, source []byte, observe bool, refuse error) ([]benchmarkRowAt, int, error) {
	t.Helper()
	var rows []benchmarkRowAt
	events := 0
	request := runnercontrol.GoEventStreamRequest{Source: bytes.NewReader(source),
		OnString: func(runnercontrol.GoEventStringFragment) error { return nil },
		OnEvent:  func(runnercontrol.GoEventFrame) error { events++; return nil }}
	if observe {
		request.OnBenchmark = func(row runprotocol.GoBenchmarkRow) error {
			rows = append(rows, benchmarkRowAt{row: row, events: events})
			return refuse
		}
	}
	err := runnercontrol.ReadGoEventStream(context.Background(), request)
	return rows, events, err
}

func benchmarkFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "go_event_benchmark", name+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func measuredRow(t *testing.T, name string, nanoseconds uint64) runprotocol.GoBenchmarkRow {
	t.Helper()
	label, err := runprotocol.NewName(name)
	if err != nil {
		t.Fatal(err)
	}
	return runprotocol.GoBenchmarkRow{Kind: runprotocol.GoBenchmarkRowMeasured, Measurement: &runprotocol.BenchmarkMeasurement{Name: label, Iterations: 1, NanosecondsPerOp: runprotocol.DecimalMeasurement{Coefficient: nanoseconds}}}
}

func kindRow(kind runprotocol.GoBenchmarkRowKind) runprotocol.GoBenchmarkRow {
	return runprotocol.GoBenchmarkRow{Kind: kind}
}

// sameBenchmarkRow compares kinds and measured values, not pointers.
func sameBenchmarkRow(got, want runprotocol.GoBenchmarkRow) bool {
	if got.Kind != want.Kind || (got.Measurement == nil) != (want.Measurement == nil) {
		return false
	}
	return got.Measurement == nil || *got.Measurement == *want.Measurement
}

// hostileEvents frames synthetic test2json events, one per line. An empty
// package frames a non-output pass event.
func hostileEvents(events ...[2]string) []byte {
	var out strings.Builder
	quote := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\t", `\t`)
	for _, event := range events {
		pkg, output := event[0], event[1]
		if pkg == "" {
			out.WriteString(`{"Action":"pass","Package":"example.com/a"}` + "\n")
			continue
		}
		out.WriteString(`{"Action":"output","Package":"` + pkg + `","Output":"` + quote.Replace(output) + `"}` + "\n")
	}
	return []byte(out.String())
}

// Every benchmark result row is reported once, typed, as its newline is
// decoded and before that event's OnEvent; none is dropped or invented.
// Rows reassemble across consecutive output events of one package: native
// test2json splits both "name\t" from its numbers and lines over 1024 bytes.
func TestGoEventStreamBenchmarkRowsLayerTriad(t *testing.T) {
	t.Parallel()
	refusal := benchmarkConsumerRefusal{}
	missing := kindRow(runprotocol.GoBenchmarkRowMetricsMissing)
	for _, tc := range []struct {
		layer, name string
		source      func(*testing.T) []byte
		observe     bool
		refuse      error
		want        []runprotocol.GoBenchmarkRow
		// wantAt is each row's count of closed events when delivered.
		wantAt  []int
		wantErr error
	}{
		{layer: "positive", name: "native -benchmem rows, one split name from its numbers", source: func(t *testing.T) []byte { return benchmarkFixture(t, "benchmem") }, observe: true,
			want:   []runprotocol.GoBenchmarkRow{measuredRow(t, "BenchmarkB", 333), measuredRow(t, "BenchmarkB-2", 83), measuredRow(t, "BenchmarkSub/s", 250), measuredRow(t, "BenchmarkSub/s-2", 83)},
			wantAt: []int{8, 10, 17, 18}},
		{layer: "positive", name: "a native row split over 1024 bytes is unrepresentable, not mislabeled", source: func(t *testing.T) []byte { return benchmarkFixture(t, "longname") }, observe: true,
			want: []runprotocol.GoBenchmarkRow{kindRow(runprotocol.GoBenchmarkRowNameUnrepresentable)}},
		{layer: "negative", name: "native rows without -benchmem are typed and the stream continues", source: func(t *testing.T) []byte { return benchmarkFixture(t, "nobenchmem") }, observe: true,
			want: []runprotocol.GoBenchmarkRow{missing}},
		{layer: "negative", name: "a native row with ns/op suppressed is metrics missing, not dropped", source: func(t *testing.T) []byte { return benchmarkFixture(t, "custom") }, observe: true,
			want: []runprotocol.GoBenchmarkRow{missing}},
		{layer: "negative", name: "a native -benchmem row with ns/op suppressed is metrics missing, not malformed", source: func(t *testing.T) []byte { return benchmarkFixture(t, "custommem") }, observe: true,
			want: []runprotocol.GoBenchmarkRow{missing}},
		{layer: "negative", name: "a native row of only a name and count (ns/op suppressed) is metrics missing", source: func(t *testing.T) []byte { return benchmarkFixture(t, "countonly") }, observe: true,
			want: []runprotocol.GoBenchmarkRow{missing}},
		{layer: "negative", name: "a space-separated custom-unit row is metrics missing", source: func(*testing.T) []byte {
			return hostileEvents([2]string{"example.com/a", "BenchmarkX 1 2 widgets/op\n"})
		}, observe: true, want: []runprotocol.GoBenchmarkRow{missing}},
		{layer: "negative", name: "another package cuts a pending row", source: func(*testing.T) []byte {
			return hostileEvents([2]string{"example.com/a", "BenchmarkX-2 \t"}, [2]string{"example.com/b", "       1\t       5.0 ns/op\t       0 B/op\t       0 allocs/op\n"})
		}, observe: true, want: []runprotocol.GoBenchmarkRow{kindRow(runprotocol.GoBenchmarkRowInterrupted)}},
		{layer: "negative", name: "a non-output event cuts a pending row", source: func(*testing.T) []byte {
			return hostileEvents([2]string{"example.com/a", "BenchmarkX-2 \t"}, [2]string{}, [2]string{"example.com/a", "       1\t       5.0 ns/op\t       0 B/op\t       0 allocs/op\n"})
		}, observe: true, want: []runprotocol.GoBenchmarkRow{kindRow(runprotocol.GoBenchmarkRowInterrupted)}},
		{layer: "negative", name: "the end of the stream cuts a pending row", source: func(*testing.T) []byte {
			return hostileEvents([2]string{"example.com/a", "BenchmarkX-2 \t       1\t"})
		}, observe: true, want: []runprotocol.GoBenchmarkRow{kindRow(runprotocol.GoBenchmarkRowInterrupted)}},
		{layer: "negative", name: "a benchmark-shaped row with invalid fields is malformed", source: func(*testing.T) []byte {
			return hostileEvents([2]string{"example.com/a", "BenchmarkX \tmany\t       5.0 ns/op\t       0 B/op\t       0 allocs/op\n"})
		}, observe: true, want: []runprotocol.GoBenchmarkRow{kindRow(runprotocol.GoBenchmarkRowMalformed)}},
		{layer: "negative", name: "a frame whose Output precedes its Package is unattributed", source: func(*testing.T) []byte {
			return []byte(`{"Action":"output","Output":"BenchmarkX \t1\t5 ns/op\t0 B/op\t0 allocs/op\n","Package":"example.com/a"}` + "\n")
		}, observe: true, want: []runprotocol.GoBenchmarkRow{kindRow(runprotocol.GoBenchmarkRowUnattributed)}},
		{layer: "negative", name: "a frame whose Output precedes its Action is unattributed", source: func(*testing.T) []byte {
			return []byte(`{"Package":"example.com/a","Output":"BenchmarkX \t1\t5 ns/op\t0 B/op\t0 allocs/op\n","Action":"output"}` + "\n")
		}, observe: true, want: []runprotocol.GoBenchmarkRow{kindRow(runprotocol.GoBenchmarkRowUnattributed)}},
		{layer: "negative", name: "an unattributed frame never continues a pending row", source: func(*testing.T) []byte {
			return append(hostileEvents([2]string{"example.com/a", "BenchmarkX-2 \t"}), []byte(`{"Action":"output","Output":"       1\t       5.0 ns/op\t       0 B/op\t       0 allocs/op\n","Package":"example.com/a"}`+"\n")...)
		}, observe: true, want: []runprotocol.GoBenchmarkRow{kindRow(runprotocol.GoBenchmarkRowInterrupted)}},
		{layer: "negative", name: "another package's output frame without Output cuts a pending row", source: func(*testing.T) []byte {
			return slices.Concat(hostileEvents([2]string{"example.com/a", "BenchmarkX-2 \t"}), []byte(`{"Action":"output","Package":"example.com/b"}`+"\n"), hostileEvents([2]string{"example.com/a", "       1\t       5.0 ns/op\t       0 B/op\t       0 allocs/op\n"}))
		}, observe: true, want: []runprotocol.GoBenchmarkRow{kindRow(runprotocol.GoBenchmarkRowInterrupted)}},
		{layer: "negative", name: "another package's output frame with empty Output cuts a pending row", source: func(*testing.T) []byte {
			return slices.Concat(hostileEvents([2]string{"example.com/a", "BenchmarkX-2 \t"}), []byte(`{"Action":"output","Package":"example.com/b","Output":""}`+"\n"), hostileEvents([2]string{"example.com/a", "       1\t       5.0 ns/op\t       0 B/op\t       0 allocs/op\n"}))
		}, observe: true, want: []runprotocol.GoBenchmarkRow{kindRow(runprotocol.GoBenchmarkRowInterrupted)}},
		{layer: "negative", name: "a build-output frame without Package still cuts a pending row", source: func(*testing.T) []byte {
			return slices.Concat(hostileEvents([2]string{"example.com/a", "BenchmarkX-2 \t"}), []byte(`{"ImportPath":"example.com/a","Action":"build-output","Output":"# example.com/a\n"}`+"\n"), hostileEvents([2]string{"example.com/a", "       1\t       5.0 ns/op\t       0 B/op\t       0 allocs/op\n"}))
		}, observe: true, want: []runprotocol.GoBenchmarkRow{kindRow(runprotocol.GoBenchmarkRowInterrupted)}},
		{layer: "negative", name: "a consumer refusal of an interrupted row at the end keeps the stream identity", source: func(*testing.T) []byte {
			return hostileEvents([2]string{"example.com/a", "BenchmarkX-2 \t       1\t"})
		}, observe: true, refuse: refusal, wantErr: refusal},
		{layer: "negative", name: "its own package never continues an unattributed pending row", source: func(*testing.T) []byte {
			return slices.Concat([]byte(`{"Package":"example.com/a","Output":"BenchmarkX-2 \t","Action":"output"}`+"\n"), hostileEvents([2]string{"example.com/a", "       1\t       5.0 ns/op\t       0 B/op\t       0 allocs/op\n"}))
		}, observe: true, want: []runprotocol.GoBenchmarkRow{kindRow(runprotocol.GoBenchmarkRowInterrupted)}},
		{layer: "negative", name: "a consumer refusal stops the stream unchanged", source: func(t *testing.T) []byte { return benchmarkFixture(t, "benchmem") }, observe: true, refuse: refusal, wantErr: refusal},
		{layer: "neutral", name: "without a consumer the same rows read as before", source: func(t *testing.T) []byte { return benchmarkFixture(t, "nobenchmem") }},
		{layer: "neutral", name: "a tests-only stream has no benchmark row", source: func(t *testing.T) []byte { return benchmarkFixture(t, "tests") }, observe: true},
		{layer: "neutral", name: "a build-output frame without Package is not a row", source: func(*testing.T) []byte {
			return []byte(`{"ImportPath":"example.com/a","Action":"build-output","Output":"BenchmarkX \t1\t5 ns/op\t0 B/op\t0 allocs/op\n"}` + "\n")
		}, observe: true},
		{layer: "neutral", name: "a measured-looking line not named Benchmark is not a row", source: func(*testing.T) []byte {
			return hostileEvents([2]string{"example.com/a", "Total \t1\t5 ns/op\t0 B/op\t0 allocs/op\n"})
		}, observe: true},
		{layer: "neutral", name: "lowercase after the prefix, headers and logs are not rows", source: func(*testing.T) []byte {
			return hostileEvents([2]string{"example.com/a", "Benchmarking \t1\t5 ns/op\n"}, [2]string{"example.com/a", "BenchmarkX\n"}, [2]string{"example.com/a", "BenchmarkFoo is slow\n"})
		}, observe: true},
	} {
		t.Run(tc.layer+"/"+tc.name, func(t *testing.T) {
			t.Parallel()
			source := tc.source(t)
			rows, events, err := readBenchmarkRows(t, source, tc.observe, tc.refuse)
			if tc.wantErr != nil {
				// Every callback refusal, completed or interrupted, carries the
				// stream's observation identity beside its own cause.
				if !errors.Is(err, tc.wantErr) || !errors.Is(err, core.ErrPrimitiveContract) || len(rows) != 1 {
					t.Fatalf("ReadGoEventStream = %v after %d rows, want %v and %v after the first row", err, len(rows), tc.wantErr, core.ErrPrimitiveContract)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if lines := bytes.Count(source, []byte("\n")); events != lines {
				t.Fatalf("events got %d, want every one of %d lines", events, lines)
			}
			if len(rows) != len(tc.want) {
				t.Fatalf("rows got %+v, want %+v", rows, tc.want)
			}
			for i, got := range rows {
				if !sameBenchmarkRow(got.row, tc.want[i]) {
					t.Fatalf("row %d got %+v, want %+v", i, got.row, tc.want[i])
				}
				if i < len(tc.wantAt) && got.events != tc.wantAt[i] {
					t.Fatalf("row %d delivered after %d events, want %d: before its own event's OnEvent", i, got.events, tc.wantAt[i])
				}
			}
		})
	}
}

// GoTestObservationCompiler uses the same reassembled rows. A natively split
// row compiles to its true measurement; a row whose name cannot be
// represented is refused instead of being kept under its tail fragment; a
// frame that never closes contributes nothing, while earlier rows remain.
func TestGoTestObservationCompilerBenchmarkRowsLayerTriad(t *testing.T) {
	t.Parallel()
	unclosed := append(benchmarkFixture(t, "benchmem"), []byte(`{"Action":"output","Package":"example.com/nm","Output":"BenchmarkZ \t1\t5 ns/op\t0 B/op\t0 allocs/op\n"`)...)
	measured := []runprotocol.GoBenchmarkRow{measuredRow(t, "BenchmarkB", 333), measuredRow(t, "BenchmarkB-2", 83), measuredRow(t, "BenchmarkSub/s", 250), measuredRow(t, "BenchmarkSub/s-2", 83)}
	for _, tc := range []struct {
		layer, name string
		source      []byte
		want        []runprotocol.GoBenchmarkRow
		wantErr     error
	}{
		{layer: "positive", name: "a natively split row compiles to its true measurement", source: benchmarkFixture(t, "benchmem"), want: measured},
		{layer: "negative", name: "an unrepresentable long name is refused, not mislabeled", source: benchmarkFixture(t, "longname"), wantErr: core.ErrJSONContract},
		{layer: "negative", name: "rows without -benchmem are refused", source: benchmarkFixture(t, "nobenchmem"), wantErr: core.ErrJSONContract},
		{layer: "negative", name: "a frame that never closes keeps earlier rows and adds none", source: unclosed, want: measured, wantErr: core.ErrJSONContract},
		{layer: "neutral", name: "a tests-only stream compiles without benchmarks", source: benchmarkFixture(t, "tests")},
	} {
		t.Run(tc.layer+"/"+tc.name, func(t *testing.T) {
			t.Parallel()
			compiler, err := runnercontrol.NewGoTestObservationCompiler(runnercontrol.ObservationPolicy{Format: runnercontrol.ObservationGoTestJSON, ExpectedUnits: 1})
			if err != nil {
				t.Fatal(err)
			}
			if written, err := compiler.Write(tc.source); err != nil || written != len(tc.source) {
				t.Fatalf("Write = (%d, %v), want the whole stream", written, err)
			}
			got, err := compiler.Seal(nil)
			if (tc.wantErr == nil) != (err == nil) || tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("Seal error = %v, want %v", err, tc.wantErr)
			}
			if len(got.Benchmarks) != len(tc.want) {
				t.Fatalf("Seal benchmarks = %+v, want %d", got.Benchmarks, len(tc.want))
			}
			for i := range tc.want {
				if got.Benchmarks[i] != *tc.want[i].Measurement {
					t.Fatalf("benchmark %d got %+v, want %+v", i, got.Benchmarks[i], *tc.want[i].Measurement)
				}
			}
		})
	}
}

// Splitting output between consecutive events of one package never changes
// the rows: test2json may cut any line, so reassembly must be invariant. No
// measured row may name text that is absent from the output.
func FuzzGoEventStreamBenchmarkRowsSplitInvariance(f *testing.F) {
	f.Add("BenchmarkB-2     \t       1\t        83.00 ns/op\t       0 B/op\t       0 allocs/op\n", uint16(18), uint16(40))
	f.Add("BenchmarkB\nBenchmarkB       \t       1\t       333.0 ns/op\t       0 B/op\t       0 allocs/op\n", uint16(11), uint16(30))
	f.Add("BenchmarkB-10    \t       1\t       208.0 ns/op\n", uint16(3), uint16(25))
	f.Add("BenchmarkCustom-10    \t       1\t         5.000 widgets/op\n", uint16(9), uint16(33))
	f.Add("BenchmarkX \tmany\t5.0 ns/op\t0 B/op\t0 allocs/op\nok\n", uint16(1), uint16(2))
	f.Fuzz(func(t *testing.T, text string, first, second uint16) {
		if !printableJSONText(text) {
			return
		}
		cuts := runeCuts(text, int(first), int(second))
		whole, _, wholeErr := readBenchmarkRows(t, hostileEvents([2]string{"example.com/a", text}), true, nil)
		var parts [][2]string
		start := 0
		for _, cut := range append(cuts, len(text)) {
			if cut > start {
				parts = append(parts, [2]string{"example.com/a", text[start:cut]})
				start = cut
			}
		}
		split, _, splitErr := readBenchmarkRows(t, hostileEvents(parts...), true, nil)
		if (wholeErr != nil) != (splitErr != nil) {
			t.Fatalf("whole error %v, split error %v, want the same outcome", wholeErr, splitErr)
		}
		if len(whole) != len(split) {
			t.Fatalf("whole rows %+v, split rows %+v at cuts %v", whole, split, cuts)
		}
		for i := range whole {
			if !sameBenchmarkRow(whole[i].row, split[i].row) {
				t.Fatalf("row %d whole %+v, split %+v at cuts %v", i, whole[i].row, split[i].row, cuts)
			}
			if whole[i].row.Measurement != nil && !strings.Contains(text, whole[i].row.Measurement.Name.String()) {
				t.Fatalf("measured name %q is not in the output", whole[i].row.Measurement.Name.String())
			}
		}
	})
}

// printableJSONText keeps fuzz text to what test2json can carry: valid UTF-8
// without control bytes other than tab and newline.
func printableJSONText(text string) bool {
	if !utf8.ValidString(text) {
		return false
	}
	for _, value := range text {
		if value < ' ' && value != '\t' && value != '\n' || value == 0x7f {
			return false
		}
	}
	return true
}

// runeCuts maps two fuzz offsets onto ordered rune boundaries inside text.
func runeCuts(text string, first, second int) []int {
	var boundaries []int
	for index := range text {
		if index > 0 {
			boundaries = append(boundaries, index)
		}
	}
	if len(boundaries) == 0 {
		return nil
	}
	a, b := boundaries[first%len(boundaries)], boundaries[second%len(boundaries)]
	if a > b {
		a, b = b, a
	}
	if a == b {
		return []int{a}
	}
	return []int{a, b}
}
