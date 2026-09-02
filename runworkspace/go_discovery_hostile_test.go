package runworkspace

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/standard"
)

func TestParseGoDeclarationsHostileBoundaries(t *testing.T) {
	t.Parallel()

	allKinds := []standard.ProbeKind{standard.ProbeKindGoTest, standard.ProbeKindGoRace, standard.ProbeKindGoBenchmark, standard.ProbeKindGoFuzz, standard.ProbeKindGoDiagnosticProfile}
	goTest := []standard.ProbeKind{standard.ProbeKindGoTest}
	benchmark := []standard.ProbeKind{standard.ProbeKindGoBenchmark}
	fuzz := []standard.ProbeKind{standard.ProbeKindGoFuzz}
	cases := []struct {
		wantErr error
		setup   func() []byte
		name    string
		kinds   []standard.ProbeKind
		want    []string
	}{
		{name: "valid uppercase test declaration is admitted", setup: sourceFixture("func TestAlpha(t *testing.T) {}"), kinds: goTest, want: []string{"go_test:TestAlpha"}},
		{name: "valid numeric test suffix is admitted", setup: sourceFixture("func Test1(t *testing.T) {}"), kinds: goTest, want: []string{"go_test:Test1"}},
		{name: "valid underscore test suffix is admitted", setup: sourceFixture("func Test_boundary(t *testing.T) {}"), kinds: goTest, want: []string{"go_test:Test_boundary"}},
		{name: "valid benchmark declaration is admitted", setup: sourceFixture("func BenchmarkEncode(b *testing.B) {}"), kinds: benchmark, want: []string{"go_benchmark:BenchmarkEncode"}},
		{name: "valid fuzz declaration is admitted", setup: sourceFixture("func FuzzDecode(f *testing.F) {}"), kinds: fuzz, want: []string{"go_fuzz:FuzzDecode"}},
		{name: "valid example with output oracle is admitted", setup: sourceFixture("func ExampleEncode() {\n// Output: encoded\n}"), kinds: goTest, want: []string{"go_test:ExampleEncode"}},
		{name: "valid example with unordered output oracle is admitted", setup: sourceFixture("func ExampleMap() {\n// Unordered output: values\n}"), kinds: goTest, want: []string{"go_test:ExampleMap"}},
		{name: "valid race projection creates distinct child kind", setup: sourceFixture("func TestRace(t *testing.T) {}"), kinds: []standard.ProbeKind{standard.ProbeKindGoRace}, want: []string{"go_race:TestRace"}},
		{name: "valid diagnostic projection creates distinct child kind", setup: sourceFixture("func TestProfile(t *testing.T) {}"), kinds: []standard.ProbeKind{standard.ProbeKindGoDiagnosticProfile}, want: []string{"go_diagnostic_profile:TestProfile"}},
		{name: "valid mixed declarations are sorted by kind then symbol", setup: sourceFixture("func BenchmarkZ(b *testing.B) {}\nfunc TestZ(t *testing.T) {}\nfunc TestA(t *testing.T) {}"), kinds: []standard.ProbeKind{standard.ProbeKindGoTest, standard.ProbeKindGoBenchmark}, want: []string{"go_test:TestA", "go_test:TestZ", "go_benchmark:BenchmarkZ"}},

		{name: "reject empty external source", setup: func() []byte { return nil }, kinds: goTest, wantErr: core.ErrPrimitiveContract},
		{name: "reject invalid UTF8 source", setup: func() []byte { return []byte("package p\n\xff") }, kinds: goTest, wantErr: core.ErrPrimitiveContract},
		{name: "reject truncated function body", setup: sourceFixture("func TestBroken(t *testing.T) {"), kinds: goTest, wantErr: core.ErrPrimitiveContract},
		{name: "reject missing package clause", setup: func() []byte { return []byte("func TestNoPackage(t *testing.T) {}") }, kinds: goTest, wantErr: core.ErrPrimitiveContract},
		{name: "reject absent requested kind set", setup: sourceFixture("func TestAlpha(t *testing.T) {}"), kinds: nil, wantErr: core.ErrPrimitiveContract},
		{name: "reject duplicate requested kinds", setup: sourceFixture("func TestAlpha(t *testing.T) {}"), kinds: []standard.ProbeKind{standard.ProbeKindGoTest, standard.ProbeKindGoTest}, wantErr: core.ErrPrimitiveContract},
		{name: "reject reversed requested kinds", setup: sourceFixture("func TestAlpha(t *testing.T) {}"), kinds: []standard.ProbeKind{standard.ProbeKindGoBenchmark, standard.ProbeKindGoTest}, wantErr: core.ErrPrimitiveContract},
		{name: "reject unknown future requested kind", setup: sourceFixture("func TestAlpha(t *testing.T) {}"), kinds: []standard.ProbeKind{standard.ProbeKind(255)}, wantErr: core.ErrPrimitiveContract},
		{name: "reject selection kind as a child", setup: sourceFixture("func TestAlpha(t *testing.T) {}"), kinds: []standard.ProbeKind{standard.ProbeKindGoFileSelection}, wantErr: core.ErrPrimitiveContract},
		{name: "reject JavaScript kind at Go discovery boundary", setup: sourceFixture("func TestAlpha(t *testing.T) {}"), kinds: []standard.ProbeKind{standard.ProbeKindJavaScriptTest}, wantErr: core.ErrPrimitiveContract},

		{name: "boundary package with no executable declaration is neutral", setup: sourceFixture("var Value = 1"), kinds: allKinds, want: []string{}},
		{name: "boundary exact Test prefix is admitted", setup: sourceFixture("func Test(t *testing.T) {}"), kinds: goTest, want: []string{"go_test:Test"}},
		{name: "boundary lowercase test suffix is excluded", setup: sourceFixture("func Testhelper(t *testing.T) {}"), kinds: goTest, want: []string{}},
		{name: "boundary exact Benchmark prefix is admitted", setup: sourceFixture("func Benchmark(b *testing.B) {}"), kinds: benchmark, want: []string{"go_benchmark:Benchmark"}},
		{name: "boundary lowercase benchmark suffix is excluded", setup: sourceFixture("func Benchmarkhelper(b *testing.B) {}"), kinds: benchmark, want: []string{}},
		{name: "boundary exact Fuzz prefix is admitted", setup: sourceFixture("func Fuzz(f *testing.F) {}"), kinds: fuzz, want: []string{"go_fuzz:Fuzz"}},
		{name: "boundary lowercase fuzz suffix is excluded", setup: sourceFixture("func Fuzzhelper(f *testing.F) {}"), kinds: fuzz, want: []string{}},
		{name: "boundary example without output oracle is excluded", setup: sourceFixture("func ExampleNoOracle() {}"), kinds: goTest, want: []string{}},
		{name: "boundary output comment before example body is excluded", setup: sourceFixture("// Output: not-owned\nfunc ExampleWrongPlace() {}"), kinds: goTest, want: []string{}},
		{name: "boundary test without parameter is excluded", setup: sourceFixture("func TestNoParameter() {}"), kinds: goTest, want: []string{}},
		{name: "boundary test value parameter is excluded", setup: sourceFixture("func TestValue(t testing.T) {}"), kinds: goTest, want: []string{}},
		{name: "boundary test foreign package parameter is excluded", setup: sourceFixture("func TestForeign(t *other.T) {}"), kinds: goTest, want: []string{}},
		{name: "boundary test with two parameters is excluded", setup: sourceFixture("func TestTwo(t *testing.T, n int) {}"), kinds: goTest, want: []string{}},
		{name: "boundary test with result is excluded", setup: sourceFixture("func TestResult(t *testing.T) error { return nil }"), kinds: goTest, want: []string{}},
		{name: "boundary generic test is excluded", setup: sourceFixture("func TestGeneric[T any](t *testing.T) {}"), kinds: goTest, want: []string{}},
		{name: "boundary method named as test is excluded", setup: sourceFixture("type S struct{}\nfunc (S) TestMethod(t *testing.T) {}"), kinds: goTest, want: []string{}},
		{name: "boundary one below byte ceiling remains admitted", setup: boundedSourceFixture(GoDiscoverySourceMaximumBytes - 1), kinds: goTest, want: []string{"go_test:TestCeiling"}},
		{name: "boundary exact byte ceiling remains admitted", setup: boundedSourceFixture(GoDiscoverySourceMaximumBytes), kinds: goTest, want: []string{"go_test:TestCeiling"}},
		{name: "boundary one above byte ceiling is refused before parsing", setup: boundedSourceFixture(GoDiscoverySourceMaximumBytes + 1), kinds: goTest, wantErr: core.ErrPrimitiveContract},
		{name: "boundary ordinary request excludes race-only child", setup: sourceFixture("func TestRaceOnly(t *testing.T) {}"), kinds: goTest, want: []string{"go_test:TestRaceOnly"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := ParseGoDeclarations(tc.setup(), tc.kinds)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("ParseGoDeclarations() error = %v, want errors.Is(..., %v)", gotErr, tc.wantErr)
			}
			if gotErr != nil {
				return
			}
			gotLabels := declarationLabels(got)
			if !slices.Equal(gotLabels, tc.want) {
				t.Fatalf("ParseGoDeclarations() = %v, want %v", gotLabels, tc.want)
			}
		})
	}
}

