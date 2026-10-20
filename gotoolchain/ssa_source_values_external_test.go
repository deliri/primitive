package gotoolchain_test

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/types"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSASourceValuesProducerMatrix(t *testing.T) {
	t.Parallel()
	for _, expression := range []struct{ name, text string }{
		{name: "native field", text: "p.N"},
		{name: "borrowed local", text: "count"},
		{name: "parenthesized local", text: "(count)"},
		{name: "native arithmetic", text: "count+1"},
		{name: "identity conversion", text: "int(count)"},
		{name: "native unary", text: "-count"},
		{name: "native call result", text: "identity(count)"},
		{name: "called closure result", text: "func()int{return count}()"},
	} {
		for _, site := range []struct {
			name, body string
			child      bool
		}{
			{name: "straight line", body: "probe=%s;consume(probe)"},
			{name: "constant false arm", body: "if false{probe=%s;consume(probe)}"},
			{name: "constant true arm", body: "if true{probe=%s;consume(probe)}"},
			{name: "runtime arm", body: "if flag{probe=%s;consume(probe)}"},
			{name: "called lexical child", body: "func(){probe=%s;consume(probe)}()", child: true},
			{name: "native loop body", body: "for range 2{probe=%s;consume(probe)}"},
			{name: "single pass native loop", body: "for range p.N{probe=%s;consume(probe);break}"},
		} {
			t.Run(expression.name+"/"+site.name, func(t *testing.T) {
				t.Parallel()
				analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), ssaSourceFixture(fmt.Sprintf(site.body, expression.text)))
				unit := analysis.Units[0]
				loans := 0
				if err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
					if view.Source != unit || view.Program.Fset != unit.Fset {
						t.Fatal("source identity was replaced")
					}
					fn := view.Package.Func("F")
					if fn == nil {
						t.Fatal("native function missing")
					}
					if site.child {
						if len(fn.AnonFuncs) != 1 {
							t.Fatalf("lexical children%d", len(fn.AnonFuncs))
						}
						fn = fn.AnonFuncs[0]
					}
					expr := ssaSourceProbeExpression(t, fn.Syntax())
					value, address := fn.ValueForExpr(expr)
					if value == nil || address || !types.Identical(value.Type(), unit.TypesInfo.TypeOf(expr)) {
						t.Fatalf("native expression %T value%v address%t type%v", expr, value, address, unit.TypesInfo.TypeOf(expr))
					}
					references := 0
					for _, block := range fn.Blocks {
						for _, instruction := range block.Instrs {
							reference, ok := instruction.(*ssa.DebugRef)
							if ok && reference.Expr == ast.Unparen(expr) {
								if reference.X != value || reference.IsAddr || reference.Parent() != fn {
									t.Fatalf("native source reference%v", reference)
								}
								references++
							}
						}
					}
					if references != 1 {
						t.Fatalf("native source references%d want1", references)
					}
					loans++
					return nil
				}}); err != nil {
					t.Fatal(err)
				}
				if loans != 1 {
					t.Fatalf("native source loans%d", loans)
				}
			})
		}
	}
}

func TestSSASourceValuesAreOptInAndKeepNativeAbsence(t *testing.T) {
	t.Parallel()
	analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), ssaSourceFixture("probe=count;consume(probe)"))
	unit := analysis.Units[0]
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			if err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: enabled, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				fn := view.Package.Func("F")
				expr := ssaSourceProbeExpression(t, fn.Syntax())
				value, address := fn.ValueForExpr(expr)
				if (value != nil) != enabled || address {
					t.Fatalf("native source option%t value%v/address%t", enabled, value, address)
				}
				if enabled {
					foreign := &ast.Ident{Name: "count", NamePos: expr.Pos()}
					if value, address := fn.ValueForExpr(foreign); value != nil || address {
						t.Fatalf("equal source spelling borrowed native identity%v/%t", value, address)
					}
				}
				return nil
			}}); err != nil {
				t.Fatal(err)
			}
		})
	}
	analysis = nativeSymbolSourceAnalysis(t, t.TempDir(), ssaSourceFixture("probe=3;consume(probe)"))
	unit = analysis.Units[0]
	if err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		fn := view.Package.Func("F")
		expr := ssaSourceProbeExpression(t, fn.Syntax())
		value, address := fn.ValueForExpr(expr)
		if value != nil || address || unit.TypesInfo.Types[expr].Value == nil {
			t.Fatalf("native constant absence%v/%t", value, address)
		}
		return nil
	}}); err != nil {
		t.Fatal(err)
	}
}

