package gotoolchain

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/testserial"
)

func syntaxWalkOracle(root ast.Node) []SyntaxNodeObservation {
	var observations []SyntaxNodeObservation
	var ancestors []ast.Node
	ast.Inspect(root, func(node ast.Node) bool {
		if node == nil {
			ancestors = ancestors[:len(ancestors)-1]
			return true
		}
		observations = append(observations, SyntaxNodeObservation{Node: node, Ancestors: append([]ast.Node(nil), ancestors...)})
		ancestors = append(ancestors, node)
		return true
	})
	return observations
}

func TestSyntaxWalkBorrowsExactNativePreorderAndAncestors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, source string }{
		{name: "package", source: "package sample\n"},
		{name: "declarations", source: "package sample\nconst Count=1\ntype Value struct{Count int}\n"},
		{name: "closures_and_loops", source: "package sample\nfunc Read(){for {go func(){for range 2 {read()}}();break}}"},
		{name: "if_initializer_and_else", source: "package sample\nfunc Read(){if n:=read();n>0 {read()} else if n==0 {read()}}"},
		{name: "switch_and_type_switch", source: "package sample\nfunc Read(v any){switch v {case 1:read();default:read()};switch v.(type){case int:read()}}"},
		{name: "select_and_channels", source: "package sample\nfunc Read(c chan int){select{case c<-1:read();case n:=<-c:_=n;default:read()}}"},
		{name: "generics_and_methods", source: "package sample\ntype Value[T any] struct{X T}\nfunc(v Value[T]) Read()T{return v.X}"},
		{name: "labels_and_branches", source: "package sample\nfunc Read(){Done:for{if read(){break Done};continue};return}"},
		{name: "defer_and_nested_literals", source: "package sample\nfunc Read(){defer func(){read()}();_ = []int{func()int{return 1}()}}"},
		{name: "source_directives_are_native_ast_data", source: "package sample\n//line invented.go:700\nfunc Read(){read()}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := sourceFileRequestForTest(t, t.TempDir(), tc.source)
			file, err := ParseFile(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			expected := syntaxWalkOracle(file)
			index := 0
			for observation, err := range WalkSyntax(t.Context(), SyntaxWalkRequest{Root: file}) {
				if err != nil || observation.Validate() != nil || index >= len(expected) || observation.Node != expected[index].Node || !slices.Equal(observation.Ancestors, expected[index].Ancestors) {
					t.Fatalf("native syntax observation[%d]=%+v,%v,want exact native node and parent path", index, observation, err)
				}
				index++
			}
			if index != len(expected) {
				t.Fatalf("native syntax nodes=%d,want %d", index, len(expected))
			}
		})
	}
}

func TestSyntaxWalkConsumerControlsFutureNativeSyntax(t *testing.T) {
	t.Parallel()
	request := sourceFileRequestForTest(t, t.TempDir(), "package sample\nfunc Read(){read();read()}\n")
	file, err := ParseFile(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	for observation, err := range WalkSyntax(t.Context(), SyntaxWalkRequest{Root: file}) {
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := observation.Node.(*ast.CallExpr); !ok {
			continue
		}
		calls++
		file.Decls[0].(*ast.FuncDecl).Body.List[1] = &ast.EmptyStmt{}
	}
	if calls != 1 {
		t.Fatalf("borrowed future syntax calls=%d,want one before mutation", calls)
	}
	seen := 0
	WalkSyntax(t.Context(), SyntaxWalkRequest{Root: file})(func(observation SyntaxNodeObservation, err error) bool {
		if err != nil || observation.Node != file {
			t.Fatalf("root loan=%+v,%v", observation, err)
		}
		seen++
		file.Decls = []ast.Decl{(*ast.FuncDecl)(nil)}
		return false
	})
	if seen != 1 {
		t.Fatalf("consumer refusal observations=%d,want root only", seen)
	}
}

func TestSyntaxWalkPreservesConsumerPanicIdentity(t *testing.T) {
	t.Parallel()
	defer func() {
		if got := recover(); got != io.ErrClosedPipe {
			t.Fatalf("consumer panic=%v,want exact native marker", got)
		}
	}()
	for range WalkSyntax(t.Context(), SyntaxWalkRequest{Root: ast.NewIdent("value")}) {
		panic(io.ErrClosedPipe)
	}
}

func TestSyntaxWalkRefusesMissingOwnershipAndPreservesNativeCancellation(t *testing.T) {
	t.Parallel()
	for _, during := range []bool{false, true} {
		t.Run(strconv.FormatBool(during), func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			if !during {
				cancel(io.ErrClosedPipe)
			}
			root := &ast.BinaryExpr{X: ast.NewIdent("a"), Y: ast.NewIdent("b"), Op: token.ADD}
			loans, refusals := 0, 0
			for observation, err := range WalkSyntax(ctx, SyntaxWalkRequest{Root: root}) {
				if err != nil {
					if observation.Node != nil || len(observation.Ancestors) != 0 || !errors.Is(err, context.Canceled) || !errors.Is(err, io.ErrClosedPipe) {
						t.Fatalf("native syntax refusal=%+v,%v", observation, err)
					}
					refusals++
					continue
				}
				loans++
				cancel(io.ErrClosedPipe)
			}
			want := 0
			if during {
				want = 1
			}
			if loans != want || refusals != 1 {
				t.Fatalf("syntax loans/refusals=%d/%d,want %d/1", loans, refusals, want)
			}
		})
	}
	for _, tc := range []struct {
		ctx     context.Context
		request SyntaxWalkRequest
		want    error
	}{
		{ctx: t.Context(), want: core.ErrGoToolchainContract},
		{request: SyntaxWalkRequest{Root: ast.NewIdent("value")}, want: core.ErrNilContext},
	} {
		seen := 0
		for observation, err := range WalkSyntax(tc.ctx, tc.request) {
			if observation.Node != nil || len(observation.Ancestors) != 0 || !errors.Is(err, tc.want) {
				t.Fatalf("syntax admission=%+v,%v,want %v", observation, err, tc.want)
			}
			seen++
		}
		if seen != 1 {
			t.Fatalf("syntax admission refusals=%d,want one", seen)
		}
	}
	if (SyntaxNodeObservation{}).Validate() != core.ErrGoToolchainContract {
		t.Fatal("missing syntax node accepted")
	}
}

