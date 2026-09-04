package gotoolchain_test

import (
	"context"
	"errors"
	"go/ast"
	"go/types"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"github.com/deliri/primitive/v2026/hostfacts"
)

func TestCapabilityProductionPathLayerTriad(t *testing.T) {
	t.Parallel()

	limits, err := gotoolchain.DefaultLimits()
	if err != nil {
		t.Fatalf("gotoolchain.DefaultLimits() error = %v, want nil", err)
	}
	capability, err := gotoolchain.Open(context.Background(), gotoolchain.Configuration{
		Workspace: gotoolchain.WorkspaceModeDisabled,
		Limits:    limits,
	})
	if err != nil {
		t.Fatalf("gotoolchain.Open() error = %v, want nil", err)
	}
	directory, err := hostfacts.WorkingDirectory()
	if err != nil {
		t.Fatalf("hostfacts.WorkingDirectory() error = %v, want nil", err)
	}
	directory, err = directory.Parent()
	if err != nil {
		t.Fatalf("working directory Parent() error = %v, want nil", err)
	}

	t.Run("positive cmd go emits typed module build package and compilation evidence", func(t *testing.T) {
		t.Parallel()

		request := gotoolchain.ObservationRequest{WorkingDirectory: directory}
		module, moduleErr := capability.ObserveModule(context.Background(), request)
		if moduleErr != nil || module.String() != core.PrimitiveModulePath {
			t.Fatalf("Capability.ObserveModule() = (%q, %v), want (%q, nil)", module.String(), moduleErr, core.PrimitiveModulePath)
		}
		build, buildErr := capability.ObserveBuildContext(context.Background(), request)
		if buildErr != nil {
			t.Fatalf("Capability.ObserveBuildContext() error = %v, want nil", buildErr)
		}
		if err := build.Validate(); err != nil {
			t.Fatalf("BuildContext.Validate() error = %v, want nil", err)
		}
		catalog, listErr := capability.ListPackages(context.Background(), gotoolchain.ListRequest{
			WorkingDirectory: directory,
			Pattern:          "./gomodule",
		})
		if listErr != nil {
			t.Fatalf("Capability.ListPackages() error = %v, want nil", listErr)
		}
		if len(catalog.Packages) != 1 || catalog.Packages[0].ImportPath.String() != core.PrimitiveModulePath+"/gomodule" {
			t.Fatalf("Capability.ListPackages() = %+v, want exact gomodule package", catalog)
		}
		compilation, compileErr := capability.CompilePackage(context.Background(), gotoolchain.CompileRequest{
			WorkingDirectory: directory,
			Pattern:          "./gotoolchain/testdata/compileonly",
		})
		if compileErr != nil {
			t.Fatalf("Capability.CompilePackage() error = %v, want nil", compileErr)
		}
		if err := compilation.Validate(); err != nil {
			t.Fatalf("Compilation.Validate() error = %v, want nil", err)
		}
		packagePath, pathErr := gomodule.ParseImportPath(core.PrimitiveModulePath + "/gotoolchain")
		if pathErr != nil {
			t.Fatalf("gomodule.ParseImportPath(gotoolchain) error = %v, want nil", pathErr)
		}
		analysis, analysisErr := capability.AnalyzePackage(context.Background(), gotoolchain.AnalysisRequest{
			WorkingDirectory: directory,
			Package:          packagePath,
			IncludeTests:     true,
		})
		if analysisErr != nil {
			t.Fatalf("Capability.AnalyzePackage() error = %v, want nil", analysisErr)
		}
		gotDeclaration := analysisHasDeclaration(analysis, "Capability")
		if !gotDeclaration {
			t.Fatalf("Capability.AnalyzePackage() type scope has Capability = %t, want true", gotDeclaration)
		}
		gotPackageFunction, gotMethodSelection := analysisHasCompilerObjects(analysis)
		if !gotPackageFunction || !gotMethodSelection {
			t.Fatalf("Capability.AnalyzePackage() objects = package-function:%t method-selection:%t, want both true", gotPackageFunction, gotMethodSelection)
		}
	})

	t.Run("negative flag-shaped package operands are refused before cmd go", func(t *testing.T) {
		t.Parallel()

		got, gotErr := capability.CompilePackage(context.Background(), gotoolchain.CompileRequest{
			WorkingDirectory: directory,
			Pattern:          "-run=TestMain",
		})
		if !errors.Is(gotErr, core.ErrGoToolchainContract) || got != (gotoolchain.Compilation{}) {
			t.Fatalf("Capability.CompilePackage(flag operand) = (%v, %v), want zero and errors.Is(..., %v)", got, gotErr, core.ErrGoToolchainContract)
		}
	})

	t.Run("positive exact generated exclusion keeps authored compiler objects with explicit partial state", func(t *testing.T) {
		t.Parallel()

		packagePath, pathErr := gomodule.ParseImportPath(core.PrimitiveModulePath + "/gotoolchain/testdata/analysisgenerated")
		excluded, excludedErr := directory.ResolveText("gotoolchain/testdata/analysisgenerated/generated.go")
		if err := errors.Join(pathErr, excludedErr); err != nil {
			t.Fatalf("generated analysis coordinates error = %v, want nil", err)
		}
		analysis, analysisErr := capability.AnalyzePackage(context.Background(), gotoolchain.AnalysisRequest{
			WorkingDirectory: directory,
			Package:          packagePath,
			SyntaxExclusions: []core.AbsolutePath{excluded},
		})
		if analysisErr != nil {
			t.Fatalf("Capability.AnalyzePackage(generated exclusion) error = %v, want nil partial graph", analysisErr)
		}
		if analysis.State != gotoolchain.AnalysisStatePartial || len(analysis.SyntaxExclusions) != 1 || analysis.SyntaxExclusions[0] != excluded {
			t.Fatalf("generated exclusion analysis = state:%s exclusions:%v, want partial and exact path", analysis.State, analysis.SyntaxExclusions)
		}
		gotAuthoredObject := analysisFileHasCompilerObject(analysis, "authored.go", "os", "Open")
		if !gotAuthoredObject {
			t.Fatalf("generated exclusion authored.go compiler-resolved os.Open = %t, want true", gotAuthoredObject)
		}
		gotGeneratedPackageClauseOnly := analysisFileHasPackageClauseOnly(analysis, "generated.go")
		if !gotGeneratedPackageClauseOnly {
			t.Fatalf("generated exclusion generated.go package-clause-only syntax = %t, want true", gotGeneratedPackageClauseOnly)
		}
	})

	t.Run("negative absent package pattern is refused before cmd go", func(t *testing.T) {
		t.Parallel()

		got, gotErr := capability.ListPackages(context.Background(), gotoolchain.ListRequest{WorkingDirectory: directory})
		if !errors.Is(gotErr, core.ErrGoToolchainContract) || len(got.Packages) != 0 {
			t.Fatalf("Capability.ListPackages(absent pattern) = (%v, %v), want zero and errors.Is(..., %v)", got, gotErr, core.ErrGoToolchainContract)
		}
	})

	t.Run("negative ill typed package returns typed compiler output refusal", func(t *testing.T) {
		t.Parallel()

		packagePath, pathErr := gomodule.ParseImportPath(core.PrimitiveModulePath + "/gotoolchain/testdata/analysisbroken")
		if pathErr != nil {
			t.Fatalf("gomodule.ParseImportPath(analysisbroken) error = %v, want nil", pathErr)
		}
		got, gotErr := capability.AnalyzePackage(context.Background(), gotoolchain.AnalysisRequest{WorkingDirectory: directory, Package: packagePath})
		if !errors.Is(gotErr, core.ErrGoToolchainOutput) || len(got.Units) != 0 {
			t.Fatalf("Capability.AnalyzePackage(ill typed) = (%v units, %v), want zero and errors.Is(..., %v)", len(got.Units), gotErr, core.ErrGoToolchainOutput)
		}
	})

	t.Run("negative syntax exclusions cannot escape the declared analysis root", func(t *testing.T) {
		t.Parallel()

		packagePath, pathErr := gomodule.ParseImportPath(core.PrimitiveModulePath + "/gotoolchain")
		foreignRoot, rootErr := core.ParseAbsolutePath(t.TempDir())
		foreign, foreignErr := foreignRoot.ResolveText("foreign.go")
		if err := errors.Join(pathErr, rootErr, foreignErr); err != nil {
			t.Fatalf("foreign analysis coordinates error = %v, want nil", err)
		}
		got, gotErr := capability.AnalyzePackage(context.Background(), gotoolchain.AnalysisRequest{
			WorkingDirectory: directory, Package: packagePath, SyntaxExclusions: []core.AbsolutePath{foreign},
		})
		if !errors.Is(gotErr, core.ErrGoToolchainContract) || len(got.Units) != 0 {
			t.Fatalf("Capability.AnalyzePackage(foreign exclusion) = (%d units, %v), want zero and errors.Is(..., %v)", len(got.Units), gotErr, core.ErrGoToolchainContract)
		}
	})

	t.Run("negative syntax exclusions admit only Go source", func(t *testing.T) {
		t.Parallel()

		packagePath, pathErr := gomodule.ParseImportPath(core.PrimitiveModulePath + "/gotoolchain")
		nonGo, nonGoErr := directory.ResolveText("go.mod")
		if err := errors.Join(pathErr, nonGoErr); err != nil {
			t.Fatalf("non-Go analysis coordinates error = %v, want nil", err)
		}
		got, gotErr := capability.AnalyzePackage(context.Background(), gotoolchain.AnalysisRequest{
			WorkingDirectory: directory, Package: packagePath, SyntaxExclusions: []core.AbsolutePath{nonGo},
		})
		if !errors.Is(gotErr, core.ErrGoToolchainContract) || len(got.Units) != 0 {
			t.Fatalf("Capability.AnalyzePackage(non-Go exclusion) = (%d units, %v), want zero and errors.Is(..., %v)", len(got.Units), gotErr, core.ErrGoToolchainContract)
		}
	})

	t.Run("neutral pre-cancelled observation preserves cancellation and emits no module", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		got, gotErr := capability.ObserveModule(ctx, gotoolchain.ObservationRequest{WorkingDirectory: directory})
		if !errors.Is(gotErr, context.Canceled) || !errors.Is(gotErr, core.ErrGoToolchainExecution) || got.String() != "" {
			t.Fatalf("Capability.ObserveModule(cancelled) = (%q, %v), want zero with context.Canceled and %v", got.String(), gotErr, core.ErrGoToolchainExecution)
		}
		packagePath, pathErr := gomodule.ParseImportPath(core.PrimitiveModulePath + "/gotoolchain")
		if pathErr != nil {
			t.Fatalf("gomodule.ParseImportPath(gotoolchain) error = %v, want nil", pathErr)
		}
		analysis, analysisErr := capability.AnalyzePackage(ctx, gotoolchain.AnalysisRequest{WorkingDirectory: directory, Package: packagePath})
		if !errors.Is(analysisErr, context.Canceled) || !errors.Is(analysisErr, core.ErrGoToolchainExecution) || len(analysis.Units) != 0 {
			t.Fatalf("Capability.AnalyzePackage(cancelled) = (%v units, %v), want zero with context.Canceled and %v", len(analysis.Units), analysisErr, core.ErrGoToolchainExecution)
		}
	})
}

