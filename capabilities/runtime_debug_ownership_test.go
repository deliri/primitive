package capabilities_test

import (
	"runtime/debug"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/capabilities"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
)

// Native function references make each positive row a real exported Go
// operation. The qualifier/receiver mutations exercise independent coordinates
// of ownership; membership in a package must never imply an invented effect.
func TestRuntimeDebugOwnershipUsesPrimitiveHostAndCompositeFileOwners(t *testing.T) {
	t.Parallel()
	operations := []struct {
		native      any
		name        string
		disposition capabilities.StandardSymbolDisposition
		secondary   []capabilities.Effect
	}{
		{native: debug.FreeOSMemory, name: "FreeOSMemory"},
		{native: debug.PrintStack, name: "PrintStack", secondary: []capabilities.Effect{capabilities.EffectFilesystem}},
		{native: debug.ReadBuildInfo, name: "ReadBuildInfo"},
		{native: debug.ReadGCStats, name: "ReadGCStats"},
		{native: debug.SetCrashOutput, name: "SetCrashOutput", secondary: []capabilities.Effect{capabilities.EffectFilesystem}},
		{native: debug.SetGCPercent, name: "SetGCPercent"},
		{native: debug.SetMaxStack, name: "SetMaxStack"},
		{native: debug.SetMaxThreads, name: "SetMaxThreads"},
		{native: debug.SetMemoryLimit, name: "SetMemoryLimit"},
		{native: debug.SetPanicOnFault, name: "SetPanicOnFault"},
		{native: debug.SetTraceback, name: "SetTraceback"},
		{native: debug.Stack, name: "Stack"},
		{native: debug.WriteHeapDump, name: "WriteHeapDump", secondary: []capabilities.Effect{capabilities.EffectFilesystem}},
		{native: debug.ParseBuildInfo, name: "ParseBuildInfo", disposition: capabilities.StandardSymbolPure},
		{name: "FutureHostObservation", disposition: capabilities.StandardSymbolUnresolved},
	}
	geometry := []struct {
		name, imported, receiver, suffix string
		qualified                        bool
	}{
		{name: "exact Go function", imported: "runtime/debug", qualified: true},
		{name: "same selector in unrelated package", imported: "example.com/debug"},
		{name: "same spelling on Go BuildInfo receiver", imported: "runtime/debug", receiver: "BuildInfo"},
		{name: "unlisted suffixed selector", imported: "runtime/debug", suffix: "Future"},
	}
	catalog, err := capabilities.All()
	if err != nil {
		t.Fatalf("All() = %v, want nil", err)
	}
	for _, operation := range operations {
		for _, shape := range geometry {
			t.Run(operation.name+"/"+shape.name, func(t *testing.T) {
				t.Parallel()
				path, err := gomodule.ParseImportPath(shape.imported)
				if err != nil {
					t.Fatal(err)
				}
				name, err := capabilities.ParseSymbolName(operation.name + shape.suffix)
				if err != nil {
					t.Fatal(err)
				}
				symbol := capabilities.StandardSymbol{ImportPath: path, Selector: name}
				if shape.receiver != "" {
					receiver, err := capabilities.ParseSymbolName(shape.receiver)
					if err != nil {
						t.Fatal(err)
					}
					symbol.Receiver = &receiver
				}
				fact, err := capabilities.ResolveStandardSymbol(symbol)
				if err != nil {
					t.Fatalf("ResolveStandardSymbol(%v) = %v, want nil", symbol, err)
				}
				want := capabilities.StandardSymbolUnresolved
				if shape.qualified {
					want = operation.disposition
					if want == capabilities.StandardSymbolUnknown {
						want = capabilities.StandardSymbolEffect
					}
				}
				if fact.Disposition != want || fact.Symbol.ImportPath != path || fact.Symbol.Selector != name {
					t.Fatalf("catalog fact = %+v, want exact input and disposition %v", fact, want)
				}
				if err := fact.Validate(); err != nil {
					t.Fatalf("fact.Validate() = %v, want nil", err)
				}
				if fact.Operation != capabilities.OperationUnavailable {
					t.Fatalf("replacement = %v, want unavailable rather than invented API", fact.Operation)
				}
				if want != capabilities.StandardSymbolEffect {
					if fact.Effect != capabilities.EffectUnknown || len(fact.Secondary) != 0 {
						t.Fatalf("non-effect fact invented ownership: %+v", fact)
					}
					return
				}
				if operation.native == nil || fact.Effect != capabilities.EffectHost || !slices.Equal(fact.Secondary, operation.secondary) {
					t.Fatalf("ownership = %+v, want host and secondary %v for a native Go export", fact, operation.secondary)
				}
				owner, err := catalog.Resolve(capabilities.ForEffect(capabilities.ScopeProduction, fact.Effect))
				if err != nil || owner.Capability.Package != core.PackageHostFacts {
					t.Fatalf("host owner = (%+v,%v), want hostfacts", owner, err)
				}
				for _, effect := range fact.Secondary {
					owner, err := catalog.Resolve(capabilities.ForEffect(capabilities.ScopeProduction, effect))
					if err != nil || owner.Capability.Package != core.PackageFilestore {
						t.Fatalf("file owner = (%+v,%v), want filestore", owner, err)
					}
				}
			})
		}
	}
}
