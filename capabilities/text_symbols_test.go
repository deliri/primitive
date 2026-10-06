package capabilities

import (
	"errors"
	"go/importer"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
)

// The selectors are independently reviewed standard-library operations. A
// callback-bearing sibling must never inherit a text package's pure status.
func TestTextSymbolOwnershipBoundaries(t *testing.T) {
	t.Parallel()
	groups := []struct {
		path        string
		disposition StandardSymbolDisposition
		selectors   []string
	}{
		{"strings", StandardSymbolPure, []string{"Clone", "Compare", "Contains", "ContainsAny", "ContainsRune", "Count", "Cut", "CutPrefix", "CutSuffix", "CutLast", "EqualFold", "Fields", "FieldsSeq", "HasPrefix", "HasSuffix", "Index", "IndexAny", "IndexByte", "IndexRune", "Join", "LastIndex", "LastIndexAny", "LastIndexByte", "NewReader", "NewReplacer", "Repeat", "Replace", "ReplaceAll", "Split", "SplitAfter", "SplitAfterN", "SplitAfterSeq", "SplitN", "SplitSeq", "Title", "ToLower", "ToLowerSpecial", "ToTitle", "ToTitleSpecial", "ToUpper", "ToUpperSpecial", "ToValidUTF8", "Trim", "TrimLeft", "TrimPrefix", "TrimRight", "TrimSpace", "TrimSuffix"}},
		{"strings", StandardSymbolContextual, []string{"ContainsFunc", "FieldsFunc", "FieldsFuncSeq", "IndexFunc", "LastIndexFunc", "Map", "TrimFunc", "TrimLeftFunc", "TrimRightFunc"}},
		{"bytes", StandardSymbolPure, []string{"Clone", "Compare", "Contains", "Cut", "Equal", "HasPrefix", "Join", "NewBuffer", "NewBufferString", "NewReader", "Runes", "TrimSpace"}},
		{"bytes", StandardSymbolContextual, []string{"ContainsFunc", "FieldsFunc", "FieldsFuncSeq", "IndexFunc", "LastIndexFunc", "Map", "TrimFunc", "TrimLeftFunc", "TrimRightFunc"}},
		{"strconv", StandardSymbolPure, []string{"AppendBool", "AppendFloat", "AppendInt", "AppendQuote", "AppendUint", "Atoi", "CanBackquote", "FormatBool", "FormatComplex", "FormatFloat", "FormatInt", "FormatUint", "Itoa", "ParseBool", "ParseComplex", "ParseFloat", "ParseInt", "ParseUint", "Quote", "QuotedPrefix", "QuoteRune", "Unquote", "UnquoteChar"}},
		{"strings", StandardSymbolUnresolved, []string{"FutureOperation", "ReadFile", "WriteFile", "Now", "Open"}},
		{"example.com/product/strings", StandardSymbolUnresolved, []string{"Contains", "Map", "TrimSpace"}},
	}
	for _, group := range groups {
		for _, name := range group.selectors {
			t.Run(group.path+"/"+name, func(t *testing.T) {
				t.Parallel()
				path, err := gomodule.ParseImportPath(group.path)
				if err != nil {
					t.Fatal(err)
				}
				selector, err := ParseSymbolName(name)
				if err != nil {
					t.Fatal(err)
				}
				symbol := StandardSymbol{ImportPath: path, Selector: selector}
				got, err := ResolveStandardSymbol(symbol)
				want := Classification{Disposition: group.disposition}
				if err != nil || got.Symbol != symbol || !got.Classification.Equal(want) || got.Validate() != nil {
					t.Fatalf("classification = %+v, %v; want %+v with exact symbol", got, err, want)
				}
			})
		}
	}
}

func FuzzTextSymbolNamespaceAndCallbacks(f *testing.F) {
	for _, suffix := range []string{"", "future", "界", "\x00", "\xff"} {
		f.Add(suffix, false, false, false)
		f.Add(suffix, true, false, false)
		f.Add(suffix, true, true, false)
		f.Add(suffix, false, false, true)
	}
	f.Fuzz(func(t *testing.T, suffix string, callback, foreign, method bool) {
		name := "Contains"
		want := StandardSymbolPure
		if callback {
			name = "ContainsFunc"
			want = StandardSymbolContextual
		}
		if suffix != "" {
			name += "_" + suffix
			want = StandardSymbolUnresolved
		}
		selector, err := ParseSymbolName(name)
		if err != nil {
			if !errors.Is(err, core.ErrCapabilitiesContract) {
				t.Fatal(err)
			}
			return
		}
		packagePath := "strings"
		if foreign {
			packagePath = "example.invalid/strings"
			want = StandardSymbolUnresolved
		}
		path, err := gomodule.ParseImportPath(packagePath)
		if err != nil {
			t.Fatal(err)
		}
		symbol := StandardSymbol{ImportPath: path, Selector: selector}
		if method {
			symbol.Receiver = &selector
			want = StandardSymbolUnresolved
		}
		got, err := ResolveStandardSymbol(symbol)
		if err != nil || got.Symbol != symbol || !got.Classification.Equal(Classification{Disposition: want}) {
			t.Fatalf("symbol %+v = (%+v, %v); want %s", symbol, got, err, want)
		}
	})
}

// The actual toolchain proves the catalog is exhaustive for these reviewed
// packages and independently exposes every callback parameter.
func TestCompilerTextFunctionCatalogCoverage(t *testing.T) {
	t.Parallel()
	for _, importPath := range []string{"strings", "bytes", "strconv"} {
		t.Run(importPath, func(t *testing.T) {
			t.Parallel()
			pkg, err := importer.Default().Import(importPath)
			if err != nil {
				t.Fatal(err)
			}
			path, err := gomodule.ParseImportPath(importPath)
			if err != nil {
				t.Fatal(err)
			}
			observed := 0
			for _, name := range pkg.Scope().Names() {
				function, ok := pkg.Scope().Lookup(name).(*types.Func)
				if !ok || !function.Exported() {
					continue
				}
				observed++
				selector, err := ParseSymbolName(name)
				if err != nil {
					t.Fatal(err)
				}
				want := StandardSymbolPure
				for parameter := range function.Signature().Params().Variables() {
					if _, callback := parameter.Type().Underlying().(*types.Signature); callback {
						want = StandardSymbolContextual
					}
				}
				got, err := ResolveStandardSymbol(StandardSymbol{ImportPath: path, Selector: selector})
				if err != nil || !got.Classification.Equal(Classification{Disposition: want}) {
					t.Errorf("%s.%s classification = %+v, %v; compiler callback contract requires %s", path, selector, got, err, want)
				}
			}
			if observed == 0 {
				t.Fatal("compiler returned no exported functions")
			}
		})
	}
}
