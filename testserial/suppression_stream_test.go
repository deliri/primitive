package testserial_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"runtime"
	"strings"
	"testing"
	"testing/iotest"

	primitivecore "github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/testserial"
)

type repeatedSuppressionScanByte byte

func (b repeatedSuppressionScanByte) Read(destination []byte) (int, error) {
	for i := range destination {
		destination[i] = byte(b)
	}
	return len(destination), nil
}

func TestSuppressionScanWorkingMemoryDoesNotFollowLineExtent(t *testing.T) {
	testserial.Declare(t, primitivecore.TestIsolationDeclaration{
		Hazard: primitivecore.TestIsolationHazardRuntimeAllocation,
		Scope:  primitivecore.TestIsolationScopePackageProcess,
	})
	pattern := suppressionPattern()
	if _, err := firstSuppressionLine(t.Context(), strings.NewReader("warmup\n"), pattern); err != nil {
		t.Fatal(err)
	}
	for _, extent := range [...]int64{64 << 10, 1 << 20, 16 << 20} {
		prefix := &io.LimitedReader{R: repeatedSuppressionScanByte('x'), N: extent}
		tail := strings.NewReader("\n// " + forbiddenSuppressionDirective)
		source := io.MultiReader(prefix, tail)
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		got, err := firstSuppressionLine(t.Context(), source, pattern)
		runtime.ReadMemStats(&after)
		allocated := after.TotalAlloc - before.TotalAlloc
		if err != nil || got != 2 || prefix.N != 0 || tail.Len() != 0 {
			t.Fatalf("extent %d scan = %d/%v, remaining = %d/%d, want line 2 and fully consumed source", extent, got, err, prefix.N, tail.Len())
		}
		t.Logf("source_bytes=%d scanner_allocated_bytes=%d", extent, allocated)
		if allocated > 128<<10 {
			t.Fatalf("extent %d allocated %d bytes, want fixed windows below %d", extent, allocated, 128<<10)
		}
	}
}

func TestSuppressionScanWithholdsProvisionalMatchOnSourceFailure(t *testing.T) {
	t.Parallel()
	source := io.MultiReader(strings.NewReader("// "+forbiddenSuppressionDirective+"\n"), iotest.ErrReader(io.ErrClosedPipe))
	got, err := firstSuppressionLine(t.Context(), source, suppressionPattern())
	if got != 0 || !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("failed source = %d/%v, want withheld coordinate and native closed-pipe cause", got, err)
	}
}

func TestSuppressionScanCancellationReadsNothing(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	source := strings.NewReader("// " + forbiddenSuppressionDirective)
	before := source.Len()
	got, err := firstSuppressionLine(ctx, source, suppressionPattern())
	if got != 0 || !errors.Is(err, context.Canceled) || source.Len() != before {
		t.Fatalf("cancelled scan = %d/%v, unread=%d, want zero/cancelled/unread=%d", got, err, source.Len(), before)
	}
}

func FuzzSuppressionScanMatchesNativeByteOracle(f *testing.F) {
	f.Add([]byte("// " + forbiddenSuppressionDirective))
	f.Add([]byte("const data = 1\n"))
	f.Add([]byte{})
	pattern := suppressionPattern()
	f.Fuzz(func(t *testing.T, data []byte) {
		var want uint64
		if match := pattern.FindIndex(data); match != nil {
			want = uint64(bytes.Count(data[:match[0]], []byte{'\n'})) + 1
		}
		got, err := firstSuppressionLine(t.Context(), bytes.NewReader(data), pattern)
		if err != nil || got != want {
			t.Fatalf("streamed coordinate = %d/%v, want native byte-oracle coordinate %d/nil", got, err, want)
		}
	})
}