func analysisHasDeclaration(analysis gotoolchain.PackageAnalysis, name string) bool {
	for _, unit := range analysis.Units {
		if unit.PkgPath == analysis.Package.String() && unit.Types.Scope().Lookup(name) != nil {
			return true
		}
	}
	return false
}

func analysisHasCompilerObjects(analysis gotoolchain.PackageAnalysis) (bool, bool) {
	packageFunction := false
	methodSelection := false
	for _, unit := range analysis.Units {
		for _, file := range unit.Syntax {
			ast.Inspect(file, func(node ast.Node) bool {
				selector, ok := node.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if object, ok := unit.TypesInfo.Uses[selector.Sel].(*types.Func); ok && compilerObjectIsProcess(object, "Resolve") {
					packageFunction = true
				}
				selection := unit.TypesInfo.Selections[selector]
				if selection != nil && compilerObjectIsProcess(selection.Obj(), "Strings") {
					methodSelection = true
				}
				return true
			})
		}
	}
	return packageFunction, methodSelection
}

func compilerObjectIsProcess(object types.Object, wantName string) bool {
	return object != nil && object.Pkg() != nil && object.Pkg().Path() == core.PrimitiveModulePath+"/process" && object.Name() == wantName
}

func analysisFileHasCompilerObject(analysis gotoolchain.PackageAnalysis, fileName, packagePath, objectName string) bool {
	for _, unit := range analysis.Units {
		for index, file := range unit.Syntax {
			if filepath.Base(unit.CompiledGoFiles[index]) != fileName {
				continue
			}
			found := false
			ast.Inspect(file, func(node ast.Node) bool {
				selector, ok := node.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				object, ok := unit.TypesInfo.Uses[selector.Sel].(*types.Func)
				found = found || ok && object.Pkg() != nil && object.Pkg().Path() == packagePath && object.Name() == objectName
				return !found
			})
			if found {
				return true
			}
		}
	}
	return false
}

