package capabilities

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/gomodule"
)

// Eight fixed requests cover a package function, pure/contextual helpers,
// composite effects, methods, and an unlisted product symbol. Every result is
// observed after the loop; request construction is outside timing.
func BenchmarkMixedStandardSymbols(b *testing.B) {
	cases := []struct {
		path, selector, receiver string
		disposition              StandardSymbolDisposition
		effect                   Effect
	}{
		{path: "os", selector: "Open", disposition: StandardSymbolEffect, effect: EffectFilesystem},
		{path: "fmt", selector: "Sprintf", disposition: StandardSymbolContextual},
		{path: standardPackageBuiltin, selector: "len", disposition: StandardSymbolPure},
		{path: timeContractText, selector: "Now", disposition: StandardSymbolEffect, effect: EffectTime},
		{path: catalogNetHttp, selector: symbolServeFile, disposition: StandardSymbolEffect, effect: EffectTransport},
		{path: "example.com/product", selector: "Run", disposition: StandardSymbolUnresolved},
		{path: "os", receiver: symbolFile, selector: symbolRead, disposition: StandardSymbolEffect, effect: EffectFilesystem},
		{path: "net", receiver: symbolConn, selector: symbolLocalAddr, disposition: StandardSymbolPure},
	}
	requests := make([]StandardSymbol, len(cases))
	results := make([]StandardSymbolFact, len(cases))
	for index, fixture := range cases {
		path, pathErr := gomodule.ParseImportPath(fixture.path)
		selector, selectorErr := ParseSymbolName(fixture.selector)
		if err := errors.Join(pathErr, selectorErr); err != nil {
			b.Fatal(err)
		}
		requests[index] = StandardSymbol{ImportPath: path, Selector: selector}
		if fixture.receiver != "" {
			receiver, err := ParseSymbolName(fixture.receiver)
			if err != nil {
				b.Fatal(err)
			}
			requests[index].Receiver = &receiver
		}
	}
	b.ReportAllocs()
	for b.Loop() {
		for index, request := range requests {
			var err error
			results[index], err = ResolveStandardSymbol(request)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
	for index, result := range results {
		if err := result.Validate(); err != nil || result.Disposition != cases[index].disposition || result.Effect != cases[index].effect {
			b.Fatalf("symbol %d = %+v/%v, want disposition %v/effect %v", index, result, err, cases[index].disposition, cases[index].effect)
		}
	}
}
