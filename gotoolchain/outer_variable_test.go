package gotoolchain

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/testserial"
)

func outerVariableFixture(t testing.TB, source string) (*token.FileSet, *types.Info) {
	t.Helper()
	positions := token.NewFileSet()
	file, err := parser.ParseFile(positions, "scope.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	information := &types.Info{Defs: make(map[*ast.Ident]types.Object), Uses: make(map[*ast.Ident]types.Object), Scopes: make(map[ast.Node]*types.Scope)}
	configuration := types.Config{}
	if _, err := configuration.Check("example.com/scope", positions, []*ast.File{file}, information); err != nil {
		t.Fatal(err)
	}
	return positions, information
}

func TestOuterVariableNativeScopeLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source  string
		target, outer int
	}{
		{name: "function local has no outer variable", source: "package p\nfunc F(){\nerr:=error(nil);_=err\n}\n", target: 3},
		{name: "inner block borrows outer local", source: "package p\nfunc F(){\nvar err error;_=err\n{\nerr:=error(nil);_=err\n}\n}\n", target: 5, outer: 3},
		{name: "function parameter is an outer declaration", source: "package p\nfunc F(err error){\n{\nerr:=error(nil);_=err\n}\n_=err\n}\n", target: 4, outer: 2},
		{name: "sibling block declaration is invisible", source: "package p\nfunc F(){\n{var err error;_=err}\n{\nerr:=error(nil);_=err\n}\n}\n", target: 5},
		{name: "nearest enclosing variable wins", source: "package p\nfunc F(){\nvar err error;_=err\n{\nvar err error;_=err\n{\nerr:=error(nil);_=err\n}\n}\n}\n", target: 7, outer: 5},
		{name: "package declaration is native outer scope", source: "package p\nvar err error\nfunc F(){\nerr:=error(nil);_=err\n}\n", target: 4, outer: 2},
		{name: "forward package declaration stays visible", source: "package p\nfunc F(){\nerr:=error(nil);_=err\n}\nvar err error\n", target: 3, outer: 5},
		{name: "nearer constant hides more distant variable", source: "package p\nvar err error\nfunc F(){\nconst err=1;_=err\n{\nerr:=error(nil);_=err\n}\n}\n", target: 6},
		{name: "if initializer retains native parent", source: "package p\nfunc F(){\nvar err error;_=err\nif err:=error(nil);err!=nil{_=err}\n}\n", target: 4, outer: 3},
		{name: "if body borrows initializer variable", source: "package p\nfunc F(){\nif err:=error(nil);err!=nil{\nerr:=error(nil);_=err\n}\n}\n", target: 4, outer: 3},
		{name: "for initializer retains native parent", source: "package p\nfunc F(){\nvar err error;_=err\nfor err:=error(nil);err!=nil;{_=err;break}\n}\n", target: 4, outer: 3},
		{name: "range binding retains native parent", source: "package p\nfunc F(){\nvar err error;_=err\nfor _,err:=range []error{nil}{_=err}\n}\n", target: 4, outer: 3},
		{name: "case body borrows switch initializer", source: "package p\nfunc F(){\nswitch err:=error(nil);err{\ncase nil:\nerr:=error(nil);_=err\n}\n}\n", target: 5, outer: 3},
		{name: "select receiver retains native outer variable", source: "package p\nfunc F(ch chan error){\nvar err error;_=err\nselect{case err:=<-ch:_=err}\n}\n", target: 4, outer: 3},
		{name: "type parameter hides more distant variable", source: "package p\nvar err error\nfunc F[err any](){\n{\nerr:=error(nil);_=err\n}\n}\n", target: 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			positions, information := outerVariableFixture(t, tc.source)
			var target, want *types.Var
			for identifier, object := range information.Defs {
				variable, ok := object.(*types.Var)
				if !ok || identifier.Name != "err" {
					continue
				}
				line := positions.PositionFor(identifier.Pos(), false).Line
				if line == tc.target {
					target = variable
				}
				if line == tc.outer {
					want = variable
				}
			}
			if target == nil || tc.outer != 0 && want == nil {
				t.Fatalf("native fixture target=%v expected outer=%v", target, want)
			}
			got, err := ObserveOuterVariable(t.Context(), OuterVariableRequest{Variable: target, Position: target.Pos()})
			if err != nil || got.Variable != want {
				t.Fatalf("native outer variable=%v refusal=%v,want%v", got.Variable, err, want)
			}
		})
	}
}

func TestOuterVariableNativeVisibilityBoundary(t *testing.T) {
	t.Parallel()
	positions, information := outerVariableFixture(t, "package p\nfunc F(){\nvar err error\n_=err\n{\nerr:=error(nil);_=err\n}\n}\n")
	var target, want *types.Var
	for identifier, object := range information.Defs {
		variable, ok := object.(*types.Var)
		if !ok || identifier.Name != "err" {
			continue
		}
		if positions.PositionFor(identifier.Pos(), false).Line == 3 {
			want = variable
		} else {
			target = variable
		}
	}
	for _, tc := range []struct {
		name     string
		position token.Pos
		want     *types.Var
	}{
		{name: "before outer declaration", position: want.Pos() - 1},
		{name: "at outer declaration before scope begins", position: want.Pos()},
		{name: "after complete declaration", position: target.Pos(), want: want},
		{name: "current declaration remains excluded when visible", position: token.Pos(1 << 30), want: want},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ObserveOuterVariable(t.Context(), OuterVariableRequest{Variable: target, Position: tc.position})
			if err != nil || got.Variable != tc.want {
				t.Fatalf("native visibility position%d=%v,%v,want%v", tc.position, got.Variable, err, tc.want)
			}
		})
	}
}