func analysisFileHasPackageClauseOnly(analysis gotoolchain.PackageAnalysis, fileName string) bool {
	for _, unit := range analysis.Units {
		for index, file := range unit.Syntax {
			if filepath.Base(unit.CompiledGoFiles[index]) == fileName && len(file.Decls) == 0 {
				return true
			}
		}
	}
	return false
}

func TestCompilerScalarsRejectUnknownAndPreserveCanonicalValues(t *testing.T) {
	t.Parallel()

	workspaceModes := []struct {
		name       string
		wantString string
		value      gotoolchain.WorkspaceMode
		wantValid  bool
	}{
		{name: "zero mode is outside the compiler-owned domain", value: gotoolchain.WorkspaceModeUnknown, wantString: core.UnknownEnumDiagnostic},
		{name: "ambient mode preserves cmd go workspace discovery", value: gotoolchain.WorkspaceModeAmbient, wantValid: true, wantString: "ambient"},
		{name: "disabled mode seals cmd go away from ambient workspaces", value: gotoolchain.WorkspaceModeDisabled, wantValid: true, wantString: "workspace_disabled"},
		{name: "next mode is refused instead of becoming future policy", value: gotoolchain.WorkspaceModeDisabled + 1, wantString: core.UnknownEnumDiagnostic},
		{name: "maximum uint8 mode cannot enter the closed domain", value: gotoolchain.WorkspaceMode(^uint8(0)), wantString: core.UnknownEnumDiagnostic},
	}
	for _, tc := range workspaceModes {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.value.Validate()
			if tc.value.IsValid() != tc.wantValid || (gotErr == nil) != tc.wantValid || tc.value.String() != tc.wantString {
				t.Fatalf("WorkspaceMode(%d) valid/error/string = (%t, %v, %q), want (%t, matching error posture, %q)", tc.value, tc.value.IsValid(), gotErr, tc.value.String(), tc.wantValid, tc.wantString)
			}
			tc.value.OffWireEnum()
		})
	}

	versions := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "minimum canonical release", value: "go1.0"},
		{name: "current canonical patch release", value: "go1.27.0"},
		{name: "future canonical major component", value: "go1.100.2"},
		{name: "absent token refused", value: "", wantErr: true},
		{name: "missing go prefix refused", value: "1.27.0", wantErr: true},
		{name: "missing major one refused", value: "go2.0", wantErr: true},
		{name: "empty numeric component refused", value: "go1..2", wantErr: true},
		{name: "trailing empty numeric component refused", value: "go1.27.", wantErr: true},
		{name: "nonnumeric suffix refused", value: "go1.27rc1", wantErr: true},
		{name: "whitespace refused", value: "go1.27.0 ", wantErr: true},
	}
	for _, tc := range versions {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := gotoolchain.ParseToolchainVersion(tc.value)
			if (gotErr != nil) != tc.wantErr {
				t.Fatalf("ParseToolchainVersion(%q) error = %v, want error %t", tc.value, gotErr, tc.wantErr)
			}
			if tc.wantErr {
				if !errors.Is(gotErr, core.ErrGoToolchainContract) || got.String() != "" {
					t.Fatalf("ParseToolchainVersion(%q) = (%q, %v), want zero and typed rejection", tc.value, got.String(), gotErr)
				}
				return
			}
			if got.String() != tc.value {
				t.Fatalf("ParseToolchainVersion(%q).String() = %q, want %q", tc.value, got.String(), tc.value)
			}
		})
	}
}
