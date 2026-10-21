package gotoolchain_test

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/types"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestErrorPropagationNativeCallAndIdentityLayerTriad(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, declarations, body string
		want                     bool
	}{
		{name: "direct call", body: "return validate()", want: true},
		{name: "parenthesized call", body: "return ((validate()))", want: true},
		{name: "copied error", body: "err:=validate();copy:=err;return copy", want: true},
		{name: "original replaced after copy", body: "err:=validate();copy:=err;err=nil;return copy", want: true},
		{name: "returned copy replaced", body: "err:=validate();copy:=err;copy=nil;return copy"},
		{name: "error guard", body: "if err:=validate();err!=nil{return err};return nil", want: true},
		{name: "inverted nil guard", body: "err:=validate();if err==nil{return nil};return err", want: true},
		{name: "guarded copied value", body: "err:=validate();copy:=err;if copy!=nil{return copy};return nil", want: true},
		{name: "discarded expression", body: "validate();return nil"},
		{name: "blank assignment", body: "_=validate();return nil"},
		{name: "deferred call", body: "defer validate();return nil"},
		{name: "concurrent call", body: "go validate();return nil"},
		{name: "second call cannot recover original", body: "if validate()!=nil{return validate()};return nil"},
		{name: "shadow cannot recover original", body: "if err:=validate();err!=nil{var err error;return err};return nil"},
		{name: "unrelated early return", body: "if flag{return nil};err:=validate();return err", want: true},
		{name: "conditional region nil exit", body: "if flag{err:=validate();if err!=nil{return err}};return nil", want: true},
		{name: "conditional region non nil leak", body: "if flag{err:=validate();if err!=nil{if other{return err}}};return nil"},
		{name: "goto carries original", body: "err:=validate();copy:=err;goto done;done:return copy", want: true},
		{name: "both live arms retain original", body: "err:=validate();if flag{return err};return err", want: true},
		{name: "later live return loses original", body: "err:=validate();if flag{return err};return nil"},
		{name: "constant false drop is not executed", body: "err:=validate();if false{return nil};return err", want: true},
		{name: "uncalled closure retains no return authority", body: "err:=validate();_=func()error{return err};return nil"},
		{name: "native Join", body: "return errors.Join(validate())", want: true},
		{name: "assigned Join guarded by nil", body: "err:=errors.Join(validate());if err!=nil{return err};return nil", want: true},
		{name: "native format wrapping", body: "return fmt.Errorf(\"%w\",validate())", want: true},
		{name: "plain formatting loses original", body: "return fmt.Errorf(\"%v\",validate())"},
		{name: "local helper retains original", declarations: "func carry(err error)error{return err}", body: "return carry(validate())", want: true},
		{name: "local helper replaces original", declarations: "func carry(err error)error{return nil}", body: "return carry(validate())"},
		{name: "helper copy survives original replacement", declarations: "func carry(err error)error{copy:=err;err=nil;return copy}", body: "return carry(validate())", want: true},
		{name: "helper copied value replaced", declarations: "func carry(err error)error{copy:=err;copy=nil;return copy}", body: "return carry(validate())"},
		{name: "helper nil input is neutral", declarations: "func carry(err error)error{if err==nil{return nil};return err}", body: "return carry(validate())", want: true},
		{name: "unrelated helper nil guard", declarations: "func carry(err error)error{var other error;if flag{other=err};if other==nil{return nil};return err}", body: "return carry(validate())"},
		{name: "recursive helper cannot invent return", declarations: "func carry(err error)error{return carry(err)}", body: "return carry(validate())"},
		{name: "generic helper preserves error", declarations: "func carry[E error](err E)error{return err}", body: "return carry(validate())", want: true},
		{name: "generic helper preserves native result", declarations: "func carry[E error](err E)E{return err}", body: "return carry(validate())", want: true},
		{name: "generic helper cannot replace error", declarations: "func carry[E error](err E)error{return nil}", body: "return carry(validate())"},
		{name: "method expression shifted slot", declarations: "type carrier struct{};func(carrier)carry(err error)error{return err}", body: "return carrier.carry(carrier{},validate())", want: true},
		{name: "method selected receiver", declarations: "type carrier struct{};func(carrier)carry(err error)error{return err}", body: "return(carrier{}).carry(validate())", want: true},
		{name: "named error result", declarations: "func carry(err error)(result error){result=err;return}", body: "return carry(validate())", want: true},
		{name: "helper other parameter cannot grant authority", declarations: "func carry(err,other error)error{return other}", body: "return carry(validate(),nil)"},
		{name: "helper copied native wrapping", declarations: "func carry(err error)error{copy:=err;return fmt.Errorf(\"%w\",copy)}", body: "return carry(validate())", want: true},
		{name: "helper copied plain formatting", declarations: "func carry(err error)error{copy:=err;return fmt.Errorf(\"%v\",copy)}", body: "return carry(validate())"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			source := "package sample;import(\"errors\";\"fmt\");var flag,other bool;var _=errors.Join;var _=fmt.Errorf;func validate()error{return nil};" + row.declarations + "\nfunc F()error{" + row.body + "}"
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
			observed := 0
			for _, unit := range analysis.Units {
				err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
					for _, file := range unit.Syntax {
						for _, declaration := range file.Decls {
							fn, ok := declaration.(*ast.FuncDecl)
							if !ok || fn.Name.Name != "F" {
								continue
							}
							for syntax, err := range gotoolchain.WalkSyntax(ctx, gotoolchain.SyntaxWalkRequest{Root: fn.Body}) {
								if err != nil {
									return err
								}
								call, ok := syntax.Node.(*ast.CallExpr)
								if !ok {
									continue
								}
								name, ok := call.Fun.(*ast.Ident)
								if !ok || name.Name != "validate" {
									continue
								}
								got, err := view.ErrorPropagation(ctx, gotoolchain.ErrorPropagationRequest{Analysis: analysis, Call: call})
								if err != nil || got.Preserved != row.want || got.Validate() != nil {
									t.Fatalf("native error propagation got%+v/%v want preserved%t/nil", got, err, row.want)
								}
								observed++
								return nil
							}
						}
					}
					return nil
				}})
				if err != nil {
					t.Fatal(err)
				}
			}
			if observed != 1 {
				t.Fatalf("native call observations got%d want1", observed)
			}
		})
	}
}

