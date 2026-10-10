package gotoolchain_test

import (
	"errors"
	"math"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestGoBenchmarkRecordConsumesActualGoFormatter(t *testing.T) {
	t.Parallel()
	for _, result := range []testing.BenchmarkResult{
		{N: 1, T: 125 * time.Nanosecond, MemBytes: 7, MemAllocs: 3},
		{N: 2, T: 125 * time.Nanosecond, MemBytes: 128, MemAllocs: 4},
		{N: 100, T: time.Microsecond, MemBytes: 4096, MemAllocs: 100},
	} {
		t.Run(strconv.Itoa(result.N), func(t *testing.T) {
			t.Parallel()
			source := "BenchmarkNative-8 " + result.String() + " " + result.MemString()
			fields := strings.Fields(source)
			wantTime, err := strconv.ParseFloat(fields[2], 64)
			if err != nil {
				t.Fatal(err)
			}
			got, err := gotoolchain.ObserveGoBenchmarkRecord(gotoolchain.GoBenchmarkRecordRequest{Source: []byte(source)})
			if err != nil || got.Name().String() != "BenchmarkNative-8" || got.Iterations != int64(result.N) || math.Float64bits(got.Nanoseconds) != math.Float64bits(wantTime) || got.Bytes != int64(result.MemBytes)/int64(result.N) || got.Allocations != int64(result.MemAllocs)/int64(result.N) {
				t.Fatalf("Go formatter %q projected %+v/%v, want exact native name/count/value facts", source, got, err)
			}
		})
	}
}

func TestGoBenchmarkRecordNameDoesNotBorrowMutableSource(t *testing.T) {
	t.Parallel()
	source := []byte("BenchmarkStable-8 1 5 ns/op")
	got, err := gotoolchain.ObserveGoBenchmarkRecord(gotoolchain.GoBenchmarkRecordRequest{Source: source})
	if err != nil {
		t.Fatal(err)
	}
	source[0] = 'X'
	if got.Name().String() != "BenchmarkStable-8" {
		t.Fatalf("observed identity changed after source reuse: %q", got.Name().String())
	}
}

func TestGoBenchmarkRecordNativeBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source                   string
		presence                       gotoolchain.GoBenchmarkRecordPresence
		fields                         gotoolchain.GoBenchmarkMetricFields
		iterations, bytes, allocations int64
		nanoseconds                    float64
		refused                        bool
	}{
		{name: "empty record", presence: gotoolchain.GoBenchmarkRecordAbsent},
		{name: "Go platform metadata", source: "goos: linux", presence: gotoolchain.GoBenchmarkRecordAbsent},
		{name: "Go successful exit", source: "PASS", presence: gotoolchain.GoBenchmarkRecordAbsent},
		{name: "Go failure exit", source: "FAIL", presence: gotoolchain.GoBenchmarkRecordAbsent},
		{name: "unrelated name", source: "Other 1 5 ns/op", presence: gotoolchain.GoBenchmarkRecordAbsent},
		{name: "ordinary benchmark", source: "BenchmarkOne-8 10 5 ns/op", presence: gotoolchain.GoBenchmarkRecordPresent, fields: gotoolchain.GoBenchmarkMetricTime, iterations: 10, nanoseconds: 5},
		{name: "fraction remains fractional", source: "BenchmarkOne-8 1 0.125 ns/op", presence: gotoolchain.GoBenchmarkRecordPresent, fields: gotoolchain.GoBenchmarkMetricTime, iterations: 1, nanoseconds: 0.125},
		{name: "explicit zero time", source: "BenchmarkOne 1 0 ns/op", presence: gotoolchain.GoBenchmarkRecordPresent, fields: gotoolchain.GoBenchmarkMetricTime, iterations: 1},
		{name: "absent time differs from zero", source: "BenchmarkOne 1 7 MB/s", presence: gotoolchain.GoBenchmarkRecordPresent, iterations: 1},
		{name: "zero iterations", source: "BenchmarkOne 0 5 ns/op", presence: gotoolchain.GoBenchmarkRecordPresent, fields: gotoolchain.GoBenchmarkMetricTime, nanoseconds: 5},
		{name: "all measured fields", source: "BenchmarkOne/case-8 20 1.5 ns/op 7 B/op 3 allocs/op", presence: gotoolchain.GoBenchmarkRecordPresent, fields: gotoolchain.GoBenchmarkMetricTime | gotoolchain.GoBenchmarkMetricBytes | gotoolchain.GoBenchmarkMetricAllocations, iterations: 20, nanoseconds: 1.5, bytes: 7, allocations: 3},
		{name: "unknown fields do not acquire known authority", source: "BenchmarkOne 1 999 MB/s 5 ns/op 999 custom", presence: gotoolchain.GoBenchmarkRecordPresent, fields: gotoolchain.GoBenchmarkMetricTime, iterations: 1, nanoseconds: 5},
		{name: "last native field value", source: "BenchmarkOne 1 5 ns/op 7 ns/op", presence: gotoolchain.GoBenchmarkRecordPresent, fields: gotoolchain.GoBenchmarkMetricTime, iterations: 1, nanoseconds: 7},
		{name: "tab columns", source: "BenchmarkOne\t1\t5\tns/op", presence: gotoolchain.GoBenchmarkRecordPresent, fields: gotoolchain.GoBenchmarkMetricTime, iterations: 1, nanoseconds: 5},
		{name: "CRLF record", source: "BenchmarkOne 1 5 ns/op\r\n", presence: gotoolchain.GoBenchmarkRecordPresent, fields: gotoolchain.GoBenchmarkMetricTime, iterations: 1, nanoseconds: 5},
		{name: "positive signed count", source: "BenchmarkOne +1 +5 ns/op", presence: gotoolchain.GoBenchmarkRecordPresent, fields: gotoolchain.GoBenchmarkMetricTime, iterations: 1, nanoseconds: 5},
		{name: "scientific time", source: "BenchmarkOne 1 1.25e2 ns/op", presence: gotoolchain.GoBenchmarkRecordPresent, fields: gotoolchain.GoBenchmarkMetricTime, iterations: 1, nanoseconds: 125},
		{name: "decimal begins at period", source: "BenchmarkOne 1 .5 ns/op", presence: gotoolchain.GoBenchmarkRecordPresent, fields: gotoolchain.GoBenchmarkMetricTime, iterations: 1, nanoseconds: 0.5},
		{name: "decimal ends at period", source: "BenchmarkOne 1 5. ns/op", presence: gotoolchain.GoBenchmarkRecordPresent, fields: gotoolchain.GoBenchmarkMetricTime, iterations: 1, nanoseconds: 5},
		{name: "integer leading zeros", source: "BenchmarkOne 0001 5 ns/op", presence: gotoolchain.GoBenchmarkRecordPresent, fields: gotoolchain.GoBenchmarkMetricTime, iterations: 1, nanoseconds: 5},
		{name: "signed integer zero", source: "BenchmarkOne -0 5 ns/op", presence: gotoolchain.GoBenchmarkRecordPresent, fields: gotoolchain.GoBenchmarkMetricTime, nanoseconds: 5},
		{name: "maximum signed iterations", source: "BenchmarkOne 9223372036854775807 5 ns/op", presence: gotoolchain.GoBenchmarkRecordPresent, fields: gotoolchain.GoBenchmarkMetricTime, iterations: math.MaxInt64, nanoseconds: 5},
		{name: "iteration overflow", source: "BenchmarkOne 9223372036854775808 5 ns/op", refused: true},
		{name: "negative iteration", source: "BenchmarkOne -1 5 ns/op", refused: true},
		{name: "missing iterations", source: "BenchmarkOne", refused: true},
		{name: "missing metric pair", source: "BenchmarkOne 1", refused: true},
		{name: "unpaired metric value", source: "BenchmarkOne 1 5", refused: true},
		{name: "nondecimal iteration", source: "BenchmarkOne 1x 5 ns/op", refused: true},
		{name: "negative time", source: "BenchmarkOne 1 -1 ns/op", refused: true},
		{name: "NaN time", source: "BenchmarkOne 1 NaN ns/op", refused: true},
		{name: "positive infinite time", source: "BenchmarkOne 1 +Inf ns/op", refused: true},
		{name: "negative infinite time", source: "BenchmarkOne 1 -Inf ns/op", refused: true},
		{name: "negative bytes", source: "BenchmarkOne 1 -1 B/op", refused: true},
		{name: "negative allocations", source: "BenchmarkOne 1 -1 allocs/op", refused: true},
		{name: "allocation overflow", source: "BenchmarkOne 1 9223372036854775808 allocs/op", refused: true},
		{name: "missing exponent", source: "BenchmarkOne 1 1e ns/op", refused: true},
		{name: "missing signed exponent", source: "BenchmarkOne 1 1e+ ns/op", refused: true},
		{name: "multiple periods", source: "BenchmarkOne 1 1.2.3 ns/op", refused: true},
		{name: "invalid exponent suffix", source: "BenchmarkOne 1 1e2x ns/op", refused: true},
		{name: "invalid UTF8 name", source: "Benchmark\xff 1 5 ns/op", refused: true},
		{name: "control in name", source: "Benchmark\x00 1 5 ns/op", refused: true},
		{name: "float range overflow", source: "BenchmarkOne 1 1e309 ns/op", refused: true},
		{name: "large finite float preserved", source: "BenchmarkOne 1 1e308 ns/op", presence: gotoolchain.GoBenchmarkRecordPresent, fields: gotoolchain.GoBenchmarkMetricTime, iterations: 1, nanoseconds: 1e308},
		{name: "subnormal time", source: "BenchmarkOne 1 5e-324 ns/op", presence: gotoolchain.GoBenchmarkRecordPresent, fields: gotoolchain.GoBenchmarkMetricTime, iterations: 1, nanoseconds: math.SmallestNonzeroFloat64},
		{name: "time underflow is native zero", source: "BenchmarkOne 1 1e-325 ns/op", presence: gotoolchain.GoBenchmarkRecordPresent, fields: gotoolchain.GoBenchmarkMetricTime, iterations: 1},
		{name: "huge positive zero exponent", source: "BenchmarkOne 1 0e999999999999999999999 ns/op", presence: gotoolchain.GoBenchmarkRecordPresent, fields: gotoolchain.GoBenchmarkMetricTime, iterations: 1},
		{name: "huge negative exponent", source: "BenchmarkOne 1 1e-999999999999999999999 ns/op", presence: gotoolchain.GoBenchmarkRecordPresent, fields: gotoolchain.GoBenchmarkMetricTime, iterations: 1},
		{name: "huge positive nonzero exponent", source: "BenchmarkOne 1 1e999999999999999999999 ns/op", refused: true},
		{name: "integer width does not become source quota", source: "BenchmarkOne " + strings.Repeat("0", 1<<20) + "1 5 ns/op", presence: gotoolchain.GoBenchmarkRecordPresent, fields: gotoolchain.GoBenchmarkMetricTime, iterations: 1, nanoseconds: 5},
		{name: "float width does not become source quota", source: "BenchmarkOne 1 1." + strings.Repeat("0", 1<<20) + " ns/op", presence: gotoolchain.GoBenchmarkRecordPresent, fields: gotoolchain.GoBenchmarkMetricTime, iterations: 1, nanoseconds: 1},
		{name: "unknown value width is ignored", source: "BenchmarkOne 1 " + strings.Repeat("9", 1<<20) + " custom", presence: gotoolchain.GoBenchmarkRecordPresent, iterations: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := gotoolchain.ObserveGoBenchmarkRecord(gotoolchain.GoBenchmarkRecordRequest{Source: []byte(tc.source)})
			if tc.refused {
				if !errors.Is(err, core.ErrGoToolchainOutput) || got != (gotoolchain.GoBenchmarkRecord{}) {
					t.Fatalf("refusal=%+v/%v, want zero and constant", got, err)
				}
				return
			}
			if err != nil || got.Presence != tc.presence || got.Fields != tc.fields || got.Iterations != tc.iterations || got.Nanoseconds != tc.nanoseconds || got.Bytes != tc.bytes || got.Allocations != tc.allocations {
				t.Fatalf("record=%+v/%v, want native facts %+v", got, err, tc)
			}
			if err := got.Validate(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGoBenchmarkRecordNumericFieldsUseConstantWorkingMemory(t *testing.T) {
	// witness:waiver test/parallel/default -- serial allocation-byte measurement owns process heap accounting.
	for _, tc := range []struct {
		name, body string
		refused    bool
	}{
		{name: "long valid integer", body: "BenchmarkMemory " + strings.Repeat("0", 1<<20) + "1 5 ns/op"},
		{name: "long valid finite float", body: "BenchmarkMemory 1 1." + strings.Repeat("0", 1<<20) + "e308 ns/op"},
		{name: "long overflowing float", body: "BenchmarkMemory 1 9." + strings.Repeat("0", 1<<20) + "e308 ns/op", refused: true},
		{name: "long invalid name", body: "Benchmark" + strings.Repeat("a", 1<<20) + "\x00 1 5 ns/op", refused: true},
		{name: "long valid name in refused row", body: "Benchmark" + strings.Repeat("a", 1<<20) + " 1 invalid ns/op", refused: true},
		{name: "long malformed float", body: "BenchmarkMemory 1 1." + strings.Repeat("0", 1<<20) + "x ns/op", refused: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// witness:waiver test/parallel/default -- serial native allocation-byte pressure must exclude parallel fixture work.
			source := []byte(tc.body)
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			got, err := gotoolchain.ObserveGoBenchmarkRecord(gotoolchain.GoBenchmarkRecordRequest{Source: source})
			runtime.ReadMemStats(&after)
			if (err != nil) != tc.refused {
				t.Fatalf("native record refusal=%v want %t", err, tc.refused)
			}
			if tc.refused && got != (gotoolchain.GoBenchmarkRecord{}) {
				t.Fatalf("refusal leaked record %+v", got)
			}
			if bytes := after.TotalAlloc - before.TotalAlloc; bytes > 64<<10 {
				t.Fatalf("decoder copied source-sized numeric text: allocated=%d, want fixed working window", bytes)
			}
		})
	}
}

func FuzzGoBenchmarkRecordDecimalValueMatchesGo(f *testing.F) {
	for _, value := range []string{"0", "1.25", "1e308", "1e309", "5e-324", "1.7976931348623157e308", "2.2250738585072014e-308", "1.00000000000000011102230246251565404236316680908203125"} {
		f.Add(value)
	}
	for _, value := range []string{"1.00000000000000011102230246251565404236316680908203125" + strings.Repeat("0", 900) + "1", "2." + strings.Repeat("0", 900) + "1e-324", "1" + strings.Repeat("0", 900) + "e-900"} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value string) {
		// Decimal fixture grammar is independent: strconv owns the value oracle;
		// hexadecimal, special words, whitespace and underscore spellings are
		// outside Go's benchmark formatter's decimal output domain.
		if value == "" || strings.ContainsAny(value, "xXpP_ \t\r\n") {
			return
		}
		for _, c := range value {
			if !(c >= '0' && c <= '9' || c == '.' || c == 'e' || c == 'E' || c == '+' || c == '-') {
				return
			}
		}
		want, wantErr := strconv.ParseFloat(value, 64)
		accepted := wantErr == nil && want >= 0 && !math.IsNaN(want) && !math.IsInf(want, 0)
		got, err := gotoolchain.ObserveGoBenchmarkRecord(gotoolchain.GoBenchmarkRecordRequest{Source: []byte("BenchmarkOracle 1 " + value + " ns/op")})
		if (err == nil) != accepted {
			t.Fatalf("decimal %q admission=%v want Go %v/%v", value, err, want, wantErr)
		}
		if !accepted {
			if got != (gotoolchain.GoBenchmarkRecord{}) || !errors.Is(err, core.ErrGoToolchainOutput) {
				t.Fatalf("refusal=%+v/%v", got, err)
			}
			return
		}
		if got.Presence != gotoolchain.GoBenchmarkRecordPresent || got.Fields != gotoolchain.GoBenchmarkMetricTime || math.Float64bits(got.Nanoseconds) != math.Float64bits(want) {
			t.Fatalf("decimal %q native=%+v, want exact Go float bits %x", value, got, math.Float64bits(want))
		}
	})
}
