package projectstandards

import (
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/capabilities"
	"github.com/deliri/primitive/v2026/core"
)

func TestSourceFileClosedEnumsExhaustEveryUint8Value(t *testing.T) {
	t.Parallel()

	for raw := 0; raw <= math.MaxUint8; raw++ {
		language := SourceLanguage(raw)
		wantLanguage := language >= SourceLanguageGo && language <= SourceLanguageOther
		if language.IsValid() != wantLanguage {
			t.Errorf("SourceLanguage(%d).IsValid() = %t, want %t", raw, language.IsValid(), wantLanguage)
		}

		kind := SourceFileKind(raw)
		wantKind := kind >= SourceFileKindProduction && kind <= SourceFileKindAsset
		if kind.IsValid() != wantKind {
			t.Errorf("SourceFileKind(%d).IsValid() = %t, want %t", raw, kind.IsValid(), wantKind)
		}

		posture := PrimitiveEffectPosture(raw)
		wantPosture := posture >= PrimitiveEffectNotApplicable && posture <= PrimitiveEffectUnresolved
		if posture.IsValid() != wantPosture {
			t.Errorf("PrimitiveEffectPosture(%d).IsValid() = %t, want %t", raw, posture.IsValid(), wantPosture)
		}
	}
}

func TestSourceFileContractHostileBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		setup   func(*testing.T) SourceFile
		name    string
		wantVia bool
	}{
		{name: "valid production policy reaches transport through Exchange", setup: sourceFileFixture, wantVia: true},
		{name: "valid effect-free production policy carries no capability", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Effects = SourceFileEffects{Posture: PrimitiveEffectPurePolicy}
			return got
		}},
		{name: "valid Primitive implementation names its own package", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Package = fixturePath(t, "exchange")
			got.Path = fixturePath(t, "exchange/client.go")
			got.Effects = sourceEffectsFixture(t, PrimitiveEffectImplementation, core.PackageExchange)
			return got
		}},
		{name: "valid direct filesystem bypass names Filestore as required owner", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Effects = sourceEffectsFixture(t, PrimitiveEffectDirectObserved, core.PackageFilestore)
			return got
		}},
		{name: "valid unresolved effect site remains visibly unresolved", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Effects = unresolvedEffectsFixture(t, 1)
			return got
		}},
		{name: "valid partial scan retains resolved capability beside unresolved site", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Effects.Posture = PrimitiveEffectUnresolved
			got.Effects.Unresolved = unresolvedEffectSites(t, 1)
			return got
		}},
		{name: "valid direct bypass remains primary beside an unresolved sibling", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Effects = sourceEffectsFixture(t, PrimitiveEffectDirectObserved, core.PackageFilestore)
			got.Effects.Unresolved = unresolvedEffectSites(t, 1)
			return got
		}},
		{name: "valid test helper file has zero test declarations", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Path = fixturePath(t, "projectstandards/helper_test.go")
			got.Kind = SourceFileKindTest
			got.Declarations = SourceFileDeclarations{}
			got.Effects = SourceFileEffects{Posture: PrimitiveEffectPurePolicy}
			return got
		}},
		{name: "valid test file may use Testserial", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Path = fixturePath(t, "projectstandards/serial_test.go")
			got.Kind = SourceFileKindTest
			got.Declarations = SourceFileDeclarations{TestDeclarations: 1}
			got.Effects = sourceEffectsFixture(t, PrimitiveEffectMediated, core.PackageTestSerial)
			return got
		}, wantVia: true},
		{name: "valid Markdown document is effect-inert", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Path = fixturePath(t, "projectstandards/README.md")
			got.Language = SourceLanguageMarkdown
			got.Kind = SourceFileKindDocumentation
			got.Effects = SourceFileEffects{Posture: PrimitiveEffectNotApplicable}
			return got
		}},
		{name: "valid generated production file preserves its effect posture", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Generated = true
			return got
		}, wantVia: true},
		{name: "boundary exact declaration ceiling is admitted", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Path = fixturePath(t, "projectstandards/ceiling_test.go")
			got.Kind = SourceFileKindTest
			got.Declarations = SourceFileDeclarations{TestDeclarations: SourceFileDeclarationMaximum, Benchmarks: 128, FuzzTargets: 128}
			got.Effects = SourceFileEffects{Posture: PrimitiveEffectPurePolicy}
			return got
		}},
		{name: "boundary one declaration is admitted", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Path = fixturePath(t, "projectstandards/one_test.go")
			got.Kind = SourceFileKindTest
			got.Declarations = SourceFileDeclarations{TestDeclarations: 1}
			got.Effects = SourceFileEffects{Posture: PrimitiveEffectPurePolicy}
			return got
		}},
		{name: "boundary maximum retained source sites remain explicit", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Effects = unresolvedEffectsFixture(t, SourceEffectSiteMaximum)
			return got
		}},
		{name: "boundary complete Primitive catalog is admitted for test scope", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Path = fixturePath(t, "projectstandards/all_capabilities_test.go")
			got.Kind = SourceFileKindTest
			got.Declarations = SourceFileDeclarations{TestDeclarations: 1}
			got.Effects = mediatedEffectsForUses(t, allPrimitiveCapabilityUses(t))
			return got
		}, wantVia: true},
		{name: "invalid unknown source language is rejected", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Language = SourceLanguageUnknown
			return got
		}, wantErr: core.ErrProjectStandardsContract},
		{name: "invalid future source language is rejected", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Language = SourceLanguage(math.MaxUint8)
			return got
		}, wantErr: core.ErrProjectStandardsContract},
		{name: "invalid unknown source kind is rejected", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Kind = SourceFileKindUnknown
			return got
		}, wantErr: core.ErrProjectStandardsContract},
		{name: "invalid future source kind is rejected", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Kind = SourceFileKind(math.MaxUint8)
			return got
		}, wantErr: core.ErrProjectStandardsContract},
		{name: "invalid unknown effect posture is rejected", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Effects.Posture = PrimitiveEffectPostureUnknown
			return got
		}, wantErr: core.ErrProjectStandardsContract},
		{name: "invalid future effect posture is rejected", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Effects.Posture = PrimitiveEffectPosture(math.MaxUint8)
			return got
		}, wantErr: core.ErrProjectStandardsContract},
		{name: "invalid file outside package is rejected", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Path = fixturePath(t, "other/catalog.go")
			return got
		}, wantErr: core.ErrProjectStandardsConflict},
		{name: "invalid production file with test declarations is rejected", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Declarations.TestDeclarations = 1
			return got
		}, wantErr: core.ErrProjectStandardsConflict},
		{name: "invalid document with Go language is rejected", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Path = fixturePath(t, "projectstandards/README.md")
			got.Kind = SourceFileKindDocumentation
			got.Effects = SourceFileEffects{Posture: PrimitiveEffectNotApplicable}
			return got
		}, wantErr: core.ErrProjectStandardsConflict},
		{name: "invalid document with executable posture is rejected", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Path = fixturePath(t, "projectstandards/README.md")
			got.Language = SourceLanguageMarkdown
			got.Kind = SourceFileKindDocumentation
			return got
		}, wantErr: core.ErrProjectStandardsConflict},
		{name: "invalid production use of Testserial preserves typed refusal", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Effects = sourceEffectsFixture(t, PrimitiveEffectMediated, core.PackageTestSerial)
			return got
		}, wantErr: core.ErrCapabilityUnavailable},
		{name: "invalid duplicate capability cannot pad the file record", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Effects.Capabilities = append(got.Effects.Capabilities, PrimitiveCapabilityUse{Package: core.PackageExchange})
			return got
		}, wantErr: core.ErrProjectStandardsConflict},
		{name: "invalid pure policy cannot claim a capability", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Effects.Posture = PrimitiveEffectPurePolicy
			return got
		}, wantErr: core.ErrProjectStandardsConflict},
		{name: "invalid mediated posture cannot omit capability evidence", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Effects.Capabilities = nil
			return got
		}, wantErr: core.ErrProjectStandardsConflict},
		{name: "invalid resolved posture cannot retain unresolved sites", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Effects.Unresolved = unresolvedEffectSites(t, 1)
			return got
		}, wantErr: core.ErrProjectStandardsConflict},
		{name: "invalid unresolved posture requires an unresolved site", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Effects = SourceFileEffects{Posture: PrimitiveEffectUnresolved}
			return got
		}, wantErr: core.ErrProjectStandardsConflict},
		{name: "boundary one declaration above per-file ceiling is rejected", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Path = fixturePath(t, "projectstandards/overflow_test.go")
			got.Kind = SourceFileKindTest
			got.Declarations = SourceFileDeclarations{TestDeclarations: SourceFileDeclarationMaximum + 1}
			got.Effects = SourceFileEffects{Posture: PrimitiveEffectPurePolicy}
			return got
		}, wantErr: core.ErrProjectStandardsContract},
		{name: "boundary benchmark count cannot exceed total declarations", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Path = fixturePath(t, "projectstandards/benchmark_test.go")
			got.Kind = SourceFileKindTest
			got.Declarations = SourceFileDeclarations{TestDeclarations: 1, Benchmarks: 2}
			got.Effects = SourceFileEffects{Posture: PrimitiveEffectPurePolicy}
			return got
		}, wantErr: core.ErrProjectStandardsConflict},
		{name: "boundary fuzz count cannot exceed total declarations", setup: func(t *testing.T) SourceFile {
			got := sourceFileFixture(t)
			got.Path = fixturePath(t, "projectstandards/fuzz_test.go")
			got.Kind = SourceFileKindTest
			got.Declarations = SourceFileDeclarations{TestDeclarations: 1, FuzzTargets: 2}
			got.Effects = SourceFileEffects{Posture: PrimitiveEffectPurePolicy}
			return got
		}, wantErr: core.ErrProjectStandardsConflict},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := testCase.setup(t)
			gotErr := got.Validate()
			if testCase.wantErr == nil {
				if gotErr != nil || got.ExecutesPolicyThroughPrimitive() != testCase.wantVia {
					t.Fatalf("SourceFile.Validate()/ExecutesPolicyThroughPrimitive() = (%v, %t), want (nil, %t)", gotErr, got.ExecutesPolicyThroughPrimitive(), testCase.wantVia)
				}
				return
			}
			if !errors.Is(gotErr, testCase.wantErr) || got.ExecutesPolicyThroughPrimitive() {
				t.Fatalf("SourceFile.Validate()/ExecutesPolicyThroughPrimitive() = (%v, %t), want (errors.Is(..., %v), false)", gotErr, got.ExecutesPolicyThroughPrimitive(), testCase.wantErr)
			}
		})
	}
}

func TestPackageFileCatalogLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive exact files close against package inventory", func(t *testing.T) {
		t.Parallel()

		catalog := fixtureCatalog(t)
		files := fixturePackageFileCatalog(t)
		catalog.Packages[0].Code.Files = &files
		if gotErr := catalog.Validate(); gotErr != nil {
			t.Fatalf("Catalog.Validate() error = %v, want nil", gotErr)
		}
	})

	t.Run("negative one missing test declaration cannot close", func(t *testing.T) {
		t.Parallel()

		catalog := fixtureCatalog(t)
		files := fixturePackageFileCatalog(t)
		files.Files[1].Declarations.TestDeclarations = 0
		catalog.Packages[0].Code.Files = &files
		gotErr := catalog.Validate()
		if !errors.Is(gotErr, core.ErrProjectStandardsConflict) {
			t.Fatalf("Catalog.Validate() error = %v, want %v", gotErr, core.ErrProjectStandardsConflict)
		}
	})

	t.Run("neutral absent file scan remains unavailable rather than zero", func(t *testing.T) {
		t.Parallel()

		catalog := fixtureCatalog(t)
		if catalog.Packages[0].Code.Files != nil {
			t.Fatalf("Code.Files = %+v, want nil unavailable scan", catalog.Packages[0].Code.Files)
		}
		if gotErr := catalog.Validate(); gotErr != nil {
			t.Fatalf("Catalog.Validate() error = %v, want nil", gotErr)
		}
	})
}

