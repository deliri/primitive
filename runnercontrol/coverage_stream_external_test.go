package runnercontrol_test

import (
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/runnercontrol"
)

func TestGoCoverageContinuesBeyondHistoricalExtents(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		chunk   string
		repeats int
		suffix  string
		want    uint64
	}{
		{name: "eight MiB location uses repeated fixed fragments", chunk: strings.Repeat("x", 4096), repeats: 2048, suffix: ":1.1,1.2 1 1\n", want: 1},
		{name: "more than one million records conserve statements", chunk: strings.Repeat("a.go:1.1,1.2 1 1\n", 256), repeats: 4097, want: 256 * 4097},
		{name: "long leading zero count has no numeric extent quota", chunk: strings.Repeat("0", 4096), repeats: 512, suffix: "1\n", want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compiler := runnercontrol.NewGoCoverageCompiler()
			header := "mode: count\n"
			if tc.suffix == "1\n" {
				header += "a.go:1.1,1.2 1 "
			}
			if _, err := compiler.Write([]byte(header)); err != nil {
				t.Fatalf("Write(header) error = %v, want nil", err)
			}
			chunk := []byte(tc.chunk)
			for range tc.repeats {
				if n, err := compiler.Write(chunk); n != len(chunk) || err != nil {
					t.Fatalf("Write(fragment) = %d/%v, want %d/nil", n, err, len(chunk))
				}
			}
			if _, err := compiler.Write([]byte(tc.suffix)); err != nil {
				t.Fatalf("Write(suffix) error = %v, want nil", err)
			}
			got, err := compiler.Seal()
			want := runnercontrol.GoCoverageObservation{Mode: runnercontrol.CoverageCount, Statements: tc.want, Covered: tc.want, BasisPoints: 10000}
			if err != nil || got != want {
				t.Fatalf("Seal() = %+v/%v, want %+v/nil", got, err, want)
			}
		})
	}
}

var benchmarkCoverageStreamingSink runnercontrol.GoCoverageObservation

func BenchmarkGoCoverageStreaming(b *testing.B) {
	b.ReportAllocs()
	for _, size := range []int{1024, 65536} {
		name := "one KiB location"
		if size == 65536 {
			name = "64 KiB location"
		}
		b.Run(name, func(b *testing.B) {
			data := []byte("mode: count\n" + strings.Repeat("a", size) + ":1.1,1.2 1 1\n")
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for b.Loop() {
				compiler := runnercontrol.NewGoCoverageCompiler()
				if _, err := compiler.Write(data); err != nil {
					b.Fatalf("Write() error = %v, want nil", err)
				}
				got, err := compiler.Seal()
				if err != nil || got.Statements != 1 || got.Covered != 1 {
					b.Fatalf("Seal() = %+v/%v, want one covered statement/nil", got, err)
				}
				benchmarkCoverageStreamingSink = got
			}
		})
	}
}
