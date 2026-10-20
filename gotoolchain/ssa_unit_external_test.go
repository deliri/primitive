package gotoolchain_test

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/constant"
	"go/types"
	"io"
	"runtime"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

func TestSSAUnitLoansNativeCallKindsTargetsAndOperands(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body                                                string
		calls                                                     int
		deferred, concurrent, closure, methodValue, constantFalse bool
	}{
		{name: "direct receiver", body: "m.Run()", calls: 1},
		{name: "copied receiver is same native operand", body: "copy:=m;copy.Run()", calls: 1},
		{name: "native closure retains capture", body: "callback:=func(){m.Run()};callback()", calls: 1, closure: true},
		{name: "native closure copy retains capture", body: "callback:=func(){m.Run()};next:=callback;next()", calls: 1, closure: true},
		{name: "native method wrapper retains receiver", body: "run:=m.Run;run()", calls: 1, closure: true, methodValue: true},
		{name: "native defer retains distinct instruction", body: "callback:=func(){m.Run()};defer callback()", calls: 1, deferred: true, closure: true},
		{name: "native goroutine retains distinct instruction", body: "callback:=func(){m.Run()};go callback()", calls: 1, concurrent: true, closure: true},
		{name: "native CFG retains a constant false branch", body: "if false {m.Run()}", calls: 1, constantFalse: true},
		{name: "uncalled method value is no invocation", body: "run:=m.Run;_=run"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			source := "package sample\ntype runner struct{};func(*runner)Run()int{return 0}\n//line forged.go:900\nfunc F(m *runner){" + row.body + "}"
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
			unit := analysis.Units[0]
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				if err := view.Validate(); err != nil {
					return err
				}
				object := unit.Types.Scope().Lookup("F").(*types.Func)
				fn := view.Program.FuncValue(object)
				if view.Source != unit || fn == nil || fn != view.Package.Func("F") || fn.Object() != object || len(fn.Params) != 1 {
					t.Fatalf("foreign or absent native function%v", fn)
				}
				count := 0
				for _, block := range fn.Blocks {
					for _, instruction := range block.Instrs {
						call, ok := instruction.(ssa.CallInstruction)
						if !ok {
							continue
						}
						count++
						if count > row.calls {
							t.Fatalf("unexpected native call%v", call)
						}
						_, deferred := instruction.(*ssa.Defer)
						_, concurrent := instruction.(*ssa.Go)
						if deferred != row.deferred || concurrent != row.concurrent {
							t.Fatalf("native call kind%T", instruction)
						}
						position := view.Source.Fset.PositionFor(instruction.Pos(), false)
						if position.Line != 4 || !strings.HasSuffix(position.Filename, "/source.go") {
							t.Fatalf("native physical call%+v", position)
						}
						if row.constantFalse {
							requireSSAUnitConstantFalseEdge(t, fn, instruction.Block())
						}
						common := call.Common()
						target := common.StaticCallee()
						if target == nil {
							t.Fatal("native target absent")
						}
						if row.closure {
							binding, ok := common.Value.(*ssa.MakeClosure)
							if !ok || len(binding.Bindings) != 1 || len(target.FreeVars) != 1 {
								t.Fatalf("native capture=%v target%v", common.Value, target)
							}
							requireSSAUnitNativeCaptureOrigin(t, binding.Bindings[0], fn.Params[0])
							if row.methodValue {
								observed := 0
								for _, innerBlock := range target.Blocks {
									for _, inner := range innerBlock.Instrs {
										invoked, ok := inner.(*ssa.Call)
										if !ok {
											continue
										}
										called := invoked.Common().StaticCallee()
										if called == nil || called.Object() == nil || called.Object().Pkg() != unit.Types || called.Object().Name() != "Run" || len(invoked.Common().Args) != 1 || invoked.Common().Args[0] != target.FreeVars[0] {
											t.Fatalf("native method wrapper instruction%v", inner)
										}
										observed++
									}
								}
								if observed != 1 {
									t.Fatalf("native method target calls%d", observed)
								}
							} else if _, ok := target.Syntax().(*ast.FuncLit); !ok {
								t.Fatalf("native literal syntax%T", target.Syntax())
							}
						} else if target.Object() == nil || target.Object().Name() != "Run" || target.Object().Pkg() != unit.Types || len(common.Args) != 1 || common.Args[0] != fn.Params[0] {
							t.Fatalf("native receiver call%v", common)
						}
					}
				}
				if count != row.calls {
					t.Fatalf("native call population%d want%d", count, row.calls)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSSAUnitRefusesMissingForeignAndIncompleteOwners(t *testing.T) {
	t.Parallel()
	analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample;func F(){}")
	unit := analysis.Units[0]
	clone := *unit
	foreign := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample;func F(){}")
	for _, row := range []struct {
		name              string
		source            gotoolchain.PackageAnalysis
		unit              *packages.Package
		noUse, incomplete bool
		want              error
	}{
		{name: "missing unit", source: analysis, want: core.ErrGoToolchainContract},
		{name: "copied native metadata is not selected unit", source: analysis, unit: &clone, want: core.ErrGoToolchainContract},
		{name: "foreign equal source is not selected unit", source: analysis, unit: foreign.Units[0], want: core.ErrGoToolchainContract},
		{name: "absent analysis", unit: unit, want: core.ErrGoToolchainContract},
		{name: "absent callback", source: analysis, unit: unit, noUse: true, want: core.ErrGoToolchainContract},
		{name: "incomplete analysis cannot supply native IR", source: analysis, unit: unit, incomplete: true, want: core.ErrGoToolchainOutput},
	} {
		t.Run(row.name, func(t *testing.T) {
			called := false
			use := func(context.Context, gotoolchain.SSAUnitView) error { called = true; return nil }
			if row.noUse {
				use = nil
			}
			source := row.source
			if row.incomplete {
				source.Incomplete = true
			}
			err := source.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: row.unit, Use: use})
			if !errors.Is(err, row.want) || called {
				t.Fatalf("native scope refusal%v use%v want%v", err, called, row.want)
			}
		})
	}
}

func TestSSAUnitPreservesConsumerAndCancellationCauses(t *testing.T) {
	t.Parallel()
	analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample;func F(){}")
	for _, row := range []struct {
		name           string
		before, inside bool
		callback       error
		want           error
	}{
		{name: "consumer refusal", callback: io.ErrClosedPipe, want: io.ErrClosedPipe},
		{name: "cancel before build", before: true, want: io.ErrClosedPipe},
		{name: "cancel during loan", inside: true, want: io.ErrClosedPipe},
	} {
		t.Run(row.name, func(t *testing.T) {
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			if row.before {
				cancel(io.ErrClosedPipe)
			}
			called := false
			err := analysis.WithSSAUnit(ctx, gotoolchain.SSAUnitRequest{Unit: analysis.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				called = true
				if view.Package.Func("F") == nil {
					t.Fatal("missing native function")
				}
				if row.inside {
					cancel(io.ErrClosedPipe)
				}
				return row.callback
			}})
			if !errors.Is(err, row.want) || called == row.before {
				t.Fatalf("scope cause%v use%v before%v", err, called, row.before)
			}
		})
	}
}