func TestPackageFileCatalogCompleteObservationBoundary(t *testing.T) {
	t.Parallel()

	partial := fixturePackageFileCatalog(t)
	if gotErr := partial.Validate(); gotErr != nil {
		t.Fatalf("PackageFileCatalog.Validate() error = %v, want nil", gotErr)
	}
	if gotErr := partial.ValidateComplete(); !errors.Is(gotErr, core.ErrProjectStandardsConflict) {
		t.Fatalf("PackageFileCatalog.ValidateComplete(partial) error = %v, want %v", gotErr, core.ErrProjectStandardsConflict)
	}

	complete := fixturePackageFileCatalog(t)
	for index := range complete.Files {
		complete.Files[index].Imports = &SourceFileImports{}
	}
	if gotErr := complete.ValidateComplete(); gotErr != nil {
		t.Fatalf("PackageFileCatalog.ValidateComplete(complete) error = %v, want nil", gotErr)
	}
}

func TestPackageFileCatalogRejectsOrderingDuplicationAndInventoryDrift(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		mutate  func(*testing.T, *Catalog, *PackageFileCatalog)
		name    string
	}{
		{name: "empty present catalog is not folded into zero", mutate: func(_ *testing.T, _ *Catalog, files *PackageFileCatalog) { files.Files = nil }, wantErr: core.ErrProjectStandardsContract},
		{name: "catalog package differs from code package", mutate: func(t *testing.T, _ *Catalog, files *PackageFileCatalog) { files.Package = fixturePath(t, "other") }, wantErr: core.ErrProjectStandardsConflict},
		{name: "file package differs from catalog package", mutate: func(t *testing.T, _ *Catalog, files *PackageFileCatalog) {
			files.Files[0].Package = fixturePath(t, "other")
		}, wantErr: core.ErrProjectStandardsConflict},
		{name: "file paths are out of canonical order", mutate: func(_ *testing.T, _ *Catalog, files *PackageFileCatalog) {
			files.Files[0], files.Files[1] = files.Files[1], files.Files[0]
		}, wantErr: core.ErrProjectStandardsConflict},
		{name: "duplicate file path is rejected", mutate: func(_ *testing.T, _ *Catalog, files *PackageFileCatalog) { files.Files[1].Path = files.Files[0].Path }, wantErr: core.ErrProjectStandardsConflict},
		{name: "file count differs from inventory", mutate: func(_ *testing.T, catalog *Catalog, _ *PackageFileCatalog) {
			catalog.Packages[0].Code.Inventory.Files++
		}, wantErr: core.ErrProjectStandardsConflict},
		{name: "test-file count differs from inventory", mutate: func(_ *testing.T, catalog *Catalog, _ *PackageFileCatalog) {
			catalog.Packages[0].Code.Inventory.TestFiles = 0
		}, wantErr: core.ErrProjectStandardsConflict},
		{name: "document count differs from inventory", mutate: func(_ *testing.T, catalog *Catalog, files *PackageFileCatalog) {
			files.Files[0].Language = SourceLanguageMarkdown
			files.Files[0].Kind = SourceFileKindDocumentation
			files.Files[0].Effects = SourceFileEffects{Posture: PrimitiveEffectNotApplicable}
			catalog.Packages[0].Code.Inventory.Documents = 0
		}, wantErr: core.ErrProjectStandardsConflict},
		{name: "benchmark count differs from inventory", mutate: func(_ *testing.T, catalog *Catalog, files *PackageFileCatalog) {
			files.Files[1].Declarations.Benchmarks = 1
			catalog.Packages[0].Code.Inventory.Benchmarks = 0
		}, wantErr: core.ErrProjectStandardsConflict},
		{name: "fuzz-target count differs from inventory", mutate: func(_ *testing.T, catalog *Catalog, files *PackageFileCatalog) {
			files.Files[1].Declarations.FuzzTargets = 1
			catalog.Packages[0].Code.Inventory.FuzzTargets = 0
		}, wantErr: core.ErrProjectStandardsConflict},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			catalog := fixtureCatalog(t)
			files := fixturePackageFileCatalog(t)
			testCase.mutate(t, &catalog, &files)
			catalog.Packages[0].Code.Files = &files
			gotErr := catalog.Validate()
			if !errors.Is(gotErr, testCase.wantErr) {
				t.Fatalf("Catalog.Validate() error = %v, want %v", gotErr, testCase.wantErr)
			}
		})
	}
}

