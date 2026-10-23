package gotoolchain_test

import (
	"context"
	"errors"
	"go/constant"
	"go/types"
	"io"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/capabilities"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

const nativeConditionalSource = `package sample;func consume(bool){};func yes(){};func no(){};func Entry(flag bool){a:=!flag;consume(a);b:=a==false;consume(b);if b{yes()}else{no()}};func Other(flag bool){if flag{yes()}else{no()}};func Plain(){yes()}`

func TestSSAConditionalBranchUsesCatalogOwner(t *testing.T) {
	t.Parallel()
	match, err := capabilities.Resolve(capabilities.ForPackage(capabilities.ScopeProduction, core.PackageGoToolchain))
	if err != nil {
		t.Fatalf("owner got=%+v,%v, want capability,nil", match, err)
	}
	path, err := match.Capability.ImportPath()
	if err != nil || reflect.TypeFor[gotoolchain.SSAConditionalBranchRequest]().PkgPath() != path || reflect.TypeFor[gotoolchain.SSAConditionalBranchObservation]().PkgPath() != path {
		t.Fatalf("conditional owner got=%s,%s,%v, want %s,nil", reflect.TypeFor[gotoolchain.SSAConditionalBranchRequest]().PkgPath(), reflect.TypeFor[gotoolchain.SSAConditionalBranchObservation]().PkgPath(), err, path)
	}
}

// Every composition is compiled independently. The oracle evaluates authored
// operators on true; it never asks the query or a second IR walker for polarity.
func TestSSAConditionalBranchExhaustiveBooleanProjection(t *testing.T) {
	t.Parallel()
	var source strings.Builder
	source.WriteString(`package sample;type Flag=bool;func consume(Flag){};func yes(){};func no(){};`)
	for code := 0; code < 128; code++ {
		source.WriteString("func F" + strconv.Itoa(code) + "(flag Flag){a:=flag;")
		for step := 0; step < 7; step++ {
			expression := "!a"
			if code&(1<<step) != 0 {
				expression = "a==false"
			}
			if step%3 == 1 {
				expression = "true!=a"
				if code&(1<<step) != 0 {
					expression = "a!=false"
				}
			}
			if step%3 == 2 {
				expression = "a==true"
				if code&(1<<step) != 0 {
					expression = "false==a"
				}
			}
			source.WriteString("a=" + expression + ";consume(a);")
		}
		source.WriteString("if a{yes()}else{no()}};")
	}
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), source.String())
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		for code := 0; code < 128; code++ {
			fn := v.Package.Func("F" + strconv.Itoa(code))
			block := fn.Blocks[0]
			want := true
			for step := 0; step < 7; step++ {
				switch step % 3 {
				case 0:
					want = !want
				case 1:
					if code&(1<<step) == 0 {
						want = !want
					}
				case 2:
					if code&(1<<step) != 0 {
						want = !want
					}
				}
			}
			got, queryErr := v.ConditionalBranch(ctx, gotoolchain.SSAConditionalBranchRequest{Block: block})
			positive, negative := block.Succs[0], block.Succs[1]
			if !want {
				positive, negative = negative, positive
			}
			if queryErr != nil || got.Condition != fn.Params[0] || got.WhenTrue != positive || got.WhenFalse != negative {
				t.Fatalf("composition=%d got=%+v,%v, want native parameter and edges %d/%d", code, got, queryErr, positive.Index, negative.Index)
			}
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}

