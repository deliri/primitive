package gotoolchain_test

import (
	"context"
	"errors"
	"go/ast"
	"io"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

const literalInvocationFixture = `package sample
//line invented.go:900
func Direct(){_=(func()int{return 1})()}
func Stored(){value:=0;fn:=func()int{value++;return value};_=fn()}
func Reassigned(){fn:=func()int{return 1};fn=func()int{return 2};_=fn()}
func Conditional(flag bool){fn:=func()int{return 1};if flag{fn=func()int{return 2}};_=fn()}
func Asynchronous(){go(func(){})()}
func Deferred(){defer(func(){})()}
func Named()int{return 1}
func NamedValue(){fn:=Named;_=fn()}
func Nested(){fn:=func()int{nested:=func()int{return 2};return nested()};_=fn()}
var Initial=(func()int{return 1})()
`

func TestLiteralInvocationBorrowsOnlyNativeSynchronousTarget(t *testing.T) {
	t.Parallel()
	analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), literalInvocationFixture)
	for _, tc := range []struct {
		name          string
		call, literal int
		absent        bool
	}{
		{name: "Direct"},
		{name: "Stored"},
		{name: "Reassigned", literal: 1},
		{name: "Conditional", absent: true},
		{name: "Asynchronous", absent: true},
		{name: "Deferred", absent: true},
		{name: "NamedValue", absent: true},
		{name: "Nested", literal: 1},
		{name: "Nested", call: 1},
		{name: "Initial"},
	} {
		t.Run(tc.name+strconv.Itoa(tc.call), func(t *testing.T) {
			t.Parallel()
			var owner ast.Node
			for _, unit := range analysis.Units {
				for _, file := range unit.Syntax {
					for _, declaration := range file.Decls {
						if fn, ok := declaration.(*ast.FuncDecl); ok && fn.Name.Name == tc.name {
							owner = fn
						}
						if group, ok := declaration.(*ast.GenDecl); ok {
							for _, spec := range group.Specs {
								if value, ok := spec.(*ast.ValueSpec); ok && value.Names[0].Name == tc.name {
									owner = value
								}
							}
						}
					}
				}
			}
			if owner == nil {
				t.Fatal("native caller absent")
			}
			var calls []*ast.CallExpr
			var literals []*ast.FuncLit
			ast.Inspect(owner, func(node ast.Node) bool {
				if call, ok := node.(*ast.CallExpr); ok {
					calls = append(calls, call)
				}
				if literal, ok := node.(*ast.FuncLit); ok {
					literals = append(literals, literal)
				}
				return true
			})
			if tc.call >= len(calls) {
				t.Fatalf("native call index%d outside%d", tc.call, len(calls))
			}
			got, err := analysis.LiteralInvocation(t.Context(), gotoolchain.LiteralInvocationRequest{Call: calls[tc.call]})
			if err != nil || got.Validate() != nil {
				t.Fatalf("native literal target=%+v/%v", got, err)
			}
			if tc.absent {
				if got != (gotoolchain.LiteralInvocationObservation{Presence: gotoolchain.FunctionSourceAbsent}) {
					t.Fatalf("unknown or nonsynchronous target gained literal: %+v", got)
				}
				return
			}
			if tc.literal >= len(literals) || got.Presence != gotoolchain.FunctionSourcePresent || got.Literal != literals[tc.literal] || got.Types != analysis.Units[0].TypesInfo || got.Positions != analysis.Units[0].Fset {
				t.Fatalf("native target=%+v want actual literal%d", got, tc.literal)
			}
			physical := got.Positions.PositionFor(got.Literal.Pos(), false)
			if physical.Filename != analysis.Units[0].CompiledGoFiles[0] || physical.Line >= 900 {
				t.Fatalf("literal physical source=%+v", physical)
			}
		})
	}
}