func FuzzParseGoDeclarationsSemanticClosure(f *testing.F) {
	f.Add([]byte("package p\nimport \"testing\"\nfunc TestSeed(t *testing.T) {}"))
	f.Add([]byte("package p\nfunc ExampleSeed() {\n// Output: seed\n}"))
	f.Add([]byte("package p"))
	f.Add([]byte{})
	kinds := []standard.ProbeKind{standard.ProbeKindGoTest, standard.ProbeKindGoRace, standard.ProbeKindGoBenchmark, standard.ProbeKindGoFuzz, standard.ProbeKindGoDiagnosticProfile}
	f.Fuzz(func(t *testing.T, source []byte) {
		got, gotErr := ParseGoDeclarations(source, kinds)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrPrimitiveContract) {
				t.Fatalf("ParseGoDeclarations(rejected) error = %v, want errors.Is(..., %v)", gotErr, core.ErrPrimitiveContract)
			}
			return
		}
		for index := range got {
			if err := got[index].Validate(); err != nil {
				t.Fatalf("ParseGoDeclarations(accepted)[%d].Validate() error = %v, want nil", index, err)
			}
			if index > 0 && !goDeclarationLess(got[index-1], got[index]) {
				t.Fatalf("ParseGoDeclarations(accepted) order at %d = %v then %v, want strict canonical order", index, got[index-1], got[index])
			}
		}
		second, secondErr := ParseGoDeclarations(source, kinds)
		if secondErr != nil || !slices.Equal(second, got) {
			t.Fatalf("ParseGoDeclarations(same accepted bytes) = (%v, %v), want deterministic %v and nil", second, secondErr, got)
		}
	})
}

func sourceFixture(body string) func() []byte {
	return func() []byte { return []byte("package p\nimport \"testing\"\n" + body) }
}

func boundedSourceFixture(size int) func() []byte {
	return func() []byte {
		prefix := "package p\nimport \"testing\"\nfunc TestCeiling(t *testing.T) {}\n/*"
		suffix := "*/"
		return []byte(prefix + strings.Repeat("x", size-len(prefix)-len(suffix)) + suffix)
	}
}

func declarationLabels(declarations []GoDeclaration) []string {
	labels := make([]string, len(declarations))
	for index := range declarations {
		labels[index] = declarations[index].Kind.String() + ":" + declarations[index].Symbol.String()
	}
	return labels
}
