package runprotocol_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/runprotocol"
)

func TestGoBenchmarkSourceConservesExactNativeRangesAndValues(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		present    bool
		iterations int64
		time       float64
	}{
		{name: "ordinary", body: "BenchmarkRow-8 17 0.125 ns/op 7 B/op 3 allocs/op\n", present: true, iterations: 17, time: 0.125},
		{name: "final source without LF", body: "BenchmarkRow-8 17 0.125 ns/op 7 B/op 3 allocs/op", present: true, iterations: 17, time: 0.125},
		{name: "CRLF", body: "BenchmarkRow-8 17 0.125 ns/op 7 B/op 3 allocs/op\r\n", present: true, iterations: 17, time: 0.125},
		{name: "Unicode separators", body: "BenchmarkRow-8\u00a017\u00a00.125\u00a0ns/op\u00a07\u00a0B/op\u00a03\u00a0allocs/op\n", present: true, iterations: 17, time: 0.125},
		{name: "zero iterations", body: "BenchmarkRow-8 0 0.125 ns/op 7 B/op 3 allocs/op\n", present: true, time: 0.125},
		{name: "zero time", body: "BenchmarkRow-8 17 0 ns/op 7 B/op 3 allocs/op\n", present: true, iterations: 17},
		{name: "large finite time", body: "BenchmarkRow-8 17 1e308 ns/op 7 B/op 3 allocs/op\n", present: true, iterations: 17, time: 1e308},
		{name: "subnormal time", body: "BenchmarkRow-8 17 5e-324 ns/op 7 B/op 3 allocs/op\n", present: true, iterations: 17, time: math.SmallestNonzeroFloat64},
		{name: "blank row", body: " \t\n"},
		{name: "metadata", body: "goos: linux\n"},
		{name: "arbitrary unrelated source", body: "unrelated\x00\xff source\n"},
		{name: "nonzero native start", body: "BenchmarkRow-8 17 0.125 ns/op 7 B/op 3 allocs/op\n", present: true, iterations: 17, time: 0.125},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			prefix := "ignored prefix\n"
			tail := "PASS\n"
			wantRecords := 2
			if !strings.HasSuffix(tc.body, "\n") {
				tail = ""
				wantRecords = 1
			}
			source := strings.NewReader(prefix + tc.body + tail)
			if _, err := source.Seek(int64(len(prefix)), io.SeekStart); err != nil {
				t.Fatal(err)
			}
			observed := 0
			for record, err := range runprotocol.GoBenchmarkRecords(t.Context(), runprotocol.GoBenchmarkSourceRequest{Source: source}) {
				if err != nil {
					t.Fatal(err)
				}
				extent := record.SourceExtent()
				size, err := extent.Bytes.Int64()
				if err != nil {
					t.Fatal(err)
				}
				got := make([]byte, int(size))
				if _, err := source.ReadAt(got, int64(extent.Offset)); err != nil {
					t.Fatal(err)
				}
				if observed == 0 {
					if string(got) != tc.body {
						t.Fatalf("range=%q want %q", got, tc.body)
					}
					want := runprotocol.GoBenchmarkRecordAbsent
					if tc.present {
						want = runprotocol.GoBenchmarkRecordPresent
					}
					if record.Presence != want || record.Iterations != tc.iterations || record.Nanoseconds != tc.time {
						t.Fatalf("record=%+v want %+v", record, tc)
					}
					if tc.present && (record.Name().String() != "BenchmarkRow-8" || record.Bytes != 7 || record.Allocations != 3) {
						t.Fatalf("native facts=%+v", record)
					}
				} else if observed == 1 {
					if string(got) != "PASS\n" || record.Presence != runprotocol.GoBenchmarkRecordAbsent {
						t.Fatalf("tail=%q/%+v", got, record)
					}
				} else {
					t.Fatal("extra record")
				}
				observed++
			}
			if observed != wantRecords {
				t.Fatalf("observations=%d want%d", observed, wantRecords)
			}
		})
	}
}

