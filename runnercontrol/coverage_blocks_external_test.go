package runnercontrol_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
)

func TestCoverageBlocksPreserveGrammarAndSourceRefusals(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, body       string
		statements, hits int64
		mode             runnercontrol.CoverageMode
		refused          bool
	}{
		{name: "set", body: "mode: set\nx.go:1.1,2.2 3 1", statements: 3, hits: 1, mode: runnercontrol.CoverageSet},
		{name: "count", body: "mode: count\nx.go:1.1,2.2 3 8\n", statements: 3, hits: 8, mode: runnercontrol.CoverageCount},
		{name: "atomic", body: "mode: atomic\nx.go:1.1,2.2 3 8\r\n", statements: 3, hits: 8, mode: runnercontrol.CoverageAtomic},
		{name: "drive colon", body: "mode: set\nC:x:1.1,2.2 3 1", statements: 3, hits: 1, mode: runnercontrol.CoverageSet},
		{name: "zero counts", body: "mode: set\nx.go:0.0,0.0 0 0", mode: runnercontrol.CoverageSet},
		{name: "empty", refused: true},
		{name: "unknown mode", body: "mode: unknown\n", refused: true},
		{name: "missing position", body: "mode: set\n1.1,2.2 3 1", refused: true},
		{name: "empty filename", body: "mode: set\n:1.1,2.2 3 1", refused: true},
		{name: "missing comma", body: "mode: set\nx:1.1.2.2 3 1", refused: true},
		{name: "missing coordinate", body: "mode: set\nx:1.,2.2 3 1", refused: true},
		{name: "signed coordinate", body: "mode: set\nx:+1.1,2.2 3 1", refused: true},
		{name: "overflow coordinate", body: "mode: set\nx:9223372036854775808.1,2.2 3 1", refused: true},
		{name: "signed statements", body: "mode: set\nx:1.1,2.2 +3 1", refused: true},
		{name: "overflow statements", body: "mode: set\nx:1.1,2.2 9223372036854775808 1", refused: true},
		{name: "overflow hits", body: "mode: count\nx:1.1,2.2 1 9223372036854775808", refused: true},
		{name: "set multiplicity", body: "mode: set\nx:1.1,2.2 1 2", refused: true},
		{name: "four fields", body: "mode: set\nx:1.1,2.2 1 1 1", refused: true},
		{name: "missing hits", body: "mode: set\nx:1.1,2.2 1\n", refused: true},
		{name: "NUL cannot terminate record", body: "mode: set\nx:1.1,2.2 1 1\x00hidden", refused: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, source := range []io.Reader{strings.NewReader(tc.body), iotest.OneByteReader(strings.NewReader(tc.body))} {
				var observed runnercontrol.CoverageBlock
				count := 0
				var failure error
				for block, err := range runnercontrol.CoverageBlocks(t.Context(), runnercontrol.CoverageBlockRequest{Source: source}) {
					if err != nil {
						failure = err
						break
					}
					observed = block
					count++
				}
				if tc.refused {
					if failure == nil {
						t.Fatalf("invalid source accepted with %d blocks", count)
					}
					continue
				}
				want := runnercontrol.CoverageBlock{Mode: tc.mode, Statements: tc.statements, Hits: tc.hits}
				if failure != nil || count != 1 || observed != want {
					t.Fatalf("blocks=%d observation=%+v error=%v want=%+v", count, observed, failure, want)
				}
			}
		})
	}
}
func TestCoverageBlocksHeaderOnlyIsNeutralAndContextIsAuthoritative(t *testing.T) {
	t.Parallel()
	for _, body := range []string{"mode: set", "\n\tmode: atomic\n\n ", "mode: count" + strings.Repeat(" ", 2<<20)} {
		for block, err := range runnercontrol.CoverageBlocks(t.Context(), runnercontrol.CoverageBlockRequest{Source: strings.NewReader(body)}) {
			t.Fatalf("neutral header published %+v/%v", block, err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, request := range []runnercontrol.CoverageBlockRequest{{}, {Source: strings.NewReader("mode: set")}} {
		for _, err := range runnercontrol.CoverageBlocks(ctx, request) {
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("terminal context error=%v", err)
			}
		}
	}
	for _, block := range []runnercontrol.CoverageBlock{{}, {Mode: runnercontrol.CoverageMode(255)}, {Mode: runnercontrol.CoverageSet, Hits: 2}, {Mode: runnercontrol.CoverageCount, Statements: -1}} {
		if !errors.Is(block.Validate(), core.ErrPrimitiveContract) {
			t.Fatalf("invalid block admitted: %+v", block)
		}
	}
}

func FuzzCoverageBlocksFragmentationPreservesKnownCounts(f *testing.F) {
	f.Add(uint8(0), uint16(1), uint16(3))
	f.Add(uint8(1), uint16(4096), uint16(0))
	f.Add(uint8(2), uint16(65535), uint16(19))
	f.Fuzz(func(t *testing.T, selector uint8, extent, hits uint16) {
		modes := [...]runnercontrol.CoverageMode{runnercontrol.CoverageSet, runnercontrol.CoverageCount, runnercontrol.CoverageAtomic}
		mode := modes[selector%uint8(len(modes))]
		count := int64(hits)
		if mode == runnercontrol.CoverageSet {
			count %= 2
		}
		// The expected numeric facts are fixture inputs, never reparsed source.
		body := "mode: " + mode.String() + "\n" + strings.Repeat("x", int(extent)+1) + ":1.1,2.2 3 " + decimalCoverageFixture(count) + "\n"
		for _, source := range []io.Reader{strings.NewReader(body), iotest.HalfReader(strings.NewReader(body))} {
			observed := 0
			for block, err := range runnercontrol.CoverageBlocks(t.Context(), runnercontrol.CoverageBlockRequest{Source: source}) {
				if err != nil || block != (runnercontrol.CoverageBlock{Mode: mode, Statements: 3, Hits: count}) {
					t.Fatalf("fragment projection=%+v/%v", block, err)
				}
				observed++
			}
			if observed != 1 {
				t.Fatalf("observed=%d", observed)
			}
		}
	})
}
func TestCoverageBlocksPreserveInterruptedSourceIdentity(t *testing.T) {
	t.Parallel()
	for _, failure := range []error{io.ErrUnexpectedEOF, errors.Join(io.EOF, io.ErrClosedPipe)} {
		source := io.MultiReader(strings.NewReader("mode: set\nx:1.1,2.2 3 1\n"), iotest.ErrReader(failure))
		observed := 0
		var refusal error
		for _, err := range runnercontrol.CoverageBlocks(t.Context(), runnercontrol.CoverageBlockRequest{Source: source}) {
			if err != nil {
				refusal = err
				break
			}
			observed++
		}
		if observed != 1 || !errors.Is(refusal, failure) || !errors.Is(refusal, core.ErrLineIOScan) {
			t.Fatalf("prefix=%d refusal=%v want source=%v", observed, refusal, failure)
		}
	}
	for _, request := range []runnercontrol.CoverageBlockRequest{{}, {Source: (*strings.Reader)(nil)}} {
		observed := 0
		for _, err := range runnercontrol.CoverageBlocks(t.Context(), request) {
			observed++
			if !errors.Is(err, core.ErrLineIOContract) {
				t.Fatalf("invalid source error=%v", err)
			}
		}
		if observed != 1 {
			t.Fatalf("invalid source results=%d", observed)
		}
	}
}

func decimalCoverageFixture(value int64) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	offset := len(digits)
	for value > 0 {
		offset--
		digits[offset] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[offset:])
}
func BenchmarkCoverageBlocksIgnoredLocation(b *testing.B) {
	for _, extent := range []int{2 << 10, 2 << 20} {
		b.Run(decimalCoverageFixture(int64(extent)), func(b *testing.B) {
			body := "mode: count\n" + strings.Repeat("x", extent) + ":1.1,2.2 3 7\n"
			b.SetBytes(int64(len(body)))
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				count := 0
				for block, err := range runnercontrol.CoverageBlocks(b.Context(), runnercontrol.CoverageBlockRequest{Source: strings.NewReader(body)}) {
					if err != nil || block.Statements != 3 || block.Hits != 7 {
						b.Fatalf("projection=%+v/%v", block, err)
					}
					count++
				}
				if count != 1 {
					b.Fatalf("blocks=%d", count)
				}
			}
		})
	}
}