func TestLiteralInvocationRefusesForeignAndMissingCompilerFacts(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"foreign call", "missing callee fact", "missing call fact", "incomplete analysis", "absent call", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample\nfunc F(){fn:=func()int{return 1};_=fn()}")
			var call *ast.CallExpr
			ast.Inspect(analysis.Units[0].Syntax[0], func(node ast.Node) bool {
				if found, ok := node.(*ast.CallExpr); ok {
					call = found
				}
				return true
			})
			if call == nil {
				t.Fatal("native call absent")
			}
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			want := error(core.ErrGoToolchainOutput)
			switch mode {
			case "foreign call":
				copied := *call
				call = &copied
				want = core.ErrGoToolchainContract
			case "missing callee fact":
				if name, ok := call.Fun.(*ast.Ident); ok {
					delete(analysis.Units[0].TypesInfo.Uses, name)
				}
				delete(analysis.Units[0].TypesInfo.Types, call.Fun)
			case "missing call fact":
				delete(analysis.Units[0].TypesInfo.Types, call)
			case "incomplete analysis":
				analysis.Incomplete = true
			case "absent call":
				call = nil
				want = core.ErrGoToolchainContract
			case "cancelled":
				cancel(io.ErrClosedPipe)
				want = io.ErrClosedPipe
			default:
				t.Fatal("unhandled authored case")
			}
			got, err := analysis.LiteralInvocation(ctx, gotoolchain.LiteralInvocationRequest{Call: call})
			if got != (gotoolchain.LiteralInvocationObservation{}) || !errors.Is(err, want) {
				t.Fatalf("invalid literal query=%+v/%v want zero/%v", got, err, want)
			}
		})
	}
}

func TestLiteralInvocationObservationRefusesContradictoryPresence(t *testing.T) {
	t.Parallel()
	for _, got := range []gotoolchain.LiteralInvocationObservation{{}, {Presence: 255}, {Presence: gotoolchain.FunctionSourcePresent}, {Presence: gotoolchain.FunctionSourceAbsent, Literal: &ast.FuncLit{}}} {
		if !errors.Is(got.Validate(), core.ErrGoToolchainContract) {
			t.Fatalf("invalid literal observation=%+v admitted", got)
		}
	}
	if err := (gotoolchain.LiteralInvocationRequest{}).Validate(); !errors.Is(err, core.ErrGoToolchainContract) {
		t.Fatalf("empty native request=%v", err)
	}
}

func FuzzLiteralInvocationPreservesNativeTargetsUnderRenaming(f *testing.F) {
	for _, mode := range []uint8{0, 1, 2, 3, 4, 5} {
		f.Add(uint64(0), mode)
	}
	f.Fuzz(func(t *testing.T, spelling uint64, mode uint8) {
		name := "fn" + strconv.FormatUint(spelling, 36)
		source := "package sample\nfunc Named()int{return 3};func F(flag bool){"
		body := ""
		switch mode % 6 {
		case 0:
			body := name + ":=func()int{return 1};_=" + name + "()"
			source += body
		case 1:
			body = name + ":=func()int{return 1};" + name + "=func()int{return 2};_=" + name + "()"
			source += body
		case 2:
			body = name + ":=func()int{return 1};if flag{" + name + "=func()int{return 2}};_=" + name + "()"
			source += body
		case 3:
			source += "defer(func(){})()"
		case 4:
			source += "go(func(){})()"
		case 5:
			source += name + ":=Named;_=" + name + "()"
		}
		source += "}"
		analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
		var call *ast.CallExpr
		var literal *ast.FuncLit
		ast.Inspect(analysis.Units[0].Syntax[0], func(node ast.Node) bool {
			if found, ok := node.(*ast.CallExpr); ok {
				call = found
			}
			if found, ok := node.(*ast.FuncLit); ok {
				literal = found
			}
			return true
		})
		if call == nil {
			t.Fatal("generated native call absent")
		}
		got, err := analysis.LiteralInvocation(t.Context(), gotoolchain.LiteralInvocationRequest{Call: call})
		if err != nil || got.Validate() != nil {
			t.Fatalf("native renamed target=%+v/%v", got, err)
		}
		want := gotoolchain.FunctionSourceAbsent
		if mode%6 < 2 {
			want = gotoolchain.FunctionSourcePresent
		}
		if got.Presence != want || want == gotoolchain.FunctionSourcePresent && got.Literal != literal {
			t.Fatalf("native target at mode%d=%+v want%v/exact last literal", mode, got, want)
		}
	})
}
