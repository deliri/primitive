package capabilities

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"go/token"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
)

// Go decodes the source independently; the finite typed domain supplies exact
// admitted values. A decoder that refuses every input or substitutes one valid
// value for another fails this oracle before any round trip can hide the loss.
func jsonEnumOracle[T interface {
	comparable
	String() string
}](data []byte, domain []T) (T, bool) {
	var zero T
	var source string
	if len(data) > core.JSONDocumentMaximumBytes || bytes.Equal(bytes.TrimSpace(data), []byte("null")) || json.Unmarshal(data, &source) != nil {
		return zero, false
	}
	for _, value := range domain {
		if source == value.String() {
			return value, true
		}
	}
	return zero, false
}
func identityDomain() []Identity {
	result := make([]Identity, 0, IdentityCount)
	for effect := EffectFilesystem; effect < effectLimit; effect++ {
		result = append(result, Identity{effect: effect})
	}
	return result
}
func operationDomain() []Operation {
	return []Operation{OperationUnavailable, OperationReadFile, OperationWriteFile, OperationRunProcess, OperationObserveTime}
}
func dispositionDomain() []StandardSymbolDisposition {
	return []StandardSymbolDisposition{StandardSymbolPure, StandardSymbolContextual, StandardSymbolEffect, StandardSymbolUnresolved}
}

func FuzzParseIdentityExactDomain(f *testing.F) {
	for _, value := range identityDomain() {
		f.Add(value.String(), uint8(value.effect))
	}
	f.Add("", uint8(EffectUnknown))
	f.Add("future", uint8(effectLimit))
	f.Add("\xff", uint8(255))
	f.Fuzz(func(t *testing.T, source string, raw uint8) {
		constructed, constructorErr := IdentityForEffect(Effect(raw))
		constructorValid := raw >= uint8(EffectFilesystem) && raw < uint8(effectLimit)
		if constructorValid {
			if constructorErr != nil || constructed.effect != Effect(raw) {
				t.Fatalf("effect identity = (%v,%v), want effect %d", constructed, constructorErr, raw)
			}
		} else if !errors.Is(constructorErr, core.ErrCapabilitiesContract) || constructed != (Identity{}) {
			t.Fatalf("constructor refusal = (%v,%v)", constructed, constructorErr)
		}
		var want Identity
		admitted := false
		for _, candidate := range identityDomain() {
			if source == candidate.String() {
				want = candidate
				admitted = true
				break
			}
		}
		got, err := ParseIdentity(source)
		if admitted {
			if err != nil || got != want {
				t.Fatalf("identity = (%v,%v), want %v", got, err, want)
			}
			return
		}
		if !errors.Is(err, core.ErrCapabilitiesContract) || got != (Identity{}) {
			t.Fatalf("identity refusal = (%v,%v)", got, err)
		}
	})
}

func FuzzSymbolNameGoIdentifier(f *testing.F) {
	for _, value := range []string{symbolReadFile, symbolWrite, "世界", "_x", "x0", "", "_", "for", "0x", "x.y", "x\x00", "\xff"} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, source string) {
		want := token.IsIdentifier(source) && source != "_"
		got, err := ParseSymbolName(source)
		if want {
			if err != nil || got.String() != source {
				t.Fatalf("Go identifier %q = (%q,%v)", source, got.String(), err)
			}
			return
		}
		if !errors.Is(err, core.ErrCapabilitiesContract) || got != (SymbolName{}) {
			t.Fatalf("identifier refusal = (%+v,%v)", got, err)
		}
	})
}