func TestErrorPropagationNativeTupleSlot(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, signature, returns, body string
		want                           bool
	}{
		{name: "first error slot", signature: "(error,int)", returns: "nil,7", body: "err,n:=validate();_=n;return err", want: true},
		{name: "last error slot", signature: "(int,error)", returns: "7,nil", body: "n,err:=validate();_=n;return err", want: true},
		{name: "discarded error slot", signature: "(error,int)", returns: "nil,7", body: "_,n:=validate();_=n;return nil"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample;func validate()"+row.signature+"{return "+row.returns+"};func F()error{"+row.body+"}")
			unit := analysis.Units[0]
			observed := 0
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				for _, file := range unit.Syntax {
					for _, declaration := range file.Decls {
						fn, ok := declaration.(*ast.FuncDecl)
						if !ok || fn.Name.Name != "F" {
							continue
						}
						for syntax, err := range gotoolchain.WalkSyntax(ctx, gotoolchain.SyntaxWalkRequest{Root: fn.Body}) {
							if err != nil {
								return err
							}
							call, ok := syntax.Node.(*ast.CallExpr)
							if !ok {
								continue
							}
							got, err := view.ErrorPropagation(ctx, gotoolchain.ErrorPropagationRequest{Analysis: analysis, Call: call})
							if err != nil || got.Preserved != row.want {
								t.Fatalf("tuple propagation got%+v/%v want%t/nil", got, err, row.want)
							}
							observed++
							return nil
						}
					}
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
			if observed != 1 {
				t.Fatalf("tuple observations got%d want1", observed)
			}
		})
	}
}

