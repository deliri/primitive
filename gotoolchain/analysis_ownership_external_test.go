package gotoolchain_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"github.com/deliri/primitive/v2026/hostfacts"
	"golang.org/x/tools/go/packages"
)

func TestAnalysisPackageOwnershipLayerTriad(t *testing.T) {
	t.Parallel()
	limits, err := gotoolchain.DefaultLimits()
	if err != nil {
		t.Fatal(err)
	}
	capability, err := gotoolchain.Open(t.Context(), gotoolchain.Configuration{Workspace: gotoolchain.WorkspaceModeDisabled, Limits: limits})
	if err != nil {
		t.Fatal(err)
	}
	directory, err := hostfacts.WorkingDirectory()
	if err != nil {
		t.Fatal(err)
	}
	path, err := gomodule.ParseImportPath(core.PrimitiveModulePath + "/gotoolchain/testdata/analysisgenerated")
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := capability.AnalyzePackage(t.Context(), gotoolchain.AnalysisRequest{WorkingDirectory: directory, Package: path})
	if err != nil || len(baseline.Units) != 1 || !analysisHasDeclaration(baseline, "Generated") {
		t.Fatalf("AnalyzePackage() = (%d units,%v), want one real unit declaring Generated", len(baseline.Units), err)
	}
	if err := baseline.Validate(); err != nil {
		t.Fatalf("Validate(real compiler unit) error = %v, want nil", err)
	}
	foreignPath, err := gomodule.ParseImportPath(core.PrimitiveModulePath + "/gomodule")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := capability.AnalyzePackage(t.Context(), gotoolchain.AnalysisRequest{WorkingDirectory: directory, Package: foreignPath})
	if err != nil || len(foreign.Units) != 1 {
		t.Fatalf("AnalyzePackage(foreign package) = (%d units,%v), want one real unit", len(foreign.Units), err)
	}
	for _, tc := range []struct {
		name  string
		units []*packages.Package
	}{
		{name: "foreign compiler unit cannot join the requested subject", units: []*packages.Package{baseline.Units[0], foreign.Units[0]}},
		{name: "duplicate unit cannot inflate retained analysis", units: []*packages.Package{baseline.Units[0], baseline.Units[0]}},
		{name: "absent units cannot become compiler evidence"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			candidate := baseline
			candidate.IncludeTests = true
			candidate.Units = tc.units
			if err := candidate.Validate(); !errors.Is(err, core.ErrGoToolchainContract) {
				t.Fatalf("Validate(mutated compiler units) error = %v, want %v", err, core.ErrGoToolchainContract)
			}
		})
	}
}
