package capabilities

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
)

func TestOwnershipRejectsForgedCapabilityFields(t *testing.T) {
	t.Parallel()
	for contract := range core.PrimitiveArchitecture().Packages() {
		original := Capability{Package: contract.Identity, Kind: contract.Kind, Role: contract.Role}
		for effect := EffectFilesystem; effect < effectLimit; effect++ {
			if !original.Owns(effect) {
				continue
			}
			t.Run(contract.Identity.String()+" kind and role byte domains", func(t *testing.T) {
				t.Parallel()
				for raw := range 256 {
					kind := original
					kind.Kind = core.PackageKind(raw)
					role := original
					role.Role = core.PackageRole(raw)
					for _, candidate := range []Capability{kind, role} {
						want := candidate == original
						if got := candidate.Owns(effect); got != want {
							t.Errorf("Owns(%+v,%v) = %t, want %t", candidate, effect, got, want)
						}
						if !want {
							path, err := candidate.ImportPath()
							if path != "" || !errors.Is(err, core.ErrCapabilitiesContract) {
								t.Fatalf("forged import path = (%q,%v), want empty typed refusal", path, err)
							}
						}
					}
				}
			})
		}
	}
}

func TestReplacementRejectsContradictoryEffect(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, path, receiver, selector string
		want                           Operation
		owner                          Effect
	}{
		{"file read", "os", "", symbolReadFile, OperationReadFile, EffectFilesystem},
		{"file write", "os", "", symbolWriteFile, OperationWriteFile, EffectFilesystem},
		{"clock observation", timeContractText, "", "Now", OperationObserveTime, EffectTime},
		{"command execution", catalogOsExec, "Cmd", "Run", OperationRunProcess, EffectProcess},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path, err := gomodule.ParseImportPath(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			selector, err := ParseSymbolName(tc.selector)
			if err != nil {
				t.Fatal(err)
			}
			symbol := StandardSymbol{ImportPath: path, Selector: selector}
			if tc.receiver != "" {
				receiver, err := ParseSymbolName(tc.receiver)
				if err != nil {
					t.Fatal(err)
				}
				symbol.Receiver = &receiver
			}
			for _, retained := range operationDomain() {
				fact := StandardSymbolFact{Symbol: symbol, Classification: Classification{Disposition: StandardSymbolEffect, Effect: tc.owner, Operation: retained}}
				got, err := fact.Replacement()
				if retained == OperationUnavailable || retained == tc.want {
					if err != nil || got != tc.want {
						t.Fatalf("retained %v replacement = (%v,%v), want %v", retained, got, err, tc.want)
					}
					continue
				}
				if !errors.Is(err, core.ErrCapabilitiesContract) || got != OperationUnavailable {
					t.Fatalf("contradictory retained operation %v = (%v,%v)", retained, got, err)
				}
			}
			for effect := EffectFilesystem; effect < effectLimit; effect++ {
				fact := StandardSymbolFact{Symbol: symbol, Classification: Classification{Disposition: StandardSymbolEffect, Effect: effect}}
				got, err := fact.Replacement()
				if effect == tc.owner {
					if err != nil || got != tc.want {
						t.Fatalf("replacement = (%v,%v), want %v", got, err, tc.want)
					}
					continue
				}
				if !errors.Is(err, core.ErrCapabilitiesContract) || got != OperationUnavailable {
					t.Errorf("effect %v replacement = (%v,%v), want unavailable typed contradiction", effect, got, err)
				}
			}
		})
	}
}