func TestSSAConditionalBranchRefusesBrokenOperandCustody(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name    string
		mutate  func(gotoolchain.SSAUnitView, *ssa.Function, *ssa.If)
		wantErr error
	}{
		{"missing", func(_ gotoolchain.SSAUnitView, _ *ssa.Function, b *ssa.If) { b.Cond = nil }, core.ErrGoToolchainOutput},
		{"typed nil", func(_ gotoolchain.SSAUnitView, _ *ssa.Function, b *ssa.If) { b.Cond = (*ssa.Call)(nil) }, core.ErrGoToolchainOutput},
		{"foreign parameter", func(v gotoolchain.SSAUnitView, _ *ssa.Function, b *ssa.If) {
			b.Cond = v.Package.Func("Other").Params[0]
		}, core.ErrGoToolchainOutput},
		{"detached parameter", func(_ gotoolchain.SSAUnitView, f *ssa.Function, b *ssa.If) { p := *f.Params[0]; b.Cond = &p }, core.ErrGoToolchainOutput},
		{"detached condition", func(_ gotoolchain.SSAUnitView, _ *ssa.Function, b *ssa.If) { x := *b.Cond.(*ssa.BinOp); b.Cond = &x }, core.ErrGoToolchainOutput},
		{"missing child", func(_ gotoolchain.SSAUnitView, _ *ssa.Function, b *ssa.If) { b.Cond.(*ssa.BinOp).X = nil }, core.ErrGoToolchainOutput},
		{"typed nil child", func(_ gotoolchain.SSAUnitView, _ *ssa.Function, b *ssa.If) { b.Cond.(*ssa.BinOp).X = (*ssa.UnOp)(nil) }, core.ErrGoToolchainOutput},
		{"detached child", func(_ gotoolchain.SSAUnitView, _ *ssa.Function, b *ssa.If) {
			x := *b.Cond.(*ssa.BinOp).X.(*ssa.UnOp)
			b.Cond.(*ssa.BinOp).X = &x
		}, core.ErrGoToolchainOutput},
		{"foreign child", func(v gotoolchain.SSAUnitView, _ *ssa.Function, b *ssa.If) {
			b.Cond.(*ssa.BinOp).X = v.Package.Func("Other").Params[0]
		}, core.ErrGoToolchainOutput},
		{"self cycle", func(_ gotoolchain.SSAUnitView, _ *ssa.Function, b *ssa.If) { b.Cond.(*ssa.BinOp).X = b.Cond }, core.ErrGoToolchainOutput},
		{"two value cycle", func(_ gotoolchain.SSAUnitView, _ *ssa.Function, b *ssa.If) {
			b.Cond.(*ssa.BinOp).X.(*ssa.UnOp).X = b.Cond
		}, core.ErrGoToolchainOutput},
		{"nonboolean child", func(_ gotoolchain.SSAUnitView, _ *ssa.Function, b *ssa.If) {
			b.Cond.(*ssa.BinOp).X = ssa.NewConst(constant.MakeInt64(1), types.Typ[types.Int])
		}, core.ErrGoToolchainOutput},
		{"missing boolean constant", func(_ gotoolchain.SSAUnitView, _ *ssa.Function, b *ssa.If) {
			b.Cond.(*ssa.BinOp).Y.(*ssa.Const).Value = nil
		}, core.ErrGoToolchainOutput},
		{"wrong boolean constant", func(_ gotoolchain.SSAUnitView, _ *ssa.Function, b *ssa.If) {
			b.Cond.(*ssa.BinOp).Y = ssa.NewConst(constant.MakeInt64(1), types.Typ[types.Bool])
		}, core.ErrGoToolchainOutput},
		{"missing edge", func(_ gotoolchain.SSAUnitView, f *ssa.Function, _ *ssa.If) { f.Blocks[0].Succs = f.Blocks[0].Succs[:1] }, core.ErrGoToolchainOutput},
		{"foreign edge", func(v gotoolchain.SSAUnitView, f *ssa.Function, _ *ssa.If) {
			f.Blocks[0].Succs[0] = v.Package.Func("Other").Blocks[0]
		}, core.ErrGoToolchainOutput},
		{"detached edge", func(_ gotoolchain.SSAUnitView, f *ssa.Function, _ *ssa.If) {
			x := *f.Blocks[0].Succs[0]
			f.Blocks[0].Succs[0] = &x
		}, core.ErrGoToolchainOutput},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeConditionalSource)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				branch := fn.Blocks[0].Instrs[len(fn.Blocks[0].Instrs)-1].(*ssa.If)
				row.mutate(v, fn, branch)
				got, queryErr := v.ConditionalBranch(ctx, gotoolchain.SSAConditionalBranchRequest{Block: fn.Blocks[0]})
				if got != (gotoolchain.SSAConditionalBranchObservation{}) || !errors.Is(queryErr, row.wantErr) {
					t.Fatalf("custody got=%+v,%v, want zero,%v", got, queryErr, row.wantErr)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}

