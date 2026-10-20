package gotoolchain_test

import (
	"context"
	"errors"
	"go/ast"
	"io"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestSymbolOccurrencesConserveNativeNodeAndAncestry(t *testing.T) {
	t.Parallel()
	analysis := nativeSymbolAnalysis(t, t.TempDir())
	count := 0
	for got, err := range analysis.SymbolOccurrences(t.Context()) {
		if err != nil || got.Validate() != nil {
			t.Fatalf("occurrence=(%+v,%v), want valid compiler fact and syntax loan", got, err)
		}
		identifier, ok := got.Syntax.Node.(*ast.Ident)
		if !ok || identifier.Name != got.Reference.Symbol.Selector.String() || len(got.Syntax.Ancestors) == 0 {
			t.Fatalf("native occurrence=%+v, want matching identifier and ancestry", got)
		}
		found := false
		for _, unit := range analysis.Units {
			if _, ok := unit.TypesInfo.Uses[identifier]; !ok {
				continue
			}
			found = true
			position := unit.Fset.PositionFor(identifier.Pos(), false)
			if position.Filename != got.Reference.File.String() || uint64(position.Offset) != got.Reference.Offset || uint64(position.Line) != got.Reference.Line || uint64(position.Column) != got.Reference.Column {
				t.Fatalf("native node position=%+v, reference=%+v", position, got.Reference)
			}
		}
		if !found {
			t.Fatal("occurrence node is not owned by native type information")
		}
		for index, parent := range got.Syntax.Ancestors {
			var child ast.Node = identifier
			if index+1 < len(got.Syntax.Ancestors) {
				child = got.Syntax.Ancestors[index+1]
			}
			linked := false
			ast.Inspect(parent, func(node ast.Node) bool {
				if node == parent {
					return true
				}
				if node == child {
					linked = true
				}
				return false
			})
			if !linked {
				t.Fatalf("ancestry edge%d is not a direct native child", index)
			}
		}
		count++
	}
	if count != 8 {
		t.Fatalf("native occurrences%d,want8", count)
	}
	if err := (gotoolchain.SymbolOccurrence{}).Validate(); !errors.Is(err, core.ErrGoToolchainContract) {
		t.Fatalf("empty occurrence=%v, want typed refusal", err)
	}
}

func TestSymbolOccurrencesStopAndCancellation(t *testing.T) {
	t.Parallel()
	analysis := nativeSymbolAnalysis(t, t.TempDir())
	for _, stop := range []bool{true, false} {
		t.Run(strconv.FormatBool(stop), func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			seen, refused := 0, 0
			for got, err := range analysis.SymbolOccurrences(ctx) {
				if err != nil {
					if stop || got.Syntax.Node != nil || got.Reference.Line != 0 || !errors.Is(err, context.Canceled) || !errors.Is(err, io.ErrClosedPipe) {
						t.Fatalf("cancelled occurrence=(%+v,%v)", got, err)
					}
					refused++
					continue
				}
				seen++
				if stop {
					break
				}
				cancel(io.ErrClosedPipe)
			}
			wantRefused := 1
			if stop {
				wantRefused = 0
			}
			if seen != 1 || refused != wantRefused {
				t.Fatalf("loans/refusals=%d/%d,want1/%d", seen, refused, wantRefused)
			}
		})
	}
	analysis.Incomplete = true
	events := 0
	for got, err := range analysis.SymbolOccurrences(t.Context()) {
		events++
		if got.Syntax.Node != nil || got.Reference.Line != 0 || !errors.Is(err, core.ErrGoToolchainOutput) {
			t.Fatalf("incomplete occurrence=(%+v,%v)", got, err)
		}
	}
	if events != 1 {
		t.Fatalf("incomplete observations%d,want1 refusal", events)
	}
}

func TestSymbolOccurrencesRetainNoReferencePopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, population := range []int{64, 4097} {
		t.Run(strconv.Itoa(population), func(t *testing.T) {
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample\nimport \"os\"\nfunc F(file *os.File){"+strings.Repeat("file.Write(nil);", population)+"}")
			runtime.GC()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			count := 0
			for got, err := range analysis.SymbolOccurrences(t.Context()) {
				if err != nil || got.Reference.Symbol.Selector.String() != "Write" {
					t.Fatalf("native loan=(%+v,%v)", got, err)
				}
				count++
				runtime.GC()
				runtime.ReadMemStats(&after)
				growth := int64(after.HeapAlloc) - int64(before.HeapAlloc)
				t.Logf("native occurrences%d first-loan retained%d bytes", population, growth)
				if growth > 128<<10 {
					t.Fatalf("native occurrence stream retained%d bytes of population", growth)
				}
				break
			}
			runtime.KeepAlive(analysis)
			if count != 1 {
				t.Fatalf("native first loan%d,want1", count)
			}
		})
	}
}

func FuzzSymbolOccurrencesConserveNativeLaunchAncestry(f *testing.F) {
	for mode := uint8(0); mode < 4; mode++ {
		f.Add(mode, uint8(0))
	}
	f.Fuzz(func(t *testing.T, mode, suffix uint8) {
		source := "package sample\nimport \"sync\"\nfunc F" + strconv.Itoa(int(suffix)) + "(counter *sync.WaitGroup){"
		wantGo := false
		switch mode % 4 {
		case 0:
			source += "counter.Add(1)"
		case 1:
			source += "go counter.Add(1)"
			wantGo = true
		case 2:
			source += "go func(){counter.Add(1)}()"
			wantGo = true
		case 3:
			source += "_ = counter.Add"
		}
		source += "}"
		analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
		count := 0
		for got, err := range analysis.SymbolOccurrences(t.Context()) {
			if err != nil || got.Validate() != nil {
				t.Fatalf("occurrence=%+v,%v", got, err)
			}
			count++
			goAncestor := false
			for _, parent := range got.Syntax.Ancestors {
				if _, ok := parent.(*ast.GoStmt); ok {
					goAncestor = true
				}
			}
			if goAncestor != wantGo || got.Reference.Symbol.ImportPath.String() != "sync" || got.Reference.Symbol.Receiver == nil || got.Reference.Symbol.Receiver.String() != "WaitGroup" || got.Reference.Symbol.Selector.String() != "Add" {
				t.Fatalf("mode%d native occurrence=%+v go%t,want sync.WaitGroup.Add and go%t", mode%4, got, goAncestor, wantGo)
			}
		}
		if count != 1 {
			t.Fatalf("native occurrences%d,want1", count)
		}
	})
}