func TestGoBenchmarkSourceRefusesMalformedRowsWithoutPublishedFacts(t *testing.T) {
	t.Parallel()
	for _, row := range []string{"BenchmarkRow\n", "BenchmarkRow 1\n", "BenchmarkRow x 5 ns/op\n", "BenchmarkRow -1 5 ns/op\n", "BenchmarkRow 9223372036854775808 5 ns/op\n", "BenchmarkRow 1 5\n", "BenchmarkRow 1 5 ns/op 7\n", "BenchmarkRow 1 -5 ns/op\n", "BenchmarkRow 1 NaN ns/op\n", "BenchmarkRow 1 +Inf ns/op\n", "BenchmarkRow 1 1e309 ns/op\n", "BenchmarkRow 1 1e+ ns/op\n", "BenchmarkRow 1 1e ns/op\n", "BenchmarkRow 1 1.2.3 ns/op\n", "BenchmarkRow 1 5 ns/op -1 B/op\n", "BenchmarkRow 1 5 ns/op -1 allocs/op\n", "BenchmarkRow 1 5 ns/op 1,024 B/op\n", "BenchmarkRow\x00 1 5 ns/op\n", "BenchmarkRow\xff 1 5 ns/op\n"} {
		t.Run(strconv.Quote(row), func(t *testing.T) {
			t.Parallel()
			observed := 0
			for record, err := range runprotocol.GoBenchmarkRecords(t.Context(), runprotocol.GoBenchmarkSourceRequest{Source: strings.NewReader(row)}) {
				observed++
				if err != nil || record.Presence != runprotocol.GoBenchmarkRecordRefused || record.Refusal() != core.ErrGoToolchainOutput || record.Name().String() != "" || record.Iterations != 0 || record.Fields != runprotocol.GoBenchmarkMetricFieldsNone || record.SourceExtent().Bytes.Uint64() != uint64(len(row)) {
					t.Fatalf("refusal=%+v/%v", record, err)
				}
			}
			if observed != 1 {
				t.Fatalf("refusals=%d", observed)
			}
		})
	}
}

func TestGoBenchmarkSourceWorkingMemoryDoesNotFollowFieldOrRecordExtent(t *testing.T) {
	// witness:waiver test/parallel/default -- process-wide allocation accounting excludes parallel fixture work.
	for _, tc := range []struct {
		name, row string
		refused   bool
	}{
		{name: "unknown field", row: "BenchmarkMemory 1 " + strings.Repeat("9", 1<<20) + " custom 5 ns/op\n"},
		{name: "iteration field", row: "BenchmarkMemory " + strings.Repeat("0", 1<<20) + "1 5 ns/op\n"},
		{name: "float field", row: "BenchmarkMemory 1 1." + strings.Repeat("0", 1<<20) + " ns/op\n"},
		{name: "overflow field", row: "BenchmarkMemory 1 9." + strings.Repeat("0", 1<<20) + "e308 ns/op\n", refused: true},
		{name: "malformed field", row: "BenchmarkMemory 1 1." + strings.Repeat("0", 1<<20) + "x ns/op\n", refused: true},
		{name: "refused long name", row: "Benchmark" + strings.Repeat("a", 1<<20) + " 1 invalid ns/op\n", refused: true},
		{name: "invalid long name", row: "Benchmark" + strings.Repeat("a", 1<<20) + "\x00 1 5 ns/op\n", refused: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// witness:waiver test/parallel/default -- serial source allocation-byte measurement.
			source := strings.NewReader(tc.row)
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			count := 0
			for record, err := range runprotocol.GoBenchmarkRecords(t.Context(), runprotocol.GoBenchmarkSourceRequest{Source: source}) {
				count++
				if err != nil || (record.Presence == runprotocol.GoBenchmarkRecordRefused) != tc.refused {
					t.Fatalf("refusal=%v want%t", err, tc.refused)
				}
				if tc.refused && (record.Name().String() != "" || record.Iterations != 0 || record.Fields != runprotocol.GoBenchmarkMetricFieldsNone || record.Refusal() != core.ErrGoToolchainOutput) {
					t.Fatal("refusal leaked facts")
				}
			}
			runtime.ReadMemStats(&after)
			if count != 1 {
				t.Fatalf("records=%d want1", count)
			}
			if used := after.TotalAlloc - before.TotalAlloc; used > 128<<10 {
				t.Fatalf("source-sized field allocation: %d bytes", used)
			}
		})
	}
}

type benchmarkSourcePressure struct {
	*strings.Reader
	chunk              int
	readErr, readAtErr error
	cancel             context.CancelFunc
	readCalls          int
	readFailureAfter   int
	readAtErrorOffset  int64
}