func sourceFileFixture(t *testing.T) SourceFile {
	t.Helper()

	return SourceFile{
		Path:     fixturePath(t, "projectstandards/catalog.go"),
		Package:  fixturePath(t, "projectstandards"),
		Language: SourceLanguageGo,
		Kind:     SourceFileKindProduction,
		Effects:  sourceEffectsFixture(t, PrimitiveEffectMediated, core.PackageExchange),
	}
}

func fixturePackageFileCatalog(t *testing.T) PackageFileCatalog {
	t.Helper()

	path := fixturePath(t, "projectstandards")
	return PackageFileCatalog{
		Package: path,
		Files: []SourceFile{
			{
				Path: fixturePath(t, "projectstandards/catalog.go"), Package: path,
				Language: SourceLanguageGo, Kind: SourceFileKindProduction,
				Effects: sourceEffectsFixture(t, PrimitiveEffectMediated, core.PackageExchange),
			},
			{
				Path: fixturePath(t, "projectstandards/catalog_test.go"), Package: path,
				Language: SourceLanguageGo, Kind: SourceFileKindTest,
				Declarations: SourceFileDeclarations{TestDeclarations: 1},
				Effects:      SourceFileEffects{Posture: PrimitiveEffectPurePolicy},
			},
		},
	}
}

