package gotoolchain_test

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/types"
	"io"
	"reflect"
	"testing"

	"github.com/deliri/primitive/v2026/capabilities"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

const functionAncestorsSource = `package sample
var Global = func()int{_ = func(){};return 7}()
func Entry(){_ = func(){_ = func(){}}}
type Holder struct{}
func(Holder) Method(){_ = func(){_ = func(){}}}
func Generic[T any](value T){_ = func(){_ = func(){}}}
func Instantiate(){Generic(3)}
`

func TestSSAFunctionAncestorsRequestAndCatalog(t *testing.T) {
	t.Parallel()
	if err := (gotoolchain.SSAFunctionAncestorsRequest{}).Validate(); !errors.Is(err, core.ErrGoToolchainContract) {
		t.Fatalf("missing frame validation=%v", err)
	}
	owner, err := capabilities.Resolve(capabilities.ForPackage(capabilities.ScopeProduction, core.PackageGoToolchain))
	if err != nil {
		t.Fatal(err)
	}
	path, err := owner.Capability.ImportPath()
	if err != nil || reflect.TypeFor[gotoolchain.SSAFunctionAncestorsRequest]().PkgPath() != path {
		t.Fatalf("ancestor request owner=%s,%v", path, err)
	}
}

func TestSSAFunctionAncestorsFiniteSourceFrameDomain(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"Entry", "Method", "Generic", "initializer"} {
		for depth := 0; depth < 3; depth++ {
			for _, copied := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/depth%d/copied%t", kind, depth, copied), func(t *testing.T) {
					t.Parallel()
					a := nativeSymbolSourceAnalysis(t, t.TempDir(), functionAncestorsSource)
					err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
						var root *ssa.Function
						if kind == "initializer" {
							root = v.Package.Func("init")
						} else {
							for _, file := range v.Source.Syntax {
								for _, decl := range file.Decls {
									if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == kind {
										root = v.Program.FuncValue(v.Source.TypesInfo.Defs[fn.Name].(*types.Func))
									}
								}
							}
						}
						if root == nil || len(root.AnonFuncs) != 1 || len(root.AnonFuncs[0].AnonFuncs) != 1 {
							t.Fatal("fixture lost its native three-frame shape")
						}
						chain := [3]*ssa.Function{root, root.AnonFuncs[0], root.AnonFuncs[0].AnonFuncs[0]}
						selected := chain[depth]
						if copied {
							clone := *selected
							selected = &clone
							if selected == chain[depth] || selected.Syntax() != chain[depth].Syntax() {
								t.Fatal("copy mutation changed source instead of native frame identity")
							}
						}
						loans, refusals := 0, 0
						for fn, queryErr := range v.FunctionAncestors(ctx, gotoolchain.SSAFunctionAncestorsRequest{Function: selected}) {
							if copied {
								if fn != nil || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
									t.Fatalf("copied source frame loan=%v,%v, want nil and native output refusal", fn, queryErr)
								}
								refusals++
								continue
							}
							if queryErr != nil || loans > depth || fn != chain[depth-loans] {
								t.Fatalf("native ancestor loan%d=%v,%v, want exact selected-to-root identity", loans, fn, queryErr)
							}
							loans++
						}
						if copied {
							if loans != 0 || refusals != 1 {
								t.Fatalf("copy loans/refusals=%d/%d, want0/1", loans, refusals)
							}
						} else if loans != depth+1 || refusals != 0 {
							t.Fatalf("native loans/refusals=%d/%d, want%d/0", loans, refusals, depth+1)
						}
						return nil
					}})
					if err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

func TestSSAFunctionAncestorsStopAndCancellation(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"stop before broken parent", "cancel before first loan", "cancel after first loan", "caller panic"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), functionAncestorsSource)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				root := v.Package.Func("Entry")
				leaf := root.AnonFuncs[0].AnonFuncs[0]
				ctx, cancel := context.WithCancelCause(ctx)
				defer cancel(nil)
				if mode == "cancel before first loan" {
					cancel(io.ErrClosedPipe)
				}
				if mode == "stop before broken parent" {
					root.AnonFuncs[0].Signature = nil
				}
				loans, refusals := 0, 0
				if mode == "stop before broken parent" {
					v.FunctionAncestors(ctx, gotoolchain.SSAFunctionAncestorsRequest{Function: leaf})(func(fn *ssa.Function, queryErr error) bool {
						if fn != leaf || queryErr != nil || loans != 0 {
							t.Fatalf("stopped native callback=%v,%v after%d loans, want selected frame once", fn, queryErr, loans)
						}
						loans++
						return false
					})
					if loans != 1 {
						t.Fatalf("stopped native loans=%d, want1", loans)
					}
					return nil
				}
				if mode == "caller panic" {
					func() {
						defer func() {
							if got := recover(); got != io.ErrClosedPipe {
								t.Fatalf("caller panic=%v, want exact native cause", got)
							}
						}()
						for range v.FunctionAncestors(ctx, gotoolchain.SSAFunctionAncestorsRequest{Function: leaf}) {
							panic(io.ErrClosedPipe)
						}
						t.Fatal("ancestor query swallowed caller panic")
					}()
					return nil
				}
				for fn, queryErr := range v.FunctionAncestors(ctx, gotoolchain.SSAFunctionAncestorsRequest{Function: leaf}) {
					if queryErr != nil {
						if fn != nil || !errors.Is(queryErr, context.Canceled) || !errors.Is(queryErr, io.ErrClosedPipe) {
							t.Fatalf("ancestor cancellation=%v,%v", fn, queryErr)
						}
						refusals++
						continue
					}
					if fn != leaf || loans != 0 {
						t.Fatalf("first ancestor=%v after%d loans, want exact leaf once", fn, loans)
					}
					loans++
					cancel(io.ErrClosedPipe)
				}
				wantLoans, wantRefusals := 1, 1
				if mode == "cancel before first loan" {
					wantLoans = 0
				}
				if loans != wantLoans || refusals != wantRefusals {
					t.Fatalf("ancestor loans/refusals=%d/%d, want%d/%d", loans, refusals, wantLoans, wantRefusals)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
