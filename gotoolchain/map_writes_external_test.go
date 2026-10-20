package gotoolchain_test

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestMapWritesNativeOperandAndInstructionConservation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source           string
		nilWrites, otherWrites int
	}{
		{name: "local zero map is native nil constant", source: `func F(){var values map[string]int;values["key"]=1}`, nilWrites: 1},
		{name: "native alias retains constant", source: `type bucket=map[string]int;func F(){var values bucket;values["key"]=1}`, nilWrites: 1},
		{name: "native defined map retains constant", source: `type bucket map[string]int;func F(){var values bucket;values["key"]=1}`, nilWrites: 1},
		{name: "explicit nil initializer is constant", source: `func F(){var values map[string]int=nil;values["key"]=1}`, nilWrites: 1},
		{name: "converted nil is constant", source: `func F(){values:=map[string]int(nil);values["key"]=1}`, nilWrites: 1},
		{name: "native make operand is not nil constant", source: `func F(){values:=make(map[string]int);values["key"]=1}`, otherWrites: 1},
		{name: "native map literal is not nil constant", source: `func F(){values:=map[string]int{};values["key"]=1}`, otherWrites: 1},
		{name: "literal entries retain their own instructions", source: `func F(){values:=map[string]int{"first":1};values["key"]=1}`, otherWrites: 2},
		{name: "assignment before write removes constant through Go SSA", source: `func F(){var values map[string]int;values=make(map[string]int);values["key"]=1}`, otherWrites: 1},
		{name: "later nil assignment is actual constant", source: `func F(){values:=make(map[string]int);values=nil;values["key"]=1}`, nilWrites: 1},
		{name: "shadowed name retains independent native operand", source: `var values map[string]int;func F(){values:=map[string]int{};values["key"]=1}`, otherWrites: 1},
		{name: "unrelated initialized shadow cannot initialize outer", source: `func F(){var values map[string]int;{values:=make(map[string]int);values["inner"]=1};values["key"]=1}`, nilWrites: 1, otherWrites: 1},
		{name: "read is not a map update", source: `func F(){var values map[string]int;_=values["key"]}`},
		{name: "delete is not a map update", source: `func F(){var values map[string]int;delete(values,"key")}`},
		{name: "increment has a native map update", source: `func F(){var values map[string]int;values["key"]++}`, nilWrites: 1},
		{name: "compound assignment has one native update", source: `func F(){var values map[string]int;values["key"]+=1}`, nilWrites: 1},
		{name: "multiple targets retain both updates", source: `func F(){var values map[string]int;values["a"],values["b"]=1,2}`, nilWrites: 2},
		{name: "multiple declarations retain both operands", source: `func F(){var a,b map[string]int;a["key"]=1;b["key"]=2}`, nilWrites: 2},
		{name: "function parameter is unknown rather than certified nonnil", source: `func F(values map[string]int){values["key"]=1}`, otherWrites: 1},
		{name: "global load is unknown at arbitrary function entry", source: `var values map[string]int;func F(){values["key"]=1}`, otherWrites: 1},
		{name: "merged nil and initialized paths remain unknown", source: `func F(condition bool){var values map[string]int;if condition{values=make(map[string]int)};values["key"]=1}`, otherWrites: 1},
		{name: "local zero map in anonymous function", source: `func F(){func(){var values map[string]int;values["key"]=1}()}`, nilWrites: 1},
		{name: "local zero map in source initializer", source: `func init(){var values map[string]int;values["key"]=1}`, nilWrites: 1},
		{name: "package initializer anonymous function belongs to native init", source: `var created=func()int{var values map[string]int;values["key"]=1;return 0}()`, nilWrites: 1},
		{name: "generic map value retains native typed nil", source: `func F[T any](){var values map[string]T;var value T;values["key"]=value}`, nilWrites: 1},
		{name: "slice update is not a map update", source: `func F(values []int){values[0]=1}`},
		{name: "array update is not a map update", source: `func F(){var values [1]int;values[0]=1}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample\n"+tc.source)
			nilWrites, otherWrites := 0, 0
			for got, err := range analysis.MapWrites(t.Context()) {
				if err != nil || got.Validate() != nil {
					t.Fatalf("native map update=(%+v,%v)", got, err)
				}
				switch got.Origin {
				case gotoolchain.MapValueOriginNilConstant:
					nilWrites++
				case gotoolchain.MapValueOriginOther:
					otherWrites++
				default:
					t.Fatalf("unknown native operand%v", got.Origin)
				}
			}
			if nilWrites != tc.nilWrites || otherWrites != tc.otherWrites {
				t.Fatalf("native nil/other updates%d/%d,want%d/%d", nilWrites, otherWrites, tc.nilWrites, tc.otherWrites)
			}
		})
	}
}

func TestMapWritesNativePhysicalSource(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	source := "package sample\n//line forged.go:900\nfunc F(){var values map[string]int;values[\"key\"]=1}"
	analysis := nativeSymbolSourceAnalysis(t, directory, source)
	seen := 0
	for got, err := range analysis.MapWrites(t.Context()) {
		if err != nil || got.File.String() != directory+"/source.go" || got.Line != 3 || got.Column != 42 || got.Offset >= uint64(len(source)) || source[got.Offset] != '[' || got.Origin != gotoolchain.MapValueOriginNilConstant {
			t.Fatalf("physical native update=(%+v,%v)", got, err)
		}
		seen++
	}
	if seen != 1 {
		t.Fatalf("native source updates%d,want1", seen)
	}
}

func TestMapWritesNativeStopCancellationAndRefusals(t *testing.T) {
	t.Parallel()
	for _, stop := range []bool{true, false} {
		t.Run(strconv.FormatBool(stop), func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample\nfunc F(){var values map[string]int;values[\"a\"]=1;values[\"b\"]=2}")
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			seen, refused := 0, 0
			analysis.MapWrites(ctx)(func(got gotoolchain.MapWriteObservation, err error) bool {
				if err != nil {
					if stop || got != (gotoolchain.MapWriteObservation{}) || !errors.Is(err, context.Canceled) || !errors.Is(err, io.ErrClosedPipe) {
						t.Fatalf("native map refusal=(%+v,%v)", got, err)
					}
					refused++
					return true
				}
				seen++
				if stop {
					return false
				}
				cancel(io.ErrClosedPipe)
				return true
			})
			want := 1
			if stop {
				want = 0
			}
			if seen != 1 || refused != want {
				t.Fatalf("native map loans/refusals%d/%d,want1/%d", seen, refused, want)
			}
		})
	}
	for _, ctx := range []context.Context{nil, t.Context()} {
		seen := 0
		for got, err := range (gotoolchain.PackageAnalysis{}).MapWrites(ctx) {
			want := error(core.ErrGoToolchainContract)
			if ctx == nil {
				want = core.ErrNilContext
			}
			if got != (gotoolchain.MapWriteObservation{}) || !errors.Is(err, want) {
				t.Fatalf("missing native map analysis=(%+v,%v)", got, err)
			}
			seen++
		}
		if seen != 1 {
			t.Fatalf("native map refusals%d,want1", seen)
		}
	}
	analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample\nfunc F(){}")
	analysis.Incomplete = true
	seen := 0
	for got, err := range analysis.MapWrites(t.Context()) {
		if got != (gotoolchain.MapWriteObservation{}) || !errors.Is(err, core.ErrGoToolchainOutput) {
			t.Fatalf("incomplete native map analysis=(%+v,%v)", got, err)
		}
		seen++
	}
	if seen != 1 {
		t.Fatalf("incomplete map refusals%d,want1", seen)
	}
}

func TestMapWriteObservationClosedContracts(t *testing.T) {
	t.Parallel()
	file, err := core.ParseAbsolutePath("/source.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, origin := range []gotoolchain.MapValueOrigin{gotoolchain.MapValueOriginNilConstant, gotoolchain.MapValueOriginOther} {
		if origin.Validate() != nil || !origin.IsValid() || origin.String() == core.UnknownEnumDiagnostic {
			t.Fatalf("valid native origin%v", origin)
		}
		origin.OffWireEnum()
		got := gotoolchain.MapWriteObservation{File: file, Origin: origin, Line: 1, Column: 1}
		if err := got.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, origin := range []gotoolchain.MapValueOrigin{gotoolchain.MapValueOriginUnknown, 255} {
		if origin.IsValid() || !errors.Is(origin.Validate(), core.ErrGoToolchainContract) || origin.String() != core.UnknownEnumDiagnostic {
			t.Fatalf("invalid origin%v accepted", origin)
		}
	}
	for _, got := range []gotoolchain.MapWriteObservation{{}, {File: file, Origin: gotoolchain.MapValueOriginNilConstant, Column: 1}, {File: file, Origin: gotoolchain.MapValueOriginNilConstant, Line: 1}, {Origin: gotoolchain.MapValueOriginNilConstant, Line: 1, Column: 1}, {File: file, Line: 1, Column: 1}} {
		if !errors.Is(got.Validate(), core.ErrGoToolchainContract) {
			t.Fatalf("invalid native map observation%+v accepted", got)
		}
	}
}

func FuzzMapWritesConserveNativeConstantAndMultiplicity(f *testing.F) {
	for mode := uint8(0); mode < 4; mode++ {
		f.Add(mode, uint8(1))
	}
	f.Fuzz(func(t *testing.T, mode, count uint8) {
		population := int(count%5) + 1
		prefix, wantNil := "var values map[string]int;", population
		switch mode % 4 {
		case 0:
		case 1:
			prefix, wantNil = "values:=make(map[string]int);", 0
		case 2:
			prefix = "type bucket=map[string]int;var values bucket;"
		case 3:
			prefix, wantNil = "values:=map[string]int{};", 0
		}
		analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample\nfunc F(){"+prefix+strings.Repeat("values[\"key\"]=1;", population)+"}")
		seen, nilWrites := 0, 0
		for got, err := range analysis.MapWrites(t.Context()) {
			if err != nil || got.Validate() != nil {
				t.Fatalf("native map fuzz=(%+v,%v)", got, err)
			}
			seen++
			if got.Origin == gotoolchain.MapValueOriginNilConstant {
				nilWrites++
			}
		}
		if seen != population || nilWrites != wantNil {
			t.Fatalf("native map mode%d total/nil%d/%d,want%d/%d", mode%4, seen, nilWrites, population, wantNil)
		}
	})
}
