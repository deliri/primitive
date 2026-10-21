package gotoolchain

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"io"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestGoBuildConstraintNativeProducerClassification(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ name, text string }{
		{name: "ordinary documentation", text: "// ordinary documentation"},
		{name: "quoted directive is documentation", text: `// documentation "//go:build ignore"`},
		{name: "legacy directive remains a different native form", text: "// +build ignore"},
		{name: "generate directive is not build selection", text: "//go:generate echo ignore"},
		{name: "wrong directive case remains ordinary", text: "//Go:build ignore"},
		{name: "block comment is not a line directive", text: "/*go:build ignore*/"},
		{name: "missing directive colon remains ordinary", text: "//go build ignore"},
		{name: "directive name lookalike remains ordinary", text: "//go:builder ignore"},
		{name: "native ignore tag", text: "//go:build ignore"},
		{name: "native linux tag", text: "//go:build linux"},
		{name: "native windows tag", text: "//go:build windows"},
		{name: "native architecture tag", text: "//go:build amd64"},
		{name: "dotted tag identity", text: "//go:build custom.tag"},
		{name: "underscored tag identity", text: "//go:build custom_tag"},
		{name: "numeric tag identity", text: "//go:build custom123"},
		{name: "true spelling remains a tag", text: "//go:build true"},
		{name: "false spelling remains a tag", text: "//go:build false"},
		{name: "negated tag", text: "//go:build !ignore"},
		{name: "double negation", text: "//go:build !!ignore"},
		{name: "parenthesized native tag", text: "//go:build (ignore)"},
		{name: "conjunction", text: "//go:build linux && amd64"},
		{name: "disjunction", text: "//go:build linux || windows"},
		{name: "operator precedence", text: "//go:build linux || windows && amd64"},
		{name: "grouped conjunction", text: "//go:build (linux || windows) && amd64"},
		{name: "negated expression", text: "//go:build !(linux || windows)"},
		{name: "mixed negated conjunction", text: "//go:build linux && !amd64"},
		{name: "horizontal whitespace", text: "//go:build \tlinux\t&&\tamd64\t"},
		{name: "extra marker space", text: "//go:build  ignore  "},
		{name: "missing expression", text: "//go:build"},
		{name: "empty expression", text: "//go:build    "},
		{name: "missing left conjunction", text: "//go:build && linux"},
		{name: "missing right conjunction", text: "//go:build linux &&"},
		{name: "missing left disjunction", text: "//go:build || linux"},
		{name: "missing right disjunction", text: "//go:build linux ||"},
		{name: "single ampersand is not conjunction", text: "//go:build linux & windows"},
		{name: "single pipe is not disjunction", text: "//go:build linux | windows"},
		{name: "xor has no native grammar", text: "//go:build linux ^ windows"},
		{name: "addition has no native grammar", text: "//go:build linux + windows"},
		{name: "comparison has no native grammar", text: "//go:build linux == windows"},
		{name: "unclosed grouping", text: "//go:build (linux"},
		{name: "unopened grouping", text: "//go:build linux)"},
		{name: "empty grouping", text: "//go:build ()"},
		{name: "adjacent operands", text: "//go:build linux windows"},
		{name: "unsupported dollar tag", text: "//go:build $tag"},
		{name: "unsupported hyphen tag", text: "//go:build custom-tag"},
		{name: "unsupported slash tag", text: "//go:build custom/tag"},
		{name: "legacy comma is not modern conjunction", text: "//go:build linux,windows"},
		{name: "semicolon cannot append tokens", text: "//go:build linux;windows"},
		{name: "quoted string is not a tag", text: `//go:build "linux"`},
		{name: "quoted rune is not a tag", text: "//go:build 'linux'"},
		{name: "comment suffix is not a tag", text: "//go:build linux /* ignored */"},
		{name: "bare negation", text: "//go:build !"},
		{name: "operator after negation", text: "//go:build !&&linux"},
		{name: "native depth refusal", text: "//go:build " + strings.Repeat("(", 1100) + "linux" + strings.Repeat(")", 1100)},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			file, err := parser.ParseFile(token.NewFileSet(), "subject.go", row.text+"\npackage subject\n", parser.ParseComments)
			if err != nil || len(file.Comments) == 0 {
				t.Fatalf("native comment producer%+v/%v", file, err)
			}
			proveGoBuildConstraintNative(t, file.Comments[0].List[0])
		})
	}
}

func proveGoBuildConstraintNative(t testing.TB, comment *ast.Comment) {
	t.Helper()
	observed, err := ObserveGoBuildConstraint(t.Context(), GoBuildConstraintRequest{Comment: comment})
	if !constraint.IsGoBuild(comment.Text) {
		if err != nil || observed != (GoBuildConstraintObservation{}) || observed.Validate() != nil {
			t.Fatalf("ordinary comment observation%+v/%v", observed, err)
		}
		return
	}
	native, nativeErr := constraint.Parse(comment.Text)
	if nativeErr != nil {
		var wanted, got *constraint.SyntaxError
		if observed != (GoBuildConstraintObservation{}) || !errors.Is(err, core.ErrGoToolchainOutput) || !errors.As(nativeErr, &wanted) || !errors.As(err, &got) || *wanted != *got {
			t.Fatalf("native constraint refusal%+v/%v want%v", observed, err, nativeErr)
		}
		return
	}
	if err != nil || !observed.Present || observed.Validate() != nil || observed.Expression.String() != native.String() {
		t.Fatalf("native constraint observation%+v/%v want%s", observed, err, native)
	}
	for mask := range 16 {
		tags := [...]string{"linux", "amd64", "ignore", "custom.tag"}
		accept := func(tag string) bool {
			for index, name := range tags {
				if tag == name {
					return mask&(1<<index) != 0
				}
			}
			return false
		}
		if got, want := observed.Expression.Eval(accept), native.Eval(accept); got != want {
			t.Fatalf("native constraint evaluation mask%d got%t want%t", mask, got, want)
		}
	}
}

