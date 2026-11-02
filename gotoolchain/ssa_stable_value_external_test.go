package gotoolchain_test

import (
	"context"
	"errors"
	"go/constant"
	"go/token"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSAStableValueNativeCellContracts(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, declaration, kind, initial, tail string
		want, wantStorage                      bool
	}{
		{name: "string readonly capture", kind: "string", initial: `source()`, want: true},
		{name: "integer readonly capture", kind: "int64", initial: `source()`, want: true},
		{name: "boolean readonly capture", kind: "bool", initial: `source()`, want: true},
		{name: "record readonly capture", kind: "struct{ N int }", initial: `source()`, want: true},
		{name: "array readonly capture", kind: "[2]int", initial: `source()`, want: true},
		{name: "pointer readonly capture preserves storage proof", declaration: `type Item struct{N int};`, kind: "*Item", initial: `source()`, want: true, wantStorage: true},
		{name: "interface readonly capture preserves storage proof", kind: "any", initial: `source()`, want: true, wantStorage: true},
		{name: "callable readonly capture", kind: "func()", initial: `source()`, want: true},
		{name: "named scalar readonly capture", declaration: `type Label string;`, kind: "Label", initial: `source()`, want: true},
		{name: "later parent replacement remains opaque", kind: "string", initial: `source()`, tail: `value=other`},
		{name: "later sibling closure write remains opaque", kind: "string", initial: `source()`, tail: `func(){value=other}()`},
		{name: "conditional later write remains opaque", kind: "string", initial: `source()`, tail: `if flag {value=other}`},
		{name: "opaque address escape remains opaque", declaration: `func escape(*string){};`, kind: "string", initial: `source()`, tail: `escape(&value)`},
		{name: "stored address escape remains opaque", declaration: `var escaped *string;`, kind: "string", initial: `source()`, tail: `escaped=&value`},
		{name: "retained untaken store remains opaque", kind: "string", initial: `source()`, tail: `if false {value=other}`},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			initial := row.initial
			source := `package sample;` + row.declaration + `func source()(out ` + row.kind + `){return};func sink(` + row.kind + `){};func Entry(other ` + row.kind + `,flag bool){value:=` + initial + `;sink(value);defer func(){sink(value)}();` + row.tail + `}`
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				args := stableValueSinkArguments(ctx, t, v, fn)
				left, err := v.StableValue(ctx, gotoolchain.SSAStableValueRequest{Value: args[0]})
				if err != nil {
					return err
				}
				right, err := v.StableValue(ctx, gotoolchain.SSAStableValueRequest{Value: args[1]})
				if err != nil {
					return err
				}
				if err := left.Validate(); err != nil {
					return err
				}
				if err := right.Validate(); err != nil {
					return err
				}
				if row.want {
					if left.Value == nil || left.Value != right.Value || left.Value == args[0] || right.Value == args[1] {
						t.Fatalf("stable parent/closure=%v/%v, want identical stored value rather than either load", left.Value, right.Value)
					}
				} else if left.Value != nil || right.Value != nil {
					t.Fatalf("opaque parent/closure=%v/%v, want no stable value", left.Value, right.Value)
				}
				legacy, err := v.SameStorage(ctx, gotoolchain.SSAStorageRequest{Left: args[0], Right: args[1]})
				if err != nil {
					return err
				}
				if legacy.Proven != row.wantStorage {
					t.Fatalf("legacy storage=%t,want unchanged %t", legacy.Proven, row.wantStorage)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func stableValueSinkArguments(ctx context.Context, t testing.TB, v gotoolchain.SSAUnitView, fn *ssa.Function) [2]ssa.Value {
	t.Helper()
	var args [2]ssa.Value
	count := 0
	for instruction, err := range v.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: fn, Extent: gotoolchain.SSAInstructionExtentLexical}) {
		if err != nil {
			t.Fatal(err)
		}
		call, ok := instruction.(*ssa.Call)
		if !ok {
			continue
		}
		callee := call.Common().StaticCallee()
		if callee == nil || callee.Name() != "sink" {
			continue
		}
		if len(call.Common().Args) != 1 || count == len(args) {
			t.Fatalf("sink arguments/calls=%d/%d,want one argument and two calls", len(call.Common().Args), count)
		}
		args[count] = call.Common().Args[0]
		count++
	}
	if count != len(args) {
		t.Fatalf("sink calls=%d,want two native path observations", count)
	}
	return args
}

func TestSSAStableValueNeutralAndRefusalContracts(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func sink(string){};func Entry(value string){sink(value);defer func(){sink(value)}()}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		args := stableValueSinkArguments(ctx, t, v, fn)
		// The parameter has one capture-cell store; a direct native constant is not a load.
		literal := ssa.NewConst(constant.MakeString("native constant"), fn.Params[0].Type())
		got, queryErr := v.StableValue(ctx, gotoolchain.SSAStableValueRequest{Value: literal})
		if queryErr != nil || got.Value != literal {
			t.Fatalf("constant=%v/%v,want same native value", got.Value, queryErr)
		}
		cause := errors.New("caller canceled native observation")
		canceled, cancel := context.WithCancelCause(ctx)
		cancel(cause)
		got, queryErr = v.StableValue(canceled, gotoolchain.SSAStableValueRequest{Value: args[0]})
		if got.Value != nil || !errors.Is(queryErr, context.Canceled) || !errors.Is(queryErr, cause) {
			t.Fatalf("canceled=%v/%v,want zero observation and both native causes", got.Value, queryErr)
		}
		for _, request := range []gotoolchain.SSAStableValueRequest{{}, {Value: (*ssa.UnOp)(nil)}, {Value: new(ssa.Const)}} {
			got, queryErr = v.StableValue(ctx, request)
			if got.Value != nil || !errors.Is(queryErr, core.ErrGoToolchainContract) {
				t.Fatalf("invalid request=%v/%v,want zero and typed contract refusal", got.Value, queryErr)
			}
		}
		got, queryErr = (gotoolchain.SSAUnitView{}).StableValue(ctx, gotoolchain.SSAStableValueRequest{Value: args[0]})
		if got.Value != nil || !errors.Is(queryErr, core.ErrGoToolchainContract) {
			t.Fatalf("absent loan=%v/%v,want zero and contract refusal", got.Value, queryErr)
		}
		var absent context.Context
		got, queryErr = v.StableValue(absent, gotoolchain.SSAStableValueRequest{Value: args[0]})
		if got.Value != nil || !errors.Is(queryErr, core.ErrNilContext) {
			t.Fatalf("nil context=%v/%v,want zero and nil-context refusal", got.Value, queryErr)
		}
		if err := (gotoolchain.SSAStableValueObservation{Value: (*ssa.UnOp)(nil)}).Validate(); !errors.Is(err, core.ErrGoToolchainOutput) {
			t.Fatalf("typed nil observation=%v,want output refusal", err)
		}
		// An unowned forged instruction cannot obtain a native value loan.
		forged := &ssa.UnOp{Op: token.MUL, X: args[0]}
		got, queryErr = v.StableValue(ctx, gotoolchain.SSAStableValueRequest{Value: forged})
		if got.Value != nil || !errors.Is(queryErr, core.ErrGoToolchainContract) {
			t.Fatalf("foreign instruction=%v/%v,want zero and contract refusal", got.Value, queryErr)
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func FuzzSSAStableValuePreservesScalarAndRefusesSiblingWrite(f *testing.F) {
	f.Add("native path", false)
	f.Add("native path", true)
	f.Add("\x00\xff", false)
	f.Fuzz(func(t *testing.T, payload string, rewrite bool) {
		tail := ""
		if rewrite {
			tail = `func(){value="\\x00"+value}()`
		}
		source := `package sample;func identity(s string)string{return s};func sink(string){};func Entry(){value:=identity(` + strconv.Quote(payload) + `);sink(value);defer func(){sink(value)}();` + tail + `}`
		a := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
		err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
			args := stableValueSinkArguments(ctx, t, v, v.Package.Func("Entry"))
			for _, argument := range args {
				got, queryErr := v.StableValue(ctx, gotoolchain.SSAStableValueRequest{Value: argument})
				if queryErr != nil {
					return queryErr
				}
				if rewrite {
					if got.Value != nil {
						t.Fatalf("sibling write admitted %v,want no stable value", got.Value)
					}
					continue
				}
				call, ok := got.Value.(*ssa.Call)
				if !ok || call.Common().StaticCallee() != v.Package.Func("identity") || len(call.Common().Args) != 1 {
					t.Fatalf("stable value=%v,want exact native identity call", got.Value)
				}
				c, ok := call.Common().Args[0].(*ssa.Const)
				if !ok || c.Value == nil || constant.StringVal(c.Value) != payload {
					t.Fatalf("stable input=%v,want exact original payload", call.Common().Args[0])
				}
			}
			return nil
		}})
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestSSAStableValueNativeDeferredCall(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, tail string
		want       bool
	}{
		{name: "readonly native call result captured for restore", want: true},
		{name: "parent replacement after deferred capture", tail: `path="other"`},
		{name: "sibling closure replacement after deferred capture", tail: `func(){path="other"}()`},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			source := `package sample;import("os";"path/filepath");func Entry(dir string,report func(error)){path:=filepath.Join(dir,"file");if err:=os.Chmod(path,0);err!=nil{report(err);return};defer func(){if err:=os.Chmod(path,0600);err!=nil{report(err)}}();` + row.tail + `}`
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				var observations [2]ssa.Value
				count := 0
				for instruction, scanErr := range v.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: v.Package.Func("Entry"), Extent: gotoolchain.SSAInstructionExtentLexical}) {
					if scanErr != nil {
						return scanErr
					}
					call, ok := instruction.(*ssa.Call)
					if !ok {
						continue
					}
					callee := call.Common().StaticCallee()
					if callee == nil || callee.Pkg == nil || callee.Pkg.Pkg.Path() != "os" || callee.Name() != "Chmod" {
						continue
					}
					if count == len(observations) {
						t.Fatalf("native Chmod calls exceed %d", len(observations))
					}
					got, queryErr := v.StableValue(ctx, gotoolchain.SSAStableValueRequest{Value: call.Common().Args[0]})
					if queryErr != nil {
						return queryErr
					}
					observations[count] = got.Value
					count++
				}
				if count != 2 {
					t.Fatalf("native Chmod calls=%d,want two", count)
				}
				if row.want {
					if observations[0] == nil || observations[0] != observations[1] {
						t.Fatalf("native deferred path=%v/%v,want same borrowed source value", observations[0], observations[1])
					}
				} else if observations[0] != nil || observations[1] != nil {
					t.Fatalf("replaced native path=%v/%v,want no stable value", observations[0], observations[1])
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSSAStableValueRefusesDamagedNativeLoad(t *testing.T) {
	t.Parallel()
	for _, form := range []string{"missing address", "foreign address", "mismatched pointee"} {
		t.Run(form, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func sink(string){};func Entry(value string,other *int){sink(value);defer func(){sink(value)}()}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				args := stableValueSinkArguments(ctx, t, v, fn)
				load, ok := args[0].(*ssa.UnOp)
				if !ok {
					t.Fatalf("native argument=%T,want captured string load", args[0])
				}
				initial := load.X
				baseline, queryErr := v.StableValue(ctx, gotoolchain.SSAStableValueRequest{Value: load})
				if queryErr != nil || baseline.Value == nil {
					t.Fatalf("native precondition=%v/%v,want stable value", baseline.Value, queryErr)
				}
				switch form {
				case "missing address":
					load.X = nil
				case "foreign address":
					clone := *fn.Params[1]
					load.X = &clone
				case "mismatched pointee":
					load.X = fn.Params[1]
				default:
					t.Fatalf("damage=%q,want an authored native mutation", form)
				}
				got, queryErr := v.StableValue(ctx, gotoolchain.SSAStableValueRequest{Value: load})
				load.X = initial
				if got.Value != nil || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
					t.Fatalf("damaged native load=%v/%v,want zero and output refusal", got.Value, queryErr)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