func TestSSAUnitConsumerPanicIsNotCompilerRefusal(t *testing.T) {
	t.Parallel()
	analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample;func F(){}")
	defer func() {
		if failure := recover(); failure != io.ErrClosedPipe {
			t.Fatalf("consumer panic=%v", failure)
		}
	}()
	_ = analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: analysis.Units[0], Use: func(context.Context, gotoolchain.SSAUnitView) error { panic(io.ErrClosedPipe) }})
	t.Fatal("consumer panic was swallowed")
}

func TestSSAUnitMalformedNativeFactsDoNotBecomeLoan(t *testing.T) {
	t.Parallel()
	analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample;func F(){G()};func G(){}")
	unit := analysis.Units[0]
	object := unit.Types.Scope().Lookup("F")
	removed := 0
	for identifier, defined := range unit.TypesInfo.Defs {
		if defined == object {
			delete(unit.TypesInfo.Defs, identifier)
			removed++
		}
	}
	if removed != 1 {
		t.Fatalf("removed native declarations%d", removed)
	}
	used := false
	err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, Use: func(context.Context, gotoolchain.SSAUnitView) error { used = true; return nil }})
	if !errors.Is(err, core.ErrGoToolchainOutput) || used {
		t.Fatalf("malformed native facts%v loan%v", err, used)
	}
	var native runtime.Error
	if !errors.As(err, &native) {
		t.Fatalf("native compiler assertion cause was erased: %v", err)
	}
}