func sourceEffectsFixture(t *testing.T, posture PrimitiveEffectPosture, capability core.PackageIdentity) SourceFileEffects {
	t.Helper()

	use := PrimitiveCapabilityUse{Package: capability}
	site := sourceEffectSiteFixture(t, &use, 1)
	effects := SourceFileEffects{Capabilities: []PrimitiveCapabilityUse{use}, Posture: posture}
	switch posture {
	case PrimitiveEffectDirectObserved:
		effects.Direct = []SourceEffectSite{site}
	case PrimitiveEffectMediated:
		effects.Mediated = []SourceEffectSite{site}
	case PrimitiveEffectImplementation:
		effects.Implementation = []SourceEffectSite{site}
	}
	return effects
}

func unresolvedEffectsFixture(t *testing.T, count int) SourceFileEffects {
	t.Helper()

	return SourceFileEffects{Posture: PrimitiveEffectUnresolved, Unresolved: unresolvedEffectSites(t, count)}
}

func unresolvedEffectSites(t *testing.T, count int) []SourceEffectSite {
	t.Helper()

	sites := make([]SourceEffectSite, count)
	for index := range sites {
		sites[index] = sourceEffectSiteFixture(t, nil, uint32(index+1))
	}
	return sites
}

func sourceEffectSiteFixture(t *testing.T, capability *PrimitiveCapabilityUse, line uint32) SourceEffectSite {
	t.Helper()

	selector := fixtureIdentifier(t, "Observe")
	return SourceEffectSite{
		Capability: capability,
		ImportPath: fixturePath(t, "example.com/effect"),
		Selector:   &selector,
		Line:       line,
		Column:     1,
	}
}

func mediatedEffectsForUses(t *testing.T, uses []PrimitiveCapabilityUse) SourceFileEffects {
	t.Helper()

	sites := make([]SourceEffectSite, len(uses))
	for index := range uses {
		use := uses[index]
		sites[index] = sourceEffectSiteFixture(t, &use, uint32(index+1))
	}
	return SourceFileEffects{Capabilities: uses, Mediated: sites, Posture: PrimitiveEffectMediated}
}

func allPrimitiveCapabilityUses(t *testing.T) []PrimitiveCapabilityUse {
	t.Helper()

	catalog, gotErr := capabilities.All()
	if gotErr != nil {
		t.Fatalf("capabilities.All() error = %v, want nil", gotErr)
	}
	got := make([]PrimitiveCapabilityUse, 0, core.PrimitivePackageCount)
	for capability := range catalog.Capabilities() {
		got = append(got, PrimitiveCapabilityUse{Package: capability.Package})
	}
	return got
}