func TestSSAConditionalBranchProducerLayerTriad(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeConditionalSource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		block := fn.Blocks[0]
		got, queryErr := v.ConditionalBranch(ctx, gotoolchain.SSAConditionalBranchRequest{Block: block})
		if queryErr != nil || got.Condition != fn.Params[0] || got.WhenTrue != block.Succs[0] || got.WhenFalse != block.Succs[1] {
			t.Fatalf("positive got=%+v,%v, want flag and original edges,nil", got, queryErr)
		}
		broken := *block
		got, queryErr = v.ConditionalBranch(ctx, gotoolchain.SSAConditionalBranchRequest{Block: &broken})
		if got != (gotoolchain.SSAConditionalBranchObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
			t.Fatalf("negative got=%+v,%v, want zero,%v", got, queryErr, core.ErrGoToolchainOutput)
		}
		got, queryErr = v.ConditionalBranch(ctx, gotoolchain.SSAConditionalBranchRequest{Block: v.Package.Func("Plain").Blocks[0]})
		if got != (gotoolchain.SSAConditionalBranchObservation{}) || queryErr != nil {
			t.Fatalf("neutral got=%+v,%v, want zero,nil", got, queryErr)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}

type nativeConditionalContext struct {
	context.Context
	checks, failAt int
	cancel         context.CancelCauseFunc
}

func (c *nativeConditionalContext) Err() error {
	c.checks++
	if c.checks == c.failAt {
		c.cancel(io.ErrClosedPipe)
	}
	return c.Context.Err()
}

func TestSSAConditionalBranchCancellationAtEveryObservation(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeConditionalSource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		r := gotoolchain.SSAConditionalBranchRequest{Block: v.Package.Func("Entry").Blocks[0]}
		parent, cancel := context.WithCancelCause(ctx)
		defer cancel(nil)
		probe := &nativeConditionalContext{Context: parent, cancel: cancel}
		got, queryErr := v.ConditionalBranch(probe, r)
		if queryErr != nil || got.Condition == nil || probe.checks < 3 {
			t.Fatalf("calibration got=%+v,%v checks=%d, want native projection and observations", got, queryErr, probe.checks)
		}
		t.Logf("native fixture query context observations=%d", probe.checks)
		for at := 1; at <= probe.checks; at++ {
			parent, cancel := context.WithCancelCause(ctx)
			controlled := &nativeConditionalContext{Context: parent, cancel: cancel, failAt: at}
			got, queryErr := v.ConditionalBranch(controlled, r)
			cancel(nil)
			if got != (gotoolchain.SSAConditionalBranchObservation{}) || !errors.Is(queryErr, context.Canceled) || !errors.Is(queryErr, io.ErrClosedPipe) {
				t.Fatalf("observation=%d got=%+v,%v, want zero,cancelled+cause", at, got, queryErr)
			}
		}
		var absent context.Context
		got, queryErr = v.ConditionalBranch(absent, r)
		if got != (gotoolchain.SSAConditionalBranchObservation{}) || !errors.Is(queryErr, core.ErrNilContext) {
			t.Fatalf("nil context got=%+v,%v, want zero,%v", got, queryErr, core.ErrNilContext)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}

// This native fixture has thirteen context observations, including the final
// receipt check. Freeze that schedule so removing the final check cannot also
// move the test's cancellation earlier and disguise the missing boundary.
func TestSSAConditionalBranchFinalCancellationZerosProjection(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeConditionalSource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		parent, cancel := context.WithCancelCause(ctx)
		defer cancel(nil)
		controlled := &nativeConditionalContext{Context: parent, cancel: cancel, failAt: 13}
		got, queryErr := v.ConditionalBranch(controlled, gotoolchain.SSAConditionalBranchRequest{Block: v.Package.Func("Entry").Blocks[0]})
		if got != (gotoolchain.SSAConditionalBranchObservation{}) || !errors.Is(queryErr, context.Canceled) || !errors.Is(queryErr, io.ErrClosedPipe) {
			t.Fatalf("final receipt got=%+v,%v checks=%d, want zero,cancelled with native cause", got, queryErr, controlled.checks)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}
