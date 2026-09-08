package capabilities

import (
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"testing"
)

func BenchmarkResolveEffect(b *testing.B) {
	request := ForEffect(ScopeProduction, EffectTransport)
	want := Match{Requirement: request, Capability: Capability{Package: core.PackageExchange, Kind: core.PackageKindProduction, Role: core.PackageRoleEffectCapability}}
	var got Match
	b.ReportAllocs()
	for b.Loop() {
		var err error
		got, err = Resolve(request)
		if err != nil {
			b.Fatal(err)
		}
	}
	if got != want {
		b.Fatalf("Resolve = %+v, want %+v", got, want)
	}
}

func BenchmarkResolveStandardFunction(b *testing.B) {
	b.ReportAllocs()
	benchmarkStandardSymbol(b, catalogNetHttp, "", symbolServeFile, Classification{Disposition: StandardSymbolEffect, Effect: EffectTransport, Secondary: []Effect{EffectFilesystem}})
}
func BenchmarkResolveStandardMethod(b *testing.B) {
	b.ReportAllocs()
	benchmarkStandardSymbol(b, catalogOsExec, "Cmd", "Run", Classification{Disposition: StandardSymbolEffect, Effect: EffectProcess, Operation: OperationRunProcess})
}
func BenchmarkResolveStandardUnresolved(b *testing.B) {
	b.ReportAllocs()
	benchmarkStandardSymbol(b, catalogNetHttp, "", "FutureEffect", Classification{Disposition: StandardSymbolUnresolved})
}
func benchmarkStandardSymbol(b *testing.B, path, receiver, selector string, want Classification) {
	b.Helper()
	imported, err := gomodule.ParseImportPath(path)
	if err != nil {
		b.Fatal(err)
	}
	name, err := ParseSymbolName(selector)
	if err != nil {
		b.Fatal(err)
	}
	symbol := StandardSymbol{ImportPath: imported, Selector: name}
	if receiver != "" {
		value, err := ParseSymbolName(receiver)
		if err != nil {
			b.Fatal(err)
		}
		symbol.Receiver = &value
	}
	var got StandardSymbolFact
	b.ReportAllocs()
	for b.Loop() {
		got, err = ResolveStandardSymbol(symbol)
		if err != nil {
			b.Fatal(err)
		}
	}
	if got.Symbol != symbol || !got.Classification.Equal(want) {
		b.Fatalf("resolution = %+v, want symbol %+v and classification %+v", got, symbol, want)
	}
}

func BenchmarkClassificationJSONMaximum(b *testing.B) {
	want := Classification{Disposition: StandardSymbolEffect, Effect: EffectFilesystem, Operation: OperationReadFile}
	for effect := EffectProcess; effect < effectLimit; effect++ {
		want.Secondary = append(want.Secondary, effect)
	}
	data, err := want.MarshalJSON()
	if err != nil {
		b.Fatal(err)
	}
	var got Classification
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		var next Classification
		if err := next.UnmarshalJSON(data); err != nil {
			b.Fatal(err)
		}
		got = next
	}
	if !got.Equal(want) {
		b.Fatalf("decode = %+v, want %+v", got, want)
	}
}
