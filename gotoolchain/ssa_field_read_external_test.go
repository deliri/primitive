package gotoolchain_test

import (
	"context"
	"errors"
	"go/token"
	"go/types"
	"reflect"
	"testing"

	"github.com/deliri/primitive/v2026/capabilities"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

const nativeFieldReadSource = `package sample;type S struct{A int;B int;Flag bool};type P *S;func Entry(s *S)int{return s.A};func Second(s *S)int{return s.B};func Flag(s *S)bool{return s.Flag};func NamedPointer(s P)int{return s.A};func Mixed(s *S)(int,bool){return s.A,s.Flag};func Plain(p *int)int{return *p};func Not(flag bool)bool{return !flag};func Other(s *S)int{return s.A};func Untaken(s *S)int{if false{return s.A};return 0};func Closure(s *S)func()int{return func()int{return s.A}}`

func nativeFieldReadInstruction(t testing.TB, fn *ssa.Function, field bool) *ssa.UnOp {
	t.Helper()
	for _, block := range fn.Blocks {
		for _, instruction := range block.Instrs {
			read, ok := instruction.(*ssa.UnOp)
			if !ok {
				continue
			}
			if _, ok := read.X.(*ssa.FieldAddr); ok == field {
				return read
			}
		}
	}
	t.Fatalf("native unary instruction got absent in %v, want field=%t", fn, field)
	return nil
}

func TestSSAFieldReadUsesPrimitiveCatalogOwner(t *testing.T) {
	t.Parallel()
	match, err := capabilities.Resolve(capabilities.ForPackage(capabilities.ScopeProduction, core.PackageGoToolchain))
	if err != nil {
		t.Fatalf("field-read owner got=%+v,%v, want compiled capability,nil", match, err)
	}
	path, err := match.Capability.ImportPath()
	if err != nil || reflect.TypeFor[gotoolchain.SSAFieldReadRequest]().PkgPath() != path {
		t.Fatalf("field-read request owner got=%s,%v, want %s,nil", reflect.TypeFor[gotoolchain.SSAFieldReadRequest]().PkgPath(), err, path)
	}
}

func TestSSAFieldReadLendsExactNativeFieldAndBase(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeFieldReadSource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		record := view.Source.Types.Scope().Lookup("S").Type().Underlying().(*types.Struct)
		for _, row := range []struct {
			name      string
			wantField int
			closure   bool
		}{
			{"Entry", 0, false}, {"Second", 1, false}, {"Flag", 2, false}, {"NamedPointer", 0, false}, {"Closure", 0, true},
		} {
			fn := view.Package.Func(row.name)
			if row.closure {
				fn = fn.AnonFuncs[0]
			}
			read := nativeFieldReadInstruction(t, fn, true)
			address := read.X.(*ssa.FieldAddr)
			got, queryErr := view.FieldRead(ctx, gotoolchain.SSAFieldReadRequest{Read: read})
			if queryErr != nil || got.Base != address.X || got.Field != record.Field(row.wantField) || got.Validate() != nil {
				t.Fatalf("%s field read got=%+v,%v, want base=%v field=%v,nil", row.name, got, queryErr, address.X, record.Field(row.wantField))
			}
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}

type nativeFieldDamage uint8

const (
	nativeFieldMissingAddress nativeFieldDamage = iota
	nativeFieldTypedNilAddress
	nativeFieldDetachedAddress
	nativeFieldForeignAddress
	nativeFieldMissingBase
	nativeFieldTypedNilBase
	nativeFieldDetachedBase
	nativeFieldForeignBase
	nativeFieldNegativeOrdinal
	nativeFieldAtWidthOrdinal
	nativeFieldWrongMemberType
	nativeFieldWrongReadType
	nativeFieldWrongAddressType
	nativeFieldCommaOK
	nativeFieldForwardAddress
)

func TestSSAFieldReadRefusesUnownedOrContradictoryOperands(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name    string
		mode    nativeFieldDamage
		wantErr error
	}{
		{"missing address", nativeFieldMissingAddress, core.ErrGoToolchainOutput},
		{"typed nil address", nativeFieldTypedNilAddress, core.ErrGoToolchainOutput},
		{"detached address", nativeFieldDetachedAddress, core.ErrGoToolchainOutput},
		{"foreign address", nativeFieldForeignAddress, core.ErrGoToolchainOutput},
		{"missing base", nativeFieldMissingBase, core.ErrGoToolchainOutput},
		{"typed nil base", nativeFieldTypedNilBase, core.ErrGoToolchainOutput},
		{"detached base", nativeFieldDetachedBase, core.ErrGoToolchainOutput},
		{"foreign base", nativeFieldForeignBase, core.ErrGoToolchainOutput},
		{"negative ordinal", nativeFieldNegativeOrdinal, core.ErrGoToolchainOutput},
		{"ordinal at width", nativeFieldAtWidthOrdinal, core.ErrGoToolchainOutput},
		{"member type contradiction", nativeFieldWrongMemberType, core.ErrGoToolchainOutput},
		{"read type contradiction", nativeFieldWrongReadType, core.ErrGoToolchainOutput},
		{"address type contradiction", nativeFieldWrongAddressType, core.ErrGoToolchainOutput},
		{"dereference cannot be comma ok", nativeFieldCommaOK, core.ErrGoToolchainOutput},
		{"address after read", nativeFieldForwardAddress, core.ErrGoToolchainOutput},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeFieldReadSource)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				fn := view.Package.Func("Entry")
				if row.mode == nativeFieldWrongReadType || row.mode == nativeFieldWrongAddressType {
					fn = view.Package.Func("Mixed")
				}
				read := nativeFieldReadInstruction(t, fn, true)
				address := read.X.(*ssa.FieldAddr)
				before, queryErr := view.FieldRead(ctx, gotoolchain.SSAFieldReadRequest{Read: read})
				if queryErr != nil || before.Base != fn.Params[0] || before.Field == nil || before.Field.Name() != "A" {
					t.Fatalf("field precondition got=%+v,%v, want actual parameter and A,nil", before, queryErr)
				}
				switch row.mode {
				case nativeFieldMissingAddress:
					read.X = nil
				case nativeFieldTypedNilAddress:
					read.X = (*ssa.FieldAddr)(nil)
				case nativeFieldDetachedAddress:
					copied := *address
					read.X = &copied
				case nativeFieldForeignAddress:
					read.X = nativeFieldReadInstruction(t, view.Package.Func("Other"), true).X
				case nativeFieldMissingBase:
					address.X = nil
				case nativeFieldTypedNilBase:
					address.X = (*ssa.Parameter)(nil)
				case nativeFieldDetachedBase:
					copied := *fn.Params[0]
					address.X = &copied
				case nativeFieldForeignBase:
					address.X = view.Package.Func("Other").Params[0]
				case nativeFieldNegativeOrdinal:
					address.Field = -1
				case nativeFieldAtWidthOrdinal:
					address.Field = 3
				case nativeFieldWrongMemberType:
					address.Field = 2
				case nativeFieldWrongReadType, nativeFieldWrongAddressType:
					var donor *ssa.UnOp
					for _, block := range fn.Blocks {
						for _, instruction := range block.Instrs {
							if load, ok := instruction.(*ssa.UnOp); ok {
								if field, ok := load.X.(*ssa.FieldAddr); ok && field.Field == 2 {
									donor = load
								}
							}
						}
					}
					if donor == nil || donor.Block() != read.Block() {
						t.Fatalf("type donor got=%v, want same-block boolean field read", donor)
					}
					if row.mode == nativeFieldWrongReadType {
						copied := *donor
						copied.X = read.X
						*read = copied
					} else {
						copied := *donor.X.(*ssa.FieldAddr)
						copied.X = address.X
						copied.Field = address.Field
						*address = copied
					}
				case nativeFieldCommaOK:
					read.CommaOk = true
				case nativeFieldForwardAddress:
					block := read.Block()
					left, right := -1, -1
					for index, current := range block.Instrs {
						if current == address {
							left = index
						}
						if current == read {
							right = index
						}
					}
					if left < 0 || right <= left {
						t.Fatalf("operand positions got=%d,%d, want address before read", left, right)
					}
					block.Instrs[left], block.Instrs[right] = block.Instrs[right], block.Instrs[left]
				default:
					t.Fatalf("field damage got=%d, want named contract boundary", row.mode)
				}
				got, queryErr := view.FieldRead(ctx, gotoolchain.SSAFieldReadRequest{Read: read})
				if got != (gotoolchain.SSAFieldReadObservation{}) || !errors.Is(queryErr, row.wantErr) {
					t.Fatalf("damaged field read got=%+v,%v, want zero,%v", got, queryErr, row.wantErr)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}

func TestSSAFieldReadProducerLayerTriad(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeFieldReadSource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		read := nativeFieldReadInstruction(t, view.Package.Func("Entry"), true)
		got, queryErr := view.FieldRead(ctx, gotoolchain.SSAFieldReadRequest{Read: read})
		if queryErr != nil || got.Base != view.Package.Func("Entry").Params[0] || got.Field == nil || got.Field.Name() != "A" {
			t.Fatalf("positive field got=%+v,%v, want native parameter and A,nil", got, queryErr)
		}
		address := read.X.(*ssa.FieldAddr)
		saved := address.Field
		address.Field = -1
		got, queryErr = view.FieldRead(ctx, gotoolchain.SSAFieldReadRequest{Read: read})
		address.Field = saved
		if got != (gotoolchain.SSAFieldReadObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
			t.Fatalf("negative field got=%+v,%v, want zero,%v", got, queryErr, core.ErrGoToolchainOutput)
		}
		for _, row := range []struct {
			name   string
			field  bool
			wantOp token.Token
		}{{"Plain", false, token.MUL}, {"Not", false, token.NOT}, {"Untaken", true, token.MUL}} {
			read := nativeFieldReadInstruction(t, view.Package.Func(row.name), row.field)
			if read.Op != row.wantOp {
				t.Fatalf("%s unary got=%v, want %v", row.name, read.Op, row.wantOp)
			}
			got, queryErr := view.FieldRead(ctx, gotoolchain.SSAFieldReadRequest{Read: read})
			if got != (gotoolchain.SSAFieldReadObservation{}) || queryErr != nil {
				t.Fatalf("%s neutral got=%+v,%v, want zero,nil", row.name, got, queryErr)
			}
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}