func TestErrorPropagationRefusalsKeepNativeCauseAndZeroObservation(t *testing.T) {
	t.Parallel()
	analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample;func validate()error{return nil};func F()error{return validate()}")
	unit := analysis.Units[0]
	err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		fn := unit.Syntax[0].Decls[1].(*ast.FuncDecl)
		call := fn.Body.List[0].(*ast.ReturnStmt).Results[0].(*ast.CallExpr)
		for _, row := range []struct {
			name    string
			view    gotoolchain.SSAUnitView
			request gotoolchain.ErrorPropagationRequest
			cancel  bool
			want    error
		}{
			{name: "empty view", request: gotoolchain.ErrorPropagationRequest{Analysis: analysis, Call: call}, want: core.ErrGoToolchainContract},
			{name: "missing call", view: view, request: gotoolchain.ErrorPropagationRequest{Analysis: analysis}, want: core.ErrGoToolchainContract},
			{name: "missing source owner", view: view, request: gotoolchain.ErrorPropagationRequest{Call: call}, want: core.ErrGoToolchainContract},
			{name: "native cancellation cause", view: view, request: gotoolchain.ErrorPropagationRequest{Analysis: analysis, Call: call}, cancel: true, want: io.ErrClosedPipe},
		} {
			t.Run(row.name, func(t *testing.T) {
				ctx, cancel := context.WithCancelCause(ctx)
				defer cancel(nil)
				if row.cancel {
					cancel(io.ErrClosedPipe)
				}
				got, err := row.view.ErrorPropagation(ctx, row.request)
				if got != (gotoolchain.ErrorIdentityObservation{}) || !errors.Is(err, row.want) {
					t.Fatalf("propagation refusal got%+v/%v want zero/%v", got, err, row.want)
				}
			})
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestErrorPropagationQueryKeepsFixedMemory(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, extent := range []int{64, 4097} {
		t.Run(strconv.Itoa(extent), func(t *testing.T) {
			source := "package sample;func validate()error{return nil};func carry(err error,flag bool)error{" + strings.Repeat("if flag{return err};", extent-1) + "return err};func F(flag bool)error{return carry(validate(),flag)}"
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
			unit := analysis.Units[0]
			observed := 0
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				fn := unit.Syntax[0].Decls[2].(*ast.FuncDecl)
				outer := fn.Body.List[0].(*ast.ReturnStmt).Results[0].(*ast.CallExpr)
				call := outer.Args[0].(*ast.CallExpr)
				before, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				got, err := view.ErrorPropagation(ctx, gotoolchain.ErrorPropagationRequest{Analysis: analysis, Call: call})
				if err != nil || !got.Preserved {
					t.Fatalf("native query extent%d got%+v/%v want preserved/nil", extent, got, err)
				}
				after, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				allocated := after.Uint64() - before.Uint64()
				t.Logf("native error query extent%d allocated%d; Go owns the compiled unit", extent, allocated)
				if allocated > 32<<10 {
					t.Fatalf("native error query allocated%d want at most32768", allocated)
				}
				observed++
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
			if observed != 1 {
				t.Fatalf("memory observations got%d want1", observed)
			}
		})
	}
}

func FuzzErrorIdentityNativeFormatMatchesWrappedCause(f *testing.F) {
	f.Add("%w")
	f.Add("%v")
	f.Fuzz(func(t *testing.T, format string) {
		native := fmt.Errorf(format, core.ErrGoToolchainContract)
		analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample;import \"fmt\";func F(err error)error{return fmt.Errorf("+strconv.Quote(format)+",err)}")
		unit := analysis.Units[0]
		observed := 0
		err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
			object := unit.Types.Scope().Lookup("F").(*types.Func)
			fn := unit.Syntax[0].Decls[1].(*ast.FuncDecl)
			call := fn.Body.List[0].(*ast.ReturnStmt).Results[0].(*ast.CallExpr)
			got, err := view.ErrorIdentity(ctx, gotoolchain.ErrorIdentityRequest{Analysis: analysis, Function: object, Parameter: object.Type().(*types.Signature).Params().At(0), Expression: call})
			if err != nil || got.Preserved != errors.Is(native, core.ErrGoToolchainContract) {
				t.Fatalf("native format %q identity got%+v/%v want preserved%t/nil", format, got, err, errors.Is(native, core.ErrGoToolchainContract))
			}
			observed++
			return nil
		}})
		if err != nil {
			t.Fatal(err)
		}
		if observed != 1 {
			t.Fatalf("format observations got%d want1", observed)
		}
	})
}