func TestSyntaxWalkRetainsDepthInsteadOfNodePopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, population := range []int{64, 16385} {
		t.Run(strconv.Itoa(population), func(t *testing.T) {
			request := sourceFileRequestForTest(t, t.TempDir(), "package sample\nfunc Read(){"+strings.Repeat("read();", population)+"}\n")
			file, err := ParseFile(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			before, err := hostfacts.ObserveCollectedGoHeap(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			calls, loans := 0, 0
			for observation, err := range WalkSyntax(t.Context(), SyntaxWalkRequest{Root: file}) {
				if err != nil {
					t.Fatal(err)
				}
				loans++
				if loans == 1 {
					after, err := hostfacts.ObserveCollectedGoHeap(t.Context())
					if err != nil {
						t.Fatal(err)
					}
					growth := int64(after.Uint64()) - int64(before.Uint64())
					t.Logf("native calls=%d retained walker bytes=%d", population, growth)
					if growth > 256<<10 {
						t.Fatalf("native walker retained %d bytes of node population", growth)
					}
				}
				if _, ok := observation.Node.(*ast.CallExpr); ok {
					calls++
				}
			}
			if calls != population {
				t.Fatalf("native calls=%d,want complete extent %d", calls, population)
			}
			runtime.KeepAlive(file)
		})
	}
}

func FuzzSyntaxWalkNativeNodeAndAncestryConservation(f *testing.F) {
	f.Add("package sample\nfunc Read(){for {read();break}}\n")
	f.Add("package sample\ntype Value[T any] struct{X T}\n")
	f.Add("")
	f.Fuzz(func(t *testing.T, source string) {
		file, err := parser.ParseFile(token.NewFileSet(), "source.go", source, parser.ParseComments)
		if err != nil {
			return
		}
		expected := syntaxWalkOracle(file)
		index := 0
		for observation, err := range WalkSyntax(t.Context(), SyntaxWalkRequest{Root: file}) {
			if err != nil || index >= len(expected) || observation.Node != expected[index].Node || !slices.Equal(observation.Ancestors, expected[index].Ancestors) {
				t.Fatalf("native syntax conservation[%d]=%+v,%v", index, observation, err)
			}
			index++
		}
		if index != len(expected) {
			t.Fatalf("native syntax census=%d,want %d", index, len(expected))
		}
	})
}

func TestSyntaxWalkKeepsNativeMalformedTreePanic(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		walk func(ast.Node)
	}{
		{name: "native_standard_walker", walk: func(root ast.Node) { ast.PreorderStack(root, nil, func(ast.Node, []ast.Node) bool { return true }) }},
		{name: "primitive_native_observations", walk: func(root ast.Node) {
			for range WalkSyntax(t.Context(), SyntaxWalkRequest{Root: root}) {
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				if _, ok := recover().(runtime.Error); !ok {
					t.Fatal("malformed typed-nil AST did not retain native runtime panic")
				}
			}()
			var malformed *ast.FuncDecl
			tc.walk(malformed)
		})
	}
}