func (s *benchmarkSourcePressure) Read(destination []byte) (int, error) {
	s.readCalls++
	if s.chunk > 0 && len(destination) > s.chunk {
		destination = destination[:s.chunk]
	}
	n, err := s.Reader.Read(destination)
	if s.cancel != nil {
		s.cancel()
	}
	if s.readErr != nil && s.readCalls > s.readFailureAfter {
		return n, s.readErr
	}
	return n, err
}
func (s *benchmarkSourcePressure) ReadAt(destination []byte, offset int64) (int, error) {
	n, err := s.Reader.ReadAt(destination, offset)
	if s.readAtErr != nil && (s.readAtErrorOffset == 0 || offset == s.readAtErrorOffset) {
		return n, s.readAtErr
	}
	return n, err
}

func TestGoBenchmarkSourcePartialReadsCancellationAndCauses(t *testing.T) {
	t.Parallel()
	cause := io.ErrUnexpectedEOF
	for _, tc := range []struct {
		name                                                          string
		chunk                                                         int
		reader, rangeError                                            error
		preCancel, duringCancel, empty, unitRangeOnly, valueRangeOnly bool
	}{
		{name: "single byte reads", chunk: 1}, {name: "three byte reads", chunk: 3}, {name: "native empty source", empty: true},
		{name: "source failure before row end", reader: cause, chunk: 5}, {name: "wrapped EOF is source failure", reader: errors.Join(core.ErrLineIOScan, io.EOF), chunk: 5},
		{name: "full range count does not hide source failure", rangeError: cause}, {name: "wrapped range EOF is source failure", rangeError: errors.Join(core.ErrLineIOScan, io.EOF)},
		{name: "full metric-unit count must preserve source failure", rangeError: cause, unitRangeOnly: true},
		{name: "full metric-value count must preserve source failure", rangeError: cause, valueRangeOnly: true},
		{name: "cancel before read", preCancel: true}, {name: "cancel during physical read", duringCancel: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			body := "BenchmarkCause-8 1 5 ns/op\n"
			if tc.empty {
				body = ""
			}
			source := &benchmarkSourcePressure{Reader: strings.NewReader(body), chunk: tc.chunk, readErr: tc.reader, readAtErr: tc.rangeError}
			if tc.unitRangeOnly {
				source.readAtErrorOffset = int64(strings.Index(body, "ns/op"))
			}
			if tc.valueRangeOnly {
				source.readAtErrorOffset = int64(strings.Index(body, " 5 ") + 1)
			}
			if tc.preCancel {
				cancel()
			}
			if tc.duringCancel {
				source.cancel = cancel
			}
			records := 0
			var gotErr error
			for record, err := range runprotocol.GoBenchmarkRecords(ctx, runprotocol.GoBenchmarkSourceRequest{Source: source}) {
				if err != nil {
					gotErr = err
					if record != (runprotocol.GoBenchmarkRecord{}) {
						t.Fatal("source failure leaked facts")
					}
				} else {
					records++
				}
			}
			switch {
			case tc.preCancel || tc.duringCancel:
				if !errors.Is(gotErr, context.Canceled) || records != 0 {
					t.Fatalf("cancellation=%v records%d", gotErr, records)
				}
				if tc.preCancel && source.readCalls != 0 {
					t.Fatal("pre-canceled source was read")
				}
			case tc.reader != nil:
				if !errors.Is(gotErr, tc.reader) || records != 0 {
					t.Fatalf("source refusal=%v records%d want cause%v", gotErr, records, tc.reader)
				}
			case tc.rangeError != nil:
				if !errors.Is(gotErr, tc.rangeError) || records != 0 {
					t.Fatalf("range refusal=%v records%d want cause%v", gotErr, records, tc.rangeError)
				}
			case tc.empty:
				if gotErr != nil || records != 0 {
					t.Fatalf("empty=%v records%d", gotErr, records)
				}
			default:
				if gotErr != nil || records != 1 {
					t.Fatalf("chunked=%v records%d", gotErr, records)
				}
			}
		})
	}
}