func TestSSAUnitViewRequiresOneBuiltNativeGraph(t *testing.T) {
	t.Parallel()
	analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample;func F(){}")
	if err := (gotoolchain.SSAUnitView{}).Validate(); !errors.Is(err, core.ErrGoToolchainContract) {
		t.Fatalf("empty native view%v", err)
	}
	err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: analysis.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		foreign := ssa.NewProgram(view.Source.Fset, ssa.BuilderMode(0))
		for _, row := range []gotoolchain.SSAUnitView{
			{Source: view.Source, Program: foreign, Package: view.Package},
			{Source: view.Source, Program: view.Program},
			{Source: view.Source, Package: view.Package},
			{Program: view.Program, Package: view.Package},
		} {
			if err := row.Validate(); !errors.Is(err, core.ErrGoToolchainContract) {
				t.Fatalf("mixed native view%v", err)
			}
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func FuzzSSAUnitNativeOperandClosure(f *testing.F) {
	for mode := uint8(0); mode < 6; mode++ {
		f.Add(mode, uint8(0))
	}
	f.Fuzz(func(t *testing.T, mode, suffix uint8) {
		name := fmt.Sprintf("receiver%d", suffix)
		body := name + ".Run()"
		want := 1
		switch mode % 6 {
		case 0:
			body = "copy:=" + name + ";copy.Run()"
		case 1:
			body = "callback:=func(){" + name + ".Run()};callback()"
		case 2:
			body = "callback:=func(){" + name + ".Run()};defer callback()"
		case 3:
			body = "callback:=func(){" + name + ".Run()};go callback()"
		case 4:
			body = "if false {" + name + ".Run()}"
		case 5:
			body = "run:=" + name + ".Run;_=run"
			want = 0
		}
		analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample;type runner struct{};func(*runner)Run(){};func F("+name+" *runner){"+body+"}")
		err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: analysis.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
			fn := view.Package.Func("F")
			if fn == nil || len(fn.Params) != 1 {
				t.Fatal("native parameter missing")
			}
			count := 0
			for _, block := range fn.Blocks {
				for _, instruction := range block.Instrs {
					call, ok := instruction.(ssa.CallInstruction)
					if !ok {
						continue
					}
					count++
					switch mode % 6 {
					case 0:
						if len(call.Common().Args) != 1 || call.Common().Args[0] != fn.Params[0] {
							t.Fatalf("native copied operand%v", call)
						}
					case 4:
						requireSSAUnitConstantFalseEdge(t, fn, instruction.Block())
					case 1, 2, 3:
						closure, ok := call.Common().Value.(*ssa.MakeClosure)
						if !ok || len(closure.Bindings) != 1 {
							t.Fatalf("native closure%v", call)
						}
						requireSSAUnitNativeCaptureOrigin(t, closure.Bindings[0], fn.Params[0])
						_, deferred := instruction.(*ssa.Defer)
						_, concurrent := instruction.(*ssa.Go)
						if deferred != (mode%6 == 2) || concurrent != (mode%6 == 3) {
							t.Fatalf("native execution form%T mode%d", instruction, mode)
						}
					}
				}
			}
			if count != want {
				t.Fatalf("native calls%d want%d mode%d", count, want, mode)
			}
			return nil
		}})
		if err != nil {
			t.Fatal(err)
		}
	})
}

// Go captures a mutable lexical variable through its native allocation cell;
// a bound method value may instead capture the parameter itself. Both are
// actual IR forms, proved here by pointer identity and the compiler's Store.
func requireSSAUnitNativeCaptureOrigin(t *testing.T, binding ssa.Value, parameter *ssa.Parameter) {
	t.Helper()
	if binding == parameter {
		return
	}
	cell, ok := binding.(*ssa.Alloc)
	if !ok || cell.Referrers() == nil {
		t.Fatalf("native parameter capture%T %v", binding, binding)
	}
	stores := 0
	for _, reference := range *cell.Referrers() {
		if store, ok := reference.(*ssa.Store); ok && store.Addr == cell {
			if store.Val != parameter {
				t.Fatalf("native captured cell initialized from%v want%v", store.Val, parameter)
			}
			stores++
		}
	}
	if stores != 1 {
		t.Fatalf("native captured parameter stores%d want1", stores)
	}
}
func requireSSAUnitConstantFalseEdge(t *testing.T, function *ssa.Function, called *ssa.BasicBlock) {
	t.Helper()
	for _, block := range function.Blocks {
		if len(block.Instrs) == 0 {
			continue
		}
		branch, ok := block.Instrs[len(block.Instrs)-1].(*ssa.If)
		if !ok {
			continue
		}
		condition, ok := branch.Cond.(*ssa.Const)
		if !ok || condition.Value == nil || condition.Value.Kind() != constant.Bool {
			continue
		}
		if !constant.BoolVal(condition.Value) && len(block.Succs) == 2 && block.Succs[0] == called {
			return
		}
	}
	t.Fatalf("native call block%d lacks its constant-false incoming branch", called.Index)
}
