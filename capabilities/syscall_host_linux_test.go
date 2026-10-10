//go:build linux

package capabilities_test

import (
	"github.com/deliri/primitive/v2026/capabilities"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"syscall"
	"testing"
)

func TestGoStandardKernelCapacityFunctionsUsePrimitiveHostOwner(t *testing.T) {
	t.Parallel()
	functions := []struct {
		name   string
		native any
	}{{"Sysinfo", syscall.Sysinfo}, {"Fstatfs", syscall.Fstatfs}, {"Statfs", syscall.Statfs}}
	catalog, err := capabilities.All()
	if err != nil {
		t.Fatal(err)
	}
	for _, function := range functions {
		for _, shape := range []struct {
			name, path, receiver, suffix string
			effect                       bool
		}{
			{name: "native function", path: "syscall", effect: true}, {name: "unrelated package", path: "example.com/syscall"},
			{name: "same spelling method", path: "syscall", receiver: "Statfs_t"}, {name: "unlisted suffix", path: "syscall", suffix: "Future"},
		} {
			t.Run(function.name+"/"+shape.name, func(t *testing.T) {
				t.Parallel()
				path, err := gomodule.ParseImportPath(shape.path)
				if err != nil {
					t.Fatal(err)
				}
				selector, err := capabilities.ParseSymbolName(function.name + shape.suffix)
				if err != nil {
					t.Fatal(err)
				}
				request := capabilities.StandardSymbol{ImportPath: path, Selector: selector}
				if shape.receiver != "" {
					receiver, err := capabilities.ParseSymbolName(shape.receiver)
					if err != nil {
						t.Fatal(err)
					}
					request.Receiver = &receiver
				}
				got, err := capabilities.ResolveStandardSymbol(request)
				if err != nil {
					t.Fatal(err)
				}
				if !shape.effect {
					if got.Disposition != capabilities.StandardSymbolUnresolved || got.Effect != capabilities.EffectUnknown {
						t.Fatalf("invented ownership: %+v", got)
					}
					return
				}
				if function.native == nil || got.Disposition != capabilities.StandardSymbolEffect || got.Effect != capabilities.EffectHost || len(got.Secondary) != 0 || got.Validate() != nil {
					t.Fatalf("native kernel capacity owner = %+v", got)
				}
				owner, err := catalog.Resolve(capabilities.ForEffect(capabilities.ScopeProduction, got.Effect))
				if err != nil || owner.Capability.Package != core.PackageHostFacts {
					t.Fatalf("owner = %+v/%v, want hostfacts", owner, err)
				}
			})
		}
	}
}