func FuzzClassificationTypedProjection(f *testing.F) {
	for _, disposition := range dispositionDomain() {
		for effect := EffectUnknown; effect < effectLimit; effect++ {
			f.Add(uint8(disposition), uint8(effect), uint8(OperationUnavailable), []byte{})
		}
	}
	f.Add(uint8(StandardSymbolEffect), uint8(EffectTransport), uint8(OperationUnavailable), []byte{byte(EffectFilesystem)})
	f.Add(uint8(StandardSymbolEffect), uint8(EffectFilesystem), uint8(OperationReadFile), []byte{byte(EffectProcess), byte(EffectProcess)})
	f.Fuzz(func(t *testing.T, disposition, primary, operation uint8, secondary []byte) {
		value := Classification{Disposition: StandardSymbolDisposition(disposition), Effect: Effect(primary), Operation: Operation(operation)}
		for _, raw := range secondary {
			value.Secondary = append(value.Secondary, Effect(raw))
		}
		want := classificationAdmitted(value)
		encoded, err := value.MarshalJSON()
		if !want {
			if !errors.Is(err, core.ErrCapabilitiesContract) || len(encoded) != 0 {
				t.Fatalf("invalid projection leaked (%q,%v)", encoded, err)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		var got Classification
		if err := got.UnmarshalJSON(encoded); err != nil || !got.Equal(value) {
			t.Fatalf("projection = (%+v,%v), want %+v", got, err, value)
		}
	})
}

func classificationAdmitted(value Classification) bool {
	if !slices.Contains(dispositionDomain(), value.Disposition) || !slices.Contains(operationDomain(), value.Operation) {
		return false
	}
	if value.Disposition != StandardSymbolEffect {
		return value.Effect == EffectUnknown && len(value.Secondary) == 0 && value.Operation == OperationUnavailable
	}
	if value.Effect < EffectFilesystem || value.Effect >= effectLimit {
		return false
	}
	owners := [operationLimit]Effect{OperationReadFile: EffectFilesystem, OperationWriteFile: EffectFilesystem, OperationRunProcess: EffectProcess, OperationObserveTime: EffectTime}
	if value.Operation != OperationUnavailable && owners[value.Operation] != value.Effect {
		return false
	}
	seen := uint16(1) << value.Effect
	for _, effect := range value.Secondary {
		if effect < EffectFilesystem || effect >= effectLimit || seen&(1<<effect) != 0 {
			return false
		}
		seen |= 1 << effect
	}
	return true
}

func classificationJSONOracle(data []byte) (Classification, bool) {
	if len(data) > ClassificationJSONMaximumBytes {
		return Classification{}, false
	}
	var wire classificationWire
	if json.Unmarshal(data, &wire, json.RejectUnknownMembers(true)) != nil {
		return Classification{}, false
	}
	value := Classification{Disposition: wire.Disposition, Operation: wire.Operation}
	if wire.Effect != nil {
		value.Effect = wire.Effect.effect
	}
	for _, identity := range wire.Secondary {
		value.Secondary = append(value.Secondary, identity.effect)
	}
	return value, classificationAdmitted(value)
}

func FuzzResolveRequirementExactOwnership(f *testing.F) {
	for contract := range core.PrimitiveArchitecture().Packages() {
		f.Add(uint8(ScopeProduction), uint8(RequirementTargetPackage), uint8(contract.Identity), uint8(EffectUnknown))
		f.Add(uint8(ScopeTest), uint8(RequirementTargetPackage), uint8(contract.Identity), uint8(EffectUnknown))
	}
	for effect := EffectUnknown; effect < effectLimit; effect++ {
		f.Add(uint8(ScopeProduction), uint8(RequirementTargetEffect), uint8(core.PackageUnknown), uint8(effect))
	}
	f.Add(uint8(255), uint8(255), uint8(255), uint8(255))
	f.Fuzz(func(t *testing.T, scope, target, pkg, effect uint8) {
		request := Requirement{Scope: Scope(scope), Target: RequirementTarget(target), Package: core.PackageIdentity(pkg), Effect: Effect(effect)}
		want, wantErr := requirementOracle(request)
		got, err := Resolve(request)
		if !errors.Is(err, wantErr) || got != want {
			t.Fatalf("requirement %+v = (%+v,%v), want (%+v,%v)", request, got, err, want, wantErr)
		}
	})
}

func requirementOracle(request Requirement) (Match, error) {
	if request.Scope != ScopeProduction && request.Scope != ScopeTest {
		return Match{}, core.ErrCapabilitiesContract
	}
	owner := request.Package
	switch request.Target {
	case RequirementTargetPackage:
		if request.Effect != EffectUnknown {
			return Match{}, core.ErrCapabilitiesContract
		}
	case RequirementTargetEffect:
		owners := [effectLimit]core.PackageIdentity{EffectFilesystem: core.PackageFilestore, EffectProcess: core.PackageProcess, EffectTransport: core.PackageExchange, EffectTime: core.PackageTemporal, EffectEntropy: core.PackageKeygen, EffectSecret: core.PackageSecretStore, EffectHost: core.PackageHostFacts, EffectLocking: core.PackageFileLock, EffectSignal: core.PackageShutdown, EffectObjectStorage: core.PackageObjectStore}
		if request.Package != core.PackageUnknown || request.Effect < EffectFilesystem || request.Effect >= effectLimit {
			return Match{}, core.ErrCapabilitiesContract
		}
		owner = owners[request.Effect]
	default:
		return Match{}, core.ErrCapabilitiesContract
	}
	for contract := range core.PrimitiveArchitecture().Packages() {
		if contract.Identity != owner {
			continue
		}
		if request.Scope == ScopeProduction && contract.Kind != core.PackageKindProduction {
			return Match{}, core.ErrCapabilityUnavailable
		}
		return Match{Requirement: request, Capability: Capability{Package: owner, Kind: contract.Kind, Role: contract.Role}}, nil
	}
	return Match{}, core.ErrCapabilitiesContract
}

func FuzzStandardSymbolNamespaceClosure(f *testing.F) {
	for _, suffix := range []string{"", "Future", "_", "0", "\x00", "\xff", "界"} {
		f.Add(suffix, false, false)
		f.Add(suffix, true, false)
		f.Add(suffix, false, true)
	}
	f.Fuzz(func(t *testing.T, suffix string, foreign, method bool) {
		symbol := handoffSymbol(t)
		selector, err := ParseSymbolName(symbolReadFile + suffix)
		if err != nil {
			if !errors.Is(err, core.ErrCapabilitiesContract) {
				t.Fatal(err)
			}
			return
		}
		symbol.Selector = selector
		if foreign {
			path, err := gomodule.ParseImportPath("example.invalid/" + symbol.ImportPath.String())
			if err != nil {
				t.Fatal(err)
			}
			symbol.ImportPath = path
		}
		if method {
			receiver := selector
			symbol.Receiver = &receiver
		}
		want := Classification{Disposition: StandardSymbolUnresolved}
		if suffix == "" && !foreign && !method {
			want = Classification{Disposition: StandardSymbolEffect, Effect: EffectFilesystem, Operation: OperationReadFile}
		}
		got, err := ResolveStandardSymbol(symbol)
		if err != nil || !got.Classification.Equal(want) || got.Symbol != symbol {
			t.Fatalf("symbol %+v = (%+v,%v), want %+v", symbol, got, err, want)
		}
	})
}