func TestOuterVariableNativeLookupDoesNotBuildScopeInventory(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, depth := range []int{64, 16385} {
		t.Run(strconv.Itoa(depth), func(t *testing.T) {
			root := types.NewScope(types.Universe, 1, 1<<30, "native root")
			outer := types.NewVar(2, nil, "err", types.Universe.Lookup("error").Type())
			if root.Insert(outer) != nil {
				t.Fatal("native root insertion collided")
			}
			scope := root
			for range depth {
				scope = types.NewScope(scope, 1, 1<<30, "native child")
			}
			target := types.NewVar(3, nil, "err", types.Universe.Lookup("error").Type())
			if scope.Insert(target) != nil {
				t.Fatal("native leaf insertion collided")
			}
			before, err := hostfacts.ObserveGoAllocationTotal(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			got, err := ObserveOuterVariable(t.Context(), OuterVariableRequest{Variable: target, Position: 4})
			if err != nil || got.Variable != outer {
				t.Fatalf("native depth%d lookup=%v,%v,want%v", depth, got.Variable, err, outer)
			}
			after, err := hostfacts.ObserveGoAllocationTotal(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			allocated := after.Uint64() - before.Uint64()
			t.Logf("native scope depth%d query allocated%d bytes", depth, allocated)
			if allocated > 256<<10 {
				t.Fatalf("native scope query built an inventory: allocated%d bytes", allocated)
			}
		})
	}
}

func TestOuterVariableNativeRefusalsAndUnparentedNeutral(t *testing.T) {
	t.Parallel()
	variable := types.NewVar(1, nil, "err", types.Universe.Lookup("error").Type())
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(io.ErrClosedPipe)
	for _, tc := range []struct {
		name    string
		ctx     context.Context
		request OuterVariableRequest
		want    error
	}{
		{name: "nil context", request: OuterVariableRequest{Variable: variable, Position: 1}, want: core.ErrNilContext},
		{name: "cancelled context retains owner cause", ctx: ctx, request: OuterVariableRequest{Variable: variable, Position: 1}, want: context.Canceled},
		{name: "absent variable", ctx: t.Context(), request: OuterVariableRequest{Position: 1}, want: core.ErrGoToolchainContract},
		{name: "zero native position", ctx: t.Context(), request: OuterVariableRequest{Variable: variable}, want: core.ErrGoToolchainContract},
		{name: "negative native position", ctx: t.Context(), request: OuterVariableRequest{Variable: variable, Position: -1}, want: core.ErrGoToolchainContract},
		{name: "unparented native variable is neutral", ctx: t.Context(), request: OuterVariableRequest{Variable: variable, Position: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ObserveOuterVariable(tc.ctx, tc.request)
			if !errors.Is(err, tc.want) || got != (OuterVariableObservation{}) {
				t.Fatalf("native scope refusal=%+v,%v,want zero/%v", got, err, tc.want)
			}
			if tc.ctx == ctx && !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("native custom cancellation cause=%v,want owned cause", err)
			}
		})
	}
}

func FuzzOuterVariableMatchesNativeDeclarationIdentity(f *testing.F) {
	for mode := uint8(0); mode < 4; mode++ {
		f.Add(mode, uint8(1))
	}
	f.Fuzz(func(t *testing.T, mode, depth uint8) {
		nesting := int(depth%8) + 1
		prefix := ""
		switch mode % 4 {
		case 0:
			prefix = "var err error;_=err;"
		case 1:
			prefix = "{var err error;_=err};"
		case 2:
			prefix = "const err=1;_=err;"
		case 3:
		}
		source := "package p\nfunc F(){" + prefix + strings.Repeat("{", nesting) + "err:=error(nil);_=err;" + strings.Repeat("}", nesting) + "}\n"
		_, information := outerVariableFixture(t, source)
		var first, target *types.Var
		for identifier, object := range information.Defs {
			variable, ok := object.(*types.Var)
			if !ok || identifier.Name != "err" {
				continue
			}
			if first == nil || variable.Pos() < first.Pos() {
				first = variable
			}
			if target == nil || variable.Pos() > target.Pos() {
				target = variable
			}
		}
		got, err := ObserveOuterVariable(t.Context(), OuterVariableRequest{Variable: target, Position: target.Pos()})
		var want *types.Var
		if mode%4 == 0 {
			want = first
		}
		if err != nil || got.Variable != want {
			t.Fatalf("native scope mode%d depth%s=%v,%v,want%v", mode%4, strconv.Itoa(nesting), got.Variable, err, want)
		}
	})
}