func TestSSASourceValuesPreserveLoanRefusalAndNativeCause(t *testing.T) {
	t.Parallel()
	analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), ssaSourceFixture("probe=count;consume(probe)"))
	unit := analysis.Units[0]
	for _, row := range []struct {
		name                   string
		before, inside, refuse bool
	}{
		{name: "consumer refusal", refuse: true},
		{name: "cancellation before build", before: true},
		{name: "cancellation inside source loan", inside: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			if row.before {
				cancel(io.ErrClosedPipe)
			}
			loans := 0
			err := analysis.WithSSAUnit(ctx, gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				fn := view.Package.Func("F")
				expr := ssaSourceProbeExpression(t, fn.Syntax())
				if value, _ := fn.ValueForExpr(expr); value == nil {
					t.Fatal("source loan has no native expression")
				}
				loans++
				if row.inside {
					cancel(io.ErrClosedPipe)
				}
				if row.refuse {
					return io.ErrClosedPipe
				}
				return nil
			}})
			if !errors.Is(err, io.ErrClosedPipe) || loans != 1 && !row.before || row.before && loans != 0 {
				t.Fatalf("source loan refusal%v loans%d", err, loans)
			}
		})
	}
	defer func() {
		if failure := recover(); failure != io.ErrClosedPipe {
			t.Fatalf("source consumer panic%v", failure)
		}
	}()
	_ = analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(context.Context, gotoolchain.SSAUnitView) error { panic(io.ErrClosedPipe) }})
	t.Fatal("native source consumer panic was swallowed")
}

func FuzzSSASourceValuesKeepNativeIdentity(f *testing.F) {
	f.Add(uint8(0))
	f.Add(uint8(255))
	f.Fuzz(func(t *testing.T, mode uint8) {
		expressions := [...]string{"p.N", "count", "(count)", "count+1", "int(count)", "-count", "identity(count)", "func()int{return count}()"}
		body := "probe=" + expressions[mode%8] + ";consume(probe)"
		if mode&8 != 0 {
			body = "for range p.N{" + body + ";break}"
		}
		enabled := mode&16 == 0
		analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), ssaSourceFixture(body))
		unit := analysis.Units[0]
		if err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: enabled, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
			fn := view.Package.Func("F")
			expr := ssaSourceProbeExpression(t, fn.Syntax())
			value, address := fn.ValueForExpr(expr)
			if address || (value != nil) != enabled {
				t.Fatalf("native source mode%d value%v/address%t", mode, value, address)
			}
			if value != nil && !types.Identical(value.Type(), unit.TypesInfo.TypeOf(expr)) {
				t.Fatalf("native source type%v want%v", value.Type(), unit.TypesInfo.TypeOf(expr))
			}
			return nil
		}}); err != nil {
			t.Fatal(err)
		}
	})
}

func ssaSourceFixture(body string) string {
	return "package sample;type counter struct{N int};func identity(n int)int{return n};func consume(int){};func F(p *counter,flag bool){count:=p.N;_=count;var probe int;_=probe;" + body + "}"
}

func ssaSourceProbeExpression(t testing.TB, source ast.Node) ast.Expr {
	t.Helper()
	var result ast.Expr
	ast.Inspect(source, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
			return true
		}
		identifier, ok := assignment.Lhs[0].(*ast.Ident)
		if ok && identifier.Name == "probe" {
			if result != nil {
				t.Fatal("native source contains duplicate probes")
			}
			result = assignment.Rhs[0]
		}
		return true
	})
	if result == nil {
		t.Fatal("native source probe missing")
	}
	return result
}

func BenchmarkSSASourceValuesNativeBuild(b *testing.B) {
	// Native package preparation is outside each measurement. Both modes use
	// this exact source; the source-reference option adds Go-owned metadata.
	analysis := nativeSymbolSourceAnalysis(b, b.TempDir(), ssaSourceFixture("probe=count;consume(probe)"))
	unit := analysis.Units[0]
	for _, enabled := range []bool{false, true} {
		b.Run(fmt.Sprint(enabled), func(b *testing.B) {
			b.ReportAllocs()
			observed := 0
			var lastErr error
			b.ResetTimer()
			for range b.N {
				lastErr = analysis.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: enabled, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
					observed += len(view.Package.Func("F").Blocks)
					return nil
				}})
			}
			b.StopTimer()
			if lastErr != nil || observed != b.N {
				b.Fatalf("native builds%d iterations%d error%v", observed, b.N, lastErr)
			}
		})
	}
}
