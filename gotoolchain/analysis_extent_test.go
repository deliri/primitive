package gotoolchain

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"go/types"
	"io"
	"math"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
)

// Start with actual cmd/go metadata and vary only stream extent/identity.
// Reaching a historical default is not malformed compiler output. Explicit
// caller budgets remain enforceable; their exact EOF boundary must not lose
// the last record or accept one extra byte.
func TestAnalysisMetadataExtentIsCallerOwned(t *testing.T) {
	t.Parallel()
	seed, defaults := compilerMetadataSeed(t, t.TempDir())
	var wire analysisPackageWire
	if err := json.Unmarshal(seed, &wire); err != nil {
		t.Fatal(err)
	}
	var stream bytes.Buffer
	count := uint64(DefaultPackageMaximum + 1)
	wire.Name = strings.Repeat("p", 1024)
	for i := range count {
		wire.ImportPath = fmt.Sprintf("example.com/extent/p%d", i)
		encoded, err := json.Marshal(wire)
		if err != nil {
			t.Fatal(err)
		}
		stream.Write(encoded)
		stream.WriteByte('\n')
	}
	if stream.Len() <= DefaultOutputBytes {
		t.Fatalf("fixture bytes = %d, want above default %d", stream.Len(), DefaultOutputBytes)
	}
	for _, tc := range []struct {
		name     string
		bytes    uint64
		packages uint64
		want     error
	}{
		{name: "mechanical stream extent", bytes: math.MaxInt64, packages: math.MaxUint64},
		{name: "exact caller extent", bytes: uint64(stream.Len()), packages: count},
		{name: "caller byte budget is one short", bytes: uint64(stream.Len() - 1), packages: count, want: core.ErrGoToolchainOutput},
		{name: "caller package budget is one short", bytes: uint64(stream.Len()), packages: count - 1, want: core.ErrGoToolchainOutput},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			limits := defaults
			var err error
			limits.OutputBytes, err = core.NewByteCount(tc.bytes)
			if err != nil {
				t.Fatal(err)
			}
			limits.PackageMaximum = tc.packages
			if err := limits.Validate(); err != nil {
				t.Fatalf("representable caller-owned extent refused: %v", err)
			}
			got, err := decodeAnalysisMetadata(bytes.NewReader(stream.Bytes()), limits, types.SizesFor("gc", "arm64"))
			if !errors.Is(err, tc.want) {
				t.Fatalf("compiler stream = %v, want %v", err, tc.want)
			}
			if tc.want != nil {
				if got != nil {
					t.Fatalf("refused stream units = %d, want nil graph", len(got))
				}
				return
			}
			if len(got) != int(count) {
				t.Fatalf("retained %d units, want %d", len(got), count)
			}
			for i, u := range got {
				if u.ID != fmt.Sprintf("example.com/extent/p%d", i) || u.Name != wire.Name || len(u.CompiledGoFiles) != len(wire.CompiledGoFiles) {
					t.Fatalf("unit %d lost compiler identity or membership", i)
				}
			}
		})
	}
}

// EOF accompanied by a transport failure is not clean completion, including
// when the last JSON byte exactly exhausts the caller's declared budget.
func TestAnalysisMetadataEOFCannotHideReaderFailure(t *testing.T) {
	t.Parallel()
	seed, limits := compilerMetadataSeed(t, t.TempDir())
	for _, exact := range []bool{false, true} {
		t.Run(fmt.Sprintf("exact_budget_%t", exact), func(t *testing.T) {
			t.Parallel()
			budget := limits
			if exact {
				var err error
				budget.OutputBytes, err = core.NewByteCount(uint64(len(seed)))
				if err != nil {
					t.Fatal(err)
				}
			}
			for _, failure := range []error{io.ErrUnexpectedEOF, errors.Join(io.EOF, io.ErrUnexpectedEOF)} {
				reader := io.MultiReader(bytes.NewReader(seed), iotest.ErrReader(failure))
				got, err := decodeAnalysisMetadata(reader, budget, types.SizesFor("gc", "arm64"))
				if !errors.Is(err, core.ErrGoToolchainOutput) || !errors.Is(err, io.ErrUnexpectedEOF) || got != nil {
					t.Fatalf("reader failure became completed metadata: %d units/%v", len(got), err)
				}
			}
		})
	}
}