func FuzzGoBenchmarkSourceMatchesIndependentGoDecimalFacts(f *testing.F) {
	for _, value := range []string{"0", "0.125", "1e308", "1e309", "-1", "5e-324", "1.00000000000000011102230246251565404236316680908203125" + strings.Repeat("0", 900) + "1"} {
		f.Add(value, uint8(1))
		f.Add(value, uint8(7))
	}
	f.Fuzz(func(t *testing.T, value string, chunk uint8) {
		if value == "" {
			return
		}
		for _, c := range value {
			if !(c >= '0' && c <= '9' || c == '.' || c == 'e' || c == 'E' || c == '+' || c == '-') {
				return
			}
		}
		want, wantErr := strconv.ParseFloat(value, 64)
		accepted := wantErr == nil && want >= 0 && !math.IsNaN(want) && !math.IsInf(want, 0)
		row := "BenchmarkOracle-8 17 " + value + " ns/op\n"
		source := &benchmarkSourcePressure{Reader: strings.NewReader(row), chunk: int(chunk) + 1}
		observed := 0
		for record, err := range runprotocol.GoBenchmarkRecords(t.Context(), runprotocol.GoBenchmarkSourceRequest{Source: source}) {
			observed++
			if err != nil || (record.Presence == runprotocol.GoBenchmarkRecordPresent) != accepted {
				t.Fatalf("decimal%q admission%v wantGo%v/%v", value, err, want, wantErr)
			}
			if !accepted {
				if err != nil || record.Presence != runprotocol.GoBenchmarkRecordRefused || record.Refusal() != core.ErrGoToolchainOutput || record.Name().String() != "" || record.Iterations != 0 || record.Fields != runprotocol.GoBenchmarkMetricFieldsNone || record.SourceExtent().Bytes.Uint64() != uint64(len(row)) {
					t.Fatalf("refusal=%+v/%v", record, err)
				}
				continue
			}
			if math.Float64bits(record.Nanoseconds) != math.Float64bits(want) || record.Iterations != 17 || record.Name().String() != "BenchmarkOracle-8" || record.SourceExtent().Bytes.Uint64() != uint64(len(row)) {
				t.Fatalf("native=%+v wantGoFloat%x extent%d", record, math.Float64bits(want), len(row))
			}
		}
		if observed != 1 {
			t.Fatalf("observations=%d want1", observed)
		}
	})
}

func TestGoBenchmarkSourceStopsWithoutFurtherNativeReads(t *testing.T) {
	t.Parallel()
	source := &benchmarkSourcePressure{Reader: strings.NewReader("BenchmarkOne 1 5 ns/op\n" + strings.Repeat("padding\n", 8192)), chunk: 1}
	for record, err := range runprotocol.GoBenchmarkRecords(t.Context(), runprotocol.GoBenchmarkSourceRequest{Source: source}) {
		if err != nil || record.Name().String() != "BenchmarkOne" {
			t.Fatalf("first=%+v/%v", record, err)
		}
		break
	}
	if source.readCalls != len("BenchmarkOne 1 5 ns/op\n") {
		t.Fatalf("stopped consumer caused%d physical reads", source.readCalls)
	}
}