func TestGoBuildConstraintHasNoTextExtentQuota(t *testing.T) {
	t.Parallel()
	for _, extent := range []int{(32 << 10) - 1, 32 << 10, (32 << 10) + 1, (64 << 10) - 1, 64 << 10, (64 << 10) + 1, 8 << 20} {
		t.Run(fmt.Sprint(extent), func(t *testing.T) {
			t.Parallel()
			proveGoBuildConstraintNative(t, &ast.Comment{Text: "//go:build " + strings.Repeat(" ", extent) + "ignore"})
			proveGoBuildConstraintNative(t, &ast.Comment{Text: "//go:build " + strings.Repeat("x", extent)})
		})
	}
}

func TestGoBuildConstraintRefusesContradictoryAndCancelledFacts(t *testing.T) {
	t.Parallel()
	comment := &ast.Comment{Text: "//go:build ignore"}
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(io.ErrClosedPipe)
	for _, row := range []struct {
		name    string
		ctx     context.Context
		request GoBuildConstraintRequest
		want    error
	}{
		{name: "missing native comment", ctx: t.Context(), want: core.ErrGoToolchainContract},
		{name: "missing context", request: GoBuildConstraintRequest{Comment: comment}, want: core.ErrGoToolchainContract},
		{name: "native custom cancellation", ctx: ctx, request: GoBuildConstraintRequest{Comment: comment}, want: io.ErrClosedPipe},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			observed, err := ObserveGoBuildConstraint(row.ctx, row.request)
			if observed != (GoBuildConstraintObservation{}) || !errors.Is(err, row.want) {
				t.Fatalf("native constraint ingress%+v/%v want%v", observed, err, row.want)
			}
		})
	}
	for _, row := range []struct {
		name        string
		observation GoBuildConstraintObservation
	}{
		{name: "present without native expression", observation: GoBuildConstraintObservation{Present: true}},
		{name: "expression without presence", observation: GoBuildConstraintObservation{Expression: &constraint.TagExpr{Tag: "ignore"}}},
		{name: "typed nil native expression", observation: GoBuildConstraintObservation{Expression: (*constraint.TagExpr)(nil), Present: true}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if err := row.observation.Validate(); !errors.Is(err, core.ErrGoToolchainContract) {
				t.Fatalf("native constraint contradiction%+v/%v", row.observation, err)
			}
		})
	}
}

type goBuildConstraintCancellationContext struct {
	context.Context
	cancel context.CancelCauseFunc
	checks int
}

func (c *goBuildConstraintCancellationContext) Err() error {
	c.checks++
	if c.checks == 2 {
		c.cancel(io.ErrClosedPipe)
	}
	return c.Context.Err()
}

func TestGoBuildConstraintWithholdsFactsOnTerminalCancellation(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"//go:build ignore", "// ordinary", "//go:build linux &&"} {
		t.Run(text, func(t *testing.T) {
			t.Parallel()
			parent, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			ctx := &goBuildConstraintCancellationContext{Context: parent, cancel: cancel}
			observed, err := ObserveGoBuildConstraint(ctx, GoBuildConstraintRequest{Comment: &ast.Comment{Text: text}})
			if observed != (GoBuildConstraintObservation{}) || !errors.Is(err, io.ErrClosedPipe) || !errors.Is(err, context.Canceled) {
				t.Fatalf("terminal native constraint%+v/%v checks%d", observed, err, ctx.checks)
			}
			if constraint.IsGoBuild(text) {
				if _, nativeErr := constraint.Parse(text); nativeErr != nil {
					var native *constraint.SyntaxError
					if !errors.As(err, &native) {
						t.Fatalf("terminal constraint lost native syntax error%v", err)
					}
				}
			}
		})
	}
}

func FuzzGoBuildConstraintConservesNativeGrammar(f *testing.F) {
	for _, text := range []string{"//go:build ignore", "//go:build (linux || windows) && !amd64", "//go:build linux &&", "// ordinary", "// +build ignore"} {
		f.Add(text)
	}
	f.Fuzz(func(t *testing.T, text string) { proveGoBuildConstraintNative(t, &ast.Comment{Text: text}) })
}

// An embedded native interface promotes its sealed methods into a foreign Go
// carrier. Such a carrier is not one of the parser's native expression nodes.
type goBuildConstraintForeignExpression struct{ constraint.Expr }

func TestGoBuildConstraintRefusesForeignExpressionCarriers(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name       string
		expression constraint.Expr
	}{
		{name: "value carrier", expression: goBuildConstraintForeignExpression{Expr: &constraint.TagExpr{Tag: "ignore"}}},
		{name: "pointer carrier", expression: &goBuildConstraintForeignExpression{Expr: &constraint.TagExpr{Tag: "ignore"}}},
		{name: "nil pointer carrier", expression: (*goBuildConstraintForeignExpression)(nil)},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if err := (GoBuildConstraintObservation{Present: true, Expression: row.expression}).Validate(); !errors.Is(err, core.ErrGoToolchainContract) {
				t.Fatalf("foreign expression carrier admitted%T/%v", row.expression, err)
			}
		})
	}
}