func TestGoBenchmarkSourceExtentCopiesExactPhysicalBytes(t *testing.T) {
	t.Parallel()
	body := []byte("goos: linux\nBenchmarkRange/λ-8 1 0.125 ns/op\r\nPASS\n")
	source := bytes.NewReader(body)
	var copied bytes.Buffer
	for record, err := range runprotocol.GoBenchmarkRecords(t.Context(), runprotocol.GoBenchmarkSourceRequest{Source: source}) {
		if err != nil {
			t.Fatal(err)
		}
		extent := record.SourceExtent()
		length, err := extent.Bytes.Int64()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(&copied, io.NewSectionReader(source, int64(extent.Offset), length)); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(copied.Bytes(), body) {
		t.Fatalf("native ranges lost source bytes: got%q want%q", copied.Bytes(), body)
	}
}

func TestGoBenchmarkSourceReadsNativeScratchScopeWithoutBorrowEscape(t *testing.T) {
	t.Parallel()
	parent, err := core.ParseAbsolutePath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	err = filestore.WithScratchScope(t.Context(), filestore.ScratchScopeRequest{Parent: parent, Use: func(ctx context.Context, root *os.Root) error {
		path, err := core.ParseRelativePath("benchmark-source")
		if err != nil {
			return err
		}
		body := "BenchmarkNative/λ-8 1 0.125 ns/op 7 B/op 3 allocs/op\n"
		receipt, err := filestore.WithScratchReplayScope(ctx, filestore.ScratchReplayScopeRequest{Scratch: filestore.ScratchRequest{Location: filestore.Location{Root: root, Path: path}, Mode: 0600}, Use: func(ctx context.Context, source filestore.ScratchReplayFile) error {
			if _, err := filestore.CopyContent(ctx, filestore.CopyContentRequest{Destination: source, Source: strings.NewReader(body)}); err != nil {
				return err
			}
			if _, err := source.Seek(0, io.SeekStart); err != nil {
				return err
			}
			observed := 0
			for record, err := range runprotocol.GoBenchmarkRecords(ctx, runprotocol.GoBenchmarkSourceRequest{Source: source}) {
				if err != nil {
					return err
				}
				observed++
				if record.Name().String() != "BenchmarkNative/λ-8" || record.Nanoseconds != 0.125 || record.Bytes != 7 || record.Allocations != 3 {
					t.Fatalf("native scratch projection=%+v", record)
				}
				extent := record.SourceExtent()
				length, err := extent.Bytes.Int64()
				if err != nil {
					return err
				}
				var copied strings.Builder
				if _, err := filestore.CopyContent(ctx, filestore.CopyContentRequest{Destination: &copied, Source: io.NewSectionReader(source, int64(extent.Offset), length)}); err != nil {
					return err
				}
				if copied.String() != body {
					t.Fatalf("native scratch range lost source bytes: %q", copied.String())
				}
			}
			if observed != 1 {
				t.Fatalf("native scratch records=%d want1", observed)
			}
			return nil
		}})
		if err != nil {
			return err
		}
		return errors.Join(receipt.OperationError(), receipt.CleanupError())
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGoBenchmarkSourceContinuesAfterCompleteSemanticRefusal(t *testing.T) {
	t.Parallel()
	for _, bad := range []string{"BenchmarkBad x 5 ns/op\n", "BenchmarkBad 1 1e+ ns/op\n", "BenchmarkBad\x00 1 5 ns/op\n", "BenchmarkBad x " + strings.Repeat("tail", 32768) + " custom\n"} {
		t.Run(strconv.Quote(bad[:min(len(bad), 32)]), func(t *testing.T) {
			t.Parallel()
			good := "BenchmarkGood 17 0.125 ns/op\n"
			source := &benchmarkSourcePressure{Reader: strings.NewReader(bad + good), chunk: 3}
			count := 0
			for record, err := range runprotocol.GoBenchmarkRecords(t.Context(), runprotocol.GoBenchmarkSourceRequest{Source: source}) {
				if err != nil {
					t.Fatal(err)
				}
				switch count {
				case 0:
					if record.Presence != runprotocol.GoBenchmarkRecordRefused || record.Refusal() != core.ErrGoToolchainOutput || record.SourceExtent().Bytes.Uint64() != uint64(len(bad)) {
						t.Fatalf("refused=%+v", record)
					}
				case 1:
					if record.Presence != runprotocol.GoBenchmarkRecordPresent || record.Name().String() != "BenchmarkGood" || record.Iterations != 17 || record.Nanoseconds != 0.125 || int64(record.SourceExtent().Offset) != int64(len(bad)) {
						t.Fatalf("continued=%+v", record)
					}
				default:
					t.Fatal("extra observation")
				}
				count++
			}
			if count != 2 {
				t.Fatalf("observations=%d want2", count)
			}
		})
	}
}

func TestGoBenchmarkSourceRefusalDrainPreservesPhysicalFailure(t *testing.T) {
	t.Parallel()
	source := &benchmarkSourcePressure{Reader: strings.NewReader("BenchmarkBad x more fields without row end"), chunk: 1, readErr: io.ErrUnexpectedEOF, readFailureAfter: len("BenchmarkBad x ") + 5}
	count := 0
	for record, err := range runprotocol.GoBenchmarkRecords(t.Context(), runprotocol.GoBenchmarkSourceRequest{Source: source}) {
		count++
		if record != (runprotocol.GoBenchmarkRecord{}) || !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("physical failure=%+v/%v", record, err)
		}
	}
	if count != 1 {
		t.Fatalf("observations=%d", count)
	}
}
